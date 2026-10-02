package domain

import (
	"errors"
	"testing"
	"time"
)

func ip(n int) *int { return &n }

func TestDrinkingWindow(t *testing.T) {
	cases := []struct {
		name        string
		from, until *int
		year        int
		want        WindowStatus
	}{
		{"no bounds", nil, nil, 2026, WindowUnknown},
		{"too early", ip(2028), ip(2035), 2026, WindowTooEarly},
		{"first year ready", ip(2024), ip(2033), 2024, WindowReady},
		{"middle third is peak", ip(2024), ip(2033), 2028, WindowPeak},
		{"after peak still ready", ip(2024), ip(2033), 2032, WindowReady},
		{"last year is last call", ip(2024), ip(2033), 2033, WindowLastCall},
		{"past window is late", ip(2018), ip(2023), 2026, WindowLate},
		{"only from, before", ip(2027), nil, 2026, WindowTooEarly},
		{"only from, after", ip(2020), nil, 2026, WindowReady},
		{"only until, before", nil, ip(2030), 2026, WindowReady},
		{"only until, that year", nil, ip(2026), 2026, WindowLastCall},
		{"only until, after", nil, ip(2025), 2026, WindowLate},
		{"single-year window", ip(2026), ip(2026), 2026, WindowReady},
		{"two-year window, no peak", ip(2025), ip(2026), 2025, WindowReady},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DrinkingWindow(c.from, c.until, c.year); got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestWindowUsesInjectedClock(t *testing.T) {
	it := CellarItem{DrinkFrom: ip(2020), DrinkUntil: ip(2025)}
	clk := FixedClock{T: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)}
	if got := it.Window(clk.Now()); got != WindowLastCall {
		t.Fatalf("2025: got %s", got)
	}
	clk.T = clk.T.AddDate(1, 0, 0)
	if got := it.Window(clk.Now()); got != WindowLate {
		t.Fatalf("2026: got %s", got)
	}
	if !WindowPeak.ReadyToDrink() || WindowTooEarly.ReadyToDrink() || WindowLate.ReadyToDrink() {
		t.Fatal("ReadyToDrink mapping is wrong")
	}
	if WindowLate.Urgency() <= WindowLastCall.Urgency() || WindowLastCall.Urgency() <= WindowPeak.Urgency() {
		t.Fatal("urgency order is wrong")
	}
	for _, s := range []WindowStatus{WindowUnknown, WindowTooEarly, WindowReady, WindowPeak, WindowLastCall, WindowLate} {
		if s.Label(LocaleRU) == "" || s.Label(LocaleEN) == "" || s.Label(LocaleRU) == s.Label(LocaleEN) {
			t.Fatalf("labels for %s missing", s)
		}
	}
}

func TestDrinkDecrement(t *testing.T) {
	it := CellarItem{Quantity: 2}
	if n, err := it.Drink(1); err != nil || n != 1 {
		t.Fatalf("got %d %v", n, err)
	}
	if n, err := it.Drink(2); err != nil || n != 0 {
		t.Fatalf("got %d %v", n, err)
	}
	if _, err := it.Drink(3); !errors.Is(err, ErrOutOfStock) {
		t.Fatalf("want out of stock, got %v", err)
	}
	if _, err := it.Drink(0); !IsValidation(err) {
		t.Fatalf("want validation, got %v", err)
	}
	empty := CellarItem{Quantity: 0}
	if _, err := empty.Drink(1); !errors.Is(err, ErrOutOfStock) {
		t.Fatalf("empty: %v", err)
	}
}

func TestWineValidate(t *testing.T) {
	ok := func() Wine {
		return Wine{Name: " Саперави ", Color: ColorRed, Grapes: []string{" Саперави ", ""}}
	}
	w := ok()
	w.Normalize()
	if err := w.Validate(); err != nil {
		t.Fatal(err)
	}
	if w.Name != "Саперави" || len(w.Grapes) != 1 || w.Source != SourceManual {
		t.Fatalf("normalize: %+v", w)
	}
	if w.VintageLabel() != "NV" {
		t.Fatal("NV label")
	}
	bad := map[string]func(*Wine){
		"name":      func(w *Wine) { w.Name = "" },
		"color":     func(w *Wine) { w.Color = "blue" },
		"sweetness": func(w *Wine) { w.Sweetness = "very" },
		"vintage":   func(w *Wine) { w.Vintage = ip(1800) },
		"abv":       func(w *Wine) { a := 40.0; w.ABV = &a },
		"body":      func(w *Wine) { w.Body = 6 },
		"source":    func(w *Wine) { w.Source = "x" },
	}
	for field, mut := range bad {
		w := ok()
		w.Normalize()
		mut(&w)
		err := w.Validate()
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: got %v", field, err)
		}
	}
	long := ok()
	for range 210 {
		long.Name += "я"
	}
	if err := long.Validate(); !IsValidation(err) {
		t.Error("long name accepted")
	}
}

