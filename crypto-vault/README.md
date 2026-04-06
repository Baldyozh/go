# AES-256 GCM Encryption Module with Vault Integration

This module provides AES-256 GCM encryption functionality with HashiCorp Vault integration for the Kafka log processor project.

## Features

- **AES-256 GCM encryption**: Industry-standard symmetric encryption with authentication
- **Vault integration**: Seamless key management with HashiCorp Vault
- **Automatic key rotation**: Configurable key rotation with version support
- **Multi-version decryption**: Decrypt data encrypted with previous key versions
- **JSON field encryption**: Selective encryption of specific fields in JSON data
- **Thread-safe**: Can be used concurrently in multiple goroutines
- **Key caching**: Performance optimization with configurable caching

## Quick Start

### 1. Vault Setup

First, set up HashiCorp Vault with the transit secrets engine:

```bash
# Enable transit secrets engine
vault secrets enable transit

# Create encryption key
vault write -f transit/keys/kafka-encryption
```

### 2. Environment Configuration

Set the required environment variables:

```bash
export VAULT_ADDR="http://localhost:8200"
export VAULT_TOKEN="your-vault-token"
export VAULT_TRANSIT_PATH="transit"
export VAULT_KEY_NAME="kafka-encryption"
```

### 3. Initialize Vault Integration

```go
import "your-project/crypto-vault"

// Load configuration from environment
config, err := crypto.LoadVaultConfigFromEnv()
if err != nil {
    log.Fatal("Failed to load Vault config:", err)
}

// Create Vault key manager
vaultManager, err := crypto.NewVaultKeyManager(config)
if err != nil {
    log.Fatal("Failed to create Vault key manager:", err)
}
defer vaultManager.Close()

// Initialize global Vault cipher
err = crypto.InitGlobalVaultCipher(vaultManager)
if err != nil {
    log.Fatal("Failed to initialize Vault cipher:", err)
}
```

### 4. Encrypt JSON Fields

```go
jsonData := `{
    "user": {
        "username": "john_doe",
        "email": "john@example.com",
        "password": "secret123"
    },
    "api_key": "sk-production-key"
}`

// Encrypt sensitive fields
encryptedFields := []string{
    "user.password",  // Nested field
    "api_key",        // Global field name
}

encryptedJSON, err := crypto.EncryptJSONFields(jsonData, encryptedFields)
if err != nil {
    log.Fatal("Failed to encrypt:", err)
}

fmt.Println("Encrypted:", string(encryptedJSON))
```

### 5. Direct Vault Encryption

```go
// Get the global Vault cipher
cipher := crypto.GetGlobalVaultCipher()

// Encrypt using Vault transit engine
encryptedData, err := cipher.EncryptWithVault([]byte("secret message"))
if err != nil {
    log.Fatal("Failed to encrypt:", err)
}

// Decrypt using Vault transit engine
decrypted, err := cipher.DecryptWithVault(encryptedData)
if err != nil {
    log.Fatal("Failed to decrypt:", err)
}

fmt.Println("Decrypted:", string(decrypted))
```

## Vault Integration

### Key Rotation

The module supports automatic key rotation:

```go
// Manual key rotation
err := vaultManager.RotateKey()
if err != nil {
    log.Printf("Key rotation failed: %v", err)
}

// Automatic rotation (configured in Vault)
// Keys are automatically rotated every 24 hours by default
```

### Multi-Version Support

Data encrypted with previous key versions can still be decrypted:

```go
// Get key information
keyInfo := vaultManager.GetKeyInfo()
for version, key := range keyInfo {
    status := "inactive"
    if key.IsActive {
        status = "active"
    }
    fmt.Printf("Version %d: %s\n", version, status)
}

// Decrypt data regardless of which key version was used
decrypted, err := cipher.DecryptWithVault(encryptedData)
```

### Configuration Options

```go
config := &crypto.VaultConfig{
    Address:      "http://localhost:8200",
    Token:        "vault-token",
    TransitPath:  "transit",
    KeyName:      "kafka-encryption",
    CacheEnabled: true,
    TTL:          time.Hour,
}
```

## API Reference

### Vault Management

#### `NewVaultKeyManager(config *VaultConfig) (*VaultKeyManager, error)`
Creates a new Vault key manager with the provided configuration.

#### `LoadVaultConfigFromEnv() (*VaultConfig, error)`
Loads Vault configuration from environment variables.

#### `DefaultVaultConfig() *VaultConfig`
Returns a default configuration for development.

#### `ValidateVaultConfig(config *VaultConfig) error`
Validates the Vault configuration.

### VaultKeyManager Methods

#### `GetCurrentKey() (*KeyVersion, error)`
Returns the current active encryption key.

#### `GetKey(version int) (*KeyVersion, error)`
Returns a specific key version.

#### `Encrypt(plaintext []byte) (*EncryptedData, error)`
Encrypts data using Vault transit engine.

