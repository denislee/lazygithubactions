package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"github.com/dns/lazygithubactions/internal/tui"
)

func main() {
	if os.Getenv("LAZYGH_DEBUG") != "" {
		f, err := tea.LogToFile("debug.log", "")
		if err != nil {
			fmt.Println("Error setting up logging:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

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
