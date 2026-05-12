package benchmarks

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Baldyozh/log-processor/internal/infrastructure/config"
	"github.com/Baldyozh/log-processor/internal/infrastructure/vault"
	"github.com/Baldyozh/log-processor/internal/usecase/process_logs"
)

// BenchmarkEncryptionComparison compares local vs Vault encryption performance
func BenchmarkEncryptionComparison(b *testing.B) {
	testData := []byte(`{"user_id": 123, "passport": "AB123456", "snils": "123-456-789", "action": "test", "data": "sample sensitive data"}`)
	fields := []string{"passport", "snils"}

	// Test configurations
	testCases := []struct {
		name           string
		encryptionType string
		setupFunc      func() (process_logs.LogEncrypter, error)
	}{
		{
			name:           "vault_transit_encryption",
			encryptionType: "vault",
			setupFunc:      setupVaultEncrypter,
		},
		{
			name:           "local_encryption",
			encryptionType: "local", 
			setupFunc:      setupLocalEncrypter,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			encrypter, err := tc.setupFunc()
			if err != nil {
				b.Skipf("Failed to setup encrypter: %v", err)
				return
			}
			defer func() {
				if closer, ok := encrypter.(interface{ Close() error }); ok {
					closer.Close()
				}
			}()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := encrypter.EncryptFields(testData, fields)
				if err != nil {
					b.Fatalf("Encryption failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkEncryptionWithDifferentDataSizes tests encryption with various data sizes
func BenchmarkEncryptionWithDifferentDataSizes(b *testing.B) {
	dataSizes := []int{
		100,   // 100 bytes
		1024,  // 1KB
		10240, // 10KB
	}

	// Create test data with sensitive fields
	testData := map[int][]byte{}
	for _, size := range dataSizes {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte('A' + (i % 26))
		}
		
		// Create JSON with sensitive fields
		jsonData := map[string]interface{}{
			"user_id":  123,
			"passport": string(data[:len(data)/3]),
			"snils":    string(data[len(data)/3 : 2*len(data)/3]),
			"data":     string(data[2*len(data)/3:]),
		}
		
		jsonBytes, _ := json.Marshal(jsonData)
		testData[size] = jsonBytes
	}

	for _, size := range dataSizes {
		b.Run(fmt.Sprintf("local_encryption_%dB", size), func(b *testing.B) {
			encrypter, err := setupLocalEncrypter()
			if err != nil {
				b.Skipf("Failed to setup local encrypter: %v", err)
				return
			}
			defer encrypter.(interface{ Close() error }).Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := encrypter.EncryptFields(testData[size], []string{"passport", "snils"})
				if err != nil {
					b.Fatalf("Encryption failed: %v", err)
				}
			}

			b.ReportMetric(float64(size), "data_size_bytes")
		})
	}
}

// BenchmarkEncryptionLatency measures encryption latency
func BenchmarkEncryptionLatency(b *testing.B) {
	encrypter, err := setupLocalEncrypter()
	if err != nil {
		b.Skipf("Failed to setup local encrypter: %v", err)
		return
	}
	defer encrypter.(interface{ Close() error }).Close()

	testData := []byte(`{"user_id": 123, "passport": "AB123456", "snils": "123-456-789", "action": "test"}`)
	fields := []string{"passport", "snils"}

	// Measure single operation latency
	b.ResetTimer()
	start := time.Now()

	for i := 0; i < b.N; i++ {
		_, err := encrypter.EncryptFields(testData, fields)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}
	}

	elapsed := time.Since(start)
	avgLatency := elapsed.Nanoseconds() / int64(b.N)
	b.ReportMetric(float64(avgLatency), "avg_latency_ns")
	b.ReportMetric(float64(1000000000/avgLatency), "ops_per_sec")
}

// BenchmarkKeyRotationOverhead measures overhead of key rotation
func BenchmarkKeyRotationOverhead(b *testing.B) {
	// Setup local encrypter with short rotation interval for testing
	shortIntervalConfig := config.LocalEncryptionConfig{
		KeyRotationInterval: 1 * time.Second, // Very short for testing
		Algorithm:           "aes-gcm",
	}

	vaultConfig := &vault.Config{
		Address:     "http://localhost:8200",
		Token:       "my-root-token",
		TransitPath: "transit",
		KeyName:     "kafka-encryption",
	}

	vaultManager, err := vault.NewManager(vaultConfig)
	if err != nil {
		b.Skipf("Failed to create Vault manager: %v", err)
		return
	}
	defer vaultManager.Close()

	encrypter := vault.NewLocalEncrypter(vaultManager, shortIntervalConfig, []string{"passport", "snils"})
	if err := encrypter.Initialize(); err != nil {
		b.Skipf("Failed to initialize local encrypter: %v", err)
		return
	}
	defer encrypter.Close()

	testData := []byte(`{"passport": "AB123456", "snils": "123-456-789"}`)
	fields := []string{"passport", "snils"}

	// Run for longer to see key rotation effects
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := encrypter.EncryptFields(testData, fields)
			if err != nil {
				b.Fatalf("Encryption failed: %v", err)
			}
		}
	})
}

