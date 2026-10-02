package domain

import "strings"

// GrapeTraits are typical structural traits of a grape variety on a 1..5 scale.
// They are used when a wine card does not specify body/acidity/tannin explicitly.
type GrapeTraits struct {
	Canonical string // canonical English key, e.g. "saperavi"
	NameRU    string
	NameEN    string
	Body      int
	Acidity   int
	Tannin    int
	DarkFruit bool // dark (blackberry, plum, cherry jam) vs red/fresh fruit
}

// grapeTable is a small, opinionated table of varieties relevant to the owner's cellar.
// Keep it short and explicit; extend when real bottles need it.
var grapeTable = []GrapeTraits{
	{"saperavi", "Саперави", "Saperavi", 5, 4, 4, true},
	{"krasnostop", "Красностоп", "Krasnostop", 4, 4, 5, true},
	{"tsimlyansky_cherny", "Цимлянский чёрный", "Tsimlyansky Cherny", 4, 3, 3, true},
	{"cabernet_sauvignon", "Каберне Совиньон", "Cabernet Sauvignon", 4, 3, 5, true},
	{"merlot", "Мерло", "Merlot", 4, 3, 3, true},
	{"syrah", "Сира", "Syrah", 4, 3, 4, true},
	{"pinot_noir", "Пино Нуар", "Pinot Noir", 2, 4, 2, false},
	{"grenache", "Гренаш", "Grenache", 3, 2, 2, true},
	{"malbec", "Мальбек", "Malbec", 4, 3, 4, true},
	{"rkatsiteli", "Ркацители", "Rkatsiteli", 3, 4, 1, false},
	{"kisi", "Киси", "Kisi", 3, 3, 1, false},
	{"kokur", "Кокур", "Kokur", 2, 4, 0, false},
	{"viognier", "Вионье", "Viognier", 4, 2, 0, false},
	{"chardonnay", "Шардоне", "Chardonnay", 4, 3, 0, false},
	{"sauvignon_blanc", "Совиньон Блан", "Sauvignon Blanc", 2, 5, 0, false},
	{"riesling", "Рислинг", "Riesling", 2, 5, 0, false},
	{"aligote", "Алиготе", "Aligote", 2, 5, 0, false},
	{"gewurztraminer", "Гевюрцтраминер", "Gewurztraminer", 4, 2, 0, false},
	{"muscat", "Мускат", "Muscat", 3, 2, 0, false},
}

var grapeIndex = func() map[string]GrapeTraits {
	m := make(map[string]GrapeTraits, len(grapeTable)*3)
	for _, g := range grapeTable {
		m[normKey(g.Canonical)] = g
		m[normKey(g.NameRU)] = g
		m[normKey(g.NameEN)] = g
	}
	// common spelling aliases
	alias := map[string]string{
		"шираз": "syrah", "shiraz": "syrah", "цимлянский": "tsimlyansky_cherny",
		"цимлянский черный": "tsimlyansky_cherny", "pinot": "pinot_noir", "пино": "pinot_noir",
		"каберне": "cabernet_sauvignon", "cabernet": "cabernet_sauvignon", "мускат белый": "muscat",
	}
	for a, c := range alias {
		m[normKey(a)] = m[normKey(c)]
	}
	return m
}()

func normKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.Join(strings.Fields(s), " ")
}

// LookupGrape finds traits for a grape name in RU or EN.
func LookupGrape(name string) (GrapeTraits, bool) {
	g, ok := grapeIndex[normKey(name)]
	return g, ok
}

// CanonicalGrape returns the canonical key for a grape, or the normalized input if unknown.
func CanonicalGrape(name string) string {
	if g, ok := LookupGrape(name); ok {
		return g.Canonical
	}
	return normKey(name)
}

// Structure is the resolved structural profile of a wine (1..5 each).
type Structure struct {
	Body      int
	Acidity   int
	Tannin    int
	DarkFruit bool
}

// ResolveStructure returns explicit values from the card, falling back to grape traits and color defaults.
func (w *Wine) ResolveStructure() Structure {
	var s Structure
	var n, body, acid, tan int
	dark := 0
	for _, g := range w.Grapes {
		if t, ok := LookupGrape(g); ok {
			n++
			body += t.Body
			acid += t.Acidity
			tan += t.Tannin
			if t.DarkFruit {
				dark++
			}
		}
	}
	if n > 0 {
		s = Structure{Body: roundDiv(body, n), Acidity: roundDiv(acid, n), Tannin: roundDiv(tan, n), DarkFruit: dark*2 >= n && dark > 0}
	} else {
		s = colorDefaults(w.Color)
	}
	if w.Body > 0 {
		s.Body = w.Body
	}
	if w.Acidity > 0 {
		s.Acidity = w.Acidity
	}
	if w.Tannin > 0 {
		s.Tannin = w.Tannin
	}
	if w.Color != ColorRed && w.Color != ColorFortified {
		// whites/roses/sparkling have no meaningful tannin unless stated (orange wines do).
		if w.Tannin == 0 && w.Color != ColorOrange {
			s.Tannin = 0
		}
	}
	return s
}

func colorDefaults(c Color) Structure {
	switch c {
	case ColorRed:
		return Structure{Body: 3, Acidity: 3, Tannin: 3, DarkFruit: true}
	case ColorOrange:
		return Structure{Body: 3, Acidity: 3, Tannin: 2}
	case ColorSparkling:
		return Structure{Body: 2, Acidity: 5}
	case ColorDessert:
		return Structure{Body: 4, Acidity: 3}
	case ColorFortified:
		return Structure{Body: 5, Acidity: 2, Tannin: 2, DarkFruit: true}
	case ColorRose:
		return Structure{Body: 2, Acidity: 4}
	default:
		return Structure{Body: 3, Acidity: 3}
	}
}

func roundDiv(a, b int) int { return (2*a + b) / (2 * b) }
