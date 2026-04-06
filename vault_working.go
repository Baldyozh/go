/*
package main


import (
	"encoding/json"
	"fmt"
	"log"
	"log-processor/config"
	"log-processor/consumer"
	crypto "log-processor/crypto-vault"
	"log-processor/entities"

	"github.com/segmentio/kafka-go"
)

func generator(doneCh chan struct{}, logReader *consumer.KafkaConsumer) <-chan entities.Log {
	logCh := make(chan entities.Log)
	go func() {
		for {
			select {
			case <-doneCh:
				return
			default:
				log, err := logReader.ReadLog()
				if err != nil {
					fmt.Printf("Ошибка при чтении лога: %v\n", err)
					continue
				}
				logCh <- log
			}
		}
	}()
	return logCh
}

func encryptJson(doneCh chan struct{}, logCh <-chan entities.Log, vaultManager *crypto.VaultKeyManager) <-chan entities.Log {
	outputCh := make(chan entities.Log)
	go func() {
		for {
			select {
			case log := <-logCh:
				fmt.Println("=== Encrypt Json ===")
				
				// Используем Vault напрямую для шифрования JSON
				encryptedData, err := vaultManager.Encrypt([]byte(log.LogBody))
				if err != nil {
					fmt.Printf("Error encrypting log: %v\n", err)
					continue
				}
				
				// Создаем новый лог с зашифрованным телом
				encryptedLog := log
				
				// Сериализуем зашифрованные данные в JSON
				encryptedBytes, err := json.Marshal(encryptedData)
				if err != nil {
					fmt.Printf("Error marshaling encrypted data: %v\n", err)
					continue
				}
				
				encryptedLog.LogBody = string(encryptedBytes)
				outputCh <- encryptedLog
				
			case <-doneCh:
				return
			}
		}
	}()
	return outputCh
}

func main() {
	// Настройка Vault
	vaultConfig := &crypto.VaultConfig{
		Address:      "http://localhost:8200",
		Token:        "my-root-token",
		TransitPath:  "transit",
		KeyName:      "kafka-encryption",
		CacheEnabled: true,
		TTL:          3600, // 1 час
	}

	// Создание Vault менеджера
	vaultManager, err := crypto.NewVaultKeyManager(vaultConfig)
	if err != nil {
		log.Fatalf("Failed to create Vault key manager: %v", err)
	}
	defer vaultManager.Close()

	fmt.Printf("=== Vault Integration Working ===\n")
	fmt.Printf("Vault Address: %s\n", vaultConfig.Address)
	fmt.Printf("Transit Path: %s\n", vaultConfig.TransitPath)
	fmt.Printf("Key Name: %s\n", vaultConfig.KeyName)

	// Показать информацию о ключах
	keyInfo := vaultManager.GetKeyInfo()
	fmt.Printf("Loaded %d key versions\n", len(keyInfo))
	for version, key := range keyInfo {
		status := "inactive"
		if key.IsActive {
			status = "active"
		}
		fmt.Printf("  Version %d: %s\n", version, status)
	}

	// Тест шифрования/дешифрования
	fmt.Printf("\n=== Testing Vault Encryption ===\n")
	testMessage := `{"user": {"username": "john", "password": "secret123"}, "api_key": "sk-production-key"}`
	fmt.Printf("Original JSON: %s\n", testMessage)

	encryptedData, err := vaultManager.Encrypt([]byte(testMessage))
	if err != nil {
		log.Fatalf("Failed to encrypt test message: %v", err)
	}

	fmt.Printf("Encrypted: %s\n", encryptedData.Ciphertext)
	fmt.Printf("Key Version: %d\n", encryptedData.KeyVersion)

	decrypted, err := vaultManager.Decrypt(encryptedData)
	if err != nil {
		log.Fatalf("Failed to decrypt test message: %v", err)
	}

	fmt.Printf("Decrypted: %s\n", string(decrypted))

	// Демонстрация ротации ключей
	fmt.Printf("\n=== Testing Key Rotation ===\n")
	fmt.Printf("Note: Manual rotation requires Vault permissions\n")
	fmt.Printf("In production, keys rotate automatically every 24 hours\n")
	
	// Показываем информацию о текущих ключах
	keyInfo = vaultManager.GetKeyInfo()
	fmt.Printf("Current key versions: %d\n", len(keyInfo))
	for version, key := range keyInfo {
		status := "inactive"
		if key.IsActive {
			status = "active"
		}
		fmt.Printf("  Version %d: %s\n", version, status)
	}

	// Тест шифрования с текущим ключом
	testMessage2 := `{"user": {"username": "alice", "password": "newpass456"}, "session_token": "sess-abc123"}`
	encryptedData2, err := vaultManager.Encrypt([]byte(testMessage2))
	if err != nil {
		log.Fatalf("Failed to encrypt with new key: %v", err)
	}

	fmt.Printf("New message encrypted with version: %d\n", encryptedData2.KeyVersion)

	// Проверяем что старое сообщение все еще дешифруется
	_, err = vaultManager.Decrypt(encryptedData)
	if err != nil {
		log.Fatalf("Failed to decrypt old message: %v", err)
	}
	fmt.Printf("Old message still decryptable: ✅\n")

	// Проверяем что новое сообщение дешифруется
	_, err = vaultManager.Decrypt(encryptedData2)
	if err != nil {
		log.Fatalf("Failed to decrypt new message: %v", err)
	}
	fmt.Printf("New message decryptable: ✅\n")

	// Загрузка конфигурации Kafka
	fmt.Printf("\n=== Kafka Integration ===\n")
	cfg, err := config.NewConfig("config/config.yaml")
	if err != nil {
		log.Printf("Warning: Could not load Kafka config: %v", err)
		fmt.Printf("Skipping Kafka integration demo\n")
		return
	}
	
	consumerConfig := kafka.ReaderConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topic,
		GroupID: cfg.Kafka.GroupID,
	}
	reader := kafka.NewReader(consumerConfig)
	logReader := consumer.NewKafkaConsumer(*reader)
	
	doneCh := make(chan struct{})
	logCh := generator(doneCh, logReader)
	outputCh := encryptJson(doneCh, logCh, vaultManager)
	
	fmt.Printf("Starting Kafka log processing with Vault encryption...\n")
	fmt.Printf("Press Ctrl+C to stop\n")
	
	// Обработка зашифрованных логов
	for encryptedLog := range outputCh {
		fmt.Printf("Processed encrypted log: %+v\n", encryptedLog)
	}

	defer reader.Close()
}

 */
