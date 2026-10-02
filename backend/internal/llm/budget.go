package llm

import (
	"context"
	"sync"
	"time"
)

// UsageStore persists LLM usage for monthly budgeting.
type UsageStore interface {
	UsageSince(ctx context.Context, t time.Time) (tokens int64, costMicros int64, err error)
}

// Budget limits monthly spend in tokens and/or money. Zero limits mean "unlimited".
type Budget struct {
	MonthlyTokens int64
	MonthlyMicros int64
	PricePer1KIn  int64 // micros
	PricePer1KOut int64 // micros
	Store         UsageStore
	Now           func() time.Time
	mu            sync.Mutex
}

// Cost returns the cost of a call in micros.
func (b *Budget) Cost(u Usage) int64 {
	return (int64(u.PromptTokens)*b.PricePer1KIn + int64(u.CompletionTokens)*b.PricePer1KOut) / 1000
}

// Check returns ErrBudgetExceeded if the month's usage plus the estimated prompt would exceed a limit.
func (b *Budget) Check(ctx context.Context, estimatedPromptTokens int) error {
	if b == nil || (b.MonthlyTokens == 0 && b.MonthlyMicros == 0) {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now().UTC()
	if b.Now != nil {
		now = b.Now().UTC()
	}
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	tokens, micros, err := b.Store.UsageSince(ctx, start)
	if err != nil {
		return err
	}
	if b.MonthlyTokens > 0 && tokens+int64(estimatedPromptTokens) > b.MonthlyTokens {
		return ErrBudgetExceeded
	}
	est := b.Cost(Usage{PromptTokens: estimatedPromptTokens})
	if b.MonthlyMicros > 0 && micros+est > b.MonthlyMicros {
		return ErrBudgetExceeded
	}
	return nil
}
