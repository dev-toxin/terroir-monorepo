package llm

import (
	"context"
	"strings"
	"testing"
)

func facts(intent, locale string) *Facts {
	return &Facts{Locale: locale, Intent: intent, RecipeName: "Стейк",
		Candidates: []Candidate{
			{ItemID: "i-1", Title: "Саперави 2018", Window: "пик", Reasons: []string{"танины", "тело"}},
			{ItemID: "i-2", Title: "Сира 2020"},
			{ItemID: "i-3", Title: "Мерло 2016"},
			{ItemID: "i-4", Title: "Кокур 2022"},
		}}
}

func TestMock_DeterministicAndReferences(t *testing.T) {
	m := Mock{}
	r := Request{Messages: []Message{{Role: "user", Content: "что открыть к стейку"}}, Facts: facts("pair_recipe", "ru")}
	a, err := m.Complete(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Complete(context.Background(), r)
	if a.Content != b.Content {
		t.Fatal("mock must be deterministic")
	}
	for _, want := range []string{"[item:i-1]", "[item:i-2]", "[item:i-3]", "Стейк", "танины; тело"} {
		if !strings.Contains(a.Content, want) {
			t.Fatalf("missing %q in %q", want, a.Content)
		}
	}
	if strings.Contains(a.Content, "[item:i-4]") {
		t.Fatal("only 2 alternatives expected")
	}
	if a.Usage.PromptTokens == 0 || a.Usage.CompletionTokens == 0 || a.Model != m.Model() || m.Name() != "mock" {
		t.Fatalf("usage/model %+v", a)
	}
}

func TestMock_Intents(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		f    *Facts
		want []string
	}{
		{facts("drink_now", "ru"), []string{"Пора открыть", "[item:i-1]", "пик"}},
		{facts("general", "ru"), []string{"Из вашего погреба", "[item:i-1]"}},
		{facts("pair_recipe", "en"), []string{"I would open", "Why:", "Alternatives:"}},
		{facts("drink_now", "en"), []string{"Time to open"}},
		{facts("general", "en"), []string{"From your cellar"}},
		{&Facts{Locale: "ru", Intent: "pair_recipe"}, []string{"нет подходящих"}},
		{&Facts{Locale: "en", Intent: "general"}, []string{"no suitable bottles"}},
		{&Facts{Locale: "ru", Intent: "buy_again"}, []string{"Пока нет вин"}},
		{&Facts{Locale: "en", Intent: "buy_again"}, []string{"No wines rated"}},
		{&Facts{Locale: "ru", Intent: "buy_again", BuyAgain: []string{"Саперави — 5/5"}}, []string{"купить снова", "• Саперави — 5/5"}},
		{&Facts{Locale: "en", Intent: "buy_again", BuyAgain: []string{"Saperavi"}}, []string{"Worth buying again", "• Saperavi"}},
		{nil, []string{"нет подходящих"}},
	}
	for i, c := range cases {
		resp, err := Mock{}.Complete(ctx, Request{Facts: c.f})
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range c.want {
			if !strings.Contains(resp.Content, w) {
				t.Errorf("case %d: missing %q in %q", i, w, resp.Content)
			}
		}
		if c.f != nil && c.f.Intent == "buy_again" && strings.Contains(resp.Content, "[item:") {
			t.Errorf("case %d: buy_again must not reference cellar items", i)
		}
	}
}

func TestEstimateAndTrimTokens(t *testing.T) {
	if EstimateTokens("") != 0 || EstimateTokens("ab") != 1 || EstimateTokens("абвгдеё") != 2 {
		t.Fatal("estimate")
	}
	s := "line one\nline two\nline three\n"
	if TrimToTokens(s, 0) != s || TrimToTokens(s, 1000) != s {
		t.Fatal("no trim expected")
	}
	got := TrimToTokens(s, 6)
	if !strings.HasPrefix(s, got) || got == s || !strings.HasSuffix(got, "\n") {
		t.Fatalf("trim on line boundary, got %q", got)
	}
	if EstimateTokens(got) > 6 {
		t.Fatalf("over budget: %q", got)
	}
}

func TestSystemPrompt(t *testing.T) {
	if !strings.Contains(SystemPrompt("ru"), "[item:") || !strings.Contains(SystemPrompt("en"), "buy") {
		t.Fatal("prompts must be embedded")
	}
	if SystemPrompt("xx") != SystemPrompt("ru") {
		t.Fatal("ru default")
	}
}
