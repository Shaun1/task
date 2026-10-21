package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TASK_HOME", home)

	cfg, err := Load()
	if err != nil || cfg.Home != home || cfg.IDE != "" {
		t.Fatalf("no config file: got %+v, %v", cfg, err)
	}

	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(`{"ide": "goland"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if cfg, err = Load(); err != nil || cfg.IDE != "goland" {
		t.Fatalf("with config file: got %+v, %v", cfg, err)
	}

	if err := os.WriteFile(filepath.Join(home, "config.json"), []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	home := filepath.Join(t.TempDir(), "not-yet")
	t.Setenv("TASK_HOME", home)
	if err := (Config{Home: home, IDE: "open -a 'IntelliJ IDEA'"}).Save(); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil || cfg.IDE != "open -a 'IntelliJ IDEA'" {
		t.Fatalf("got %+v, %v", cfg, err)
	}
}