// setupVaultEncrypter creates a Vault transit encrypter for testing
func setupVaultEncrypter() (process_logs.LogEncrypter, error) {
	vaultConfig := &vault.Config{
		Address:     "http://localhost:8200",
		Token:       "my-root-token",
		TransitPath: "transit",
		KeyName:     "kafka-encryption",
	}

	vaultManager, err := vault.NewManager(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault manager: %w", err)
	}

	encrypter := vault.NewEncrypter(vaultManager)
	return encrypter, nil
}

// setupLocalEncrypter creates a local encrypter for testing
func setupLocalEncrypter() (process_logs.LogEncrypter, error) {
	vaultConfig := &vault.Config{
		Address:     "http://localhost:8200",
		Token:       "my-root-token",
		TransitPath: "transit",
		KeyName:     "kafka-encryption",
	}

	vaultManager, err := vault.NewManager(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault manager: %w", err)
	}

	localConfig := config.LocalEncryptionConfig{
		KeyRotationInterval: 24 * time.Hour,
		Algorithm:           "aes-gcm",
	}

	encrypter := vault.NewLocalEncrypter(vaultManager, localConfig, []string{"passport", "snils"})
	if err := encrypter.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize local encrypter: %w", err)
	}

	return encrypter, nil
}

// BenchmarkEndToEndEncryption compares full log processing with different encryption types
func BenchmarkEndToEndEncryption(b *testing.B) {
	testCases := []struct {
		name           string
		encryptionType string
		configFile     string
	}{
		{
			name:           "vault_transit",
			encryptionType: "vault",
			configFile:     "config/config.yaml",
		},
		{
			name:           "local_encryption",
			encryptionType: "local",
			configFile:     "config/config-local.yaml",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Load configuration
			cfg, err := config.NewConfig(tc.configFile)
			if err != nil {
				b.Skipf("Failed to load config %s: %v", tc.configFile, err)
				return
			}

			// Setup components (mock for benchmarking)
			consumer := NewMockConsumer(1000 * b.N)
			storage := NewMockStorage()

			// Setup encrypter based on type
			var encrypter process_logs.LogEncrypter
			switch cfg.Crypto.EncryptionType {
			case "local":
				vaultConfig := &vault.Config{
					Address:     cfg.Vault.Address,
					Token:       cfg.Vault.Token,
					TransitPath: cfg.Vault.TransitPath,
					KeyName:     cfg.Vault.KeyName,
				}
				vaultManager, err := vault.NewManager(vaultConfig)
				if err != nil {
					b.Skipf("Failed to create Vault manager: %v", err)
					return
				}
				defer vaultManager.Close()

				localEncrypter := vault.NewLocalEncrypter(vaultManager, cfg.Crypto.LocalEncryption, cfg.Crypto.FieldsToEncrypt)
				if err := localEncrypter.Initialize(); err != nil {
					b.Skipf("Failed to initialize local encrypter: %v", err)
					return
				}
				defer localEncrypter.Close()
				encrypter = localEncrypter
			case "vault":
				vaultConfig := &vault.Config{
					Address:     cfg.Vault.Address,
					Token:       cfg.Vault.Token,
					TransitPath: cfg.Vault.TransitPath,
					KeyName:     cfg.Vault.KeyName,
				}
				vaultManager, err := vault.NewManager(vaultConfig)
				if err != nil {
					b.Skipf("Failed to create Vault manager: %v", err)
					return
				}
				defer vaultManager.Close()
				encrypter = vault.NewEncrypter(vaultManager)
			}

			// Create processor
			processor := process_logs.NewLogProcessor(
				consumer,
				encrypter,
				storage,
				cfg.Crypto.FieldsToEncrypt,
				process_logs.WorkerPoolConfig{
					WorkerCount:       2, // Reduced for benchmark stability
					ChannelBufferSize: 100,
				},
				process_logs.BatchConfig{
					Size:          100,
					FlushInterval: 100 * time.Millisecond,
				},
			)

			b.ResetTimer()
			start := time.Now()

			// Process logs
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
			b.ReportMetric(elapsed.Seconds(), "duration_seconds")
			b.ReportMetric(float64(b.N), "benchmark_iterations")
		})
	}
}
