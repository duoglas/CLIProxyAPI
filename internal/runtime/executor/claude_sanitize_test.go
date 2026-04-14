package executor

import (
    "strings"
    "testing"
    "github.com/tidwall/gjson"
)

func TestSanitizeClaudeThinkingBlocks_RemovesInvalidThinkingSignatures(t *testing.T) {
    input := []byte(`{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"unsigned"},{"type":"thinking","thinking":"sentinel","signature":"skip_thought_signature_validator"},{"type":"thinking","thinking":"prefixed","signature":"claude#abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"},{"type":"thinking","thinking":"valid","signature":"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"},{"type":"text","text":"keep me"}]},{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
    out := sanitizeClaudeThinkingBlocks(input)
    content := gjson.GetBytes(out, "messages.0.content").Array()
    if len(content) != 2 {
        t.Fatalf("content len = %d, want 2; body=%s", len(content), string(out))
    }
    if got := content[0].Get("type").String(); got != "thinking" {
        t.Fatalf("content[0].type = %q, want thinking; body=%s", got, string(out))
    }
    if got := content[0].Get("signature").String(); got != "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz" {
        t.Fatalf("thinking signature = %q", got)
    }
    if got := content[1].Get("type").String(); got != "text" {
        t.Fatalf("content[1].type = %q, want text; body=%s", got, string(out))
    }
}

func TestIsAnthropicThinkingSignature(t *testing.T) {
    cases := []struct {
        name string
        sig  string
        want bool
    }{
        {"empty", "", false},
        {"sentinel", "skip_thought_signature_validator", false},
        {"prefixed", "claude#abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", false},
        {"short", "short-signature", false},
        {"valid", "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", true},
    }
    for _, tc := range cases {
        if got := isAnthropicThinkingSignature(tc.sig); got != tc.want {
            t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
        }
    }
}

func TestHasSignedAssistantThinking(t *testing.T) {
    cases := []struct {
        name string
        body string
        want bool
    }{
        {
            name: "no messages field",
            body: `{}`,
            want: false,
        },
        {
            name: "user-only conversation",
            body: `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`,
            want: false,
        },
        {
            name: "assistant text only, no thinking",
            body: `{"messages":[{"role":"assistant","content":[{"type":"text","text":"hello"}]}]}`,
            want: false,
        },
        {
            name: "assistant thinking without signature",
            body: `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"x"}]}]}`,
            want: false,
        },
        {
            name: "assistant thinking with empty signature",
            body: `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"x","signature":""}]}]}`,
            want: false,
        },
        {
            name: "assistant thinking with whitespace signature",
            body: `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"x","signature":"   "}]}]}`,
            want: false,
        },
        {
            name: "assistant thinking with real-looking signature",
            body: `{"messages":[{"role":"user","content":[{"type":"text","text":"q"}]},{"role":"assistant","content":[{"type":"thinking","thinking":"reasoning","signature":"abc123"},{"type":"text","text":"answer"}]},{"role":"user","content":[{"type":"text","text":"q2"}]}]}`,
            want: true,
        },
        {
            name: "thinking in user role (ignored)",
            body: `{"messages":[{"role":"user","content":[{"type":"thinking","thinking":"x","signature":"abc123"}]}]}`,
            want: false,
        },
    }
    for _, tc := range cases {
        if got := hasSignedAssistantThinking([]byte(tc.body)); got != tc.want {
            t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
        }
    }
}

func TestGenerateBillingHeader_StableAcrossTurns(t *testing.T) {
    // Two turns of the same conversation differ only in `messages` (a new user
    // turn appended). system and tools are unchanged — the billing header must
    // be byte-identical so Anthropic's thinking signature validates on replay.
    turn1 := []byte(`{
        "system":[{"type":"text","text":"you are helpful"}],
        "tools":[{"name":"bash","description":"run shell"}],
        "messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]
    }`)
    turn2 := []byte(`{
        "system":[{"type":"text","text":"you are helpful"}],
        "tools":[{"name":"bash","description":"run shell"}],
        "messages":[
            {"role":"user","content":[{"type":"text","text":"hi"}]},
            {"role":"assistant","content":[{"type":"thinking","thinking":"...","signature":"sig"},{"type":"text","text":"hello"}]},
            {"role":"user","content":[{"type":"text","text":"next"}]}
        ]
    }`)

    h1 := generateBillingHeader(turn1, "2.1.63")
    h2 := generateBillingHeader(turn2, "2.1.63")
    if h1 != h2 {
        t.Fatalf("billing header drifted between turns:\n t1=%s\n t2=%s", h1, h2)
    }
    if !strings.Contains(h1, "cc_version=2.1.63.") {
        t.Fatalf("unexpected billing header format: %s", h1)
    }
    if !strings.Contains(h1, "cch=") {
        t.Fatalf("billing header missing cch: %s", h1)
    }
}

func TestGenerateBillingHeader_ChangesWhenSystemChanges(t *testing.T) {
    // Sanity: if system content actually changes, the header is expected to
    // differ — this is NOT a bug; it reflects a different session.
    a := []byte(`{"system":[{"type":"text","text":"A"}],"tools":[]}`)
    b := []byte(`{"system":[{"type":"text","text":"B"}],"tools":[]}`)
    if generateBillingHeader(a, "") == generateBillingHeader(b, "") {
        t.Fatalf("billing header should differ when system content differs")
    }
}

func TestGenerateBillingHeader_DefaultVersion(t *testing.T) {
    // When version is empty, fall back to the hardcoded default.
    out := generateBillingHeader([]byte(`{}`), "")
    if !strings.Contains(out, "cc_version=2.1.63.") {
        t.Fatalf("expected default version 2.1.63, got %s", out)
    }
}

