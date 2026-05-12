package vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// Config holds Vault connection configuration
type Config struct {
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

// Manager handles Vault key management and encryption operations
type Manager struct {
	Client         *api.Client
	config         *Config
	keys           map[int]*KeyVersion
	currentVersion int
	mutex          sync.RWMutex
	lastRotation   time.Time
}

// NewManager creates a new Vault key manager
func NewManager(config *Config) (*Manager, error) {
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

	km := &Manager{
		Client: client,
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
func (km *Manager) initializeKey() error {
	secret, err := km.Client.Logical().Read(fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName))
	if err != nil {
		return fmt.Errorf("failed to check key existence: %w", err)
	}

	if secret != nil {
		return nil
	}

	// Create new key
	data := map[string]interface{}{
		"type":        "aes256-gcm96",
		"auto_rotate": "24h",
	}

	_, err = km.Client.Logical().Write(
		fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName),
		data,
	)
	if err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}

	return nil
}

// loadKeys loads all key versions from Vault
func (km *Manager) loadKeys() error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	secret, err := km.Client.Logical().Read(fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName))
	if err != nil {
		return fmt.Errorf("failed to read key info: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return fmt.Errorf("key not found in Vault")
	}

	keysData, ok := secret.Data["keys"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid key data format")
	}

	km.keys = make(map[int]*KeyVersion)

	for versionStr := range keysData {
		version := 0
		if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
			continue
		}

		kv := &KeyVersion{
			Version:   version,
			CreatedAt: time.Now(),
			IsActive:  false,
		}

		if latestVersionNum, ok := secret.Data["latest_version"].(json.Number); ok {
			if v, err := latestVersionNum.Int64(); err == nil && int(v) == version {
				kv.IsActive = true
				km.currentVersion = version
			}
		}

		km.keys[version] = kv
	}

	if km.currentVersion == 0 && len(km.keys) > 0 {
		for version := range km.keys {
			if version > km.currentVersion {
				km.currentVersion = version
			}
		}
		km.keys[km.currentVersion].IsActive = true
	}

	return nil
}

// rotateKey performs key rotation
func (km *Manager) rotateKey() error {
	_, err := km.Client.Logical().Write(
		fmt.Sprintf("%s/keys/%s/rotate", km.config.TransitPath, km.config.KeyName),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}

	if err := km.loadKeys(); err != nil {
		return fmt.Errorf("failed to reload keys after rotation: %w", err)
	}

	km.lastRotation = time.Now()
	return nil
}

// startKeyRotation starts the automatic key rotation routine
func (km *Manager) startKeyRotation() {
	ticker := time.NewTicker(KeyRotationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := km.rotateKey(); err != nil {
				// In production, use proper logging
			}
		}
	}
}

// GetCurrentKey returns the current active encryption key
func (km *Manager) GetCurrentKey() (*KeyVersion, error) {
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
func (km *Manager) GetKey(version int) (*KeyVersion, error) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	key, exists := km.keys[version]
	if !exists {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return key, nil
}

// Encrypt encrypts data using Vault transit engine
func (km *Manager) Encrypt(plaintext []byte) (*EncryptedData, error) {
	currentKey, err := km.GetCurrentKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get current key: %w", err)
	}

	data := map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	}

	path := fmt.Sprintf("%s/encrypt/%s", km.config.TransitPath, km.config.KeyName)
	secret, err := km.Client.Logical().Write(path, data)
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
func (km *Manager) Decrypt(encryptedData *EncryptedData) ([]byte, error) {
	data := map[string]interface{}{
		"ciphertext": encryptedData.Ciphertext,
	}

	path := fmt.Sprintf("%s/decrypt/%s", km.config.TransitPath, km.config.KeyName)
	secret, err := km.Client.Logical().Write(path, data)
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
func (km *Manager) GetKeyInfo() map[int]*KeyVersion {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	result := make(map[int]*KeyVersion)
	for version, key := range km.keys {
		keyCopy := *key
		result[version] = &keyCopy
	}

	return result
}

// Close cleans up resources
func (km *Manager) Close() error {
	return nil
}

// ValidateConfig validates the Vault configuration
func ValidateConfig(config *Config) error {
	if config.Address == "" {
		return fmt.Errorf("Vault address is required")
	}
	if config.Token == "" {
		return fmt.Errorf("Vault token is required")
	}
	if config.TransitPath == "" {
		return fmt.Errorf("Vault transit path is required")
	}
	if config.KeyName == "" {
		return fmt.Errorf("Vault key name is required")
	}
	if config.TTL <= 0 {
		return fmt.Errorf("Vault cache TTL must be positive")
	}
	return nil
}
