package main

import (
	"fmt"
	"log"
	"log-processor/internal/infrastructure/clickhouse"
	"os"
	"os/signal"
	"syscall"

	"log-processor/internal/infrastructure/config"
	"log-processor/internal/infrastructure/kafka"
	"log-processor/internal/infrastructure/vault"
	"log-processor/internal/usecase/process_logs"
)

func main() {
	// Load configuration
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.NewConfig(configPath)
	if err != nil {
		log.Printf("Warning: Could not load config: %v, using defaults", err)
		cfg = config.DefaultConfig()
	}
	// Initialize ClickHouse client
	clickhouseClient := clickhouse.New(clickhouse.DBConfig{
		Address:  cfg.ClickHouse.Address,
		Port:     cfg.ClickHouse.Port,
		Login:    cfg.ClickHouse.Login,
		Password: cfg.ClickHouse.Password,
	})

	clickhouseClient.Close()
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

	// Create encrypter
	encrypter := vault.NewEncrypter(vaultManager)

	// Create Kafka consumer
	consumer := kafka.NewConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.GroupID,
	)
	defer consumer.Close()

	// Create log processor (use case)
	processor := process_logs.NewLogProcessor(consumer, encrypter, cfg.Crypto.FieldsToEncrypt)

	// Setup graceful shutdown
	doneCh := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")

		close(doneCh)
	}()

	fmt.Printf("Starting Kafka log processor with Vault encryption...\n")
	fmt.Printf("Vault Address: %s\n", vaultConfig.Address)
	fmt.Printf("Topic: %s\n", cfg.Kafka.Topic)
	fmt.Printf("Fields to encrypt: %v\n", cfg.Crypto.FieldsToEncrypt)
	fmt.Printf("Press Ctrl+C to stop\n")

	// Process logs
	outputCh := processor.ProcessLogs(doneCh)

	for encryptedLog := range outputCh {
		fmt.Printf("Processed log: %s\n", encryptedLog.TimeStamp.Format("2006-01-02 15:04:05"))
	}

	fmt.Println("Processor stopped")
}

//docker exec -it go-vault-1 sh -c "VAULT_ADDR='http://127.0.0.1:8200' VAULT_TOKEN='my-root-token' vault secrets enable transit"
