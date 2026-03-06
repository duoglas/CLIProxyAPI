package thinking

import "testing"

func TestExtractOpenAIConfig_兼容Flat与Nested(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       []byte
		expectMode ThinkingMode
		expect     ThinkingLevel
	}{
		{
			name:       "flat reasoning_effort 优先",
			body:       []byte(`{"reasoning_effort":"high"}`),
			expectMode: ModeLevel,
			expect:     ThinkingLevel("high"),
		},
		{
			name:       "nested reasoning.effort 回退",
			body:       []byte(`{"reasoning":{"effort":"high"}}`),
			expectMode: ModeLevel,
			expect:     ThinkingLevel("high"),
		},
		{
			name:       "none 语义保持",
			body:       []byte(`{"reasoning":{"effort":"none"}}`),
			expectMode: ModeNone,
			expect:     ThinkingLevel(""),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := extractOpenAIConfig(tt.body)
			if cfg.Mode != tt.expectMode {
				t.Fatalf("mode = %q, want %q", cfg.Mode, tt.expectMode)
			}
			if cfg.Level != tt.expect {
				t.Fatalf("level = %q, want %q", cfg.Level, tt.expect)
			}
		})
	}
}

func TestExtractCodexConfig_兼容Nested与Flat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       []byte
		expectMode ThinkingMode
		expect     ThinkingLevel
	}{
		{
			name:       "nested reasoning.effort 优先",
			body:       []byte(`{"reasoning":{"effort":"high"}}`),
			expectMode: ModeLevel,
			expect:     ThinkingLevel("high"),
		},
		{
			name:       "flat reasoning_effort 回退",
			body:       []byte(`{"reasoning_effort":"high"}`),
			expectMode: ModeLevel,
			expect:     ThinkingLevel("high"),
		},
		{
			name:       "none 语义保持",
			body:       []byte(`{"reasoning_effort":"none"}`),
			expectMode: ModeNone,
			expect:     ThinkingLevel(""),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := extractCodexConfig(tt.body)
			if cfg.Mode != tt.expectMode {
				t.Fatalf("mode = %q, want %q", cfg.Mode, tt.expectMode)
			}
			if cfg.Level != tt.expect {
				t.Fatalf("level = %q, want %q", cfg.Level, tt.expect)
			}
		})
	}
}
