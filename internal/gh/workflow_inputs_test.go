package gh

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFindWorkflowDispatch(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantNil bool
	}{
		{
			name:    "scalar on (no dispatch)",
			yaml:    "on: push",
			wantNil: true,
		},
		{
			name:    "sequence on with dispatch but no inputs",
			yaml:    "on: [push, workflow_dispatch]",
			wantNil: true,
		},
		{
			name:    "mapping on without dispatch",
			yaml:    "on:\n  push:\n    branches: [main]",
			wantNil: true,
		},
		{
			name:    "mapping on with dispatch",
			yaml:    "on:\n  workflow_dispatch:\n    inputs:\n      env:\n        type: string",
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wf workflowYAML
			if err := yaml.Unmarshal([]byte(tt.yaml), &wf); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got, err := findWorkflowDispatch(&wf.On)
			if err != nil {
				t.Fatalf("findWorkflowDispatch: %v", err)
			}
			if tt.wantNil && got != nil {
				t.Errorf("expected nil node, got %+v", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("expected non-nil dispatch node, got nil")
			}
		})
	}
}

func TestParseInputs(t *testing.T) {
	raw := `
env:
  description: target environment
  required: true
  default: staging
  type: choice
  options: [staging, prod]
dry_run:
  type: boolean
  default: "false"
note: {}
`
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The top-level document contains one MappingNode as Content[0].
	if len(node.Content) == 0 {
		t.Fatalf("empty document")
	}

	inputs, err := parseInputs(node.Content[0])
	if err != nil {
		t.Fatalf("parseInputs: %v", err)
	}
	if got, want := len(inputs), 3; got != want {
		t.Fatalf("len(inputs) = %d, want %d", got, want)
	}

	env := inputs[0]
	if env.Name != "env" || env.Type != "choice" || !env.Required || env.Default != "staging" {
		t.Errorf("env input mismatch: %+v", env)
	}
	if len(env.Options) != 2 || env.Options[0] != "staging" || env.Options[1] != "prod" {
		t.Errorf("env options mismatch: %+v", env.Options)
	}

	dry := inputs[1]
	if dry.Name != "dry_run" || dry.Type != "boolean" {
		t.Errorf("dry_run input mismatch: %+v", dry)
	}

	// Inputs with no explicit type default to "string".
	note := inputs[2]
	if note.Name != "note" || note.Type != "string" {
		t.Errorf("note should default to string type, got %+v", note)
	}
}
