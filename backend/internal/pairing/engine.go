// Package pairing is the deterministic wine ↔ recipe pairing engine. No LLM involved.
//
// Score starts at 50 and each rule adds or subtracts points; the result is clamped to 0..100.
// Every rule that fires adds a human-readable reason (RU and EN). See docs/architecture/pairing.md.
package pairing

import (
	"fmt"
	"sort"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// Reason explains one score change.
type Reason struct {
	Code  string `json:"code"`
	Delta int    `json:"delta"`
	RU    string `json:"-"`
	EN    string `json:"-"`
}

// Text returns the reason in the requested locale.
func (r Reason) Text(l domain.Locale) string {
	if l == domain.LocaleEN {
		return r.EN
	}
	return r.RU
}

// Result is a scored pairing.
type Result struct {
	Score   int      `json:"score"`
	Reasons []Reason `json:"-"`
}

// Input for scoring a single wine with a single recipe.
type Input struct {
	Wine     domain.Wine
	Recipe   domain.Recipe
	Profile  *domain.TasteProfile // optional
	Window   domain.WindowStatus  // optional; "" = ignore
	Feedback []domain.PairingFeedback
}

const baseScore = 50

type scorer struct {
	res Result
}

func (s *scorer) add(code string, delta int, ru, en string) {
	if delta == 0 {
		return
	}
	s.res.Score += delta
	s.res.Reasons = append(s.res.Reasons, Reason{Code: code, Delta: delta, RU: ru, EN: en})
}

// ingredientAffinity: base affinity of wine color to the main ingredient.
var ingredientAffinity = map[domain.MainIngredient]map[domain.Color]int{
	domain.IngBeef:       {domain.ColorRed: 18, domain.ColorFortified: 0, domain.ColorWhite: -15, domain.ColorSparkling: -10, domain.ColorRose: -5, domain.ColorDessert: -20},
	domain.IngLamb:       {domain.ColorRed: 18, domain.ColorWhite: -15, domain.ColorSparkling: -10, domain.ColorRose: -3, domain.ColorDessert: -20},
	domain.IngGame:       {domain.ColorRed: 20, domain.ColorWhite: -15, domain.ColorSparkling: -10, domain.ColorDessert: -20},
	domain.IngPork:       {domain.ColorRed: 8, domain.ColorRose: 8, domain.ColorWhite: 5, domain.ColorOrange: 6, domain.ColorDessert: -15},
	domain.IngPoultry:    {domain.ColorWhite: 10, domain.ColorRose: 8, domain.ColorRed: 3, domain.ColorOrange: 6, domain.ColorSparkling: 5, domain.ColorDessert: -15},
	domain.IngFish:       {domain.ColorWhite: 18, domain.ColorSparkling: 12, domain.ColorRose: 6, domain.ColorOrange: 3, domain.ColorRed: -15, domain.ColorDessert: -20},
	domain.IngFattyFish:  {domain.ColorWhite: 14, domain.ColorRose: 10, domain.ColorSparkling: 8, domain.ColorRed: -2, domain.ColorDessert: -20},
	domain.IngSeafood:    {domain.ColorWhite: 18, domain.ColorSparkling: 15, domain.ColorRose: 6, domain.ColorRed: -18, domain.ColorDessert: -20},
	domain.IngCheese:     {domain.ColorRed: 6, domain.ColorWhite: 6, domain.ColorDessert: 6, domain.ColorFortified: 6, domain.ColorOrange: 6},
	domain.IngVegetables: {domain.ColorWhite: 10, domain.ColorRose: 10, domain.ColorOrange: 10, domain.ColorSparkling: 5, domain.ColorRed: -3},
	domain.IngMushrooms:  {domain.ColorRed: 10, domain.ColorWhite: 5, domain.ColorOrange: 8},
	domain.IngPasta:      {domain.ColorRed: 8, domain.ColorWhite: 5, domain.ColorRose: 5},
	domain.IngDessert:    {domain.ColorDessert: 25, domain.ColorFortified: 15, domain.ColorSparkling: 5, domain.ColorRed: -15, domain.ColorWhite: -10, domain.ColorRose: -10},
}

var colorRU = map[domain.Color]string{
	domain.ColorRed: "красное", domain.ColorWhite: "белое", domain.ColorRose: "розовое", domain.ColorOrange: "оранжевое",
	domain.ColorSparkling: "игристое", domain.ColorDessert: "десертное", domain.ColorFortified: "креплёное",
}

var ingredientRU = map[domain.MainIngredient]string{
	domain.IngBeef: "говядина", domain.IngLamb: "баранина", domain.IngGame: "дичь", domain.IngPork: "свинина",
	domain.IngPoultry: "птица", domain.IngFish: "рыба", domain.IngFattyFish: "жирная рыба", domain.IngSeafood: "морепродукты",
	domain.IngCheese: "сыр", domain.IngVegetables: "овощи", domain.IngMushrooms: "грибы", domain.IngPasta: "паста", domain.IngDessert: "десерт",
}

func isRedMeat(i domain.MainIngredient) bool {
	return i == domain.IngBeef || i == domain.IngLamb || i == domain.IngGame
}

func isFish(i domain.MainIngredient) bool {
	return i == domain.IngFish || i == domain.IngSeafood || i == domain.IngFattyFish
}

func sweetLevel(s domain.Sweetness) int {
	switch s {
	case domain.SweetnessOffDry:
		return 2
	case domain.SweetnessMedium:
		return 3
	case domain.SweetnessSweet:
		return 4
	default:
		return 1
	}
}

// Score evaluates one wine against one recipe.
func Score(in Input) Result {
	s := &scorer{res: Result{Score: baseScore}}
	w, r := in.Wine, in.Recipe
	st := w.ResolveStructure()
	sweet := sweetLevel(w.Sweetness)
	if w.Color == domain.ColorDessert && w.Sweetness == "" {
		sweet = 4
	}

	// 1. Base affinity: color vs main ingredient.
	if aff, ok := ingredientAffinity[r.MainIngredient][w.Color]; ok {
		if aff > 0 {
			s.add("color_ingredient", aff, fmt.Sprintf("%s вино — классика к блюду, где главное — %s", capital(colorRU[w.Color]), ingredientRU[r.MainIngredient]),
				fmt.Sprintf("%s wine is a classic match for %s", capital(string(w.Color)), r.MainIngredient))
		} else {
			s.add("color_ingredient", aff, fmt.Sprintf("%s вино обычно не дружит с блюдом, где главное — %s", capital(colorRU[w.Color]), ingredientRU[r.MainIngredient]),
				fmt.Sprintf("%s wine rarely suits %s", capital(string(w.Color)), r.MainIngredient))
		}
	}

	// 2. Heat: tannin and alcohol amplify burning; sweetness and freshness calm it.
	if r.Spiciness >= 2 {
		k := 1.0
		if r.Spiciness == 3 {
			k = 1.5
		}
		switch {
		case st.Tannin >= 4:
			s.add("spice_tannin", -int(25*k), "Танины усиливают жжение острого блюда", "Tannin amplifies the heat of spicy food")
		case st.Tannin == 3:
			s.add("spice_tannin", -int(10*k), "Средние танины немного усилят остроту", "Medium tannin slightly amplifies heat")
		case st.Tannin <= 1 && st.Acidity >= 3:
			s.add("spice_fresh", int(10*k), "Свежее вино без танинов освежает после острого", "A fresh, tannin-free wine refreshes after heat")
		}
		if w.ABV != nil && *w.ABV >= 14.5 {
			s.add("spice_alcohol", -int(8*k), "Высокий алкоголь разжигает остроту", "High alcohol fuels the burn")
		}
		if sweet >= 2 && sweet <= 3 {
			s.add("spice_sweet", int(14*k), "Лёгкая сладость гасит остроту", "A touch of sweetness tames the heat")
		}
		if w.Color == domain.ColorSparkling {
			s.add("spice_bubbles", 5, "Пузырьки освежают после острого", "Bubbles refresh after spice")
		}
	}

	// 3. Weight: wine body should match dish richness.
	diff := st.Body - r.Richness
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff == 0:
		s.add("body_match", 10, "Тело вина совпадает с насыщенностью блюда", "Wine body matches the richness of the dish")
	case diff == 1:
		s.add("body_match", 4, "Тело вина близко к насыщенности блюда", "Wine body is close to the dish richness")
	case diff >= 3:
		if st.Body > r.Richness {
			s.add("body_mismatch", -12, "Вино слишком мощное — перебьёт блюдо", "The wine is too powerful and will overwhelm the dish")
		} else {
			s.add("body_mismatch", -12, "Вино слишком лёгкое — потеряется на фоне блюда", "The wine is too light and will get lost")
		}
	case diff == 2:
		s.add("body_mismatch", -4, "Тело вина и насыщенность блюда немного расходятся", "Wine body and dish richness are a bit apart")
	}

	// 4. Tannin vs protein/fat, tannin vs fish.
	if isRedMeat(r.MainIngredient) && st.Tannin >= 4 && r.Spiciness < 2 {
		s.add("tannin_protein", 14, "Танины и белок/жир мяса смягчают друг друга", "Tannin and meat protein/fat soften each other")
	}
	if r.MainIngredient == domain.IngPork && r.CookingMethod == domain.MethodGrilled && st.Tannin >= 3 && r.Spiciness < 2 {
		s.add("tannin_protein", 6, "Жирная свинина на углях выдерживает танины", "Fatty grilled pork handles tannin")
	}
	if isFish(r.MainIngredient) && st.Tannin >= 3 {
		s.add("tannin_fish", -14, "Танины с рыбой дают металлический привкус", "Tannin with fish gives a metallic taste")
	}
	if r.MainIngredient == domain.IngFattyFish && w.Color == domain.ColorRed && st.Tannin <= 2 && st.Acidity >= 4 {
		s.add("light_red_fatty_fish", 12, "Лёгкое кислотное красное (как Пино Нуар) отлично с лососем", "A light, high-acid red (like Pinot Noir) works with salmon")
	}

	// 5. Acidity: sour dishes need at least as much acidity; cream needs body or acidity.
	if r.Acidity >= 4 {
		if st.Acidity >= r.Acidity-1 {
			s.add("acid_match", 10, "Кислотность вина выдерживает кислый соус", "Wine acidity stands up to the tangy sauce")
		} else {
			s.add("acid_mismatch", -10, "Соус кислее вина — вино покажется плоским", "The sauce is more acidic than the wine — it will taste flat")
		}
	}
	switch r.Sauce {
	case domain.SauceCream:
		if (w.Color == domain.ColorWhite || w.Color == domain.ColorOrange) && st.Body >= 4 {
			s.add("cream_body", 14, "Плотное белое выдерживает сливочный соус", "A full-bodied white stands up to cream")
		}
		if st.Acidity >= 4 && w.Color != domain.ColorRed {
			s.add("cream_acid", 5, "Кислотность освежает после сливок", "Acidity cuts through cream")
		}
		if st.Tannin >= 4 {
			s.add("cream_tannin", -10, "Танины спорят со сливками", "Tannin clashes with cream")
		}
	case domain.SauceTomato:
		if w.Color == domain.ColorRed && st.Acidity >= 3 {
			s.add("tomato_red", 8, "Красное с хорошей кислотностью — к томатному соусу", "A red with good acidity suits tomato sauce")
		}
	case domain.SauceSoy:
		if st.Tannin >= 4 {
			s.add("umami_tannin", -6, "Умами соевого соуса делает танины жёстче", "Soy umami makes tannin harsher")
		}
	case domain.SauceSweet:
		if sweet >= 2 {
			s.add("sweet_sauce", 10, "Сладость вина под стать сладкому соусу", "Wine sweetness matches the sweet sauce")
		} else if st.Tannin >= 4 {
			s.add("sweet_sauce", -6, "Сладкий соус сделает сухое танинное вино горьким", "A sweet sauce makes a dry tannic wine bitter")
		}
	case domain.SauceRed:
		if w.Color == domain.ColorRed {
			s.add("red_sauce", 8, "Соус на красном вине — просит красное", "A red-wine sauce calls for red")
		}
	case domain.SauceHerb:
		if st.Acidity >= 4 && w.Color != domain.ColorRed {
			s.add("herb_acid", 6, "Свежее вино к травяному соусу", "A fresh wine for a herb sauce")
		}
	case domain.SauceCitrus:
		if st.Acidity >= 4 {
			s.add("citrus_acid", 6, "Высокая кислотность к цитрусовому соусу", "High acidity for a citrus sauce")
		}
	}

	// 6. Cooking method.
	switch r.CookingMethod {
	case domain.MethodGrilled, domain.MethodSmoked:
		if w.Color == domain.ColorRed && st.Body >= 4 && st.DarkFruit {
			s.add("method_grill", 10, "Угли и дым любят плотное красное с тёмными фруктами", "Char and smoke love a full red with dark fruit")
		}
	case domain.MethodBraised, domain.MethodStewed:
		if st.Body >= 4 {
			s.add("method_braise", 8, "Долгое тушение — нужно вино с телом", "Slow braising needs a wine with body")
		}
	case domain.MethodSteamed, domain.MethodRaw:
		if st.Body <= 2 {
			s.add("method_light", 8, "Деликатная готовка — лёгкое вино", "Delicate cooking — a light wine")
		} else if st.Body >= 4 {
			s.add("method_light", -8, "Вино тяжеловато для деликатной готовки", "The wine is heavy for delicate cooking")
		}
	}

	// 7. Dessert: the wine must be at least as sweet as the dish.
	if r.MainIngredient == domain.IngDessert && sweet < 4 {
		s.add("dessert_dry", -15, "К десерту сухое вино покажется кислым", "A dry wine tastes sour with dessert")
	}

	// 8. Taste profile.
	if p := in.Profile; p != nil {
		applyProfile(s, w, st, r, p)
	}

	// 9. Drinking window.
	switch in.Window {
	case domain.WindowPeak:
		s.add("window_peak", 10, "Вино сейчас на пике", "The wine is at its peak now")
	case domain.WindowLastCall:
		s.add("window_last_call", 12, "Последний год окна питья — самое время открыть", "Last year of its window — time to open it")
	case domain.WindowReady:
		s.add("window_ready", 6, "Вино готово к питью", "The wine is ready to drink")
	case domain.WindowLate:
		s.add("window_late", 4, "Окно питья прошло — лучше не тянуть", "Past its window — don't wait longer")
	case domain.WindowTooEarly:
		s.add("window_early", -15, "Ещё рано: вину стоит полежать", "Too early: the wine needs more time")
	}

	// 10. History: how this exact pair (or this wine with similar dishes) was rated before.
	applyFeedback(s, w, r, in.Feedback)

	if s.res.Score < 0 {
		s.res.Score = 0
	}
	if s.res.Score > 100 {
		s.res.Score = 100
	}
	return s.res
}

