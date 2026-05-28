package executor

import (
	"context"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

func TestNewAntigravityHTTPClientReusesSharedHTTP11Transport(t *testing.T) {
	clientA := newAntigravityHTTPClient(context.Background(), &config.Config{}, &cliproxyauth.Auth{}, 0)
	clientB := newAntigravityHTTPClient(context.Background(), &config.Config{}, &cliproxyauth.Auth{}, 0)

	transportA, okA := clientA.Transport.(*http.Transport)
	if !okA {
		t.Fatalf("clientA transport type = %T, want *http.Transport", clientA.Transport)
	}
	transportB, okB := clientB.Transport.(*http.Transport)
	if !okB {
		t.Fatalf("clientB transport type = %T, want *http.Transport", clientB.Transport)
	}

	if transportA != transportB {
		t.Fatal("expected Antigravity HTTP/1.1 transport to be shared across clients")
	}
	if transportA.ForceAttemptHTTP2 {
		t.Fatal("expected Antigravity transport to keep HTTP/2 disabled")
	}
	if transportA.TLSNextProto == nil {
		t.Fatal("expected Antigravity transport to disable HTTP/2 TLSNextProto")
	}
	if transportA.TLSClientConfig == nil || len(transportA.TLSClientConfig.NextProtos) != 1 || transportA.TLSClientConfig.NextProtos[0] != "http/1.1" {
		t.Fatalf("expected Antigravity transport to force HTTP/1.1 ALPN, got %#v", transportA.TLSClientConfig)
	}
}
