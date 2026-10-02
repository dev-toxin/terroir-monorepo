package service_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/llm"
	"github.com/anko-soft-lab/terroir/backend/internal/seed"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
	"github.com/anko-soft-lab/terroir/backend/internal/storage/sqlstore"
)

var today = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

type env struct {
	st    *sqlstore.Store
	svc   *service.Service
	anton service.Actor
	elena service.Actor
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	st, err := sqlstore.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := seed.Run(ctx, st, today); err != nil {
		t.Fatal(err)
	}
	if err := seed.Run(ctx, st, today); err != nil { // idempotent
		t.Fatal(err)
	}
	svc := service.New(st, domain.FixedClock{T: today}, sqlstore.NewID)
	anton, err := svc.Authenticate(ctx, seed.TokenAnton)
	if err != nil {
		t.Fatal(err)
	}
	elena, err := svc.Authenticate(ctx, seed.TokenElena)
	if err != nil {
		t.Fatal(err)
	}
	return &env{st: st, svc: svc, anton: anton, elena: elena}
}

func TestAuthenticate(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if e.anton.CellarID != seed.CellarID || e.anton.Role != domain.RoleOwner || e.elena.Role != domain.RoleMember {
		t.Fatalf("actors: %+v %+v", e.anton, e.elena)
	}
	for _, tok := range []string{"", "bad-token"} {
		if _, err := e.svc.Authenticate(ctx, tok); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("%q: %v", tok, err)
		}
	}
	// user without cellar → forbidden
	_ = e.st.CreateUser(ctx, domain.User{ID: "u-x", DisplayName: "X", Locale: domain.LocaleEN, CreatedAt: today})
	if err := e.svc.RegisterTokens(ctx, map[string]string{"token-for-x-123": "u-x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Authenticate(ctx, "token-for-x-123"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("no cellar: %v", err)
	}
	if err := e.svc.RegisterTokens(ctx, map[string]string{"token-unknown-1": "u-nobody"}); err == nil {
		t.Fatal("unknown user accepted")
	}
	if service.HashToken("a") == "a" || len(service.HashToken("a")) != 64 {
		t.Fatal("hash")
	}
}

func TestSharedCellarSeparateProfiles(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, _ := e.svc.ListItems(ctx, e.anton, service.ItemFilter{})
	b, _ := e.svc.ListItems(ctx, e.elena, service.ItemFilter{})
	if len(a) != len(b) || len(a) == 0 {
		t.Fatal("partners must share the cellar")
	}
	pa, _ := e.svc.Profile(ctx, e.anton)
	pb, _ := e.svc.Profile(ctx, e.elena)
	if pa.Tannin == pb.Tannin && pa.Body == pb.Body {
		t.Fatal("profiles must differ")
	}
	if _, err := e.svc.AnswerSurvey(ctx, e.elena, []service.AnswerInput{{QuestionID: "q_tannin", Values: []string{"1"}}}); err != nil {
		t.Fatal(err)
	}
	pa2, _ := e.svc.Profile(ctx, e.anton)
	pb2, _ := e.svc.Profile(ctx, e.elena)
	if pa2.Tannin != pa.Tannin || pb2.Tannin != 1 || pb2.Version != pb.Version+1 {
		t.Fatal("survey must only change own profile")
	}
}

