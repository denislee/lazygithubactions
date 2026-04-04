# lazygithubactions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a terminal UI for managing GitHub Actions across all accessible repositories, wrapping the `gh` CLI.

**Architecture:** Elm-architecture Bubble Tea v2 app with a two-panel layout (repos left, runs right). All GitHub data is fetched by shelling out to `gh` CLI via `os/exec` and parsing JSON output. Repos are cached locally for instant startup.

**Tech Stack:** Go, Bubble Tea v2, Bubbles v2, Lipgloss v2, sahilm/fuzzy, gh CLI

---

## Phase 1: Foundation

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`

**Step 1: Initialize Go module and install dependencies**

```bash
cd /home/dns/tmp/lazygithubactions
go mod init github.com/dns/lazygithubactions
go get github.com/charmbracelet/bubbletea/v2@v2
go get github.com/charmbracelet/bubbles/v2@v2
go get github.com/charmbracelet/lipgloss/v2@latest
go get github.com/sahilm/fuzzy@latest
```

**Step 2: Create minimal main.go that launches Bubble Tea**

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
)

type model struct {
	msg string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf("\n  %s\n\n  Press q to quit.\n", m.msg)
}

func main() {
	p := tea.NewProgram(model{msg: "lazygithubactions"}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Build and run to verify**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Expected: Alt screen with "lazygithubactions" text, q to quit.

**Step 4: Commit**

```bash
git add go.mod go.sum main.go
git commit -m "feat: scaffold project with minimal bubbletea app"
```

---

### Task 2: Data Types and gh Client Wrapper

**Files:**
- Create: `internal/models/types.go`
- Create: `internal/gh/client.go`
- Create: `internal/gh/client_test.go`

**Step 1: Define shared data types**

Create `internal/models/types.go`:

```go
package models

import "time"

type Repo struct {
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	FullName  string // "owner/name"
	UpdatedAt time.Time `json:"updatedAt"`
}

type WorkflowRun struct {
	ID         int64     `json:"databaseId"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	Branch     string    `json:"headBranch"`
	Event      string    `json:"event"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	URL        string    `json:"url"`
	WorkflowName string `json:"workflowName"`
}

type Job struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Steps      []Step `json:"steps"`
}

type Step struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}

type Workflow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Path  string `json:"path"`
}

// RunDetail is the full detail of a single workflow run including jobs.
type RunDetail struct {
	WorkflowRun
	Jobs []Job `json:"jobs"`
}
```

**Step 2: Create gh client wrapper with exec helper**

Create `internal/gh/client.go`:

```go
package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps the gh CLI binary.
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// run executes a gh command and returns stdout.
// It returns an error if the command fails, including stderr in the message.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// runJSON executes a gh command and unmarshals the JSON output into dest.
func (c *Client) runJSON(ctx context.Context, dest interface{}, args ...string) error {
	out, err := c.run(ctx, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(out, dest)
}
```

**Step 3: Write test for client exec helper**

Create `internal/gh/client_test.go`:

```go
package gh

import (
	"context"
	"testing"
)

func TestRunReturnsOutput(t *testing.T) {
	c := NewClient()
	out, err := c.run(context.Background(), "version")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected output from gh version")
	}
}

func TestRunReturnsErrorOnBadCommand(t *testing.T) {
	c := NewClient()
	_, err := c.run(context.Background(), "not-a-real-command-xyz")
	if err == nil {
		t.Fatal("expected error for invalid gh command")
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/gh/ -v
```

Expected: 2 PASS (one for version output, one for error on bad command).

**Step 5: Commit**

```bash
git add internal/
git commit -m "feat: add data types and gh CLI client wrapper"
```

---

### Task 3: Repository Listing with Local Cache

**Files:**
- Create: `internal/gh/repos.go`
- Create: `internal/gh/repos_test.go`
- Create: `internal/gh/cache.go`
- Create: `internal/gh/cache_test.go`

**Step 1: Implement repo listing**

Create `internal/gh/repos.go`:

```go
package gh

import (
	"context"
	"encoding/json"

	"github.com/dns/lazygithubactions/internal/models"
)

// repoJSON matches the JSON output of gh repo list --json
type repoJSON struct {
	Name  string `json:"name"`
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// ListRepos fetches all repos accessible to the authenticated user.
func (c *Client) ListRepos(ctx context.Context) ([]models.Repo, error) {
	out, err := c.run(ctx, "repo", "list", "--json", "name,owner", "--limit", "100")
	if err != nil {
		return nil, err
	}
	var raw []repoJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	repos := make([]models.Repo, len(raw))
	for i, r := range raw {
		repos[i] = models.Repo{
			Name:     r.Name,
			Owner:    r.Owner.Login,
			FullName: r.Owner.Login + "/" + r.Name,
		}
	}
	return repos, nil
}
```

**Step 2: Implement local cache**

Create `internal/gh/cache.go`:

```go
package gh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dns/lazygithubactions/internal/models"
)

const cacheTTL = 5 * time.Minute

type repoCache struct {
	Repos     []models.Repo `json:"repos"`
	FetchedAt time.Time     `json:"fetchedAt"`
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "lazygithubactions")
}

func cachePath() string {
	return filepath.Join(cacheDir(), "repos.json")
}

// LoadCachedRepos reads repos from the local cache file.
// Returns nil, nil if cache doesn't exist or is expired.
func LoadCachedRepos() ([]models.Repo, error) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache repoCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, nil // treat corrupt cache as missing
	}
	if time.Since(cache.FetchedAt) > cacheTTL {
		return nil, nil // expired
	}
	return cache.Repos, nil
}

// SaveCachedRepos writes repos to the local cache file.
func SaveCachedRepos(repos []models.Repo) error {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return err
	}
	cache := repoCache{
		Repos:     repos,
		FetchedAt: time.Now(),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(), data, 0o644)
}
```

**Step 3: Write cache tests**

Create `internal/gh/cache_test.go`:

```go
package gh

import (
	"os"
	"testing"

	"github.com/dns/lazygithubactions/internal/models"
)

