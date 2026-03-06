package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestListAuthFiles_IncludesGuardFields(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	now := time.Now()
	_, err := manager.Register(context.Background(), &coreauth.Auth{
		ID:                   "guard-auth",
		FileName:             "guard-auth.json",
		Provider:             "codex",
		Status:               coreauth.StatusError,
		Attributes:           map[string]string{"runtime_only": "true"},
		Quarantined:          true,
		ConsecutiveFailures:  6,
		TempDisabledUntil:    now.Add(15 * time.Minute),
		GuardWindowStartedAt: now.Add(-2 * time.Hour),
		GuardWindowFailures:  6,
		GuardWindowSuccesses: 0,
		Metadata:             map[string]any{"type": "codex"},
	})
	if err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)

	h.ListAuthFiles(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err = json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	files, ok := payload["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("files = %v, want one entry", payload["files"])
	}
	entry, ok := files[0].(map[string]any)
	if !ok {
		t.Fatalf("entry type invalid: %T", files[0])
	}
	if got, _ := entry["quarantined"].(bool); !got {
		t.Fatalf("quarantined = %v, want true", entry["quarantined"])
	}
	if got, _ := entry["cleanup_candidate"].(bool); !got {
		t.Fatalf("cleanup_candidate = %v, want true", entry["cleanup_candidate"])
	}
	if got, _ := entry["consecutive_failures"].(float64); int(got) != 6 {
		t.Fatalf("consecutive_failures = %v, want 6", entry["consecutive_failures"])
	}
	if _, ok = entry["temp_disabled_until"]; !ok {
		t.Fatalf("temp_disabled_until missing")
	}
}
