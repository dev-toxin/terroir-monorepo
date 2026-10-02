package service

import (
	"context"
	"sort"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

// ---------- wines ----------

// WinePatch carries optional fields for PATCH /v1/wines/{id}.
type WinePatch struct {
	Name        Opt[string]           `json:"name"`
	Producer    Opt[string]           `json:"producer"`
	Country     Opt[string]           `json:"country"`
	Region      Opt[string]           `json:"region"`
	Appellation Opt[string]           `json:"appellation"`
	Grapes      Opt[[]string]         `json:"grapes"`
	Vintage     Opt[int]              `json:"vintage"` // null = non-vintage
	Color       Opt[domain.Color]     `json:"color"`
	Sweetness   Opt[domain.Sweetness] `json:"sweetness"`
	ABV         Opt[float64]          `json:"abv"`
	Body        Opt[int]              `json:"body"`
	Acidity     Opt[int]              `json:"acidity"`
	Tannin      Opt[int]              `json:"tannin"`
	StyleNotes  Opt[string]           `json:"style_notes"`
}

// CreateWine validates and stores a wine card in the actor's cellar.
func (s *Service) CreateWine(ctx context.Context, a Actor, w domain.Wine) (domain.Wine, error) {
	now := s.clock.Now()
	w.ID, w.CellarID, w.CreatedAt, w.UpdatedAt = s.newID(), a.CellarID, now, now
	w.Normalize()
	if err := w.Validate(); err != nil {
		return domain.Wine{}, err
	}
	return w, s.store.CreateWine(ctx, w)
}

// GetWine returns a wine of the actor's cellar.
func (s *Service) GetWine(ctx context.Context, a Actor, id string) (domain.Wine, error) {
	return s.store.GetWine(ctx, a.CellarID, id)
}

// ListWines lists wine cards.
func (s *Service) ListWines(ctx context.Context, a Actor) ([]domain.Wine, error) {
	return s.store.ListWines(ctx, a.CellarID)
}

// UpdateWine applies a patch.
func (s *Service) UpdateWine(ctx context.Context, a Actor, id string, p WinePatch) (domain.Wine, error) {
	w, err := s.store.GetWine(ctx, a.CellarID, id)
	if err != nil {
		return w, err
	}
	p.Name.Apply(&w.Name)
	p.Producer.Apply(&w.Producer)
	p.Country.Apply(&w.Country)
	p.Region.Apply(&w.Region)
	p.Appellation.Apply(&w.Appellation)
	p.StyleNotes.Apply(&w.StyleNotes)
	p.Grapes.Apply(&w.Grapes)
	p.Vintage.ApplyPtr(&w.Vintage)
	p.Color.Apply(&w.Color)
	p.Sweetness.Apply(&w.Sweetness)
	p.ABV.ApplyPtr(&w.ABV)
	p.Body.Apply(&w.Body)
	p.Acidity.Apply(&w.Acidity)
	p.Tannin.Apply(&w.Tannin)
	w.Normalize()
	if err := w.Validate(); err != nil {
		return w, err
	}
	w.UpdatedAt = s.clock.Now()
	return w, s.store.UpdateWine(ctx, w)
}

// DeleteWine deletes a wine card with no bottles.
func (s *Service) DeleteWine(ctx context.Context, a Actor, id string) error {
	return s.store.DeleteWine(ctx, a.CellarID, id)
}

// ---------- cellar items ----------

// ItemPatch carries optional fields for PATCH /v1/cellar/items/{id}.
type ItemPatch struct {
	Quantity           Opt[int]       `json:"quantity"`
	StorageLocation    Opt[string]    `json:"storage_location"`
	PurchasePriceMinor Opt[int64]     `json:"purchase_price_minor"`
	Currency           Opt[string]    `json:"currency"`
	PurchasePlace      Opt[string]    `json:"purchase_place"`
	PurchasedAt        Opt[time.Time] `json:"purchased_at"`
	DrinkFrom          Opt[int]       `json:"drink_from"`
	DrinkUntil         Opt[int]       `json:"drink_until"`
	Notes              Opt[string]    `json:"notes"`
}

// ItemView is a cellar entry enriched with the drinking-window status.
type ItemView struct {
	domain.CellarEntry
	Window      domain.WindowStatus `json:"window"`
	WindowLabel string              `json:"window_label"`
}

// ItemFilter extends the storage filter with window-based filters.
type ItemFilter struct {
	storage.CellarFilter
	ReadyToDrink bool
}

func (s *Service) view(e domain.CellarEntry, l domain.Locale) ItemView {
	w := e.Item.Window(s.clock.Now())
	return ItemView{CellarEntry: e, Window: w, WindowLabel: w.Label(l)}
}

// AddItem adds bottles of an existing wine to the cellar.
func (s *Service) AddItem(ctx context.Context, a Actor, it domain.CellarItem) (ItemView, error) {
	now := s.clock.Now()
	it.ID, it.CellarID, it.CreatedAt, it.UpdatedAt = s.newID(), a.CellarID, now, now
	it.Normalize()
	if err := it.Validate(); err != nil {
		return ItemView{}, err
	}
	if err := s.store.CreateItem(ctx, it); err != nil {
		return ItemView{}, err
	}
	e, err := s.store.GetItem(ctx, a.CellarID, it.ID)
	if err != nil {
		return ItemView{}, err
	}
	return s.view(e, a.User.Locale), nil
}

// GetItem returns one bottle stack.
func (s *Service) GetItem(ctx context.Context, a Actor, id string) (ItemView, error) {
	e, err := s.store.GetItem(ctx, a.CellarID, id)
	if err != nil {
		return ItemView{}, err
	}
	return s.view(e, a.User.Locale), nil
}

// ListItems lists bottles with filters. Default: only bottles in stock.
func (s *Service) ListItems(ctx context.Context, a Actor, f ItemFilter) ([]ItemView, error) {
	entries, err := s.store.ListItems(ctx, a.CellarID, f.CellarFilter)
	if err != nil {
		return nil, err
	}
	out := make([]ItemView, 0, len(entries))
	for _, e := range entries {
		v := s.view(e, a.User.Locale)
		if f.ReadyToDrink && !v.Window.ReadyToDrink() && v.Window != domain.WindowLate {
			continue
		}
		out = append(out, v)
	}
	if f.ReadyToDrink {
		sort.SliceStable(out, func(i, j int) bool { return out[i].Window.Urgency() > out[j].Window.Urgency() })
	}
	return out, nil
}

// UpdateItem applies a patch.
func (s *Service) UpdateItem(ctx context.Context, a Actor, id string, p ItemPatch) (ItemView, error) {
	e, err := s.store.GetItem(ctx, a.CellarID, id)
	if err != nil {
		return ItemView{}, err
	}
	it := e.Item
	p.Quantity.Apply(&it.Quantity)
	p.StorageLocation.Apply(&it.StorageLocation)
	p.Currency.Apply(&it.Currency)
	p.PurchasePlace.Apply(&it.PurchasePlace)
	p.Notes.Apply(&it.Notes)
	p.PurchasePriceMinor.ApplyPtr(&it.PurchasePriceMinor)
	p.PurchasedAt.ApplyPtr(&it.PurchasedAt)
	p.DrinkFrom.ApplyPtr(&it.DrinkFrom)
	p.DrinkUntil.ApplyPtr(&it.DrinkUntil)
	it.Normalize()
	if err := it.Validate(); err != nil {
		return ItemView{}, err
	}
	it.UpdatedAt = s.clock.Now()
	if err := s.store.UpdateItem(ctx, it); err != nil {
		return ItemView{}, err
	}
	e.Item = it
	return s.view(e, a.User.Locale), nil
}

// DrinkRequest is the body of POST /v1/cellar/items/{id}/drink.
type DrinkRequest struct {
	Bottles        int     `json:"bottles"`
	Rating         int     `json:"rating"`
	Note           string  `json:"note"`
	Occasion       string  `json:"occasion"`
	PairedRecipeID *string `json:"paired_recipe_id"`
}

// DrinkResult is returned after "drank it".
type DrinkResult struct {
	Event domain.ConsumptionEvent `json:"event"`
	Item  domain.CellarItem       `json:"item"`
}

// Drink records consumption atomically; quantity never goes below zero.
func (s *Service) Drink(ctx context.Context, a Actor, itemID string, req DrinkRequest) (DrinkResult, error) {
	ev := domain.ConsumptionEvent{
		ID: s.newID(), CellarItemID: itemID, UserID: a.User.ID, Bottles: req.Bottles, DrunkAt: s.clock.Now(),
		Rating: req.Rating, Note: req.Note, Occasion: req.Occasion, PairedRecipeID: req.PairedRecipeID,
	}
	if ev.PairedRecipeID != nil && *ev.PairedRecipeID == "" {
		ev.PairedRecipeID = nil
	}
	if err := ev.Validate(); err != nil {
		return DrinkResult{}, err
	}
	if ev.PairedRecipeID != nil {
		if _, err := s.store.GetRecipe(ctx, a.CellarID, *ev.PairedRecipeID); err != nil {
			if err == domain.ErrNotFound {
				return DrinkResult{}, domain.Invalid("paired_recipe_id", "recipe not found")
			}
			return DrinkResult{}, err
		}
	}
	item, err := s.store.Drink(ctx, a.CellarID, ev)
	if err != nil {
		return DrinkResult{}, err
	}
	ev.WineID = item.WineID
	return DrinkResult{Event: ev, Item: item}, nil
}

// ---------- summary ----------

// Summary is the cellar overview.
type Summary struct {
	TotalBottles int            `json:"total_bottles"`
	TotalItems   int            `json:"total_items"`
	ByColor      map[string]int `json:"by_color"`
	ByWindow     map[string]int `json:"by_window"`
	OpenSoon     []ItemView     `json:"open_soon"`
	RecentDrinks int            `json:"drunk_last_30_days"`
}

// Summary returns totals by color/window and the top-N bottles to open soon.
func (s *Service) Summary(ctx context.Context, a Actor, topN int) (Summary, error) {
	if topN <= 0 {
		topN = 5
	}
	items, err := s.ListItems(ctx, a, ItemFilter{})
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{ByColor: map[string]int{}, ByWindow: map[string]int{}, OpenSoon: []ItemView{}}
	var ready []ItemView
	for _, v := range items {
		sum.TotalBottles += v.Item.Quantity
		sum.TotalItems++
		sum.ByColor[string(v.Wine.Color)] += v.Item.Quantity
		sum.ByWindow[string(v.Window)] += v.Item.Quantity
		if v.Window.ReadyToDrink() || v.Window == domain.WindowLate {
			ready = append(ready, v)
		}
	}
	sort.SliceStable(ready, func(i, j int) bool {
		if ready[i].Window.Urgency() != ready[j].Window.Urgency() {
			return ready[i].Window.Urgency() > ready[j].Window.Urgency()
		}
		return untilOr(ready[i]) < untilOr(ready[j])
	})
	if len(ready) > topN {
		ready = ready[:topN]
	}
	sum.OpenSoon = append(sum.OpenSoon, ready...)
	events, err := s.store.ListConsumption(ctx, a.CellarID, "", 500)
	if err != nil {
		return Summary{}, err
	}
	since := s.clock.Now().AddDate(0, 0, -30)
	for _, e := range events {
		if e.DrunkAt.After(since) {
			sum.RecentDrinks += e.Bottles
		}
	}
	return sum, nil
}

func untilOr(v ItemView) int {
	if v.Item.DrinkUntil == nil {
		return 9999
	}
	return *v.Item.DrinkUntil
}

// History returns drink events of the cellar.
func (s *Service) History(ctx context.Context, a Actor, limit int) ([]domain.ConsumptionEvent, error) {
	return s.store.ListConsumption(ctx, a.CellarID, "", limit)
}
