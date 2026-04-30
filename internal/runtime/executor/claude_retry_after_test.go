package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

func TestParseClaudeRetryAfter(t *testing.T) {
	const tolerance = 2 * time.Second

	approxEq := func(got *time.Duration, want time.Duration) bool {
		if got == nil {
			return false
		}
		diff := *got - want
		if diff < 0 {
			diff = -diff
		}
		return diff <= tolerance
	}

	tests := []struct {
		name    string
		headers http.Header
		want    *time.Duration
	}{
		{
			name:    "retry-after seconds",
			headers: http.Header{"Retry-After": []string{"60"}},
			want:    durPtr(60 * time.Second),
		},
		{
			name:    "retry-after zero is nil",
			headers: http.Header{"Retry-After": []string{"0"}},
			want:    nil,
		},
		{
			name:    "retry-after http-date in future",
			headers: http.Header{"Retry-After": []string{time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)}},
			want:    durPtr(30 * time.Second),
		},
		{
			name:    "retry-after http-date in past",
			headers: http.Header{"Retry-After": []string{time.Now().Add(-30 * time.Second).UTC().Format(http.TimeFormat)}},
			want:    nil,
		},
		{
			name:    "anthropic unified reset future",
			headers: http.Header{"Anthropic-Ratelimit-Unified-Reset": []string{fmt.Sprintf("%d", time.Now().Add(120*time.Second).Unix())}},
			want:    durPtr(120 * time.Second),
		},
		{
			name:    "anthropic unified reset past",
			headers: http.Header{"Anthropic-Ratelimit-Unified-Reset": []string{fmt.Sprintf("%d", time.Now().Add(-60*time.Second).Unix())}},
			want:    nil,
		},
		{
			name: "retry-after wins over unified reset",
			headers: http.Header{
				"Retry-After":                       []string{"60"},
				"Anthropic-Ratelimit-Unified-Reset": []string{fmt.Sprintf("%d", time.Now().Add(9999*time.Second).Unix())},
			},
			want: durPtr(60 * time.Second),
		},
		{
			name:    "empty header is nil",
			headers: http.Header{},
			want:    nil,
		},
		{
			name:    "garbage retry-after no unified reset",
			headers: http.Header{"Retry-After": []string{"notanumber"}},
			want:    nil,
		},
		{
			name: "garbage retry-after falls back to unified reset",
			headers: http.Header{
				"Retry-After":                       []string{"notanumber"},
				"Anthropic-Ratelimit-Unified-Reset": []string{fmt.Sprintf("%d", time.Now().Add(75*time.Second).Unix())},
			},
			want: durPtr(75 * time.Second),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseClaudeRetryAfter(tc.headers)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", *got)
				}
				return
			}
			if !approxEq(got, *tc.want) {
				if got == nil {
					t.Fatalf("expected ~%v, got nil", *tc.want)
				}
				t.Fatalf("expected ~%v (±%v), got %v", *tc.want, tolerance, *got)
			}
		})
	}
}

func durPtr(d time.Duration) *time.Duration { return &d }

func TestClaudeExecutor429SetsRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"rate limit"}}`))
	}))
	defer server.Close()

	executor := NewClaudeExecutor(&config.Config{})
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"api_key":  "key-123",
		"base_url": server.URL,
	}}
	payload := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)

	_, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "claude-3-5-sonnet-20241022",
		Payload: payload,
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("claude"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var se statusErr
	if !errors.As(err, &se) {
		t.Fatalf("expected statusErr, got %T: %v", err, err)
	}
	if se.StatusCode() != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want 429", se.StatusCode())
	}
	ra := se.RetryAfter()
	if ra == nil {
		t.Fatalf("RetryAfter = nil, want ~90s")
	}
	if *ra < 88*time.Second || *ra > 92*time.Second {
		t.Fatalf("RetryAfter = %v, want ~90s", *ra)
	}
}
