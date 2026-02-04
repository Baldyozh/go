package main

import (
	"fmt"
	"io/ioutil"
	"log-processor/config"
	"sync"
	"time"

	consumer2 "log-processor/consumer"
	"log-processor/test"

	"github.com/segmentio/kafka-go"
	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (*config.Config, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg config.Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	cfg, err := LoadConfig("config/config.yaml")
	if err != nil {
		fmt.Errorf("Ошибка при загрузке конфигурации: %s", err)
	}

	StartTime := time.Now()
	test.AddLogsToKafka()

	consumerConfig := kafka.ReaderConfig{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topic,
		GroupID: cfg.Kafka.GroupID,
	}
	reader := kafka.NewReader(consumerConfig)
	defer reader.Close()
	var consumer = consumer2.NewKafkaConsumer(*reader)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			consumer.ReadLog()
		}
		wg.Done()
		fmt.Println(time.Since(StartTime))
	}()
	wg.Wait()

}
