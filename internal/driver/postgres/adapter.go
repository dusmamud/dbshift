package postgres

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/dusmamud/dbshift/internal/core"
)

// Driver implements core.Driver for CockroachDB, PostgreSQL, Supabase, and Neon.
type Driver struct {
	pool        *pgxpool.Pool
	uri         string
	dialect     string
	isCockroach bool
	mu          sync.RWMutex
}

// NewDriver creates an uninitialized Postgres driver.
func NewDriver() *Driver {
	return &Driver{}
}

// Engine returns core.EnginePostgres.
func (d *Driver) Engine() core.EngineType {
	return core.EnginePostgres
}

// DialectInfo returns the detected database dialect and version.
func (d *Driver) DialectInfo() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.dialect != "" {
		return d.dialect
	}
	return "PostgreSQL (wire-compatible)"
}

// Connect initializes connection pool with pgx.
func (d *Driver) Connect(ctx context.Context, uri string) error {
	d.uri = uri
	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return core.NewError(core.EnginePostgres, "Connect", "invalid connection URI", err)
	}

	// Performance tuning for migration
	config.MaxConns = 16
	config.MinConns = 2
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return core.NewError(core.EnginePostgres, "Connect", "failed to create connection pool", err)
	}

	d.pool = pool

	// Detect dialect (CockroachDB vs PostgreSQL)
	var versionStr string
	if err := pool.QueryRow(ctx, "SELECT version()").Scan(&versionStr); err == nil {
		d.mu.Lock()
		if strings.Contains(strings.ToLower(versionStr), "cockroachdb") {
			d.isCockroach = true
			parts := strings.Split(versionStr, " ")
			if len(parts) >= 3 {
				d.dialect = "CockroachDB " + parts[2]
			} else {
				d.dialect = "CockroachDB Cloud"
			}
		} else if strings.Contains(strings.ToLower(versionStr), "supabase") {
			d.dialect = "Supabase PostgreSQL"
		} else if strings.Contains(strings.ToLower(versionStr), "neon") {
			d.dialect = "Neon Serverless Postgres"
		} else {
			parts := strings.Split(versionStr, ",")
			d.dialect = parts[0]
		}
		d.mu.Unlock()
	}

	return nil
}

// Ping checks if connection is alive.
func (d *Driver) Ping(ctx context.Context) error {
	if d.pool == nil {
		return core.NewError(core.EnginePostgres, "Ping", "connection pool is not initialized", nil)
	}
	var one int
	err := d.pool.QueryRow(ctx, "SELECT 1").Scan(&one)
	if err != nil {
		return core.NewError(core.EnginePostgres, "Ping", "ping query failed", err)
	}
	return nil
}

