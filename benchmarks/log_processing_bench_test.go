package benchmarks

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/usecase/process_logs"
)

// MockConsumer implements LogConsumer interface for benchmarking
type MockConsumer struct {
	logs    []entities.Log
	current int
}

func NewMockConsumer(logCount int) *MockConsumer {
	logs := make([]entities.Log, logCount)
	for i := 0; i < logCount; i++ {
		logs[i] = generateTestLog(i)
	}
	return &MockConsumer{logs: logs, current: 0}
}

func (m *MockConsumer) ReadLog() (entities.Log, error) {
	if m.current >= len(m.logs) {
		return entities.Log{}, fmt.Errorf("no more logs")
	}
	log := m.logs[m.current]
	m.current++
	return log, nil
}

func (m *MockConsumer) Close() error {
	return nil
}

// MockStorage implements LogStorage interface for benchmarking
type MockStorage struct {
	insertedCount int
	batchSizes    []int
}

func NewMockStorage() *MockStorage {
	return &MockStorage{insertedCount: 0, batchSizes: []int{}}
}

func (m *MockStorage) InsertBatch(records []entities.ClickHouseLogRecord) error {
	m.insertedCount += len(records)
	m.batchSizes = append(m.batchSizes, len(records))
	return nil
}

func (m *MockStorage) GetInsertedCount() int {
	return m.insertedCount
}

func (m *MockStorage) GetBatchSizes() []int {
	return m.batchSizes
}

// MockEncrypter implements LogEncrypter interface for benchmarking
type MockEncrypter struct {
	encryptTime time.Duration
}

func NewMockEncrypter(encryptTime time.Duration) *MockEncrypter {
	return &MockEncrypter{encryptTime: encryptTime}
}

func (m *MockEncrypter) EncryptFields(data []byte, fields []string) ([]byte, error) {
	if m.encryptTime > 0 {
		time.Sleep(m.encryptTime)
	}
	// Simple mock encryption - just return the data with a prefix
	return append([]byte("encrypted_"), data...), nil
}

// generateTestLog creates a realistic test log entry
func generateTestLog(id int) entities.Log {
	return entities.Log{
		TimeStamp: time.Now().Add(-time.Duration(id) * time.Second),
		LogBody: entities.ClickHouseLogRecord{
			LogID:         fmt.Sprintf("log_%d", id),
			Timestamp:     time.Now().Add(-time.Duration(id) * time.Second),
			IntegrationID: fmt.Sprintf("integration_%d", id%10),
			RequestID:     fmt.Sprintf("req_%d", id),
			HTTPMethod:    []string{"GET", "POST", "PUT", "DELETE"}[id%4],
			Endpoint:      fmt.Sprintf("/api/v1/resource/%d", id%100),
			RequestBody:   fmt.Sprintf(`{"user_id": %d, "action": "test", "data": "sample data for request %d"}`, id%1000, id),
			ResponseBody:  fmt.Sprintf(`{"status": "success", "data": {"id": %d, "result": "processed"}}`, id),
			DurationMs:    uint32(50 + id%200),
			StatusCode:    uint16(200 + id%4),
			Success:       id%10 != 0, // 90% success rate
			ErrorMessage:  "",
			UserID:        func() *uint32 { u := uint32(id % 1000); return &u }(),
		},
	}
}

// BenchmarkLogProcessing benchmarks the complete log processing pipeline
func BenchmarkLogProcessing(b *testing.B) {
	testCases := []struct {
		name           string
		workerCount    int
		batchSize      int
		flushInterval  time.Duration
		logCount       int
		encryptTime    time.Duration
	}{
		{
			name:          "1_worker_small_batch",
			workerCount:   1,
			batchSize:     10,
			flushInterval: 100 * time.Millisecond,
			logCount:      1000,
			encryptTime:   0,
		},
		{
			name:          "4_workers_medium_batch",
			workerCount:   4,
			batchSize:     50,
			flushInterval: 200 * time.Millisecond,
			logCount:      1000,
			encryptTime:   0,
		},
		{
			name:          "8_workers_large_batch",
			workerCount:   8,
			batchSize:     100,
			flushInterval: 500 * time.Millisecond,
			logCount:      1000,
			encryptTime:   0,
		},
		{
			name:          "4_workers_with_encryption",
			workerCount:   4,
			batchSize:     50,
			flushInterval: 200 * time.Millisecond,
			logCount:      1000,
			encryptTime:   1 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkLogProcessingConfig(b, tc.workerCount, tc.batchSize, tc.flushInterval, tc.logCount, tc.encryptTime)
		})
	}
}

