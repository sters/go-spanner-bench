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
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"github.com/montanaflynn/stats"
	"google.golang.org/api/iterator"

	"github.com/sters/go-spanner-bench/fulltext/config"
	"github.com/sters/go-spanner-bench/internal/logging"
)

var (
	projectID   = flag.String("project", "", "Spanner Project ID")
	instanceID  = flag.String("instance", "", "Spanner Instance ID")
	databaseID  = flag.String("database", "", "Spanner Database ID")
	bqProjectID = flag.String("bq-project", "", "BigQuery Project ID")
	bqQuery     = flag.String("bq-query", "", "BigQuery SQL Query")
	batchSize   = flag.Int("batch-size", 100, "Batch size for inserts")
	maxRows     = flag.Int("max-rows", 0, "Maximum rows to insert (0 for no limit)")
	targetTable = flag.String("table", "all", "Target table: all, base, fulltext, substring, ngrams")

	// Logging options.
	logFile     = flag.String("log-file", "", "Log file path (optional)")
	resultsFile = flag.String("results-file", "", "Results output file path (optional)")
)

type benchmarkResult struct {
	duration time.Duration
}

func loadConfig() {
	if *projectID == "" {
		*projectID = os.Getenv("SPANNER_PROJECT_ID")
	}
	if *instanceID == "" {
		*instanceID = os.Getenv("SPANNER_INSTANCE_ID")
	}
	if *databaseID == "" {
		*databaseID = os.Getenv("SPANNER_DATABASE_ID")
	}
	if *bqProjectID == "" {
		*bqProjectID = os.Getenv("BQ_PROJECT_ID")
	}
}

func validateConfig(logger *slog.Logger) {
	if *projectID == "" || *instanceID == "" || *databaseID == "" {
		logger.Error("Spanner connection info required",
			"project", *projectID,
			"instance", *instanceID,
			"database", *databaseID)
		os.Exit(1)
	}
	if *bqProjectID == "" || *bqQuery == "" {
		logger.Error("BigQuery info required",
			"bq-project", *bqProjectID,
			"bq-query", *bqQuery)
		os.Exit(1)
	}
}

func setupClients(ctx context.Context, logger *slog.Logger) (*bigquery.Client, *spanner.Client) {
	bqClient, err := bigquery.NewClient(ctx, *bqProjectID)
	if err != nil {
		logger.Error("Failed to create BigQuery client", "error", err)
		os.Exit(1)
	}

	spannerDB := fmt.Sprintf("projects/%s/instances/%s/databases/%s",
		*projectID, *instanceID, *databaseID)
	spannerClient, err := spanner.NewClient(ctx, spannerDB)
	if err != nil {
		bqClient.Close()
		logger.Error("Failed to create Spanner client", "error", err)
		os.Exit(1)
	}

	return bqClient, spannerClient
}

