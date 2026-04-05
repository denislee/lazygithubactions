# lazygithubactions

A terminal UI for managing GitHub Actions across all your repositories, inspired by `lazygit` and `lazydocker`. Built with Go and the [Charm](https://charm.sh) framework.

`lazygithubactions` wraps the GitHub CLI (`gh`) to provide a fast, keyboard-driven interface for monitoring and triggering your CI/CD pipelines without leaving the terminal.

## Features

- **Multi-Repo Management**: Easily switch between all accessible repositories.
- **Workflow Monitoring**: Real-time view of workflow runs, statuses, and conclusions.
- **Drill-down View**: Inspect jobs and individual steps within a run.
- **Actions**: Trigger `workflow_dispatch` events, cancel runs, and re-run failed jobs directly from the TUI.
- **Log Viewer**: View workflow logs without opening a browser.
- **Artifact Management**: Download run artifacts to your local machine.
- **Local Cache**: Repository lists are cached for near-instant startup.
- **Fuzzy Search**: Quickly filter repositories and workflows.

## Prerequisites

- **Go**: 1.25.0 or later.
- **GitHub CLI (`gh`)**: Must be installed and authenticated. Run `gh auth login` if you haven't already.

## Installation

```bash
# Clone the repository
git clone https://github.com/dns/lazygithubactions.git
cd lazygithubactions

# Build the binary
go build -o lazygithubactions .

# Run it
./lazygithubactions
```

## Key Bindings

| Key | Action |
|-----|--------|
| `tab` | Switch between Repository and Run panels |
| `j`/`k` or `↑`/`↓` | Navigate lists |
| `enter` | Select repository / Drill into run details |
| `esc` | Go back / Close detail view |
| `T` | Trigger a workflow (opens picker) |
| `C` | Cancel the selected run (with confirmation) |
| `R` | Re-run failed jobs of the selected run |
| `L` | View logs for the selected run |
| `D` | Download artifacts for the selected run |
| `r` | Manual refresh |
| `/` | Filter/Search |
| `ctrl+k` | Quick switch repository |
| `q` or `ctrl+c` | Quit |

## Tech Stack

- **[Bubble Tea v2](https://github.com/charmbracelet/bubbletea)**: TUI framework.
- **[Bubbles v2](https://github.com/charmbracelet/bubbles)**: TUI components.
- **[Lipgloss v2](https://github.com/charmbracelet/lipgloss)**: Terminal styling.
- **[GitHub CLI](https://cli.github.com/)**: Backend data source.

## License

MIT
