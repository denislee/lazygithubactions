package tui

import "github.com/dns/lazygithubactions/internal/tui/theme"

// Re-export message types from theme package for backward compatibility.
type ReposLoadedMsg = theme.ReposLoadedMsg
type CurrentRepoLoadedMsg = theme.CurrentRepoLoadedMsg
type RunsLoadedMsg = theme.RunsLoadedMsg
type RunDetailLoadedMsg = theme.RunDetailLoadedMsg
type LogLoadedMsg = theme.LogLoadedMsg
type WorkflowsLoadedMsg = theme.WorkflowsLoadedMsg
type ActionResultMsg = theme.ActionResultMsg
type TickMsg = theme.TickMsg
