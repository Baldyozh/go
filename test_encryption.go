package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log-processor/config"
	crypto "log-processor/crypto-vault"
)

func main() {
	// Настройка Vault
	vaultConfig := &crypto.VaultConfig{
		Address:      "http://localhost:8200",
		Token:        "my-root-token",
		TransitPath:  "transit",
		KeyName:      "kafka-encryption",
		CacheEnabled: true,
		TTL:          3600,
	}

	vaultManager, err := crypto.NewVaultKeyManager(vaultConfig)
	if err != nil {
		log.Fatalf("Failed to create Vault key manager: %v", err)
	}
	defer vaultManager.Close()

	// Инициализируем глобальный Vault шифратор
	err = crypto.InitGlobalVaultCipher(vaultManager)
	if err != nil {
		log.Fatalf("Failed to initialize Vault cipher: %v", err)
	}

	// Тест JSON из Kafka
	testJSON := `{
		"email":"root@example.com",
		"user": {"email":"alice@example.com", "id":0, "profile": {"contact":{"email":"deep@example.com"}}},
		"companies": [{"email":"c1@example.com"}, {"sens": {"passport":"1233123233"}}]
	}`

	fmt.Printf("Original JSON:\n%s\n\n", testJSON)

	// Загружаем конфигурацию
	cfg, err := config.NewConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Fields to encrypt: %v\n\n", cfg.Crypto.FieldsToEncrypt)

	// Шифруем
	encrypted, err := crypto.EncryptJSONFields([]byte(testJSON), cfg.Crypto.FieldsToEncrypt)
	if err != nil {
		log.Fatalf("Failed to encrypt JSON: %v", err)
	}

	fmt.Printf("Encrypted JSON:\n%s\n\n", string(encrypted))

	// Парсим для проверки
	var result map[string]interface{}
	err = json.Unmarshal(encrypted, &result)
	if err != nil {
		log.Fatalf("Failed to parse encrypted JSON: %v", err)
	}

	// Проверяем companies
	if companies, ok := result["companies"].([]interface{}); ok {
		for i, company := range companies {
			if companyMap, ok := company.(map[string]interface{}); ok {
				fmt.Printf("Company %d:\n", i)
				for k, v := range companyMap {
					fmt.Printf("  %s: %v\n", k, v)
				}
			}
		}
	}
}