func TestCacheRoundTrip(t *testing.T) {
	// Use a temp dir for testing
	origCacheDir := os.Getenv("XDG_CACHE_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CACHE_HOME", tmpDir)
	defer os.Setenv("XDG_CACHE_HOME", origCacheDir)

	repos := []models.Repo{
		{Name: "test-repo", Owner: "testuser", FullName: "testuser/test-repo"},
	}

	if err := SaveCachedRepos(repos); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadCachedRepos()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].FullName != "testuser/test-repo" {
		t.Fatalf("unexpected repos: %+v", loaded)
	}
}

func TestCacheMissOnEmpty(t *testing.T) {
	origCacheDir := os.Getenv("XDG_CACHE_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CACHE_HOME", tmpDir)
	defer os.Setenv("XDG_CACHE_HOME", origCacheDir)

	repos, err := LoadCachedRepos()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repos != nil {
		t.Fatalf("expected nil repos on cache miss, got %+v", repos)
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/gh/ -v
```

Expected: All tests pass.

**Step 5: Commit**

```bash
git add internal/
git commit -m "feat: add repository listing and local cache"
```

---

### Task 4: Workflow Runs, Workflows, and Artifacts

**Files:**
- Create: `internal/gh/runs.go`
- Create: `internal/gh/workflows.go`
- Create: `internal/gh/artifacts.go`

**Step 1: Implement run operations**

Create `internal/gh/runs.go`:

```go
package gh

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dns/lazygithubactions/internal/models"
)

// ListRuns fetches recent workflow runs for a repo.
func (c *Client) ListRuns(ctx context.Context, repo string) ([]models.WorkflowRun, error) {
	var runs []models.WorkflowRun
	err := c.runJSON(ctx, &runs, "run", "list",
		"-R", repo,
		"--json", "databaseId,name,status,conclusion,headBranch,event,createdAt,updatedAt,url,workflowName",
		"--limit", "30",
	)
	return runs, err
}

// ViewRun fetches detailed info for a single run including jobs.
func (c *Client) ViewRun(ctx context.Context, repo string, runID int64) (*models.RunDetail, error) {
	id := strconv.FormatInt(runID, 10)
	out, err := c.run(ctx, "run", "view", id, "-R", repo, "--json",
		"databaseId,name,status,conclusion,headBranch,event,createdAt,updatedAt,url,workflowName,jobs")
	if err != nil {
		return nil, err
	}
	var detail models.RunDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// ViewRunLog fetches the logs for a run as plain text.
func (c *Client) ViewRunLog(ctx context.Context, repo string, runID int64) (string, error) {
	id := strconv.FormatInt(runID, 10)
	out, err := c.run(ctx, "run", "view", id, "-R", repo, "--log")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CancelRun cancels an in-progress run.
func (c *Client) CancelRun(ctx context.Context, repo string, runID int64) error {
	id := strconv.FormatInt(runID, 10)
	_, err := c.run(ctx, "run", "cancel", id, "-R", repo)
	return err
}

// RerunFailed re-runs only the failed jobs of a run.
func (c *Client) RerunFailed(ctx context.Context, repo string, runID int64) error {
	id := strconv.FormatInt(runID, 10)
	_, err := c.run(ctx, "run", "rerun", id, "-R", repo, "--failed")
	return err
}

// DownloadArtifacts downloads all artifacts for a run to the given directory.
func (c *Client) DownloadArtifacts(ctx context.Context, repo string, runID int64, dir string) error {
	id := strconv.FormatInt(runID, 10)
	args := []string{"run", "download", id, "-R", repo}
	if dir != "" {
		args = append(args, "-D", dir)
	}
	_, err := c.run(ctx, args...)
	return err
}
```

Note: `runs.go` needs the `encoding/json` import — add it to the import block.

**Step 2: Implement workflow listing and triggering**

Create `internal/gh/workflows.go`:

```go
package gh

import (
	"context"
	"fmt"

	"github.com/dns/lazygithubactions/internal/models"
)

// ListWorkflows fetches workflows for a repo.
func (c *Client) ListWorkflows(ctx context.Context, repo string) ([]models.Workflow, error) {
	var workflows []models.Workflow
	err := c.runJSON(ctx, &workflows, "workflow", "list",
		"-R", repo,
		"--json", "id,name,state,path",
	)
	return workflows, err
}

// TriggerWorkflow dispatches a workflow_dispatch event.
// branch is the ref to run on (e.g., "main").
func (c *Client) TriggerWorkflow(ctx context.Context, repo string, workflowFile string, branch string) error {
	args := []string{"workflow", "run", workflowFile, "-R", repo}
	if branch != "" {
		args = append(args, "--ref", branch)
	}
	_, err := c.run(ctx, args...)
	return err
}
```

**Step 3: Create artifacts helper (thin wrapper)**

Create `internal/gh/artifacts.go`:

```go
package gh

// Artifact download is handled by Client.DownloadArtifacts in runs.go.
// This file is reserved for future artifact-specific operations (list, etc).
```

Actually, let's skip this empty file — download is already in runs.go.

**Step 4: Verify it compiles**

```bash
go build ./...
```

Expected: No errors.

**Step 5: Commit**

```bash
git add internal/gh/runs.go internal/gh/workflows.go
git commit -m "feat: add workflow runs, workflows, and action operations"
```

---

## Phase 2: TUI Components

### Task 5: Styles, Key Bindings, and Messages

**Files:**
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keys.go`
- Create: `internal/tui/messages.go`

**Step 1: Define lipgloss styles**

Create `internal/tui/styles.go`:

```go
package tui

import "github.com/charmbracelet/lipgloss/v2"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	successColor   = lipgloss.Color("#04B575")
	failureColor   = lipgloss.Color("#FF4444")
	warningColor   = lipgloss.Color("#FFAA00")
	runningColor   = lipgloss.Color("#00AAFF")
	dimColor       = lipgloss.Color("#666666")
	textColor      = lipgloss.Color("#FAFAFA")
	subtextColor   = lipgloss.Color("#AAAAAA")

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	// Title styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1)

	// Status styles
	statusSuccess  = lipgloss.NewStyle().Foreground(successColor)
	statusFailure  = lipgloss.NewStyle().Foreground(failureColor)
	statusRunning  = lipgloss.NewStyle().Foreground(runningColor)
	statusPending  = lipgloss.NewStyle().Foreground(warningColor)
	statusCancelled = lipgloss.NewStyle().Foreground(dimColor)

	// List item styles
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(subtextColor)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(subtextColor).
			Padding(0, 1)

	// Help bar at bottom
	helpBarStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1)

	// Overlay (for quick switcher)
	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Width(60)

	// Dialog
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(warningColor).
			Padding(1, 2).
			Width(50)
)

// StatusStyle returns the appropriate style for a run status/conclusion.
func StatusStyle(status, conclusion string) lipgloss.Style {
	switch {
	case status == "in_progress" || status == "queued" || status == "waiting":
		return statusRunning
	case conclusion == "success":
		return statusSuccess
	case conclusion == "failure":
		return statusFailure
	case conclusion == "cancelled":
		return statusCancelled
	default:
		return statusPending
	}
}

// StatusIcon returns a unicode icon for a run status/conclusion.
func StatusIcon(status, conclusion string) string {
	switch {
	case status == "in_progress":
		return "●"
	case status == "queued" || status == "waiting":
		return "◷"
	case conclusion == "success":
		return "✓"
	case conclusion == "failure":
		return "✗"
	case conclusion == "cancelled":
		return "⊘"
	case conclusion == "skipped":
		return "⊘"
	default:
		return "?"
	}
}
```

**Step 2: Define key bindings**

Create `internal/tui/keys.go`:

```go
package tui

import "github.com/charmbracelet/bubbles/v2/key"

type KeyMap struct {
	Quit        key.Binding
	Up          key.Binding
	Down        key.Binding
	Tab         key.Binding
	Enter       key.Binding
	Back        key.Binding
	Refresh     key.Binding
	Trigger     key.Binding
	Cancel      key.Binding
	Rerun       key.Binding
	Logs        key.Binding
	Download    key.Binding
	QuickSwitch key.Binding
	Filter      key.Binding
	Help        key.Binding
}

var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select/drill-down"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Trigger: key.NewBinding(
		key.WithKeys("T"),
		key.WithHelp("T", "trigger"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "cancel"),
	),
	Rerun: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "rerun failed"),
	),
	Logs: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "view logs"),
	),
	Download: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "download artifacts"),
	),
	QuickSwitch: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "quick switch"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}
```

**Step 3: Define custom Bubble Tea messages**

Create `internal/tui/messages.go`:

```go
package tui

import (
	"github.com/dns/lazygithubactions/internal/models"
)

// Data-fetch result messages
type ReposLoadedMsg struct {
	Repos []models.Repo
	Err   error
}

type RunsLoadedMsg struct {
	Repo string
	Runs []models.WorkflowRun
	Err  error
}

type RunDetailLoadedMsg struct {
	Detail *models.RunDetail
	Err    error
}

type LogLoadedMsg struct {
	Log string
	Err error
}

type WorkflowsLoadedMsg struct {
	Workflows []models.Workflow
	Err       error
}

// Action result messages
type ActionResultMsg struct {
	Action  string // "cancel", "rerun", "trigger", "download"
	Success bool
	Message string
}

// Timer tick for auto-refresh
type TickMsg struct{}
```

**Step 4: Verify it compiles**

```bash
go build ./...
```

Expected: No errors.

**Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat: add TUI styles, key bindings, and message types"
```

---

### Task 6: Repo List Panel Component

**Files:**
- Create: `internal/tui/components/repolist.go`

**Step 1: Implement repo list component**

Create `internal/tui/components/repolist.go`:

```go
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui"
)

type RepoList struct {
	repos    []models.Repo
	cursor   int
	width    int
	height   int
	focused  bool
	filter   string
	filtered []models.Repo
}

func NewRepoList() RepoList {
	return RepoList{}
}

func (r *RepoList) SetRepos(repos []models.Repo) {
	r.repos = repos
	r.applyFilter()
}

func (r *RepoList) SetSize(w, h int) {
	r.width = w
	r.height = h
}

func (r *RepoList) SetFocused(f bool) {
	r.focused = f
}

func (r *RepoList) SelectedRepo() *models.Repo {
	if len(r.filtered) == 0 {
		return nil
	}
	if r.cursor >= len(r.filtered) {
		r.cursor = len(r.filtered) - 1
	}
	return &r.filtered[r.cursor]
}

func (r *RepoList) SetFilter(f string) {
	r.filter = f
	r.applyFilter()
	r.cursor = 0
}

func (r *RepoList) applyFilter() {
	if r.filter == "" {
		r.filtered = r.repos
		return
	}
	lower := strings.ToLower(r.filter)
	r.filtered = nil
	for _, repo := range r.repos {
		if strings.Contains(strings.ToLower(repo.FullName), lower) {
			r.filtered = append(r.filtered, repo)
		}
	}
}

func (r *RepoList) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, tui.Keys.Up):
			if r.cursor > 0 {
				r.cursor--
			}
		case key.Matches(msg, tui.Keys.Down):
			if r.cursor < len(r.filtered)-1 {
				r.cursor++
			}
		}
	}
	return nil
}

