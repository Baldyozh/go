package add_logs

import (
	"context"
	"log"

	"log-processor/internal/domain/entities"
)

// LogProducer is the interface for writing logs to a message broker
type LogProducer interface {
	WriteLog(ctx context.Context, log entities.Log) error
	Close() error
}

// LogAdder handles the business logic of adding logs
type LogAdder struct {
	producer LogProducer
}

// NewLogAdder creates a new LogAdder
func NewLogAdder(producer LogProducer) *LogAdder {
	return &LogAdder{
		producer: producer,
	}
}

// AddLogs writes a batch of logs to the message broker
func (a *LogAdder) AddLogs(ctx context.Context, logs []entities.Log) error {
	for _, logEntry := range logs {
		if err := a.producer.WriteLog(ctx, logEntry); err != nil {
			log.Printf("error writing log: %v", err)
			return err
		}
	}
	return nil
}
