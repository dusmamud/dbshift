package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Orchestrator coordinates the overall migration workflow.
type Orchestrator struct {
	source  Driver
	target  Driver
	options MigrationOptions
}

// NewOrchestrator creates a new migration orchestrator.
func NewOrchestrator(source Driver, target Driver, options MigrationOptions) *Orchestrator {
	if options.BatchSize <= 0 {
		options.BatchSize = 2500
	}
	if options.Concurrency <= 0 {
		options.Concurrency = 4
	}
	return &Orchestrator{
		source:  source,
		target:  target,
		options: options,
	}
}

// PlanSummary contains pre-flight discovery information.
type PlanSummary struct {
	SourceEngine  EngineType
	TargetEngine  EngineType
	SourceDialect string
	TargetDialect string
	TotalItems    int
	TotalRows     int64
	Items         []CollectionMeta
}

// Prepare inspects the source database and filters the plan.
func (o *Orchestrator) Prepare(ctx context.Context) (*PlanSummary, error) {
	if err := o.source.Ping(ctx); err != nil {
		return nil, fmt.Errorf("source connection test failed: %w", err)
	}
	if err := o.target.Ping(ctx); err != nil {
		return nil, fmt.Errorf("target connection test failed: %w", err)
	}

	items, err := o.source.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect source database: %w", err)
	}

	// Filter by selected only if specified
	if len(o.options.SelectedOnly) > 0 {
		selectedMap := make(map[string]bool)
		for _, name := range o.options.SelectedOnly {
			selectedMap[name] = true
		}
		var filtered []CollectionMeta
		for _, it := range items {
			if selectedMap[it.Name] {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}

	var totalRows int64
	for _, it := range items {
		totalRows += it.EstimatedRows
	}

	return &PlanSummary{
		SourceEngine:  o.source.Engine(),
		TargetEngine:  o.target.Engine(),
		SourceDialect: o.source.DialectInfo(),
		TargetDialect: o.target.DialectInfo(),
		TotalItems:    len(items),
		TotalRows:     totalRows,
		Items:         items,
	}, nil
}

// Execute runs the migration with real-time progress reporting.
func (o *Orchestrator) Execute(
	ctx context.Context,
	items []CollectionMeta,
	progressChan chan<- ProgressEvent,
	onSchemaStart func(name string),
	onSchemaDone func(name string, err error),
) error {
	// Step 0: Pre-drop all tables on target with CASCADE if DropExisting is requested
	// This wipes any incomplete/stale tables from previous runs cleanly.
	if o.options.DropExisting && (o.options.Mode == ModeFull || o.options.Mode == ModeSchemaOnly) {
		for _, item := range items {
			_ = o.target.CreateSchema(ctx, CollectionMeta{Name: item.Name}, true)
		}
	}

	// Step 1: Schema Migration (if Mode is Full or SchemaOnly)
	if o.options.Mode == ModeFull || o.options.Mode == ModeSchemaOnly {
		for _, item := range items {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if onSchemaStart != nil {
				onSchemaStart(item.Name)
			}
			err := o.target.CreateSchema(ctx, item, false)
			if onSchemaDone != nil {
				onSchemaDone(item.Name, err)
			}
			if err != nil {
				return fmt.Errorf("schema creation failed for %s: %w", item.Name, err)
			}
		}
	}

	if o.options.Mode == ModeSchemaOnly {
		return nil
	}

	// Step 2: Data Migration via Worker Pool
	var wg sync.WaitGroup
	itemQueue := make(chan CollectionMeta, len(items))
	errChan := make(chan error, len(items))

	for _, it := range items {
		itemQueue <- it
	}
	close(itemQueue)

	concurrency := o.options.Concurrency
	if concurrency > len(items) {
		concurrency = len(items)
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemQueue {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
				}

				err := o.source.StreamCopy(ctx, o.target, item, o.options.BatchSize, progressChan)
				if err != nil {
					errChan <- fmt.Errorf("data streaming failed for %s: %w", item.Name, err)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check if any worker failed
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	// Step 3: Post Migration Fixups (e.g. Postgres sequence sync)
	if err := o.target.PostMigrationFixups(ctx, items); err != nil {
		return fmt.Errorf("post-migration fixups failed: %w", err)
	}

	return nil
}

// Verify runs count-level audit between source and target databases.
func (o *Orchestrator) Verify(ctx context.Context, items []CollectionMeta) ([]AuditResult, error) {
	results := make([]AuditResult, len(items))

	for i, item := range items {
		start := time.Now()
		res := AuditResult{Name: item.Name}

		srcCount, err := o.source.VerifyCounts(ctx, item.Name)
		if err != nil {
			res.Error = fmt.Sprintf("source count error: %v", err)
			results[i] = res
			continue
		}
		res.SourceCount = srcCount

		tgtCount, err := o.target.VerifyCounts(ctx, item.Name)
		if err != nil {
			res.Error = fmt.Sprintf("target count error: %v", err)
			results[i] = res
			continue
		}
		res.TargetCount = tgtCount
		res.Matched = (srcCount == tgtCount)
		res.Duration = time.Since(start)

		results[i] = res
	}

	return results, nil
}
