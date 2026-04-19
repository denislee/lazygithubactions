package tui

import (
	"testing"

	"github.com/dns/lazygithubactions/internal/models"
)

func TestIsActiveStatus(t *testing.T) {
	cases := map[string]bool{
		"in_progress": true,
		"queued":      true,
		"waiting":     true,
		"completed":   false,
		"success":     false,
		"failure":     false,
		"":            false,
	}
	for status, want := range cases {
		if got := isActiveStatus(status); got != want {
			t.Errorf("isActiveStatus(%q) = %v, want %v", status, got, want)
		}
	}
}

func TestFormatActiveRun(t *testing.T) {
	tests := []struct {
		name string
		run  *models.RunDetail
		want string
	}{
		{
			name: "workflow only, no active job",
			run: &models.RunDetail{
				WorkflowRun: models.WorkflowRun{WorkflowName: "CI"},
				Jobs:         []models.Job{{Name: "build", Status: "completed"}},
			},
			want: "● CI",
		},
		{
			name: "in-progress job without in-progress step",
			run: &models.RunDetail{
				WorkflowRun: models.WorkflowRun{WorkflowName: "CI"},
				Jobs: []models.Job{
					{Name: "build", Status: "in_progress"},
				},
			},
			want: "● CI → build",
		},
		{
			name: "in-progress job and step",
			run: &models.RunDetail{
				WorkflowRun: models.WorkflowRun{WorkflowName: "CI"},
				Jobs: []models.Job{
					{Name: "build", Status: "in_progress", Steps: []models.Step{
						{Name: "setup", Status: "completed"},
						{Name: "compile", Status: "in_progress"},
					}},
				},
			},
			want: "● CI → build → compile",
		},
		{
			name: "queued job falls back if no in_progress",
			run: &models.RunDetail{
				WorkflowRun: models.WorkflowRun{WorkflowName: "CI"},
				Jobs: []models.Job{
					{Name: "lint", Status: "queued"},
					{Name: "build", Status: "waiting"},
				},
			},
			want: "● CI → lint",
		},
		{
			name: "in_progress wins over earlier queued",
			run: &models.RunDetail{
				WorkflowRun: models.WorkflowRun{WorkflowName: "CI"},
				Jobs: []models.Job{
					{Name: "lint", Status: "queued"},
					{Name: "build", Status: "in_progress"},
				},
			},
			want: "● CI → build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatActiveRun(tt.run); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterReposByOrg(t *testing.T) {
	app := NewApp()
	app.allRepos = []models.Repo{
		{Owner: "acme", Name: "a", FullName: "acme/a"},
		{Owner: "acme", Name: "b", FullName: "acme/b"},
		{Owner: "other", Name: "c", FullName: "other/c"},
	}

	// Empty org → all repos.
	app.selectedOrg = ""
	app.filterReposByOrg()
	if got, want := len(app.repos), 3; got != want {
		t.Errorf("empty org: len(repos) = %d, want %d", got, want)
	}

	// Specific org → only matching repos.
	app.selectedOrg = "acme"
	app.filterReposByOrg()
	if got, want := len(app.repos), 2; got != want {
		t.Fatalf("acme: len(repos) = %d, want %d", got, want)
	}
	for _, r := range app.repos {
		if r.Owner != "acme" {
			t.Errorf("acme filter leaked repo %s", r.FullName)
		}
	}

	// Unknown org → empty.
	app.selectedOrg = "nonexistent"
	app.filterReposByOrg()
	if got, want := len(app.repos), 0; got != want {
		t.Errorf("nonexistent: len(repos) = %d, want %d", got, want)
	}
}
