package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultAPIBase = "https://voidpaste.com"
	configFileName = "config.json"
)

type Config struct {
	APIBase string `json:"api_base"`
	APIKey  string `json:"api_key"`
}

func configDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("VP_CONFIG_DIR")); v != "" {
		return v, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "voidpaste"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func loadConfig() (Config, error) {
	cfg := Config{APIBase: defaultAPIBase}
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	return cfg, nil
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

// resolve merges file config with env and CLI flags.
// Precedence: flag > env > config file > default.
func resolve(apiFlag, keyFlag string) (Config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cfg, err
	}
	if v := strings.TrimSpace(os.Getenv("VP_API")); v != "" {
		cfg.APIBase = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("VP_API_KEY")); v != "" {
		cfg.APIKey = v
	} else if v := strings.TrimSpace(os.Getenv("VP_TOKEN")); v != "" {
		// Back-compat with earlier curl docs.
		cfg.APIKey = v
	}
	if strings.TrimSpace(apiFlag) != "" {
		cfg.APIBase = strings.TrimRight(strings.TrimSpace(apiFlag), "/")
	}
	if strings.TrimSpace(keyFlag) != "" {
		cfg.APIKey = strings.TrimSpace(keyFlag)
	}
	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}
	return cfg, nil
}

func requireKey(cfg Config) error {
	if cfg.APIKey == "" {
		return errors.New("API key required: run `voidpaste auth login --key vp_live_…` or set VP_API_KEY")
	}
	return nil
}
