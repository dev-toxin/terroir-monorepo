package sqlstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

// stores returns the implementations under test: SQLite in-memory, SQLite file, and PostgreSQL
// when TERROIR_PG_TEST_URL is set (skipped otherwise).
func stores(t *testing.T) map[string]*Store {
	t.Helper()
	ctx := context.Background()
	out := map[string]*Store{}
	mem, err := OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mem.Close() })
	out["memory"] = mem
	file, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	out["sqlite"] = file
	if url := os.Getenv("TERROIR_PG_TEST_URL"); url != "" {
		pg, err := OpenPostgres(ctx, url)
		if err != nil {
			t.Fatalf("postgres: %v", err)
		}
		truncateAll(t, pg)
		t.Cleanup(func() { truncateAll(t, pg); _ = pg.Close() })
		out["postgres"] = pg
	}
	return out
}

func truncateAll(t *testing.T, s *Store) {
	t.Helper()
	_, err := s.db.Exec(`TRUNCATE users, user_tokens, cellars, cellar_members, wines, cellar_items, consumption_events,
taste_profiles, survey_answers, recipes, pairing_feedback, chat_messages, memory_facts, llm_usage CASCADE`)
	if err != nil {
		t.Fatal(err)
	}
}

var t0 = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

func fixture(t *testing.T, s *Store, qty int) (cellarID, userID, itemID string) {
	t.Helper()
	ctx := context.Background()
	u := domain.User{ID: NewID(), DisplayName: "A", Locale: domain.LocaleRU, CreatedAt: t0}
	must(t, s.CreateUser(ctx, u))
	c := domain.Cellar{ID: NewID(), Name: "home", CreatedAt: t0}
	must(t, s.CreateCellar(ctx, c))
	must(t, s.AddMember(ctx, domain.Membership{CellarID: c.ID, UserID: u.ID, Role: domain.RoleOwner}))
	v := 2019
	abv := 13.5
	w := domain.Wine{ID: NewID(), CellarID: c.ID, Name: "Саперави", Grapes: []string{"Саперави"}, Vintage: &v, ABV: &abv,
		Color: domain.ColorRed, Source: domain.SourceManual, Country: "Грузия", CreatedAt: t0, UpdatedAt: t0}
	must(t, s.CreateWine(ctx, w))
	from, until := 2022, 2028
	price := int64(150000)
	it := domain.CellarItem{ID: NewID(), CellarID: c.ID, WineID: w.ID, Quantity: qty, StorageLocation: "Полка 1",
		PurchasePriceMinor: &price, Currency: "RUB", PurchasedAt: &t0, DrinkFrom: &from, DrinkUntil: &until, CreatedAt: t0, UpdatedAt: t0}
	must(t, s.CreateItem(ctx, it))
	return c.ID, u.ID, it.ID
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cid, uid, iid := fixture(t, s, 3)

			e, err := s.GetItem(ctx, cid, iid)
			must(t, err)
			if e.Wine.Name != "Саперави" || *e.Wine.Vintage != 2019 || *e.Item.DrinkUntil != 2028 || *e.Item.PurchasePriceMinor != 150000 {
				t.Fatalf("roundtrip: %+v", e)
			}
			if !e.Item.PurchasedAt.Equal(t0) || !e.Item.CreatedAt.Equal(t0) {
				t.Fatalf("time roundtrip: %v %v", e.Item.PurchasedAt, e.Item.CreatedAt)
			}
			if _, err := s.GetItem(ctx, "other-cellar", iid); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("cross-cellar read must be not found, got %v", err)
			}

			// filters
			for _, f := range []storage.CellarFilter{{Color: domain.ColorRed}, {Country: "грузия"}, {Grape: "saperavi"}, {Query: "сапер"}, {Location: "полка"}} {
				xs, err := s.ListItems(ctx, cid, f)
				must(t, err)
				if len(xs) != 1 {
					t.Fatalf("filter %+v: %d", f, len(xs))
				}
			}
			xs, _ := s.ListItems(ctx, cid, storage.CellarFilter{Color: domain.ColorWhite})
			if len(xs) != 0 {
				t.Fatal("white filter")
			}

			// update wine & item
			e.Wine.Name, e.Wine.Vintage, e.Wine.UpdatedAt = "Саперави резерв", nil, t0.Add(time.Hour)
			must(t, s.UpdateWine(ctx, e.Wine))
			w, _ := s.GetWine(ctx, cid, e.Wine.ID)
			if w.Name != "Саперави резерв" || w.Vintage != nil {
				t.Fatalf("update wine: %+v", w)
			}
			e.Item.Notes, e.Item.DrinkFrom = "подарок", nil
			must(t, s.UpdateItem(ctx, e.Item))
			e2, _ := s.GetItem(ctx, cid, iid)
			if e2.Item.Notes != "подарок" || e2.Item.DrinkFrom != nil {
				t.Fatalf("update item: %+v", e2.Item)
			}
			if err := s.DeleteWine(ctx, cid, w.ID); !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("delete referenced wine: %v", err)
			}
			if err := s.UpdateWine(ctx, domain.Wine{ID: "nope", CellarID: cid}); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("update missing: %v", err)
			}

			// users/tokens
			must(t, s.AddToken(ctx, uid, "hash1"))
			must(t, s.AddToken(ctx, uid, "hash1")) // idempotent
			u, err := s.UserByToken(ctx, "hash1")
			if err != nil || u.ID != uid {
				t.Fatalf("token: %v", err)
			}
			if _, err := s.UserByToken(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal("unknown token")
			}
			ms, _ := s.MembershipsOf(ctx, uid)
			members, _ := s.MembersOf(ctx, cid)
			if len(ms) != 1 || len(members) != 1 {
				t.Fatal("membership")
			}
		})
	}
}

