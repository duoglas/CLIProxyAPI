package cliproxy

import (
	"net/http"
	"strings"
	"sync"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/proxyutil"
	log "github.com/sirupsen/logrus"
)

// defaultRoundTripperProvider returns a per-auth HTTP RoundTripper based on
// the Auth.ProxyURL value. It caches transports per proxy URL string.
type defaultRoundTripperProvider struct {
	mu    sync.RWMutex
	cache map[string]http.RoundTripper
}

func newDefaultRoundTripperProvider() *defaultRoundTripperProvider {
	return &defaultRoundTripperProvider{cache: make(map[string]http.RoundTripper)}
}

// RoundTripperFor implements coreauth.RoundTripperProvider.
// Each credential gets its own transport to prevent HTTP/2 connection multiplexing
// from leaking credential correlations to the upstream server.
func (p *defaultRoundTripperProvider) RoundTripperFor(auth *coreauth.Auth) http.RoundTripper {
	if auth == nil {
		return nil
	}
	proxyStr := strings.TrimSpace(auth.ProxyURL)
	if proxyStr == "" {
		return nil
	}
	// Use auth ID + proxy URL as cache key so each credential gets an isolated
	// connection pool, preventing HTTP/2 multiplexing from correlating accounts.
	cacheKey := auth.ID + "::" + proxyStr
	p.mu.RLock()
	rt := p.cache[cacheKey]
	p.mu.RUnlock()
	if rt != nil {
		return rt
	}
	transport, _, errBuild := proxyutil.BuildHTTPTransport(proxyStr)
	if errBuild != nil {
		log.Errorf("%v", errBuild)
		return nil
	}
	if transport == nil {
		return nil
	}
	p.mu.Lock()
	// Re-check under write lock to avoid creating duplicate transports
	// when multiple goroutines race on the same cache key.
	if existing := p.cache[cacheKey]; existing != nil {
		p.mu.Unlock()
		return existing
	}
	p.cache[cacheKey] = transport
	p.mu.Unlock()
	return transport
}
