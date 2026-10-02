// Package llm is the LLM provider layer. Exactly one provider is used at a time:
//   - mock (default): deterministic, no network; answers from the supplied context;
//   - openai_compat: any OpenAI-compatible POST {base_url}/chat/completions (e.g. Yandex AI Studio).
//
// Region policy: no Gemini/Anthropic/OpenAI endpoints and no proxies/VPNs to bypass regions.
package llm

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// Message is a chat message.
type Message struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// Request to a provider.
type Request struct {
	Messages []Message
	// Facts is structured context for the mock provider (ignored by real providers,
	// which receive the same information rendered into Messages).
	Facts *Facts
}

// Usage of one call.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Response from a provider.
type Response struct {
	Content string
	Usage   Usage
	Model   string
}

// Provider is an LLM backend.
type Provider interface {
	Name() string
	Model() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// ErrBudgetExceeded is returned when the monthly budget is exhausted.
var ErrBudgetExceeded = errors.New("llm monthly budget exceeded")

// Candidate is a pre-ranked bottle passed to the model (and to the mock).
type Candidate struct {
	ItemID   string
	Title    string
	Score    int
	Window   string
	Reasons  []string
	Quantity int
}

// Facts is the structured context the assistant is built from.
type Facts struct {
	Locale     string
	Intent     string // pair_recipe | drink_now | buy_again | general
	RecipeName string
	Candidates []Candidate
	BuyAgain   []string
	UserName   string
}

// EstimateTokens is a cheap, provider-independent estimate (~4 bytes/token for Latin, ~2 runes for Cyrillic).
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	runes := utf8.RuneCountInString(s)
	n := runes / 3
	if n == 0 {
		n = 1
	}
	return n
}

// TrimToTokens cuts s to roughly max tokens on a line boundary.
func TrimToTokens(s string, max int) string {
	if max <= 0 || EstimateTokens(s) <= max {
		return s
	}
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for _, l := range lines {
		if EstimateTokens(b.String()+l+"\n") > max {
			break
		}
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}
