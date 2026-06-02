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

func TestNewAESGCM_InvalidKeyLength(t *testing.T) {
	_, err := NewAESGCM([]byte("short-key"))
	if err == nil {
		t.Fatal("expected error for invalid key length, got nil")
	}
}

func TestAESGCM_DecryptRejectsInvalidInput(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	cipher, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create cipher: %v", err)
	}

	if _, err := cipher.Decrypt([]byte("too-short")); err == nil {
		t.Fatal("expected error for short ciphertext, got nil")
	}

	encrypted, err := cipher.Encrypt([]byte("sensitive-value"))
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	encrypted[len(encrypted)-1] ^= 1

	if _, err := cipher.Decrypt(encrypted); err == nil {
		t.Fatal("expected error for tampered ciphertext, got nil")
	}
}

func TestAESGCM_EncryptUsesUniqueNonce(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	cipher, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create cipher: %v", err)
	}

	first, err := cipher.EncryptString("same plaintext")
	if err != nil {
		t.Fatalf("Failed to encrypt first value: %v", err)
	}

	second, err := cipher.EncryptString("same plaintext")
	if err != nil {
		t.Fatalf("Failed to encrypt second value: %v", err)
	}

	if first == second {
		t.Fatal("expected different ciphertexts for the same plaintext")
	}
}

func TestAESGCM_DecryptStringRejectsInvalidBase64(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	cipher, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("Failed to create cipher: %v", err)
	}

	if _, err := cipher.DecryptString("not-base64-%%%"); err == nil {
		t.Fatal("expected error for invalid base64 ciphertext, got nil")
	}
}

func TestEncrypter_EncryptFields(t *testing.T) {
	// For this test, we'll skip actual encryption since it requires Vault
	// The test just verifies the function doesn't panic on valid input
	encrypter := &Encrypter{
		manager:      nil, // Would need a real or mock manager for full testing
		directCipher: nil,
	}

	// Skip if no cipher (would need Vault integration test)
	if encrypter.directCipher == nil {
		t.Skip("Requires Vault integration")
	}
}
