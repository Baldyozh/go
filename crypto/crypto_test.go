package crypto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
)

func decodeOrFail(t *testing.T, s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("failed to base64-decode %q: %v", s, err)
	}
	return string(b)
}

func TestEncryptJSONFields_GlobalNameAnywhere(t *testing.T) {
	input := []byte(`{
		"email":"root@example.com",
		"user": {"email":"alice@example.com", "profile": {"contact":{"email":"deep@example.com"}}},
		"companies": [{"email":"c1@example.com"}, {"meta": {"email":"c2@example.com"}}]
	}`)
	paths := []string{"email"} // глобально: все поля с именем "email"

	out, err := EncryptJSONFields(input, paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid output json: %v", err)
	}

	// root email
	rootEnc, ok := got["email"].(string)
	if !ok {
		t.Fatalf("root email missing or not string")
	}
	if decodeOrFail(t, rootEnc) != "root@example.com" {
		t.Fatalf("root email wrong after decode")
	}

	// user.email
	user := got["user"].(map[string]interface{})
	uEnc := user["email"].(string)
	if decodeOrFail(t, uEnc) != "alice@example.com" {
		t.Fatalf("user.email wrong after decode")
	}

	// deep email
	profile := user["profile"].(map[string]interface{})
	contact := profile["contact"].(map[string]interface{})
	deepEnc := contact["email"].(string)
	if decodeOrFail(t, deepEnc) != "deep@example.com" {
		t.Fatalf("deep email wrong after decode")
	}

	// companies array elements
	companies := got["companies"].([]interface{})
	c0 := companies[0].(map[string]interface{})
	if decodeOrFail(t, c0["email"].(string)) != "c1@example.com" {
		t.Fatalf("companies[0].email wrong")
	}
	c1 := companies[1].(map[string]interface{})
	meta := c1["meta"].(map[string]interface{})
	if decodeOrFail(t, meta["email"].(string)) != "c2@example.com" {
		t.Fatalf("companies[1].meta.email wrong")
	}
}

func TestEncryptJSONFields_PositionalPathOnly(t *testing.T) {
	input := []byte(`{
		"companies": [
			{"name":"Co1", "email":"co1@example.com"},
			{"name":"Co2", "email":"co2@example.com"}
		],
		"users": [{"email":"u1@example.com"}, {"email":"u2@example.com"}],
		"profile": {"email":"profile@example.com"}
	}`)
	paths := []string{"companies.email"} // должен шифровать только companies.*.email

	out, err := EncryptJSONFields(input, paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid output json: %v", err)
	}

	// companies.*.email must be encrypted
	companies := got["companies"].([]interface{})
	for i := range companies {
		obj := companies[i].(map[string]interface{})
		enc := obj["email"].(string)
		if decodeOrFail(t, enc) != fmt.Sprintf("co%d@example.com", i+1) {
			t.Fatalf("companies[%d].email wrong", i)
		}
	}

	// users emails and profile.email must remain unchanged (not encrypted)
	users := got["users"].([]interface{})
	u0 := users[0].(map[string]interface{})
	if u0["email"] != "u1@example.com" {
		t.Fatalf("users[0].email changed unexpectedly")
	}
	profile := got["profile"].(map[string]interface{})
	if profile["email"] != "profile@example.com" {
		t.Fatalf("profile.email changed unexpectedly")
	}
}

func TestEncryptJSONFields_MixedGlobalAndPositional(t *testing.T) {
	input := []byte(`{
		"users": [{"email":"u1@example.com", "secret":"s1"}, {"email":"u2@example.com", "secret":"s2"}],
		"admins": [{"email":"a1@example.com", "secret":"as1"}],
		"companies": [{"email":"c1@example.com", "secret":"cs1"}]
	}`)
	paths := []string{"secret", "users.email"} // secret — глобально, users.email — только в users

	out, err := EncryptJSONFields(input, paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid output json: %v", err)
	}

	// Все secret независимо от места зашифрованы
	checkSecret := func(arrKey string, idx int, want string) {
		arr := got[arrKey].([]interface{})
		obj := arr[idx].(map[string]interface{})
		enc := obj["secret"].(string)
		if decodeOrFail(t, enc) != want {
			t.Fatalf("%s[%d].secret wrong: got %q want %q", arrKey, idx, decodeOrFail(t, enc), want)
		}
	}

	checkSecret("users", 0, "s1")
	checkSecret("users", 1, "s2")
	checkSecret("admins", 0, "as1")
	checkSecret("companies", 0, "cs1")

	// users.email зашифрован, admins.email и companies.email остаются неизменными
	u0 := got["users"].([]interface{})[0].(map[string]interface{})
	if decodeOrFail(t, u0["email"].(string)) != "u1@example.com" {
		t.Fatalf("users[0].email wrong")
	}
	adminEmail := got["admins"].([]interface{})[0].(map[string]interface{})["email"]
	if adminEmail != "a1@example.com" {
		t.Fatalf("admins[0].email changed unexpectedly")
	}
	companyEmail := got["companies"].([]interface{})[0].(map[string]interface{})["email"]
	if companyEmail != "c1@example.com" {
		t.Fatalf("companies[0].email changed unexpectedly")
	}
}

func TestEncryptJSONFields_WildcardArrayAndIndex(t *testing.T) {
	input := []byte(`{
		"items":[{"secret":"s1","id":1},{"secret":"s2","id":2}],
		"payments":[{"card_number":"1111"},{"card_number":"2222"}]
	}`)
	paths := []string{"items.*.secret", "payments.0.card_number"}

	out, err := EncryptJSONFields(input, paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}

	items := root["items"].([]interface{})
	for i, it := range items {
		obj := it.(map[string]interface{})
		enc := obj["secret"].(string)
		if decodeOrFail(t, enc) != "s"+strconv.Itoa(i+1) {
			t.Fatalf("items[%d].secret decoded wrong", i)
		}
	}

	payments := root["payments"].([]interface{})
	p0 := payments[0].(map[string]interface{})
	if decodeOrFail(t, p0["card_number"].(string)) != "1111" {
		t.Fatalf("payments[0].card_number decoded wrong")
	}
	// payments[1] должен остаться неизменным
	p1 := payments[1].(map[string]interface{})
	if p1["card_number"] != "2222" {
		t.Fatalf("payments[1].card_number changed unexpectedly")
	}
}

func TestEncryptJSONFields_MissingPositionalPathReturnsError(t *testing.T) {
	input := []byte(`{"a":{}}`)
	paths := []string{"a.b.c"} // b отсутствует

	_, err := EncryptJSONFields(input, paths)
	if err == nil {
		t.Fatalf("expected error for missing positional path, got nil")
	}
}

func TestEncryptJSONFields_NonStringTargetReturnsError(t *testing.T) {
	input := []byte(`{"user":{"age":30, "email":"e@example.com"}}`)
	paths := []string{"user.age", "email"} // user.age не строка, email глобально

	_, err := EncryptJSONFields(input, paths)
	if err == nil {
		t.Fatalf("expected error for non-string positional target, got nil")
	}
}
