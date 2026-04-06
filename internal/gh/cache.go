package gh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/dns/lazygithubactions/internal/models"
)

const cacheTTL = 5 * time.Minute

type repoCache struct {
	Repos     []models.Repo `json:"repos"`
	FetchedAt time.Time     `json:"fetchedAt"`
}

// RunStatusEntry stores the cached status of a repo's latest workflow run.
type RunStatusEntry struct {
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	FetchedAt  time.Time `json:"fetchedAt"`
}

type runStatusCache struct {
	Entries map[string]RunStatusEntry `json:"entries"` // fullName -> entry
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "lazygithubactions")
}

func cachePath() string {
	return filepath.Join(cacheDir(), "repos.json")
}

func LoadCachedRepos() ([]models.Repo, bool, error) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var cache repoCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false, nil
	}
	expired := time.Since(cache.FetchedAt) > cacheTTL
	return cache.Repos, expired, nil
}

func SaveCachedRepos(repos []models.Repo) error {
	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return err
	}
	cache := repoCache{
		Repos:     repos,
		FetchedAt: time.Now(),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(), data, 0o644)
}

func runStatusCachePath() string {
	return filepath.Join(cacheDir(), "run_status.json")
}

// LoadCachedRunStatuses loads all cached repo run statuses from disk.
func LoadCachedRunStatuses() (map[string]RunStatusEntry, error) {
	data, err := os.ReadFile(runStatusCachePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache runStatusCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, nil
	}
	return cache.Entries, nil
}

// SaveRunStatus persists a single repo's run status to the cache file.
func SaveRunStatus(repo, status, conclusion string) {
	entries, _ := LoadCachedRunStatuses()
	if entries == nil {
		entries = make(map[string]RunStatusEntry)
	}
	entries[repo] = RunStatusEntry{
		Status:     status,
		Conclusion: conclusion,
		FetchedAt:  time.Now(),
	}
	_ = os.MkdirAll(cacheDir(), 0o755)
	data, err := json.MarshalIndent(runStatusCache{Entries: entries}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(runStatusCachePath(), data, 0o644)
}
