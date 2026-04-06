package gh

import (
	"os"
	"testing"

	"github.com/dns/lazygithubactions/internal/models"
)

func TestCacheRoundTrip(t *testing.T) {
	origCacheDir := os.Getenv("XDG_CACHE_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CACHE_HOME", tmpDir)
	defer os.Setenv("XDG_CACHE_HOME", origCacheDir)

	repos := []models.Repo{
		{Name: "test-repo", Owner: "testuser", FullName: "testuser/test-repo"},
	}

	if err := SaveCachedRepos(repos); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, _, err := LoadCachedRepos()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].FullName != "testuser/test-repo" {
		t.Fatalf("unexpected repos: %+v", loaded)
	}
}

func TestCacheMissOnEmpty(t *testing.T) {
	origCacheDir := os.Getenv("XDG_CACHE_HOME")
	tmpDir := t.TempDir()
	os.Setenv("XDG_CACHE_HOME", tmpDir)
	defer os.Setenv("XDG_CACHE_HOME", origCacheDir)

	repos, _, err := LoadCachedRepos()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repos != nil {
		t.Fatalf("expected nil repos on cache miss, got %+v", repos)
	}
}
