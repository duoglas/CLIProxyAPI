package management

import (
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type authCleanupRequest struct {
	DryRun      *bool `json:"dry_run"`
	WindowHours *int  `json:"window_hours"`
}

func (h *Handler) GetAuthsHealth(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	windowHours, ok := parseWindowHours(c.Query("window_hours"), 24)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window_hours must be an integer between 1 and 720"})
		return
	}
	window := time.Duration(windowHours) * time.Hour
	now := time.Now()
	auths := h.authManager.List()
	items := make([]gin.H, 0, len(auths))
	candidates := 0
	for i := range auths {
		auth := auths[i]
		if auth == nil {
			continue
		}
		candidate := isCleanupCandidate(auth, now, window)
		if candidate {
			candidates++
		}
		items = append(items, gin.H{
			"id":                      strings.TrimSpace(auth.ID),
			"name":                    strings.TrimSpace(auth.FileName),
			"provider":                strings.TrimSpace(auth.Provider),
			"status":                  auth.Status,
			"disabled":                auth.Disabled,
			"quarantined":             auth.Quarantined,
			"consecutive_failures":    auth.ConsecutiveFailures,
			"temp_disabled_until":     auth.TempDisabledUntil,
			"guard_window_started_at": auth.GuardWindowStartedAt,
			"guard_window_successes":  auth.GuardWindowSuccesses,
			"guard_window_failures":   auth.GuardWindowFailures,
			"last_success_at":         auth.LastSuccessAt,
			"last_failure_at":         auth.LastFailureAt,
			"cleanup_candidate":       candidate,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		idI, _ := items[i]["id"].(string)
		idJ, _ := items[j]["id"].(string)
		return strings.ToLower(idI) < strings.ToLower(idJ)
	})
	c.JSON(http.StatusOK, gin.H{
		"window_hours": windowHours,
		"total":        len(items),
		"candidates":   candidates,
		"items":        items,
	})
}

func (h *Handler) CleanupAuths(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	req := authCleanupRequest{}
	if c.Request.Body != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
				return
			}
		}
	}
	windowHours := 24
	if req.WindowHours != nil {
		if *req.WindowHours < 1 || *req.WindowHours > 720 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "window_hours must be an integer between 1 and 720"})
			return
		}
		windowHours = *req.WindowHours
	}
	if q := strings.TrimSpace(c.Query("window_hours")); q != "" {
		parsed, ok := parseWindowHours(q, windowHours)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "window_hours must be an integer between 1 and 720"})
			return
		}
		windowHours = parsed
	}
	dryRun := false
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	if q := strings.TrimSpace(c.Query("dry_run")); q != "" {
		parsed, err := strconv.ParseBool(q)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "dry_run must be a boolean"})
			return
		}
		dryRun = parsed
	}

	window := time.Duration(windowHours) * time.Hour
	now := time.Now()
	auths := h.authManager.List()
	manifest := make([]gin.H, 0)
	updated := 0
	failed := make([]gin.H, 0)
	for i := range auths {
		auth := auths[i]
		if !isCleanupCandidate(auth, now, window) {
			continue
		}
		item := gin.H{
			"id":           strings.TrimSpace(auth.ID),
			"name":         strings.TrimSpace(auth.FileName),
			"provider":     strings.TrimSpace(auth.Provider),
			"dry_run":      dryRun,
			"before_state": auth.Status,
		}
		if !dryRun {
			auth.Disabled = true
			auth.Quarantined = true
			auth.Status = coreauth.StatusDisabled
			auth.StatusMessage = "auto quarantined by management cleanup"
			auth.UpdatedAt = now
			if _, err := h.authManager.Update(c.Request.Context(), auth); err != nil {
				failed = append(failed, gin.H{
					"id":    strings.TrimSpace(auth.ID),
					"error": err.Error(),
				})
				continue
			}
			updated++
		}
		manifest = append(manifest, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"dry_run":      dryRun,
		"window_hours": windowHours,
		"candidates":   len(manifest),
		"updated":      updated,
		"failed":       failed,
		"items":        manifest,
	})
}

func parseWindowHours(raw string, fallback int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	if value < 1 || value > 720 {
		return 0, false
	}
	return value, true
}

func isCleanupCandidate(auth *coreauth.Auth, now time.Time, window time.Duration) bool {
	if auth == nil {
		return false
	}
	if auth.GuardWindowSuccesses != 0 || auth.GuardWindowFailures <= 3 {
		return false
	}
	if auth.GuardWindowStartedAt.IsZero() {
		return false
	}
	if now.Sub(auth.GuardWindowStartedAt) > window {
		return false
	}
	return true
}
