package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

func TestFillFirstSelectorPick_Deterministic(t *testing.T) {
	t.Parallel()

	selector := &FillFirstSelector{}
	auths := []*Auth{
		{ID: "b"},
		{ID: "a"},
		{ID: "c"},
	}

	got, err := selector.Pick(context.Background(), "gemini", "", cliproxyexecutor.Options{}, auths)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if got == nil {
		t.Fatalf("Pick() auth = nil")
	}
	if got.ID != "a" {
		t.Fatalf("Pick() auth.ID = %q, want %q", got.ID, "a")
	}
}

func TestRoundRobinSelectorPick_PicksFromAvailable(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}
	auths := []*Auth{
		{ID: "b"},
		{ID: "a"},
		{ID: "c"},
	}
	validIDs := map[string]bool{"a": true, "b": true, "c": true}

	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		got, err := selector.Pick(context.Background(), "gemini", "", cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		if got == nil {
			t.Fatalf("Pick() #%d auth = nil", i)
		}
		if !validIDs[got.ID] {
			t.Fatalf("Pick() #%d auth.ID = %q, not in valid set", i, got.ID)
		}
		seen[got.ID] = true
	}
	if len(seen) < 2 {
		t.Fatalf("Pick() only returned %d unique auths over 100 picks, expected diversity", len(seen))
	}
}

func TestRoundRobinSelectorPick_PriorityBuckets(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}
	auths := []*Auth{
		{ID: "c", Attributes: map[string]string{"priority": "0"}},
		{ID: "a", Attributes: map[string]string{"priority": "10"}},
		{ID: "b", Attributes: map[string]string{"priority": "10"}},
	}

	for i := 0; i < 20; i++ {
		got, err := selector.Pick(context.Background(), "mixed", "", cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		if got == nil {
			t.Fatalf("Pick() #%d auth = nil", i)
		}
		if got.ID == "c" {
			t.Fatalf("Pick() #%d unexpectedly selected lower priority auth", i)
		}
	}
}

func TestFillFirstSelectorPick_PriorityFallbackCooldown(t *testing.T) {
	t.Parallel()

	selector := &FillFirstSelector{}
	now := time.Now()
	model := "test-model"

	high := &Auth{
		ID:         "high",
		Attributes: map[string]string{"priority": "10"},
		ModelStates: map[string]*ModelState{
			model: {
				Status:         StatusActive,
				Unavailable:    true,
				NextRetryAfter: now.Add(30 * time.Minute),
				Quota: QuotaState{
					Exceeded: true,
				},
			},
		},
	}
	low := &Auth{ID: "low", Attributes: map[string]string{"priority": "0"}}

	got, err := selector.Pick(context.Background(), "mixed", model, cliproxyexecutor.Options{}, []*Auth{high, low})
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if got == nil {
		t.Fatalf("Pick() auth = nil")
	}
	if got.ID != "low" {
		t.Fatalf("Pick() auth.ID = %q, want %q", got.ID, "low")
	}
}

