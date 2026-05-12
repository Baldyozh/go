package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpdelivery "github.com/Baldyozh/log-processor/internal/delivery/http"
	"github.com/Baldyozh/log-processor/internal/delivery/http/handlers"
	"github.com/Baldyozh/log-processor/internal/infrastructure/auth"
	"github.com/Baldyozh/log-processor/internal/infrastructure/clickhouse"
	"github.com/Baldyozh/log-processor/internal/infrastructure/config"
	"github.com/Baldyozh/log-processor/internal/infrastructure/postgres"
	"github.com/Baldyozh/log-processor/internal/infrastructure/vault"
	"github.com/Baldyozh/log-processor/internal/usecase/manage_logs"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Load configuration
	configPath := "config/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to generate password hash: %v", err)
	}
	fmt.Println(string(hash))
	cfg, err := config.NewConfig(configPath)
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	// Initialize PostgreSQL connection
	postgresDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Password, cfg.Postgres.Database)
	postgresDB, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer postgresDB.Close()

	if err := postgresDB.Ping(); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	// Initialize auth repository
	authRepo := postgres.NewAuthRepository(postgresDB)

	// Initialize JWT service
	jwtService := auth.NewJWTService(cfg.API.JWTSecret, cfg.API.JWTIssuer, cfg.API.JWTExpiry)

	// Initialize ClickHouse client
	chClient := clickhouse.New(clickhouse.DBConfig{
		Address:  cfg.ClickHouse.Address,
		Port:     cfg.ClickHouse.Port,
		Login:    cfg.ClickHouse.Login,
		Password: cfg.ClickHouse.Password,
	})
	defer chClient.Close()

	// Initialize ClickHouse query repository
	chQueryRepo := clickhouse.NewQueryRepository(chClient.Conn())

	// Initialize Vault manager for decryption
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

	// Create decrypter (reuse existing encrypter interface for decryption)
	decrypter := vault.NewEncrypter(vaultManager)

	// Initialize log service
	logService := manage_logs.NewLogService(chQueryRepo, decrypter, authRepo, cfg.Crypto.FieldsToEncrypt)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authRepo, jwtService)
	logHandler := handlers.NewLogHandler(logService)

	// Initialize router
	router := httpdelivery.Router(authHandler, logHandler, jwtService, authRepo)

	// Setup graceful shutdown
	server := &http.Server{
		Addr:    ":" + cfg.API.Port,
		Handler: router,
	}

	doneCh := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		close(doneCh)
	}()

	fmt.Printf("Starting API server on port %s\n", cfg.API.Port)
	fmt.Printf("PostgreSQL: %s:%d/%s\n", cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Database)
	fmt.Printf("ClickHouse: %s:%d\n", cfg.ClickHouse.Address, cfg.ClickHouse.Port)
	fmt.Printf("Vault: %s\n", cfg.Vault.Address)
	fmt.Printf("Press Ctrl+C to stop\n")

	// Start server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	<-doneCh
	fmt.Println("Server stopped")
}