func (r *RepoList) View() string {
	var b strings.Builder
	title := tui.TitleStyle().Render("Repositories")
	b.WriteString(title + "\n")

	visibleHeight := r.height - 3 // title + border padding
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	// Scroll window
	start := 0
	if r.cursor >= visibleHeight {
		start = r.cursor - visibleHeight + 1
	}

	for i := start; i < len(r.filtered) && i < start+visibleHeight; i++ {
		repo := r.filtered[i]
		line := repo.Name
		if i == r.cursor && r.focused {
			line = tui.SelectedItemStyle().Render("> " + line)
		} else if i == r.cursor {
			line = tui.NormalItemStyle().Render("> " + line)
		} else {
			line = tui.NormalItemStyle().Render("  " + line)
		}
		b.WriteString(line + "\n")
	}

	if len(r.filtered) == 0 {
		b.WriteString(tui.NormalItemStyle().Render("  No repositories found"))
	}

	content := b.String()
	style := tui.PanelStyle()
	if r.focused {
		style = tui.ActivePanelStyle()
	}
	return style.Width(r.width).Height(r.height).Render(content)
}
```

Note: This requires styles to be exported as functions. We'll adjust `styles.go` to export the needed styles via accessor functions, e.g.:

Add to `internal/tui/styles.go`:

```go
// Accessors for use by components package
func TitleStyle() lipgloss.Style          { return titleStyle }
func PanelStyle() lipgloss.Style          { return panelStyle }
func ActivePanelStyle() lipgloss.Style    { return activePanelStyle }
func SelectedItemStyle() lipgloss.Style   { return selectedItemStyle }
func NormalItemStyle() lipgloss.Style     { return normalItemStyle }
func StatusBarStyle() lipgloss.Style      { return statusBarStyle }
func HelpBarStyle() lipgloss.Style        { return helpBarStyle }
func OverlayStyle() lipgloss.Style        { return overlayStyle }
func DialogStyle() lipgloss.Style         { return dialogStyle }
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/
git commit -m "feat: add repo list panel component"
```

---

### Task 7: Run List Panel Component

**Files:**
- Create: `internal/tui/components/runlist.go`

**Step 1: Implement run list component**

Create `internal/tui/components/runlist.go`:

```go
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui"
)

