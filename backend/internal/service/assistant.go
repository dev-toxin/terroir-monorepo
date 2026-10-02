package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/llm"
	"github.com/anko-soft-lab/terroir/backend/internal/pairing"
)

// Assistant answers chat messages from the cellar, profile, recipes and memory.
//
// Pipeline: detect intent (deterministic) → rank candidates with the pairing engine →
// build compact context → LLM (mock by default) → post-validate bottle references →
// fall back to the deterministic answer if the model mentions bottles that are not in stock.
type Assistant struct {
	svc           *Service
	provider      llm.Provider
	budget        *llm.Budget
	contextTokens int
	log           *slog.Logger
}

// NewAssistant wires the assistant.
func NewAssistant(svc *Service, p llm.Provider, b *llm.Budget, contextTokens int, log *slog.Logger) *Assistant {
	if contextTokens <= 0 {
		contextTokens = 3000
	}
	if log == nil {
		log = slog.Default()
	}
	return &Assistant{svc: svc, provider: p, budget: b, contextTokens: contextTokens, log: log}
}

// Provider returns the configured provider.
func (a *Assistant) Provider() llm.Provider { return a.provider }

// ChatRequest is the body of POST /v1/chat.
type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id"`
}

// BottleRef is a bottle referenced in an answer.
type BottleRef struct {
	ItemID   string `json:"item_id"`
	WineID   string `json:"wine_id"`
	Title    string `json:"title"`
	Quantity int    `json:"quantity"`
	Location string `json:"storage_location"`
	Window   string `json:"window"`
}

// ChatResponse is returned by POST /v1/chat.
type ChatResponse struct {
	ConversationID string      `json:"conversation_id"`
	Answer         string      `json:"answer"`
	Intent         string      `json:"intent"`
	Recipe         string      `json:"recipe,omitempty"`
	Bottles        []BottleRef `json:"bottles"`
	Provider       string      `json:"provider"`
	Fallback       bool        `json:"fallback"`
}

var itemRef = regexp.MustCompile(`\[item:([A-Za-z0-9_\-]+)\]`)

