package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestExtractAccessToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata map[string]any
		expected string
	}{
		{
			"antigravity top-level access_token",
			map[string]any{"access_token": "tok-abc"},
			"tok-abc",
		},
		{
			"gemini nested token.access_token",
			map[string]any{
				"token": map[string]any{"access_token": "tok-nested"},
			},
			"tok-nested",
		},
		{
			"top-level takes precedence over nested",
			map[string]any{
				"access_token": "tok-top",
				"token":        map[string]any{"access_token": "tok-nested"},
			},
			"tok-top",
		},
		{
			"empty metadata",
			map[string]any{},
			"",
		},
		{
			"whitespace-only access_token",
			map[string]any{"access_token": "   "},
			"",
		},
		{
			"wrong type access_token",
			map[string]any{"access_token": 12345},
			"",
		},
		{
			"token is not a map",
			map[string]any{"token": "not-a-map"},
			"",
		},
		{
			"nested whitespace-only",
			map[string]any{
				"token": map[string]any{"access_token": "  "},
			},
			"",
		},
		{
			"fallback to nested when top-level empty",
			map[string]any{
				"access_token": "",
				"token":        map[string]any{"access_token": "tok-fallback"},
			},
			"tok-fallback",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractAccessToken(tt.metadata)
			if got != tt.expected {
				t.Errorf("extractAccessToken() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFileTokenStore_PersistQuarantinedAndDisabled(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	authFile := filepath.Join(baseDir, "sample.json")
	seed := map[string]any{
		"type":        "codex",
		"disabled":    false,
		"quarantined": false,
	}
	raw, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("marshal seed metadata: %v", err)
	}
	if err = os.WriteFile(authFile, raw, 0o600); err != nil {
		t.Fatalf("write seed auth file: %v", err)
	}

	store := NewFileTokenStore()
	store.SetBaseDir(baseDir)

	auth := &cliproxyauth.Auth{
		ID:          "sample.json",
		FileName:    "sample.json",
		Provider:    "codex",
		Disabled:    true,
		Quarantined: true,
		Metadata: map[string]any{
			"type": "codex",
		},
		Attributes: map[string]string{
			"path": authFile,
		},
	}

	if _, err = store.Save(context.Background(), auth); err != nil {
		t.Fatalf("save auth metadata: %v", err)
	}

	items, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list auth metadata: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("list size = %d, want 1", len(items))
	}
	if !items[0].Disabled {
		t.Fatalf("disabled = false, want true")
	}
	if !items[0].Quarantined {
		t.Fatalf("quarantined = false, want true")
	}
}