func TestWineAndItemLifecycle(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w, err := e.svc.CreateWine(ctx, e.anton, domain.Wine{Name: "  Киси ", Color: domain.ColorWhite})
	if err != nil || w.Name != "Киси" || w.ID == "" {
		t.Fatalf("create: %v %+v", err, w)
	}
	if _, err := e.svc.CreateWine(ctx, e.anton, domain.Wine{Name: "X"}); !domain.IsValidation(err) {
		t.Fatal("missing color accepted")
	}
	var p service.WinePatch
	p.Region.Set, p.Region.Value = true, "Кахетия"
	p.Vintage.Set, p.Vintage.Value = true, 2021
	w2, err := e.svc.UpdateWine(ctx, e.anton, w.ID, p)
	if err != nil || w2.Region != "Кахетия" || *w2.Vintage != 2021 {
		t.Fatalf("patch: %v %+v", err, w2)
	}
	got, _ := e.svc.GetWine(ctx, e.elena, w.ID)
	if got.Region != "Кахетия" {
		t.Fatal("partner must see the wine")
	}
	if _, err := e.svc.UpdateWine(ctx, e.anton, "nope", p); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("patch missing")
	}

	from, until := 2027, 2030
	v, err := e.svc.AddItem(ctx, e.anton, domain.CellarItem{WineID: w.ID, Quantity: 2, DrinkFrom: &from, DrinkUntil: &until})
	if err != nil || v.Window != domain.WindowTooEarly || v.WindowLabel != "ещё рано" {
		t.Fatalf("add item: %v %+v", err, v)
	}
	if _, err := e.svc.AddItem(ctx, e.anton, domain.CellarItem{WineID: "nope", Quantity: 1}); !domain.IsValidation(err) {
		t.Fatalf("unknown wine: %v", err)
	}
	if _, err := e.svc.AddItem(ctx, e.anton, domain.CellarItem{WineID: w.ID, Quantity: -1}); !domain.IsValidation(err) {
		t.Fatal("negative quantity")
	}
	var ip service.ItemPatch
	ip.DrinkFrom.Set, ip.DrinkFrom.Null = true, true
	ip.Quantity.Set, ip.Quantity.Value = true, 3
	v2, err := e.svc.UpdateItem(ctx, e.anton, v.Item.ID, ip)
	if err != nil || v2.Item.DrinkFrom != nil || v2.Item.Quantity != 3 || v2.Window != domain.WindowReady {
		t.Fatalf("patch item: %v %+v", err, v2)
	}
	if _, err := e.svc.GetItem(ctx, e.anton, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("get missing item")
	}
	if err := e.svc.DeleteWine(ctx, e.anton, w.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("delete referenced wine")
	}
	ws, _ := e.svc.ListWines(ctx, e.anton)
	if len(ws) < 23 {
		t.Fatal("list wines")
	}
}

func TestReadyFilterAndSummary(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	ready, _ := e.svc.ListItems(ctx, e.anton, service.ItemFilter{ReadyToDrink: true})
	if len(ready) == 0 {
		t.Fatal("ready empty")
	}
	for i, v := range ready {
		if v.Window == domain.WindowTooEarly || v.Window == domain.WindowUnknown {
			t.Fatalf("not ready: %s", v.Window)
		}
		if i > 0 && v.Window.Urgency() > ready[i-1].Window.Urgency() {
			t.Fatal("ready not sorted by urgency")
		}
	}
	s, err := e.svc.Summary(ctx, e.anton, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.OpenSoon) != 3 || s.OpenSoon[0].Window != domain.WindowLate || s.TotalBottles < 30 || s.ByColor["red"] == 0 {
		t.Fatalf("summary: %+v", s)
	}
	if s.RecentDrinks != 3 { // seed: 3 drinks in last 30 days
		t.Fatalf("recent drinks %d", s.RecentDrinks)
	}
	rem, _ := e.svc.Reminders(ctx, e.anton)
	if len(rem) == 0 || rem[0].Item.Window != domain.WindowLate || !strings.Contains(rem[0].Message, "—") {
		t.Fatalf("reminders: %+v", rem)
	}
}

// Service-level race: concurrent "drank the last bottle" from both partners.
func TestDrinkLastBottleRace(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	const item = "i-cabernet-kuban" // seed has exactly 1
	var ok, out atomic.Int32
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			act := e.anton
			if i%2 == 1 {
				act = e.elena
			}
			_, err := e.svc.Drink(ctx, act, item, service.DrinkRequest{Rating: 4})
			switch {
			case err == nil:
				ok.Add(1)
			case errors.Is(err, domain.ErrOutOfStock):
				out.Add(1)
			default:
				t.Errorf("unexpected %v", err)
			}
		}()
	}
	wg.Wait()
	v, _ := e.svc.GetItem(ctx, e.anton, item)
	if ok.Load() != 1 || out.Load() != 9 || v.Item.Quantity != 0 {
		t.Fatalf("ok=%d out=%d qty=%d", ok.Load(), out.Load(), v.Item.Quantity)
	}
}

