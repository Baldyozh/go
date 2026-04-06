package crypto_vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	// Vault paths and keys
	VaultTransitPath = "transit"
	VaultKeyName     = "kafka-encryption"

	// Key rotation settings
	KeyRotationInterval = 24 * time.Hour // Rotate keys every 24 hours
	KeyCacheTTL         = 1 * time.Hour  // Cache keys for 1 hour
)

// VaultConfig holds Vault connection configuration
type VaultConfig struct {
	Address      string
	Token        string
	TransitPath  string
	KeyName      string
	CacheEnabled bool
	TTL          time.Duration
}

// KeyVersion represents an encryption key version
type KeyVersion struct {
	Version   int        `json:"version"`
	Key       []byte     `json:"key"`
	CreatedAt time.Time  `json:"created_at"`
	IsActive  bool       `json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// EncryptedData contains encrypted data with metadata
type EncryptedData struct {
	Ciphertext  string    `json:"ciphertext"`
	KeyVersion  int       `json:"key_version"`
	EncryptedAt time.Time `json:"encrypted_at"`
	Algorithm   string    `json:"algorithm"`
	TransitPath string    `json:"transit_path"`
}

// VaultKeyManager handles key rotation and Vault integration
type VaultKeyManager struct {
	client         *api.Client
	config         *VaultConfig
	keys           map[int]*KeyVersion
	currentVersion int
	mutex          sync.RWMutex
	lastRotation   time.Time
}

// NewVaultKeyManager creates a new Vault key manager
func NewVaultKeyManager(config *VaultConfig) (*VaultKeyManager, error) {
	if config.TransitPath == "" {
		config.TransitPath = VaultTransitPath
	}
	if config.KeyName == "" {
		config.KeyName = VaultKeyName
	}
	if config.TTL == 0 {
		config.TTL = KeyCacheTTL
	}

	// Create Vault client
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = config.Address

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	client.SetToken(config.Token)

	km := &VaultKeyManager{
		client: client,
		config: config,
		keys:   make(map[int]*KeyVersion),
	}

	// Initialize key in Vault
	if err := km.initializeKey(); err != nil {
		return nil, fmt.Errorf("failed to initialize key: %w", err)
	}

	// Load existing keys
	if err := km.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	// Start key rotation routine
	go km.startKeyRotation()

	return km, nil
}

// initializeKey creates the encryption key in Vault if it doesn't exist
func (km *VaultKeyManager) initializeKey() error {
	// Check if key already exists
	secret, err := km.client.Logical().Read(fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName))
	if err != nil {
		return fmt.Errorf("failed to check key existence: %w", err)
	}

	if secret != nil {
		log.Printf("Key %s already exists in Vault", km.config.KeyName)
		return nil
	}

	// Create new key
	data := map[string]interface{}{
		"type":        "aes256-gcm96",
		"auto_rotate": "24h", // Auto-rotate every 24 hours
	}

	_, err = km.client.Logical().Write(
		fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName),
		data,
	)
	if err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}

	log.Printf("Created new key %s in Vault", km.config.KeyName)
	return nil
}

// loadKeys loads all key versions from Vault
func (km *VaultKeyManager) loadKeys() error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	secret, err := km.client.Logical().Read(fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName))
	if err != nil {
		return fmt.Errorf("failed to read key info: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return fmt.Errorf("key not found in Vault")
	}

	// Parse key versions
	keysData, ok := secret.Data["keys"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid key data format")
	}

	km.keys = make(map[int]*KeyVersion)

	for versionStr, keyInfo := range keysData {
		version := 0
		if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
			continue
		}

		// We don't need to use keyInfo for basic functionality
		_ = keyInfo

		kv := &KeyVersion{
			Version:   version,
			CreatedAt: time.Now(), // Vault doesn't provide creation time in API response
			IsActive:  false,
		}

		// Check if this is the latest version
		if latestVersionNum, ok := secret.Data["latest_version"].(json.Number); ok {
			if v, err := latestVersionNum.Int64(); err == nil && int(v) == version {
				kv.IsActive = true
				km.currentVersion = version
			}
		}

		km.keys[version] = kv
	}

	if km.currentVersion == 0 && len(km.keys) > 0 {
		// If no active version found, use the highest version number
		for version := range km.keys {
			if version > km.currentVersion {
				km.currentVersion = version
			}
		}
		km.keys[km.currentVersion].IsActive = true
	}

	log.Printf("Loaded %d key versions, current version: %d", len(km.keys), km.currentVersion)
	return nil
}

// rotateKey performs key rotation
func (km *VaultKeyManager) rotateKey() error {
	log.Println("Starting key rotation...")

	// Rotate key in Vault
	_, err := km.client.Logical().Write(
		fmt.Sprintf("%s/keys/%s/rotate", km.config.TransitPath, km.config.KeyName),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}

	// Reload keys
	if err := km.loadKeys(); err != nil {
		return fmt.Errorf("failed to reload keys after rotation: %w", err)
	}

	km.lastRotation = time.Now()
	log.Printf("Key rotation completed. New version: %d", km.currentVersion)
	return nil
}

// startKeyRotation starts the automatic key rotation routine
func (km *VaultKeyManager) startKeyRotation() {
	ticker := time.NewTicker(KeyRotationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := km.rotateKey(); err != nil {
				log.Printf("Key rotation failed: %v", err)
			}
		}
	}
}

// GetCurrentKey returns the current active encryption key
func (km *VaultKeyManager) GetCurrentKey() (*KeyVersion, error) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if km.currentVersion == 0 {
		return nil, fmt.Errorf("no active key version available")
	}

	key, exists := km.keys[km.currentVersion]
	if !exists {
		return nil, fmt.Errorf("current key version %d not found", km.currentVersion)
	}

	return key, nil
}

// GetKey returns a specific key version
func (km *VaultKeyManager) GetKey(version int) (*KeyVersion, error) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	key, exists := km.keys[version]
	if !exists {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return key, nil
}

// Encrypt encrypts data using Vault transit engine
func (km *VaultKeyManager) Encrypt(plaintext []byte) (*EncryptedData, error) {
	currentKey, err := km.GetCurrentKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get current key: %w", err)
	}

	// Use Vault transit engine for encryption
	data := map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	}

	path := fmt.Sprintf("%s/encrypt/%s", km.config.TransitPath, km.config.KeyName)
	secret, err := km.client.Logical().Write(path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt with Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no encryption response from Vault")
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid ciphertext response from Vault")
	}

	encryptedData := &EncryptedData{
		Ciphertext:  ciphertext,
		KeyVersion:  currentKey.Version,
		EncryptedAt: time.Now(),
		Algorithm:   "aes256-gcm96",
		TransitPath: km.config.TransitPath,
	}

	return encryptedData, nil
}

// Decrypt decrypts data using Vault transit engine
func (km *VaultKeyManager) Decrypt(encryptedData *EncryptedData) ([]byte, error) {
	// Use Vault transit engine for decryption
	data := map[string]interface{}{
		"ciphertext": encryptedData.Ciphertext,
	}

	path := fmt.Sprintf("%s/decrypt/%s", km.config.TransitPath, km.config.KeyName)
	secret, err := km.client.Logical().Write(path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt with Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no decryption response from Vault")
	}

	plaintext, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid plaintext response from Vault")
	}

	return []byte(plaintext), nil
}

// GetKeyInfo returns information about all key versions
func (km *VaultKeyManager) GetKeyInfo() map[int]*KeyVersion {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[int]*KeyVersion)
	for version, key := range km.keys {
		keyCopy := *key
		result[version] = &keyCopy
	}

	return result
}

// Close cleans up resources
func (km *VaultKeyManager) Close() error {
	// In a real implementation, you might want to clean up resources
	// For now, Vault client doesn't require explicit cleanup
	return nil
}
