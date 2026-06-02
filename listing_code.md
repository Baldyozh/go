# Приложение А. Листинг программного кода

<style>
body { font-family: 'Times New Roman', serif; font-size: 14pt; line-height: 1.5; }
p { text-align: justify; text-indent: 1.25cm; margin: 0 0 6pt; }
pre, code { font-family: 'Courier New', monospace; font-size: 10pt; line-height: 1.0; }
pre { white-space: pre-wrap; break-inside: avoid; }
@media print { .listing { column-count: 2; column-gap: 1cm; } }
</style>

Настоящее приложение содержит фрагменты листинга программного кода серверного приложения обработки логов. Поясняющий текст к каждому фрагменту оформлен как основной текст документа: шрифт Times New Roman, размер 14 пт, выравнивание по ширине, абзацный отступ 1,25 см, междустрочный интервал 1,5. Листинги программного кода оформляются моноширинным шрифтом Courier New размером 10 пт с одинарным междустрочным интервалом. При печати допускается расположение листинга в две колонки.

В приложение включены основные модули системы: запуск фонового процессора, потоковая обработка сообщений, шифрование и расшифрование данных, управление ключами Vault, доступ к ClickHouse, прикладной сервис управления логами, HTTP-слой и интеграция с Kafka. Совокупный объем приведенного кода превышает десять страниц при заданных параметрах оформления.

## Листинг А.1 - Точка входа процессора логов

Фрагмент предназначен для запуска фонового обработчика логов. В нем выполняется загрузка конфигурации, инициализация клиентов ClickHouse, Vault и Kafka, выбор режима шифрования, создание пула обработчиков и корректное завершение работы по сигналам операционной системы.

Файл: `cmd/processor/main.go`.

<div class="listing">

```go
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Baldyozh/log-processor/internal/infrastructure/clickhouse"
	"github.com/Baldyozh/log-processor/internal/infrastructure/config"
	"github.com/Baldyozh/log-processor/internal/infrastructure/kafka"
	"github.com/Baldyozh/log-processor/internal/infrastructure/vault"
	"github.com/Baldyozh/log-processor/internal/usecase/process_logs"
)

func main() {
	// Load configuration
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	// Initialize ClickHouse client
	chClient := clickhouse.New(clickhouse.DBConfig{
		Address:  cfg.ClickHouse.Address,
		Port:     cfg.ClickHouse.Port,
		Login:    cfg.ClickHouse.Login,
		Password: cfg.ClickHouse.Password,
	})
	defer chClient.Close()

	// Initialize Vault manager
	vaultConfig := &vault.Config{
		Address:     cfg.Vault.Address,
		Token:       cfg.Vault.Token,
		TransitPath: cfg.Vault.TransitPath,
		KeyName:     cfg.Vault.KeyName,
	}

	vaultManager, err := vault.NewManager(vaultConfig)
	if err != nil {
		log.Fatalf("Failed to create Vault manager: %v", err)
	}
	defer vaultManager.Close()

	// Create encrypter based on configuration
	var encrypter process_logs.LogEncrypter
	switch cfg.Crypto.EncryptionType {
	case "local":
		localEncrypter := vault.NewLocalEncrypter(vaultManager, cfg.Crypto.LocalEncryption, cfg.Crypto.FieldsToEncrypt)
		if err := localEncrypter.Initialize(); err != nil {
			log.Fatalf("Failed to initialize local encrypter: %v", err)
		}
		encrypter = localEncrypter
		fmt.Printf("Using local encryption with key from Vault\n")
	case "vault":
		encrypter = vault.NewEncrypter(vaultManager)
		fmt.Printf("Using Vault transit encryption\n")
	default:
		log.Fatalf("Unknown encryption type: %s", cfg.Crypto.EncryptionType)
	}

	// Create Kafka consumer
	consumer := kafka.NewConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.GroupID,
	)
	defer consumer.Close()

	// Create log processor with worker pool and batch config
	processor := process_logs.NewLogProcessor(
		consumer,
		encrypter,
		chClient,
		cfg.Crypto.FieldsToEncrypt,
		process_logs.WorkerPoolConfig{
			WorkerCount:       cfg.WorkerPool.WorkerCount,
			ChannelBufferSize: cfg.WorkerPool.ChannelBufferSize,
		},
		process_logs.BatchConfig{
			Size:          cfg.Batch.Size,
			FlushInterval: cfg.Batch.FlushInterval,
		},
	)

	// Setup graceful shutdown
	doneCh := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		close(doneCh)
	}()

	fmt.Printf("Starting Kafka log processor...\n")
	fmt.Printf("Vault Address: %s\n", vaultConfig.Address)
	fmt.Printf("ClickHouse Address: %s:%d\n", cfg.ClickHouse.Address, cfg.ClickHouse.Port)
	fmt.Printf("Worker Pool: %d workers\n", cfg.WorkerPool.WorkerCount)
	fmt.Printf("Batch Size: %d, Flush Interval: %s\n", cfg.Batch.Size, cfg.Batch.FlushInterval)
	fmt.Printf("Topic: %s\n", cfg.Kafka.Topic)
	fmt.Printf("Encryption Type: %s\n", cfg.Crypto.EncryptionType)
	fmt.Printf("Fields to encrypt: %v\n", cfg.Crypto.FieldsToEncrypt)
	if cfg.Crypto.EncryptionType == "local" {
		fmt.Printf("Key Rotation Interval: %s\n", cfg.Crypto.LocalEncryption.KeyRotationInterval)
		fmt.Printf("Algorithm: %s\n", cfg.Crypto.LocalEncryption.Algorithm)
	}
	fmt.Printf("Press Ctrl+C to stop\n")

	// Process logs (blocks until shutdown)
	processor.ProcessLogs(doneCh)

	fmt.Println("Processor stopped")
}

//docker exec -it go-vault-1 sh -c "VAULT_ADDR='http://127.0.0.1:8200' VAULT_TOKEN='my-root-token' vault secrets enable transit"
```

</div>

## Листинг А.2 - Бизнес-логика потоковой обработки

Фрагмент содержит основной сценарий обработки сообщений: чтение логов из брокера, параллельное шифрование чувствительных полей и пакетную запись результата в аналитическое хранилище. Интерфейсы LogConsumer, LogEncrypter и LogStorage отделяют бизнес-логику от конкретных инфраструктурных реализаций.

Файл: `internal/usecase/process_logs/processor.go`.

<div class="listing">

