package service

import (
	"context"
	"sort"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// Reminder is a "time to open" notice (no delivery yet — only the selection).
type Reminder struct {
	Item    ItemView `json:"bottle"`
	Message string   `json:"message"`
}

// Reminders selects bottles whose window is in its last year or already past, plus peak bottles
// with only one left. Sorted by urgency, then by drink_until.
func (s *Service) Reminders(ctx context.Context, a Actor) ([]Reminder, error) {
	items, err := s.ListItems(ctx, a, ItemFilter{})
	if err != nil {
		return nil, err
	}
	var out []Reminder
	for _, v := range items {
		switch {
		case v.Window == domain.WindowLate, v.Window == domain.WindowLastCall:
		case v.Window == domain.WindowPeak && v.Item.Quantity == 1:
		default:
			continue
		}
		out = append(out, Reminder{Item: v, Message: WineTitle(v.Wine) + " — " + v.WindowLabel})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Item.Window.Urgency() != out[j].Item.Window.Urgency() {
			return out[i].Item.Window.Urgency() > out[j].Item.Window.Urgency()
		}
		return untilOr(out[i].Item) < untilOr(out[j].Item)
	})
	return out, nil
}
