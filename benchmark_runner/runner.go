package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// BenchmarkResult represents the parsed result of a benchmark run
type BenchmarkResult struct {
	Name           string             `json:"name"`
	LogsPerSecond  float64            `json:"logs_per_second"`
	TotalLogs      float64            `json:"total_logs"`
	Duration       float64            `json:"duration_seconds"`
	Workers        float64            `json:"workers,omitempty"`
	BatchSize      float64            `json:"batch_size,omitempty"`
	BodySize       float64            `json:"body_size_bytes,omitempty"`
	ErrorRate      float64            `json:"error_rate,omitempty"`
	ThroughputMBps float64            `json:"mb_per_sec,omitempty"`
	Metrics        map[string]float64 `json:"metrics"`
}

// BenchmarkReport represents a complete benchmark report
type BenchmarkReport struct {
	Timestamp       time.Time         `json:"timestamp"`
	Environment     map[string]string `json:"environment"`
	Summary         SummaryStats      `json:"summary"`
	Results         []BenchmarkResult `json:"results"`
	Recommendations []string          `json:"recommendations"`
}

// SummaryStats contains aggregated statistics
type SummaryStats struct {
	TotalBenchmarks int     `json:"total_benchmarks"`
	AvgThroughput   float64 `json:"avg_throughput_logs_per_sec"`
	MaxThroughput   float64 `json:"max_throughput_logs_per_sec"`
	MinThroughput   float64 `json:"min_throughput_logs_per_sec"`
	OptimalConfig   string  `json:"optimal_config"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run runner.go <benchmark_name>")
		fmt.Println("Available benchmarks:")
		fmt.Println("  - all (run all benchmarks)")
		fmt.Println("  - throughput (basic throughput benchmarks)")
		fmt.Println("  - patterns (load pattern benchmarks)")
		fmt.Println("  - scaling (scaling benchmarks)")
		os.Exit(1)
	}

	benchmarkType := os.Args[1]

	switch benchmarkType {
	case "all":
		runAllBenchmarks()
	case "throughput":
		runThroughputBenchmarks()
	case "patterns":
		runPatternBenchmarks()
	case "scaling":
		runScalingBenchmarks()
	default:
		fmt.Printf("Unknown benchmark type: %s\n", benchmarkType)
		os.Exit(1)
	}
}

func runAllBenchmarks() {
	fmt.Println("Running all benchmarks...")

	report := BenchmarkReport{
		Timestamp:   time.Now(),
		Environment: getEnvironmentInfo(),
		Results:     []BenchmarkResult{},
	}

	// Run different benchmark suites
	suites := []struct {
		name       string
		benchmarks []string
	}{
		{
			name: "Basic Throughput",
			benchmarks: []string{
				"BenchmarkLogProcessing/1_worker_small_batch",
				"BenchmarkLogProcessing/4_workers_medium_batch",
				"BenchmarkLogProcessing/8_workers_large_batch",
				"BenchmarkLogProcessing/4_workers_with_encryption",
			},
		},
		{
			name: "Load Patterns",
			benchmarks: []string{
				"BenchmarkLoadPatterns/steady_load",
				"BenchmarkLoadPatterns/burst_load",
				"BenchmarkLoadPatterns/gradual_increase",
				"BenchmarkLoadPatterns/high_volume",
			},
		},
		{
			name: "Log Sizes",
			benchmarks: []string{
				"BenchmarkLogSizes/small_logs_100b",
				"BenchmarkLogSizes/medium_logs_1kb",
				"BenchmarkLogSizes/large_logs_10kb",
				"BenchmarkLogSizes/xlarge_logs_100kb",
			},
		},
		{
			name: "Error Rates",
			benchmarks: []string{
				"BenchmarkErrorRates/error_rate_0%",
				"BenchmarkErrorRates/error_rate_1%",
				"BenchmarkErrorRates/error_rate_5%",
				"BenchmarkErrorRates/error_rate_10%",
				"BenchmarkErrorRates/error_rate_20%",
			},
		},
	}

	for _, suite := range suites {
		fmt.Printf("\n=== %s ===\n", suite.name)
		suiteResults := runBenchmarkSuite(suite.benchmarks)
		report.Results = append(report.Results, suiteResults...)
	}

	// Generate summary and recommendations
	report.Summary = generateSummary(report.Results)
	report.Recommendations = generateRecommendations(report.Results)

	// Save report
	saveReport(report, "benchmark_report.json")

	// Print summary
	printSummary(report)
}

func runThroughputBenchmarks() {
	fmt.Println("Running throughput benchmarks...")
	benchmarks := []string{
		"BenchmarkLogProcessing/1_worker_small_batch",
		"BenchmarkLogProcessing/4_workers_medium_batch",
		"BenchmarkLogProcessing/8_workers_large_batch",
		"BenchmarkLogProcessing/4_workers_with_encryption",
	}

	results := runBenchmarkSuite(benchmarks)
	report := BenchmarkReport{
		Timestamp:       time.Now(),
		Environment:     getEnvironmentInfo(),
		Results:         results,
		Summary:         generateSummary(results),
		Recommendations: generateRecommendations(results),
	}

	saveReport(report, "throughput_benchmark_report.json")
	printSummary(report)
}

func runPatternBenchmarks() {
	fmt.Println("Running load pattern benchmarks...")
	benchmarks := []string{
		"BenchmarkLoadPatterns/steady_load",
		"BenchmarkLoadPatterns/burst_load",
		"BenchmarkLoadPatterns/gradual_increase",
		"BenchmarkLoadPatterns/high_volume",
	}

	results := runBenchmarkSuite(benchmarks)
	report := BenchmarkReport{
		Timestamp:       time.Now(),
		Environment:     getEnvironmentInfo(),
		Results:         results,
		Summary:         generateSummary(results),
		Recommendations: generateRecommendations(results),
	}

	saveReport(report, "pattern_benchmark_report.json")
	printSummary(report)
}

func runScalingBenchmarks() {
	fmt.Println("Running scaling benchmarks...")

	// Run scaling benchmarks with different parameters
	results := []BenchmarkResult{}
	workerCounts := []int{1, 2, 4, 8, 16}
	batchSizes := []int{10, 50, 100}

	for _, workers := range workerCounts {
		for _, batchSize := range batchSizes {
			benchmarkName := fmt.Sprintf("BenchmarkThroughputScaling/workers_%d_batch_%d", workers, batchSize)
			fmt.Printf("Running %s...\n", benchmarkName)

			result := runSingleBenchmark(benchmarkName)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	report := BenchmarkReport{
		Timestamp:       time.Now(),
		Environment:     getEnvironmentInfo(),
		Results:         results,
		Summary:         generateSummary(results),
		Recommendations: generateRecommendations(results),
	}

	saveReport(report, "scaling_benchmark_report.json")
	printSummary(report)
}

func runBenchmarkSuite(benchmarks []string) []BenchmarkResult {
	results := []BenchmarkResult{}

	for _, benchmark := range benchmarks {
		fmt.Printf("Running %s...\n", benchmark)
		result := runSingleBenchmark(benchmark)
		if result != nil {
			results = append(results, *result)
		}
	}

	return results
}

func runSingleBenchmark(benchmarkName string) *BenchmarkResult {
	cmd := exec.Command("go", "test", "-bench", benchmarkName, "-benchmem", "./benchmarks/")
	cmd.Dir = "."

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running %s: %v\n", benchmarkName, err)
		fmt.Printf("Output: %s\n", string(output))
		return nil
	}

	return parseBenchmarkOutput(string(output), benchmarkName)
}

func parseBenchmarkOutput(output, benchmarkName string) *BenchmarkResult {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, benchmarkName) {
			return parseBenchmarkLine(line, benchmarkName)
		}
	}

	return nil
}

func parseBenchmarkLine(line, benchmarkName string) *BenchmarkResult {
	// Example line: BenchmarkLogProcessing/4_workers_medium_batch-8   	  12345	   98765 ns/op	   2048 B/op	      12 allocs/op  1234.56 logs/sec  50000 total_logs  40.52 duration_seconds

	fields := strings.Fields(line)
	if len(fields) < 10 {
		return nil
	}

	result := BenchmarkResult{
		Name:    benchmarkName,
		Metrics: make(map[string]float64),
	}

	// Parse metrics from the line
	for i := 5; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			metricName := fields[i]
			metricValue := fields[i+1]

			// Clean metric name (remove trailing characters)
			metricName = strings.TrimSuffix(metricName, "/sec")
			metricName = strings.TrimSuffix(metricName, "/op")
			metricName = strings.TrimSuffix(metricName, "_seconds")

			// Parse value
			if value, err := strconv.ParseFloat(metricValue, 64); err == nil {
				switch strings.ToLower(fields[i]) {
				case "logs/sec":
					result.LogsPerSecond = value
				case "total_logs":
					result.TotalLogs = value
				case "duration_seconds":
					result.Duration = value
				case "workers":
					result.Workers = value
				case "batch_size":
					result.BatchSize = value
				case "body_size_bytes":
					result.BodySize = value
				case "error_rate":
					result.ErrorRate = value
				case "mb/sec":
					result.ThroughputMBps = value
				default:
					result.Metrics[metricName] = value
				}
			}
		}
	}

	return &result
}

func getEnvironmentInfo() map[string]string {
	info := make(map[string]string)

	// Get Go version
	if cmd := exec.Command("go", "version"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			info["go_version"] = strings.TrimSpace(string(output))
		}
	}

	// Get system info
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["hostname"], _ = os.Hostname()

	return info
}

func generateSummary(results []BenchmarkResult) SummaryStats {
	if len(results) == 0 {
		return SummaryStats{}
	}

	var total, max, min float64
	max = 0
	min = float64(^uint(0) >> 1) // Max float64

	var bestConfig BenchmarkResult

	for _, result := range results {
		total += result.LogsPerSecond
		if result.LogsPerSecond > max {
			max = result.LogsPerSecond
			bestConfig = result
		}
		if result.LogsPerSecond < min {
			min = result.LogsPerSecond
		}
	}

	return SummaryStats{
		TotalBenchmarks: len(results),
		AvgThroughput:   total / float64(len(results)),
		MaxThroughput:   max,
		MinThroughput:   min,
		OptimalConfig:   bestConfig.Name,
	}
}

func generateRecommendations(results []BenchmarkResult) []string {
	recommendations := []string{}

	// Find best performing configuration
	var bestResult BenchmarkResult
	maxThroughput := 0.0

	for _, result := range results {
		if result.LogsPerSecond > maxThroughput {
			maxThroughput = result.LogsPerSecond
			bestResult = result
		}
	}

	if maxThroughput > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Best performance: %.2f logs/sec with %s", maxThroughput, bestResult.Name))
	}

	// Analyze worker count impact
	workerAnalysis := analyzeWorkerImpact(results)
	if len(workerAnalysis) > 0 {
		recommendations = append(recommendations, workerAnalysis...)
	}

	// Analyze batch size impact
	batchAnalysis := analyzeBatchSizeImpact(results)
	if len(batchAnalysis) > 0 {
		recommendations = append(recommendations, batchAnalysis...)
	}

	// General recommendations
	if maxThroughput < 1000 {
		recommendations = append(recommendations, "Consider increasing worker count or optimizing batch size for better throughput")
	}

	if maxThroughput > 10000 {
		recommendations = append(recommendations, "Excellent throughput! Monitor resource usage under load")
	}

	return recommendations
}

func analyzeWorkerImpact(results []BenchmarkResult) []string {
	// Group results by worker count
	workerGroups := make(map[float64][]BenchmarkResult)

	for _, result := range results {
		if result.Workers > 0 {
			workerGroups[result.Workers] = append(workerGroups[result.Workers], result)
		}
	}

	if len(workerGroups) < 2 {
		return []string{}
	}

	// Calculate average throughput per worker count
	workerThroughput := make(map[float64]float64)
	for workers, results := range workerGroups {
		var total float64
		for _, result := range results {
			total += result.LogsPerSecond
		}
		workerThroughput[workers] = total / float64(len(results))
	}

	// Find trend
	recommendations := []string{}

	if workerThroughput[8] > workerThroughput[4]*1.5 {
		recommendations = append(recommendations, "Increasing workers from 4 to 8 significantly improves throughput")
	}

	if workerThroughput[16] > workerThroughput[8]*1.2 {
		recommendations = append(recommendations, "Higher worker counts (16+) show good scaling")
	}

	return recommendations
}

func analyzeBatchSizeImpact(results []BenchmarkResult) []string {
	// Group results by batch size
	batchGroups := make(map[float64][]BenchmarkResult)

	for _, result := range results {
		if result.BatchSize > 0 {
			batchGroups[result.BatchSize] = append(batchGroups[result.BatchSize], result)
		}
	}

	if len(batchGroups) < 2 {
		return []string{}
	}

	recommendations := []string{}

	// Find optimal batch size
	var bestBatchSize float64
	var bestThroughput float64

	for batchSize, results := range batchGroups {
		var total float64
		for _, result := range results {
			total += result.LogsPerSecond
		}
		avgThroughput := total / float64(len(results))

		if avgThroughput > bestThroughput {
			bestThroughput = avgThroughput
			bestBatchSize = batchSize
		}
	}

	recommendations = append(recommendations,
		fmt.Sprintf("Optimal batch size appears to be %.0f for best throughput", bestBatchSize))

	return recommendations
}

func saveReport(report BenchmarkReport, filename string) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling report: %v\n", err)
		return
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		fmt.Printf("Error saving report: %v\n", err)
		return
	}

	fmt.Printf("Report saved to %s\n", filename)
}

func printSummary(report BenchmarkReport) {
	fmt.Print("\n" + strings.Repeat("=", 60) + "\n")
	fmt.Printf("BENCHMARK SUMMARY\n")
	fmt.Print(strings.Repeat("=", 60) + "\n")

	fmt.Printf("Timestamp: %s\n", report.Timestamp.Format(time.RFC3339))
	fmt.Printf("Total Benchmarks: %d\n", report.Summary.TotalBenchmarks)
	fmt.Printf("Average Throughput: %.2f logs/sec\n", report.Summary.AvgThroughput)
	fmt.Printf("Max Throughput: %.2f logs/sec\n", report.Summary.MaxThroughput)
	fmt.Printf("Min Throughput: %.2f logs/sec\n", report.Summary.MinThroughput)
	fmt.Printf("Optimal Config: %s\n", report.Summary.OptimalConfig)

	fmt.Printf("\nTOP 5 PERFORMERS:\n")
	// Sort results by throughput
	sortedResults := make([]BenchmarkResult, len(report.Results))
	copy(sortedResults, report.Results)

	// Simple bubble sort for demo
	for i := 0; i < len(sortedResults); i++ {
		for j := i + 1; j < len(sortedResults); j++ {
			if sortedResults[j].LogsPerSecond > sortedResults[i].LogsPerSecond {
				sortedResults[i], sortedResults[j] = sortedResults[j], sortedResults[i]
			}
		}
	}

	for i, result := range sortedResults {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. %s: %.2f logs/sec\n", i+1, result.Name, result.LogsPerSecond)
	}

	fmt.Printf("\nRECOMMENDATIONS:\n")
	for i, rec := range report.Recommendations {
		fmt.Printf("%d. %s\n", i+1, rec)
	}

	fmt.Print("\n" + strings.Repeat("=", 60) + "\n")
}