```go
package process_logs

import (
	"log"
	"sync"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

// LogConsumer is the interface for reading logs from a message broker
type LogConsumer interface {
	ReadLog() (entities.Log, error)
	Close() error
}

// LogEncrypter is the interface for encrypting log data
type LogEncrypter interface {
	EncryptFields(data []byte, fields []string) ([]byte, error)
}

// LogStorage is the interface for storing encrypted logs
type LogStorage interface {
	InsertBatch(records []entities.ClickHouseLogRecord) error
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	WorkerCount       int
	ChannelBufferSize int
}

// BatchConfig holds configuration for batch processing
type BatchConfig struct {
	Size          int
	FlushInterval time.Duration
}

// LogProcessor handles the business logic of processing logs
type LogProcessor struct {
	consumer        LogConsumer
	encrypter       LogEncrypter
	storage         LogStorage
	sensitiveFields []string
	workerPoolCfg   WorkerPoolConfig
	batchCfg        BatchConfig
}

// NewLogProcessor creates a new LogProcessor
func NewLogProcessor(
	consumer LogConsumer,
	encrypter LogEncrypter,
	storage LogStorage,
	sensitiveFields []string,
	workerPoolCfg WorkerPoolConfig,
	batchCfg BatchConfig,
) *LogProcessor {
	return &LogProcessor{
		consumer:        consumer,
		encrypter:       encrypter,
		storage:         storage,
		sensitiveFields: sensitiveFields,
		workerPoolCfg:   workerPoolCfg,
		batchCfg:        batchCfg,
	}
}

// ProcessLogs reads logs from the consumer, encrypts sensitive fields, and stores them in ClickHouse
func (p *LogProcessor) ProcessLogs(doneCh <-chan struct{}) {
	// Channel for raw logs from Kafka
	rawLogCh := make(chan entities.Log, p.workerPoolCfg.ChannelBufferSize)

	// Channel for encrypted logs ready for batch insertion
	encryptedLogCh := make(chan entities.ClickHouseLogRecord, p.workerPoolCfg.ChannelBufferSize)

	// Start worker pool for concurrent encryption
	var wg sync.WaitGroup
	for i := 0; i < p.workerPoolCfg.WorkerCount; i++ {
		wg.Add(1)
		go p.encryptWorker(i, rawLogCh, encryptedLogCh, &wg)
	}

	// Start async batch writer
	writerDone := make(chan struct{})
	go p.batchWriter(encryptedLogCh, doneCh, writerDone)

	// Kafka reader loop
	defer close(rawLogCh)
	for {
		select {
		case <-doneCh:
			return
		default:
			logEntry, err := p.consumer.ReadLog()
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			rawLogCh <- logEntry
		}
	}
}

// encryptWorker is a worker that reads raw logs, encrypts sensitive fields, and sends to encrypted channel
func (p *LogProcessor) encryptWorker(id int, inCh <-chan entities.Log, outCh chan<- entities.ClickHouseLogRecord, wg *sync.WaitGroup) {
	defer wg.Done()

	for logEntry := range inCh {
		// Encrypt request body
		encryptedRequestBody, err := p.encrypter.EncryptFields([]byte(logEntry.LogBody.RequestBody), p.sensitiveFields)
		if err != nil {
			log.Printf("[Worker %d] Failed to encrypt request body: %v", id, err)
			continue
		}

		// Encrypt response body
		encryptedResponseBody, err := p.encrypter.EncryptFields([]byte(logEntry.LogBody.ResponseBody), p.sensitiveFields)
		if err != nil {
			log.Printf("[Worker %d] Failed to encrypt response body: %v", id, err)
			continue
		}

		// Update encrypted log record
		logEntry.LogBody.RequestBody = string(encryptedRequestBody)
		logEntry.LogBody.ResponseBody = string(encryptedResponseBody)

		// Send to batch writer
		outCh <- logEntry.LogBody
	}
}

// batchWriter collects encrypted logs and inserts them in batches to ClickHouse
func (p *LogProcessor) batchWriter(inCh <-chan entities.ClickHouseLogRecord, doneCh <-chan struct{}, writerDone chan<- struct{}) {
	defer close(writerDone)

	batch := make([]entities.ClickHouseLogRecord, 0, p.batchCfg.Size)
	flushTicker := time.NewTicker(p.batchCfg.FlushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-doneCh:
			// Flush remaining logs before exit
			if len(batch) > 0 {
				p.flushBatch(batch)
			}
			return
		case <-flushTicker.C:
			if len(batch) > 0 {
				p.flushBatch(batch)
				batch = batch[:0]
			}
		case record, ok := <-inCh:
			if !ok {
				// Channel closed, flush remaining
				if len(batch) > 0 {
					p.flushBatch(batch)
				}
				return
			}
			batch = append(batch, record)

			if len(batch) >= p.batchCfg.Size {
				p.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (p *LogProcessor) flushBatch(batch []entities.ClickHouseLogRecord) {
	if err := p.storage.InsertBatch(batch); err != nil {
		log.Printf("Failed to insert batch to ClickHouse: %v", err)
	} else {
		log.Printf("Successfully inserted %d logs to ClickHouse", len(batch))
	}
}
```

</div>

## Листинг А.3 - Шифрование и расшифрование полей JSON

Фрагмент реализует криптографический слой приложения. Код поддерживает локальный AES-GCM, прямое шифрование через Vault transit, обход вложенной JSON-структуры, обработку глобальных имен полей и позиционных путей, включая элементы массивов.

Файл: `internal/infrastructure/vault/crypto.go`.

<div class="listing">

