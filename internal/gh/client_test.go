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
