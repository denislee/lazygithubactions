package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/tui"
)

func main() {
	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error: gh CLI is not authenticated. Run 'gh auth login' first.")
		os.Exit(1)
	}

	app := tui.NewApp()
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