#### `Decrypt(encryptedData *EncryptedData) ([]byte, error)`
Decrypts data using Vault transit engine.

#### `RotateKey() error`
Manually rotates the encryption key.

#### `GetKeyInfo() map[int]*KeyVersion`
Returns information about all key versions.

### VaultAwareAESGCM Methods

#### `EncryptWithVault(plaintext []byte) (*EncryptedData, error)`
Encrypts data using Vault transit engine.

#### `DecryptWithVault(encryptedData *EncryptedData) ([]byte, error)`
Decrypts data using Vault transit engine.

#### `EncryptStringWithVault(plainText string) (string, error)`
Encrypts a string using Vault and returns base64-encoded result.

#### `DecryptStringWithVault(cipherText string) (string, error)`
Decrypts a Vault-encrypted string.

## Data Structures

### EncryptedData
```go
type EncryptedData struct {
    Ciphertext  string    `json:"ciphertext"`
    KeyVersion  int       `json:"key_version"`
    EncryptedAt time.Time `json:"encrypted_at"`
    Algorithm   string    `json:"algorithm"`
    TransitPath string    `json:"transit_path"`
}
```

### KeyVersion
```go
type KeyVersion struct {
    Version   int       `json:"version"`
    Key       []byte    `json:"key"`
    CreatedAt time.Time `json:"created_at"`
    IsActive  bool      `json:"is_active"`
    ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
```

## Security Notes

1. **Vault Security**: Ensure Vault is properly secured with proper authentication methods
2. **Token Management**: Use short-lived tokens and proper authentication methods (AppRole, Kubernetes, etc.)
3. **Network Security**: Use TLS for all Vault communications
4. **Key Rotation**: Regular key rotation is essential for security
5. **Audit Logging**: Enable Vault audit logging for compliance
6. **Access Control**: Implement proper Vault policies for key access

## Production Deployment

### Environment Variables
```bash
# Required
VAULT_ADDR="https://vault.example.com"
VAULT_TOKEN="vault-token"

# Optional
VAULT_TRANSIT_PATH="transit"
VAULT_KEY_NAME="kafka-encryption"
VAULT_CACHE_ENABLED="true"
VAULT_CACHE_TTL="1h"
```

### Docker Configuration
```yaml
version: '3.8'
services:
  kafka-processor:
    image: your-kafka-processor
    environment:
      - VAULT_ADDR=https://vault.example.com
      - VAULT_TOKEN=${VAULT_TOKEN}
      - VAULT_TRANSIT_PATH=transit
      - VAULT_KEY_NAME=kafka-encryption
    depends_on:
      - vault
```

### Kubernetes Configuration
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vault-secrets
type: Opaque
data:
  VAULT_TOKEN: <base64-encoded-token>
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kafka-processor
spec:
  template:
    spec:
      containers:
      - name: processor
        env:
        - name: VAULT_ADDR
          value: "https://vault.example.com"
        - name: VAULT_TOKEN
          valueFrom:
            secretKeyRef:
              name: vault-secrets
              key: VAULT_TOKEN
```

## Examples

### Kafka Log Processing with Vault
```go
func processLogEntry(logEntry []byte) ([]byte, error) {
    // Encrypt sensitive fields before sending to Kafka
    sensitiveFields := []string{
        "user.password",
        "user.email",
        "api_key",
        "credit_card",
    }
    
    return crypto.EncryptJSONFields(logEntry, sensitiveFields)
}
```

### Key Rotation Monitoring
```go
func monitorKeyRotation(vaultManager *crypto.VaultKeyManager) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    
    for range ticker.C {
        keyInfo := vaultManager.GetKeyInfo()
        for version, key := range keyInfo {
            if key.IsActive {
                log.Printf("Current key version: %d", version)
            }
        }
    }
}
```

## Testing

Run the test suite:

```bash
go test ./crypto-vault -v
```

The tests cover:
- Basic encryption/decryption operations
- JSON field encryption with various path patterns
- Vault configuration and validation
- Key rotation functionality
- Multi-version decryption
- Error handling for invalid inputs

## Dependencies

This module uses:
- Go standard library cryptographic packages
- HashiCorp Vault API client
- No external cryptographic dependencies

## Troubleshooting

### Common Issues

1. **Connection Failed**: Ensure Vault is accessible and network connectivity is working
2. **Authentication Failed**: Verify Vault token and permissions
3. **Key Not Found**: Ensure the transit key exists in Vault
4. **Permission Denied**: Check Vault policies for transit operations

### Debug Mode

Enable debug logging:

```go
vaultManager, err := crypto.NewVaultKeyManager(config)
if err != nil {
    log.Printf("Vault connection error: %v", err)
}
```

### Health Checks

```go
// Check Vault connectivity
keyInfo := vaultManager.GetKeyInfo()
if len(keyInfo) == 0 {
    log.Println("No keys available - check Vault configuration")
}
```
