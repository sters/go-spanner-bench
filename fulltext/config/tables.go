package config

import (
	"fmt"
	"strings"
)

// TableType represents different table types for benchmarking.
type TableType string

const (
	TableTypeBase      TableType = "base"
	TableTypeFulltext  TableType = "fulltext"
	TableTypeSubstring TableType = "substring"
	TableTypeNgrams    TableType = "ngrams"
	TableTypeAll       TableType = "all"
)

// TableConfig holds configuration for each table.
type TableConfig struct {
	Name            string
	Type            TableType
	CreateStatement string
	IndexStatements []string
	Description     string
}

// QueryConfig holds query configuration for benchmarking.
type QueryConfig struct {
	SelectAll        string
	SearchQuery      string
	SearchQueryParam string // Parameter for the search query
}

// TableDefinitions contains all table configurations.
var TableDefinitions = map[TableType]*TableConfig{
	TableTypeBase: {
		Name: "BenchBase",
		Type: TableTypeBase,
		CreateStatement: `CREATE TABLE BenchBase (
			UUID STRING(36) NOT NULL,
			Body STRING(MAX) NOT NULL
		) PRIMARY KEY (UUID)`,
		IndexStatements: []string{},
		Description:     "Base table without search index",
	},
	TableTypeFulltext: {
		Name: "BenchFulltext",
		Type: TableTypeFulltext,
		CreateStatement: `CREATE TABLE BenchFulltext (
			UUID STRING(36) NOT NULL,
			Body STRING(MAX) NOT NULL,
			BodyTokens TOKENLIST AS (TOKENIZE_FULLTEXT(Body)) HIDDEN
		) PRIMARY KEY (UUID)`,
		IndexStatements: []string{
			`CREATE SEARCH INDEX BenchFulltext_BodyTokens ON BenchFulltext(BodyTokens)`,
		},
		Description: "Table with FULLTEXT search index",
	},
	TableTypeSubstring: {
		Name: "BenchSubstring",
		Type: TableTypeSubstring,
		CreateStatement: `CREATE TABLE BenchSubstring (
			UUID STRING(36) NOT NULL,
			Body STRING(MAX) NOT NULL,
			BodyTokens TOKENLIST AS (TOKENIZE_SUBSTRING(Body)) HIDDEN
		) PRIMARY KEY (UUID)`,
		IndexStatements: []string{
			`CREATE SEARCH INDEX BenchSubstring_BodyTokens ON BenchSubstring(BodyTokens)`,
		},
		Description: "Table with SUBSTRING search index",
	},
	TableTypeNgrams: {
		Name: "BenchNgrams",
		Type: TableTypeNgrams,
		CreateStatement: `CREATE TABLE BenchNgrams (
			UUID STRING(36) NOT NULL,
			Body STRING(MAX) NOT NULL,
			BodyTokens TOKENLIST AS (TOKENIZE_NGRAMS(Body, ngram_size_min=>2, ngram_size_max=>3)) HIDDEN
		) PRIMARY KEY (UUID)`,
		IndexStatements: []string{
			`CREATE SEARCH INDEX BenchNgrams_BodyTokens ON BenchNgrams(BodyTokens)`,
		},
		Description: "Table with NGRAMS search index (2-3 grams)",
	},
}

// GetTableNames returns table names for the specified type.
func GetTableNames(tableType string) []string {
	tt := TableType(strings.ToLower(tableType))

	if tt == TableTypeAll {
		var names []string
		// Keep the order consistent
		for _, tableType := range []TableType{TableTypeBase, TableTypeFulltext, TableTypeSubstring, TableTypeNgrams} {
			names = append(names, TableDefinitions[tableType].Name)
		}

		return names
	}

	if config, ok := TableDefinitions[tt]; ok {
		return []string{config.Name}
	}

	return nil
}

// GetCreateStatements returns all CREATE statements for initialization.
func GetCreateStatements() []string {
	statements := make([]string, 0, len(TableDefinitions)*2)

	// Add CREATE TABLE statements
	for _, tableType := range []TableType{TableTypeBase, TableTypeFulltext, TableTypeSubstring, TableTypeNgrams} {
		config := TableDefinitions[tableType]
		statements = append(statements, config.CreateStatement)
		statements = append(statements, config.IndexStatements...)
	}

	return statements
}

// GetQueryForTable generates appropriate query for the table type.
func GetQueryForTable(tableType string, searchTerm string, limit int) string {
	tt := TableType(strings.ToLower(tableType))
	config, ok := TableDefinitions[tt]
	if !ok {
		return ""
	}

	if limit <= 0 {
		limit = 10
	}

	switch tt {
	case TableTypeBase:
		// Use LIKE for base table (no search index)
		return fmt.Sprintf("SELECT UUID, Body FROM %s WHERE Body LIKE '%%%s%%' LIMIT %d",
			config.Name, searchTerm, limit)
	case TableTypeFulltext:
		// Use SEARCH for FULLTEXT
		return fmt.Sprintf("SELECT UUID, Body FROM %s WHERE SEARCH(BodyTokens, '%s') LIMIT %d",
			config.Name, searchTerm, limit)
	case TableTypeSubstring:
		// Use SEARCH_SUBSTRING for SUBSTRING
		return fmt.Sprintf("SELECT UUID, Body FROM %s WHERE SEARCH_SUBSTRING(BodyTokens, '%s') LIMIT %d",
			config.Name, searchTerm, limit)
	case TableTypeNgrams:
		// Use SEARCH_NGRAMS for NGRAMS
		return fmt.Sprintf("SELECT UUID, Body FROM %s WHERE SEARCH_NGRAMS(BodyTokens, '%s') LIMIT %d",
			config.Name, searchTerm, limit)
	default:
		return ""
	}
}

// GetAllQueries returns queries for all table types (useful for comparison).
func GetAllQueries(searchTerm string, limit int) map[TableType]string {
	queries := make(map[TableType]string)
	for tableType := range TableDefinitions {
		queries[tableType] = GetQueryForTable(string(tableType), searchTerm, limit)
	}

	return queries
}

// GetScanQuery returns a full table scan query.
func GetScanQuery(tableType string, limit int) string {
	tt := TableType(strings.ToLower(tableType))
	config, ok := TableDefinitions[tt]
	if !ok {
		return ""
	}

	if limit <= 0 {
		limit = 100
	}

	return fmt.Sprintf("SELECT UUID, Body FROM %s LIMIT %d", config.Name, limit)
}

// ValidateTableType checks if a table type is valid.
func ValidateTableType(tableType string) bool {
	tt := TableType(strings.ToLower(tableType))
	if tt == TableTypeAll {
		return true
	}
	_, ok := TableDefinitions[tt]

	return ok
}

// GetTableDescription returns description for the table type.
func GetTableDescription(tableType string) string {
	tt := TableType(strings.ToLower(tableType))
	if config, ok := TableDefinitions[tt]; ok {
		return config.Description
	}

	return "Unknown table type"
}

// GetAvailableTableTypes returns all available table types.
func GetAvailableTableTypes() []string {
	return []string{
		string(TableTypeAll),
		string(TableTypeBase),
		string(TableTypeFulltext),
		string(TableTypeSubstring),
		string(TableTypeNgrams),
	}
}
