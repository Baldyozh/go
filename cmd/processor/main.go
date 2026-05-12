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
