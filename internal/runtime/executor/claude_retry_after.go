package executor

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// parseClaudeRetryAfter inspects an Anthropic 429 response's headers and
// returns how long the caller should wait before retrying. Priority:
//  1. RFC 7231 Retry-After (integer seconds or HTTP-date)
//  2. anthropic-ratelimit-unified-reset (Unix epoch seconds)
//
// Returns nil when nothing parses or the resolved duration is non-positive.
func parseClaudeRetryAfter(h http.Header) *time.Duration {
	if h == nil {
		return nil
	}
	if v := strings.TrimSpace(h.Get("Retry-After")); v != "" {
		if secs, err := strconv.Atoi(v); err == nil {
			if secs > 0 {
				d := time.Duration(secs) * time.Second
				return &d
			}
			return nil
		}
		if t, err := http.ParseTime(v); err == nil {
			d := time.Until(t)
			if d > 0 {
				return &d
			}
			return nil
		}
	}
	if v := strings.TrimSpace(h.Get("Anthropic-Ratelimit-Unified-Reset")); v != "" {
		if epoch, err := strconv.ParseInt(v, 10, 64); err == nil {
			d := time.Until(time.Unix(epoch, 0))
			if d > 0 {
				return &d
			}
		}
	}
	return nil
}
