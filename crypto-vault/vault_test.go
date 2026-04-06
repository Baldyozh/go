package crypto_vault

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestVaultConfigFromEnv(t *testing.T) {
	// Save original environment variables
	origAddr := os.Getenv("VAULT_ADDR")
	origToken := os.Getenv("VAULT_TOKEN")
	origPath := os.Getenv("VAULT_TRANSIT_PATH")
	origKey := os.Getenv("VAULT_KEY_NAME")
	origCache := os.Getenv("VAULT_CACHE_ENABLED")
	origTTL := os.Getenv("VAULT_CACHE_TTL")

	t.Setenv("VAULT_ADDR", "https://vault.example.com")
	t.Setenv("VAULT_TOKEN", "test-token")
	t.Setenv("VAULT_TRANSIT_PATH", "custom-transit")
	t.Setenv("VAULT_KEY_NAME", "custom-key")
	t.Setenv("VAULT_CACHE_ENABLED", "false")
	t.Setenv("VAULT_CACHE_TTL", "2h")

	defer func() {
		os.Setenv("VAULT_ADDR", origAddr)
		os.Setenv("VAULT_TOKEN", origToken)
		os.Setenv("VAULT_TRANSIT_PATH", origPath)
		os.Setenv("VAULT_KEY_NAME", origKey)
		os.Setenv("VAULT_CACHE_ENABLED", origCache)
		os.Setenv("VAULT_CACHE_TTL", origTTL)
	}()

	config, err := LoadVaultConfigFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config from env: %v", err)
	}

	if config.Address != "https://vault.example.com" {
		t.Errorf("Expected address 'https://vault.example.com', got '%s'", config.Address)
	}
	if config.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", config.Token)
	}
	if config.TransitPath != "custom-transit" {
		t.Errorf("Expected transit path 'custom-transit', got '%s'", config.TransitPath)
	}
	if config.KeyName != "custom-key" {
		t.Errorf("Expected key name 'custom-key', got '%s'", config.KeyName)
	}
	if config.CacheEnabled {
		t.Errorf("Expected cache disabled, got enabled")
	}
	if config.TTL != 2*time.Hour {
		t.Errorf("Expected TTL 2h, got %v", config.TTL)
	}
}

func TestVaultConfigMissingToken(t *testing.T) {
	// Clear VAULT_TOKEN
	origToken := os.Getenv("VAULT_TOKEN")
	os.Setenv("VAULT_TOKEN", "")
	defer os.Setenv("VAULT_TOKEN", origToken)

	_, err := LoadVaultConfigFromEnv()
	if err == nil {
		t.Error("Expected error for missing VAULT_TOKEN, got nil")
	}
}

func TestDefaultVaultConfig(t *testing.T) {
	config := DefaultVaultConfig()

	if config.Address != "http://localhost:8200" {
		t.Errorf("Expected default address 'http://localhost:8200', got '%s'", config.Address)
	}
	if config.Token != "root-token" {
		t.Errorf("Expected default token 'root-token', got '%s'", config.Token)
	}
	if config.TransitPath != "transit" {
		t.Errorf("Expected default transit path 'transit', got '%s'", config.TransitPath)
	}
	if config.KeyName != "kafka-encryption" {
		t.Errorf("Expected default key name 'kafka-encryption', got '%s'", config.KeyName)
	}
}

func TestValidateVaultConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *VaultConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &VaultConfig{
				Address:     "http://localhost:8200",
				Token:       "test-token",
				TransitPath: "transit",
				KeyName:     "test-key",
				TTL:         time.Hour,
			},
			wantErr: false,
		},
		{
			name: "missing address",
			config: &VaultConfig{
				Token:       "test-token",
				TransitPath: "transit",
				KeyName:     "test-key",
				TTL:         time.Hour,
			},
			wantErr: true,
		},
		{
			name: "missing token",
			config: &VaultConfig{
				Address:     "http://localhost:8200",
				TransitPath: "transit",
				KeyName:     "test-key",
				TTL:         time.Hour,
			},
			wantErr: true,
		},
		{
			name: "missing transit path",
			config: &VaultConfig{
				Address: "http://localhost:8200",
				Token:   "test-token",
				KeyName: "test-key",
				TTL:     time.Hour,
			},
			wantErr: true,
		},
		{
			name: "missing key name",
			config: &VaultConfig{
				Address:     "http://localhost:8200",
				Token:       "test-token",
				TransitPath: "transit",
				TTL:         time.Hour,
			},
			wantErr: true,
		},
		{
			name: "negative TTL",
			config: &VaultConfig{
				Address:     "http://localhost:8200",
				Token:       "test-token",
				TransitPath: "transit",
				KeyName:     "test-key",
				TTL:         -time.Hour,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVaultConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVaultConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncryptedDataSerialization(t *testing.T) {
	originalData := &EncryptedData{
		Ciphertext:  "vault:v1:test-ciphertext",
		KeyVersion:  1,
		EncryptedAt: time.Now(),
		Algorithm:   "aes256-gcm",
		TransitPath: "transit",
	}

	// Serialize to JSON
	dataBytes, err := json.Marshal(originalData)
	if err != nil {
		t.Fatalf("Failed to marshal encrypted data: %v", err)
	}

	// Deserialize from JSON
	var deserializedData EncryptedData
	if err := json.Unmarshal(dataBytes, &deserializedData); err != nil {
		t.Fatalf("Failed to unmarshal encrypted data: %v", err)
	}

	// Verify data integrity
	if deserializedData.Ciphertext != originalData.Ciphertext {
		t.Errorf("Ciphertext mismatch: got %s, want %s", deserializedData.Ciphertext, originalData.Ciphertext)
	}
	if deserializedData.KeyVersion != originalData.KeyVersion {
		t.Errorf("Key version mismatch: got %d, want %d", deserializedData.KeyVersion, originalData.KeyVersion)
	}
	if deserializedData.Algorithm != originalData.Algorithm {
		t.Errorf("Algorithm mismatch: got %s, want %s", deserializedData.Algorithm, originalData.Algorithm)
	}
	if deserializedData.TransitPath != originalData.TransitPath {
		t.Errorf("Transit path mismatch: got %s, want %s", deserializedData.TransitPath, originalData.TransitPath)
	}
}

func TestKeyVersion(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour)

	key := &KeyVersion{
		Version:   1,
		Key:       []byte("test-key-32-bytes-long-!!"),
		CreatedAt: now,
		IsActive:  true,
		ExpiresAt: &future,
	}

	if key.Version != 1 {
		t.Errorf("Expected version 1, got %d", key.Version)
	}
	if !key.IsActive {
		t.Error("Expected key to be active")
	}
	if key.ExpiresAt == nil {
		t.Error("Expected expiration time to be set")
	}
	if !key.ExpiresAt.After(now) {
		t.Error("Expected expiration time to be in the future")
	}
}

// Mock VaultKeyManager for testing
type MockVaultKeyManager struct {
	keys           map[int]*KeyVersion
	currentVersion int
	encryptCount   int
	decryptCount   int
}

func NewMockVaultKeyManager() *MockVaultKeyManager {
	return &MockVaultKeyManager{
		keys: make(map[int]*KeyVersion),
	}
}

func (m *MockVaultKeyManager) GetCurrentKey() (*KeyVersion, error) {
	if m.currentVersion == 0 {
		return nil, fmt.Errorf("no current key")
	}
	return m.keys[m.currentVersion], nil
}

func (m *MockVaultKeyManager) GetKey(version int) (*KeyVersion, error) {
	key, exists := m.keys[version]
	if !exists {
		return nil, fmt.Errorf("key version %d not found", version)
	}
	return key, nil
}

func (m *MockVaultKeyManager) Encrypt(plaintext []byte) (*EncryptedData, error) {
	m.encryptCount++
	return &EncryptedData{
		Ciphertext:  fmt.Sprintf("mock-encrypted-%d", m.encryptCount),
		KeyVersion:  m.currentVersion,
		EncryptedAt: time.Now(),
		Algorithm:   "aes256-gcm",
		TransitPath: "transit",
	}, nil
}

func (m *MockVaultKeyManager) Decrypt(encryptedData *EncryptedData) ([]byte, error) {
	m.decryptCount++
	return []byte(fmt.Sprintf("decrypted-%d", m.decryptCount)), nil
}

func (m *MockVaultKeyManager) AddTestKey(version int, isActive bool) {
	key := make([]byte, 32) // 32 bytes for AES-256
	for i := range key {
		key[i] = byte(version + i) // Make each key version unique
	}

	kv := &KeyVersion{
		Version:   version,
		Key:       key,
		CreatedAt: time.Now(),
		IsActive:  isActive,
	}
	m.keys[version] = kv
	if isActive {
		m.currentVersion = version
	}
}

func TestVaultAwareAESGCM(t *testing.T) {
	// Create mock Vault manager
	mockVault := NewMockVaultKeyManager()
	mockVault.AddTestKey(1, true)

	// Create Vault-aware cipher
	cipher, err := NewVaultAwareAESGCM(mockVault)
	if err != nil {
		t.Fatalf("Failed to create Vault-aware cipher: %v", err)
	}

	// Test encryption with Vault
	plaintext := []byte("test message")
	encryptedData, err := cipher.EncryptWithVault(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt with Vault: %v", err)
	}

	if encryptedData.KeyVersion != 1 {
		t.Errorf("Expected key version 1, got %d", encryptedData.KeyVersion)
	}

	// Test decryption with Vault
	decrypted, err := cipher.DecryptWithVault(encryptedData)
	if err != nil {
		t.Fatalf("Failed to decrypt with Vault: %v", err)
	}

	if string(decrypted) != "decrypted-1" {
		t.Errorf("Expected 'decrypted-1', got '%s'", string(decrypted))
	}
}

func TestVaultAwareAESGCMStringEncryption(t *testing.T) {
	// Create mock Vault manager
	mockVault := NewMockVaultKeyManager()
	mockVault.AddTestKey(1, true)

	// Create Vault-aware cipher
	cipher, err := NewVaultAwareAESGCM(mockVault)
	if err != nil {
		t.Fatalf("Failed to create Vault-aware cipher: %v", err)
	}

	// Test string encryption
	plainText := "test secret message"
	encrypted, err := cipher.EncryptStringWithVault(plainText)
	if err != nil {
		t.Fatalf("Failed to encrypt string with Vault: %v", err)
	}

	// Verify it's base64 encoded
	if len(encrypted) == 0 {
		t.Error("Encrypted string is empty")
	}

	// Test string decryption
	decrypted, err := cipher.DecryptStringWithVault(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt string with Vault: %v", err)
	}

	if decrypted != "decrypted-1" {
		t.Errorf("Expected 'decrypted-1', got '%s'", decrypted)
	}
}

func TestGlobalVaultCipher(t *testing.T) {
	// Create mock Vault manager
	mockVault := NewMockVaultKeyManager()
	mockVault.AddTestKey(1, true)

	// Initialize global Vault cipher
	err := InitGlobalVaultCipher(mockVault)
	if err != nil {
		t.Fatalf("Failed to initialize global Vault cipher: %v", err)
	}

	// Test global cipher access
	globalCipher := GetGlobalVaultCipher()
	if globalCipher == nil {
		t.Error("Global Vault cipher is nil")
	}

	// Test encryption through global interface
	encrypted, err := encryptString("test message")
	if err != nil {
		t.Fatalf("Failed to encrypt through global interface: %v", err)
	}

	// Test decryption through global interface
	decrypted, err := decryptString(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt through global interface: %v", err)
	}

	if decrypted != "decrypted-1" {
		t.Errorf("Expected 'decrypted-1', got '%s'", decrypted)
	}
}