func TestDrinkWithRecipeTeachesPairing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	before, _ := e.svc.WinesForRecipe(ctx, e.anton, "r-salmon", 20)
	rid := "r-salmon"
	res, err := e.svc.Drink(ctx, e.anton, "i-syrah-kuban", service.DrinkRequest{Rating: 5, PairedRecipeID: &rid, Note: "неожиданно хорошо"})
	if err != nil || res.Item.Quantity != 1 || res.Event.WineID != "w-syrah-kuban" {
		t.Fatalf("drink: %v %+v", err, res)
	}
	after, _ := e.svc.WinesForRecipe(ctx, e.anton, "r-salmon", 20)
	score := func(xs []service.WineSuggestion) int {
		for _, x := range xs {
			if x.Item.Item.ID == "i-syrah-kuban" {
				return x.Score
			}
		}
		return -1
	}
	if score(after) <= score(before) {
		t.Fatalf("feedback must raise the pairing: %d → %d", score(before), score(after))
	}
	empty := ""
	if _, err := e.svc.Drink(ctx, e.anton, "i-syrah-kuban", service.DrinkRequest{PairedRecipeID: &empty}); err != nil {
		t.Fatalf("empty recipe id must be ignored: %v", err)
	}
	bad := "nope"
	if _, err := e.svc.Drink(ctx, e.anton, "i-tsimla", service.DrinkRequest{PairedRecipeID: &bad}); !domain.IsValidation(err) {
		t.Fatal("unknown recipe")
	}
	if _, err := e.svc.Drink(ctx, e.anton, "i-tsimla", service.DrinkRequest{Rating: 9}); !domain.IsValidation(err) {
		t.Fatal("bad rating")
	}
	h, _ := e.svc.History(ctx, e.anton, 100)
	if len(h) < 6 {
		t.Fatalf("history %d", len(h))
	}
}

func TestSurveyBatches(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	b, err := e.svc.NextSurveyBatch(ctx, e.anton, 99) // out of range → default 6
	if err != nil || len(b.Questions) != 6 || b.Done || b.Total != len(domain.SurveyBank) {
		t.Fatalf("batch: %v %+v", err, b)
	}
	for !b.Done {
		var ans []service.AnswerInput
		for _, q := range b.Questions {
			v := "3"
			if len(q.Options) > 0 {
				v = q.Options[0].Value
			}
			ans = append(ans, service.AnswerInput{QuestionID: q.ID, Values: []string{v}})
		}
		if _, err := e.svc.AnswerSurvey(ctx, e.anton, ans); err != nil {
			t.Fatal(err)
		}
		b, _ = e.svc.NextSurveyBatch(ctx, e.anton, 5)
	}
	if b.Answered != b.Total {
		t.Fatal("not all answered")
	}
	en := e.anton
	en.User.Locale = domain.LocaleEN
	e2 := newEnv(t)
	eb, _ := e2.svc.NextSurveyBatch(ctx, en, 7)
	if len(eb.Questions) != 7 || !strings.Contains(eb.Questions[0].Text, "wine") {
		t.Fatal("EN batch")
	}
	if _, err := e.svc.AnswerSurvey(ctx, e.anton, nil); !domain.IsValidation(err) {
		t.Fatal("empty answers")
	}
	// all-or-nothing: an invalid answer in a batch stores nothing
	p0, _ := e2.svc.Profile(ctx, e2.anton)
	_, err = e2.svc.AnswerSurvey(ctx, e2.anton, []service.AnswerInput{{QuestionID: "q_body", Values: []string{"1"}}, {QuestionID: "q_nope", Values: []string{"1"}}})
	p1, _ := e2.svc.Profile(ctx, e2.anton)
	if !domain.IsValidation(err) || p1.Body != p0.Body {
		t.Fatal("partial batch applied")
	}
}

