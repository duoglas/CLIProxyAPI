package auth

import (
	"context"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func TestManager_MarkResult_连续失败触发临时禁用并成功后恢复(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, nil, nil)
	m.SetConfig(&internalconfig.Config{
		AuthGuard: internalconfig.AuthGuardConfig{
			FailureThreshold:     5,
			TempDisableMinutes:   30,
			HardFailurePatterns:  []string{"invalid_grant"},
			GlobalFailureProtect: internalconfig.AuthGlobalFailureProtectConfig{Enabled: false},
		},
	})

	_, err := m.Register(context.Background(), &Auth{
		ID:       "auth-1",
		Provider: "codex",
		Metadata: map[string]any{"type": "codex"},
		Status:   StatusActive,
	})
	if err != nil {
		t.Fatalf("register auth: %v", err)
	}

	for i := 0; i < 5; i++ {
		m.MarkResult(context.Background(), Result{
			AuthID:   "auth-1",
			Provider: "codex",
			Model:    "gpt-5.4",
			Success:  false,
			Error:    &Error{HTTPStatus: 503, Message: "upstream unavailable"},
		})
	}

	updated, ok := m.GetByID("auth-1")
	if !ok || updated == nil {
		t.Fatalf("expected auth-1 exists")
	}
	if updated.ConsecutiveFailures != 5 {
		t.Fatalf("consecutive_failures = %d, want 5", updated.ConsecutiveFailures)
	}
	if updated.TempDisabledUntil.IsZero() {
		t.Fatalf("temp_disabled_until should not be zero")
	}
	if !updated.TempDisabledUntil.After(time.Now()) {
		t.Fatalf("temp_disabled_until should be in future, got %v", updated.TempDisabledUntil)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:   "auth-1",
		Provider: "codex",
		Model:    "gpt-5.4",
		Success:  true,
	})

	recovered, _ := m.GetByID("auth-1")
	if recovered.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive_failures = %d, want 0", recovered.ConsecutiveFailures)
	}
	if !recovered.TempDisabledUntil.IsZero() {
		t.Fatalf("temp_disabled_until = %v, want zero", recovered.TempDisabledUntil)
	}
}

func TestManager_MarkResult_硬失败立即隔离(t *testing.T) {
	t.Parallel()

	m := NewManager(nil, nil, nil)
	m.SetConfig(&internalconfig.Config{
		AuthGuard: internalconfig.AuthGuardConfig{
			FailureThreshold:    5,
			TempDisableMinutes:  30,
			HardFailurePatterns: []string{"invalid_grant", "invalid_refresh_token"},
		},
	})

	_, err := m.Register(context.Background(), &Auth{
		ID:       "auth-hard",
		Provider: "codex",
		Metadata: map[string]any{"type": "codex"},
		Status:   StatusActive,
	})
	if err != nil {
		t.Fatalf("register auth: %v", err)
	}

	m.MarkResult(context.Background(), Result{
		AuthID:   "auth-hard",
		Provider: "codex",
		Model:    "gpt-5.4",
		Success:  false,
		Error:    &Error{HTTPStatus: 400, Message: "oauth error: invalid_grant"},
	})

	updated, ok := m.GetByID("auth-hard")
	if !ok || updated == nil {
		t.Fatalf("expected auth-hard exists")
	}
	if !updated.Disabled {
		t.Fatalf("disabled = false, want true")
	}
	if !updated.Quarantined {
		t.Fatalf("quarantined = false, want true")
	}
	if updated.Status != StatusDisabled {
		t.Fatalf("status = %q, want %q", updated.Status, StatusDisabled)
	}
}
