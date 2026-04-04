package gh

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type UserConfig struct {
	SelectedOrg string `json:"selectedOrg"` // "" means "All"
}

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "lazygithubactions")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func LoadConfig() UserConfig {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return UserConfig{}
	}
	var cfg UserConfig
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func SaveConfig(cfg UserConfig) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}
