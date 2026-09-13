package core

import (
	"context"
	"time"
)

// EngineType represents supported database engines.
type EngineType string

const (
	EnginePostgres EngineType = "postgres" // CockroachDB, PostgreSQL, Supabase, Neon
	EngineMongo    EngineType = "mongo"    // MongoDB Atlas, Local MongoDB
)

// MigrationMode represents what components to replicate.
type MigrationMode string

const (
	ModeFull       MigrationMode = "full"        // Schema + Data
	ModeSchemaOnly MigrationMode = "schema-only" // Only Tables / Collections & Indexes
	ModeDataOnly   MigrationMode = "data-only"   // Only Data (Tables must already exist)
)

// ColumnMeta holds metadata for a database column.
type ColumnMeta struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value,omitempty"`
	IsPrimaryKey bool   `json:"is_primary_key"`
}

// IndexMeta represents an index on a table or collection.
type IndexMeta struct {
	Name      string                 `json:"name"`
	Columns   []string               `json:"columns"`
	IsUnique  bool                   `json:"is_unique"`
	RawDDL    string                 `json:"raw_ddl,omitempty"`
	ExtraOpts map[string]interface{} `json:"extra_opts,omitempty"`
}

// CollectionMeta represents a table or MongoDB collection to be migrated.
type CollectionMeta struct {
	Name          string       `json:"name"`
	SchemaName    string       `json:"schema_name,omitempty"` // For Postgres / Cockroach (e.g. "public")
	EstimatedRows int64        `json:"estimated_rows"`
	Columns       []ColumnMeta `json:"columns,omitempty"`
	Indexes       []IndexMeta  `json:"indexes,omitempty"`
	ForeignKeys   []string     `json:"foreign_keys,omitempty"` // Deferred ALTER TABLE ADD CONSTRAINT statements
	DDL           string       `json:"ddl,omitempty"`          // Generated CREATE TABLE statement
}

// MigrationOptions configures the migration execution.
type MigrationOptions struct {
	Mode         MigrationMode `json:"mode"`
	BatchSize    int           `json:"batch_size"`
	Concurrency  int           `json:"concurrency"`
	DropExisting bool          `json:"drop_existing"`
	Truncate     bool          `json:"truncate"`
	SelectedOnly []string      `json:"selected_only,omitempty"`
}

// DefaultOptions returns standard production defaults.
func DefaultOptions() MigrationOptions {
	return MigrationOptions{
		Mode:         ModeFull,
		BatchSize:    2500,
		Concurrency:  4,
		DropExisting: false,
		Truncate:     false,
	}
}

// ProgressEvent is emitted as rows are streamed.
type ProgressEvent struct {
	CollectionName string
	TotalRows      int64
	CopiedRows     int64
	SpeedRowsPerS  float64
	IsCompleted    bool
	Err            error
}

// AuditResult holds verification status for a single table/collection.
type AuditResult struct {
	Name        string        `json:"name"`
	SourceCount int64         `json:"source_count"`
	TargetCount int64         `json:"target_count"`
	Matched     bool          `json:"matched"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

// Driver defines the contract every database adapter must fulfill.
type Driver interface {
	// Connect establishes database connection.
	Connect(ctx context.Context, uri string) error

	// Ping tests network reachability and authentication.
	Ping(ctx context.Context) error

	// Inspect retrieves metadata, table/collection names and row counts.
	Inspect(ctx context.Context) ([]CollectionMeta, error)

	// CreateSchema applies table DDL, schemas and indexes to the target.
	CreateSchema(ctx context.Context, meta CollectionMeta, dropExisting bool) error

	// StreamCopy copies rows from source to target collection with progress updates.
	StreamCopy(ctx context.Context, target Driver, meta CollectionMeta, batchSize int, progress chan<- ProgressEvent) error

	// VerifyCounts returns the verified row/doc count on this driver.
	VerifyCounts(ctx context.Context, collectionName string) (int64, error)

	// PostMigrationFixups performs sequence synchronization, constraint re-enabling, etc.
	PostMigrationFixups(ctx context.Context, collections []CollectionMeta) error

	// Engine returns the engine type of this driver.
	Engine() EngineType

	// DialectInfo returns human-readable version/dialect (e.g. "CockroachDB v24.2", "PostgreSQL 16", "MongoDB 7.0").
	DialectInfo() string

	// Close terminates open connections.
	Close(ctx context.Context) error
}
