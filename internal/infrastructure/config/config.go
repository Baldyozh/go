package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Kafka      KafkaConfig      `yaml:"kafka"`
	Crypto     CryptoConfig     `yaml:"crypto"`
	Vault      VaultConfig      `yaml:"vault"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	Topic   string   `yaml:"topic"`
	GroupID string   `yaml:"group_id"`
}

// CryptoConfig holds encryption configuration
type CryptoConfig struct {
	FieldsToEncrypt []string `yaml:"fields_to_encrypt"`
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
		},
		Vault: VaultConfig{
			Address:     "http://localhost:8200",
			Token:       "my-root-token",
			TransitPath: "transit",
			KeyName:     "kafka-encryption",
		},
		ClickHouse: ClickHouseConfig{
			Address:  "localhost",
			Port:     8123,
			Login:    "default",
			Password: "default",
		},
	}
}
