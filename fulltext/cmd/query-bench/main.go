package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/montanaflynn/stats"
	"google.golang.org/api/iterator"

	"github.com/sters/go-spanner-bench/fulltext/config"
	"github.com/sters/go-spanner-bench/internal/logging"
)

var (
	projectID  = flag.String("project", "", "Spanner Project ID")
	instanceID = flag.String("instance", "", "Spanner Instance ID")
	databaseID = flag.String("database", "", "Spanner Database ID")
	tableType  = flag.String("table", "fulltext", "Table type: base, fulltext, substring, ngrams")
	searchTerm = flag.String("search", "test", "Search term for the query")
	queryType  = flag.String("query-type", "search", "Query type: search, scan")
	queryLimit = flag.Int("limit", 10, "Query result limit")
	iterations = flag.Int("iterations", 100, "Number of iterations per goroutine")
	goroutines = flag.Int("goroutines", 10, "Number of concurrent goroutines")
	timeout    = flag.Duration("timeout", 30*time.Second, "Query timeout")

	// Optional: custom query override.
	customQuery = flag.String("custom-query", "", "Custom SQL query (overrides table/search settings)")

	// Logging options.
	logFile     = flag.String("log-file", "", "Log file path (optional)")
	resultsFile = flag.String("results-file", "", "Results output file path (optional)")
)

type queryResult struct {
	duration time.Duration
	err      error
}

func main() {
	flag.Parse()

	// Generate timestamped paths for logs and results
	timestampedLogFile := logging.GenerateTimestampedPath(*logFile)
	timestampedResultsFile := logging.GenerateTimestampedPath(*resultsFile)

	// Setup logger
	logger, logFileHandle, err := logging.SetupLogger(timestampedLogFile)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}
	slog.SetDefault(logger)

	// Setup results file
	resultsFileHandle, err := logging.SetupResultsFile(timestampedResultsFile)
	if err != nil {
		logger.Error("Failed to setup results file", "error", err)
		if logFileHandle != nil {
			logFileHandle.Close()
		}
		os.Exit(1)
	}
	if resultsFileHandle != nil {
		defer func() {
			if err := resultsFileHandle.Close(); err != nil {
				logger.Error("Failed to close results file", "error", err)
			}
		}()
	}

	if *projectID == "" {
		*projectID = os.Getenv("SPANNER_PROJECT_ID")
	}
	if *instanceID == "" {
		*instanceID = os.Getenv("SPANNER_INSTANCE_ID")
	}
	if *databaseID == "" {
		*databaseID = os.Getenv("SPANNER_DATABASE_ID")
	}

	if *projectID == "" || *instanceID == "" || *databaseID == "" {
		logger.Error("Spanner connection info required",
			"project", *projectID,
			"instance", *instanceID,
			"database", *databaseID)
		os.Exit(1)
	}

	// Determine the query to execute
	var query string
	if *customQuery != "" {
		query = *customQuery
		logger.Info("Using custom query", "query", query)
	} else {
		// Validate table type
		if !config.ValidateTableType(*tableType) {
			logger.Error("Invalid table type",
				"table", *tableType,
				"available", config.GetAvailableTableTypes())
			os.Exit(1)
		}

		// Generate query based on query type
		switch *queryType {
		case "search":
			query = config.GetQueryForTable(*tableType, *searchTerm, *queryLimit)
		case "scan":
			query = config.GetScanQuery(*tableType, *queryLimit)
		default:
			logger.Error("Invalid query type",
				"type", *queryType,
				"available", []string{"search", "scan"})
			os.Exit(1)
		}

		if query == "" {
			logger.Error("Failed to generate query", "table", *tableType)
			os.Exit(1)
		}
	}

	ctx := context.Background()

	spannerDB := fmt.Sprintf("projects/%s/instances/%s/databases/%s",
		*projectID, *instanceID, *databaseID)
	spannerClient, err := spanner.NewClient(ctx, spannerDB)
	if err != nil {
		logger.Error("Failed to create Spanner client", "error", err)
		os.Exit(1)
	}
	defer spannerClient.Close()

	logger.Info("Starting benchmark",
		"goroutines", *goroutines,
		"iterations", *iterations,
		"table_type", *tableType,
		"query_type", *queryType,
		"search_term", *searchTerm,
		"query", query)

	results := make(chan queryResult, *goroutines**iterations)
	var wg sync.WaitGroup

	startTime := time.Now()

	for g := range *goroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			runQueries(ctx, spannerClient, goroutineID, query, results, logger)
		}(g)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allResults := make([]queryResult, 0, *goroutines**iterations)
	successCount := 0
	errorCount := 0

	for r := range results {
		allResults = append(allResults, r)
		if r.err != nil {
			errorCount++
			logger.Warn("Query error", "error", r.err)
		} else {
			successCount++
		}
	}

	totalDuration := time.Since(startTime)

	printStats(allResults, successCount, errorCount, totalDuration, resultsFileHandle, logger)
}

