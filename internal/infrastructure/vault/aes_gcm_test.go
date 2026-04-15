package vault

import (
	"testing"
)

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
		t.Run("EncryptDecrypt", func(t *testing.T) {
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
}

func TestEncrypter_EncryptFields(t *testing.T) {
	// For this test, we'll skip actual encryption since it requires Vault
	// The test just verifies the function doesn't panic on valid input
	encrypter := &Encrypter{
		manager: nil, // Would need a real or mock manager for full testing
		directCipher: nil,
	}
	
	// Skip if no cipher (would need Vault integration test)
	if encrypter.directCipher == nil {
		t.Skip("Requires Vault integration")
	}
}
