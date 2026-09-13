package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/dusmamud/dbshift/internal/core"
)

// StreamCopy streams rows from the source table to the target using pgx CopyFrom protocol.
func (d *Driver) StreamCopy(
	ctx context.Context,
	target core.Driver,
	meta core.CollectionMeta,
	batchSize int,
	progress chan<- core.ProgressEvent,
) error {
	targetPg, ok := target.(*Driver)
	if !ok || targetPg.pool == nil {
		return core.NewError(core.EnginePostgres, "StreamCopy", "target is not a valid Postgres/Cockroach driver", nil)
	}

	if batchSize <= 0 {
		batchSize = 2500
	}

	startTime := time.Now()
	totalEstimated := meta.EstimatedRows
	var totalCopied int64

	// Emit initial progress
	if progress != nil {
		progress <- core.ProgressEvent{
			CollectionName: meta.Name,
			TotalRows:      totalEstimated,
			CopiedRows:     0,
			SpeedRowsPerS:  0,
		}
	}

	// 1. Query all rows from source table
	query := fmt.Sprintf("SELECT * FROM %s", quoteIdentifier(meta.Name))
	rows, err := d.pool.Query(ctx, query)
	if err != nil {
		return core.NewError(core.EnginePostgres, "StreamCopy", "failed to select from source table", err)
	}
	defer rows.Close()

	// 2. Extract column names from field descriptions
	fieldDescs := rows.FieldDescriptions()
	colNames := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		colNames[i] = string(fd.Name)
	}

	if len(colNames) == 0 {
		if progress != nil {
			progress <- core.ProgressEvent{
				CollectionName: meta.Name,
				TotalRows:      0,
				CopiedRows:     0,
				IsCompleted:    true,
			}
		}
		return nil
	}

	// 3. Batch reading and streaming
	batch := make([][]interface{}, 0, batchSize)

	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}

		// Try binary CopyFrom first (fastest)
		_, err := targetPg.pool.CopyFrom(
			ctx,
			pgx.Identifier{meta.Name},
			colNames,
			pgx.CopyFromRows(batch),
		)

		// If CopyFrom fails (e.g. CockroachDB constraint strictness or foreign keys), fallback to batch INSERT
		if err != nil {
			fallbackErr := fallbackBatchInsert(ctx, targetPg.pool, meta.Name, colNames, batch)
			if fallbackErr != nil {
				return fmt.Errorf("copy and fallback insert failed for table %s: %w", meta.Name, fallbackErr)
			}
		}

		totalCopied += int64(len(batch))
		elapsed := time.Since(startTime).Seconds()
		speed := 0.0
		if elapsed > 0 {
			speed = float64(totalCopied) / elapsed
		}

		if progress != nil {
			progress <- core.ProgressEvent{
				CollectionName: meta.Name,
				TotalRows:      totalEstimated,
				CopiedRows:     totalCopied,
				SpeedRowsPerS:  speed,
			}
		}

		batch = batch[:0] // reset buffer
		return nil
	}

	for rows.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		values, err := rows.Values()
		if err != nil {
			return core.NewError(core.EnginePostgres, "StreamCopy", "failed reading row values", err)
		}

		batch = append(batch, values)
		if len(batch) >= batchSize {
			if err := flushBatch(); err != nil {
				return err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return core.NewError(core.EnginePostgres, "StreamCopy", "error iterating source rows", err)
	}

	// Flush remaining rows
	if err := flushBatch(); err != nil {
		return err
	}

	// Final completed event
	elapsed := time.Since(startTime).Seconds()
	speed := 0.0
	if elapsed > 0 {
		speed = float64(totalCopied) / elapsed
	}

	if progress != nil {
		progress <- core.ProgressEvent{
			CollectionName: meta.Name,
			TotalRows:      totalCopied,
			CopiedRows:     totalCopied,
			SpeedRowsPerS:  speed,
			IsCompleted:    true,
		}
	}

	return nil
}

// fallbackBatchInsert inserts rows using parameterized multi-value INSERT statement with ON CONFLICT DO NOTHING.
func fallbackBatchInsert(ctx context.Context, targetPool poolExecer, tableName string, colNames []string, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}

	chunkSize := 500 // smaller sub-batch for parameterized SQL limit
	for i := 0; i < len(rows); i += chunkSize {
		end := i + chunkSize
		if end > len(rows) {
			end = len(rows)
		}
		subRows := rows[i:end]

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("INSERT INTO %s (", quoteIdentifier(tableName)))
		for idx, c := range colNames {
			if idx > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(quoteIdentifier(c))
		}
		sb.WriteString(") VALUES ")

		args := make([]interface{}, 0, len(subRows)*len(colNames))
		argIdx := 1

		for rIdx, row := range subRows {
			if rIdx > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("(")
			for cIdx, val := range row {
				if cIdx > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("$%d", argIdx))
				args = append(args, val)
				argIdx++
			}
			sb.WriteString(")")
		}

		sb.WriteString(" ON CONFLICT DO NOTHING;")

		_, err := targetPool.Exec(ctx, sb.String(), args...)
		if err != nil {
			return err
		}
	}

	return nil
}

type poolExecer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}
