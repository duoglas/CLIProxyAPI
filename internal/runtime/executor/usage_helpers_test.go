package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestParseOpenAIUsageChatCompletions(t *testing.T) {
	data := []byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":5}}}`)
	detail := parseOpenAIUsage(data)
	if detail.InputTokens != 1 {
		t.Fatalf("input tokens = %d, want %d", detail.InputTokens, 1)
	}
	if detail.OutputTokens != 2 {
		t.Fatalf("output tokens = %d, want %d", detail.OutputTokens, 2)
	}
	if detail.TotalTokens != 3 {
		t.Fatalf("total tokens = %d, want %d", detail.TotalTokens, 3)
	}
	if detail.CachedTokens != 4 {
		t.Fatalf("cached tokens = %d, want %d", detail.CachedTokens, 4)
	}
	if detail.ReasoningTokens != 5 {
		t.Fatalf("reasoning tokens = %d, want %d", detail.ReasoningTokens, 5)
	}
}

func TestParseOpenAIUsageResponses(t *testing.T) {
	data := []byte(`{"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30,"input_tokens_details":{"cached_tokens":7},"output_tokens_details":{"reasoning_tokens":9}}}`)
	detail := parseOpenAIUsage(data)
	if detail.InputTokens != 10 {
		t.Fatalf("input tokens = %d, want %d", detail.InputTokens, 10)
	}
	if detail.OutputTokens != 20 {
		t.Fatalf("output tokens = %d, want %d", detail.OutputTokens, 20)
	}
	if detail.TotalTokens != 30 {
		t.Fatalf("total tokens = %d, want %d", detail.TotalTokens, 30)
	}
	if detail.CachedTokens != 7 {
		t.Fatalf("cached tokens = %d, want %d", detail.CachedTokens, 7)
	}
	if detail.ReasoningTokens != 9 {
		t.Fatalf("reasoning tokens = %d, want %d", detail.ReasoningTokens, 9)
	}
}

func TestUsageReporterBuildRecordIncludesLatency(t *testing.T) {
	reporter := &usageReporter{
		provider:    "openai",
		model:       "gpt-5.4",
		requestedAt: time.Now().Add(-1500 * time.Millisecond),
	}

	record := reporter.buildRecord(usage.Detail{TotalTokens: 3}, false)
	if record.Latency < time.Second {
		t.Fatalf("latency = %v, want >= 1s", record.Latency)
	}
	if record.Latency > 3*time.Second {
		t.Fatalf("latency = %v, want <= 3s", record.Latency)
	}
}

type usageCapturePlugin struct {
	ch chan usage.Record
}

func (p *usageCapturePlugin) HandleUsage(_ context.Context, record usage.Record) {
	select {
	case p.ch <- record:
	default:
	}
}

func awaitUsageRecord(t *testing.T, ch <-chan usage.Record, provider string) usage.Record {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case record := <-ch:
			if record.Provider == provider {
				return record
			}
		case <-deadline:
			t.Fatalf("timed out waiting for usage record for provider %q", provider)
		}
	}
}

func TestUsageReporterFinalizePublishesFailureOnError(t *testing.T) {
	plugin := &usageCapturePlugin{ch: make(chan usage.Record, 8)}
	usage.RegisterPlugin(plugin)

	reporter := &usageReporter{
		provider:    "test-finalize-failure",
		model:       "model",
		requestedAt: time.Now(),
	}
	err := errors.New("boom")

	reporter.finalize(context.Background(), &err)

	record := awaitUsageRecord(t, plugin.ch, reporter.provider)
	if !record.Failed {
		t.Fatalf("record.Failed = false, want true")
	}
}

func TestUsageReporterFinalizePublishesSuccessWithoutError(t *testing.T) {
	plugin := &usageCapturePlugin{ch: make(chan usage.Record, 8)}
	usage.RegisterPlugin(plugin)

	reporter := &usageReporter{
		provider:    "test-finalize-success",
		model:       "model",
		requestedAt: time.Now(),
	}
	var err error

	reporter.finalize(context.Background(), &err)

	record := awaitUsageRecord(t, plugin.ch, reporter.provider)
	if record.Failed {
		t.Fatalf("record.Failed = true, want false")
	}
}
