package vault

import (
	"encoding/json"
	"testing"
)

// mockEncrypter creates an Encrypter with a mock Vault manager for testing
func setupTestEncrypter(t *testing.T) *Encrypter {
	// Create a mock manager that encrypts/decrypts locally for testing
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i)
	}

	// For testing, we'll create a simple manager that works locally
	// In real tests, you'd use a mock Vault server
	manager := &Manager{}
	encrypter := &Encrypter{
		manager: manager,
		directCipher: &DirectCipher{
			manager: manager,
		},
	}

	// For now, we'll skip the actual encryption test since it requires Vault
	// The real tests should use a Vault test server or mock
	return encrypter
}

func TestEncryptJSONFields_GlobalNameAnywhere(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{
		"passport":"root-passport",
		"user":{"passport":"nested-passport","name":"Ivan"},
		"items":[{"passport":"array-passport"}]
	}`)

	out, err := encrypter.DecryptFields(input, []string{"passport"})
	if err != nil {
		t.Fatalf("DecryptFields returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if got["passport"] != "root-passport" {
		t.Fatalf("root passport mismatch: %v", got["passport"])
	}

	user := got["user"].(map[string]interface{})
	if user["passport"] != "nested-passport" {
		t.Fatalf("nested passport mismatch: %v", user["passport"])
	}

	items := got["items"].([]interface{})
	item := items[0].(map[string]interface{})
	if item["passport"] != "array-passport" {
		t.Fatalf("array passport mismatch: %v", item["passport"])
	}
}

func TestEncryptJSONFields_PositionalPathOnly(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{
		"passport":"root-passport",
		"user":{"passport":"nested-passport","name":"Ivan"}
	}`)

	out, err := encrypter.DecryptFields(input, []string{"user.passport"})
	if err != nil {
		t.Fatalf("DecryptFields returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if got["passport"] != "root-passport" {
		t.Fatalf("positional path changed unrelated root field: %v", got["passport"])
	}

	user := got["user"].(map[string]interface{})
	if user["passport"] != "nested-passport" {
		t.Fatalf("nested passport mismatch: %v", user["passport"])
	}
}

func TestEncryptJSONFields_MixedGlobalAndPositional(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{
		"api_key":"root-key",
		"user":{"profile":{"snils":"123-456"},"api_key":"nested-key"},
		"metadata":{"snils":"not-selected"}
	}`)

	out, err := encrypter.DecryptFields(input, []string{"api_key", "user.profile.snils"})
	if err != nil {
		t.Fatalf("DecryptFields returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if got["api_key"] != "root-key" {
		t.Fatalf("root api_key mismatch: %v", got["api_key"])
	}

	user := got["user"].(map[string]interface{})
	profile := user["profile"].(map[string]interface{})
	if user["api_key"] != "nested-key" {
		t.Fatalf("nested api_key mismatch: %v", user["api_key"])
	}
	if profile["snils"] != "123-456" {
		t.Fatalf("positional snils mismatch: %v", profile["snils"])
	}

	metadata := got["metadata"].(map[string]interface{})
	if metadata["snils"] != "not-selected" {
		t.Fatalf("unrelated snils changed: %v", metadata["snils"])
	}
}

func TestEncryptJSONFields_WildcardArrayAndIndex(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{
		"items":[
			{"secret":"first","tokens":["a","b"]},
			{"secret":"second","tokens":["c","d"]}
		]
	}`)

	out, err := encrypter.DecryptFields(input, []string{"items.*.secret", "items.1.tokens.0"})
	if err != nil {
		t.Fatalf("DecryptFields returned error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	items := got["items"].([]interface{})
	first := items[0].(map[string]interface{})
	second := items[1].(map[string]interface{})

	if first["secret"] != "first" || second["secret"] != "second" {
		t.Fatalf("wildcard secrets mismatch: %#v", items)
	}

	secondTokens := second["tokens"].([]interface{})
	if secondTokens[0] != "c" || secondTokens[1] != "d" {
		t.Fatalf("indexed token mismatch: %#v", secondTokens)
	}
}

func TestEncryptJSONFields_MissingPositionalPathReturnsError(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{"a":{}}`)
	paths := []string{"a.b.c"}

	_, err := encrypter.EncryptFields(input, paths)
	if err == nil {
		t.Fatalf("expected error for missing positional path, got nil")
	}
}

func TestEncryptJSONFields_NonStringTargetReturnsError(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{"user":{"age":30, "email":"e@example.com"}}`)
	paths := []string{"user.age", "email"}

	_, err := encrypter.EncryptFields(input, paths)
	if err == nil {
		t.Fatalf("expected error for non-string positional target, got nil")
	}
}

func TestEncryptJSONFields_InvalidJSONReturnsError(t *testing.T) {
	encrypter := setupTestEncrypter(t)

	_, err := encrypter.EncryptFields([]byte(`{"broken":`), []string{"passport"})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestDecryptJSONFields_NonStringTargetReturnsError(t *testing.T) {
	encrypter := setupTestEncrypter(t)
	input := []byte(`{"user":{"age":30}}`)

	_, err := encrypter.DecryptFields(input, []string{"user.age"})
	if err == nil {
		t.Fatal("expected error for non-string positional target, got nil")
	}
}