func runQueries(ctx context.Context, client *spanner.Client, goroutineID int, query string, results chan<- queryResult, logger *slog.Logger) {
	for i := range *iterations {
		result := executeQuery(ctx, client, query)
		results <- result

		if i > 0 && i%10 == 0 {
			logger.Info("Progress",
				"goroutine", goroutineID,
				"completed", i,
				"total", *iterations)
		}
	}
}

func executeQuery(ctx context.Context, client *spanner.Client, query string) queryResult {
	queryCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	start := time.Now()
	stmt := spanner.Statement{SQL: query}

	rowCount := 0
	err := client.Single().Query(queryCtx, stmt).Do(func(_ *spanner.Row) error {
		rowCount++

		return nil
	})

	duration := time.Since(start)

	if err != nil && !errors.Is(err, iterator.Done) {
		return queryResult{duration: duration, err: err}
	}

	return queryResult{duration: duration, err: nil}
}

// BenchmarkResults represents the benchmark results in a structured format.
type BenchmarkResults struct {
	Timestamp       string            `json:"timestamp"`
	TableType       string            `json:"tableType"`
	QueryType       string            `json:"queryType"`
	SearchTerm      string            `json:"searchTerm,omitempty"`
	Query           string            `json:"query"`
	TotalQueries    int               `json:"totalQueries"`
	Successful      int               `json:"successful"`
	Failed          int               `json:"failed"`
	SuccessRate     float64           `json:"successRate"`
	DurationSeconds float64           `json:"durationSeconds"`
	QPS             float64           `json:"qps"`
	Latency         LatencyStatistics `json:"latency"`
	Config          BenchmarkConfig   `json:"config"`
}

// LatencyStatistics represents latency statistics.
type LatencyStatistics struct {
	MinMS float64 `json:"minMs"`
	MaxMS float64 `json:"maxMs"`
	AvgMS float64 `json:"avgMs"`
	P50MS float64 `json:"p50Ms"`
	P75MS float64 `json:"p75Ms"`
	P90MS float64 `json:"p90Ms"`
	P95MS float64 `json:"p95Ms,omitempty"`
	P99MS float64 `json:"p99Ms,omitempty"`
}

// BenchmarkConfig represents the benchmark configuration.
type BenchmarkConfig struct {
	Goroutines int           `json:"goroutines"`
	Iterations int           `json:"iterations"`
	Timeout    time.Duration `json:"timeout"`
	Limit      int           `json:"limit"`
}