type RunList struct {
	runs    []models.WorkflowRun
	repo    string
	cursor  int
	width   int
	height  int
	focused bool
}

func NewRunList() RunList {
	return RunList{}
}

func (r *RunList) SetRuns(runs []models.WorkflowRun, repo string) {
	r.runs = runs
	r.repo = repo
	if r.cursor >= len(runs) && len(runs) > 0 {
		r.cursor = len(runs) - 1
	}
}

func (r *RunList) SetSize(w, h int) {
	r.width = w
	r.height = h
}

func (r *RunList) SetFocused(f bool) {
	r.focused = f
}

func (r *RunList) SelectedRun() *models.WorkflowRun {
	if len(r.runs) == 0 {
		return nil
	}
	if r.cursor >= len(r.runs) {
		r.cursor = len(r.runs) - 1
	}
	return &r.runs[r.cursor]
}

func (r *RunList) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, tui.Keys.Up):
			if r.cursor > 0 {
				r.cursor--
			}
		case key.Matches(msg, tui.Keys.Down):
			if r.cursor < len(r.runs)-1 {
				r.cursor++
			}
		}
	}
	return nil
}

func (r *RunList) View() string {
	var b strings.Builder
	title := tui.TitleStyle().Render(fmt.Sprintf("Workflow Runs — %s", r.repo))
	b.WriteString(title + "\n")

	if len(r.runs) == 0 {
		b.WriteString(tui.NormalItemStyle().Render("  No workflow runs"))
		content := b.String()
		style := tui.PanelStyle()
		if r.focused {
			style = tui.ActivePanelStyle()
		}
		return style.Width(r.width).Height(r.height).Render(content)
	}

	visibleHeight := r.height - 3
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := 0
	if r.cursor >= visibleHeight {
		start = r.cursor - visibleHeight + 1
	}

	for i := start; i < len(r.runs) && i < start+visibleHeight; i++ {
		run := r.runs[i]
		icon := tui.StatusIcon(run.Status, run.Conclusion)
		stStyle := tui.StatusStyle(run.Status, run.Conclusion)
		status := stStyle.Render(icon)

		ago := timeAgo(run.UpdatedAt)
		line := fmt.Sprintf("%s  #%d %-20s %-12s %s",
			status, run.ID, truncate(run.WorkflowName, 20), run.Branch, ago)

		if i == r.cursor && r.focused {
			line = tui.SelectedItemStyle().Render("> " + line)
		} else if i == r.cursor {
			line = tui.NormalItemStyle().Render("> " + line)
		} else {
			line = tui.NormalItemStyle().Render("  " + line)
		}
		b.WriteString(line + "\n")
	}

	content := b.String()
	style := tui.PanelStyle()
	if r.focused {
		style = tui.ActivePanelStyle()
	}
	return style.Width(r.width).Height(r.height).Render(content)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/tui/components/runlist.go
git commit -m "feat: add run list panel component"
```

---

## Phase 3: App Shell

### Task 8: Root App Model — Two-Panel Layout with Navigation

**Files:**
- Create: `internal/tui/app.go`
- Modify: `main.go` — replace placeholder model with real app

**Step 1: Implement the root app model**

Create `internal/tui/app.go`:

```go
package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/gh"
	"github.com/dns/lazygithubactions/internal/tui/components"
)

type panel int

const (
	repoPanel panel = iota
	runPanel
)

type view int

const (
	mainView view = iota
	detailView
	logView
	quickSwitchView
	triggerView
	confirmView
)

type App struct {
	client   *gh.Client
	repoList components.RepoList
	runList  components.RunList
	// detail   components.RunDetail  — added in Task 9
	// logView  components.LogViewer  — added in Task 11
	// quickSw  components.QuickSwitch — added in Task 12

	activePanel panel
	activeView  view
	width       int
	height      int

	lastRepo string // track which repo is selected for run refresh
	loading  bool
	err      error
	message  string // temporary status message
}