// Chat handles one user message.
func (a *Assistant) Chat(ctx context.Context, act Actor, req ChatRequest) (ChatResponse, error) {
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		return ChatResponse{}, domain.Invalid("message", "is required")
	}
	if len([]rune(msg)) > 2000 {
		return ChatResponse{}, domain.Invalid("message", "is too long (max 2000 characters)")
	}
	conv := req.ConversationID
	if conv == "" {
		conv = a.svc.newID()
	}
	s := a.svc
	locale := act.User.Locale
	if detectEnglish(msg) {
		locale = domain.LocaleEN
	}

	entries, err := s.store.ListItems(ctx, act.CellarID, ItemFilter{}.CellarFilter)
	if err != nil {
		return ChatResponse{}, err
	}
	profile, err := s.store.GetProfile(ctx, act.User.ID)
	if err != nil {
		return ChatResponse{}, err
	}
	recipes, err := s.store.ListRecipes(ctx, act.CellarID)
	if err != nil {
		return ChatResponse{}, err
	}
	fb, err := s.store.ListPairingFeedback(ctx, act.CellarID)
	if err != nil {
		return ChatResponse{}, err
	}
	facts, err := s.store.ListFacts(ctx, act.User.ID, 10)
	if err != nil {
		return ChatResponse{}, err
	}
	history, err := s.store.ListMessages(ctx, conv, 6)
	if err != nil {
		return ChatResponse{}, err
	}

	intent, recipe := DetectIntent(msg, recipes)
	year := s.clock.Now().Year()
	f := &llm.Facts{Locale: string(locale), Intent: intent, UserName: act.User.DisplayName}
	var cands []pairing.Candidate
	switch intent {
	case IntentPairRecipe:
		f.RecipeName = recipe.Title
		cands = pairing.RankWines(entries, recipe, &profile, year, fb)
	case IntentBuyAgain:
		items, err := s.BuyAgain(ctx, act)
		if err != nil {
			return ChatResponse{}, err
		}
		for _, it := range items {
			stock := pick(locale, fmt.Sprintf("в погребе: %d", it.InStock), fmt.Sprintf("in cellar: %d", it.InStock))
			f.BuyAgain = append(f.BuyAgain, fmt.Sprintf("%s — %.1f/5, %s", WineTitle(it.Wine), it.AvgRating, stock))
		}
	default:
		cands = rankDrinkNow(entries, &profile, year)
	}
	for i, c := range cands {
		if i == 5 {
			break
		}
		if c.Result.Score < 35 && intent == IntentPairRecipe {
			continue // the engine considers it a bad match; don't offer it
		}
		win := c.Window.Label(locale)
		f.Candidates = append(f.Candidates, llm.Candidate{ItemID: c.Entry.Item.ID, Title: WineTitle(c.Entry.Wine), Score: c.Result.Score,
			Window: win, Reasons: pairing.TopReasons(c.Result, locale, 3), Quantity: c.Entry.Item.Quantity})
	}

	contextText := BuildContext(locale, profile, entries, recipes, facts, f, year, a.contextTokens)
	messages := []llm.Message{{Role: "system", Content: llm.SystemPrompt(string(locale)) + "\n\n" + contextText}}
	for _, h := range history {
		messages = append(messages, llm.Message{Role: h.Role, Content: h.Content})
	}
	messages = append(messages, llm.Message{Role: "user", Content: msg})

	prompt := 0
	for _, m := range messages {
		prompt += llm.EstimateTokens(m.Content)
	}
	if err := a.budget.Check(ctx, prompt); err != nil {
		if errors.Is(err, llm.ErrBudgetExceeded) {
			return ChatResponse{}, domain.ErrBudget
		}
		return ChatResponse{}, err
	}

	resp, callErr := a.provider.Complete(ctx, llm.Request{Messages: messages, Facts: f})
	fallback := false
	answer := resp.Content
	if callErr != nil {
		a.log.Warn("llm call failed, using deterministic answer", slog.String("provider", a.provider.Name()), slog.String("error", callErr.Error()))
		fallback = true
	} else {
		usage := domain.LLMUsage{ID: s.newID(), UserID: act.User.ID, Provider: a.provider.Name(), Model: resp.Model,
			PromptTokens: resp.Usage.PromptTokens, CompletionTokens: resp.Usage.CompletionTokens, CreatedAt: s.clock.Now()}
		if a.budget != nil {
			usage.CostMicros = a.budget.Cost(resp.Usage)
		}
		if err := s.store.AddUsage(ctx, usage); err != nil {
			return ChatResponse{}, err
		}
	}

	inStock := map[string]domain.CellarEntry{}
	for _, e := range entries {
		if e.Item.Quantity > 0 {
			inStock[e.Item.ID] = e
		}
	}
	refs, ok := validateRefs(answer, inStock)
	if !fallback && (!ok || (len(refs) == 0 && len(f.Candidates) > 0)) {
		a.log.Info("llm answer failed validation, using deterministic answer", slog.String("intent", intent))
		fallback = true
	}
	if fallback {
		det, _ := llm.Mock{}.Complete(ctx, llm.Request{Facts: f})
		answer = det.Content
		refs, _ = validateRefs(answer, inStock)
	}

	out := ChatResponse{ConversationID: conv, Answer: answer, Intent: intent, Bottles: make([]BottleRef, 0, len(refs)),
		Provider: a.provider.Name(), Fallback: fallback}
	if intent == IntentPairRecipe {
		out.Recipe = recipe.Title
	}
	for _, id := range refs {
		e := inStock[id]
		out.Bottles = append(out.Bottles, BottleRef{ItemID: id, WineID: e.Wine.ID, Title: WineTitle(e.Wine), Quantity: e.Item.Quantity,
			Location: e.Item.StorageLocation, Window: string(e.Item.Window(s.clock.Now()))})
	}

	now := s.clock.Now()
	if err := s.store.AddMessage(ctx, domain.ChatMessage{ID: s.newID(), ConversationID: conv, UserID: act.User.ID, Role: "user", Content: msg, CreatedAt: now}); err != nil {
		return out, err
	}
	if err := s.store.AddMessage(ctx, domain.ChatMessage{ID: s.newID(), ConversationID: conv, UserID: act.User.ID, Role: "assistant", Content: answer, CreatedAt: now.Add(1)}); err != nil {
		return out, err
	}
	if fact := ExtractFact(msg); fact != "" {
		if err := s.store.AddFact(ctx, domain.MemoryFact{ID: s.newID(), UserID: act.User.ID, Fact: fact, Source: "chat", CreatedAt: now}); err != nil {
			return out, err
		}
	}
	return out, nil
}