```go
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

	"github.com/Baldyozh/log-processor/internal/usecase/process_logs"
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
	manager      *Manager
	directCipher *DirectCipher
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
	// Try to parse as JSON first
	var encryptedData EncryptedData
	err := json.Unmarshal([]byte(cipherText), &encryptedData)
	if err != nil {
		// If direct parsing fails, try to handle escaped JSON strings
		// Check if it looks like an escaped JSON string
		if strings.HasPrefix(cipherText, "{\"") && strings.HasSuffix(cipherText, "\"}") {
			// Try to unescape and parse again
			unescaped, unescapeErr := jsonUnescape(cipherText)
			if unescapeErr == nil {
				err = json.Unmarshal([]byte(unescaped), &encryptedData)
				if err == nil {
					// Successfully parsed unescaped JSON
					decrypted, decryptErr := v.manager.Decrypt(&encryptedData)
					if decryptErr != nil {
						return "", decryptErr
					}
					return string(decrypted), nil
				}
			}
		}
		// If all parsing attempts fail, return original string (not encrypted)
		return cipherText, nil
	}

	decrypted, err := v.manager.Decrypt(&encryptedData)
	if err != nil {
		return "", err
	}

	decryptedStr := string(decrypted)
	
	// Try to decode base64 if the result looks like base64
	if isBase64(decryptedStr) {
		if decoded, err := base64.StdEncoding.DecodeString(decryptedStr); err == nil {
			return string(decoded), nil
		}
	}

	return decryptedStr, nil
}

// isBase64 checks if a string looks like base64 encoded data
func isBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil && len(s) > 0 && len(s)%4 == 0
}

// jsonUnescape unescapes a JSON string that was stored as a string within JSON
func jsonUnescape(s string) (string, error) {
	var unescaped string
	err := json.Unmarshal([]byte("\""+s+"\""), &unescaped)
	return unescaped, err
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

// DecryptFields decrypts encrypted fields in JSON data
func (e *Encrypter) DecryptFields(inputJSON []byte, paths []string) ([]byte, error) {
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
	if err := e.walkAndDecrypt(root, positional, globalNames); err != nil {
		return nil, err
	}

	out, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkAndDecrypt recursively traverses node and decrypts matching fields
func (e *Encrypter) walkAndDecrypt(node interface{}, positional [][]string, globalNames map[string]struct{}) error {
	switch n := node.(type) {
	case map[string]interface{}:
		// First process all keys: global names and recursive traversal of values
		for k, v := range n {
			// Global name - if key matches and value is string, decrypt
			if _, ok := globalNames[k]; ok {
				if s, isStr := v.(string); isStr {
					dec, err := e.directCipher.DecryptString(s)
					if err != nil {
						return err
					}
					n[k] = dec
					continue
				}
			}
			// Recursive traversal for all values
			if err := e.walkAndDecrypt(v, positional, globalNames); err != nil {
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
						dec, err := e.directCipher.DecryptString(s)
						if err != nil {
							return err
						}
						n[first] = dec
						continue
					} else {
						return fmt.Errorf("value at path %q is not a string", strings.Join(segs, "."))
					}
				}
				if err := e.applyPositionalToNodeDecrypt(val, segs[1:]); err != nil {
					return err
				}
			}
		}
		return nil

	case []interface{}:
		for i := range n {
			if err := e.walkAndDecrypt(n[i], positional, globalNames); err != nil {
				return err
			}
		}
		return nil

	default:
		return nil
	}
}

// applyPositionalToNodeDecrypt applies segments to node for decryption
func (e *Encrypter) applyPositionalToNodeDecrypt(node interface{}, segments []string) error {
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
				dec, err := e.directCipher.DecryptString(s)
				if err != nil {
					return err
				}
				n[key] = dec
				return nil
			}
			return fmt.Errorf("value at %q is not a string", strings.Join(segments, "."))
		}
		return e.applyPositionalToNodeDecrypt(val, segments[1:])

	case []interface{}:
		seg := segments[0]

		if seg == "*" {
			for i := range n {
				if len(segments) == 1 {
					if s, ok := n[i].(string); ok {
						dec, err := e.directCipher.DecryptString(s)
						if err != nil {
							return err
						}
						n[i] = dec
					} else {
						return fmt.Errorf("array element %d is not a string", i)
					}
				} else {
					if err := e.applyPositionalToNodeDecrypt(n[i], segments[1:]); err != nil {
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
					dec, err := e.directCipher.DecryptString(s)
					if err != nil {
						return err
					}
					n[idx] = dec
					return nil
				}
				return fmt.Errorf("array element %d is not a string", idx)
			}
			return e.applyPositionalToNodeDecrypt(n[idx], segments[1:])
		}

		for i := range n {
			if err := e.applyPositionalToNodeDecrypt(n[i], segments); err != nil {
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
```

</div>

## Листинг А.4 - Управление ключами Vault

Фрагмент предназначен для работы с HashiCorp Vault: создания клиента, инициализации transit-ключа, загрузки версий ключей, ротации, а также выполнения операций шифрования и расшифрования через Vault API.

Файл: `internal/infrastructure/vault/manager.go`.

<div class="listing">

```go
package vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
)

const (
	// Vault paths and keys
	VaultTransitPath = "transit"
	VaultKeyName     = "kafka-encryption"

	// Key rotation settings
	KeyRotationInterval = 24 * time.Hour // Rotate keys every 24 hours
	KeyCacheTTL         = 1 * time.Hour  // Cache keys for 1 hour
)

// Config holds Vault connection configuration
type Config struct {
	Address      string
	Token        string
	TransitPath  string
	KeyName      string
	CacheEnabled bool
	TTL          time.Duration
}

// KeyVersion represents an encryption key version
type KeyVersion struct {
	Version   int        `json:"version"`
	Key       []byte     `json:"key"`
	CreatedAt time.Time  `json:"created_at"`
	IsActive  bool       `json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// EncryptedData contains encrypted data with metadata
type EncryptedData struct {
	Ciphertext  string    `json:"ciphertext"`
	KeyVersion  int       `json:"key_version"`
	EncryptedAt time.Time `json:"encrypted_at"`
	Algorithm   string    `json:"algorithm"`
	TransitPath string    `json:"transit_path"`
}

// Manager handles Vault key management and encryption operations
type Manager struct {
	Client         *api.Client
	config         *Config
	keys           map[int]*KeyVersion
	currentVersion int
	mutex          sync.RWMutex
	lastRotation   time.Time
}

// NewManager creates a new Vault key manager
func NewManager(config *Config) (*Manager, error) {
	if config.TransitPath == "" {
		config.TransitPath = VaultTransitPath
	}
	if config.KeyName == "" {
		config.KeyName = VaultKeyName
	}
	if config.TTL == 0 {
		config.TTL = KeyCacheTTL
	}

	// Create Vault client
	vaultConfig := api.DefaultConfig()
	vaultConfig.Address = config.Address

	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	client.SetToken(config.Token)

	km := &Manager{
		Client: client,
		config: config,
		keys:   make(map[int]*KeyVersion),
	}

	// Initialize key in Vault
	if err := km.initializeKey(); err != nil {
		return nil, fmt.Errorf("failed to initialize key: %w", err)
	}

	// Load existing keys
	if err := km.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	// Start key rotation routine
	go km.startKeyRotation()

	return km, nil
}

