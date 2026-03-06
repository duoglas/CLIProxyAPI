package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestGetAuthsHealth_ReturnsCleanupCandidates(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	now := time.Now()
	_, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:                   "auth-fail",
		FileName:             "auth-fail.json",
		Provider:             "codex",
		Status:               coreauth.StatusError,
		GuardWindowStartedAt: now.Add(-2 * time.Hour),
		GuardWindowFailures:  12,
		GuardWindowSuccesses: 0,
		Metadata:             map[string]any{"type": "codex"},
	})
	if err != nil {
		t.Fatalf("register auth-fail: %v", err)
	}
	_, err = manager.Register(context.Background(), &coreauth.Auth{
		ID:                   "auth-ok",
		FileName:             "auth-ok.json",
		Provider:             "codex",
		Status:               coreauth.StatusActive,
		GuardWindowStartedAt: now.Add(-2 * time.Hour),
		GuardWindowFailures:  3,
		GuardWindowSuccesses: 2,
		Metadata:             map[string]any{"type": "codex"},
	})
	if err != nil {
		t.Fatalf("register auth-ok: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auths/health", nil)

	h.GetAuthsHealth(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err = json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got, _ := payload["candidates"].(float64); int(got) != 1 {
		t.Fatalf("candidates = %v, want 1", payload["candidates"])
	}
}

func TestCleanupAuths_DisablesAndQuarantinesCandidates(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	_, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:                   "auth-clean",
		FileName:             "auth-clean.json",
		Provider:             "codex",
		Status:               coreauth.StatusError,
		GuardWindowStartedAt: time.Now().Add(-1 * time.Hour),
		GuardWindowFailures:  8,
		GuardWindowSuccesses: 0,
		Metadata:             map[string]any{"type": "codex"},
	})
	if err != nil {
		t.Fatalf("register auth-clean: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := strings.NewReader(`{"dry_run":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auths/cleanup", body)
	c.Request.Header.Set("Content-Type", "application/json")

	h.CleanupAuths(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := manager.GetByID("auth-clean")
	if !ok || updated == nil {
		t.Fatalf("auth-clean not found")
	}
	if !updated.Disabled {
		t.Fatalf("disabled = false, want true")
	}
	if !updated.Quarantined {
		t.Fatalf("quarantined = false, want true")
	}
	if updated.Status != coreauth.StatusDisabled {
		t.Fatalf("status = %q, want %q", updated.Status, coreauth.StatusDisabled)
	}
}
