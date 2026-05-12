package process_logs

import (
	"log"
	"sync"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
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

// LogStorage is the interface for storing encrypted logs
type LogStorage interface {
	InsertBatch(records []entities.ClickHouseLogRecord) error
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	WorkerCount       int
	ChannelBufferSize int
}

// BatchConfig holds configuration for batch processing
type BatchConfig struct {
	Size          int
	FlushInterval time.Duration
}

// LogProcessor handles the business logic of processing logs
type LogProcessor struct {
	consumer        LogConsumer
	encrypter       LogEncrypter
	storage         LogStorage
	sensitiveFields []string
	workerPoolCfg   WorkerPoolConfig
	batchCfg        BatchConfig
}

// NewLogProcessor creates a new LogProcessor
func NewLogProcessor(
	consumer LogConsumer,
	encrypter LogEncrypter,
	storage LogStorage,
	sensitiveFields []string,
	workerPoolCfg WorkerPoolConfig,
	batchCfg BatchConfig,
) *LogProcessor {
	return &LogProcessor{
		consumer:        consumer,
		encrypter:       encrypter,
		storage:         storage,
		sensitiveFields: sensitiveFields,
		workerPoolCfg:   workerPoolCfg,
		batchCfg:        batchCfg,
	}
}

// ProcessLogs reads logs from the consumer, encrypts sensitive fields, and stores them in ClickHouse
func (p *LogProcessor) ProcessLogs(doneCh <-chan struct{}) {
	// Channel for raw logs from Kafka
	rawLogCh := make(chan entities.Log, p.workerPoolCfg.ChannelBufferSize)

	// Channel for encrypted logs ready for batch insertion
	encryptedLogCh := make(chan entities.ClickHouseLogRecord, p.workerPoolCfg.ChannelBufferSize)

	// Start worker pool for concurrent encryption
	var wg sync.WaitGroup
	for i := 0; i < p.workerPoolCfg.WorkerCount; i++ {
		wg.Add(1)
		go p.encryptWorker(i, rawLogCh, encryptedLogCh, &wg)
	}

	// Start async batch writer
	writerDone := make(chan struct{})
	go p.batchWriter(encryptedLogCh, doneCh, writerDone)

	// Kafka reader loop
	defer close(rawLogCh)
	for {
		select {
		case <-doneCh:
			return
		default:
			logEntry, err := p.consumer.ReadLog()
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			rawLogCh <- logEntry
		}
	}
}

// encryptWorker is a worker that reads raw logs, encrypts sensitive fields, and sends to encrypted channel
func (p *LogProcessor) encryptWorker(id int, inCh <-chan entities.Log, outCh chan<- entities.ClickHouseLogRecord, wg *sync.WaitGroup) {
	defer wg.Done()

	for logEntry := range inCh {
		// Encrypt request body
		encryptedRequestBody, err := p.encrypter.EncryptFields([]byte(logEntry.LogBody.RequestBody), p.sensitiveFields)
		if err != nil {
			log.Printf("[Worker %d] Failed to encrypt request body: %v", id, err)
			continue
		}

		// Encrypt response body
		encryptedResponseBody, err := p.encrypter.EncryptFields([]byte(logEntry.LogBody.ResponseBody), p.sensitiveFields)
		if err != nil {
			log.Printf("[Worker %d] Failed to encrypt response body: %v", id, err)
			continue
		}

		// Update encrypted log record
		logEntry.LogBody.RequestBody = string(encryptedRequestBody)
		logEntry.LogBody.ResponseBody = string(encryptedResponseBody)

		// Send to batch writer
		outCh <- logEntry.LogBody
	}
}

// batchWriter collects encrypted logs and inserts them in batches to ClickHouse
func (p *LogProcessor) batchWriter(inCh <-chan entities.ClickHouseLogRecord, doneCh <-chan struct{}, writerDone chan<- struct{}) {
	defer close(writerDone)

	batch := make([]entities.ClickHouseLogRecord, 0, p.batchCfg.Size)
	flushTicker := time.NewTicker(p.batchCfg.FlushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-doneCh:
			// Flush remaining logs before exit
			if len(batch) > 0 {
				p.flushBatch(batch)
			}
			return
		case <-flushTicker.C:
			if len(batch) > 0 {
				p.flushBatch(batch)
				batch = batch[:0]
			}
		case record, ok := <-inCh:
			if !ok {
				// Channel closed, flush remaining
				if len(batch) > 0 {
					p.flushBatch(batch)
				}
				return
			}
			batch = append(batch, record)

			if len(batch) >= p.batchCfg.Size {
				p.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (p *LogProcessor) flushBatch(batch []entities.ClickHouseLogRecord) {
	if err := p.storage.InsertBatch(batch); err != nil {
		log.Printf("Failed to insert batch to ClickHouse: %v", err)
	} else {
		log.Printf("Successfully inserted %d logs to ClickHouse", len(batch))
	}
}
