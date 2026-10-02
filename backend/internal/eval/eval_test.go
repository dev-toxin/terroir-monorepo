package eval_test

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/eval"
	"github.com/anko-soft-lab/terroir/backend/internal/llm"
	"github.com/anko-soft-lab/terroir/backend/internal/seed"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage/sqlstore"
)

// TestBlindQuestionsOnMock runs the reference set through the mock assistant over the demo
// household and checks every declared property. This is the baseline the future blind
// comparison with a general chatbot builds on.
func TestBlindQuestionsOnMock(t *testing.T) {
	qs, err := eval.Questions()
	if err != nil || len(qs) != 10 {
		t.Fatalf("questions: %v (%d)", err, len(qs))
	}
	ctx := context.Background()
	today := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	st, err := sqlstore.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := seed.Run(ctx, st, today); err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, domain.FixedClock{T: today}, sqlstore.NewID)
	ast := service.NewAssistant(svc, llm.Mock{}, nil, 0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	tokens := map[string]string{"anton": seed.TokenAnton, "elena": seed.TokenElena}

	for _, q := range qs {
		t.Run(q.ID, func(t *testing.T) {
			act, err := svc.Authenticate(ctx, tokens[q.User])
			if err != nil {
				t.Fatal(err)
			}
			res, err := ast.Chat(ctx, act, service.ChatRequest{Message: q.Question})
			if err != nil {
				t.Fatal(err)
			}
			if res.Intent != q.Intent {
				t.Fatalf("intent %s, want %s", res.Intent, q.Intent)
			}
			ex := q.Expect
			if len(res.Bottles) < ex.MinBottles {
				t.Fatalf("bottles %d < %d: %s", len(res.Bottles), ex.MinBottles, res.Answer)
			}
			for _, b := range res.Bottles {
				v, err := svc.GetItem(ctx, act, b.ItemID)
				if err != nil {
					t.Fatalf("referenced bottle %s does not exist", b.ItemID)
				}
				if ex.InStockOnly && v.Item.Quantity <= 0 {
					t.Fatalf("referenced bottle %s is not in stock", b.ItemID)
				}
			}
			if len(res.Bottles) > 0 {
				top, _ := svc.GetItem(ctx, act, res.Bottles[0].ItemID)
				st := top.Wine.ResolveStructure()
				if len(ex.TopColors) > 0 && !slices.Contains(ex.TopColors, string(top.Wine.Color)) {
					t.Errorf("top colour %s not in %v (%s)", top.Wine.Color, ex.TopColors, top.Wine.Name)
				}
				if ex.TopMaxTannin != nil && st.Tannin > *ex.TopMaxTannin {
					t.Errorf("top tannin %d > %d (%s)", st.Tannin, *ex.TopMaxTannin, top.Wine.Name)
				}
				if ex.TopMinTannin != nil && st.Tannin < *ex.TopMinTannin {
					t.Errorf("top tannin %d < %d (%s)", st.Tannin, *ex.TopMinTannin, top.Wine.Name)
				}
				if ex.TopMinBody != nil && st.Body < *ex.TopMinBody {
					t.Errorf("top body %d < %d (%s)", st.Body, *ex.TopMinBody, top.Wine.Name)
				}
				if len(ex.TopWindows) > 0 && !slices.Contains(ex.TopWindows, string(top.Window)) {
					t.Errorf("top window %s not in %v", top.Window, ex.TopWindows)
				}
			}
			low := strings.ToLower(res.Answer)
			if len(ex.AnswerContainsAny) > 0 && !slices.ContainsFunc(ex.AnswerContainsAny, func(s string) bool { return strings.Contains(low, strings.ToLower(s)) }) {
				t.Errorf("answer has none of %v: %s", ex.AnswerContainsAny, res.Answer)
			}
			for _, s := range ex.AnswerContainsAll {
				if !strings.Contains(res.Answer, s) {
					t.Errorf("answer misses %q: %s", s, res.Answer)
				}
			}
			for _, s := range ex.AnswerNotContains {
				if strings.Contains(res.Answer, s) {
					t.Errorf("answer must not contain %q: %s", s, res.Answer)
				}
			}
			t.Logf("%s → %s", q.Question, res.Answer)
		})
	}
}
