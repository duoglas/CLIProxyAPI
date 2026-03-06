package chat_completions

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertOpenAIRequestToAntigravity_兼容ReasoningEffort回退(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       []byte
		expectPath  string
		expectValue string
	}{
		{
			name:        "flat reasoning_effort",
			input:       []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`),
			expectPath:  "request.generationConfig.thinkingConfig.thinkingLevel",
			expectValue: "high",
		},
		{
			name:        "nested reasoning.effort",
			input:       []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}],"reasoning":{"effort":"high"}}`),
			expectPath:  "request.generationConfig.thinkingConfig.thinkingLevel",
			expectValue: "high",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := ConvertOpenAIRequestToAntigravity("gpt-5.4", tt.input, false)
			value := gjson.GetBytes(out, tt.expectPath).String()
			if value != tt.expectValue {
				t.Fatalf("%s = %q, want %q", tt.expectPath, value, tt.expectValue)
			}
		})
	}
}