func processBatch(ctx context.Context, client *spanner.Client, mutations []*spanner.Mutation, tables []string) (benchmarkResult, int, error) {
	start := time.Now()
	_, err := client.Apply(ctx, mutations)
	if err != nil {
		return benchmarkResult{}, 0, fmt.Errorf("failed to apply mutations: %w", err)
	}

	return benchmarkResult{duration: time.Since(start)}, len(mutations) / len(tables), nil
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

	loadConfig()
	validateConfig(logger)

	ctx := context.Background()

	bqClient, spannerClient := setupClients(ctx, logger)
	defer bqClient.Close()
	defer spannerClient.Close()

	logger.Info("Executing BigQuery query", "query", *bqQuery)
	q := bqClient.Query(*bqQuery)
	it, err := q.Read(ctx)
	if err != nil {
		logger.Error("Failed to execute BigQuery query", "error", err)
		os.Exit(1)
	}

	// Determine target tables using config
	tables := config.GetTableNames(*targetTable)
	if len(tables) == 0 {
		logger.Error("Invalid table option",
			"table", *targetTable,
			"available", config.GetAvailableTableTypes())
		os.Exit(1)
	}
	logger.Info("Target tables", "tables", tables)

	var results []benchmarkResult
	mutations := make([]*spanner.Mutation, 0, *batchSize*len(tables))
	totalRows := 0

	for {
		var row []bigquery.Value
		err := it.Next(&row)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logger.Error("Failed to read row", "error", err)
			os.Exit(1)
		}

		if len(row) == 0 {
			continue
		}

		bodyValue := fmt.Sprintf("%v", row[0])
		if len(row) > 1 {
			bodyValue = fmt.Sprintf("%v", row[1])
		}

		id := uuid.New().String()

		// Insert into all target tables
		for _, table := range tables {
			mutations = append(mutations, spanner.Insert(
				table,
				[]string{"UUID", "Body"},
				[]interface{}{id, bodyValue},
			))
		}

		if len(mutations) >= *batchSize*len(tables) {
			if result, rows, err := processBatch(ctx, spannerClient, mutations, tables); err != nil {
				logger.Warn("Failed to insert batch", "error", err)
			} else {
				results = append(results, result)
				totalRows += rows
				logger.Info("Inserted batch",
					"rows_per_table", rows,
					"total_rows", totalRows)
			}
			mutations = mutations[:0]

			if *maxRows > 0 && totalRows >= *maxRows {
				break
			}
		}
	}

	if len(mutations) > 0 {
		if result, rows, err := processBatch(ctx, spannerClient, mutations, tables); err != nil {
			logger.Warn("Failed to insert final batch", "error", err)
		} else {
			results = append(results, result)
			totalRows += rows
			logger.Info("Inserted final batch",
				"rows_per_table", rows,
				"total_rows", totalRows)
		}
	}

	printStats(results, resultsFileHandle, logger)
	logger.Info("Total rows inserted", "count", totalRows)
}

func printStats(results []benchmarkResult, resultsFile *os.File, logger *slog.Logger) {
	if len(results) == 0 {
		logger.Warn("No results to display")

		return
	}

	durations := make([]float64, len(results))
	for i, r := range results {
		durations[i] = float64(r.duration.Milliseconds())
	}

	sort.Float64s(durations)

	minVal, _ := stats.Min(durations)
	maxVal, _ := stats.Max(durations)
	mean, _ := stats.Mean(durations)
	p50, _ := stats.Percentile(durations, 50)
	p75, _ := stats.Percentile(durations, 75)
	p90, _ := stats.Percentile(durations, 90)

	fmt.Println("\n=== Insert Benchmark Results (ms) ===")
	fmt.Printf("Batches:  %d\n", len(results))
	fmt.Printf("Min:      %.2f ms\n", minVal)
	fmt.Printf("Max:      %.2f ms\n", maxVal)
	fmt.Printf("Avg:      %.2f ms\n", mean)
	fmt.Printf("P50:      %.2f ms\n", p50)
	fmt.Printf("P75:      %.2f ms\n", p75)
	fmt.Printf("P90:      %.2f ms\n", p90)

	// Log structured results
	logger.Info("Insert benchmark completed",
		"batches", len(results),
		"min_ms", minVal,
		"max_ms", maxVal,
		"avg_ms", mean,
		"p50_ms", p50,
		"p75_ms", p75,
		"p90_ms", p90)

	// Write JSON results to file if specified
	if resultsFile != nil {
		resultsData := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"operation": "insert",
			"batches":   len(results),
			"latency": map[string]float64{
				"minMs": minVal,
				"maxMs": maxVal,
				"avgMs": mean,
				"p50Ms": p50,
				"p75Ms": p75,
				"p90Ms": p90,
			},
		}
		if err := logging.WriteJSONResults(resultsFile, resultsData); err != nil {
			logger.Error("Failed to write JSON results", "error", err)
		}
	}
}
