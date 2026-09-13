package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/dusmamud/dbshift/internal/config"
	"github.com/dusmamud/dbshift/internal/core"
)

// InteractiveSession holds all user inputs gathered during interactive setup.
type InteractiveSession struct {
	Engine       core.EngineType
	SourceURI    string
	TargetURI    string
	Mode         core.MigrationMode
	Concurrency  int
	BatchSize    int
	DropExisting bool
	Selected     []string
}

// MaskURI masks sensitive password credentials in database URIs.
func MaskURI(raw string) string {
	atIdx := strings.LastIndex(raw, "@")
	if atIdx == -1 {
		return raw
	}
	colonSlashIdx := strings.Index(raw, "://")
	if colonSlashIdx == -1 {
		return raw
	}
	userPass := raw[colonSlashIdx+3 : atIdx]
	passIdx := strings.Index(userPass, ":")
	if passIdx == -1 {
		return raw
	}
	return raw[:colonSlashIdx+3+passIdx+1] + "******" + raw[atIdx:]
}

// PromptWithNumber asks a numbered single-choice question where the user can just type a number or press Enter for default.
func PromptWithNumber(title string, options []string, defaultIndex int) (int, error) {
	pterm.Println()
	pterm.DefaultSection.Println(title)
	for i, opt := range options {
		marker := " "
		if i == defaultIndex {
			marker = pterm.LightGreen("★ (Default)")
		}
		pterm.Printf("  [%s] %s %s\n", pterm.Cyan(strconv.Itoa(i+1)), opt, marker)
	}

	promptText := fmt.Sprintf("Enter choice [1-%d] (Press Enter for [%d])", len(options), defaultIndex+1)
	input, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(promptText).
		Show()
	if err != nil {
		return defaultIndex, err
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return defaultIndex, nil
	}

	num, err := strconv.Atoi(trimmed)
	if err != nil || num < 1 || num > len(options) {
		// Attempt substring match
		for i, opt := range options {
			if strings.Contains(strings.ToLower(opt), strings.ToLower(trimmed)) {
				return i, nil
			}
		}
		pterm.Warning.Printf("Invalid choice %q, using default [%d]\n", trimmed, defaultIndex+1)
		return defaultIndex, nil
	}

	return num - 1, nil
}

// RunInteractiveWizard prompts the user step-by-step with smart .env detection and numbered prompts.
func RunInteractiveWizard(initialCfg *config.Config) (*InteractiveSession, error) {
	session := &InteractiveSession{
		Concurrency: 4,
		BatchSize:   2500,
	}

	// Step 1: Check Source URI from .env
	if initialCfg != nil && strings.TrimSpace(initialCfg.SourceURI) != "" {
		pterm.Println()
		pterm.Info.Printf("Loaded SOURCE_URI from .env: %s\n", pterm.Cyan(MaskURI(initialCfg.SourceURI)))
		useEnvSrc, err := pterm.DefaultInteractiveConfirm.
			WithDefaultText("Use this as Source database?").
			WithDefaultValue(true).
			Show()
		if err == nil && useEnvSrc {
			session.SourceURI = strings.TrimSpace(initialCfg.SourceURI)
		}
	}

	// Step 2: Check Target URI from .env
	if initialCfg != nil && strings.TrimSpace(initialCfg.TargetURI) != "" {
		pterm.Info.Printf("Loaded TARGET_URI from .env: %s\n", pterm.Cyan(MaskURI(initialCfg.TargetURI)))
		useEnvTgt, err := pterm.DefaultInteractiveConfirm.
			WithDefaultText("Use this as Target database?").
			WithDefaultValue(true).
			Show()
		if err == nil && useEnvTgt {
			session.TargetURI = strings.TrimSpace(initialCfg.TargetURI)
		}
	}

	// Step 3: Check if Engine can be auto-detected from URIs
	detectedEngine := core.EngineType("")
	if session.SourceURI != "" {
		detectedEngine = config.DetectEngineFromURI(session.SourceURI)
	}
	if detectedEngine == "" && session.TargetURI != "" {
		detectedEngine = config.DetectEngineFromURI(session.TargetURI)
	}

	if detectedEngine != "" {
		session.Engine = detectedEngine
		engineLabel := "PostgreSQL / CockroachDB / Supabase / Neon (SQL)"
		if session.Engine == core.EngineMongo {
			engineLabel = "MongoDB Atlas / DocumentDB (NoSQL)"
		}
		pterm.Println()
		pterm.Success.Printf("✔ Auto-detected Engine: %s (matched from connection URI)\n", pterm.Bold.Sprint(engineLabel))
	} else {
		// Prompt with number selection
		engineOptions := []string{
			"PostgreSQL / CockroachDB / Supabase / Neon (SQL)",
			"MongoDB Atlas / DocumentDB (NoSQL)",
		}
		idx, err := PromptWithNumber("Select Database Engine", engineOptions, 0)
		if err != nil {
			return nil, err
		}
		if idx == 1 {
			session.Engine = core.EngineMongo
		} else {
			session.Engine = core.EnginePostgres
		}
	}

	// Step 4: Prompt Source URI if not set
	if session.SourceURI == "" {
		var sourcePlaceholder string
		if session.Engine == core.EngineMongo {
			sourcePlaceholder = "mongodb+srv://username:password@example.com/dbname"
		} else {
			sourcePlaceholder = "postgresql://username:password@localhost:5432/dbname"
		}
		pterm.Println()
		pterm.Info.Println("Tip: Paste your connection string from your cloud console.")
		srcInput, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText(fmt.Sprintf("Enter Source (Old DB) URI (e.g. %s)", sourcePlaceholder)).
			Show()
		if err != nil || strings.TrimSpace(srcInput) == "" {
			return nil, fmt.Errorf("source URI cannot be empty")
		}
		session.SourceURI = strings.TrimSpace(srcInput)
	}

	// Step 5: Prompt Target URI if not set
	if session.TargetURI == "" {
		var targetPlaceholder string
		if session.Engine == core.EngineMongo {
			targetPlaceholder = "mongodb+srv://username:password@example.com/dbname"
		} else {
			targetPlaceholder = "postgresql://username:password@localhost:5432/dbname"
		}
		tgtInput, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText(fmt.Sprintf("Enter Target (New DB) URI (e.g. %s)", targetPlaceholder)).
			Show()
		if err != nil || strings.TrimSpace(tgtInput) == "" {
			return nil, fmt.Errorf("target URI cannot be empty")
		}
		session.TargetURI = strings.TrimSpace(tgtInput)
	}

	// Step 6: Select Migration Mode using numbers
	modeOptions := []string{
		"Full Migration (Replicate Schema + Stream Data)",
		"Data Only (Target tables/collections must already exist)",
		"Schema Only (Create tables & indexes without copying data)",
	}
	modeIdx, err := PromptWithNumber("Select Migration Mode", modeOptions, 0)
	if err != nil {
		return nil, err
	}
	switch modeIdx {
	case 1:
		session.Mode = core.ModeDataOnly
	case 2:
		session.Mode = core.ModeSchemaOnly
	default:
		session.Mode = core.ModeFull
	}

	// Step 7: Select Concurrency using numbers
	workerOptions := []string{
		"2 workers (conservative)",
		"4 workers (recommended)",
		"8 workers (high-speed)",
		"16 workers (maximum throughput)",
	}
	workerIdx, _ := PromptWithNumber("Select Parallel Workers", workerOptions, 1) // default 4
	switch workerIdx {
	case 0:
		session.Concurrency = 2
	case 2:
		session.Concurrency = 8
	case 3:
		session.Concurrency = 16
	default:
		session.Concurrency = 4
	}

	// Step 8: Drop existing tables if full mode
	if session.Mode == core.ModeFull {
		pterm.Println()
		pterm.Info.Println("Recreating tables on Target ensures 100% matching schema and avoids conflicts from previous runs.")
		dropConfirm, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultText("Clean & recreate tables on Target?").
			WithDefaultValue(true).
			Show()
		session.DropExisting = dropConfirm
	}

	return session, nil
}

