package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeUsage struct {
	tokens, micros int64
	since          time.Time
	err            error
}

func (f *fakeUsage) UsageSince(_ context.Context, t time.Time) (int64, int64, error) {
	f.since = t
	return f.tokens, f.micros, f.err
}

func fixedNow() time.Time { return time.Date(2026, 10, 17, 15, 30, 0, 0, time.UTC) }

func TestBudget_Unlimited(t *testing.T) {
	var nilB *Budget
	if err := nilB.Check(context.Background(), 1e9); err != nil {
		t.Fatal(err)
	}
	b := &Budget{Store: &fakeUsage{tokens: 1e12}}
	if err := b.Check(context.Background(), 1e9); err != nil {
		t.Fatal(err)
	}
}

func TestBudget_TokenLimit(t *testing.T) {
	st := &fakeUsage{tokens: 900}
	b := &Budget{MonthlyTokens: 1000, Store: st, Now: fixedNow}
	if err := b.Check(context.Background(), 100); err != nil {
		t.Fatalf("exactly at limit must pass: %v", err)
	}
	if err := b.Check(context.Background(), 101); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("want ErrBudgetExceeded, got %v", err)
	}
	if want := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); !st.since.Equal(want) {
		t.Fatalf("month start %v, want %v", st.since, want)
	}
}

func TestBudget_MoneyLimit(t *testing.T) {
	// 1000 micros per 1K input tokens; spent 9000 micros; limit 10000 micros.
	st := &fakeUsage{micros: 9000}
	b := &Budget{MonthlyMicros: 10000, PricePer1KIn: 1000, Store: st, Now: fixedNow}
	if err := b.Check(context.Background(), 1000); err != nil {
		t.Fatalf("at limit must pass: %v", err)
	}
	if err := b.Check(context.Background(), 2000); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("want ErrBudgetExceeded, got %v", err)
	}
	st.micros = 10001
	if err := b.Check(context.Background(), 0); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("already over: %v", err)
	}
}

func TestBudget_StoreError(t *testing.T) {
	boom := errors.New("db down")
	b := &Budget{MonthlyTokens: 10, Store: &fakeUsage{err: boom}}
	if err := b.Check(context.Background(), 1); !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
}

func TestBudget_DefaultNowUsesCurrentMonth(t *testing.T) {
	st := &fakeUsage{}
	b := &Budget{MonthlyTokens: 10, Store: st}
	if err := b.Check(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if st.since.Day() != 1 || st.since.Month() != now.Month() || st.since.Year() != now.Year() {
		t.Fatalf("since %v", st.since)
	}
}

func TestBudget_Cost(t *testing.T) {
	b := &Budget{PricePer1KIn: 2000, PricePer1KOut: 6000}
	if got := b.Cost(Usage{PromptTokens: 1500, CompletionTokens: 500}); got != 6000 {
		t.Fatalf("cost %d", got)
	}
}
