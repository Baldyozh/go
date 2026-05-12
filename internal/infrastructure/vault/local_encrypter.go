package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/Baldyozh/log-processor/internal/infrastructure/config"
	"github.com/hashicorp/vault/api"
)

// LocalEncrypter performs local encryption using keys fetched from Vault
type LocalEncrypter struct {
	vaultClient     *api.Client
	vaultConfig     *config.VaultConfig
	localConfig     config.LocalEncryptionConfig
	sensitiveFields []string

	// Key management
	encryptionKey  []byte
	keyMutex       sync.RWMutex
	lastKeyRefresh time.Time

	// Performance metrics
	encryptCount    int64
	lastMetricsTime time.Time
}

// NewLocalEncrypter creates a new local encrypter
func NewLocalEncrypter(vaultManager *Manager, cfg config.LocalEncryptionConfig, sensitiveFields []string) *LocalEncrypter {
	return &LocalEncrypter{
		vaultClient:     vaultManager.Client,
		vaultConfig:     createVaultConfig(vaultManager.config),
		localConfig:     cfg,
		sensitiveFields: sensitiveFields,
		lastMetricsTime: time.Now(),
	}
}

// createVaultConfig converts internal vault.Config to config.VaultConfig
func createVaultConfig(cfg *Config) *config.VaultConfig {
	return &config.VaultConfig{
		Address:     cfg.Address,
		Token:       cfg.Token,
		TransitPath: cfg.TransitPath,
		KeyName:     cfg.KeyName,
	}
}

// Initialize fetches encryption key from Vault
func (le *LocalEncrypter) Initialize() error {
	log.Printf("Initializing local encrypter with algorithm: %s", le.localConfig.Algorithm)

	if err := le.refreshKey(); err != nil {
		return fmt.Errorf("failed to initialize encryption key: %w", err)
	}

	// Start key rotation goroutine
	if le.localConfig.KeyRotationInterval > 0 {
		go le.keyRotationWorker()
	}

	log.Printf("Local encrypter initialized successfully")
	return nil
}

// refreshKey fetches a new encryption key from Vault
func (le *LocalEncrypter) refreshKey() error {
	le.keyMutex.Lock()
	defer le.keyMutex.Unlock()

	log.Printf("Fetching encryption key from Vault...")

	// Request data encryption key from Vault
	secret, err := le.vaultClient.Logical().Write(
		fmt.Sprintf("%s/datakey/plaintext/%s", le.vaultConfig.TransitPath, le.vaultConfig.KeyName),
		map[string]interface{}{
			"bits": 256, // 256-bit key for AES-256
		},
	)

	if err != nil {
		return fmt.Errorf("failed to get data key from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return fmt.Errorf("no data returned from Vault")
	}

	// Extract plaintext key from response
	plaintextKey, ok := secret.Data["plaintext"].(string)
	if !ok {
		return fmt.Errorf("no plaintext key in Vault response")
	}

	// Decode base64 key
	key, err := base64.StdEncoding.DecodeString(plaintextKey)
	if err != nil {
		return fmt.Errorf("failed to decode encryption key: %w", err)
	}

	le.encryptionKey = key
	le.lastKeyRefresh = time.Now()

	log.Printf("Successfully fetched encryption key from Vault (key ID: %v)", secret.Data["key_id"])
	return nil
}

// keyRotationWorker periodically refreshes the encryption key
func (le *LocalEncrypter) keyRotationWorker() {
	ticker := time.NewTicker(le.localConfig.KeyRotationInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := le.refreshKey(); err != nil {
			log.Printf("Failed to rotate encryption key: %v", err)
		} else {
			log.Printf("Successfully rotated encryption key")
		}
	}
}

// EncryptFields encrypts specified fields in JSON data using local encryption
func (le *LocalEncrypter) EncryptFields(data []byte, fields []string) ([]byte, error) {
	le.keyMutex.RLock()
	currentKey := le.encryptionKey
	le.keyMutex.RUnlock()

	if currentKey == nil {
		return nil, fmt.Errorf("encryption key not initialized")
	}

	// Parse JSON data
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Encrypt specified fields
	for _, field := range fields {
		if value, exists := jsonData[field]; exists {
			encryptedValue, err := le.encryptValue(fmt.Sprintf("%v", value), currentKey)
			if err != nil {
				log.Printf("Failed to encrypt field %s: %v", field, err)
				continue
			}
			jsonData[field] = encryptedValue
		}
	}

	// Marshal back to JSON
	encryptedData, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal encrypted JSON: %w", err)
	}

	// Update metrics
	le.encryptCount++
	le.logMetrics()

	return encryptedData, nil
}

// encryptValue encrypts a single value using AES-GCM
func (le *LocalEncrypter) encryptValue(value string, key []byte) (string, error) {
	switch le.localConfig.Algorithm {
	case "aes-gcm":
		return le.encryptAESGCM(value, key)
	default:
		return "", fmt.Errorf("unsupported encryption algorithm: %s", le.localConfig.Algorithm)
	}
}

// encryptAESGCM encrypts value using AES-GCM
func (le *LocalEncrypter) encryptAESGCM(plaintext string, key []byte) (string, error) {
	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to create nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64 encoded result (nonce + ciphertext)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// logMetrics logs performance metrics periodically
func (le *LocalEncrypter) logMetrics() {
	now := time.Now()
	if now.Sub(le.lastMetricsTime) >= 30*time.Second {
		duration := now.Sub(le.lastMetricsTime).Seconds()
		rate := float64(le.encryptCount) / duration

		log.Printf("Local encryption metrics: %.2f encryptions/sec, total: %d", rate, le.encryptCount)

		le.encryptCount = 0
		le.lastMetricsTime = now
	}
}

// GetKeyInfo returns information about the current encryption key
func (le *LocalEncrypter) GetKeyInfo() map[string]interface{} {
	le.keyMutex.RLock()
	defer le.keyMutex.RUnlock()

	return map[string]interface{}{
		"algorithm":         le.localConfig.Algorithm,
		"last_key_refresh":  le.lastKeyRefresh,
		"rotation_interval": le.localConfig.KeyRotationInterval,
		"key_initialized":   le.encryptionKey != nil,
	}
}

// Close cleanup resources
func (le *LocalEncrypter) Close() error {
	log.Printf("Local encrypter closed")
	return nil
}