// validateRefs returns unique referenced item ids; ok=false if any is not in stock.
func validateRefs(answer string, inStock map[string]domain.CellarEntry) ([]string, bool) {
	seen := map[string]bool{}
	var out []string
	ok := true
	for _, m := range itemRef.FindAllStringSubmatch(answer, -1) {
		id := m[1]
		if _, exists := inStock[id]; !exists {
			ok = false
			continue
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, ok
}

func rankDrinkNow(entries []domain.CellarEntry, p *domain.TasteProfile, year int) []pairing.Candidate {
	var out []pairing.Candidate
	for _, e := range entries {
		if e.Item.Quantity <= 0 {
			continue
		}
		win := domain.DrinkingWindow(e.Item.DrinkFrom, e.Item.DrinkUntil, year)
		if !win.ReadyToDrink() && win != domain.WindowLate {
			continue
		}
		score := win.Urgency() * 10
		var reasons []pairing.Reason
		reasons = append(reasons, pairing.Reason{Code: "window", Delta: score, RU: win.Label(domain.LocaleRU), EN: win.Label(domain.LocaleEN)})
		for _, g := range e.Wine.Grapes {
			if p.Likes(g) {
				score += 5
				reasons = append(reasons, pairing.Reason{Code: "liked", Delta: 5, RU: g + " — среди любимых сортов", EN: g + " is one of your favourite grapes"})
				break
			}
			if p.Rejects(g) {
				score -= 30
			}
		}
		out = append(out, pairing.Candidate{Entry: e, Window: win, Result: pairing.Result{Score: score, Reasons: reasons}})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Result.Score != out[j].Result.Score {
			return out[i].Result.Score > out[j].Result.Score
		}
		return untilYear(out[i].Entry) < untilYear(out[j].Entry)
	})
	return out
}

func untilYear(e domain.CellarEntry) int {
	if e.Item.DrinkUntil == nil {
		return 9999
	}
	return *e.Item.DrinkUntil
}

func pick(l domain.Locale, ru, en string) string {
	if l == domain.LocaleEN {
		return en
	}
	return ru
}

var latin = regexp.MustCompile(`[A-Za-z]`)
var cyr = regexp.MustCompile(`[А-Яа-яЁё]`)

func detectEnglish(s string) bool {
	return len(latin.FindAllString(s, -1)) > 3*len(cyr.FindAllString(s, -1)) && len(latin.FindAllString(s, -1)) > 5
}

var factPattern = regexp.MustCompile(`(?i)(не люблю|терпеть не могу|обожаю|я люблю|мне нравится|мне не нравится|i love|i hate|i don't like|i like)\s+([^.!?\n]{2,80})`)

// ExtractFact pulls a short preference fact from a user message (deterministic, no LLM).
func ExtractFact(msg string) string {
	m := factPattern.FindStringSubmatch(msg)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1] + " " + m[2])
}
