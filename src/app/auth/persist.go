package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// StateFile is the path of the JSON file used to persist registered clients and
// refresh tokens across restarts. Empty means in-memory only.
type StateFile string

// persistedState is the on-disk representation of the store.
// Authorization codes are deliberately not persisted: they live for minutes only.
type persistedState struct {
	Version       int                          `json:"version"`
	Clients       map[string]*RegisteredClient `json:"clients"`
	RefreshTokens map[string]*RefreshToken     `json:"refresh_tokens"`
}

const persistedStateVersion = 1

// NewStoreFromConfig creates a Store. When stateFile is non-empty, existing state is
// loaded from it and every change to clients or refresh tokens is written back, so
// connected MCP clients keep working after a container restart.
func NewStoreFromConfig(stateFile StateFile) (*Store, error) {
	store := NewStore()

	if stateFile == "" {
		slog.Warn("auth: no --state-file configured; OAuth clients and refresh tokens " +
			"are lost on restart and connected clients must re-authenticate")

		return store, nil
	}

	store.statePath = string(stateFile)

	if err := store.load(time.Now()); err != nil {
		return nil, err
	}

	// Verify the location is writable now, rather than failing silently later.
	store.mu.Lock()
	err := store.persistLocked()
	store.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("auth: state file %q is not writable: %w", store.statePath, err)
	}

	slog.Info("auth: persistent OAuth state enabled",
		"path", store.statePath,
		"clients", len(store.clients),
		"refresh_tokens", len(store.refreshTokens))

	return store, nil
}

func (s *Store) load(now time.Time) error {
	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("auth: read state file: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		// A corrupt file should not keep the server down; start fresh but say so loudly.
		slog.Error("auth: state file is corrupt, starting with empty state", "path", s.statePath, "error", err)

		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, c := range state.Clients {
		if c != nil && c.ClientID == id {
			s.clients[id] = c
		}
	}

	for tok, rt := range state.RefreshTokens {
		if rt != nil && rt.Token == tok {
			s.refreshTokens[tok] = rt
		}
	}

	s.evictExpiredLocked(now)

	return nil
}

// persistLocked writes the current clients and refresh tokens to disk atomically
// (write to temp file, fsync, rename). The caller must hold s.mu.
func (s *Store) persistLocked() error {
	if s.statePath == "" {
		return nil
	}

	data, err := json.Marshal(persistedState{
		Version:       persistedStateVersion,
		Clients:       s.clients,
		RefreshTokens: s.refreshTokens,
	})
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	dir := filepath.Dir(s.statePath)

	tmp, err := os.CreateTemp(dir, ".oauth-state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // best-effort cleanup; no-op after successful rename.

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()

		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()

		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()

		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpName, s.statePath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// persistOrLog persists and logs failures. Persistence errors must never break
// an OAuth flow that already succeeded in memory. The caller must hold s.mu.
func (s *Store) persistOrLog() {
	if err := s.persistLocked(); err != nil {
		slog.Error("auth: failed to persist OAuth state", "path", s.statePath, "error", err)
	}
}