func NewApp() App {
	return App{
		client:      gh.NewClient(),
		repoList:    components.NewRepoList(),
		runList:     components.NewRunList(),
		activePanel: repoPanel,
		activeView:  mainView,
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.loadRepos(),
		a.tick(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updatePanelSizes()
		return a, nil

	case tea.KeyPressMsg:
		// Global keys always active
		switch {
		case key.Matches(msg, Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, Keys.QuickSwitch):
			// TODO: open quick switcher (Task 12)
			return a, nil
		}

		// Main view keys
		if a.activeView == mainView {
			switch {
			case key.Matches(msg, Keys.Tab):
				a.togglePanel()
				return a, nil
			case key.Matches(msg, Keys.Refresh):
				return a, a.refreshCurrent()
			case key.Matches(msg, Keys.Enter):
				if a.activePanel == repoPanel {
					return a, a.selectRepo()
				}
				// TODO: drill into run detail (Task 9)
			case key.Matches(msg, Keys.Trigger):
				// TODO: trigger workflow (Task 10)
			case key.Matches(msg, Keys.Cancel):
				if run := a.runList.SelectedRun(); run != nil {
					return a, a.cancelRun(run.ID)
				}
			case key.Matches(msg, Keys.Rerun):
				if run := a.runList.SelectedRun(); run != nil {
					return a, a.rerunFailed(run.ID)
				}
			case key.Matches(msg, Keys.Logs):
				// TODO: view logs (Task 11)
			case key.Matches(msg, Keys.Download):
				if run := a.runList.SelectedRun(); run != nil {
					return a, a.downloadArtifacts(run.ID)
				}
			}

			// Forward to active panel
			if a.activePanel == repoPanel {
				cmd := a.repoList.Update(msg)
				cmds = append(cmds, cmd)
				// If selection changed, load runs for new repo
				if repo := a.repoList.SelectedRepo(); repo != nil && repo.FullName != a.lastRepo {
					cmds = append(cmds, a.loadRuns(repo.FullName))
				}
			} else {
				cmd := a.runList.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case ReposLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			a.err = msg.Err
		} else {
			a.repoList.SetRepos(msg.Repos)
			// Auto-select first repo
			if repo := a.repoList.SelectedRepo(); repo != nil {
				cmds = append(cmds, a.loadRuns(repo.FullName))
			}
		}

	case RunsLoadedMsg:
		if msg.Err != nil {
			a.err = msg.Err
		} else {
			a.runList.SetRuns(msg.Runs, msg.Repo)
			a.lastRepo = msg.Repo
		}

	case ActionResultMsg:
		a.message = msg.Message
		if msg.Success {
			cmds = append(cmds, a.refreshCurrent())
		}

	case TickMsg:
		// Auto-refresh runs for active repo
		if a.lastRepo != "" && a.activeView == mainView {
			cmds = append(cmds, a.loadRuns(a.lastRepo))
		}
		cmds = append(cmds, a.tick())
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	// Two-panel layout
	repoWidth := a.width / 4
	runWidth := a.width - repoWidth - 2 // account for borders
	panelHeight := a.height - 3          // room for status + help bars

	a.repoList.SetSize(repoWidth, panelHeight)
	a.repoList.SetFocused(a.activePanel == repoPanel)
	a.runList.SetSize(runWidth, panelHeight)
	a.runList.SetFocused(a.activePanel == runPanel)

	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		a.repoList.View(),
		a.runList.View(),
	)

	// Help bar
	help := helpBarStyle.Width(a.width).Render(
		"tab:switch  j/k:nav  enter:select  T:trigger  C:cancel  R:rerun  L:logs  D:download  r:refresh  ctrl+k:search  q:quit",
	)

	// Status message
	status := ""
	if a.err != nil {
		status = statusBarStyle.Foreground(failureColor).Width(a.width).Render("Error: " + a.err.Error())
		a.err = nil // show once
	} else if a.message != "" {
		status = statusBarStyle.Width(a.width).Render(a.message)
	}

	return lipgloss.JoinVertical(lipgloss.Left, panels, status, help)
}

// --- Commands ---

func (a *App) loadRepos() tea.Cmd {
	return func() tea.Msg {
		// Try cache first
		if repos, err := gh.LoadCachedRepos(); err == nil && repos != nil {
			// Refresh cache in background
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if fresh, err := a.client.ListRepos(ctx); err == nil {
					gh.SaveCachedRepos(fresh)
				}
			}()
			return ReposLoadedMsg{Repos: repos}
		}
		// No cache — fetch directly
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repos, err := a.client.ListRepos(ctx)
		if err == nil {
			gh.SaveCachedRepos(repos)
		}
		return ReposLoadedMsg{Repos: repos, Err: err}
	}
}

func (a *App) loadRuns(repo string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		runs, err := a.client.ListRuns(ctx, repo)
		return RunsLoadedMsg{Repo: repo, Runs: runs, Err: err}
	}
}

func (a *App) cancelRun(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := a.client.CancelRun(ctx, repo, runID)
		if err != nil {
			return ActionResultMsg{Action: "cancel", Success: false, Message: "Cancel failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "cancel", Success: true, Message: "Run cancelled"}
	}
}

func (a *App) rerunFailed(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := a.client.RerunFailed(ctx, repo, runID)
		if err != nil {
			return ActionResultMsg{Action: "rerun", Success: false, Message: "Rerun failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "rerun", Success: true, Message: "Re-running failed jobs"}
	}
}

func (a *App) downloadArtifacts(runID int64) tea.Cmd {
	repo := a.lastRepo
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		err := a.client.DownloadArtifacts(ctx, repo, runID, "")
		if err != nil {
			return ActionResultMsg{Action: "download", Success: false, Message: "Download failed: " + err.Error()}
		}
		return ActionResultMsg{Action: "download", Success: true, Message: "Artifacts downloaded"}
	}
}

func (a *App) refreshCurrent() tea.Cmd {
	if a.lastRepo != "" {
		return a.loadRuns(a.lastRepo)
	}
	return a.loadRepos()
}

func (a *App) selectRepo() tea.Cmd {
	repo := a.repoList.SelectedRepo()
	if repo == nil {
		return nil
	}
	a.activePanel = runPanel
	return a.loadRuns(repo.FullName)
}

func (a *App) togglePanel() {
	if a.activePanel == repoPanel {
		a.activePanel = runPanel
	} else {
		a.activePanel = repoPanel
	}
}

func (a *App) tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (a *App) updatePanelSizes() {
	// Sizes are computed in View(), so nothing to persist here
}
```

**Step 2: Update main.go to use App**

Replace `main.go`:

```go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/tui"
)

