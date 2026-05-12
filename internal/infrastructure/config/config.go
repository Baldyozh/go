package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Kafka      KafkaConfig      `yaml:"kafka"`
	Crypto     CryptoConfig     `yaml:"crypto"`
	Vault      VaultConfig      `yaml:"vault"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	WorkerPool WorkerPoolConfig `yaml:"worker_pool"`
	Batch      BatchConfig      `yaml:"batch"`
	Postgres   PostgresConfig   `yaml:"postgres"`
	API        APIConfig        `yaml:"api"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	GroupID string   `yaml:"group_id"`
}

// CryptoConfig holds encryption configuration
type CryptoConfig struct {
	FieldsToEncrypt   []string              `yaml:"fields_to_encrypt"`
	EncryptionType    string                `yaml:"encryption_type"`    // "vault" or "local"
	LocalEncryption   LocalEncryptionConfig `yaml:"local_encryption"`
}

// LocalEncryptionConfig holds local encryption configuration
type LocalEncryptionConfig struct {
	KeyRotationInterval time.Duration `yaml:"key_rotation_interval"`
	Algorithm           string        `yaml:"algorithm"`
}

// VaultConfig holds Vault configuration
type VaultConfig struct {
	Address     string `yaml:"address"`
	Token       string `yaml:"token"`
	TransitPath string `yaml:"transit_path"`
	KeyName     string `yaml:"key_name"`
}
type ClickHouseConfig struct {
	Address  string `yaml:"address"`
	Port     int    `yaml:"port"`
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
}

type WorkerPoolConfig struct {
	WorkerCount     int `yaml:"worker_count"`
	ChannelBufferSize int `yaml:"channel_buffer_size"`
}

type BatchConfig struct {
	Size          int           `yaml:"size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

// PostgresConfig holds PostgreSQL configuration
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// APIConfig holds API server configuration
type APIConfig struct {
	Port         string        `yaml:"port"`
	JWTSecret    string        `yaml:"jwt_secret"`
	JWTExpiry    time.Duration `yaml:"jwt_expiry"`
	JWTIssuer    string        `yaml:"jwt_issuer"`
}

// NewConfig loads configuration from a YAML file
func NewConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DefaultConfig returns a default configuration for development
func DefaultConfig() *Config {
	return &Config{
		Kafka: KafkaConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   "logs",
			GroupID: "log-processor",
		},
		Crypto: CryptoConfig{
			FieldsToEncrypt: []string{"password", "api_key", "passport", "snils"},
			EncryptionType:  "vault",
			LocalEncryption: LocalEncryptionConfig{
				KeyRotationInterval: 24 * time.Hour,
				Algorithm:           "aes-gcm",
			},
		},
		Vault: VaultConfig{
			Address:     "http://localhost:8200",
			Token:       "my-root-token",
			TransitPath: "transit",
			KeyName:     "kafka-encryption",
		},
		ClickHouse: ClickHouseConfig{
			Address:  "localhost",
			Port:     9000,
			Login:    "default",
			Password: "default",
		},
		WorkerPool: WorkerPoolConfig{
			WorkerCount:       4,
			ChannelBufferSize: 100,
		},
		Batch: BatchConfig{
			Size:          10,
			FlushInterval: 5 * time.Second,
		},
	}
}