func TestRecipesAndSuggestions(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	r, err := e.svc.CreateRecipe(ctx, e.anton, domain.Recipe{Title: "Хинкали", MainIngredient: domain.IngBeef, CookingMethod: domain.MethodBoiled, Spiciness: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.CreateRecipe(ctx, e.anton, domain.Recipe{Title: ""}); !domain.IsValidation(err) {
		t.Fatal("empty title")
	}
	var p service.RecipePatch
	p.Richness.Set, p.Richness.Value = true, 4
	r2, err := e.svc.UpdateRecipe(ctx, e.elena, r.ID, p)
	if err != nil || r2.Richness != 4 {
		t.Fatal("patch recipe")
	}
	p.Spiciness.Set, p.Spiciness.Value = true, 5
	if _, err := e.svc.UpdateRecipe(ctx, e.anton, r.ID, p); !domain.IsValidation(err) {
		t.Fatal("bad spiciness")
	}
	if _, err := e.svc.UpdateRecipe(ctx, e.anton, "nope", p); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("missing recipe patch")
	}
	got, _ := e.svc.GetRecipe(ctx, e.anton, r.ID)
	if got.Title != "Хинкали" {
		t.Fatal("get recipe")
	}
	ws, err := e.svc.WinesForRecipe(ctx, e.anton, r.ID, 0)
	if err != nil || len(ws) != 5 || ws[0].Score < ws[4].Score || len(ws[0].Reasons) == 0 {
		t.Fatalf("wines for recipe: %v %d", err, len(ws))
	}
	for _, w := range ws {
		if w.Item.Item.Quantity <= 0 {
			t.Fatal("empty bottle suggested")
		}
	}
	rs, err := e.svc.RecipesForItem(ctx, e.anton, "i-riesling-off", 0)
	if err != nil || len(rs) != 5 {
		t.Fatalf("recipes for item: %v", err)
	}
	if _, err := e.svc.RecipesForItem(ctx, e.anton, "nope", 3); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("missing item")
	}
	if _, err := e.svc.WinesForRecipe(ctx, e.anton, "nope", 3); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("missing recipe")
	}
	if _, err := e.svc.AddPairingFeedback(ctx, e.anton, r.ID, service.PairingFeedbackInput{WineID: "w-saperavi-kakheti", Rating: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddPairingFeedback(ctx, e.anton, r.ID, service.PairingFeedbackInput{WineID: "nope", Rating: 5}); !domain.IsValidation(err) {
		t.Fatal("unknown wine feedback")
	}
	if _, err := e.svc.AddPairingFeedback(ctx, e.anton, "nope", service.PairingFeedbackInput{WineID: "w-saperavi-kakheti", Rating: 5}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("unknown recipe feedback")
	}
	if err := e.svc.DeleteRecipe(ctx, e.anton, r.ID); err != nil {
		t.Fatal(err)
	}
	all, _ := e.svc.ListRecipes(ctx, e.anton)
	if len(all) != 11 {
		t.Fatalf("recipes %d", len(all))
	}
}

func TestBuyAgain(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	xs, err := e.svc.BuyAgain(ctx, e.anton)
	if err != nil || len(xs) != 2 {
		t.Fatalf("buy again: %v %+v", err, xs)
	}
	if xs[0].Wine.ID != "w-saperavi-empty" || xs[0].InStock != 0 || xs[0].AvgRating != 5 {
		t.Fatalf("order: %+v", xs[0])
	}
	// Elena rated Rkatsiteli 3 → not in her list; Saperavi reserve 5 → in.
	ys, _ := e.svc.BuyAgain(ctx, e.elena)
	if len(ys) != 1 || ys[0].Wine.ID != "w-saperavi-empty" {
		t.Fatalf("elena: %+v", ys)
	}
}

func TestCSVRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	var buf bytes.Buffer
	if err := e.svc.ExportCSV(ctx, e.anton, &buf); err != nil {
		t.Fatal(err)
	}
	exported := buf.String()
	if !strings.HasPrefix(exported, strings.Join(service.CSVHeader, ",")) || !strings.Contains(exported, "Саперави|") == strings.Contains(exported, "never") {
		t.Fatal("header")
	}
	fresh := newEnvEmpty(t)
	res, err := fresh.svc.ImportCSV(ctx, fresh.anton, strings.NewReader(exported))
	if err != nil || res.Imported != 22 || len(res.Errors) != 0 {
		t.Fatalf("import: %v %+v", err, res)
	}
	items, _ := fresh.svc.ListItems(ctx, fresh.anton, service.ItemFilter{CellarFilter: storage.CellarFilter{IncludeEmpty: true}})
	if len(items) != 22 {
		t.Fatalf("imported items %d", len(items))
	}
	bad := "name,color,abv,quantity,purchase_price_minor,drink_from,vintage\n" +
		"A,red,abc,1,,,\nB,red,,x,,,\nC,red,,1,y,,\nD,red,,1,,z,\nE,red,,1,,,1700\n\"unterminated,red\n"
	res, err = fresh.svc.ImportCSV(ctx, fresh.anton, strings.NewReader(bad))
	if err != nil || res.Imported != 0 || len(res.Errors) < 5 {
		t.Fatalf("bad rows: %v %+v", err, res)
	}
	if _, err := fresh.svc.ImportCSV(ctx, fresh.anton, strings.NewReader("color\nred\n")); !domain.IsValidation(err) {
		t.Fatal("missing name col")
	}
	if _, err := fresh.svc.ImportCSV(ctx, fresh.anton, strings.NewReader("name\nX\n")); !domain.IsValidation(err) {
		t.Fatal("missing color col")
	}
	if _, err := fresh.svc.ImportCSV(ctx, fresh.anton, strings.NewReader("")); !domain.IsValidation(err) {
		t.Fatal("empty file")
	}
}

