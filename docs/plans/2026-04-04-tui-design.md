# lazygithubactions — TUI Design

## Overview

A terminal UI for GitHub Actions management, inspired by lazygit. Built in Go with Bubble Tea, wrapping the `gh` CLI for all GitHub operations.

## Decisions

- **Data layer:** `gh` CLI wrapper via `os/exec` — no auth code, leverages existing `gh` install
- **TUI framework:** Bubble Tea (charmbracelet/bubbletea) with lipgloss styling and bubbles components
- **Scale:** < 20 repos, eager fetching, no complex pagination
- **Refresh:** Auto-refresh every 30s for workflow runs
- **Repo caching:** Local cache at `~/.cache/lazygithubactions/repos.json`, loaded on startup, refreshed async every 5 minutes

## Layout

```
┌─────────────────────────────────────────────────────┐
│  lazygithubactions                   Ctrl+K:search   │
├──────────────┬──────────────────────────────────────┤
│ Repositories │  Workflow Runs                        │
│              │                                       │
│ > my-app     │  #123 CI       ✓ success   2m ago    │
│   my-lib     │  #122 Deploy   ✗ failure   5m ago    │
│   dotfiles   │  #121 CI       ● running   1m ago    │
│              │                                       │
├──────────────┴──────────────────────────────────────┤
│ [T]rigger  [C]ancel  [R]erun  [L]ogs  [D]ownload   │
└─────────────────────────────────────────────────────┘
```

- **Left panel:** Repository list (j/k or arrows to navigate)
- **Right panel:** Workflow runs for selected repo
- **Enter** on a run: drill into job/step details
- **Tab:** switch focus between panels
- **Ctrl+K:** fuzzy search quick switcher overlay
- **Bottom bar:** context-sensitive action keys

## Ctrl+K Quick Switcher

- Floating overlay with text input at top
- Live fuzzy search — every keystroke re-ranks results using `sahilm/fuzzy`
- Arrow keys to navigate results, Enter to select, Esc to dismiss
- Ranked by match quality, best match at top

## Features

| Action | Key | gh command |
|---|---|---|
| List repos | startup | `gh repo list --json name,owner --limit 100` |
| List runs | select repo | `gh run list -R owner/repo --json` |
| View run details | Enter | `gh run view <id> -R owner/repo --json` |
| View logs | L | `gh run view <id> -R owner/repo --log` |
| Trigger workflow | T | `gh workflow run <workflow> -R owner/repo` |
| Cancel run | C | `gh run cancel <id> -R owner/repo` |
| Re-run failed | R | `gh run rerun <id> -R owner/repo --failed` |
| Download artifacts | D | `gh run download <id> -R owner/repo` |
| Refresh | r | re-fetches current view |
| Filter/Search | / | client-side filter on current list |
| Quick switch repo | Ctrl+K | client-side fuzzy search overlay |

## Project Structure

```
lazygithubactions/
├── main.go
├── internal/
│   ├── gh/
│   │   ├── client.go        # Exec gh commands, parse JSON
│   │   ├── cache.go         # Local repo cache (~/.cache/lazygithubactions/)
│   │   ├── repos.go         # List repos
│   │   ├── runs.go          # List/view/cancel/rerun runs
│   │   ├── workflows.go     # List/trigger workflows
│   │   └── artifacts.go     # Download artifacts
│   ├── tui/
│   │   ├── app.go           # Root model, panel management
│   │   ├── keys.go          # Key bindings
│   │   ├── styles.go        # Lipgloss styles
│   │   ├── components/
│   │   │   ├── repolist.go  # Left panel: repo list
│   │   │   ├── runlist.go   # Right panel: workflow runs
│   │   │   ├── rundetail.go # Drill-down: jobs/steps
│   │   │   ├── logviewer.go # Log pager
│   │   │   ├── statusbar.go # Bottom bar with actions
│   │   │   ├── quickswitch.go # Ctrl+K fuzzy search overlay
│   │   │   └── dialog.go    # Confirmation/input dialogs
│   │   └── messages.go      # Custom Bubble Tea messages (Cmds)
│   └── models/
│       └── types.go         # Repo, Run, Job, Step structs
├── go.mod
└── go.sum
```

## Data Flow

1. On startup: load repos from cache (instant), then `gh repo list` async to refresh cache
2. On repo select: `gh run list -R` to populate right panel
3. Auto-refresh: `tickMsg` every 30s re-fetches runs for active repo
4. Actions (trigger/cancel/rerun): exec `gh` command, show result, refresh
5. All `gh` calls are async via Bubble Tea `Cmd` — UI never blocks

## Dependencies

- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — styling
- `github.com/charmbracelet/bubbles` — reusable components (list, viewport, textinput)
- `sahilm/fuzzy` — fuzzy matching for Ctrl+K quick switcher

## Rate Limit Budget

- 30s auto-refresh = ~120 calls/hour for the active repo view
- Repo list cached locally, refreshed every 5 min = ~12 calls/hour
- Total: ~132 calls/hour — well within 5,000/hour limit
