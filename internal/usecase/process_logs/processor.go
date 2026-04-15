package process_logs

import (
	"log-processor/internal/domain/entities"
)

// LogConsumer is the interface for reading logs from a message broker
type LogConsumer interface {
	ReadLog() (entities.Log, error)
	Close() error
}

// LogEncrypter is the interface for encrypting log data
type LogEncrypter interface {
	EncryptFields(data []byte, fields []string) ([]byte, error)
}

// LogProcessor handles the business logic of processing logs
type LogProcessor struct {
	consumer        LogConsumer
	encrypter       LogEncrypter
	sensitiveFields []string
}

// NewLogProcessor creates a new LogProcessor
func NewLogProcessor(consumer LogConsumer, encrypter LogEncrypter, sensitiveFields []string) *LogProcessor {
	return &LogProcessor{
		consumer:        consumer,
		encrypter:       encrypter,
		sensitiveFields: sensitiveFields,
	}
}

// ProcessLogs reads logs from the consumer, encrypts sensitive fields, and returns them via the output channel
func (p *LogProcessor) ProcessLogs(doneCh <-chan struct{}) <-chan entities.Log {
	outputCh := make(chan entities.Log)

	go func() {
		defer close(outputCh)

		for {
			select {
			case <-doneCh:
				return
			default:
				log, err := p.consumer.ReadLog()
				if err != nil {
					continue
				}

				encryptedRequestBody, err := p.encrypter.EncryptFields([]byte(log.LogBody.RequestBody), p.sensitiveFields)
				if err != nil {
					continue
				}
				encryptedResponseBody, err := p.encrypter.EncryptFields([]byte(log.LogBody.ResponseBody), p.sensitiveFields)
				log.LogBody.ResponseBody = string(encryptedResponseBody)
				log.LogBody.RequestBody = string(encryptedRequestBody)
				outputCh <- log
			}
		}
	}()

	return outputCh
}