func printStats(results []queryResult, successCount, errorCount int, totalDuration time.Duration, resultsFile *os.File, logger *slog.Logger) {
	if len(results) == 0 {
		logger.Warn("No results to display")

		return
	}

	var successDurations []float64
	for _, r := range results {
		if r.err == nil {
			successDurations = append(successDurations, float64(r.duration.Milliseconds()))
		}
	}

	if len(successDurations) == 0 {
		logger.Error("All queries failed")

		return
	}

	sort.Float64s(successDurations)

	minVal, _ := stats.Min(successDurations)
	maxVal, _ := stats.Max(successDurations)
	mean, _ := stats.Mean(successDurations)
	p50, _ := stats.Percentile(successDurations, 50)
	p75, _ := stats.Percentile(successDurations, 75)
	p90, _ := stats.Percentile(successDurations, 90)
	p95, _ := stats.Percentile(successDurations, 95)
	p99, _ := stats.Percentile(successDurations, 99)

	totalQueries := *goroutines * *iterations
	qps := float64(successCount) / totalDuration.Seconds()
	successRate := float64(successCount) * 100 / float64(totalQueries)

	// Determine the query that was executed
	query := *customQuery
	if query == "" {
		query = config.GetQueryForTable(*tableType, *searchTerm, *queryLimit)
	}

	// Create structured results
	benchResults := BenchmarkResults{
		Timestamp:       time.Now().Format(time.RFC3339),
		TableType:       *tableType,
		QueryType:       *queryType,
		SearchTerm:      *searchTerm,
		Query:           query,
		TotalQueries:    totalQueries,
		Successful:      successCount,
		Failed:          errorCount,
		SuccessRate:     successRate,
		DurationSeconds: totalDuration.Seconds(),
		QPS:             qps,
		Latency: LatencyStatistics{
			MinMS: minVal,
			MaxMS: maxVal,
			AvgMS: mean,
			P50MS: p50,
			P75MS: p75,
			P90MS: p90,
			P95MS: p95,
			P99MS: p99,
		},
		Config: BenchmarkConfig{
			Goroutines: *goroutines,
			Iterations: *iterations,
			Timeout:    *timeout,
			Limit:      *queryLimit,
		},
	}

	// Log structured results
	logger.Info("Benchmark completed",
		"total_queries", totalQueries,
		"successful", successCount,
		"failed", errorCount,
		"success_rate", fmt.Sprintf("%.1f%%", successRate),
		"duration_seconds", totalDuration.Seconds(),
		"qps", qps,
		"min_ms", minVal,
		"max_ms", maxVal,
		"avg_ms", mean,
		"p50_ms", p50,
		"p75_ms", p75,
		"p90_ms", p90,
	)

	// Format results for console display
	var sb strings.Builder
	sb.WriteString("\n=== Query Benchmark Results ===\n")
	sb.WriteString(fmt.Sprintf("Total Queries:    %d\n", totalQueries))
	sb.WriteString(fmt.Sprintf("Successful:       %d (%.1f%%)\n", successCount, successRate))
	sb.WriteString(fmt.Sprintf("Failed:           %d\n", errorCount))
	sb.WriteString(fmt.Sprintf("Total Duration:   %.2f s\n", totalDuration.Seconds()))
	sb.WriteString(fmt.Sprintf("QPS:              %.2f\n", qps))
	sb.WriteString("\n=== Latency Statistics (ms) ===\n")
	sb.WriteString(fmt.Sprintf("Min:              %.2f ms\n", minVal))
	sb.WriteString(fmt.Sprintf("Max:              %.2f ms\n", maxVal))
	sb.WriteString(fmt.Sprintf("Avg:              %.2f ms\n", mean))
	sb.WriteString(fmt.Sprintf("P50:              %.2f ms\n", p50))
	sb.WriteString(fmt.Sprintf("P75:              %.2f ms\n", p75))
	sb.WriteString(fmt.Sprintf("P90:              %.2f ms\n", p90))
	sb.WriteString(fmt.Sprintf("P95:              %.2f ms\n", p95))
	sb.WriteString(fmt.Sprintf("P99:              %.2f ms\n", p99))

	// Always print to console
	fmt.Print(sb.String())

	// Write JSON results to file if specified
	if resultsFile != nil {
		if err := logging.WriteJSONResults(resultsFile, benchResults); err != nil {
			logger.Error("Failed to write JSON results", "error", err)
		}
	}
}
