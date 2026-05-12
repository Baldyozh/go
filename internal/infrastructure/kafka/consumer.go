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
