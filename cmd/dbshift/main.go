package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/dusmamud/dbshift/internal/config"
	"github.com/dusmamud/dbshift/internal/core"
	"github.com/dusmamud/dbshift/internal/driver/mongo"
	"github.com/dusmamud/dbshift/internal/driver/postgres"
	"github.com/dusmamud/dbshift/internal/ui"
)

var (
	Version   = "v1.0.0"
	cfg       = config.LoadConfig()
	tablesArg string
	modeArg   string
	engineArg string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dbshift",
		Short: "Blazing fast cloud database migration & replication CLI",
		Long: `dbshift is a high-throughput, cross-cloud database migration tool in Go.
Seamlessly replicate data and schemas between CockroachDB Cloud, PostgreSQL,
Supabase, Neon, and MongoDB Atlas with zero data loss and live terminal progress.`,
		Run: func(cmd *cobra.Command, args []string) {
			// If source and target were explicitly provided via CLI flags, run headless
			if cmd.Flags().Changed("source") && cmd.Flags().Changed("target") {
				runDirectMigration()
				return
			}
			// Default to interactive wizard (with smart .env auto-detection)
			runInteractiveFlow(cfg)
		},
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVarP(&cfg.SourceURI, "source", "s", cfg.SourceURI, "Source database URI")
	rootCmd.PersistentFlags().StringVarP(&cfg.TargetURI, "target", "t", cfg.TargetURI, "Target database URI")
	rootCmd.PersistentFlags().StringVarP(&engineArg, "engine", "e", "postgres", "Database engine (postgres or mongo)")
	rootCmd.PersistentFlags().StringVarP(&modeArg, "mode", "m", "full", "Migration mode: full, data-only, schema-only")
	rootCmd.PersistentFlags().IntVarP(&cfg.Concurrency, "concurrency", "c", 4, "Number of parallel workers")
	rootCmd.PersistentFlags().IntVarP(&cfg.BatchSize, "batch-size", "b", 2500, "Rows per streaming batch")
	rootCmd.PersistentFlags().BoolVar(&cfg.DropExisting, "drop-existing", false, "Drop target tables/collections before copying")
	rootCmd.PersistentFlags().StringVar(&tablesArg, "tables", "", "Comma-separated list of specific tables/collections to migrate")

	// Subcommands
	rootCmd.AddCommand(newInspectCmd())
	rootCmd.AddCommand(newVerifyCmd())
	rootCmd.AddCommand(newVersionCmd())

	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}

func runInteractiveFlow(initialCfg *config.Config) {
	ui.PrintBanner(Version)

	session, err := ui.RunInteractiveWizard(initialCfg)
	if err != nil {
		pterm.Error.Printf("Setup cancelled: %v\n", err)
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize drivers
	srcDriver, tgtDriver, err := createDrivers(session.Engine)
	if err != nil {
		pterm.Error.Println(err)
		return
	}
	defer srcDriver.Close(ctx)
	defer tgtDriver.Close(ctx)

	// Test connections with spinner
	spinner, _ := pterm.DefaultSpinner.Start("Connecting to databases & fetching schema...")
	if err := srcDriver.Connect(ctx, session.SourceURI); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to connect to Source DB: %v", err))
		return
	}
	if err := tgtDriver.Connect(ctx, session.TargetURI); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to connect to Target DB: %v", err))
		return
	}

	items, err := srcDriver.Inspect(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to inspect source database: %v", err))
		return
	}
	spinner.Success(fmt.Sprintf("Connected! Source: %s | Target: %s", srcDriver.DialectInfo(), tgtDriver.DialectInfo()))

	if len(items) == 0 {
		pterm.Warning.Println("No tables or collections found in source database.")
		return
	}

	// Interactive collection selector
	selectedNames, err := ui.SelectCollectionsPrompt(items)
	if err != nil {
		pterm.Error.Println(err)
		return
	}

	// Filter items
	selectedMap := make(map[string]bool)
	for _, n := range selectedNames {
		selectedMap[n] = true
	}
	var filteredItems []core.CollectionMeta
	var totalRows int64
	for _, it := range items {
		if selectedMap[it.Name] {
			filteredItems = append(filteredItems, it)
			totalRows += it.EstimatedRows
		}
	}

	// Recap and confirm
	confirmed, err := ui.ConfirmStart(session, len(filteredItems), totalRows)
	if err != nil || !confirmed {
		pterm.Warning.Println("Migration aborted by user.")
		return
	}

	// Execute migration
	opts := core.MigrationOptions{
		Mode:         session.Mode,
		BatchSize:    session.BatchSize,
		Concurrency:  session.Concurrency,
		DropExisting: session.DropExisting,
	}

	orchestrator := core.NewOrchestrator(srcDriver, tgtDriver, opts)
	executeMigration(ctx, orchestrator, filteredItems)
}