func TestItemValidate(t *testing.T) {
	price := int64(100)
	neg := int64(-1)
	cases := map[string]CellarItem{
		"wine_id":              {Quantity: 1},
		"quantity":             {WineID: "w", Quantity: -1},
		"currency":             {WineID: "w", Quantity: 1, PurchasePriceMinor: &price},
		"purchase_price_minor": {WineID: "w", Quantity: 1, PurchasePriceMinor: &neg, Currency: "RUB"},
		"drink_until":          {WineID: "w", Quantity: 1, DrinkFrom: ip(2030), DrinkUntil: ip(2025)},
		"drink_from":           {WineID: "w", Quantity: 1, DrinkFrom: ip(1800)},
	}
	for field, it := range cases {
		var ve *ValidationError
		if err := it.Validate(); !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: got %v", field, err)
		}
	}
	good := CellarItem{WineID: "w", Quantity: 0, PurchasePriceMinor: &price, Currency: " rub "}
	good.Normalize()
	if err := good.Validate(); err != nil || good.Currency != "RUB" {
		t.Fatalf("good: %v %q", err, good.Currency)
	}
}

func TestConsumptionValidate(t *testing.T) {
	e := ConsumptionEvent{}
	if err := e.Validate(); err != nil || e.Bottles != 1 {
		t.Fatalf("default bottles: %v %d", err, e.Bottles)
	}
	if err := (&ConsumptionEvent{Rating: 6}).Validate(); !IsValidation(err) {
		t.Fatal("rating 6 accepted")
	}
	if err := (&ConsumptionEvent{Bottles: -1}).Validate(); !IsValidation(err) {
		t.Fatal("negative bottles accepted")
	}
}

func TestGrapesAndStructure(t *testing.T) {
	for _, name := range []string{"Саперави", "saperavi", "SAPERAVI"} {
		if CanonicalGrape(name) != "saperavi" {
			t.Fatalf("%s", name)
		}
	}
	if CanonicalGrape("Шираз") != "syrah" || CanonicalGrape("Цимлянский черный") != "tsimlyansky_cherny" {
		t.Fatal("aliases")
	}
	if CanonicalGrape("Unknown Grape") != "unknown grape" {
		t.Fatal("unknown passthrough")
	}
	w := Wine{Color: ColorRed, Grapes: []string{"Пино Нуар"}}
	s := w.ResolveStructure()
	if s.Tannin != 2 || s.Acidity != 4 || s.DarkFruit {
		t.Fatalf("pinot: %+v", s)
	}
	w.Tannin = 5
	if w.ResolveStructure().Tannin != 5 {
		t.Fatal("explicit tannin must win")
	}
	white := Wine{Color: ColorWhite, Grapes: []string{"Unknown"}}
	if st := white.ResolveStructure(); st.Tannin != 0 || st.Body != 3 {
		t.Fatalf("white defaults: %+v", st)
	}
	if !w.HasGrape("pinot noir") || w.HasGrape("merlot") {
		t.Fatal("HasGrape")
	}
	for _, c := range AllColors {
		_ = (&Wine{Color: c}).ResolveStructure()
	}
}

