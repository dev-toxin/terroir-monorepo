package domain

import (
	"strings"
	"time"
)

// Role of a user in a shared cellar ("household").
type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

// Locale is the UI/data language.
type Locale string

const (
	LocaleRU Locale = "ru"
	LocaleEN Locale = "en"
)

// ParseLocale returns ru or en (default ru).
func ParseLocale(s string) Locale {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "en") {
		return LocaleEN
	}
	return LocaleRU
}

// User of Terroir.
type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Locale      Locale    `json:"locale"`
	TelegramID  *int64    `json:"telegram_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Cellar is a household cellar shared by its members.
type Cellar struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Membership links a user to a cellar.
type Membership struct {
	CellarID string `json:"cellar_id"`
	UserID   string `json:"user_id"`
	Role     Role   `json:"role"`
}

// CellarItem is a stack of identical bottles of one wine in a cellar.
// A row with Quantity == 0 is kept for history and notes.
type CellarItem struct {
	ID                 string     `json:"id"`
	CellarID           string     `json:"cellar_id"`
	WineID             string     `json:"wine_id"`
	Quantity           int        `json:"quantity"`
	StorageLocation    string     `json:"storage_location"`
	PurchasePriceMinor *int64     `json:"purchase_price_minor"` // e.g. kopecks/cents
	Currency           string     `json:"currency"`
	PurchasePlace      string     `json:"purchase_place"`
	PurchasedAt        *time.Time `json:"purchased_at"`
	DrinkFrom          *int       `json:"drink_from"`  // year
	DrinkUntil         *int       `json:"drink_until"` // year
	Notes              string     `json:"notes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Normalize trims strings and uppercases currency.
func (c *CellarItem) Normalize() {
	c.StorageLocation = strings.TrimSpace(c.StorageLocation)
	c.Currency = strings.ToUpper(strings.TrimSpace(c.Currency))
	c.PurchasePlace = strings.TrimSpace(c.PurchasePlace)
	c.Notes = strings.TrimSpace(c.Notes)
}

// Validate checks invariants of a cellar item.
func (c *CellarItem) Validate() error {
	if c.WineID == "" {
		return Invalid("wine_id", "is required")
	}
	if c.Quantity < 0 {
		return Invalid("quantity", "must be >= 0")
	}
	if c.Quantity > 10000 {
		return Invalid("quantity", "is unrealistically large")
	}
	if c.PurchasePriceMinor != nil && *c.PurchasePriceMinor < 0 {
		return Invalid("purchase_price_minor", "must be >= 0")
	}
	if c.PurchasePriceMinor != nil && len(c.Currency) != 3 {
		return Invalid("currency", "must be a 3-letter ISO code when price is set")
	}
	for field, y := range map[string]*int{"drink_from": c.DrinkFrom, "drink_until": c.DrinkUntil} {
		if y != nil && (*y < 1900 || *y > 2150) {
			return Invalid(field, "must be a year between 1900 and 2150")
		}
	}
	if c.DrinkFrom != nil && c.DrinkUntil != nil && *c.DrinkFrom > *c.DrinkUntil {
		return Invalid("drink_until", "must be >= drink_from")
	}
	return nil
}

// Drink returns the new quantity after drinking n bottles, or ErrOutOfStock.
func (c *CellarItem) Drink(n int) (int, error) {
	if n <= 0 {
		return c.Quantity, Invalid("bottles", "must be >= 1")
	}
	if c.Quantity < n {
		return c.Quantity, ErrOutOfStock
	}
	return c.Quantity - n, nil
}

// WindowStatus is the drinking-window status of a bottle relative to "now".
type WindowStatus string

const (
	WindowUnknown  WindowStatus = "unknown"
	WindowTooEarly WindowStatus = "too_early" // ещё рано
	WindowReady    WindowStatus = "ready"     // пора пить
	WindowPeak     WindowStatus = "peak"      // пик
	WindowLastCall WindowStatus = "last_call" // последний год окна — открыть срочно
	WindowLate     WindowStatus = "late"      // уже поздно
)