func runDirectMigration() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	eng := core.EnginePostgres
	if strings.ToLower(engineArg) == "mongo" || strings.ToLower(engineArg) == "mongodb" {
		eng = core.EngineMongo
	}

	mode := core.ModeFull
	switch strings.ToLower(modeArg) {
	case "data-only":
		mode = core.ModeDataOnly
	case "schema-only":
		mode = core.ModeSchemaOnly
	}

	var selected []string
	if tablesArg != "" {
		for _, t := range strings.Split(tablesArg, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				selected = append(selected, trimmed)
			}
		}
	}

	opts := core.MigrationOptions{
		Mode:         mode,
		BatchSize:    cfg.BatchSize,
		Concurrency:  cfg.Concurrency,
		DropExisting: cfg.DropExisting,
		SelectedOnly: selected,
	}

	srcDriver, tgtDriver, err := createDrivers(eng)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
	defer srcDriver.Close(ctx)
	defer tgtDriver.Close(ctx)

	pterm.Info.Println("Connecting to Source and Target databases...")
	if err := srcDriver.Connect(ctx, cfg.SourceURI); err != nil {
		pterm.Error.Printf("Source connection failed: %v\n", err)
		os.Exit(1)
	}
	if err := tgtDriver.Connect(ctx, cfg.TargetURI); err != nil {
		pterm.Error.Printf("Target connection failed: %v\n", err)
		os.Exit(1)
	}

	orchestrator := core.NewOrchestrator(srcDriver, tgtDriver, opts)
	plan, err := orchestrator.Prepare(ctx)
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}

	pterm.Success.Printf("Plan ready: %d tables/collections (~%d rows). Starting streaming...\n", plan.TotalItems, plan.TotalRows)
	executeMigration(ctx, orchestrator, plan.Items)
}

func executeMigration(ctx context.Context, orchestrator *core.Orchestrator, items []core.CollectionMeta) {
	progressChan := make(chan core.ProgressEvent, 200)
	tracker := ui.NewProgressTracker(len(items))
	tracker.Start()

	// Consumer goroutine for progress events
	doneTracker := make(chan struct{})
	go func() {
		defer close(doneTracker)
		for ev := range progressChan {
			tracker.Update(ev)
		}
	}()

	startTime := time.Now()
	err := orchestrator.Execute(
		ctx,
		items,
		progressChan,
		func(name string) {
			// Schema creation started
		},
		func(name string, schemaErr error) {
			if schemaErr != nil {
				pterm.Warning.Printf("Schema warning for %s: %v\n", name, schemaErr)
			}
		},
	)

	close(progressChan)
	<-doneTracker
	tracker.Stop()

	if err != nil {
		pterm.Error.Printf("Migration failed with error: %v\n", err)
		return
	}

	// Run Post-Migration Verification Audit
	verifySpinner, _ := pterm.DefaultSpinner.Start("Verifying data integrity between Source and Target...")
	results, verifyErr := orchestrator.Verify(ctx, items)
	if verifyErr != nil {
		verifySpinner.Fail(fmt.Sprintf("Verification failed: %v", verifyErr))
		return
	}
	verifySpinner.Success("Verification checks complete!")

	ui.RenderAuditSummary(results, time.Since(startTime))
}

