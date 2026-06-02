# Log Processing Benchmarks

This directory contains comprehensive benchmarks for measuring log processing throughput in the Kafka log processor system.

## Overview

The benchmarks measure how many logs per second can be processed through the complete pipeline:
Kafka Consumer → Encryption → ClickHouse Storage

## Benchmark Types

### 1. Basic Throughput Benchmarks (`log_processing_bench_test.go`)

**BenchmarkLogProcessing**: Tests different worker pool and batch configurations:
- `1_worker_small_batch`: 1 worker, batch size 10
- `4_workers_medium_batch`: 4 workers, batch size 50  
- `8_workers_large_batch`: 8 workers, batch size 100
- `4_workers_with_encryption`: Includes encryption overhead

**BenchmarkLogGeneration**: Tests log creation and serialization performance

**BenchmarkEncryption**: Tests encryption performance with and without delays

**BenchmarkBatchInsertion**: Tests different batch sizes for database insertion

**BenchmarkThroughputScaling**: Comprehensive scaling tests with different worker counts and batch sizes

### 2. Load Pattern Benchmarks (`load_patterns_test.go`)

**BenchmarkLoadPatterns**: Tests realistic traffic patterns:
- `steady_load`: Constant log rate
- `burst_load`: Bursty traffic with pauses
- `gradual_increase`: Gradually increasing traffic
- `high_volume`: High volume sustained load

**BenchmarkLogSizes**: Tests performance with different log body sizes:
- `small_logs_100b`: 100 byte logs
- `medium_logs_1kb`: 1KB logs  
- `large_logs_10kb`: 10KB logs
- `xlarge_logs_100kb`: 100KB logs

**BenchmarkErrorRates**: Tests performance under different error conditions:
- `error_rate_0%`: No errors
- `error_rate_1%`: 1% error rate
- `error_rate_5%`: 5% error rate
- `error_rate_10%`: 10% error rate
- `error_rate_20%`: 20% error rate

## Running Benchmarks

### Quick Start

```bash
# Run all benchmarks with analysis
go run benchmarks/runner.go all

# Run specific benchmark suites
go run benchmarks/runner.go throughput
go run benchmarks/runner.go patterns  
go run benchmarks/runner.go scaling
```

### Manual Benchmark Execution

```bash
# Run specific benchmark
go test -bench=BenchmarkLogProcessing/4_workers_medium_batch -benchmem ./benchmarks/

# Run all benchmarks
go test -bench=. -benchmem ./benchmarks/

# Run with specific iterations
go test -bench=BenchmarkLogProcessing -benchtime=10s ./benchmarks/

# Run with CPU profiling
go test -bench=BenchmarkLogProcessing -cpuprofile=cpu.prof -benchmem ./benchmarks/

# Run with memory profiling  
go test -bench=BenchmarkLogProcessing -memprofile=mem.prof -benchmem ./benchmarks/
```

## Understanding Results

### Key Metrics

- **logs/sec**: Primary throughput metric - logs processed per second
- **total_logs**: Total number of logs processed during benchmark
- **duration_seconds**: Time taken to complete the benchmark
- **MB/sec**: Data throughput in megabytes per second (for size tests)

### Performance Factors

The benchmarks test how these factors affect throughput:

1. **Worker Count**: Number of concurrent encryption workers
2. **Batch Size**: Number of logs per database batch
3. **Log Size**: Size of log bodies (affects encryption and storage)
4. **Error Rate**: Impact of errors on processing speed
5. **Load Pattern**: Traffic patterns (steady vs burst)

### Expected Performance

Based on the architecture, you should see:
- **Higher worker counts** improve throughput up to CPU limits
- **Larger batch sizes** improve database efficiency but increase latency
- **Smaller logs** process faster (higher logs/sec)
- **Higher error rates** reduce effective throughput

## Benchmark Reports

The runner generates detailed JSON reports:

```bash
# View latest report
cat benchmark_report.json | jq '.summary'

# View recommendations  
cat benchmark_report.json | jq '.recommendations[]'

# Compare configurations
cat scaling_benchmark_report.json | jq '.results[] | {name: .name, throughput: .logs_per_second}'
```

## Profile Analysis

For deep performance analysis:

```bash
# Generate CPU profile
go test -bench=BenchmarkLogProcessing -cpuprofile=cpu.prof ./benchmarks/
go tool pprof cpu.prof

# Generate memory profile
go test -bench=BenchmarkLogProcessing -memprofile=mem.prof ./benchmarks/
go tool pprof mem.prof

# Generate trace
go test -bench=BenchmarkLogProcessing -trace=trace.out ./benchmarks/
go tool trace trace.out
```

## Custom Benchmarks

### Adding New Benchmarks

1. Create benchmark functions following Go testing conventions:
```go
func BenchmarkYourScenario(b *testing.B) {
    // Setup
    // b.ResetTimer()
    // Benchmark loop
    // Report metrics
}
```

2. Use the mock implementations:
- `MockConsumer`: Simulates log consumption
- `MockStorage`: Tracks insertion performance
- `MockEncrypter`: Simulates encryption with configurable delays

3. Report custom metrics:
```go
b.ReportMetric(customValue, "custom_metric")
```

### Mock Components

**MockConsumer**: Simulates reading logs from Kafka
- `NewMockConsumer(count)`: Simple sequential consumer
- `NewBurstConsumer(count, burstSize)`: Bursty traffic pattern
- `NewGradualConsumer(count, initialRate)`: Gradually increasing rate
- `NewMockConsumerWithErrors(count, errorRate)`: Simulates errors

**MockStorage**: Tracks database insertion performance
- `GetInsertedCount()`: Total logs inserted
- `GetBatchSizes()`: Batch size history

**MockEncrypter**: Simulates encryption overhead
- `NewMockEncrypter(delay)`: Configurable encryption delay

## Performance Tuning

Based on benchmark results, consider:

1. **Worker Pool Optimization**
   - Increase workers if CPU utilization is low
   - Decrease if context switching overhead is high

2. **Batch Size Tuning**
   - Larger batches improve database throughput
   - Smaller batches reduce latency
   - Find sweet spot for your use case

3. **Channel Buffer Sizing**
   - Match buffer size to worker count
   - Prevent bottlenecks in the pipeline

4. **Encryption Optimization**
   - Consider field-level vs full encryption
   - Cache encryption keys
   - Use hardware acceleration if available

## Troubleshooting

### Common Issues

1. **Benchmarks timeout**: Increase timeout in runner or reduce log count
2. **Memory errors**: Reduce batch size or worker count  
3. **Inconsistent results**: Run multiple times, check system load
4. **Low throughput**: Check CPU utilization, consider system resources

### Debug Mode

Enable verbose logging:
```bash
go test -bench=BenchmarkLogProcessing -v ./benchmarks/
```

### System Monitoring

Monitor system resources during benchmarks:
```bash
# CPU usage
htop

# Memory usage  
free -h

# I/O stats
iostat -x 1
```

## Continuous Integration

Add benchmarks to CI pipeline:

```yaml
- name: Run benchmarks
  run: |
    go run benchmarks/runner.go throughput
    # Upload benchmark_report.json as artifact
```

Track performance over time and set alerts for regressions.