func TestRoundRobinSelectorPick_Concurrent(t *testing.T) {
	selector := &RoundRobinSelector{}
	auths := []*Auth{
		{ID: "b"},
		{ID: "a"},
		{ID: "c"},
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	goroutines := 32
	iterations := 100
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				got, err := selector.Pick(context.Background(), "gemini", "", cliproxyexecutor.Options{}, auths)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				if got == nil {
					select {
					case errCh <- errors.New("Pick() returned nil auth"):
					default:
					}
					return
				}
				if got.ID == "" {
					select {
					case errCh <- errors.New("Pick() returned auth with empty ID"):
					default:
					}
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatalf("concurrent Pick() error = %v", err)
	default:
	}
}

func TestSelectorPick_AllCooldownReturnsModelCooldownError(t *testing.T) {
	t.Parallel()

	model := "test-model"
	now := time.Now()
	next := now.Add(60 * time.Second)
	auths := []*Auth{
		{
			ID: "a",
			ModelStates: map[string]*ModelState{
				model: {
					Status:         StatusActive,
					Unavailable:    true,
					NextRetryAfter: next,
					Quota: QuotaState{
						Exceeded:      true,
						NextRecoverAt: next,
					},
				},
			},
		},
		{
			ID: "b",
			ModelStates: map[string]*ModelState{
				model: {
					Status:         StatusActive,
					Unavailable:    true,
					NextRetryAfter: next,
					Quota: QuotaState{
						Exceeded:      true,
						NextRecoverAt: next,
					},
				},
			},
		},
	}

	t.Run("mixed provider redacts provider field", func(t *testing.T) {
		t.Parallel()

		selector := &FillFirstSelector{}
		_, err := selector.Pick(context.Background(), "mixed", model, cliproxyexecutor.Options{}, auths)
		if err == nil {
			t.Fatalf("Pick() error = nil")
		}

		var mce *modelCooldownError
		if !errors.As(err, &mce) {
			t.Fatalf("Pick() error = %T, want *modelCooldownError", err)
		}
		if mce.StatusCode() != http.StatusTooManyRequests {
			t.Fatalf("StatusCode() = %d, want %d", mce.StatusCode(), http.StatusTooManyRequests)
		}

		headers := mce.Headers()
		if got := headers.Get("Retry-After"); got == "" {
			t.Fatalf("Headers().Get(Retry-After) = empty")
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(mce.Error()), &payload); err != nil {
			t.Fatalf("json.Unmarshal(Error()) error = %v", err)
		}
		rawErr, ok := payload["error"].(map[string]any)
		if !ok {
			t.Fatalf("Error() payload missing error object: %v", payload)
		}
		if got, _ := rawErr["code"].(string); got != "model_cooldown" {
			t.Fatalf("Error().error.code = %q, want %q", got, "model_cooldown")
		}
		if _, ok := rawErr["provider"]; ok {
			t.Fatalf("Error().error.provider exists for mixed provider: %v", rawErr["provider"])
		}
	})

	t.Run("non-mixed provider includes provider field", func(t *testing.T) {
		t.Parallel()

		selector := &FillFirstSelector{}
		_, err := selector.Pick(context.Background(), "gemini", model, cliproxyexecutor.Options{}, auths)
		if err == nil {
			t.Fatalf("Pick() error = nil")
		}

		var mce *modelCooldownError
		if !errors.As(err, &mce) {
			t.Fatalf("Pick() error = %T, want *modelCooldownError", err)
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(mce.Error()), &payload); err != nil {
			t.Fatalf("json.Unmarshal(Error()) error = %v", err)
		}
		rawErr, ok := payload["error"].(map[string]any)
		if !ok {
			t.Fatalf("Error() payload missing error object: %v", payload)
		}
		if got, _ := rawErr["provider"].(string); got != "gemini" {
			t.Fatalf("Error().error.provider = %q, want %q", got, "gemini")
		}
	})
}

func TestIsAuthBlockedForModel_UnavailableWithoutNextRetryIsNotBlocked(t *testing.T) {
	t.Parallel()

	now := time.Now()
	model := "test-model"
	auth := &Auth{
		ID: "a",
		ModelStates: map[string]*ModelState{
			model: {
				Status:      StatusActive,
				Unavailable: true,
				Quota: QuotaState{
					Exceeded: true,
				},
			},
		},
	}

	blocked, reason, next := isAuthBlockedForModel(auth, model, now)
	if blocked {
		t.Fatalf("blocked = true, want false")
	}
	if reason != blockReasonNone {
		t.Fatalf("reason = %v, want %v", reason, blockReasonNone)
	}
	if !next.IsZero() {
		t.Fatalf("next = %v, want zero", next)
	}
}

func TestFillFirstSelectorPick_ThinkingSuffixFallsBackToBaseModelState(t *testing.T) {
	t.Parallel()

	selector := &FillFirstSelector{}
	now := time.Now()

	baseModel := "test-model"
	requestedModel := "test-model(high)"

	high := &Auth{
		ID:         "high",
		Attributes: map[string]string{"priority": "10"},
		ModelStates: map[string]*ModelState{
			baseModel: {
				Status:         StatusActive,
				Unavailable:    true,
				NextRetryAfter: now.Add(30 * time.Minute),
				Quota: QuotaState{
					Exceeded: true,
				},
			},
		},
	}
	low := &Auth{
		ID:         "low",
		Attributes: map[string]string{"priority": "0"},
	}

	got, err := selector.Pick(context.Background(), "mixed", requestedModel, cliproxyexecutor.Options{}, []*Auth{high, low})
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}
	if got == nil {
		t.Fatalf("Pick() auth = nil")
	}
	if got.ID != "low" {
		t.Fatalf("Pick() auth.ID = %q, want %q", got.ID, "low")
	}
}

