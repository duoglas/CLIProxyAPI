package helps

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestNewProxyAwareHTTPClientDirectBypassesGlobalProxy(t *testing.T) {
	t.Parallel()

	client := NewProxyAwareHTTPClient(
		context.Background(),
		&config.Config{SDKConfig: sdkconfig.SDKConfig{ProxyURL: "http://global-proxy.example.com:8080"}},
		&cliproxyauth.Auth{ProxyURL: "direct"},
		0,
	)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("expected direct transport to disable proxy function")
	}
}

func TestNewProxyAwareHTTPClientNoProxyUsesDefaultTransport(t *testing.T) {
	t.Parallel()

	first := NewProxyAwareHTTPClient(context.Background(), nil, nil, 0)
	second := NewProxyAwareHTTPClient(context.Background(), nil, nil, 0)

	if first == second {
		t.Fatal("expected distinct client wrappers for direct no-proxy path")
	}
	if first.Transport != nil {
		t.Fatalf("expected direct client to use default transport, got %T", first.Transport)
	}
	if second.Transport != nil {
		t.Fatalf("expected second direct client to use default transport, got %T", second.Transport)
	}
}

func TestNewProxyAwareHTTPClientProxyReusesCachedTransport(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{SDKConfig: sdkconfig.SDKConfig{ProxyURL: "http://proxy.example.com:8080"}}
	first := NewProxyAwareHTTPClient(context.Background(), cfg, nil, 0)
	second := NewProxyAwareHTTPClient(context.Background(), cfg, nil, 0)

	if first == second {
		t.Fatal("expected proxy calls to receive distinct client wrappers")
	}
	transport, ok := first.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", first.Transport)
	}
	if second.Transport != transport {
		t.Fatal("expected proxy transport to be reused across client wrappers")
	}
	proxyFunc := transport.Proxy
	if proxyFunc == nil {
		t.Fatal("expected proxy transport to configure proxy function")
	}
	targetURL, errParse := url.Parse("https://example.com")
	if errParse != nil {
		t.Fatalf("url.Parse() error = %v", errParse)
	}
	gotProxy, errProxy := proxyFunc(&http.Request{URL: targetURL})
	if errProxy != nil {
		t.Fatalf("proxy function error = %v", errProxy)
	}
	if gotProxy == nil || gotProxy.String() != "http://proxy.example.com:8080" {
		t.Fatalf("proxy = %v, want http://proxy.example.com:8080", gotProxy)
	}
}

func TestNewProxyAwareHTTPClientExplicitProxyConfiguresTransport(t *testing.T) {
	t.Parallel()

	client := NewProxyAwareHTTPClient(
		context.Background(),
		&config.Config{SDKConfig: sdkconfig.SDKConfig{ProxyURL: "http://config-proxy.example.com:8080"}},
		nil,
		0,
	)

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	req, errReq := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if errReq != nil {
		t.Fatalf("NewRequest() error = %v", errReq)
	}
	proxyURL, errProxy := transport.Proxy(req)
	if errProxy != nil {
		t.Fatalf("transport.Proxy() error = %v", errProxy)
	}
	if proxyURL == nil || proxyURL.String() != "http://config-proxy.example.com:8080" {
		t.Fatalf("proxy URL = %v, want http://config-proxy.example.com:8080", proxyURL)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewProxyAwareHTTPClientInvalidProxyFallsBackToContextTransport(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), "cliproxy.roundtripper", roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Request: req}, nil
	}))

	client := NewProxyAwareHTTPClient(ctx, &config.Config{
		SDKConfig: sdkconfig.SDKConfig{ProxyURL: "://bad-proxy"},
	}, nil, 0)

	if client.Transport == nil {
		t.Fatal("expected context transport fallback when proxy build fails")
	}
	if _, ok := client.Transport.(roundTripperFunc); !ok {
		t.Fatalf("transport type = %T, want context roundTripperFunc fallback", client.Transport)
	}
}