// DrinkingWindow evaluates the drinking window for the given year.
//
// Rules (years, inclusive):
//   - no bounds → unknown
//   - year < from → too_early
//   - year > until → late
//   - year == until (and window longer than 1 year) → last_call
//   - middle third of [from, until] → peak
//   - otherwise inside window → ready
//
// Only "from" known: ready from that year on. Only "until" known: ready until, last_call in that year.
func DrinkingWindow(from, until *int, year int) WindowStatus {
	switch {
	case from == nil && until == nil:
		return WindowUnknown
	case from != nil && year < *from:
		return WindowTooEarly
	case until != nil && year > *until:
		return WindowLate
	case until != nil && year == *until && (from == nil || *from < *until):
		return WindowLastCall
	case from != nil && until != nil:
		span := *until - *from
		if span >= 2 {
			lo := *from + span/3
			hi := *until - span/3
			if year >= lo && year <= hi {
				return WindowPeak
			}
		}
		return WindowReady
	default:
		return WindowReady
	}
}

// Window returns the drinking-window status of the item at clock time.
func (c *CellarItem) Window(now time.Time) WindowStatus {
	return DrinkingWindow(c.DrinkFrom, c.DrinkUntil, now.Year())
}

// ReadyToDrink reports whether it is a good time to open the bottle.
func (s WindowStatus) ReadyToDrink() bool {
	return s == WindowReady || s == WindowPeak || s == WindowLastCall
}

// Urgency orders bottles for "what to open now": higher is more urgent.
func (s WindowStatus) Urgency() int {
	switch s {
	case WindowLate:
		return 5 // already past — drink before it's worse
	case WindowLastCall:
		return 4
	case WindowPeak:
		return 3
	case WindowReady:
		return 2
	case WindowUnknown:
		return 1
	default:
		return 0
	}
}

// Label returns a human-readable label.
func (s WindowStatus) Label(l Locale) string {
	ru := map[WindowStatus]string{
		WindowUnknown: "окно питья неизвестно", WindowTooEarly: "ещё рано", WindowReady: "пора пить",
		WindowPeak: "на пике", WindowLastCall: "последний год окна — откройте скоро", WindowLate: "уже поздно, лучше не тянуть",
	}
	en := map[WindowStatus]string{
		WindowUnknown: "drinking window unknown", WindowTooEarly: "too early", WindowReady: "ready to drink",
		WindowPeak: "at its peak", WindowLastCall: "last year of the window — open soon", WindowLate: "past the window, don't wait",
	}
	if l == LocaleEN {
		return en[s]
	}
	return ru[s]
}

// ConsumptionEvent records that a user drank bottle(s) from a cellar item.
type ConsumptionEvent struct {
	ID             string    `json:"id"`
	CellarItemID   string    `json:"cellar_item_id"`
	WineID         string    `json:"wine_id"`
	UserID         string    `json:"user_id"`
	Bottles        int       `json:"bottles"`
	DrunkAt        time.Time `json:"drunk_at"`
	Rating         int       `json:"rating"` // 1..5, 0 = not rated
	Note           string    `json:"note"`
	Occasion       string    `json:"occasion"`
	PairedRecipeID *string   `json:"paired_recipe_id"`
}

// Validate checks a consumption event.
func (e *ConsumptionEvent) Validate() error {
	if e.Bottles == 0 {
		e.Bottles = 1
	}
	if e.Bottles < 0 || e.Bottles > 24 {
		return Invalid("bottles", "must be between 1 and 24")
	}
	if e.Rating < 0 || e.Rating > 5 {
		return Invalid("rating", "must be between 1 and 5 (0 = no rating)")
	}
	e.Note = strings.TrimSpace(e.Note)
	e.Occasion = strings.TrimSpace(e.Occasion)
	if len(e.Note) > 4000 {
		return Invalid("note", "is too long")
	}
	return nil
}

// CellarEntry is a cellar item joined with its wine. Used by services and the pairing engine.
type CellarEntry struct {
	Item CellarItem `json:"item"`
	Wine Wine       `json:"wine"`
}
