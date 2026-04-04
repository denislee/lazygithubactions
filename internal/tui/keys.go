package tui

import "github.com/dns/lazygithubactions/internal/tui/theme"

// Re-export from theme package for backward compatibility.
type KeyMap = theme.KeyMap

var Keys = theme.Keys