// initializeKey creates the encryption key in Vault if it doesn't exist
func (km *Manager) initializeKey() error {
	secret, err := km.Client.Logical().Read(fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName))
	if err != nil {
		return fmt.Errorf("failed to check key existence: %w", err)
	}

	if secret != nil {
		return nil
	}

	// Create new key
	data := map[string]interface{}{
		"type":        "aes256-gcm96",
		"auto_rotate": "24h",
	}

	_, err = km.Client.Logical().Write(
		fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName),
		data,
	)
	if err != nil {
		return fmt.Errorf("failed to create key: %w", err)
	}

	return nil
}

// loadKeys loads all key versions from Vault
func (km *Manager) loadKeys() error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	secret, err := km.Client.Logical().Read(fmt.Sprintf("%s/keys/%s", km.config.TransitPath, km.config.KeyName))
	if err != nil {
		return fmt.Errorf("failed to read key info: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return fmt.Errorf("key not found in Vault")
	}

	keysData, ok := secret.Data["keys"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid key data format")
	}

	km.keys = make(map[int]*KeyVersion)

	for versionStr := range keysData {
		version := 0
		if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
			continue
		}

		kv := &KeyVersion{
			Version:   version,
			CreatedAt: time.Now(),
			IsActive:  false,
		}

		if latestVersionNum, ok := secret.Data["latest_version"].(json.Number); ok {
			if v, err := latestVersionNum.Int64(); err == nil && int(v) == version {
				kv.IsActive = true
				km.currentVersion = version
			}
		}

		km.keys[version] = kv
	}

	if km.currentVersion == 0 && len(km.keys) > 0 {
		for version := range km.keys {
			if version > km.currentVersion {
				km.currentVersion = version
			}
		}
		km.keys[km.currentVersion].IsActive = true
	}

	return nil
}

// rotateKey performs key rotation
func (km *Manager) rotateKey() error {
	_, err := km.Client.Logical().Write(
		fmt.Sprintf("%s/keys/%s/rotate", km.config.TransitPath, km.config.KeyName),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}

	if err := km.loadKeys(); err != nil {
		return fmt.Errorf("failed to reload keys after rotation: %w", err)
	}

	km.lastRotation = time.Now()
	return nil
}

// startKeyRotation starts the automatic key rotation routine
func (km *Manager) startKeyRotation() {
	ticker := time.NewTicker(KeyRotationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := km.rotateKey(); err != nil {
				// In production, use proper logging
			}
		}
	}
}

// GetCurrentKey returns the current active encryption key
func (km *Manager) GetCurrentKey() (*KeyVersion, error) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	if km.currentVersion == 0 {
		return nil, fmt.Errorf("no active key version available")
	}

	key, exists := km.keys[km.currentVersion]
	if !exists {
		return nil, fmt.Errorf("current key version %d not found", km.currentVersion)
	}

	return key, nil
}

// GetKey returns a specific key version
func (km *Manager) GetKey(version int) (*KeyVersion, error) {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	key, exists := km.keys[version]
	if !exists {
		return nil, fmt.Errorf("key version %d not found", version)
	}

	return key, nil
}

// Encrypt encrypts data using Vault transit engine
func (km *Manager) Encrypt(plaintext []byte) (*EncryptedData, error) {
	currentKey, err := km.GetCurrentKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get current key: %w", err)
	}

	data := map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
	}

	path := fmt.Sprintf("%s/encrypt/%s", km.config.TransitPath, km.config.KeyName)
	secret, err := km.Client.Logical().Write(path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt with Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no encryption response from Vault")
	}

	ciphertext, ok := secret.Data["ciphertext"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid ciphertext response from Vault")
	}

	encryptedData := &EncryptedData{
		Ciphertext:  ciphertext,
		KeyVersion:  currentKey.Version,
		EncryptedAt: time.Now(),
		Algorithm:   "aes256-gcm96",
		TransitPath: km.config.TransitPath,
	}

	return encryptedData, nil
}

// Decrypt decrypts data using Vault transit engine
func (km *Manager) Decrypt(encryptedData *EncryptedData) ([]byte, error) {
	data := map[string]interface{}{
		"ciphertext": encryptedData.Ciphertext,
	}

	path := fmt.Sprintf("%s/decrypt/%s", km.config.TransitPath, km.config.KeyName)
	secret, err := km.Client.Logical().Write(path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt with Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no decryption response from Vault")
	}

	plaintext, ok := secret.Data["plaintext"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid plaintext response from Vault")
	}

	return []byte(plaintext), nil
}

// GetKeyInfo returns information about all key versions
func (km *Manager) GetKeyInfo() map[int]*KeyVersion {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	result := make(map[int]*KeyVersion)
	for version, key := range km.keys {
		keyCopy := *key
		result[version] = &keyCopy
	}

	return result
}

// Close cleans up resources
func (km *Manager) Close() error {
	return nil
}

// ValidateConfig validates the Vault configuration
func ValidateConfig(config *Config) error {
	if config.Address == "" {
		return fmt.Errorf("Vault address is required")
	}
	if config.Token == "" {
		return fmt.Errorf("Vault token is required")
	}
	if config.TransitPath == "" {
		return fmt.Errorf("Vault transit path is required")
	}
	if config.KeyName == "" {
		return fmt.Errorf("Vault key name is required")
	}
	if config.TTL <= 0 {
		return fmt.Errorf("Vault cache TTL must be positive")
	}
	return nil
}
```

</div>

## Листинг А.5 - Локальное шифрование с ключом из Vault

Фрагмент показывает альтернативный режим защиты данных. Ключ шифрования получается из Vault, но сама операция AES-GCM выполняется локально в приложении, что снижает количество сетевых обращений к Vault при высокой нагрузке.

Файл: `internal/infrastructure/vault/local_encrypter.go`.

<div class="listing">

```go
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/Baldyozh/log-processor/internal/infrastructure/config"
	"github.com/hashicorp/vault/api"
)

// LocalEncrypter performs local encryption using keys fetched from Vault
type LocalEncrypter struct {
	vaultClient     *api.Client
	vaultConfig     *config.VaultConfig
	localConfig     config.LocalEncryptionConfig
	sensitiveFields []string

	// Key management
	encryptionKey  []byte
	keyMutex       sync.RWMutex
	lastKeyRefresh time.Time

	// Performance metrics
	encryptCount    int64
	lastMetricsTime time.Time
}

// NewLocalEncrypter creates a new local encrypter
func NewLocalEncrypter(vaultManager *Manager, cfg config.LocalEncryptionConfig, sensitiveFields []string) *LocalEncrypter {
	return &LocalEncrypter{
		vaultClient:     vaultManager.Client,
		vaultConfig:     createVaultConfig(vaultManager.config),
		localConfig:     cfg,
		sensitiveFields: sensitiveFields,
		lastMetricsTime: time.Now(),
	}
}

