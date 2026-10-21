// Package config resolves where task keeps its clones and state, and reads
// the user's settings from <Home>/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Config holds filesystem locations and user settings. Everything lives under
// Home (default ~/.task, override with $TASK_HOME).
type Config struct {
	Home string `json:"-"`
	// IDE is the command `task open --ide` runs with the clone path appended,
	// e.g. "goland" or "open -a 'IntelliJ IDEA'".
	IDE string `json:"ide,omitempty"`
}

// Load builds a Config from the environment and <Home>/config.json (optional).
func Load() (Config, error) {
	home := os.Getenv("TASK_HOME")
	if home == "" {
		u, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("find home directory: %w", err)
		}
		home = filepath.Join(u, ".task")
	}
	cfg := Config{Home: home}
	data, err := os.ReadFile(cfg.File())
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", cfg.File(), err)
	}
	return cfg, nil
}

// Save writes the user settings to File.
func (c Config) Save() error {
	if err := os.MkdirAll(c.Home, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.File(), append(data, '\n'), 0o644)
}

// File is the user's settings file: <Home>/config.json.
func (c Config) File() string { return filepath.Join(c.Home, "config.json") }

// ClonesDir is where per-task clones are created: <Home>/clones/<repo>/<slug>.
func (c Config) ClonesDir() string { return filepath.Join(c.Home, "clones") }

// StateDir holds one JSON file per task: <Home>/state/<slug>.json.
func (c Config) StateDir() string { return filepath.Join(c.Home, "state") }