// SelectCollectionsPrompt allows the user to choose all or specific tables using numbers.
func SelectCollectionsPrompt(items []core.CollectionMeta) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no collections or tables discovered in source database")
	}

	pterm.Println()
	pterm.Success.Printf("Discovered %d tables/collections in source database:\n\n", len(items))

	tblHeader := []string{"#", "Table / Collection", "Source Rows"}
	tblData := pterm.TableData{tblHeader}
	for i, it := range items {
		tblData = append(tblData, []string{
			strconv.Itoa(i + 1),
			it.Name,
			formatNumber(it.EstimatedRows),
		})
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(tblData).Render()
	pterm.Println()

	options := []string{
		"Migrate ALL (A to Z) - All tables/collections",
		"Select specific tables/collections",
	}

	choiceIdx, err := PromptWithNumber("Choose Migration Scope", options, 0)
	if err != nil {
		return nil, err
	}

	if choiceIdx == 0 {
		// All selected
		var all []string
		for _, it := range items {
			all = append(all, it.Name)
		}
		return all, nil
	}

	// Specific table selection with numbers
	pterm.Println()
	pterm.DefaultSection.Println("Available Tables/Collections:")
	for i, it := range items {
		pterm.Printf("  [%s] %s (~%s rows)\n", pterm.Cyan(strconv.Itoa(i+1)), it.Name, formatNumber(it.EstimatedRows))
	}

	input, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText("Enter numbers to migrate separated by comma (e.g. 1, 2, 3)").
		Show()
	if err != nil {
		return nil, err
	}

	var selected []string
	parts := strings.Split(input, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if num, err := strconv.Atoi(trimmed); err == nil && num >= 1 && num <= len(items) {
			selected = append(selected, items[num-1].Name)
		} else {
			// check by name
			for _, it := range items {
				if strings.EqualFold(it.Name, trimmed) {
					selected = append(selected, it.Name)
				}
			}
		}
	}

	if len(selected) == 0 {
		pterm.Warning.Println("No valid tables selected. Defaulting to Migrate ALL.")
		for _, it := range items {
			selected = append(selected, it.Name)
		}
	}

	return selected, nil
}

// ConfirmStart shows a recap and asks for final confirmation.
func ConfirmStart(session *InteractiveSession, totalTables int, totalEstimatedRows int64) (bool, error) {
	pterm.Println()
	header := []string{"Setting", "Value"}
	data := [][]string{
		{"Engine", string(session.Engine)},
		{"Source DB", MaskURI(session.SourceURI)},
		{"Target DB", MaskURI(session.TargetURI)},
		{"Mode", string(session.Mode)},
		{"Tables Selected", strconv.Itoa(totalTables)},
		{"Estimated Rows", formatNumber(totalEstimatedRows)},
		{"Concurrency", fmt.Sprintf("%d parallel workers", session.Concurrency)},
		{"Batch Size", fmt.Sprintf("%d rows/batch", session.BatchSize)},
	}

	tableData := pterm.TableData{header}
	for _, d := range data {
		tableData = append(tableData, d)
	}

	_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	pterm.Println()

	return pterm.DefaultInteractiveConfirm.
		WithDefaultText("Start migration now?").
		WithDefaultValue(true).
		Show()
}

func formatNumber(n int64) string {
	in := strconv.FormatInt(n, 10)
	var out []byte
	l := len(in)
	for i, c := range in {
		if i > 0 && (l-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}