// createVaultConfig converts internal vault.Config to config.VaultConfig
func createVaultConfig(cfg *Config) *config.VaultConfig {
	return &config.VaultConfig{
		Address:     cfg.Address,
		Token:       cfg.Token,
		TransitPath: cfg.TransitPath,
		KeyName:     cfg.KeyName,
	}
}

// Initialize fetches encryption key from Vault
func (le *LocalEncrypter) Initialize() error {
	log.Printf("Initializing local encrypter with algorithm: %s", le.localConfig.Algorithm)

	if err := le.refreshKey(); err != nil {
		return fmt.Errorf("failed to initialize encryption key: %w", err)
	}

	// Start key rotation goroutine
	if le.localConfig.KeyRotationInterval > 0 {
		go le.keyRotationWorker()
	}

	log.Printf("Local encrypter initialized successfully")
	return nil
}

// refreshKey fetches a new encryption key from Vault
func (le *LocalEncrypter) refreshKey() error {
	le.keyMutex.Lock()
	defer le.keyMutex.Unlock()

	log.Printf("Fetching encryption key from Vault...")

	// Request data encryption key from Vault
	secret, err := le.vaultClient.Logical().Write(
		fmt.Sprintf("%s/datakey/plaintext/%s", le.vaultConfig.TransitPath, le.vaultConfig.KeyName),
		map[string]interface{}{
			"bits": 256, // 256-bit key for AES-256
		},
	)

	if err != nil {
		return fmt.Errorf("failed to get data key from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return fmt.Errorf("no data returned from Vault")
	}

	// Extract plaintext key from response
	plaintextKey, ok := secret.Data["plaintext"].(string)
	if !ok {
		return fmt.Errorf("no plaintext key in Vault response")
	}

	// Decode base64 key
	key, err := base64.StdEncoding.DecodeString(plaintextKey)
	if err != nil {
		return fmt.Errorf("failed to decode encryption key: %w", err)
	}

	le.encryptionKey = key
	le.lastKeyRefresh = time.Now()

	log.Printf("Successfully fetched encryption key from Vault (key ID: %v)", secret.Data["key_id"])
	return nil
}

// keyRotationWorker periodically refreshes the encryption key
func (le *LocalEncrypter) keyRotationWorker() {
	ticker := time.NewTicker(le.localConfig.KeyRotationInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := le.refreshKey(); err != nil {
			log.Printf("Failed to rotate encryption key: %v", err)
		} else {
			log.Printf("Successfully rotated encryption key")
		}
	}
}

// EncryptFields encrypts specified fields in JSON data using local encryption
func (le *LocalEncrypter) EncryptFields(data []byte, fields []string) ([]byte, error) {
	le.keyMutex.RLock()
	currentKey := le.encryptionKey
	le.keyMutex.RUnlock()

	if currentKey == nil {
		return nil, fmt.Errorf("encryption key not initialized")
	}

	// Parse JSON data
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Encrypt specified fields
	for _, field := range fields {
		if value, exists := jsonData[field]; exists {
			encryptedValue, err := le.encryptValue(fmt.Sprintf("%v", value), currentKey)
			if err != nil {
				log.Printf("Failed to encrypt field %s: %v", field, err)
				continue
			}
			jsonData[field] = encryptedValue
		}
	}

	// Marshal back to JSON
	encryptedData, err := json.Marshal(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal encrypted JSON: %w", err)
	}

	// Update metrics
	le.encryptCount++
	le.logMetrics()

	return encryptedData, nil
}

// encryptValue encrypts a single value using AES-GCM
func (le *LocalEncrypter) encryptValue(value string, key []byte) (string, error) {
	switch le.localConfig.Algorithm {
	case "aes-gcm":
		return le.encryptAESGCM(value, key)
	default:
		return "", fmt.Errorf("unsupported encryption algorithm: %s", le.localConfig.Algorithm)
	}
}

// encryptAESGCM encrypts value using AES-GCM
func (le *LocalEncrypter) encryptAESGCM(plaintext string, key []byte) (string, error) {
	// Create cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to create nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Return base64 encoded result (nonce + ciphertext)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// logMetrics logs performance metrics periodically
func (le *LocalEncrypter) logMetrics() {
	now := time.Now()
	if now.Sub(le.lastMetricsTime) >= 30*time.Second {
		duration := now.Sub(le.lastMetricsTime).Seconds()
		rate := float64(le.encryptCount) / duration

		log.Printf("Local encryption metrics: %.2f encryptions/sec, total: %d", rate, le.encryptCount)

		le.encryptCount = 0
		le.lastMetricsTime = now
	}
}

// GetKeyInfo returns information about the current encryption key
func (le *LocalEncrypter) GetKeyInfo() map[string]interface{} {
	le.keyMutex.RLock()
	defer le.keyMutex.RUnlock()

	return map[string]interface{}{
		"algorithm":         le.localConfig.Algorithm,
		"last_key_refresh":  le.lastKeyRefresh,
		"rotation_interval": le.localConfig.KeyRotationInterval,
		"key_initialized":   le.encryptionKey != nil,
	}
}

// Close cleanup resources
func (le *LocalEncrypter) Close() error {
	log.Printf("Local encrypter closed")
	return nil
}
```

</div>

## Листинг А.6 - Репозиторий чтения логов из ClickHouse

Фрагмент реализует запросы к ClickHouse для получения одиночной записи, поиска по request_id, фильтрации по параметрам и вычисления статистических показателей. Репозиторий является инфраструктурной реализацией интерфейса чтения логов.

Файл: `internal/infrastructure/clickhouse/repository.go`.

<div class="listing">

```go
package clickhouse

import (
	"context"
	"fmt"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/usecase/manage_logs"
	"github.com/ClickHouse/clickhouse-go/v2"
)

// QueryRepository handles read operations from ClickHouse
type QueryRepository struct {
	conn clickhouse.Conn
}

// NewQueryRepository creates a new query repository
func NewQueryRepository(conn clickhouse.Conn) *QueryRepository {
	return &QueryRepository{conn: conn}
}

// GetLogByID retrieves a single log by ID
func (r *QueryRepository) GetLogByID(ctx context.Context, logID string) (*entities.ClickHouseLogRecord, error) {
	query := `
		SELECT log_id, timestamp, integration_id, request_id, http_method, endpoint,
		       request_body, response_body, duration_ms, status_code, success, error_message, user_id
		FROM default.logs
		WHERE log_id = $1
		LIMIT 1
	`

	var record entities.ClickHouseLogRecord
	err := r.conn.QueryRow(ctx, query, logID).Scan(
		&record.LogID,
		&record.Timestamp,
		&record.IntegrationID,
		&record.RequestID,
		&record.HTTPMethod,
		&record.Endpoint,
		&record.RequestBody,
		&record.ResponseBody,
		&record.DurationMs,
		&record.StatusCode,
		&record.Success,
		&record.ErrorMessage,
		&record.UserID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get log by ID: %w", err)
	}

	return &record, nil
}

// GetLogsByRequestID retrieves logs by request ID
func (r *QueryRepository) GetLogsByRequestID(ctx context.Context, requestID string) ([]entities.ClickHouseLogRecord, error) {
	query := `
		SELECT log_id, timestamp, integration_id, request_id, http_method, endpoint,
		       request_body, response_body, duration_ms, status_code, success, error_message, user_id
		FROM default.logs
		WHERE request_id = $1
		ORDER BY timestamp ASC
	`

	return r.queryLogs(ctx, query, requestID)
}

// QueryLogs retrieves logs based on filter criteria
func (r *QueryRepository) QueryLogs(ctx context.Context, filter manage_logs.LogFilter) ([]entities.ClickHouseLogRecord, error) {
	query := `
		SELECT log_id, timestamp, integration_id, request_id, http_method, endpoint,
		       request_body, response_body, duration_ms, status_code, success, error_message, user_id
		FROM default.logs
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIndex := 1

	if filter.RequestID != "" {
		query += fmt.Sprintf(" AND request_id = $%d", argIndex)
		args = append(args, filter.RequestID)
		argIndex++
	}

	if filter.IntegrationID != "" {
		query += fmt.Sprintf(" AND integration_id = $%d", argIndex)
		args = append(args, filter.IntegrationID)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, *filter.EndTime)
		argIndex++
	}

	if filter.StatusCode != nil {
		query += fmt.Sprintf(" AND status_code = $%d", argIndex)
		args = append(args, *filter.StatusCode)
		argIndex++
	}

	if filter.HTTPMethod != "" {
		query += fmt.Sprintf(" AND http_method = $%d", argIndex)
		args = append(args, filter.HTTPMethod)
		argIndex++
	}

	if filter.Endpoint != "" {
		query += fmt.Sprintf(" AND endpoint LIKE $%d", argIndex)
		args = append(args, "%"+filter.Endpoint+"%")
		argIndex++
	}

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, *filter.UserID)
		argIndex++
	}

	if filter.Success != nil {
		query += fmt.Sprintf(" AND success = $%d", argIndex)
		args = append(args, *filter.Success)
		argIndex++
	}

	query += " ORDER BY timestamp DESC"

	return r.queryLogs(ctx, query, args...)
}

// GetLogsStats returns statistics about logs
func (r *QueryRepository) GetLogsStats(ctx context.Context, filter manage_logs.LogFilter) (map[string]interface{}, error) {
	query := `
		SELECT
			count() as total_logs,
			countIf(success = true) as successful_logs,
			countIf(success = false) as failed_logs,
			avg(duration_ms) as avg_duration_ms,
			quantile(0.95)(duration_ms) as p95_duration_ms
		FROM default.logs
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIndex := 1

	if filter.IntegrationID != "" {
		query += fmt.Sprintf(" AND integration_id = $%d", argIndex)
		args = append(args, filter.IntegrationID)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, *filter.EndTime)
		argIndex++
	}

	var stats struct {
		TotalLogs      uint64  `ch:"total_logs"`
		SuccessfulLogs uint64  `ch:"successful_logs"`
		FailedLogs     uint64  `ch:"failed_logs"`
		AvgDurationMs  float64 `ch:"avg_duration_ms"`
		P95DurationMs  float64 `ch:"p95_duration_ms"`
	}

	err := r.conn.QueryRow(ctx, query, args...).Scan(
		&stats.TotalLogs,
		&stats.SuccessfulLogs,
		&stats.FailedLogs,
		&stats.AvgDurationMs,
		&stats.P95DurationMs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get logs stats: %w", err)
	}

	return map[string]interface{}{
		"total_logs":       stats.TotalLogs,
		"successful_logs":  stats.SuccessfulLogs,
		"failed_logs":      stats.FailedLogs,
		"avg_duration_ms":  stats.AvgDurationMs,
		"p95_duration_ms":  stats.P95DurationMs,
	}, nil
}

// queryLogs is a helper method to execute log queries
func (r *QueryRepository) queryLogs(ctx context.Context, query string, args ...interface{}) ([]entities.ClickHouseLogRecord, error) {
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var records []entities.ClickHouseLogRecord
	for rows.Next() {
		var record entities.ClickHouseLogRecord
		err := rows.Scan(
			&record.LogID,
			&record.Timestamp,
			&record.IntegrationID,
			&record.RequestID,
			&record.HTTPMethod,
			&record.Endpoint,
			&record.RequestBody,
			&record.ResponseBody,
			&record.DurationMs,
			&record.StatusCode,
			&record.Success,
			&record.ErrorMessage,
			&record.UserID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}
```

</div>

## Листинг А.7 - Сервис управления логами

Фрагмент описывает прикладной сервис для просмотра, поиска, экспорта и расшифрования логов. Перед выполнением операций проверяются права пользователя, а запросы на расшифрование дополнительно фиксируются для аудита.

Файл: `internal/usecase/manage_logs/service.go`.

<div class="listing">

```go
package manage_logs

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

// LogReader is the interface for reading logs from storage
type LogReader interface {
	GetLogByID(ctx context.Context, logID string) (*entities.ClickHouseLogRecord, error)
	GetLogsByRequestID(ctx context.Context, requestID string) ([]entities.ClickHouseLogRecord, error)
	QueryLogs(ctx context.Context, filter LogFilter) ([]entities.ClickHouseLogRecord, error)
	GetLogsStats(ctx context.Context, filter LogFilter) (map[string]interface{}, error)
}

// LogEncrypter is the interface for decrypting log data
type LogEncrypter interface {
	DecryptFields(data []byte, fields []string) ([]byte, error)
}

// AuthPermissionChecker is the interface for checking permissions
type AuthPermissionChecker interface {
	HasPermission(ctx context.Context, userID int, permissionName string) (bool, error)
	LogDecryptionRequest(ctx context.Context, userID int, logID string, reason string) error
}

// LogFilter represents filter parameters
type LogFilter struct {
	RequestID      string
	IntegrationID  string
	StartTime     *time.Time
	EndTime       *time.Time
	StatusCode    *uint16
	HTTPMethod    string
	Endpoint      string
	UserID        *uint32
	Success       *bool
}

// LogService handles business logic for log management
type LogService struct {
	logReader      LogReader
	encrypter      LogEncrypter
	authChecker    AuthPermissionChecker
	sensitiveFields []string
}

// NewLogService creates a new log service
func NewLogService(
	logReader LogReader,
	encrypter LogEncrypter,
	authChecker AuthPermissionChecker,
	sensitiveFields []string,
) *LogService {
	return &LogService{
		logReader:      logReader,
		encrypter:      encrypter,
		authChecker:    authChecker,
		sensitiveFields: sensitiveFields,
	}
}

// GetLogByID retrieves a log by ID with optional decryption
func (s *LogService) GetLogByID(ctx context.Context, userID int, logID string, decrypt bool, reason string) (*entities.ClickHouseLogRecord, error) {
	// Check read permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:read")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:read required")
	}

	// Get log
	log, err := s.logReader.GetLogByID(ctx, logID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	// Decrypt if requested and user has permission
	if decrypt {
		hasDecryptPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:decrypt")
		if err != nil {
			return nil, fmt.Errorf("failed to check decrypt permission: %w", err)
		}
		if !hasDecryptPermission {
			return nil, fmt.Errorf("permission denied: logs:decrypt required")
		}

		// Log decryption request
		if err := s.authChecker.LogDecryptionRequest(ctx, userID, logID, reason); err != nil {
			return nil, fmt.Errorf("failed to log decryption request: %w", err)
		}

		// Decrypt fields
		log, err = s.decryptLog(log)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt log: %w", err)
		}
	}

	return log, nil
}

// GetLogsByRequestID retrieves logs by request ID
func (s *LogService) GetLogsByRequestID(ctx context.Context, userID int, requestID string) ([]entities.ClickHouseLogRecord, error) {
	// Check search permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:search")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:search required")
	}

	logs, err := s.logReader.GetLogsByRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs by request ID: %w", err)
	}

	return logs, nil
}

