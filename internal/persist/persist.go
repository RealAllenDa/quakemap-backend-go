// Package persist stores the API state needed to survive process restarts.
package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"quakemap-backend-go/internal/model"
)

const (
	stateVersion = 1
	stateFile    = "api_state.json"
)

type State struct {
	Version        int                  `json:"version"`
	SavedAt        time.Time            `json:"saved_at"`
	EarthquakeInfo model.EarthquakeInfo `json:"earthquake_info"`
	TsunamiInfo    model.TsunamiTotal   `json:"tsunami_info"`
}

type Repository struct {
	directory string
	path      string
}

// New creates the persistence directory if it does not exist.
func New(directory string) (*Repository, error) {
	if directory == "" {
		return nil, errors.New("persist: directory is empty")
	}
	directory = filepath.Clean(directory)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("persist: create directory: %w", err)
	}
	return &Repository{directory: directory, path: filepath.Join(directory, stateFile)}, nil
}

func (r *Repository) Path() string { return r.path }

// Load returns exists=false on the first startup, when no state has been saved.
func (r *Repository) Load() (state State, exists bool, err error) {
	raw, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		raw, err = os.ReadFile(r.path + ".bak")
		if errors.Is(err, os.ErrNotExist) {
			return State{}, false, nil
		}
	}
	if err != nil {
		return State{}, false, fmt.Errorf("persist: read state: %w", err)
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, true, fmt.Errorf("persist: decode state: %w", err)
	}
	if state.Version != stateVersion {
		return State{}, true, fmt.Errorf("persist: unsupported state version %d", state.Version)
	}
	return state, true, nil
}

// Save writes a complete state file and then replaces the previous save.
func (r *Repository) Save(earthquake model.EarthquakeInfo, tsunami model.TsunamiTotal) error {
	state := State{
		Version:        stateVersion,
		SavedAt:        time.Now().UTC(),
		EarthquakeInfo: earthquake,
		TsunamiInfo:    tsunami,
	}
	temporary, err := os.CreateTemp(r.directory, ".api-state-*.tmp")
	if err != nil {
		return fmt.Errorf("persist: create temporary state: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("persist: protect temporary state: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		return fmt.Errorf("persist: encode state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("persist: sync state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("persist: close state: %w", err)
	}
	if err := replace(temporaryPath, r.path); err != nil {
		return fmt.Errorf("persist: replace state: %w", err)
	}
	committed = true
	return nil
}

func replace(source, target string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(source, target)
	}
	// Windows does not replace an existing destination with os.Rename.
	backup := target + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(source, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	_ = os.Remove(backup)
	return nil
}
