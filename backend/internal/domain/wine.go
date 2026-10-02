package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Color is the wine style family.
type Color string

const (
	ColorRed       Color = "red"
	ColorWhite     Color = "white"
	ColorRose      Color = "rose"
	ColorOrange    Color = "orange"
	ColorSparkling Color = "sparkling"
	ColorDessert   Color = "dessert"
	ColorFortified Color = "fortified"
)

// AllColors lists colors in display order.
var AllColors = []Color{ColorRed, ColorWhite, ColorRose, ColorOrange, ColorSparkling, ColorDessert, ColorFortified}

// Valid reports whether c is a known color.
func (c Color) Valid() bool {
	for _, k := range AllColors {
		if c == k {
			return true
		}
	}
	return false
}

// Sweetness of the wine.
type Sweetness string

const (
	SweetnessDry      Sweetness = "dry"
	SweetnessOffDry   Sweetness = "off_dry"
	SweetnessMedium   Sweetness = "medium"
	SweetnessSweet    Sweetness = "sweet"
	SweetnessUnknown  Sweetness = ""
	maxWineNameLength           = 200
)

// Valid reports whether s is a known sweetness (empty = unknown is valid).
func (s Sweetness) Valid() bool {
	switch s {
	case SweetnessDry, SweetnessOffDry, SweetnessMedium, SweetnessSweet, SweetnessUnknown:
		return true
	}
	return false
}

// Source tells where a wine card came from.
type Source string

const (
	SourceManual Source = "manual"
	SourceImport Source = "import"
	SourceLLM    Source = "llm"
	SourceSeed   Source = "seed"
)

// Wine is a wine card in a household cellar catalogue.
// Body, Acidity and Tannin are 1..5 (0 = unknown; the pairing engine then infers them from grapes/color).
type Wine struct {
	ID          string    `json:"id"`
	CellarID    string    `json:"cellar_id"`
	Name        string    `json:"name"`
	Producer    string    `json:"producer"`
	Country     string    `json:"country"`
	Region      string    `json:"region"`
	Appellation string    `json:"appellation"`
	Grapes      []string  `json:"grapes"`
	Vintage     *int      `json:"vintage"` // nil = non-vintage («Н/В»)
	Color       Color     `json:"color"`
	Sweetness   Sweetness `json:"sweetness"`
	ABV         *float64  `json:"abv"`
	Body        int       `json:"body"`
	Acidity     int       `json:"acidity"`
	Tannin      int       `json:"tannin"`
	StyleNotes  string    `json:"style_notes"`
	Source      Source    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Normalize trims strings and removes empty grapes.
func (w *Wine) Normalize() {
	w.Name = strings.TrimSpace(w.Name)
	w.Producer = strings.TrimSpace(w.Producer)
	w.Country = strings.TrimSpace(w.Country)
	w.Region = strings.TrimSpace(w.Region)
	w.Appellation = strings.TrimSpace(w.Appellation)
	w.StyleNotes = strings.TrimSpace(w.StyleNotes)
	grapes := make([]string, 0, len(w.Grapes))
	for _, g := range w.Grapes {
		if g = strings.TrimSpace(g); g != "" {
			grapes = append(grapes, g)
		}
	}
	w.Grapes = grapes
	if w.Source == "" {
		w.Source = SourceManual
	}
}

// Validate checks invariants of a wine card.
func (w *Wine) Validate() error {
	if w.Name == "" {
		return Invalid("name", "is required")
	}
	if utf8.RuneCountInString(w.Name) > maxWineNameLength {
		return Invalid("name", "must be at most %d characters", maxWineNameLength)
	}
	if !w.Color.Valid() {
		return Invalid("color", "must be one of red, white, rose, orange, sparkling, dessert, fortified")
	}
	if !w.Sweetness.Valid() {
		return Invalid("sweetness", "must be one of dry, off_dry, medium, sweet")
	}
	if w.Vintage != nil && (*w.Vintage < 1900 || *w.Vintage > 2100) {
		return Invalid("vintage", "must be between 1900 and 2100 or null for non-vintage")
	}
	if w.ABV != nil && (*w.ABV < 0 || *w.ABV > 25) {
		return Invalid("abv", "must be between 0 and 25")
	}
	for field, v := range map[string]int{"body": w.Body, "acidity": w.Acidity, "tannin": w.Tannin} {
		if v < 0 || v > 5 {
			return Invalid(field, "must be between 0 (unknown) and 5")
		}
	}
	switch w.Source {
	case SourceManual, SourceImport, SourceLLM, SourceSeed:
	default:
		return Invalid("source", "must be one of manual, import, llm, seed")
	}
	return nil
}

// VintageLabel returns "2019" or "NV".
func (w *Wine) VintageLabel() string {
	if w.Vintage == nil {
		return "NV"
	}
	return strconv.Itoa(*w.Vintage)
}

// HasGrape reports whether the wine contains a grape (case-insensitive, alias-aware).
func (w *Wine) HasGrape(grape string) bool {
	want := CanonicalGrape(grape)
	for _, g := range w.Grapes {
		if CanonicalGrape(g) == want {
			return true
		}
	}
	return false
}