// QueryLogs retrieves logs based on filter criteria
func (s *LogService) QueryLogs(ctx context.Context, userID int, filter LogFilter) ([]entities.ClickHouseLogRecord, error) {
	// Check filter permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:filter")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:filter required")
	}

	logs, err := s.logReader.QueryLogs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	return logs, nil
}

// GetLogsStats retrieves statistics about logs
func (s *LogService) GetLogsStats(ctx context.Context, userID int, filter LogFilter) (map[string]interface{}, error) {
	// Check stats permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:stats")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:stats required")
	}

	stats, err := s.logReader.GetLogsStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs stats: %w", err)
	}

	return stats, nil
}

// ExportLogsToCSV exports logs to CSV format
func (s *LogService) ExportLogsToCSV(ctx context.Context, userID int, filter LogFilter, writer io.Writer) error {
	// Check filter permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:filter")
	if err != nil {
		return fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return fmt.Errorf("permission denied: logs:filter required")
	}

	logs, err := s.logReader.QueryLogs(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	header := []string{
		"log_id", "timestamp", "integration_id", "request_id",
		"http_method", "endpoint", "request_body", "response_body",
		"duration_ms", "status_code", "success", "error_message", "user_id",
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write rows
	for _, log := range logs {
		row := []string{
			log.LogID,
			log.Timestamp.Format(time.RFC3339),
			log.IntegrationID,
			log.RequestID,
			log.HTTPMethod,
			log.Endpoint,
			log.RequestBody,
			log.ResponseBody,
			fmt.Sprintf("%d", log.DurationMs),
			fmt.Sprintf("%d", log.StatusCode),
			fmt.Sprintf("%t", log.Success),
			log.ErrorMessage,
			fmt.Sprintf("%v", log.UserID),
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// decryptLog decrypts sensitive fields in a log record
func (s *LogService) decryptLog(log *entities.ClickHouseLogRecord) (*entities.ClickHouseLogRecord, error) {
	// Decrypt request body
	decryptedRequestBody, err := s.encrypter.DecryptFields([]byte(log.RequestBody), s.sensitiveFields)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt request body: %w", err)
	}
	log.RequestBody = string(decryptedRequestBody)

	// Decrypt response body
	decryptedResponseBody, err := s.encrypter.DecryptFields([]byte(log.ResponseBody), s.sensitiveFields)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt response body: %w", err)
	}
	log.ResponseBody = string(decryptedResponseBody)

	return log, nil
}
```

</div>

## Листинг А.8 - HTTP-обработчики API логов

Фрагмент предназначен для обработки HTTP-запросов к журналам. Обработчики извлекают параметры маршрута и query string, проверяют наличие пользователя в контексте, вызывают прикладной сервис и возвращают JSON либо CSV-ответ.

Файл: `internal/delivery/http/handlers/log_handler.go`.

<div class="listing">

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Baldyozh/log-processor/internal/infrastructure/auth"
	"github.com/Baldyozh/log-processor/internal/usecase/manage_logs"
	"github.com/go-chi/chi/v5"
)

// LogHandler handles HTTP requests for log operations
type LogHandler struct {
	logService *manage_logs.LogService
}

// NewLogHandler creates a new log handler
func NewLogHandler(logService *manage_logs.LogService) *LogHandler {
	return &LogHandler{logService: logService}
}

// GetLogByID retrieves a log by ID
func (h *LogHandler) GetLogByID(w http.ResponseWriter, r *http.Request) {
	logID := chi.URLParam(r, "id")
	if logID == "" {
		http.Error(w, "log_id is required", http.StatusBadRequest)
		return
	}

	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Check if decryption is requested
	decrypt := r.URL.Query().Get("decrypt") == "true"
	reason := r.URL.Query().Get("reason")

	log, err := h.logService.GetLogByID(r.Context(), userID, logID, decrypt, reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, log)
}

// GetLogsByRequestID retrieves logs by request ID
func (h *LogHandler) GetLogsByRequestID(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "request_id")
	if requestID == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}

	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	logs, err := h.logService.GetLogsByRequestID(r.Context(), userID, requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// QueryLogs retrieves logs based on filter criteria
func (h *LogHandler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	filter, err := parseLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logs, err := h.logService.QueryLogs(r.Context(), userID, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// GetLogsStats retrieves statistics about logs
func (h *LogHandler) GetLogsStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	filter, err := parseLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stats, err := h.logService.GetLogsStats(r.Context(), userID, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// ExportLogsToCSV exports logs to CSV format
func (h *LogHandler) ExportLogsToCSV(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	filter, err := parseLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=logs.csv")

	if err := h.logService.ExportLogsToCSV(r.Context(), userID, filter, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// parseLogFilter parses filter parameters from request
func parseLogFilter(r *http.Request) (manage_logs.LogFilter, error) {
	query := r.URL.Query()
	filter := manage_logs.LogFilter{}

	filter.RequestID = query.Get("request_id")
	filter.IntegrationID = query.Get("integration_id")
	filter.HTTPMethod = query.Get("http_method")
	filter.Endpoint = query.Get("endpoint")

	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return filter, err
		}
		filter.StartTime = &startTime
	}

	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return filter, err
		}
		filter.EndTime = &endTime
	}

	if statusCodeStr := query.Get("status_code"); statusCodeStr != "" {
		statusCode, err := strconv.ParseUint(statusCodeStr, 10, 16)
		if err != nil {
			return filter, err
		}
		sc := uint16(statusCode)
		filter.StatusCode = &sc
	}

	if userIDStr := query.Get("user_id"); userIDStr != "" {
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			return filter, err
		}
		uid := uint32(userID)
		filter.UserID = &uid
	}

	if successStr := query.Get("success"); successStr != "" {
		success := successStr == "true"
		filter.Success = &success
	}

	return filter, nil
}

// respondJSON writes JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
```

</div>

## Листинг А.9 - Kafka consumer

Фрагмент реализует чтение сообщений из Kafka. Полученное сообщение десериализуется из JSON в доменную структуру лога и передается в сценарий обработки через интерфейс LogConsumer.

Файл: `internal/infrastructure/kafka/consumer.go`.

<div class="listing">

```go
package kafka

import (
	"context"
	"fmt"

	"encoding/json"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/usecase/process_logs"
	"github.com/segmentio/kafka-go"
)

// Consumer implements LogConsumer interface using kafka-go
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topic,
			GroupID: groupID,
		}),
	}
}

// ReadLog reads a log message from Kafka
func (c *Consumer) ReadLog() (entities.Log, error) {
	msg, err := c.reader.ReadMessage(context.Background())
	if err != nil {
		return entities.Log{}, fmt.Errorf("failed to read message: %w", err)
	}
	logBody := entities.ClickHouseLogRecord{}
	err = json.Unmarshal(msg.Value, &logBody)
	if err != nil {
		return entities.Log{}, fmt.Errorf("failed to unmarshal log body: %w", err)
	}
	return entities.Log{
		TimeStamp: msg.Time,
		LogBody:   logBody,
	}, nil
}

// Close closes the Kafka reader
func (c *Consumer) Close() error {
	if c.reader != nil {
		return c.reader.Close()
	}
	return nil
}

// Ensure Consumer implements LogConsumer
var _ process_logs.LogConsumer = (*Consumer)(nil)
```

</div>

## Листинг А.10 - Kafka producer

Фрагмент реализует отправку логов в Kafka. Структура лога сериализуется в JSON и записывается в выбранный топик через библиотеку kafka-go.

Файл: `internal/infrastructure/kafka/producer.go`.

<div class="listing">

```go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/usecase/add_logs"

	"github.com/segmentio/kafka-go"
)

// Producer implements LogProducer interface using kafka-go
type Producer struct {
	writer *kafka.Writer
	topic  string
}

// NewProducer creates a new Kafka producer
func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:  kafka.TCP(brokers...),
			Topic: topic,
		},
		topic: topic,
	}
}

// WriteLog writes a log message to Kafka
func (p *Producer) WriteLog(ctx context.Context, log entities.Log) error {
	messageValue, err := json.Marshal(log.LogBody)
	if err != nil {
		return fmt.Errorf("failed to marshal log body: %w", err)
	}
	message := kafka.Message{
		Value: messageValue,
	}

	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Close closes the Kafka writer
func (p *Producer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}

// Ensure Producer implements LogProducer
var _ add_logs.LogProducer = (*Producer)(nil)
```

</div>

## Листинг А.11 - Доменная модель записи ClickHouse

Фрагмент содержит доменную структуру записи лога. Теги ch и json связывают поля структуры с колонками ClickHouse и форматом обмена через JSON.

Файл: `internal/domain/entities/clickhouse_record.go`.

<div class="listing">

```go
package entities

import (
	"encoding/json"
	"time"
)

// ClickHouseLogRecord represents a log record ready for insertion into ClickHouse
type ClickHouseLogRecord struct {
	LogID         string    `ch:"log_id" json:"log_id"`
	Timestamp     time.Time `ch:"timestamp" json:"timestamp"`
	IntegrationID string    `ch:"integration_id" json:"integration_id"`
	RequestID     string    `ch:"request_id" json:"request_id"`
	HTTPMethod    string    `ch:"http_method" json:"http_method"`
	Endpoint      string    `ch:"endpoint" json:"endpoint"`
	RequestBody   string    `ch:"request_body" json:"request_body"`
	ResponseBody  string    `ch:"response_body" json:"response_body"`
	DurationMs    uint32    `ch:"duration_ms" json:"duration_ms"`
	StatusCode    uint16    `ch:"status_code" json:"status_code"`
	Success       bool      `ch:"success" json:"success"`
	ErrorMessage  string    `ch:"error_message" json:"error_message"`
	UserID        *uint32   `ch:"user_id" json:"user_id,omitempty"`
}

// ToJSON serializes the record to JSON
func (r *ClickHouseLogRecord) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ClickHouseLogRecordBatch represents a batch of log records for bulk insertion
type ClickHouseLogRecordBatch struct {
	Records []*ClickHouseLogRecord
}

// NewClickHouseLogRecordBatch creates a new batch
func NewClickHouseLogRecordBatch() *ClickHouseLogRecordBatch {
	return &ClickHouseLogRecordBatch{
		Records: make([]*ClickHouseLogRecord, 0),
	}
}

// Add adds a record to the batch
func (b *ClickHouseLogRecordBatch) Add(record *ClickHouseLogRecord) {
	b.Records = append(b.Records, record)
}

// Len returns the number of records in the batch
func (b *ClickHouseLogRecordBatch) Len() int {
	return len(b.Records)
}

// Clear clears all records in the batch
func (b *ClickHouseLogRecordBatch) Clear() {
	b.Records = b.Records[:0]
}
```

</div>