func createDrivers(engine core.EngineType) (core.Driver, core.Driver, error) {
	switch engine {
	case core.EngineMongo:
		return mongo.NewDriver(), mongo.NewDriver(), nil
	case core.EnginePostgres:
		return postgres.NewDriver(), postgres.NewDriver(), nil
	default:
		return nil, nil, fmt.Errorf("unsupported engine: %s", engine)
	}
}

func newInspectCmd() *cobra.Command {
	var uri string
	var engine string
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect tables, indexes, and row counts of a database",
		Run: func(cmd *cobra.Command, args []string) {
			if uri == "" {
				pterm.Error.Println("URI flag is required: --url=<connection_string>")
				return
			}
			ctx := context.Background()
			eng := core.EnginePostgres
			if strings.ToLower(engine) == "mongo" {
				eng = core.EngineMongo
			}
			driver, _, err := createDrivers(eng)
			if err != nil {
				pterm.Error.Println(err)
				return
			}
			defer driver.Close(ctx)

			spinner, _ := pterm.DefaultSpinner.Start("Connecting and inspecting database...")
			if err := driver.Connect(ctx, uri); err != nil {
				spinner.Fail(err.Error())
				return
			}
			items, err := driver.Inspect(ctx)
			if err != nil {
				spinner.Fail(err.Error())
				return
			}
			spinner.Success(fmt.Sprintf("Found %d tables/collections in %s", len(items), driver.DialectInfo()))

			header := []string{"Name", "Estimated Rows", "Columns", "Indexes"}
			data := pterm.TableData{header}
			for _, it := range items {
				data = append(data, []string{
					it.Name,
					fmt.Sprintf("%d", it.EstimatedRows),
					fmt.Sprintf("%d", len(it.Columns)),
					fmt.Sprintf("%d", len(it.Indexes)),
				})
			}
			_ = pterm.DefaultTable.WithHasHeader().WithData(data).Render()
		},
	}
	cmd.Flags().StringVarP(&uri, "url", "u", "", "Database connection URL")
	cmd.Flags().StringVarP(&engine, "engine", "e", "postgres", "Database engine (postgres or mongo)")
	return cmd
}

func newVerifyCmd() *cobra.Command {
	var src, tgt, engine string
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Compare table row counts between source and target databases",
		Run: func(cmd *cobra.Command, args []string) {
			if src == "" || tgt == "" {
				pterm.Error.Println("Both --source and --target flags are required.")
				return
			}
			ctx := context.Background()
			eng := core.EnginePostgres
			if strings.ToLower(engine) == "mongo" {
				eng = core.EngineMongo
			}
			srcDriver, tgtDriver, err := createDrivers(eng)
			if err != nil {
				pterm.Error.Println(err)
				return
			}
			defer srcDriver.Close(ctx)
			defer tgtDriver.Close(ctx)

			spinner, _ := pterm.DefaultSpinner.Start("Running standalone verification audit...")
			_ = srcDriver.Connect(ctx, src)
			_ = tgtDriver.Connect(ctx, tgt)

			orchestrator := core.NewOrchestrator(srcDriver, tgtDriver, core.DefaultOptions())
			plan, err := orchestrator.Prepare(ctx)
			if err != nil {
				spinner.Fail(err.Error())
				return
			}

			results, err := orchestrator.Verify(ctx, plan.Items)
			if err != nil {
				spinner.Fail(err.Error())
				return
			}
			spinner.Success("Verification complete!")
			ui.RenderAuditSummary(results, 0)
		},
	}
	cmd.Flags().StringVarP(&src, "source", "s", "", "Source database URI")
	cmd.Flags().StringVarP(&tgt, "target", "t", "", "Target database URI")
	cmd.Flags().StringVarP(&engine, "engine", "e", "postgres", "Database engine (postgres or mongo)")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print dbshift version",
		Run: func(cmd *cobra.Command, args []string) {
			pterm.DefaultBasicText.Println(fmt.Sprintf("dbshift version %s (Go runtime)", Version))
		},
	}
}
