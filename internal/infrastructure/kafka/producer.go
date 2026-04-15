package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"log-processor/internal/domain/entities"
	"log-processor/internal/usecase/add_logs"

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