func TestDrinkKeepsRowAndHistory(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cid, uid, iid := fixture(t, s, 1)
			r := domain.Recipe{ID: NewID(), CellarID: cid, Title: "Стейк", MainIngredient: domain.IngBeef, CookingMethod: domain.MethodGrilled,
				Sauce: domain.SauceNone, Richness: 5, Acidity: 1, Tags: []string{"мясо"}, CreatedAt: t0, UpdatedAt: t0}
			must(t, s.CreateRecipe(ctx, r))
			ev := domain.ConsumptionEvent{ID: NewID(), CellarItemID: iid, UserID: uid, Bottles: 1, DrunkAt: t0, Rating: 5, Note: "отлично", PairedRecipeID: &r.ID}
			it, err := s.Drink(ctx, cid, ev)
			must(t, err)
			if it.Quantity != 0 {
				t.Fatalf("quantity %d", it.Quantity)
			}
			if _, err := s.Drink(ctx, cid, domain.ConsumptionEvent{ID: NewID(), CellarItemID: iid, UserID: uid, Bottles: 1, DrunkAt: t0}); !errors.Is(err, domain.ErrOutOfStock) {
				t.Fatalf("second drink: %v", err)
			}
			if _, err := s.Drink(ctx, cid, domain.ConsumptionEvent{ID: NewID(), CellarItemID: "missing", UserID: uid, Bottles: 1, DrunkAt: t0}); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("missing item: %v", err)
			}
			// row stays (history), hidden from default listing, visible with IncludeEmpty
			if _, err := s.GetItem(ctx, cid, iid); err != nil {
				t.Fatal("row with zero must stay")
			}
			def, _ := s.ListItems(ctx, cid, storage.CellarFilter{})
			all, _ := s.ListItems(ctx, cid, storage.CellarFilter{IncludeEmpty: true})
			if len(def) != 0 || len(all) != 1 {
				t.Fatalf("listing: %d %d", len(def), len(all))
			}
			evs, _ := s.ListConsumption(ctx, cid, uid, 10)
			if len(evs) != 1 || evs[0].Rating != 5 || evs[0].PairedRecipeID == nil || evs[0].WineID == "" {
				t.Fatalf("history: %+v", evs)
			}
			fb, _ := s.ListPairingFeedback(ctx, cid)
			if len(fb) != 1 || fb[0].Rating != 5 {
				t.Fatalf("drink with recipe must record pairing feedback: %+v", fb)
			}
		})
	}
}

// Two (and more) concurrent "drank the last bottle" requests: exactly one wins, quantity never < 0.
func TestConcurrentDrinkLastBottle(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cid, uid, iid := fixture(t, s, 1)
			const n = 8
			var wg sync.WaitGroup
			var ok, out atomic.Int32
			start := make(chan struct{})
			for range n {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, err := s.Drink(ctx, cid, domain.ConsumptionEvent{ID: NewID(), CellarItemID: iid, UserID: uid, Bottles: 1, DrunkAt: t0})
					switch {
					case err == nil:
						ok.Add(1)
					case errors.Is(err, domain.ErrOutOfStock):
						out.Add(1)
					default:
						t.Errorf("unexpected: %v", err)
					}
				}()
			}
			close(start)
			wg.Wait()
			if ok.Load() != 1 || out.Load() != n-1 {
				t.Fatalf("ok=%d out=%d", ok.Load(), out.Load())
			}
			e, _ := s.GetItem(ctx, cid, iid)
			if e.Item.Quantity != 0 {
				t.Fatalf("quantity %d", e.Item.Quantity)
			}
			evs, _ := s.ListConsumption(ctx, cid, "", 100)
			if len(evs) != 1 {
				t.Fatalf("events %d", len(evs))
			}
		})
	}
}