// newEnvEmpty is a household with no wines (for import tests).
func newEnvEmpty(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	st, err := sqlstore.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	_ = st.CreateUser(ctx, domain.User{ID: "u1", DisplayName: "U", Locale: domain.LocaleRU, CreatedAt: today})
	_ = st.CreateCellar(ctx, domain.Cellar{ID: "c1", Name: "c", CreatedAt: today})
	_ = st.AddMember(ctx, domain.Membership{CellarID: "c1", UserID: "u1", Role: domain.RoleOwner})
	svc := service.New(st, domain.FixedClock{T: today}, sqlstore.NewID)
	_ = svc.RegisterTokens(ctx, map[string]string{"token-u1-xxxx": "u1"})
	a, err := svc.Authenticate(ctx, "token-u1-xxxx")
	if err != nil {
		t.Fatal(err)
	}
	return &env{st: st, svc: svc, anton: a}
}

// ---------- assistant ----------

type fakeProvider struct {
	answer string
	err    error
	calls  int
	last   llm.Request
}

func (f *fakeProvider) Name() string  { return "fake" }
func (f *fakeProvider) Model() string { return "fake-1" }
func (f *fakeProvider) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	f.calls++
	f.last = r
	return llm.Response{Content: f.answer, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 10}, Model: "fake-1"}, f.err
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestAssistantMockAnswersFromCellar(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := service.NewAssistant(e.svc, llm.Mock{}, &llm.Budget{Store: e.st}, 0, quiet())
	res, err := a.Chat(ctx, e.anton, service.ChatRequest{Message: "Что открыть к токпокки?"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Intent != service.IntentPairRecipe || res.Recipe != "Токпокки с кимчи" || len(res.Bottles) == 0 || res.Fallback {
		t.Fatalf("chat: %+v", res)
	}
	tok, _, _ := e.st.UsageSince(ctx, today.AddDate(0, -1, 0))
	if tok == 0 {
		t.Fatal("usage not recorded")
	}
	msgs, _ := e.st.ListMessages(ctx, res.ConversationID, 10)
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("history: %+v", msgs)
	}
}

func TestAssistantRejectsHallucinatedBottles(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for name, answer := range map[string]string{
		"unknown id":      "Откройте [item:i-does-not-exist] — отличное вино.",
		"empty bottle":    "Откройте [item:i-saperavi-empty].",
		"no reference":    "Возьмите любое красное.",
		"provider failed": "",
	} {
		t.Run(name, func(t *testing.T) {
			fp := &fakeProvider{answer: answer}
			if name == "provider failed" {
				fp.err = errors.New("boom")
			}
			a := service.NewAssistant(e.svc, fp, nil, 2000, quiet())
			res, err := a.Chat(ctx, e.anton, service.ChatRequest{Message: "Что открыть к стейку?"})
			if err != nil {
				t.Fatal(err)
			}
			if !res.Fallback || len(res.Bottles) == 0 || strings.Contains(res.Answer, "does-not-exist") {
				t.Fatalf("expected deterministic fallback: %+v", res)
			}
			for _, b := range res.Bottles {
				if b.Quantity <= 0 {
					t.Fatal("fallback referenced empty bottle")
				}
			}
		})
	}
	// a valid model answer is kept as is
	fp := &fakeProvider{answer: "Берите [item:i-krasnostop-don]."}
	a := service.NewAssistant(e.svc, fp, nil, 2000, quiet())
	res, _ := a.Chat(ctx, e.anton, service.ChatRequest{Message: "Что открыть к стейку?"})
	if res.Fallback || res.Answer != fp.answer || len(res.Bottles) != 1 {
		t.Fatalf("valid answer replaced: %+v", res)
	}
	// prompt contains system prompt + context; no empty bottles in the context
	sys := fp.last.Messages[0].Content
	if !strings.Contains(sys, "Terroir") || !strings.Contains(sys, "[item:i-krasnostop-don]") || strings.Contains(sys, "i-saperavi-empty") {
		t.Fatalf("context: %s", sys)
	}
}

func TestAssistantBudgetAndValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := service.NewAssistant(e.svc, llm.Mock{}, &llm.Budget{MonthlyTokens: 5, Store: e.st, Now: func() time.Time { return today }}, 0, quiet())
	if _, err := a.Chat(ctx, e.anton, service.ChatRequest{Message: "Что пора выпить?"}); !errors.Is(err, domain.ErrBudget) {
		t.Fatalf("budget: %v", err)
	}
	if _, err := a.Chat(ctx, e.anton, service.ChatRequest{Message: "   "}); !domain.IsValidation(err) {
		t.Fatal("empty message")
	}
}

