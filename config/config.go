package config

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kafka  KafkaConfig  `yaml:"kafka"`
	Crypto CryptoConfig `yaml:"crypto"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"path"`
	Topic   string   `yaml:"topic"`
	GroupID string   `yaml:"group_id"`
}

type CryptoConfig struct {
	FieldsToEncrypt []string `yaml:"fields_to_encrypt"`
}

func NewConfig(path string) (*Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
