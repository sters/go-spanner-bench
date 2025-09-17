package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SetupLogger configures slog with file output if specified.
func SetupLogger(logFile string) (*slog.Logger, *os.File, error) {
	var writers []io.Writer
	var logFileHandle *os.File

	writers = append(writers, os.Stdout)

	if logFile != "" {
		dir := filepath.Dir(logFile)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
			}
		}

		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file: %w", err)
		}
		logFileHandle = f
		writers = append(writers, f)
	}

	multiWriter := io.MultiWriter(writers...)

	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	}

	handler := slog.NewJSONHandler(multiWriter, opts)
	logger := slog.New(handler)

	return logger, logFileHandle, nil
}

// SetupResultsFile creates/opens a file for writing benchmark results.
func SetupResultsFile(resultsFile string) (*os.File, error) {
	if resultsFile == "" {
		return nil, nil //nolint:nilnil // intentionally return nil,nil for empty file
	}

	dir := filepath.Dir(resultsFile)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create results directory: %w", err)
		}
	}

	f, err := os.OpenFile(resultsFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open results file: %w", err)
	}

	return f, nil
}

// WriteResults writes benchmark results to both stdout and optionally to a file.
func WriteResults(resultsFile *os.File, results string) {
	fmt.Print(results)

	if resultsFile != nil {
		if _, err := resultsFile.WriteString(results); err != nil {
			fmt.Printf("Failed to write results: %v\n", err)
		}
		if _, err := resultsFile.WriteString("\n"); err != nil {
			fmt.Printf("Failed to write newline: %v\n", err)
		}
	}
}

// WriteJSONResults writes benchmark results as JSON to a file.
func WriteJSONResults(resultsFile *os.File, results interface{}) error {
	if resultsFile == nil {
		return nil
	}

	encoder := json.NewEncoder(resultsFile)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(results); err != nil {
		return fmt.Errorf("failed to encode results: %w", err)
	}

	return nil
}

// GenerateTimestampedPath generates a timestamped path for output files.
// If the input path is empty, returns empty string.
// If the input path contains "logs" or "results", it will be placed under fulltext/outputs/yyyymmddhhmmss/.
func GenerateTimestampedPath(originalPath string) string {
	if originalPath == "" {
		return ""
	}

	dir := filepath.Dir(originalPath)
	basename := filepath.Base(originalPath)

	timestamp := time.Now().Format("20060102150405")

	if dir == "logs" || dir == "results" || dir == "./logs" || dir == "./results" {
		return filepath.Join("fulltext", "outputs", timestamp, dir, basename)
	}

	if strings.HasPrefix(originalPath, "logs/") || strings.HasPrefix(originalPath, "results/") {
		return filepath.Join("fulltext", "outputs", timestamp, originalPath)
	}

	return originalPath
}