func main() {
	app := tui.NewApp()
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Build and smoke-test**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Expected: Two-panel layout, repos load from gh CLI (or cache), selecting a repo shows runs in the right panel. Tab switches panels, j/k navigates. q quits.

Note: `gh` must be authenticated — run `gh auth login` first if needed.

**Step 4: Commit**

```bash
git add main.go internal/tui/app.go
git commit -m "feat: add root app model with two-panel layout and navigation"
```

---

## Phase 4: Detail Views and Actions

### Task 9: Run Detail View (Jobs and Steps Drill-Down)

**Files:**
- Create: `internal/tui/components/rundetail.go`
- Modify: `internal/tui/app.go` — wire Enter on run to show detail

**Step 1: Implement run detail component**

Create `internal/tui/components/rundetail.go`:

```go
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui"
)

type RunDetail struct {
	detail *models.RunDetail
	cursor int
	width  int
	height int
}

func NewRunDetail() RunDetail {
	return RunDetail{}
}

func (d *RunDetail) SetDetail(detail *models.RunDetail) {
	d.detail = detail
	d.cursor = 0
}

func (d *RunDetail) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *RunDetail) Update(msg tea.Msg) tea.Cmd {
	if d.detail == nil {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		total := d.totalItems()
		switch {
		case key.Matches(msg, tui.Keys.Up):
			if d.cursor > 0 {
				d.cursor--
			}
		case key.Matches(msg, tui.Keys.Down):
			if d.cursor < total-1 {
				d.cursor++
			}
		}
	}
	return nil
}

func (d *RunDetail) totalItems() int {
	if d.detail == nil {
		return 0
	}
	count := 0
	for _, job := range d.detail.Jobs {
		count++ // job header
		count += len(job.Steps)
	}
	return count
}

func (d *RunDetail) View() string {
	if d.detail == nil {
		return tui.ActivePanelStyle().Width(d.width).Height(d.height).Render("Loading run details...")
	}

	var b strings.Builder
	run := d.detail
	header := fmt.Sprintf("%s  #%d  %s  %s",
		tui.StatusStyle(run.Status, run.Conclusion).Render(tui.StatusIcon(run.Status, run.Conclusion)),
		run.ID, run.WorkflowName, run.Branch)
	b.WriteString(tui.TitleStyle().Render(header) + "\n\n")

	idx := 0
	for _, job := range run.Jobs {
		jobIcon := tui.StatusIcon(job.Status, job.Conclusion)
		jobStyle := tui.StatusStyle(job.Status, job.Conclusion)
		prefix := "  "
		if idx == d.cursor {
			prefix = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, jobStyle.Render(jobIcon), job.Name))
		idx++

		for _, step := range job.Steps {
			stepIcon := tui.StatusIcon(step.Status, step.Conclusion)
			stepStyle := tui.StatusStyle(step.Status, step.Conclusion)
			prefix = "    "
			if idx == d.cursor {
				prefix = "  > "
			}
			b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, stepStyle.Render(stepIcon), step.Name))
			idx++
		}
		b.WriteString("\n")
	}

	return tui.ActivePanelStyle().Width(d.width).Height(d.height).Render(b.String())
}
```

**Step 2: Wire detail view into app.go**

Add the `runDetail` field to `App` struct and handle `Enter` on a run:
- Add `runDetail components.RunDetail` field
- On `Enter` when `activePanel == runPanel`, set `activeView = detailView` and dispatch `ViewRun` command
- Handle `RunDetailLoadedMsg` to populate the detail
- On `Esc` in detailView, return to mainView

Key changes to `app.go`:
- Add `runDetail` field to App struct, initialize in NewApp
- In Update, when Enter is pressed on runPanel: fetch run detail, switch to detailView
- Handle `RunDetailLoadedMsg`: `a.runDetail.SetDetail(msg.Detail)`
- In View: if `activeView == detailView`, render runDetail full-width instead of two panels
- On Esc in detailView: set `activeView = mainView`

**Step 3: Build and test**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Expected: Enter on a run shows job/step breakdown, Esc returns to main view.

**Step 4: Commit**

```bash
git add internal/tui/components/rundetail.go internal/tui/app.go
git commit -m "feat: add run detail drill-down view with jobs and steps"
```

---

### Task 10: Trigger Workflow Dialog

**Files:**
- Create: `internal/tui/components/dialog.go`
- Modify: `internal/tui/app.go` — wire T key to trigger dialog

**Step 1: Implement dialog component**

Create `internal/tui/components/dialog.go`:

```go
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui"
)

// ConfirmDialogResultMsg is sent when the user confirms or cancels.
type ConfirmDialogResultMsg struct {
	Confirmed bool
	Action    string
	RunID     int64
}

// TriggerDialogResultMsg is sent when the user picks a workflow and branch.
type TriggerDialogResultMsg struct {
	Cancelled    bool
	WorkflowFile string
	Branch       string
}

type ConfirmDialog struct {
	Title   string
	Message string
	Action  string
	RunID   int64
	width   int
}

func NewConfirmDialog(title, message, action string, runID int64) ConfirmDialog {
	return ConfirmDialog{
		Title:   title,
		Message: message,
		Action:  action,
		RunID:   runID,
		width:   50,
	}
}

func (d ConfirmDialog) Update(msg tea.Msg) (ConfirmDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			return d, func() tea.Msg {
				return ConfirmDialogResultMsg{Confirmed: true, Action: d.Action, RunID: d.RunID}
			}
		case "n", "N", "esc":
			return d, func() tea.Msg {
				return ConfirmDialogResultMsg{Confirmed: false, Action: d.Action, RunID: d.RunID}
			}
		}
	}
	return d, nil
}

func (d ConfirmDialog) View() string {
	content := fmt.Sprintf("%s\n\n%s\n\n[y]es  [n]o", d.Title, d.Message)
	return tui.DialogStyle().Width(d.width).Render(content)
}

// TriggerDialog lets the user pick a workflow and type a branch.
type TriggerDialog struct {
	workflows  []models.Workflow
	cursor     int
	branchInput textinput.Model
	step       int // 0 = pick workflow, 1 = type branch
	width      int
}

func NewTriggerDialog(workflows []models.Workflow) TriggerDialog {
	ti := textinput.New()
	ti.SetPlaceholder("main")
	ti.Focus()
	ti.SetWidth(30)
	return TriggerDialog{
		workflows:   workflows,
		branchInput: ti,
		step:        0,
		width:       50,
	}
}

func (d TriggerDialog) Update(msg tea.Msg) (TriggerDialog, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			return d, func() tea.Msg { return TriggerDialogResultMsg{Cancelled: true} }
		}

		if d.step == 0 {
			// Picking workflow
			switch {
			case key.Matches(msg, tui.Keys.Up):
				if d.cursor > 0 {
					d.cursor--
				}
			case key.Matches(msg, tui.Keys.Down):
				if d.cursor < len(d.workflows)-1 {
					d.cursor++
				}
			case msg.String() == "enter":
				d.step = 1
				return d, d.branchInput.Focus()
			}
		} else {
			// Typing branch
			if msg.String() == "enter" {
				branch := d.branchInput.Value()
				if branch == "" {
					branch = "main"
				}
				wf := d.workflows[d.cursor]
				return d, func() tea.Msg {
					return TriggerDialogResultMsg{
						WorkflowFile: wf.Path,
						Branch:       branch,
					}
				}
			}
			var cmd tea.Cmd
			d.branchInput, cmd = d.branchInput.Update(msg)
			return d, cmd
		}
	}
	return d, nil
}

func (d TriggerDialog) View() string {
	var b strings.Builder
	b.WriteString(tui.TitleStyle().Render("Trigger Workflow") + "\n\n")

	if d.step == 0 {
		b.WriteString("Select workflow:\n\n")
		for i, wf := range d.workflows {
			prefix := "  "
			if i == d.cursor {
				prefix = "> "
			}
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, wf.Name))
		}
	} else {
		wf := d.workflows[d.cursor]
		b.WriteString(fmt.Sprintf("Workflow: %s\n\n", wf.Name))
		b.WriteString("Branch: ")
		b.WriteString(d.branchInput.View())
		b.WriteString("\n\nPress enter to trigger, esc to cancel")
	}

	return tui.DialogStyle().Width(d.width).Render(b.String())
}
```

**Step 2: Wire into app.go**

Key changes:
- Add `confirmDialog` and `triggerDialog` fields to App
- On `T` key: fetch workflows, show trigger dialog (set activeView = triggerView)
- On `C` key: show confirm dialog instead of directly cancelling
- Handle `ConfirmDialogResultMsg` and `TriggerDialogResultMsg`
- In View: overlay the dialog on top when activeView is confirmView or triggerView

**Step 3: Build and test**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Expected: T shows workflow picker → branch input → triggers. C shows confirmation → cancels.

**Step 4: Commit**

```bash
git add internal/tui/components/dialog.go internal/tui/app.go
git commit -m "feat: add trigger workflow and confirmation dialogs"
```

---

### Task 11: Log Viewer

**Files:**
- Create: `internal/tui/components/logviewer.go`
- Modify: `internal/tui/app.go` — wire L key to log view

**Step 1: Implement log viewer with viewport**

Create `internal/tui/components/logviewer.go`:

```go
package components

import (
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/tui"
)

type LogViewer struct {
	viewport viewport.Model
	title    string
	width    int
	height   int
	ready    bool
}

func NewLogViewer() LogViewer {
	return LogViewer{}
}

func (l *LogViewer) SetContent(title, content string) {
	l.title = title
	l.viewport.SetContent(content)
	l.viewport.GotoTop()
}

func (l *LogViewer) SetSize(w, h int) {
	l.width = w
	l.height = h
	if !l.ready {
		l.viewport = viewport.New()
		l.ready = true
	}
	l.viewport.SetWidth(w - 2)
	l.viewport.SetHeight(h - 4)
}

func (l *LogViewer) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return cmd
}

func (l *LogViewer) View() string {
	header := tui.TitleStyle().Render("Logs: " + l.title)
	footer := tui.HelpBarStyle().Render("↑/↓: scroll  esc: back")
	body := l.viewport.View()
	return tui.ActivePanelStyle().Width(l.width).Height(l.height).Render(
		header + "\n" + body + "\n" + footer,
	)
}
```

**Step 2: Wire into app.go**

Key changes:
- Add `logViewer components.LogViewer` field
- On `L` key with a selected run: dispatch `ViewRunLog` command, set activeView = logView
- Handle `LogLoadedMsg`: set content on logViewer
- In View: if activeView == logView, render logViewer full-width
- Forward messages to logViewer for scrolling
- Esc returns to mainView

**Step 3: Build and test**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Expected: L on a run shows scrollable logs, Esc returns.

**Step 4: Commit**

```bash
git add internal/tui/components/logviewer.go internal/tui/app.go
git commit -m "feat: add log viewer with scrollable viewport"
```

---

## Phase 5: Polish

### Task 12: Quick Switcher (Ctrl+K)

**Files:**
- Create: `internal/tui/components/quickswitch.go`
- Modify: `internal/tui/app.go` — wire Ctrl+K

**Step 1: Implement fuzzy search quick switcher**

Create `internal/tui/components/quickswitch.go`:

```go
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/v2/textinput"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/models"
	"github.com/dns/lazygithubactions/internal/tui"
	"github.com/sahilm/fuzzy"
)

// QuickSwitchResultMsg is sent when the user selects a repo or cancels.
type QuickSwitchResultMsg struct {
	Cancelled bool
	Repo      *models.Repo
}

type QuickSwitch struct {
	input    textinput.Model
	repos    []models.Repo
	matches  []fuzzy.Match
	cursor   int
	width    int
}

func NewQuickSwitch(repos []models.Repo) QuickSwitch {
	ti := textinput.New()
	ti.SetPlaceholder("Search repositories...")
	ti.Focus()
	ti.SetWidth(50)

	qs := QuickSwitch{
		input: ti,
		repos: repos,
		width: 60,
	}
	qs.updateMatches()
	return qs
}

// repoNames implements fuzzy.Source
type repoNames []models.Repo

func (r repoNames) String(i int) string { return r[i].FullName }
func (r repoNames) Len() int            { return len(r) }

func (qs *QuickSwitch) updateMatches() {
	query := qs.input.Value()
	if query == "" {
		// Show all repos when no query
		qs.matches = make([]fuzzy.Match, len(qs.repos))
		for i := range qs.repos {
			qs.matches[i] = fuzzy.Match{Index: i, Str: qs.repos[i].FullName}
		}
	} else {
		qs.matches = fuzzy.FindFrom(query, repoNames(qs.repos))
	}
	qs.cursor = 0
}

func (qs QuickSwitch) Update(msg tea.Msg) (QuickSwitch, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return qs, func() tea.Msg { return QuickSwitchResultMsg{Cancelled: true} }
		case "enter":
			if len(qs.matches) > 0 && qs.cursor < len(qs.matches) {
				repo := qs.repos[qs.matches[qs.cursor].Index]
				return qs, func() tea.Msg { return QuickSwitchResultMsg{Repo: &repo} }
			}
			return qs, nil
		case "up", "ctrl+k":
			if qs.cursor > 0 {
				qs.cursor--
			}
			return qs, nil
		case "down", "ctrl+j":
			if qs.cursor < len(qs.matches)-1 {
				qs.cursor++
			}
			return qs, nil
		}
	}

	// Forward to text input
	var cmd tea.Cmd
	prevValue := qs.input.Value()
	qs.input, cmd = qs.input.Update(msg)
	if qs.input.Value() != prevValue {
		qs.updateMatches()
	}
	return qs, cmd
}

func (qs QuickSwitch) View() string {
	var b strings.Builder
	b.WriteString(tui.TitleStyle().Render("Quick Switch (Ctrl+K)") + "\n\n")
	b.WriteString(qs.input.View() + "\n\n")

	maxVisible := 10
	for i, m := range qs.matches {
		if i >= maxVisible {
			remaining := len(qs.matches) - maxVisible
			b.WriteString(fmt.Sprintf("  ... and %d more\n", remaining))
			break
		}
		prefix := "  "
		if i == qs.cursor {
			prefix = "> "
			b.WriteString(tui.SelectedItemStyle().Render(prefix + m.Str) + "\n")
		} else {
			b.WriteString(tui.NormalItemStyle().Render(prefix + m.Str) + "\n")
		}
	}

	if len(qs.matches) == 0 {
		b.WriteString(tui.NormalItemStyle().Render("  No matches"))
	}

	return tui.OverlayStyle().Width(qs.width).Render(b.String())
}
```

**Step 2: Wire into app.go**

Key changes:
- Add `quickSwitch *components.QuickSwitch` field (pointer, nil when hidden)
- On Ctrl+K: create QuickSwitch with current repos, set activeView = quickSwitchView
- Forward messages to quickSwitch when active
- Handle `QuickSwitchResultMsg`: if not cancelled, select that repo and load runs
- In View: if quickSwitchView, overlay quickSwitch.View() centered on top of the main layout

**Step 3: Build and test**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Expected: Ctrl+K opens fuzzy search overlay, typing filters repos live, Enter selects, Esc dismisses.

**Step 4: Commit**

```bash
git add internal/tui/components/quickswitch.go internal/tui/app.go
git commit -m "feat: add Ctrl+K fuzzy search quick switcher"
```

---

### Task 13: Status Bar Component

**Files:**
- Create: `internal/tui/components/statusbar.go`
- Modify: `internal/tui/app.go` — use status bar component

**Step 1: Implement status bar**

Create `internal/tui/components/statusbar.go`:

```go
package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/dns/lazygithubactions/internal/tui"
)

type StatusBar struct {
	width   int
	message string
	isError bool
}

func NewStatusBar() StatusBar {
	return StatusBar{}
}

func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

func (s *StatusBar) SetMessage(msg string, isError bool) {
	s.message = msg
	s.isError = isError
}

func (s *StatusBar) Clear() {
	s.message = ""
	s.isError = false
}

func (s StatusBar) View() string {
	if s.message == "" {
		return ""
	}
	style := tui.StatusBarStyle().Width(s.width)
	if s.isError {
		style = style.Foreground(lipgloss.Color("#FF4444"))
	}
	return style.Render(s.message)
}

func HelpBar(width int, activeView string) string {
	var help string
	switch activeView {
	case "detail":
		help = "esc:back  j/k:nav  L:logs  R:rerun  C:cancel  q:quit"
	case "log":
		help = "esc:back  ↑/↓:scroll  q:quit"
	default:
		help = "tab:switch  j/k:nav  enter:select  T:trigger  C:cancel  R:rerun  L:logs  D:download  r:refresh  ctrl+k:search  q:quit"
	}
	return tui.HelpBarStyle().Width(width).Render(help)
}
```

**Step 2: Integrate into app.go View()**

Replace inline help/status bar rendering with the StatusBar component and HelpBar function.

**Step 3: Build and verify**

```bash
go build -o lazygithubactions .
```

**Step 4: Commit**

```bash
git add internal/tui/components/statusbar.go internal/tui/app.go
git commit -m "feat: add status bar and context-sensitive help bar"
```

---

### Task 14: Final Integration and Polish

**Files:**
- Modify: `internal/tui/app.go` — finalize all view transitions and overlay rendering
- Modify: `main.go` — add pre-flight check for gh auth

**Step 1: Add gh auth check on startup in main.go**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/tui"
)

