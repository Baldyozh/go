package crypto_vault

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// VaultEnvConfig holds Vault configuration from environment variables
type VaultEnvConfig struct {
	Address      string
	Token        string
	TransitPath  string
	KeyName      string
	CacheEnabled bool
	TTL          time.Duration
}

// LoadVaultConfigFromEnv loads Vault configuration from environment variables
func LoadVaultConfigFromEnv() (*VaultConfig, error) {
	config := &VaultConfig{
		Address:      getEnvOrDefault("VAULT_ADDR", "http://localhost:8200"),
		Token:        getEnvOrDefault("VAULT_TOKEN", ""),
		TransitPath:  getEnvOrDefault("VAULT_TRANSIT_PATH", "transit"),
		KeyName:      getEnvOrDefault("VAULT_KEY_NAME", "kafka-encryption"),
		CacheEnabled: getBoolEnvOrDefault("VAULT_CACHE_ENABLED", true),
		TTL:          getDurationEnvOrDefault("VAULT_CACHE_TTL", KeyCacheTTL),
	}

	// Validate required fields
	if config.Token == "" {
		return nil, fmt.Errorf("VAULT_TOKEN environment variable is required")
	}

	return config, nil
}

// LoadVaultConfigFromFile loads Vault configuration from a YAML file
func LoadVaultConfigFromFile(configPath string) (*VaultConfig, error) {
	// Implementation would read YAML file
	// For now, return environment-based config
	return LoadVaultConfigFromEnv()
}

// DefaultVaultConfig returns a default Vault configuration for development
func DefaultVaultConfig() *VaultConfig {
	return &VaultConfig{
		Address:      "http://localhost:8200",
		Token:        "my-root-token", // Only for development!
		TransitPath:  "transit",
		KeyName:      "kafka-encryption",
		CacheEnabled: true,
		TTL:          KeyCacheTTL,
	}
}

// ProductionVaultConfig returns a production-ready Vault configuration
func ProductionVaultConfig() (*VaultConfig, error) {
	return LoadVaultConfigFromEnv()
}

// Helper functions for environment variable parsing
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnvOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getDurationEnvOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// ValidateVaultConfig validates the Vault configuration
func ValidateVaultConfig(config *VaultConfig) error {
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