func benchmarkLogProcessingConfig(b *testing.B, workerCount, batchSize int, flushInterval time.Duration, logCount int, encryptTime time.Duration) {
	consumer := NewMockConsumer(logCount * b.N)
	storage := NewMockStorage()
	encrypter := NewMockEncrypter(encryptTime)

	processor := process_logs.NewLogProcessor(
		consumer,
		encrypter,
		storage,
		[]string{"user_id", "data"}, // sensitive fields
		process_logs.WorkerPoolConfig{
			WorkerCount:       workerCount,
			ChannelBufferSize: 1000,
		},
		process_logs.BatchConfig{
			Size:          batchSize,
			FlushInterval: flushInterval,
		},
	)

	b.ResetTimer()
	start := time.Now()

	// Process logs in a goroutine to simulate real processing
	doneCh := make(chan struct{})
	go processor.ProcessLogs(doneCh)

	// Wait for processing to complete or timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			close(doneCh)
			b.Fatalf("Benchmark timed out after 30 seconds")
		case <-ticker.C:
			if storage.GetInsertedCount() >= logCount*b.N {
				close(doneCh)
				goto done
			}
		}
	}

done:
	elapsed := time.Since(start)
	totalLogs := storage.GetInsertedCount()
	logsPerSecond := float64(totalLogs) / elapsed.Seconds()

	b.ReportMetric(logsPerSecond, "logs/sec")
	b.ReportMetric(float64(totalLogs), "total_logs")
	b.ReportMetric(elapsed.Seconds(), "duration_seconds")
}

// BenchmarkLogGeneration benchmarks log generation speed
func BenchmarkLogGeneration(b *testing.B) {
	b.Run("json_serialization", func(b *testing.B) {
		log := generateTestLog(1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := json.Marshal(log.LogBody)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("log_creation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = generateTestLog(i)
		}
	})
}

// BenchmarkEncryption benchmarks encryption performance
func BenchmarkEncryption(b *testing.B) {
	testData := []byte(`{"user_id": 123, "action": "test", "data": "sample data with sensitive information"}`)
	fields := []string{"user_id", "data"}

	b.Run("mock_encryption", func(b *testing.B) {
		encrypter := NewMockEncrypter(0)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := encrypter.EncryptFields(testData, fields)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("mock_encryption_with_delay", func(b *testing.B) {
		encrypter := NewMockEncrypter(1 * time.Millisecond)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := encrypter.EncryptFields(testData, fields)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkBatchInsertion benchmarks batch insertion performance
func BenchmarkBatchInsertion(b *testing.B) {
	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_size_%d", batchSize), func(b *testing.B) {
			storage := NewMockStorage()
			records := make([]entities.ClickHouseLogRecord, batchSize)
			for i := 0; i < batchSize; i++ {
				records[i] = generateTestLog(i).LogBody
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := storage.InsertBatch(records)
				if err != nil {
					b.Fatal(err)
				}
			}

			totalRecords := storage.GetInsertedCount()
			b.ReportMetric(float64(totalRecords)/float64(b.N), "records_per_operation")
		})
	}
}

// BenchmarkThroughputScaling benchmarks how throughput scales with different configurations
func BenchmarkThroughputScaling(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16}
	batchSizes := []int{10, 50, 100}

	for _, workers := range workerCounts {
		for _, batchSize := range batchSizes {
			name := fmt.Sprintf("workers_%d_batch_%d", workers, batchSize)
			b.Run(name, func(b *testing.B) {
				consumer := NewMockConsumer(1000 * b.N)
				storage := NewMockStorage()
				encrypter := NewMockEncrypter(0)

				processor := process_logs.NewLogProcessor(
					consumer,
					encrypter,
					storage,
					[]string{"user_id"},
					process_logs.WorkerPoolConfig{
						WorkerCount:       workers,
						ChannelBufferSize: 1000,
					},
					process_logs.BatchConfig{
						Size:          batchSize,
						FlushInterval: 100 * time.Millisecond,
					},
				)

				b.ResetTimer()
				start := time.Now()

				doneCh := make(chan struct{})
				go processor.ProcessLogs(doneCh)

				// Wait for completion
				timeout := time.After(30 * time.Second)
				ticker := time.NewTicker(50 * time.Millisecond)
				defer ticker.Stop()

				for {
					select {
					case <-timeout:
						close(doneCh)
						b.Fatalf("Timeout")
					case <-ticker.C:
						if storage.GetInsertedCount() >= 1000*b.N {
							close(doneCh)
							goto done
						}
					}
				}

			done:
				elapsed := time.Since(start)
				totalLogs := storage.GetInsertedCount()
				logsPerSecond := float64(totalLogs) / elapsed.Seconds()

				b.ReportMetric(logsPerSecond, "logs/sec")
				b.ReportMetric(float64(workers), "workers")
				b.ReportMetric(float64(batchSize), "batch_size")
			})
		}
	}
}
