package pairing

import (
	"strings"
	"testing"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// ---------- fixtures (fictional, mirror the demo cellar) ----------

func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

func mkWine(id string, c domain.Color, s domain.Sweetness, abv float64, body, acid, tan int, grapes ...string) domain.Wine {
	return domain.Wine{ID: id, Name: id, Color: c, Sweetness: s, ABV: f64(abv), Body: body, Acidity: acid, Tannin: tan, Grapes: grapes}
}

var (
	saperavi   = mkWine("saperavi", domain.ColorRed, domain.SweetnessDry, 14, 5, 4, 4, "Саперави")
	krasnostop = mkWine("krasnostop", domain.ColorRed, domain.SweetnessDry, 13.5, 4, 4, 5, "Красностоп")
	tsimla     = mkWine("tsimla", domain.ColorRed, domain.SweetnessDry, 13, 4, 3, 3, "Цимлянский чёрный")
	cabernet   = mkWine("cabernet", domain.ColorRed, domain.SweetnessDry, 14.5, 5, 3, 5, "Каберне Совиньон")
	pinot      = mkWine("pinot", domain.ColorRed, domain.SweetnessDry, 12.5, 2, 4, 2, "Пино Нуар")
	rieslingOD = mkWine("riesling", domain.ColorWhite, domain.SweetnessOffDry, 11, 2, 5, 0, "Рислинг")
	viognier   = mkWine("viognier", domain.ColorWhite, domain.SweetnessDry, 13.5, 4, 2, 0, "Вионье")
	chardonnay = mkWine("chardonnay", domain.ColorWhite, domain.SweetnessDry, 13, 4, 3, 0, "Шардоне")
	rkatsiteli = mkWine("rkatsiteli", domain.ColorWhite, domain.SweetnessDry, 12.5, 3, 4, 0, "Ркацители")
	kisiAmber  = mkWine("kisi-amber", domain.ColorOrange, domain.SweetnessDry, 13, 4, 3, 2, "Киси")
	muscat     = mkWine("muscat", domain.ColorDessert, domain.SweetnessSweet, 16, 4, 3, 0, "Мускат")
	brut       = mkWine("brut", domain.ColorSparkling, domain.SweetnessDry, 12, 2, 5, 0, "Шардоне", "Пино Нуар")
	kindzma    = mkWine("kindzmarauli", domain.ColorRed, domain.SweetnessMedium, 11.5, 4, 3, 2, "Саперави")
)

func mkRecipe(id string, ing domain.MainIngredient, m domain.CookingMethod, s domain.Sauce, spice, rich, acid int) domain.Recipe {
	return domain.Recipe{ID: id, Title: id, MainIngredient: ing, CookingMethod: m, Sauce: s, Spiciness: spice, Richness: rich, Acidity: acid}
}

var (
	tteokbokki = mkRecipe("tteokbokki", domain.IngVegetables, domain.MethodStewed, domain.SauceSweet, 3, 3, 3)
	kimchiPork = mkRecipe("kimchi-pork", domain.IngPork, domain.MethodFried, domain.SauceSoy, 3, 4, 3)
	thaiCurry  = mkRecipe("thai-curry", domain.IngPoultry, domain.MethodStewed, domain.SauceCurry, 2, 4, 2)
	tandoori   = mkRecipe("tandoori", domain.IngPoultry, domain.MethodGrilled, domain.SauceCurry, 2, 3, 3)
	steak      = mkRecipe("steak", domain.IngBeef, domain.MethodGrilled, domain.SauceNone, 0, 5, 1)
	beefCheeks = mkRecipe("beef-cheeks", domain.IngBeef, domain.MethodBraised, domain.SauceRed, 0, 5, 3)
	fishCream  = mkRecipe("fish-cream", domain.IngFish, domain.MethodBaked, domain.SauceCream, 0, 4, 2)
	salmon     = mkRecipe("salmon", domain.IngFattyFish, domain.MethodBaked, domain.SauceCitrus, 0, 3, 3)
	dessert    = mkRecipe("dessert", domain.IngDessert, domain.MethodBaked, domain.SauceSweet, 0, 4, 2)
	bolognese  = mkRecipe("bolognese", domain.IngPasta, domain.MethodStewed, domain.SauceTomato, 0, 4, 4)
	porkNeck   = mkRecipe("pork-neck", domain.IngPork, domain.MethodGrilled, domain.SauceNone, 1, 4, 2)
	khachapuri = mkRecipe("khachapuri", domain.IngCheese, domain.MethodBaked, domain.SauceNone, 0, 5, 2)
	steamedCod = mkRecipe("steamed-cod", domain.IngFish, domain.MethodSteamed, domain.SauceHerb, 0, 2, 2)
	mussels    = mkRecipe("mussels", domain.IngSeafood, domain.MethodSteamed, domain.SauceCitrus, 0, 2, 4)
	sweetPork  = mkRecipe("sweet-sour-pork", domain.IngPork, domain.MethodFried, domain.SauceSweet, 0, 3, 3)
)

func hasCode(res Result, code string) bool {
	for _, r := range res.Reasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

func codes(res Result) string {
	var cs []string
	for _, r := range res.Reasons {
		cs = append(cs, r.Code)
	}
	return strings.Join(cs, ",")
}

// ---------- relative order: better wine must beat worse wine for the same dish ----------

func TestScore_RelativeOrder(t *testing.T) {
	cases := []struct {
		name        string
		recipe      domain.Recipe
		better      domain.Wine
		worse       domain.Wine
		betterCodes []string
		worseCodes  []string
	}{
		{"spicy korean tteokbokki: off-dry riesling beats tannic saperavi", tteokbokki, rieslingOD, saperavi,
			[]string{"spice_fresh", "spice_sweet", "sweet_sauce"}, []string{"spice_tannin"}},
		{"spicy korean tteokbokki: soft pinot beats tannic saperavi", tteokbokki, pinot, saperavi, nil, []string{"spice_tannin"}},
		{"kimchi pork (spicy, soy): sparkling beats krasnostop", kimchiPork, brut, krasnostop,
			[]string{"spice_bubbles"}, []string{"spice_tannin", "umami_tannin"}},
		{"thai curry: riesling beats medium-tannin tsimlyansky", thaiCurry, rieslingOD, tsimla, []string{"spice_sweet"}, []string{"spice_tannin"}},
		{"tandoori: no tannin beats cabernet (tannin + 14.5% abv)", tandoori, rieslingOD, cabernet, nil, []string{"spice_tannin", "spice_alcohol"}},
		{"steak: tannic red beats white", steak, cabernet, chardonnay, []string{"color_ingredient", "tannin_protein", "method_grill"}, nil},
		{"steak: tannic krasnostop beats light pinot", steak, krasnostop, pinot, []string{"tannin_protein"}, []string{"body_mismatch"}},
		{"beef cheeks in red wine: saperavi beats rkatsiteli", beefCheeks, saperavi, rkatsiteli, []string{"method_braise", "red_sauce"}, nil},
		{"fish in cream: full-bodied viognier beats light red", fishCream, viognier, pinot, []string{"cream_body"}, []string{"color_ingredient"}},
		{"fish in cream: chardonnay beats tannic cabernet", fishCream, chardonnay, cabernet, []string{"cream_body"}, []string{"cream_tannin", "tannin_fish"}},
		{"salmon: pinot noir beats cabernet", salmon, pinot, cabernet, []string{"light_red_fatty_fish", "citrus_acid"}, []string{"tannin_fish"}},
		{"salmon: white beats cabernet", salmon, rkatsiteli, cabernet, nil, []string{"tannin_fish"}},
		{"dessert: dessert muscat beats dry white", dessert, muscat, chardonnay, []string{"sweet_sauce"}, []string{"dessert_dry"}},
		{"bolognese: red with acidity beats low-acid white", bolognese, tsimla, viognier, []string{"tomato_red", "acid_match"}, []string{"acid_mismatch"}},
		{"pork neck on charcoal: krasnostop beats riesling", porkNeck, krasnostop, rieslingOD, []string{"tannin_protein", "method_grill"}, nil},
		{"khachapuri: amber kisi beats light pinot", khachapuri, kisiAmber, pinot, nil, []string{"body_mismatch"}},
		{"steamed cod with herbs: fresh white beats heavy red", steamedCod, rkatsiteli, saperavi, []string{"herb_acid"}, []string{"method_light", "tannin_fish"}},
		{"mussels: sparkling beats red", mussels, brut, tsimla, []string{"method_light", "acid_match"}, []string{"tannin_fish"}},
		{"sweet-sour pork: semi-sweet kindzmarauli beats dry tannic cabernet", sweetPork, kindzma, cabernet, []string{"sweet_sauce"}, []string{"sweet_sauce"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := Score(Input{Wine: tc.better, Recipe: tc.recipe})
			w := Score(Input{Wine: tc.worse, Recipe: tc.recipe})
			if b.Score <= w.Score {
				t.Fatalf("want %s (%d: %s) > %s (%d: %s)", tc.better.ID, b.Score, codes(b), tc.worse.ID, w.Score, codes(w))
			}
			for _, c := range tc.betterCodes {
				if !hasCode(b, c) {
					t.Errorf("%s: missing reason %q, got %s", tc.better.ID, c, codes(b))
				}
			}
			for _, c := range tc.worseCodes {
				if !hasCode(w, c) {
					t.Errorf("%s: missing reason %q, got %s", tc.worse.ID, c, codes(w))
				}
			}
		})
	}
}

func TestScore_ClampedAndReasonsSumUp(t *testing.T) {
	for _, r := range []domain.Recipe{tteokbokki, kimchiPork, steak, fishCream, salmon, dessert, bolognese, porkNeck, khachapuri, steamedCod, mussels} {
		for _, w := range []domain.Wine{saperavi, krasnostop, cabernet, pinot, rieslingOD, viognier, muscat, brut, kindzma, kisiAmber} {
			res := Score(Input{Wine: w, Recipe: r, Window: domain.WindowTooEarly})
			if res.Score < 0 || res.Score > 100 {
				t.Fatalf("%s/%s: score %d out of 0..100", w.ID, r.ID, res.Score)
			}
			sum := baseScore
			for _, rs := range res.Reasons {
				if rs.Delta == 0 {
					t.Fatalf("%s/%s: zero-delta reason %q", w.ID, r.ID, rs.Code)
				}
				if rs.RU == "" || rs.EN == "" {
					t.Fatalf("%s/%s: reason %q lacks RU/EN text", w.ID, r.ID, rs.Code)
				}
				sum += rs.Delta
			}
			want := min(max(sum, 0), 100)
			if res.Score != want {
				t.Fatalf("%s/%s: score %d != clamp(sum of deltas)=%d", w.ID, r.ID, res.Score, want)
			}
		}
	}
}

// ---------- taste profile ----------

func TestScore_Profile(t *testing.T) {
	base := Score(Input{Wine: saperavi, Recipe: beefCheeks})

	rejected := &domain.TasteProfile{LikedGrapes: []string{"saperavi"}, RejectedGrapes: []string{"Saperavi"}} // EN alias matches RU grape
	res := Score(Input{Wine: saperavi, Recipe: beefCheeks, Profile: rejected})
	if !hasCode(res, "profile_rejected") || res.Score >= base.Score {
		t.Fatalf("rejected grape must be penalized: base %d, got %d (%s)", base.Score, res.Score, codes(res))
	}
	if hasCode(res, "profile_liked") {
		t.Fatalf("rejected grape must not also get the liked bonus")
	}

	liked := &domain.TasteProfile{LikedGrapes: []string{"tsimlyansky_cherny"}, FruitDark: 5}
	tb := Score(Input{Wine: tsimla, Recipe: bolognese})
	tl := Score(Input{Wine: tsimla, Recipe: bolognese, Profile: liked})
	if !hasCode(tl, "profile_liked") || !hasCode(tl, "profile_dark_fruit") || tl.Score < tb.Score {
		t.Fatalf("liked grape/dark fruit bonus expected: %d -> %d (%s)", tb.Score, tl.Score, codes(tl))
	}

	// Anton: low acidity, moderate tannin, bone dry.
	anton := &domain.TasteProfile{Acidity: 2, Tannin: 2, Sweetness: 1}
	if r := Score(Input{Wine: rieslingOD, Recipe: salmon, Profile: anton}); !hasCode(r, "profile_acid") {
		t.Errorf("high-acid riesling should get profile_acid, got %s", codes(r))
	}
	if r := Score(Input{Wine: cabernet, Recipe: steak, Profile: anton}); !hasCode(r, "profile_tannin") {
		t.Errorf("cabernet should get profile_tannin, got %s", codes(r))
	}
	if r := Score(Input{Wine: kindzma, Recipe: steak, Profile: anton}); !hasCode(r, "profile_sweet") {
		t.Errorf("semi-sweet with dry-only profile should get profile_sweet, got %s", codes(r))
	}
	if r := Score(Input{Wine: muscat, Recipe: dessert, Profile: anton}); hasCode(r, "profile_sweet") {
		t.Errorf("sweet wine with dessert must not be penalized by dry profile")
	}
}

// ---------- drinking window ----------

func TestScore_Window(t *testing.T) {
	cases := []struct {
		win  domain.WindowStatus
		code string
	}{
		{domain.WindowPeak, "window_peak"},
		{domain.WindowLastCall, "window_last_call"},
		{domain.WindowReady, "window_ready"},
		{domain.WindowLate, "window_late"},
		{domain.WindowTooEarly, "window_early"},
	}
	for _, tc := range cases {
		res := Score(Input{Wine: tsimla, Recipe: porkNeck, Window: tc.win})
		if !hasCode(res, tc.code) {
			t.Errorf("window %s: missing %s, got %s", tc.win, tc.code, codes(res))
		}
	}
	none := Score(Input{Wine: tsimla, Recipe: porkNeck, Window: domain.WindowUnknown})
	if strings.Contains(codes(none), "window_") {
		t.Errorf("unknown window must not add reasons, got %s", codes(none))
	}
	ready := Score(Input{Wine: tsimla, Recipe: bolognese, Window: domain.WindowReady})
	early := Score(Input{Wine: tsimla, Recipe: bolognese, Window: domain.WindowTooEarly})
	if early.Score >= ready.Score {
		t.Errorf("too early (%d) must score below ready (%d)", early.Score, ready.Score)
	}
}

// ---------- pairing history ----------

func TestScore_Feedback(t *testing.T) {
	w := chardonnay
	r := steak // a mediocre pair, far from the clamp
	base := Score(Input{Wine: w, Recipe: r})
	fb := func(rating int, recipeID string) []domain.PairingFeedback {
		return []domain.PairingFeedback{{WineID: w.ID, RecipeID: recipeID, Rating: rating}}
	}
	good := Score(Input{Wine: w, Recipe: r, Feedback: fb(5, r.ID)})
	bad := Score(Input{Wine: w, Recipe: r, Feedback: fb(2, r.ID)})
	neutral := Score(Input{Wine: w, Recipe: r, Feedback: fb(3, r.ID)})
	other := Score(Input{Wine: w, Recipe: r, Feedback: fb(5, "another-recipe")})

	if !hasCode(good, "history_good") || good.Score <= base.Score {
		t.Errorf("rating 5 must give a bonus: %d -> %d (%s)", base.Score, good.Score, codes(good))
	}
	if !hasCode(bad, "history_bad") || bad.Score >= base.Score {
		t.Errorf("rating 2 must give a penalty: %d -> %d (%s)", base.Score, bad.Score, codes(bad))
	}
	if neutral.Score != base.Score || other.Score != base.Score {
		t.Errorf("rating 3 or another recipe must not change the score: base %d, neutral %d, other %d", base.Score, neutral.Score, other.Score)
	}
}

// ---------- ranking ----------

func entry(id string, w domain.Wine, qty int, from, until *int) domain.CellarEntry {
	return domain.CellarEntry{Item: domain.CellarItem{ID: id, WineID: w.ID, Quantity: qty, DrinkFrom: from, DrinkUntil: until}, Wine: w}
}

func TestRankWines(t *testing.T) {
	entries := []domain.CellarEntry{
		entry("i-chard", chardonnay, 2, nil, nil),
		entry("i-empty", saperavi, 0, nil, nil),
		entry("i-cab", cabernet, 1, iptr(2020), iptr(2030)),
	}
	got := RankWines(entries, steak, nil, 2026, nil)
	if len(got) != 2 {
		t.Fatalf("empty bottles must be skipped, got %d candidates", len(got))
	}
	if got[0].Entry.Item.ID != "i-cab" || got[0].Result.Score < got[1].Result.Score {
		t.Fatalf("cabernet must be first for steak, got %s", got[0].Entry.Item.ID)
	}
	if got[0].Window != domain.WindowPeak {
		t.Fatalf("window must be computed from item years, got %s", got[0].Window)
	}
	if got[0].Recipe.ID != steak.ID {
		t.Fatalf("candidate must carry the recipe")
	}
}

func TestRankWines_TieBrokenByUrgency(t *testing.T) {
	// Both clamp to 100 with steak; the bottle in its last year must come first.
	entries := []domain.CellarEntry{
		entry("i-ready", cabernet, 1, iptr(2026), iptr(2040)),
		entry("i-last", cabernet, 1, iptr(2020), iptr(2026)),
	}
	got := RankWines(entries, steak, nil, 2026, nil)
	if got[0].Result.Score != got[1].Result.Score {
		t.Fatalf("test precondition: expected tie, got %d vs %d", got[0].Result.Score, got[1].Result.Score)
	}
	if got[0].Entry.Item.ID != "i-last" {
		t.Fatalf("last_call must win a tie over ready, got %s first", got[0].Entry.Item.ID)
	}
}

func TestRankRecipes(t *testing.T) {
	e := entry("i-sap", saperavi, 1, nil, nil)
	got := RankRecipes(e, []domain.Recipe{tteokbokki, salmon, beefCheeks}, nil, nil)
	if len(got) != 3 {
		t.Fatalf("want 3 recipes, got %d", len(got))
	}
	if got[0].Recipe.ID != beefCheeks.ID || got[len(got)-1].Recipe.ID != tteokbokki.ID {
		t.Fatalf("saperavi: want beef cheeks first and spicy tteokbokki last, got %s ... %s", got[0].Recipe.ID, got[len(got)-1].Recipe.ID)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Result.Score < got[i].Result.Score {
			t.Fatalf("recipes not sorted by score")
		}
	}
}

func TestTopReasons(t *testing.T) {
	res := Result{Reasons: []Reason{
		{Code: "a", Delta: 5, RU: "а", EN: "a"},
		{Code: "b", Delta: -20, RU: "б", EN: "b"},
		{Code: "c", Delta: 10, RU: "в", EN: "c"},
	}}
	if got := strings.Join(TopReasons(res, domain.LocaleRU, 2), ","); got != "б,в" {
		t.Errorf("RU top-2 by |delta|: got %q", got)
	}
	if got := strings.Join(TopReasons(res, domain.LocaleEN, 0), ","); got != "b,c,a" {
		t.Errorf("EN all: got %q", got)
	}
	if res.Reasons[0].Code != "a" {
		t.Errorf("TopReasons must not reorder the input")
	}

	real := Score(Input{Wine: cabernet, Recipe: steak})
	ru := TopReasons(real, domain.LocaleRU, 0)
	en := TopReasons(real, domain.LocaleEN, 0)
	if len(ru) != len(en) || len(ru) == 0 {
		t.Fatalf("RU/EN reason count mismatch: %d vs %d", len(ru), len(en))
	}
	if !strings.Contains(strings.Join(ru, " "), "Красное вино") {
		t.Errorf("RU reasons should be capitalized Russian, got %v", ru)
	}
	if !strings.Contains(strings.Join(en, " "), "Red wine") {
		t.Errorf("EN reasons should be English, got %v", en)
	}
}
