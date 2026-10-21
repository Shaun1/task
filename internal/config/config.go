// Package config resolves where task keeps its clones and state.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds filesystem locations. Everything lives under Home
// (default ~/.task, override with $TASK_HOME).
type Config struct {
	Home string
}

// Load builds a Config from the environment.
func Load() (Config, error) {
	if h := os.Getenv("TASK_HOME"); h != "" {
		return Config{Home: h}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("find home directory: %w", err)
	}
	return Config{Home: filepath.Join(home, ".task")}, nil
}

// ClonesDir is where per-task clones are created: <Home>/clones/<repo>/<slug>.
func (c Config) ClonesDir() string { return filepath.Join(c.Home, "clones") }

// StateDir holds one JSON file per task: <Home>/state/<slug>.json.
func (c Config) StateDir() string { return filepath.Join(c.Home, "state") }
