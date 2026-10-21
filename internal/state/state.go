// Package state persists one JSON record per task so `task rm` never has to guess.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ErrNotFound means no task with that name exists.
var ErrNotFound = errors.New("task not found")

// Task is everything needed to remove a task later.
type Task struct {
	Name        string    `json:"name"` // slug; also branch and clone dir name
	Repo        string    `json:"repo"`
	RepoRoot    string    `json:"repo_root"`
	OriginURL   string    `json:"origin_url"`
	ClonePath   string    `json:"clone_path"`
	Branch      string    `json:"branch"`
	SandboxName string    `json:"sandbox_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store reads and writes tasks in Dir.
type Store struct {
	Dir string
}

func (s Store) path(name string) string { return filepath.Join(s.Dir, name+".json") }

func (s Store) Save(t Task) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(t.Name), data, 0o644)
}

func (s Store) Load(name string) (Task, error) {
	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return Task{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return Task{}, err
	}
	var t Task
	if err := json.Unmarshal(data, &t); err != nil {
		return Task{}, fmt.Errorf("parse %s: %w", s.path(name), err)
	}
	return t, nil
}

// List returns all tasks, oldest first.
func (s Store) List() ([]Task, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		t, err := s.Load(e.Name()[:len(e.Name())-len(".json")])
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt.Before(tasks[j].CreatedAt) })
	return tasks, nil
}

// Delete removes a task record; deleting a missing one is not an error.
func (s Store) Delete(name string) error {
	if err := os.Remove(s.path(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
