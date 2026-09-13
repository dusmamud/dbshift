package ui

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/dusmamud/dbshift/internal/core"
)

// RenderAuditSummary displays a verification report comparing source and target.
func RenderAuditSummary(results []core.AuditResult, totalDuration time.Duration) {
	pterm.Println()
	pterm.DefaultHeader.WithFullWidth().Println("POST-MIGRATION VERIFICATION AUDIT")
	pterm.Println()

	header := []string{"Table / Collection", "Source Rows", "Target Rows", "Verification", "Audit Time"}
	tableData := pterm.TableData{header}

	allMatched := true
	var totalSource, totalTarget int64

	for _, res := range results {
		totalSource += res.SourceCount
		totalTarget += res.TargetCount

		status := pterm.Green("✔ MATCH")
		if !res.Matched {
			allMatched = false
			status = pterm.Red("✘ MISMATCH")
		}
		if res.Error != "" {
			allMatched = false
			status = pterm.Yellow("⚠ ERROR")
		}

		tableData = append(tableData, []string{
			res.Name,
			formatNumber(res.SourceCount),
			formatNumber(res.TargetCount),
			status,
			res.Duration.Round(time.Millisecond).String(),
		})
	}

	_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	pterm.Println()

	if allMatched {
		pterm.DefaultBox.WithTitle(pterm.Green(" MIGRATION SUCCESSFUL ")).Println(
			fmt.Sprintf("All %d tables/collections verified with 100%% integrity.\nTotal Rows Transferred: %s\nTotal Elapsed Time: %s",
				len(results),
				pterm.Bold.Sprint(formatNumber(totalTarget)),
				pterm.Bold.Sprint(totalDuration.Round(time.Millisecond)),
			),
		)
	} else {
		pterm.DefaultBox.WithTitle(pterm.Red(" AUDIT WARNING ")).Println(
			"One or more tables showed row count discrepancies. Please check the table above.",
		)
	}
	pterm.Println()
}