func applyProfile(s *scorer, w domain.Wine, st domain.Structure, r domain.Recipe, p *domain.TasteProfile) {
	for _, g := range w.Grapes {
		if p.Rejects(g) {
			s.add("profile_rejected", -30, fmt.Sprintf("Вы отметили, что не любите %s", g), fmt.Sprintf("You said you don't like %s", g))
			return
		}
	}
	for _, g := range w.Grapes {
		if p.Likes(g) {
			s.add("profile_liked", 8, fmt.Sprintf("%s — среди любимых сортов", g), fmt.Sprintf("%s is one of your favourite grapes", g))
			break
		}
	}
	if p.FruitDark >= 4 && st.DarkFruit {
		s.add("profile_dark_fruit", 6, "Тёмные фрукты — как вы любите", "Dark fruit, the way you like it")
	}
	if p.Acidity > 0 && st.Acidity-p.Acidity >= 2 {
		s.add("profile_acid", -8, "Кислотнее, чем вы обычно любите", "More acidic than you usually like")
	}
	if p.Tannin > 0 && st.Tannin-p.Tannin >= 2 {
		s.add("profile_tannin", -6, "Терпче, чем вы обычно любите", "More tannic than you usually like")
	}
	if p.Sweetness > 0 && p.Sweetness <= 1 && sweetLevel(w.Sweetness) >= 3 && r.MainIngredient != domain.IngDessert {
		s.add("profile_sweet", -8, "Слаще, чем вы обычно пьёте", "Sweeter than you usually drink")
	}
}

