package tui

import "time"

// Durations groups all timeouts and polling intervals used by the TUI.
// Kept as a struct (rather than scattered constants) so tests can override
// individual values without touching package-level state.
type Durations struct {
	ShortTimeout    time.Duration // current-repo detection
	DefaultTimeout  time.Duration // most API calls (ViewRun, ListRuns, etc.)
	LongTimeout     time.Duration // large payloads (ListRepos, logs)
	DownloadTimeout time.Duration // artifact downloads
	PollTimeout     time.Duration // active-run polling

	RefreshInterval   time.Duration // top-level auto-refresh
	ActiveRunInterval time.Duration // in-progress run polling
	DetailInterval    time.Duration // run-detail refresh while running
	RepoDebounceDelay time.Duration // delay before loading runs after cursor move
}

// DefaultDurations is the set of timeouts/intervals used in production.
var DefaultDurations = Durations{
	ShortTimeout:      5 * time.Second,
	DefaultTimeout:    15 * time.Second,
	LongTimeout:       30 * time.Second,
	DownloadTimeout:   60 * time.Second,
	PollTimeout:       10 * time.Second,
	RefreshInterval:   30 * time.Second,
	ActiveRunInterval: 5 * time.Second,
	DetailInterval:    5 * time.Second,
	RepoDebounceDelay: 150 * time.Millisecond,
}
