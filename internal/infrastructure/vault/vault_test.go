package vault

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDefaultVaultConfig(t *testing.T) {
	config := &Config{
		Address:     "http://localhost:8200",
		Token:       "root-token",
		TransitPath: "transit",
		KeyName:     "kafka-encryption",
	}

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
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
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
			config: &Config{
				Token:       "test-token",
				TransitPath: "transit",
				KeyName:     "test-key",
				TTL:         time.Hour,
			},
			wantErr: true,
		},
		{
			name: "missing token",
			config: &Config{
				Address:     "http://localhost:8200",
				TransitPath: "transit",
				KeyName:     "test-key",
				TTL:         time.Hour,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
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

	dataBytes, err := json.Marshal(originalData)
	if err != nil {
		t.Fatalf("Failed to marshal encrypted data: %v", err)
	}

	var deserializedData EncryptedData
	if err := json.Unmarshal(dataBytes, &deserializedData); err != nil {
		t.Fatalf("Failed to unmarshal encrypted data: %v", err)
	}

	if deserializedData.Ciphertext != originalData.Ciphertext {
		t.Errorf("Ciphertext mismatch: got %s, want %s", deserializedData.Ciphertext, originalData.Ciphertext)
	}
	if deserializedData.KeyVersion != originalData.KeyVersion {
		t.Errorf("Key version mismatch: got %d, want %d", deserializedData.KeyVersion, originalData.KeyVersion)
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
