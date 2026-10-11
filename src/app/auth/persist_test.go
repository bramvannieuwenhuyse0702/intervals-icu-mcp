package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentStoreSurvivesRestart(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "oauth-state.json")

	s1, err := NewStoreFromConfig(StateFile(path))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	now := time.Now()
	if err := s1.SaveClient(&RegisteredClient{ClientID: "c1", GrantTypes: []string{"refresh_token"}, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	if err := s1.SaveRefreshToken(&RefreshToken{Token: "rt1", ClientID: "c1", GitHubUsername: "bram", ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	// Simulate container restart.
	s2, err := NewStoreFromConfig(StateFile(path))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if _, err := s2.GetClient("c1", now); err != nil {
		t.Fatalf("client lost after restart: %v", err)
	}

	newTok := &RefreshToken{Token: "rt2", ExpiresAt: now.Add(time.Hour)}
	if _, err := s2.RotateRefreshToken("rt1", "c1", now, newTok); err != nil {
		t.Fatalf("refresh token lost after restart: %v", err)
	}

	s3, err := NewStoreFromConfig(StateFile(path))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := s3.refreshTokens["rt1"]; ok {
		t.Fatal("rotated token should not survive")
	}

	if rt, ok := s3.refreshTokens["rt2"]; !ok || rt.GitHubUsername != "bram" {
		t.Fatal("new token not persisted with identity")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %v", info.Mode().Perm())
	}
}

func TestClientTTLSlidesWithUse(t *testing.T) {
	t.Parallel()

	s := NewStore()
	created := time.Now().Add(-29 * 24 * time.Hour)

	_ = s.SaveClient(&RegisteredClient{ClientID: "c1", GrantTypes: []string{"refresh_token"}, CreatedAt: created})
	_ = s.SaveRefreshToken(&RefreshToken{Token: "rt1", ClientID: "c1", ExpiresAt: time.Now().Add(time.Hour)})

	// 31 days after creation, but only ~2 days after last use: must still be valid.
	if _, err := s.GetClient("c1", created.Add(31*24*time.Hour)); err != nil {
		t.Fatalf("active client expired: %v", err)
	}
}

func TestCorruptStateFileStartsEmpty(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "oauth-state.json")
	_ = os.WriteFile(path, []byte("{not json"), 0o600)

	s, err := NewStoreFromConfig(StateFile(path))
	if err != nil {
		t.Fatalf("corrupt file should not be fatal: %v", err)
	}

	if len(s.clients) != 0 {
		t.Fatal("expected empty store")
	}
}

func TestUnwritableStateFileFails(t *testing.T) {
	t.Parallel()

	if _, err := NewStoreFromConfig(StateFile("/nonexistent-dir/oauth-state.json")); err == nil {
		t.Fatal("expected error for unwritable path")
	}
}
