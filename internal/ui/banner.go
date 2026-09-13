package ui

import (
	"fmt"

	"github.com/pterm/pterm"
)

// PrintBanner prints a rich, modern header for dbshift.
func PrintBanner(version string) {
	bannerText := `
       __  __       __    _ _____ __ 
  ____/ / / /_  ___/ /_  (_) __// /_
 / __  / / __ \/ ___/ __ \/ / /_ / __/
/ /_/ /_/ /_/ (__  ) / / / / __// /_  
\__,_(_)_,___/____/_/ /_/_/_/   \__/  
`
	pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("db", pterm.NewStyle(pterm.FgCyan, pterm.Bold)),
		pterm.NewLettersFromStringWithStyle("shift", pterm.NewStyle(pterm.FgLightMagenta, pterm.Bold)),
	).Render()

	_ = bannerText // decorative reference

	pterm.DefaultCenter.Println(pterm.ThemeDefault.SecondaryStyle.Sprint("Blazing Fast Cloud Database Migration & Replication Engine"))
	pterm.DefaultCenter.Println(fmt.Sprintf("Version: %s | CockroachDB • PostgreSQL • MongoDB Atlas", pterm.LightGreen(version)))
	pterm.Println()
}