func applyFeedback(s *scorer, w domain.Wine, r domain.Recipe, fb []domain.PairingFeedback) {
	var exactSum, exactN int
	for _, f := range fb {
		if f.WineID == w.ID && f.RecipeID == r.ID {
			exactSum += f.Rating
			exactN++
		}
	}
	if exactN == 0 {
		return
	}
	avg := float64(exactSum) / float64(exactN)
	delta := int((avg - 3) * 8)
	if delta > 0 {
		s.add("history_good", delta, fmt.Sprintf("Вы уже пробовали это сочетание: %.1f из 5", avg), fmt.Sprintf("You tried this pairing before: %.1f of 5", avg))
	} else if delta < 0 {
		s.add("history_bad", delta, fmt.Sprintf("Прошлый раз сочетание не понравилось: %.1f из 5", avg), fmt.Sprintf("Last time this pairing disappointed: %.1f of 5", avg))
	}
}

func capital(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	switch {
	case r[0] >= 'а' && r[0] <= 'я':
		r[0] -= 'а' - 'А'
	case r[0] >= 'a' && r[0] <= 'z':
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

// Candidate is a cellar bottle scored for a recipe (or a recipe scored for a bottle).
type Candidate struct {
	Entry  domain.CellarEntry
	Recipe domain.Recipe
	Result Result
	Window domain.WindowStatus
}

// RankWines scores all in-stock cellar entries for a recipe, best first.
func RankWines(entries []domain.CellarEntry, r domain.Recipe, p *domain.TasteProfile, year int, fb []domain.PairingFeedback) []Candidate {
	out := make([]Candidate, 0, len(entries))
	for _, e := range entries {
		if e.Item.Quantity <= 0 {
			continue
		}
		win := domain.DrinkingWindow(e.Item.DrinkFrom, e.Item.DrinkUntil, year)
		res := Score(Input{Wine: e.Wine, Recipe: r, Profile: p, Window: win, Feedback: fb})
		out = append(out, Candidate{Entry: e, Recipe: r, Result: res, Window: win})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Result.Score != out[j].Result.Score {
			return out[i].Result.Score > out[j].Result.Score
		}
		return out[i].Window.Urgency() > out[j].Window.Urgency()
	})
	return out
}

// RankRecipes scores recipes for one bottle, best first. The window is ignored: the bottle is already chosen.
func RankRecipes(e domain.CellarEntry, recipes []domain.Recipe, p *domain.TasteProfile, fb []domain.PairingFeedback) []Candidate {
	out := make([]Candidate, 0, len(recipes))
	for _, r := range recipes {
		res := Score(Input{Wine: e.Wine, Recipe: r, Profile: p, Feedback: fb})
		out = append(out, Candidate{Entry: e, Recipe: r, Result: res})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Result.Score > out[j].Result.Score })
	return out
}

// TopReasons returns up to n reasons sorted by absolute impact, localized.
func TopReasons(res Result, l domain.Locale, n int) []string {
	rs := append([]Reason(nil), res.Reasons...)
	sort.SliceStable(rs, func(i, j int) bool { return abs(rs[i].Delta) > abs(rs[j].Delta) })
	if n > 0 && len(rs) > n {
		rs = rs[:n]
	}
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Text(l))
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
