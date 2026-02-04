package config

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
