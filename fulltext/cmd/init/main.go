package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"

	"github.com/sters/go-spanner-bench/internal/logging"
)

//go:embed schema.sql
var schemaSQL string

var (
	projectID  = flag.String("project", "", "Google Cloud Project ID")
	instanceID = flag.String("instance", "", "Spanner Instance ID")
	databaseID = flag.String("database", "", "Spanner Database ID")
	logFile    = flag.String("log-file", "", "Log file path (optional)")
)

func main() {
	flag.Parse()

	// Setup logger
	logger, logFileHandle, err := logging.SetupLogger(*logFile)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}
	slog.SetDefault(logger)

	if *projectID == "" {
		*projectID = os.Getenv("SPANNER_PROJECT_ID")
		if *projectID == "" {
			*projectID = "test-project"
		}
	}
	if *instanceID == "" {
		*instanceID = os.Getenv("SPANNER_INSTANCE_ID")
		if *instanceID == "" {
			*instanceID = "test-instance"
		}
	}
	if *databaseID == "" {
		*databaseID = os.Getenv("SPANNER_DATABASE_ID")
		if *databaseID == "" {
			*databaseID = "test-database"
		}
	}

	ctx := context.Background()

	if err := createInstance(ctx, *projectID, *instanceID); err != nil {
		logger.Warn("Failed to create instance (may already exist)", "error", err)
	}

	// Check if database already exists
	if exists, err := databaseExists(ctx, *projectID, *instanceID, *databaseID); err != nil {
		logger.Error("Failed to check database existence", "error", err)
		if logFileHandle != nil {
			logFileHandle.Close()
		}
		os.Exit(1)
	} else if exists {
		logger.Info("Database already exists, skipping creation",
			"project", *projectID,
			"instance", *instanceID,
			"database", *databaseID)
		return
	}

	// Drop database if it exists (shouldn't happen due to check above, but kept for safety)
	if err := dropDatabase(ctx, *projectID, *instanceID, *databaseID); err != nil {
		logger.Warn("Failed to drop database (may not exist)", "error", err)
	}

	if err := createDatabase(ctx, *projectID, *instanceID, *databaseID); err != nil {
		logger.Error("Failed to create database", "error", err)
		if logFileHandle != nil {
			logFileHandle.Close()
		}
		os.Exit(1)
	}

	logger.Info("Successfully initialized Spanner environment",
		"project", *projectID,
		"instance", *instanceID,
		"database", *databaseID)
}

func createInstance(ctx context.Context, projectID, instanceID string) error {
	instanceAdmin, err := instance.NewInstanceAdminClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create instance admin client: %w", err)
	}
	defer instanceAdmin.Close()

	req := &instancepb.CreateInstanceRequest{
		Parent:     "projects/" + projectID,
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			Config:      fmt.Sprintf("projects/%s/instanceConfigs/emulator-config", projectID),
			DisplayName: instanceID,
			NodeCount:   1,
		},
	}

	op, err := instanceAdmin.CreateInstance(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for instance creation: %w", err)
	}

	return nil
}

func databaseExists(ctx context.Context, projectID, instanceID, databaseID string) (bool, error) {
	databaseAdmin, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to create database admin client: %w", err)
	}
	defer databaseAdmin.Close()

	dbName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectID, instanceID, databaseID)

	_, err = databaseAdmin.GetDatabase(ctx, &databasepb.GetDatabaseRequest{
		Name: dbName,
	})
	if err != nil {
		// If the error indicates the database doesn't exist, return false
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		// Other errors should be returned
		return false, fmt.Errorf("failed to get database: %w", err)
	}

	// Database exists
	return true, nil
}

func dropDatabase(ctx context.Context, projectID, instanceID, databaseID string) error {
	databaseAdmin, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database admin client: %w", err)
	}
	defer databaseAdmin.Close()

	dbName := fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectID, instanceID, databaseID)

	if err := databaseAdmin.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: dbName,
	}); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	return nil
}

func createDatabase(ctx context.Context, projectID, instanceID, databaseID string) error {
	databaseAdmin, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database admin client: %w", err)
	}
	defer databaseAdmin.Close()

	op, err := databaseAdmin.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          fmt.Sprintf("projects/%s/instances/%s", projectID, instanceID),
		CreateStatement: fmt.Sprintf("CREATE DATABASE `%s`", databaseID),
		ExtraStatements: parseSchemaStatements(schemaSQL),
	})
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for database creation: %w", err)
	}

	slog.Info("Created database with tables",
		"database", databaseID,
		"tables", []string{"BenchBase", "BenchFulltext", "BenchSubstring", "BenchNgrams"})

	return nil
}

// parseSchemaStatements parses the schema SQL and returns individual statements
func parseSchemaStatements(schema string) []string {
	var statements []string
	var currentStatement strings.Builder

	lines := strings.Split(schema, "\n")
	for _, line := range lines {
		// Skip comment lines and empty lines
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "--") {
			continue
		}

		currentStatement.WriteString(line)
		currentStatement.WriteString("\n")

		// Check if this line ends with a semicolon (end of statement)
		if strings.HasSuffix(trimmedLine, ";") {
			// Remove the trailing semicolon and any trailing whitespace
			statement := strings.TrimSpace(currentStatement.String())
			statement = strings.TrimSuffix(statement, ";")
			if statement != "" {
				statements = append(statements, statement)
			}
			currentStatement.Reset()
		}
	}

	// Add any remaining statement
	if currentStatement.Len() > 0 {
		statement := strings.TrimSpace(currentStatement.String())
		statement = strings.TrimSuffix(statement, ";")
		if statement != "" {
			statements = append(statements, statement)
		}
	}

	return statements
}
