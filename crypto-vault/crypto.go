package crypto_vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	KeySize   = 32 // 256 bits for AES-256
	NonceSize = 12 // 96 bits for GCM nonce
)

type AESGCM struct {
	key []byte
	gcm cipher.AEAD
}

// NewAESGCM creates a new AES-256 GCM cipher with the provided key
func NewAESGCM(key []byte) (*AESGCM, error) {
	if len(key) != KeySize {
		return nil, errors.New("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &AESGCM{
		key: key,
		gcm: gcm,
	}, nil
}

// GenerateKey generates a new random 32-byte key for AES-256
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256 GCM
func (a *AESGCM) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := a.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256 GCM
func (a *AESGCM) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:NonceSize]
	ciphertext = ciphertext[NonceSize:]

	plaintext, err := a.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64 encoded result
func (a *AESGCM) EncryptString(plainText string) (string, error) {
	ciphertext, err := a.Encrypt([]byte(plainText))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64 encoded string
func (a *AESGCM) DecryptString(cipherText string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	plaintext, err := a.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// VaultManager interface defines the contract for key management
type VaultManager interface {
	GetCurrentKey() (*KeyVersion, error)
	GetKey(version int) (*KeyVersion, error)
	Encrypt(plaintext []byte) (*EncryptedData, error)
	Decrypt(encryptedData *EncryptedData) ([]byte, error)
}

// VaultAwareAESGCM extends AESGCM with Vault integration
type VaultAwareAESGCM struct {
	*AESGCM
	vaultManager VaultManager
}

// VaultDirectCipher uses Vault transit engine directly without local AES-GCM
type VaultDirectCipher struct {
	vaultManager VaultManager
}

// NewVaultDirectCipher creates a new Vault-direct cipher
func NewVaultDirectCipher(vaultManager VaultManager) *VaultDirectCipher {
	return &VaultDirectCipher{
		vaultManager: vaultManager,
	}
}

// EncryptString encrypts a string using Vault transit engine
func (v *VaultDirectCipher) EncryptString(plainText string) (string, error) {
	encryptedData, err := v.vaultManager.Encrypt([]byte(plainText))
	if err != nil {
		return "", err
	}
	
	// Return JSON with metadata
	result := map[string]interface{}{
		"encrypted": true,
		"ciphertext": encryptedData.Ciphertext,
		"key_version": encryptedData.KeyVersion,
		"algorithm": encryptedData.Algorithm,
		"transit_path": encryptedData.TransitPath,
		"encrypted_at": encryptedData.EncryptedAt,
	}
	
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	
	return string(resultBytes), nil
}

// DecryptString decrypts a string using Vault transit engine
func (v *VaultDirectCipher) DecryptString(cipherText string) (string, error) {
	// Check if this is encrypted data or plain text
	var encryptedData EncryptedData
	err := json.Unmarshal([]byte(cipherText), &encryptedData)
	if err != nil {
		// If not encrypted data, return as is
		return cipherText, nil
	}
	
	decrypted, err := v.vaultManager.Decrypt(&encryptedData)
	if err != nil {
		return "", err
	}
	
	return string(decrypted), nil
}

// NewVaultAwareAESGCM creates a new Vault-aware AES-GCM cipher
func NewVaultAwareAESGCM(vaultManager VaultManager) (*VaultAwareAESGCM, error) {
	currentKey, err := vaultManager.GetCurrentKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get current key from Vault: %w", err)
	}

	// For local AES-GCM, we need the actual key bytes
	// In production, you might want to use Vault's transit engine directly
	cipher, err := NewAESGCM(currentKey.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES-GCM cipher: %w", err)
	}

	return &VaultAwareAESGCM{
		AESGCM:       cipher,
		vaultManager: vaultManager,
	}, nil
}

// EncryptWithVault encrypts data using Vault transit engine
func (v *VaultAwareAESGCM) EncryptWithVault(plaintext []byte) (*EncryptedData, error) {
	return v.vaultManager.Encrypt(plaintext)
}

// DecryptWithVault decrypts data using Vault transit engine
func (v *VaultAwareAESGCM) DecryptWithVault(encryptedData *EncryptedData) ([]byte, error) {
	return v.vaultManager.Decrypt(encryptedData)
}

// EncryptStringWithVault encrypts a string using Vault
func (v *VaultAwareAESGCM) EncryptStringWithVault(plainText string) (string, error) {
	encryptedData, err := v.EncryptWithVault([]byte(plainText))
	if err != nil {
		return "", err
	}

	// Serialize encrypted data to JSON for storage
	dataBytes, err := json.Marshal(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	return base64.StdEncoding.EncodeToString(dataBytes), nil
}

// DecryptStringWithVault decrypts a Vault-encrypted string
func (v *VaultAwareAESGCM) DecryptStringWithVault(cipherText string) (string, error) {
	// Decode base64
	dataBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Unmarshal encrypted data
	var encryptedData EncryptedData
	if err := json.Unmarshal(dataBytes, &encryptedData); err != nil {
		return "", fmt.Errorf("failed to unmarshal encrypted data: %w", err)
	}

	// Decrypt using Vault
	plaintext, err := v.DecryptWithVault(&encryptedData)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Global cipher instances
var globalCipher *AESGCM
var globalVaultCipher *VaultAwareAESGCM
var globalVaultDirectCipher *VaultDirectCipher

// InitGlobalCipher initializes the global cipher with a key
func InitGlobalCipher(key []byte) error {
	cipher, err := NewAESGCM(key)
	if err != nil {
		return err
	}
	globalCipher = cipher
	return nil
}

// InitGlobalVaultCipher initializes the global Vault-aware cipher
func InitGlobalVaultCipher(vaultManager VaultManager) error {
	cipher, err := NewVaultAwareAESGCM(vaultManager)
	if err != nil {
		// If we can't create VaultAwareAESGCM, use VaultDirectCipher instead
		globalVaultDirectCipher = NewVaultDirectCipher(vaultManager)
		return nil
	}
	globalVaultCipher = cipher
	return nil
}

// InitGlobalVaultDirectCipher initializes the global Vault-direct cipher
func InitGlobalVaultDirectCipher(vaultManager VaultManager) {
	globalVaultDirectCipher = NewVaultDirectCipher(vaultManager)
}

// encryptString uses the global cipher to encrypt a string
func encryptString(plainText string) (string, error) {
	if globalVaultCipher != nil {
		return globalVaultCipher.EncryptStringWithVault(plainText)
	}
	
	if globalVaultDirectCipher != nil {
		return globalVaultDirectCipher.EncryptString(plainText)
	}

	if globalCipher == nil {
		return "", errors.New("cipher not initialized, call InitGlobalCipher or InitGlobalVaultCipher first")
	}
	return globalCipher.EncryptString(plainText)
}

// decryptString uses the global cipher to decrypt a string
func decryptString(cipherText string) (string, error) {
	if globalVaultCipher != nil {
		return globalVaultCipher.DecryptStringWithVault(cipherText)
	}
	
	if globalVaultDirectCipher != nil {
		return globalVaultDirectCipher.DecryptString(cipherText)
	}

	if globalCipher == nil {
		return "", errors.New("cipher not initialized, call InitGlobalCipher or InitGlobalVaultCipher first")
	}
	return globalCipher.DecryptString(cipherText)
}

// GetGlobalVaultCipher returns the global Vault-aware cipher
func GetGlobalVaultCipher() *VaultAwareAESGCM {
	return globalVaultCipher
}

// GetGlobalCipher returns the global AES-GCM cipher
func GetGlobalCipher() *AESGCM {
	return globalCipher
}

func EncryptJSONFields(inputJSON []byte, paths []string) ([]byte, error) {
	var root interface{}
	if err := json.Unmarshal(inputJSON, &root); err != nil {
		return nil, err
	}

	// Разделяем paths на позиционные (contain ".") и глобальные имена
	globalNames := map[string]struct{}{}
	positional := [][]string{} // slice of segments

	for _, p := range paths {
		if p == "" {
			continue
		}
		if strings.Contains(p, ".") {
			positional = append(positional, strings.Split(p, "."))
		} else {
			globalNames[p] = struct{}{}
		}
	}

	// Рекурсивный обход: node, positionalPaths (each as []string)
	if err := walkAndEncrypt(root, positional, globalNames); err != nil {
		return nil, err
	}

	out, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkAndEncrypt рекурсивно обходит node и:
// - применяет глобальные имена к любым ключам map независимо от глубины
// - пытается применить каждую позиционную path, если она соответствует текущему узлу
func walkAndEncrypt(node interface{}, positional [][]string, globalNames map[string]struct{}) error {
	switch n := node.(type) {
	case map[string]interface{}:
		// Сначала обработаем все ключи: глобальные имена и рекурсивный обход значений
		for k, v := range n {
			// Глобальное имя — если ключ совпадает и значение строка, зашифровать
			if _, ok := globalNames[k]; ok {
				if s, isStr := v.(string); isStr {
					enc, err := encryptString(s)
					if err != nil {
						return err
					}
					n[k] = enc
					// После шифрования значение стало строкой — но не нужно дальше обрабатывать этот узел для глобальных имён
					continue
				}
			}
			// Рекурсивный обход для всех значений (чтобы глобальное имя применилось глубже)
			if err := walkAndEncrypt(v, positional, globalNames); err != nil {
				return err
			}
		}

		// Теперь обработаем позиционные пути: для кажого path, если первый сегмент == some key, спустимся
		for _, segs := range positional {
			if len(segs) == 0 {
				continue
			}
			first := segs[0]
			// если первый сегмент не совпадает с любым ключом — пропускаем
			if val, exists := n[first]; exists {
				if len(segs) == 1 {
					// цель — зашифровать val (если строка)
					if s, isStr := val.(string); isStr {
						enc, err := encryptString(s)
						if err != nil {
							return err
						}
						n[first] = enc
						continue
					} else {
						return fmt.Errorf("value at path %q is not a string", strings.Join(segs, "."))
					}
				}
				// рекурсивно применять оставшиеся сегменты к val
				if err := applyPositionalToNode(val, segs[1:]); err != nil {
					return err
				}
			}
		}
		return nil

	case []interface{}:
		// Рекурсивно обходим элементы массива — глобальные имена внутри элементов будут обработаны
		for i := range n {
			if err := walkAndEncrypt(n[i], positional, globalNames); err != nil {
				return err
			}
		}

		// А также применяем позиционные пути, которые начинаются с индекса/ "*" — но такие пути обрабатываются
		// в applyPositionalToNode при спуске из map.
		return nil

	default:
		// примитивы — ничего не делаем
		return nil
	}
}

// applyPositionalToNode применяется к node согласно сегментам пути, ожидая что текущий вызов
// уже соответствовал некоторому ключу (т.е. мы спустились).
func applyPositionalToNode(node interface{}, segments []string) error {
	if len(segments) == 0 {
		return errors.New("empty positional segments")
	}

	switch n := node.(type) {
	case map[string]interface{}:
		key := segments[0]
		val, exists := n[key]
		if !exists {
			return fmt.Errorf("key %q not found", key)
		}
		if len(segments) == 1 {
			// цель — зашифровать val (если строка)
			if s, ok := val.(string); ok {
				enc, err := encryptString(s)
				if err != nil {
					return err
				}
				n[key] = enc
				return nil
			}
			return fmt.Errorf("value at %q is not a string", strings.Join(segments, "."))
		}
		return applyPositionalToNode(val, segments[1:])

	case []interface{}:
		seg := segments[0]

		// Если сегмент — "*" — применяем путь ко всем элементам массива.
		if seg == "*" {
			for i := range n {
				if len(segments) == 1 {
					// Если это последний сегмент — ожидаем, что элемент массива сам является строкой.
					if s, ok := n[i].(string); ok {
						enc, err := encryptString(s)
						if err != nil {
							return err
						}
						n[i] = enc
					} else {
						return fmt.Errorf("array element %d is not a string", i)
					}
				} else {
					// Иначе рекурсивно спускаемся по оставшимся сегментам.
					if err := applyPositionalToNode(n[i], segments[1:]); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// Попробуем интерпретировать сегмент как индекс массива (число).
		if idx, err := strconv.Atoi(seg); err == nil {
			// Если сегмент — число, применяем к конкретному элементу по индексу.
			if idx < 0 || idx >= len(n) {
				return fmt.Errorf("array index %d out of range", idx)
			}
			if len(segments) == 1 {
				// Последний сегмент — ожидаем строку в элементе по индексу.
				if s, ok := n[idx].(string); ok {
					enc, err := encryptString(s)
					if err != nil {
						return err
					}
					n[idx] = enc
					return nil
				}
				return fmt.Errorf("array element %d is not a string", idx)
			}
			// Рекурсивно спускаемся по оставшимся сегментам в выбранный элемент.
			return applyPositionalToNode(n[idx], segments[1:])
		}

		// Если сегмент не "*" и не число, это означает, что путь вроде "companies.email",
		// где "companies" — массив объектов, а следующий сегмент — ключ внутри каждого элемента.
		// В этом случае применяем те же сегменты к каждому элементу массива (не потребляя сегмент),
		// потому что текущий сегмент относится к полю внутри элементов.
		for i := range n {
			if err := applyPositionalToNode(n[i], segments); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unexpected type %T while applying positional path", node)
	}
}
