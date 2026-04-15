package vault

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

	"log-processor/internal/usecase/process_logs"
)

const (
	KeySize   = 32 // 256 bits for AES-256
	NonceSize = 12 // 96 bits for GCM nonce
)

// AESGCM represents an AES-256 GCM cipher
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

// Encrypter implements LogEncrypter interface using Vault
type Encrypter struct {
	manager         *Manager
	directCipher    *DirectCipher
}

// DirectCipher uses Vault transit engine directly without local AES-GCM
type DirectCipher struct {
	manager *Manager
}

// NewDirectCipher creates a new Vault-direct cipher
func NewDirectCipher(manager *Manager) *DirectCipher {
	return &DirectCipher{
		manager: manager,
	}
}

// EncryptString encrypts a string using Vault transit engine
func (v *DirectCipher) EncryptString(plainText string) (string, error) {
	encryptedData, err := v.manager.Encrypt([]byte(plainText))
	if err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"encrypted":    true,
		"ciphertext":   encryptedData.Ciphertext,
		"key_version":  encryptedData.KeyVersion,
		"algorithm":    encryptedData.Algorithm,
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
func (v *DirectCipher) DecryptString(cipherText string) (string, error) {
	var encryptedData EncryptedData
	err := json.Unmarshal([]byte(cipherText), &encryptedData)
	if err != nil {
		return cipherText, nil
	}

	decrypted, err := v.manager.Decrypt(&encryptedData)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// NewEncrypter creates a new Vault-based encrypter
func NewEncrypter(manager *Manager) *Encrypter {
	return &Encrypter{
		manager:      manager,
		directCipher: NewDirectCipher(manager),
	}
}

// EncryptFields implements the LogEncrypter interface
func (e *Encrypter) EncryptFields(inputJSON []byte, paths []string) ([]byte, error) {
	var root interface{}
	if err := json.Unmarshal(inputJSON, &root); err != nil {
		return nil, err
	}

	// Split paths into positional (contain ".") and global names
	globalNames := map[string]struct{}{}
	positional := [][]string{}

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

	// Recursive traversal: node, positionalPaths (each as []string)
	if err := e.walkAndEncrypt(root, positional, globalNames); err != nil {
		return nil, err
	}

	out, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkAndEncrypt recursively traverses node and:
// - applies global names to any keys regardless of depth
// - tries to apply each positional path if it matches the current node
func (e *Encrypter) walkAndEncrypt(node interface{}, positional [][]string, globalNames map[string]struct{}) error {
	switch n := node.(type) {
	case map[string]interface{}:
		// First process all keys: global names and recursive traversal of values
		for k, v := range n {
			// Global name - if key matches and value is string, encrypt
			if _, ok := globalNames[k]; ok {
				if s, isStr := v.(string); isStr {
					enc, err := e.directCipher.EncryptString(s)
					if err != nil {
						return err
					}
					n[k] = enc
					continue
				}
			}
			// Recursive traversal for all values
			if err := e.walkAndEncrypt(v, positional, globalNames); err != nil {
				return err
			}
		}

		// Now process positional paths
		for _, segs := range positional {
			if len(segs) == 0 {
				continue
			}
			first := segs[0]
			if val, exists := n[first]; exists {
				if len(segs) == 1 {
					if s, isStr := val.(string); isStr {
						enc, err := e.directCipher.EncryptString(s)
						if err != nil {
							return err
						}
						n[first] = enc
						continue
					} else {
						return fmt.Errorf("value at path %q is not a string", strings.Join(segs, "."))
					}
				}
				if err := e.applyPositionalToNode(val, segs[1:]); err != nil {
					return err
				}
			}
		}
		return nil

	case []interface{}:
		for i := range n {
			if err := e.walkAndEncrypt(n[i], positional, globalNames); err != nil {
				return err
			}
		}
		return nil

	default:
		return nil
	}
}

// applyPositionalToNode applies segments to node
func (e *Encrypter) applyPositionalToNode(node interface{}, segments []string) error {
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
			if s, ok := val.(string); ok {
				enc, err := e.directCipher.EncryptString(s)
				if err != nil {
					return err
				}
				n[key] = enc
				return nil
			}
			return fmt.Errorf("value at %q is not a string", strings.Join(segments, "."))
		}
		return e.applyPositionalToNode(val, segments[1:])

	case []interface{}:
		seg := segments[0]

		if seg == "*" {
			for i := range n {
				if len(segments) == 1 {
					if s, ok := n[i].(string); ok {
						enc, err := e.directCipher.EncryptString(s)
						if err != nil {
							return err
						}
						n[i] = enc
					} else {
						return fmt.Errorf("array element %d is not a string", i)
					}
				} else {
					if err := e.applyPositionalToNode(n[i], segments[1:]); err != nil {
						return err
					}
				}
			}
			return nil
		}

		if idx, err := strconv.Atoi(seg); err == nil {
			if idx < 0 || idx >= len(n) {
				return fmt.Errorf("array index %d out of range", idx)
			}
			if len(segments) == 1 {
				if s, ok := n[idx].(string); ok {
					enc, err := e.directCipher.EncryptString(s)
					if err != nil {
						return err
					}
					n[idx] = enc
					return nil
				}
				return fmt.Errorf("array element %d is not a string", idx)
			}
			return e.applyPositionalToNode(n[idx], segments[1:])
		}

		for i := range n {
			if err := e.applyPositionalToNode(n[i], segments); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unexpected type %T while applying positional path", node)
	}
}

// Ensure Encrypter implements LogEncrypter
var _ process_logs.LogEncrypter = (*Encrypter)(nil)