func TestAssistantMemoryAndLocale(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := service.NewAssistant(e.svc, llm.Mock{}, nil, 0, quiet())
	if a.Provider().Name() != "mock" {
		t.Fatal("provider")
	}
	res, err := a.Chat(ctx, e.anton, service.ChatRequest{Message: "Я не люблю Пино Нуар. Что пора выпить?"})
	if err != nil || res.Intent != service.IntentDrinkNow {
		t.Fatalf("%v %+v", err, res)
	}
	facts, _ := e.st.ListFacts(ctx, e.anton.User.ID, 10)
	if !strings.Contains(facts[0].Fact, "Пино Нуар") {
		t.Fatalf("fact: %+v", facts[0])
	}
	en, err := a.Chat(ctx, e.anton, service.ChatRequest{Message: "What should I open tonight with a steak?"})
	if err != nil || !strings.Contains(en.Answer, "I would open") {
		t.Fatalf("EN: %v %q", err, en.Answer)
	}
	gen, _ := a.Chat(ctx, e.anton, service.ChatRequest{Message: "Привет!"})
	if gen.Intent != service.IntentGeneral || len(gen.Bottles) == 0 {
		t.Fatalf("general: %+v", gen)
	}
}

func TestDetectIntentAndFacts(t *testing.T) {
	recipes := []domain.Recipe{{ID: "r1", Title: "Паста болоньезе", MainIngredient: domain.IngPasta}, {ID: "r2", Title: "Свиная шея на углях", MainIngredient: domain.IngPork}}
	cases := []struct {
		msg, intent, recipe string
	}{
		{"Что купить снова?", service.IntentBuyAgain, ""},
		{"Что открыть к болоньезе?", service.IntentPairRecipe, "r1"},
		{"к шее на углях что?", service.IntentPairRecipe, "r2"},
		{"Что открыть к острым креветкам?", service.IntentPairRecipe, ""},
		{"Что пора выпить?", service.IntentDrinkNow, ""},
		{"Привет", service.IntentGeneral, ""},
	}
	for _, c := range cases {
		intent, r := service.DetectIntent(c.msg, recipes)
		if intent != c.intent || r.ID != c.recipe {
			t.Errorf("%q → %s %q", c.msg, intent, r.ID)
		}
	}
	_, shrimp := service.DetectIntent("Что открыть к острым креветкам?", recipes)
	if shrimp.MainIngredient != domain.IngSeafood || shrimp.Spiciness != 3 {
		t.Fatalf("adhoc: %+v", shrimp)
	}
	if service.ExtractFact("я обожаю Саперави!") != "обожаю Саперави" || service.ExtractFact("Что выпить?") != "" {
		t.Fatal("facts")
	}
}

func TestBuildContextTrims(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	entries, _ := e.st.ListItems(ctx, seed.CellarID, storage.CellarFilter{})
	p, _ := e.st.GetProfile(ctx, e.anton.User.ID)
	full := service.BuildContext(domain.LocaleRU, p, entries, nil, nil, &llm.Facts{}, 2026, 100000)
	short := service.BuildContext(domain.LocaleRU, p, entries, nil, nil, &llm.Facts{}, 2026, 100)
	if llm.EstimateTokens(short) > 100 || len(short) >= len(full) {
		t.Fatalf("trim: %d vs %d", len(short), len(full))
	}
	if !strings.Contains(full, "Погреб") {
		t.Fatal("ru headers")
	}
}