func TestRoundRobinSelectorPick_ThinkingSuffixPicksFromSamePool(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}
	auths := []*Auth{
		{ID: "b"},
		{ID: "a"},
	}
	validIDs := map[string]bool{"a": true, "b": true}

	for _, model := range []string{"test-model(high)", "test-model(low)", "test-model"} {
		got, err := selector.Pick(context.Background(), "gemini", model, cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() model=%q error = %v", model, err)
		}
		if got == nil || !validIDs[got.ID] {
			t.Fatalf("Pick() model=%q returned invalid auth", model)
		}
	}
}

func TestRoundRobinSelectorPick_SingleAuth(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}
	auths := []*Auth{{ID: "a"}}

	for i := 0; i < 10; i++ {
		got, err := selector.Pick(context.Background(), "gemini", "m1", cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		if got == nil || got.ID != "a" {
			t.Fatalf("Pick() #%d expected auth ID 'a', got %v", i, got)
		}
	}
}

func TestRoundRobinSelectorPick_GeminiCLICredentialGrouping(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}

	// Simulate two gemini-cli credentials, each with multiple projects:
	// Credential A (parent = "cred-a.json") has 3 projects
	// Credential B (parent = "cred-b.json") has 2 projects
	auths := []*Auth{
		{ID: "cred-a.json::proj-a1", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
		{ID: "cred-a.json::proj-a2", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
		{ID: "cred-a.json::proj-a3", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
		{ID: "cred-b.json::proj-b1", Attributes: map[string]string{"gemini_virtual_parent": "cred-b.json"}},
		{ID: "cred-b.json::proj-b2", Attributes: map[string]string{"gemini_virtual_parent": "cred-b.json"}},
	}

	validIDs := map[string]bool{
		"cred-a.json::proj-a1": true, "cred-a.json::proj-a2": true, "cred-a.json::proj-a3": true,
		"cred-b.json::proj-b1": true, "cred-b.json::proj-b2": true,
	}

	seenParents := map[string]bool{}
	for i := 0; i < 50; i++ {
		got, err := selector.Pick(context.Background(), "gemini-cli", "gemini-2.5-pro", cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		if got == nil || !validIDs[got.ID] {
			t.Fatalf("Pick() #%d returned invalid auth", i)
		}
		seenParents[got.Attributes["gemini_virtual_parent"]] = true
	}
	if len(seenParents) < 2 {
		t.Fatalf("Expected picks from both credential groups, only saw %v", seenParents)
	}
}

func TestRoundRobinSelectorPick_SingleParentFallsBackToFlat(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}

	// All auths from the same parent - should fall back to flat round-robin
	// because there's only one credential group (no benefit from two-level).
	auths := []*Auth{
		{ID: "cred-a.json::proj-a1", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
		{ID: "cred-a.json::proj-a2", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
		{ID: "cred-a.json::proj-a3", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
	}

	validIDs := map[string]bool{
		"cred-a.json::proj-a1": true, "cred-a.json::proj-a2": true, "cred-a.json::proj-a3": true,
	}

	for i := 0; i < 20; i++ {
		got, err := selector.Pick(context.Background(), "gemini-cli", "gemini-2.5-pro", cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		if got == nil || !validIDs[got.ID] {
			t.Fatalf("Pick() #%d returned invalid auth", i)
		}
	}
}

func TestRoundRobinSelectorPick_MixedVirtualAndNonVirtualFallsBackToFlat(t *testing.T) {
	t.Parallel()

	selector := &RoundRobinSelector{}

	// Mix of virtual and non-virtual auths (e.g., a regular gemini-cli auth without projects
	// alongside virtual ones). Should fall back to flat round-robin.
	auths := []*Auth{
		{ID: "cred-a.json::proj-a1", Attributes: map[string]string{"gemini_virtual_parent": "cred-a.json"}},
		{ID: "cred-regular.json"}, // no gemini_virtual_parent
	}

	validIDs := map[string]bool{"cred-a.json::proj-a1": true, "cred-regular.json": true}

	for i := 0; i < 20; i++ {
		got, err := selector.Pick(context.Background(), "gemini-cli", "", cliproxyexecutor.Options{}, auths)
		if err != nil {
			t.Fatalf("Pick() #%d error = %v", i, err)
		}
		if got == nil || !validIDs[got.ID] {
			t.Fatalf("Pick() #%d returned invalid auth", i)
		}
	}
}
