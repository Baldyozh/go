package benchmarks

import (
	"fmt"
	"testing"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/usecase/process_logs"
)

// BenchmarkLoadPatterns benchmarks different realistic load patterns
func BenchmarkLoadPatterns(b *testing.B) {
	testCases := []struct {
		name        string
		pattern     string
		workerCount int
		batchSize   int
		logCount    int
	}{
		{
			name:        "steady_load",
			pattern:     "steady",
			workerCount: 4,
			batchSize:   50,
			logCount:    1000,
		},
		{
			name:        "burst_load",
			pattern:     "burst",
			workerCount: 8,
			batchSize:   100,
			logCount:    1000,
		},
		{
			name:        "gradual_increase",
			pattern:     "gradual",
			workerCount: 4,
			batchSize:   50,
			logCount:    1000,
		},
		{
			name:        "high_volume",
			pattern:     "high_volume",
			workerCount: 16,
			batchSize:   200,
			logCount:    5000,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkLoadPattern(b, tc.pattern, tc.workerCount, tc.batchSize, tc.logCount)
		})
	}
}

func benchmarkLoadPattern(b *testing.B, pattern string, workerCount, batchSize, logCount int) {
	var consumer process_logs.LogConsumer
	switch pattern {
	case "steady":
		consumer = NewMockConsumer(logCount * b.N)
	case "burst":
		consumer = NewBurstConsumer(logCount * b.N, 100) // 100 logs per burst
	case "gradual":
		consumer = NewGradualConsumer(logCount * b.N, 10) // Start with 10, gradually increase
	case "high_volume":
		consumer = NewMockConsumer(logCount * b.N)
	default:
		consumer = NewMockConsumer(logCount * b.N)
	}

	storage := NewMockStorage()
	encrypter := NewMockEncrypter(0)

	processor := process_logs.NewLogProcessor(
		consumer,
		encrypter,
		storage,
		[]string{"user_id", "data"},
		process_logs.WorkerPoolConfig{
			WorkerCount:       workerCount,
			ChannelBufferSize: 2000,
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
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			close(doneCh)
			b.Fatalf("Benchmark timed out after 60 seconds")
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

// BurstConsumer simulates bursty traffic patterns
type BurstConsumer struct {
	logs       []entities.Log
	current    int
	burstSize  int
	burstCount int
}

func NewBurstConsumer(logCount, burstSize int) *BurstConsumer {
	logs := make([]entities.Log, logCount)
	for i := 0; i < logCount; i++ {
		logs[i] = generateTestLog(i)
	}
	return &BurstConsumer{
		logs:       logs,
		current:    0,
		burstSize:  burstSize,
		burstCount: 0,
	}
}

func (b *BurstConsumer) ReadLog() (entities.Log, error) {
	if b.current >= len(b.logs) {
		return entities.Log{}, fmt.Errorf("no more logs")
	}

	// Simulate burst pattern: process burstSize logs, then pause
	if b.burstCount > 0 && b.burstCount%b.burstSize == 0 {
		time.Sleep(50 * time.Millisecond) // Pause between bursts
	}

	log := b.logs[b.current]
	b.current++
	b.burstCount++
	return log, nil
}

func (b *BurstConsumer) Close() error {
	return nil
}

// GradualConsumer simulates gradually increasing traffic
type GradualConsumer struct {
	logs        []entities.Log
	current     int
	rate        int
	increment   int
	logsInCycle int
}

func NewGradualConsumer(logCount, initialRate int) *GradualConsumer {
	logs := make([]entities.Log, logCount)
	for i := 0; i < logCount; i++ {
		logs[i] = generateTestLog(i)
	}
	return &GradualConsumer{
		logs:        logs,
		current:     0,
		rate:        initialRate,
		increment:   5,
		logsInCycle: 0,
	}
}

func (g *GradualConsumer) ReadLog() (entities.Log, error) {
	if g.current >= len(g.logs) {
		return entities.Log{}, fmt.Errorf("no more logs")
	}

	// Gradually increase rate
	if g.logsInCycle >= g.rate {
		g.rate += g.increment
		g.logsInCycle = 0
		time.Sleep(100 * time.Millisecond) // Brief pause when rate increases
	}

	log := g.logs[g.current]
	g.current++
	g.logsInCycle++
	return log, nil
}

func (g *GradualConsumer) Close() error {
	return nil
}

// BenchmarkLogSizes benchmarks performance with different log sizes
func BenchmarkLogSizes(b *testing.B) {
	sizeCases := []struct {
		name        string
		bodySize    int
		workerCount int
		batchSize   int
	}{
		{
			name:        "small_logs_100b",
			bodySize:    100,
			workerCount: 4,
			batchSize:   50,
		},
		{
			name:        "medium_logs_1kb",
			bodySize:    1024,
			workerCount: 4,
			batchSize:   50,
		},
		{
			name:        "large_logs_10kb",
			bodySize:    10240,
			workerCount: 4,
			batchSize:   25,
		},
		{
			name:        "xlarge_logs_100kb",
			bodySize:    102400,
			workerCount: 2,
			batchSize:   10,
		},
	}

	for _, tc := range sizeCases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkLogSize(b, tc.bodySize, tc.workerCount, tc.batchSize)
		})
	}
}

