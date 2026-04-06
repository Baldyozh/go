package crypto_vault

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
)

func ExampleAESGCM() {
	// Generate a new key (in production, you should store this securely)
	key, err := GenerateKey()
	if err != nil {
		log.Fatal("Failed to generate key:", err)
	}

	// Initialize the global cipher
	err = InitGlobalCipher(key)
	if err != nil {
		log.Fatal("Failed to initialize cipher:", err)
	}

	// Example 1: Simple string encryption
	plaintext := "This is a secret message"
	encrypted, err := encryptString(plaintext)
	if err != nil {
		log.Fatal("Failed to encrypt:", err)
	}

	fmt.Printf("Original: %s\n", plaintext)
	fmt.Printf("Encrypted: %s\n", encrypted)

	// Example 2: JSON field encryption
	jsonData := `{
		"user": {
			"name": "John Doe",
			"email": "john@example.com",
			"password": "secret123"
		},
		"metadata": {
			"api_key": "sk-1234567890",
			"token": "abc123xyz"
		}
	}`

	// Encrypt specific fields
	encryptedJSON, err := EncryptJSONFields([]byte(jsonData), []string{
		"user.password", // nested field
		"api_key",       // global field name
		"token",         // another global field name
	})
	if err != nil {
		log.Fatal("Failed to encrypt JSON:", err)
	}

	fmt.Printf("Original JSON: %s\n", jsonData)
	fmt.Printf("Encrypted JSON: %s\n", string(encryptedJSON))

	// Example 3: Using the cipher directly
	cipher, err := NewAESGCM(key)
	if err != nil {
		log.Fatal("Failed to create cipher:", err)
	}

	// Encrypt and decrypt
	message := []byte("Direct encryption test")
	encryptedBytes, err := cipher.Encrypt(message)
	if err != nil {
		log.Fatal("Failed to encrypt:", err)
	}

	decryptedBytes, err := cipher.Decrypt(encryptedBytes)
	if err != nil {
		log.Fatal("Failed to decrypt:", err)
	}

	fmt.Printf("Direct encryption - Original: %s\n", string(message))
	fmt.Printf("Direct encryption - Decrypted: %s\n", string(decryptedBytes))
}

func TestAESGCM(t *testing.T) {
	// Generate key and create cipher
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	cipher, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create cipher: %v", err)
	}

	// Test string encryption/decryption
	testCases := []string{
		"Hello, World!",
		"This is a longer message with more characters",
		"Special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("EncryptDecrypt_%s", tc), func(t *testing.T) {
			// Encrypt
			encrypted, err := cipher.EncryptString(tc)
			if err != nil {
				t.Fatalf("Failed to encrypt: %v", err)
			}

			// Decrypt
			decrypted, err := cipher.DecryptString(encrypted)
			if err != nil {
				t.Fatalf("Failed to decrypt: %v", err)
			}

			// Verify
			if decrypted != tc {
				t.Errorf("Decrypted message doesn't match original. Got: %s, Want: %s", decrypted, tc)
			}
		})
	}

	// Test JSON field encryption
	jsonData := map[string]interface{}{
		"username": "testuser",
		"password": "secret123",
		"email":    "test@example.com",
		"nested": map[string]interface{}{
			"token":  "abc123",
			"secret": "nested_secret",
		},
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Initialize global cipher for JSON tests
	err = InitGlobalCipher(key)
	if err != nil {
		t.Fatalf("Failed to initialize global cipher: %v", err)
	}

	// Encrypt specific fields
	encryptedJSON, err := EncryptJSONFields(jsonBytes, []string{
		"password",
		"nested.secret",
	})
	if err != nil {
		t.Fatalf("Failed to encrypt JSON fields: %v", err)
	}

	// Parse encrypted JSON
	var encryptedData map[string]interface{}
	err = json.Unmarshal(encryptedJSON, &encryptedData)
	if err != nil {
		t.Fatalf("Failed to unmarshal encrypted JSON: %v", err)
	}

	// Verify that password is encrypted (should be different from original)
	if encryptedData["password"] == "secret123" {
		t.Error("Password was not encrypted")
	}

	// Verify that username is not encrypted (should be same as original)
	if encryptedData["username"] != "testuser" {
		t.Error("Username was incorrectly encrypted")
	}

	// Verify nested field encryption
	if nested, ok := encryptedData["nested"].(map[string]interface{}); ok {
		if nested["secret"] == "nested_secret" {
			t.Error("Nested secret was not encrypted")
		}
		if nested["token"] != "abc123" {
			t.Error("Nested token was incorrectly encrypted")
		}
	}
}
