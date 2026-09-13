package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/dusmamud/dbshift/internal/core"
)

// ProgressTracker renders real-time migration status in the terminal.
type ProgressTracker struct {
	mu           sync.Mutex
	tableStats   map[string]*tableStatus
	totalTables  int
	completed    int
	totalRows    int64
	copiedRows   int64
	startTime    time.Time
	stopChan     chan struct{}
	spinner      *pterm.SpinnerPrinter
}

type tableStatus struct {
	TotalRows     int64
	CopiedRows    int64
	SpeedRowsPerS float64
	Completed     bool
	Err           error
}

// NewProgressTracker creates a new terminal progress manager.
func NewProgressTracker(totalTables int) *ProgressTracker {
	return &ProgressTracker{
		tableStats:  make(map[string]*tableStatus),
		totalTables: totalTables,
		startTime:   time.Now(),
		stopChan:    make(chan struct{}),
	}
}

// Start begins the spinner and terminal reporter.
func (pt *ProgressTracker) Start() {
	spinner, _ := pterm.DefaultSpinner.
		WithText("Streaming data across databases in parallel chunks...").
		Start()
	pt.spinner = spinner
}

// Update processes an incoming progress event.
func (pt *ProgressTracker) Update(ev core.ProgressEvent) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	stat, exists := pt.tableStats[ev.CollectionName]
	if !exists {
		stat = &tableStatus{}
		pt.tableStats[ev.CollectionName] = stat
	}

	stat.TotalRows = ev.TotalRows
	stat.CopiedRows = ev.CopiedRows
	stat.SpeedRowsPerS = ev.SpeedRowsPerS

	if ev.IsCompleted && !stat.Completed {
		stat.Completed = true
		pt.completed++
		pterm.Success.Printf("  ✓ Completed: %s (%s rows)\n", ev.CollectionName, formatNumber(ev.CopiedRows))
	}

	// Calculate aggregates
	var totalC int64
	for _, s := range pt.tableStats {
		totalC += s.CopiedRows
	}
	pt.copiedRows = totalC

	elapsed := time.Since(pt.startTime).Seconds()
	overallSpeed := 0.0
	if elapsed > 0 {
		overallSpeed = float64(pt.copiedRows) / elapsed
	}

	if pt.spinner != nil {
		pt.spinner.UpdateText(fmt.Sprintf(
			"Migrating [%d/%d tables] | %s rows copied | Speed: %s rows/s",
			pt.completed,
			pt.totalTables,
			formatNumber(pt.copiedRows),
			formatNumber(int64(overallSpeed)),
		))
	}
}

// Stop stops the progress tracker and reports completion time.
func (pt *ProgressTracker) Stop() {
	if pt.spinner != nil {
		pt.spinner.Success(fmt.Sprintf(
			"Data migration finished! %d tables streamed in %s.",
			pt.completed,
			time.Since(pt.startTime).Round(time.Millisecond),
		))
	}
}