func main() {
	// Check gh is installed and authenticated
	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: gh CLI is not authenticated. Run 'gh auth login' first.")
		os.Exit(1)
	}

	app := tui.NewApp()
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 2: Finalize app.go with all view transitions**

Ensure all views are properly handled:
- mainView: two-panel layout
- detailView: full-width run detail
- logView: full-width log viewport
- quickSwitchView: overlay on top of main
- triggerView: overlay dialog
- confirmView: overlay dialog

Ensure the overlay rendering works by centering the dialog/quickswitch view in the terminal:

```go
func (a App) renderOverlay(bg, overlay string) string {
	// Center the overlay on the background
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(lipgloss.Color("#000000"))))
}
```

**Step 3: Full build and manual test**

```bash
go build -o lazygithubactions . && ./lazygithubactions
```

Test checklist:
- [ ] Repos load on startup (from cache if available)
- [ ] Tab switches between panels
- [ ] j/k navigates within panels
- [ ] Selecting a repo loads its workflow runs
- [ ] Enter on a run shows job/step detail
- [ ] Esc returns from detail/log views
- [ ] L shows logs in scrollable viewport
- [ ] T opens trigger workflow dialog
- [ ] C shows cancel confirmation
- [ ] R triggers rerun of failed jobs
- [ ] D downloads artifacts
- [ ] r manually refreshes
- [ ] Ctrl+K opens fuzzy search, typing filters live
- [ ] Auto-refresh updates runs every 30s
- [ ] q quits

**Step 4: Commit**

```bash
git add main.go internal/
git commit -m "feat: finalize app integration with all views and pre-flight auth check"
```

---

## Summary

| Phase | Tasks | What it delivers |
|-------|-------|-----------------|
| 1: Foundation | 1-4 | Project scaffold, types, gh wrapper, cache, all gh operations |
| 2: TUI Components | 5-7 | Styles, keys, messages, repo list panel, run list panel |
| 3: App Shell | 8 | Two-panel layout with navigation, auto-refresh |
| 4: Detail Views | 9-11 | Run detail drill-down, trigger/confirm dialogs, log viewer |
| 5: Polish | 12-14 | Ctrl+K quick switcher, status bar, final integration |