func benchmarkLogSize(b *testing.B, bodySize, workerCount, batchSize int) {
	consumer := NewMockConsumerWithSize(1000*b.N, bodySize)
	storage := NewMockStorage()
	encrypter := NewMockEncrypter(0)

	processor := process_logs.NewLogProcessor(
		consumer,
		encrypter,
		storage,
		[]string{"user_id", "data"},
		process_logs.WorkerPoolConfig{
			WorkerCount:       workerCount,
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

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			close(doneCh)
			b.Fatalf("Benchmark timed out")
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
	throughputMBps := float64(totalLogs*bodySize) / (1024 * 1024 * elapsed.Seconds())

	b.ReportMetric(logsPerSecond, "logs/sec")
	b.ReportMetric(throughputMBps, "MB/sec")
	b.ReportMetric(float64(bodySize), "body_size_bytes")
}

// MockConsumerWithSize creates logs with specific body sizes
func NewMockConsumerWithSize(logCount, bodySize int) *MockConsumer {
	logs := make([]entities.Log, logCount)
	for i := 0; i < logCount; i++ {
		logs[i] = generateTestLogWithSize(i, bodySize)
	}
	return &MockConsumer{logs: logs, current: 0}
}

func generateTestLogWithSize(id, bodySize int) entities.Log {
	// Generate body content of specified size
	bodyContent := make([]byte, bodySize)
	for i := range bodyContent {
		bodyContent[i] = byte('A' + (i % 26))
	}

	return entities.Log{
		TimeStamp: time.Now().Add(-time.Duration(id) * time.Second),
		LogBody: entities.ClickHouseLogRecord{
			LogID:         fmt.Sprintf("log_%d", id),
			Timestamp:     time.Now().Add(-time.Duration(id) * time.Second),
			IntegrationID: fmt.Sprintf("integration_%d", id%10),
			RequestID:     fmt.Sprintf("req_%d", id),
			HTTPMethod:    []string{"GET", "POST", "PUT", "DELETE"}[id%4],
			Endpoint:      fmt.Sprintf("/api/v1/resource/%d", id%100),
			RequestBody:   string(bodyContent),
			ResponseBody:  string(bodyContent),
			DurationMs:    uint32(50 + id%200),
			StatusCode:    uint16(200 + id%4),
			Success:       id%10 != 0,
			ErrorMessage:  "",
			UserID:        func() *uint32 { u := uint32(id % 1000); return &u }(),
		},
	}
}

// BenchmarkErrorRates benchmarks performance under different error rates
func BenchmarkErrorRates(b *testing.B) {
	errorRates := []float64{0.0, 0.01, 0.05, 0.1, 0.2} // 0%, 1%, 5%, 10%, 20%

	for _, errorRate := range errorRates {
		name := fmt.Sprintf("error_rate_%.0f%%", errorRate*100)
		b.Run(name, func(b *testing.B) {
			benchmarkErrorRate(b, errorRate)
		})
	}
}

func benchmarkErrorRate(b *testing.B, errorRate float64) {
	consumer := NewMockConsumerWithErrors(1000*b.N, errorRate)
	storage := NewMockStorage()
	encrypter := NewMockEncrypter(0)

	processor := process_logs.NewLogProcessor(
		consumer,
		encrypter,
		storage,
		[]string{"user_id", "data"},
		process_logs.WorkerPoolConfig{
			WorkerCount:       4,
			ChannelBufferSize: 1000,
		},
		process_logs.BatchConfig{
			Size:          50,
			FlushInterval: 100 * time.Millisecond,
		},
	)

	b.ResetTimer()
	start := time.Now()

	doneCh := make(chan struct{})
	go processor.ProcessLogs(doneCh)

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			close(doneCh)
			b.Fatalf("Benchmark timed out")
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
	b.ReportMetric(float64(totalLogs), "total_logs")
	b.ReportMetric(errorRate, "error_rate")
}

// MockConsumerWithErrors simulates errors during log consumption
type MockConsumerWithErrors struct {
	logs       []entities.Log
	current    int
	errorRate  float64
	errorCount int
}

func NewMockConsumerWithErrors(logCount int, errorRate float64) *MockConsumerWithErrors {
	logs := make([]entities.Log, logCount)
	for i := 0; i < logCount; i++ {
		logs[i] = generateTestLog(i)
	}
	return &MockConsumerWithErrors{
		logs:      logs,
		current:   0,
		errorRate: errorRate,
	}
}

func (m *MockConsumerWithErrors) ReadLog() (entities.Log, error) {
	if m.current >= len(m.logs) {
		return entities.Log{}, fmt.Errorf("no more logs")
	}

	// Simulate errors based on error rate
	if float64(m.errorCount%100) < m.errorRate*100 {
		m.errorCount++
		return entities.Log{}, fmt.Errorf("simulated consumer error")
	}

	log := m.logs[m.current]
	m.current++
	return log, nil
}

func (m *MockConsumerWithErrors) Close() error {
	return nil
}