func TestSurveyApplyAndBatch(t *testing.T) {
	p := TasteProfile{}
	answered := map[string]bool{}
	b1 := NextBatch(SurveyBank, answered, 6)
	if len(b1) != 6 || b1[0].ID != "q_body" {
		t.Fatalf("batch1: %d", len(b1))
	}
	for _, q := range b1 {
		answered[q.ID] = true
	}
	b2 := NextBatch(SurveyBank, answered, 6)
	if len(b2) == 0 || b2[0].ID == b1[0].ID {
		t.Fatal("batch2 repeats")
	}
	q, _ := FindQuestion(SurveyBank, "q_acidity")
	if err := ApplyAnswer(&p, q, SurveyAnswer{Values: []string{"2"}}); err != nil || p.Acidity != 2 || p.Version != 1 {
		t.Fatalf("scale: %v %+v", err, p)
	}
	if err := ApplyAnswer(&p, q, SurveyAnswer{Values: []string{"9"}}); !IsValidation(err) {
		t.Fatal("bad scale accepted")
	}
	if err := ApplyAnswer(&p, q, SurveyAnswer{}); !IsValidation(err) {
		t.Fatal("empty accepted")
	}
	like, _ := FindQuestion(SurveyBank, "q_grapes_like")
	if err := ApplyAnswer(&p, like, SurveyAnswer{Values: []string{"saperavi", "syrah", "saperavi"}}); err != nil {
		t.Fatal(err)
	}
	if len(p.LikedGrapes) != 2 || !p.Likes("Саперави") {
		t.Fatalf("liked: %v", p.LikedGrapes)
	}
	rej, _ := FindQuestion(SurveyBank, "q_grapes_reject")
	_ = ApplyAnswer(&p, rej, SurveyAnswer{Values: []string{"syrah"}})
	if p.Likes("syrah") || !p.Rejects("Сира") {
		t.Fatal("reject must remove from liked")
	}
	spice, _ := FindQuestion(SurveyBank, "q_spice")
	if err := ApplyAnswer(&p, spice, SurveyAnswer{Values: []string{"3", "2"}}); !IsValidation(err) {
		t.Fatal("choice with two values accepted")
	}
	if err := ApplyAnswer(&p, spice, SurveyAnswer{Values: []string{"7"}}); !IsValidation(err) {
		t.Fatal("unknown option accepted")
	}
	_ = ApplyAnswer(&p, spice, SurveyAnswer{Values: []string{"3"}})
	steak, _ := FindQuestion(SurveyBank, "q_steak")
	_ = ApplyAnswer(&p, steak, SurveyAnswer{Values: []string{"jammy"}})
	budget, _ := FindQuestion(SurveyBank, "q_budget")
	_ = ApplyAnswer(&p, budget, SurveyAnswer{Values: []string{"mid"}})
	colors, _ := FindQuestion(SurveyBank, "q_colors")
	_ = ApplyAnswer(&p, colors, SurveyAnswer{Values: []string{"red", "red", "white"}})
	regions, _ := FindQuestion(SurveyBank, "q_regions")
	_ = ApplyAnswer(&p, regions, SurveyAnswer{Values: []string{"Kakheti"}})
	if p.SpiceTolerance != 3 || p.FruitDark != 5 || p.BudgetMaxMinor != 250000 || len(p.Colors) != 2 || len(p.Regions) != 1 {
		t.Fatalf("profile: %+v", p)
	}
	for _, id := range []string{"q_body", "q_tannin", "q_sweet", "q_fruit_red", "q_oak"} {
		q, ok := FindQuestion(SurveyBank, id)
		if !ok {
			t.Fatal(id)
		}
		if err := ApplyAnswer(&p, q, SurveyAnswer{Values: []string{"4"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := FindQuestion(SurveyBank, "nope"); ok {
		t.Fatal("found unknown")
	}
	if got := NextBatch(SurveyBank, map[string]bool{}, 0); len(got) != 6 {
		t.Fatal("default size")
	}
	if err := ApplyAnswer(&p, Question{ID: "x", Type: "weird"}, SurveyAnswer{Values: []string{"1"}}); !IsValidation(err) {
		t.Fatal("weird type accepted")
	}
}

func TestRecipeValidate(t *testing.T) {
	r := Recipe{Title: " Плов ", MainIngredient: IngLamb, Tags: []string{" Рис ", ""}}
	r.Normalize()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if r.CookingMethod != MethodOther || r.Sauce != SauceNone || r.Richness != 3 || len(r.Tags) != 1 || r.Tags[0] != "рис" {
		t.Fatalf("normalize: %+v", r)
	}
	bad := map[string]func(*Recipe){
		"title":           func(r *Recipe) { r.Title = "" },
		"main_ingredient": func(r *Recipe) { r.MainIngredient = "tofu" },
		"cooking_method":  func(r *Recipe) { r.CookingMethod = "microwave" },
		"sauce":           func(r *Recipe) { r.Sauce = "ketchup" },
		"spiciness":       func(r *Recipe) { r.Spiciness = 4 },
		"richness":        func(r *Recipe) { r.Richness = 9 },
		"acidity":         func(r *Recipe) { r.Acidity = 0 },
		"time_minutes":    func(r *Recipe) { r.TimeMinutes = -5 },
	}
	for field, mut := range bad {
		c := r
		mut(&c)
		var ve *ValidationError
		if err := c.Validate(); !errors.As(err, &ve) || ve.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
}

func TestMisc(t *testing.T) {
	if ParseLocale("en-US") != LocaleEN || ParseLocale("") != LocaleRU {
		t.Fatal("locale")
	}
	if (&ValidationError{Message: "m"}).Error() != "m" {
		t.Fatal("error without field")
	}
	if (SystemClock{}).Now().IsZero() {
		t.Fatal("clock")
	}
}