// Inspect discovers all user tables, columns, indexes, and row count estimates.
func (d *Driver) Inspect(ctx context.Context) ([]core.CollectionMeta, error) {
	if d.pool == nil {
		return nil, core.NewError(core.EnginePostgres, "Inspect", "not connected", nil)
	}

	// 1. Fetch all user tables in public schema
	tableQuery := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' 
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name;
	`
	rows, err := d.pool.Query(ctx, tableQuery)
	if err != nil {
		return nil, core.NewError(core.EnginePostgres, "Inspect", "failed to list tables", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		// Filter out internal migration tables if any
		if !strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "flyway_") {
			tableNames = append(tableNames, name)
		}
	}

	var results []core.CollectionMeta

	for _, tbl := range tableNames {
		meta := core.CollectionMeta{
			Name:       tbl,
			SchemaName: "public",
		}

		// 2. Estimate row count
		var estRows int64
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdentifier(tbl))
		_ = d.pool.QueryRow(ctx, countQuery).Scan(&estRows)
		meta.EstimatedRows = estRows

		// 3. CockroachDB Native DDL Extraction
		if d.isCockroach {
			var tblName, ddlStmt string
			showQuery := fmt.Sprintf(`SHOW CREATE TABLE %s`, quoteIdentifier(tbl))
			if err := d.pool.QueryRow(ctx, showQuery).Scan(&tblName, &ddlStmt); err == nil && ddlStmt != "" {
				if !strings.Contains(strings.ToUpper(ddlStmt), "IF NOT EXISTS") {
					ddlStmt = strings.Replace(ddlStmt, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)
				}
				cleanDDL, fks := separateForeignKeys(tbl, ddlStmt)
				meta.DDL = cleanDDL
				meta.ForeignKeys = fks
			}
		}

		// 4. Fetch columns reliably
		colQuery := `
			SELECT column_name, data_type, is_nullable, COALESCE(column_default, '')
			FROM information_schema.columns
			WHERE table_name = $1 AND (table_schema = 'public' OR table_schema = current_schema())
			ORDER BY ordinal_position;
		`
		colRows, err := d.pool.Query(ctx, colQuery, tbl)
		if err == nil {
			var cols []core.ColumnMeta
			for colRows.Next() {
				var colName, dType, isNullStr, colDef string
				if err := colRows.Scan(&colName, &dType, &isNullStr, &colDef); err == nil {
					cols = append(cols, core.ColumnMeta{
						Name:         colName,
						DataType:     dType,
						IsNullable:   (isNullStr == "YES"),
						DefaultValue: colDef,
					})
				}
			}
			colRows.Close()
			meta.Columns = cols
		}

		// Fallback: If information_schema returned 0 columns, inspect via LIMIT 0
		if len(meta.Columns) == 0 {
			sampleQuery := fmt.Sprintf(`SELECT * FROM %s LIMIT 0`, quoteIdentifier(tbl))
			if sampleRows, err := d.pool.Query(ctx, sampleQuery); err == nil {
				for _, fd := range sampleRows.FieldDescriptions() {
					meta.Columns = append(meta.Columns, core.ColumnMeta{
						Name:     string(fd.Name),
						DataType: "TEXT",
					})
				}
				sampleRows.Close()
			}
		}

		// 5. Fetch Index DDLs (for standard PostgreSQL)
		if !d.isCockroach {
			idxQuery := `
				SELECT indexname, indexdef
				FROM pg_indexes
				WHERE schemaname = current_schema() AND tablename = $1
				  AND indexname NOT LIKE '%_pkey';
			`
			idxRows, err := d.pool.Query(ctx, idxQuery, tbl)
			if err == nil {
				var indexes []core.IndexMeta
				for idxRows.Next() {
					var idxName, idxDef string
					if err := idxRows.Scan(&idxName, &idxDef); err == nil {
						indexes = append(indexes, core.IndexMeta{
							Name:   idxName,
							RawDDL: idxDef,
						})
					}
				}
				idxRows.Close()
				meta.Indexes = indexes
			}
		}

		// 6. Generate DDL if Cockroach SHOW CREATE TABLE wasn't available
		if meta.DDL == "" {
			meta.DDL = d.generateCreateTableDDL(meta)
		}

		results = append(results, meta)
	}

	return results, nil
}

// generateCreateTableDDL produces a clean CREATE TABLE statement compatible with CockroachDB and Postgres.
func (d *Driver) generateCreateTableDDL(meta core.CollectionMeta) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", quoteIdentifier(meta.Name)))

	var pks []string
	var colDefs []string

	for _, col := range meta.Columns {
		def := fmt.Sprintf("    %s %s", quoteIdentifier(col.Name), mapPgType(col.DataType))

		// If CockroachDB / Postgres serial sequence
		if strings.Contains(strings.ToLower(col.DefaultValue), "nextval") {
			// keep serial / generated
		} else if col.DefaultValue != "" && !strings.Contains(col.DefaultValue, "::regclass") {
			def += fmt.Sprintf(" DEFAULT %s", col.DefaultValue)
		}

		if !col.IsNullable {
			def += " NOT NULL"
		}

		if col.IsPrimaryKey {
			pks = append(pks, quoteIdentifier(col.Name))
		}

		colDefs = append(colDefs, def)
	}

	if len(pks) > 0 {
		colDefs = append(colDefs, fmt.Sprintf("    PRIMARY KEY (%s)", strings.Join(pks, ", ")))
	}

	sb.WriteString(strings.Join(colDefs, ",\n"))
	sb.WriteString("\n);")

	return sb.String()
}

// CreateSchema applies the table DDL on the target.
func (d *Driver) CreateSchema(ctx context.Context, meta core.CollectionMeta, dropExisting bool) error {
	if d.pool == nil {
		return core.NewError(core.EnginePostgres, "CreateSchema", "not connected", nil)
	}

	if dropExisting {
		dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE;", quoteIdentifier(meta.Name))
		if _, err := d.pool.Exec(ctx, dropStmt); err != nil {
			return core.NewError(core.EnginePostgres, "CreateSchema", "failed to drop table", err)
		}
	}

	// Apply CREATE TABLE
	if meta.DDL != "" {
		if _, err := d.pool.Exec(ctx, meta.DDL); err != nil {
			return core.NewError(core.EnginePostgres, "CreateSchema", "failed to create table DDL", err)
		}
	}

	return nil
}

// PostMigrationFixups synchronizes sequences and applies deferred indexes and foreign keys.
func (d *Driver) PostMigrationFixups(ctx context.Context, collections []core.CollectionMeta) error {
	if d.pool == nil {
		return nil
	}

	// 1. Resync Sequences
	for _, meta := range collections {
		for _, col := range meta.Columns {
			if strings.Contains(strings.ToLower(col.DefaultValue), "nextval") {
				syncQuery := fmt.Sprintf(`
					SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE(MAX(%s), 1))
					FROM %s;
				`, meta.Name, col.Name, quoteIdentifier(col.Name), quoteIdentifier(meta.Name))
				_, _ = d.pool.Exec(ctx, syncQuery)
			}
		}
	}

	// 2. Apply Deferred Secondary Indexes
	for _, meta := range collections {
		for _, idx := range meta.Indexes {
			if idx.RawDDL != "" {
				ddl := idx.RawDDL
				if !strings.Contains(strings.ToUpper(ddl), "IF NOT EXISTS") {
					ddl = strings.Replace(ddl, "INDEX ", "INDEX IF NOT EXISTS ", 1)
				}
				_, _ = d.pool.Exec(ctx, ddl)
			}
		}
	}

	// 3. Apply Deferred Foreign Keys
	for _, meta := range collections {
		for _, fkStmt := range meta.ForeignKeys {
			if fkStmt != "" {
				_, _ = d.pool.Exec(ctx, fkStmt)
			}
		}
	}

	return nil
}

// separateForeignKeys extracts FOREIGN KEY lines from CREATE TABLE DDL and returns (cleanDDL, alterStatements)
func separateForeignKeys(tableName, ddl string) (string, []string) {
	lines := strings.Split(ddl, "\n")
	var cleanLines []string
	var alterStmts []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "FOREIGN KEY") && strings.Contains(upper, "REFERENCES") {
			cleanConstraint := strings.TrimSuffix(trimmed, ",")
			alter := fmt.Sprintf("ALTER TABLE %s ADD %s;", quoteIdentifier(tableName), cleanConstraint)
			alterStmts = append(alterStmts, alter)
		} else {
			cleanLines = append(cleanLines, line)
		}
	}

	resultDDL := strings.Join(cleanLines, "\n")
	resultDDL = fixTrailingCommas(resultDDL)

	return resultDDL, alterStmts
}

// fixTrailingCommas cleans up trailing commas right before closing parenthesis in DDL
func fixTrailingCommas(ddl string) string {
	lines := strings.Split(ddl, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, ")") {
			for j := i - 1; j >= 0; j-- {
				prevTrimmed := strings.TrimSpace(lines[j])
				if prevTrimmed != "" {
					if strings.HasSuffix(prevTrimmed, ",") {
						lines[j] = strings.TrimSuffix(lines[j], ",")
					}
					break
				}
			}
			break
		}
	}
	return strings.Join(lines, "\n")
}

// VerifyCounts returns the verified row count of a table.
func (d *Driver) VerifyCounts(ctx context.Context, tableName string) (int64, error) {
	if d.pool == nil {
		return 0, core.NewError(core.EnginePostgres, "VerifyCounts", "not connected", nil)
	}
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(tableName))
	err := d.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, core.NewError(core.EnginePostgres, "VerifyCounts", "count query failed", err)
	}
	return count, nil
}

// Close terminates connection pool.
func (d *Driver) Close(ctx context.Context) error {
	if d.pool != nil {
		d.pool.Close()
	}
	return nil
}

// GetPool returns underlying pgx pool (for streaming adapter).
func (d *Driver) GetPool() *pgxpool.Pool {
	return d.pool
}

func quoteIdentifier(id string) string {
	return `"` + strings.ReplaceAll(id, `"`, `""`) + `"`
}

func mapPgType(t string) string {
	tLower := strings.ToLower(t)
	switch tLower {
	case "character varying", "varchar":
		return "TEXT"
	case "character", "char":
		return "TEXT"
	case "integer":
		return "INT"
	case "bigint":
		return "BIGINT"
	case "smallint":
		return "SMALLINT"
	case "boolean":
		return "BOOLEAN"
	case "timestamp without time zone":
		return "TIMESTAMP"
	case "timestamp with time zone":
		return "TIMESTAMPTZ"
	case "json":
		return "JSONB"
	default:
		return t
	}
}
