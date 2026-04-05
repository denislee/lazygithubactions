package gh

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dns/lazygithubactions/internal/models"
	"gopkg.in/yaml.v3"
)

type fileContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// workflowYAML uses yaml.Node for the On field to handle all formats:
// `on: push`, `on: [push, workflow_dispatch]`, `on: { workflow_dispatch: ... }`
type workflowYAML struct {
	On yaml.Node `yaml:"on"`
}

type workflowDispatch struct {
	Inputs yaml.Node `yaml:"inputs"`
}

// findWorkflowDispatch walks the `on` node to find workflow_dispatch inputs.
// Returns nil if not found (not an error — workflow may not have dispatch inputs).
func findWorkflowDispatch(onNode *yaml.Node) (*yaml.Node, error) {
	switch onNode.Kind {
	case yaml.ScalarNode:
		// on: push — no dispatch inputs
		return nil, nil
	case yaml.SequenceNode:
		// on: [push, workflow_dispatch] — dispatch exists but no inputs
		return nil, nil
	case yaml.MappingNode:
		// on: { workflow_dispatch: { inputs: ... } }
		for i := 0; i < len(onNode.Content)-1; i += 2 {
			if onNode.Content[i].Value == "workflow_dispatch" {
				return onNode.Content[i+1], nil
			}
		}
		return nil, nil
	}
	return nil, nil
}

func (c *Client) GetWorkflowInputs(ctx context.Context, repo, path string) ([]models.WorkflowInput, error) {
	out, err := c.run(ctx, "api",
		fmt.Sprintf("/repos/%s/contents/%s", repo, path),
	)
	if err != nil {
		return nil, err
	}

	var fc fileContent
	if err := json.Unmarshal(out, &fc); err != nil {
		return nil, err
	}

	// GitHub API returns base64 with embedded newlines
	cleaned := strings.ReplaceAll(fc.Content, "\n", "")
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	var wf workflowYAML
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow YAML: %w", err)
	}

	dispatchNode, err := findWorkflowDispatch(&wf.On)
	if err != nil || dispatchNode == nil {
		return nil, err
	}

	// Decode dispatch node to get inputs
	var dispatch workflowDispatch
	if err := dispatchNode.Decode(&dispatch); err != nil {
		return nil, fmt.Errorf("decode workflow_dispatch: %w", err)
	}

	if dispatch.Inputs.Kind == 0 {
		return nil, nil
	}

	return parseInputs(&dispatch.Inputs)
}

func parseInputs(node *yaml.Node) ([]models.WorkflowInput, error) {
	if node.Kind != yaml.MappingNode {
		return nil, nil
	}

	var inputs []models.WorkflowInput
	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		var input models.WorkflowInput
		input.Name = keyNode.Value
		if err := valNode.Decode(&input); err != nil {
			return nil, fmt.Errorf("decode input %q: %w", input.Name, err)
		}
		if input.Type == "" {
			input.Type = "string"
		}
		inputs = append(inputs, input)
	}
	return inputs, nil
}