func TestTasteRecipesAssistant(t *testing.T) {
	for name, s := range stores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cid, uid, _ := fixture(t, s, 2)
			p, err := s.GetProfile(ctx, uid)
			if err != nil || p.Version != 0 {
				t.Fatal("empty profile")
			}
			p.Acidity, p.Version, p.LikedGrapes, p.UpdatedAt = 2, 1, []string{"saperavi"}, t0
			must(t, s.SaveProfileWithAnswers(ctx, p, []domain.SurveyAnswer{{QuestionID: "q_acidity", Values: []string{"2"}, AnsweredAt: t0}}))
			p.Version = 2
			must(t, s.SaveProfileWithAnswers(ctx, p, []domain.SurveyAnswer{{QuestionID: "q_acidity", Values: []string{"3"}, AnsweredAt: t0}}))
			got, _ := s.GetProfile(ctx, uid)
			ans, _ := s.AnsweredQuestions(ctx, uid)
			if got.Acidity != 2 || got.Version != 2 || len(ans) != 1 || !ans["q_acidity"] {
				t.Fatalf("profile: %+v %v", got, ans)
			}

			r := domain.Recipe{ID: NewID(), CellarID: cid, Title: "Паста", MainIngredient: domain.IngPasta, CookingMethod: domain.MethodBoiled,
				Sauce: domain.SauceTomato, Richness: 3, Acidity: 4, CreatedAt: t0, UpdatedAt: t0}
			must(t, s.CreateRecipe(ctx, r))
			r.Title, r.Tags = "Паста болоньезе", []string{"паста"}
			must(t, s.UpdateRecipe(ctx, r))
			rs, _ := s.ListRecipes(ctx, cid)
			if len(rs) != 1 || rs[0].Title != "Паста болоньезе" || len(rs[0].Tags) != 1 {
				t.Fatalf("recipes: %+v", rs)
			}
			wines, _ := s.ListWines(ctx, cid)
			must(t, s.AddPairingFeedback(ctx, domain.PairingFeedback{ID: NewID(), CellarID: cid, RecipeID: r.ID, WineID: wines[0].ID, UserID: uid, Rating: 4, CreatedAt: t0}))
			if err := s.AddPairingFeedback(ctx, domain.PairingFeedback{ID: NewID(), CellarID: cid, RecipeID: "nope", WineID: wines[0].ID, UserID: uid, Rating: 4, CreatedAt: t0}); !domain.IsValidation(err) {
				t.Fatalf("fk: %v", err)
			}
			must(t, s.DeleteRecipe(ctx, cid, r.ID))
			if err := s.DeleteRecipe(ctx, cid, r.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal("double delete")
			}
			if _, err := s.GetRecipe(ctx, cid, r.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal("deleted recipe found")
			}

			for i := range 3 {
				must(t, s.AddMessage(ctx, domain.ChatMessage{ID: NewID(), ConversationID: "c1", UserID: uid, Role: "user", Content: string(rune('a' + i)), CreatedAt: t0.Add(time.Duration(i) * time.Second)}))
			}
			msgs, _ := s.ListMessages(ctx, "c1", 2)
			if len(msgs) != 2 || msgs[0].Content != "b" || msgs[1].Content != "c" {
				t.Fatalf("messages: %+v", msgs)
			}
			must(t, s.AddFact(ctx, domain.MemoryFact{ID: NewID(), UserID: uid, Fact: "не любит кислое", Source: "chat", CreatedAt: t0}))
			facts, _ := s.ListFacts(ctx, uid, 5)
			if len(facts) != 1 {
				t.Fatal("facts")
			}
			must(t, s.AddUsage(ctx, domain.LLMUsage{ID: NewID(), UserID: uid, Provider: "mock", Model: "m", PromptTokens: 100, CompletionTokens: 20, CostMicros: 7, CreatedAt: t0}))
			must(t, s.AddUsage(ctx, domain.LLMUsage{ID: NewID(), UserID: uid, Provider: "mock", Model: "m", PromptTokens: 1, CompletionTokens: 1, CostMicros: 1, CreatedAt: t0.AddDate(0, -1, 0)}))
			tok, cost, err := s.UsageSince(ctx, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
			if err != nil || tok != 120 || cost != 7 {
				t.Fatalf("usage: %d %d %v", tok, cost, err)
			}
			if err := s.Ping(ctx); err != nil || s.Kind() == "" {
				t.Fatal("ping/kind")
			}
		})
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.db")
	ctx := context.Background()
	s1, err := OpenSQLite(ctx, path)
	must(t, err)
	must(t, s1.Close())
	s2, err := OpenSQLite(ctx, path)
	must(t, err)
	defer s2.Close()
	var n int
	must(t, s2.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n))
	if n != 1 {
		t.Fatalf("migrations applied %d times", n)
	}
}

func TestRebind(t *testing.T) {
	s := &Store{dialect: Postgres}
	if got := s.q("a = ? AND b = ?"); got != "a = $1 AND b = $2" {
		t.Fatal(got)
	}
	if got := (&Store{dialect: SQLite}).q("a = ?"); got != "a = ?" {
		t.Fatal(got)
	}
}
