package service

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/llm"
)

// Intents recognised by the assistant.
const (
	IntentPairRecipe = "pair_recipe"
	IntentDrinkNow   = "drink_now"
	IntentBuyAgain   = "buy_again"
	IntentGeneral    = "general"
)

// dishKeyword maps a word stem in a message to a partial ad-hoc recipe.
type dishKeyword struct {
	stem  string
	apply func(r *domain.Recipe)
}

var dishKeywords = []dishKeyword{
	{"стейк", func(r *domain.Recipe) {
		r.MainIngredient, r.CookingMethod, r.Richness = domain.IngBeef, domain.MethodGrilled, 5
	}},
	{"steak", func(r *domain.Recipe) {
		r.MainIngredient, r.CookingMethod, r.Richness = domain.IngBeef, domain.MethodGrilled, 5
	}},
	{"говя", func(r *domain.Recipe) { r.MainIngredient = domain.IngBeef }},
	{"beef", func(r *domain.Recipe) { r.MainIngredient = domain.IngBeef }},
	{"баран", func(r *domain.Recipe) { r.MainIngredient = domain.IngLamb }},
	{"lamb", func(r *domain.Recipe) { r.MainIngredient = domain.IngLamb }},
	{"шашлык", func(r *domain.Recipe) {
		r.MainIngredient, r.CookingMethod, r.Richness = domain.IngPork, domain.MethodGrilled, 4
	}},
	{"свин", func(r *domain.Recipe) { r.MainIngredient = domain.IngPork }},
	{"pork", func(r *domain.Recipe) { r.MainIngredient = domain.IngPork }},
	{"кур", func(r *domain.Recipe) { r.MainIngredient = domain.IngPoultry }},
	{"chicken", func(r *domain.Recipe) { r.MainIngredient = domain.IngPoultry }},
	{"утк", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngPoultry, 4 }},
	{"лосос", func(r *domain.Recipe) { r.MainIngredient = domain.IngFattyFish }},
	{"salmon", func(r *domain.Recipe) { r.MainIngredient = domain.IngFattyFish }},
	{"рыб", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngFish, 2 }},
	{"fish", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngFish, 2 }},
	{"кревет", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngSeafood, 2 }},
	{"мидии", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngSeafood, 2 }},
	{"сыр", func(r *domain.Recipe) { r.MainIngredient = domain.IngCheese }},
	{"cheese", func(r *domain.Recipe) { r.MainIngredient = domain.IngCheese }},
	{"паст", func(r *domain.Recipe) { r.MainIngredient = domain.IngPasta }},
	{"pasta", func(r *domain.Recipe) { r.MainIngredient = domain.IngPasta }},
	{"гриб", func(r *domain.Recipe) { r.MainIngredient = domain.IngMushrooms }},
	{"десерт", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngDessert, 4 }},
	{"dessert", func(r *domain.Recipe) { r.MainIngredient, r.Richness = domain.IngDessert, 4 }},
	{"остр", func(r *domain.Recipe) { r.Spiciness = 3 }},
	{"spicy", func(r *domain.Recipe) { r.Spiciness = 3 }},
	{"корей", func(r *domain.Recipe) { r.Spiciness, r.Sauce = 3, domain.SauceSoy }},
	{"korean", func(r *domain.Recipe) { r.Spiciness, r.Sauce = 3, domain.SauceSoy }},
	{"тай", func(r *domain.Recipe) { r.Spiciness, r.Sauce = 3, domain.SauceCurry }},
	{"карри", func(r *domain.Recipe) { r.Spiciness, r.Sauce = 2, domain.SauceCurry }},
	{"curry", func(r *domain.Recipe) { r.Spiciness, r.Sauce = 2, domain.SauceCurry }},
	{"сливоч", func(r *domain.Recipe) { r.Sauce, r.Richness = domain.SauceCream, 4 }},
	{"cream", func(r *domain.Recipe) { r.Sauce, r.Richness = domain.SauceCream, 4 }},
	{"томат", func(r *domain.Recipe) { r.Sauce, r.Acidity = domain.SauceTomato, 4 }},
	{"гриль", func(r *domain.Recipe) { r.CookingMethod = domain.MethodGrilled }},
	{"углях", func(r *domain.Recipe) { r.CookingMethod = domain.MethodGrilled }},
	{"grill", func(r *domain.Recipe) { r.CookingMethod = domain.MethodGrilled }},
	{"туш", func(r *domain.Recipe) { r.CookingMethod, r.Richness = domain.MethodBraised, 4 }},
	{"томл", func(r *domain.Recipe) { r.CookingMethod, r.Richness = domain.MethodBraised, 5 }},
}

var buyAgainWords = []string{"купить снова", "купить ещё", "купить еще", "снова купить", "докупить", "повторить", "buy again", "rebuy", "buy more"}
var drinkNowWords = []string{"пора", "что выпить", "что открыть", "срочно", "пропадёт", "пропадет", "окно", "drink now", "ready", "open tonight", "what to open", "what should i open", "past"}

// DetectIntent classifies a message and, for pairing, finds the best matching household recipe
// (or builds an ad-hoc recipe from dish keywords).
func DetectIntent(msg string, recipes []domain.Recipe) (string, domain.Recipe) {
	low := strings.ToLower(msg)
	for _, w := range buyAgainWords {
		if strings.Contains(low, w) {
			return IntentBuyAgain, domain.Recipe{}
		}
	}
	if r, ok := matchRecipe(low, recipes); ok {
		return IntentPairRecipe, r
	}
	if r, ok := adhocRecipe(low); ok {
		return IntentPairRecipe, r
	}
	for _, w := range drinkNowWords {
		if strings.Contains(low, w) {
			return IntentDrinkNow, domain.Recipe{}
		}
	}
	return IntentGeneral, domain.Recipe{}
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func stem(w string) string {
	r := []rune(w)
	if len(r) > 5 {
		r = r[:5]
	}
	return string(r)
}

// matchRecipe picks the recipe whose title shares the most significant word stems with the message.
func matchRecipe(low string, recipes []domain.Recipe) (domain.Recipe, bool) {
	msgStems := map[string]bool{}
	for _, w := range words(low) {
		if len([]rune(w)) >= 4 {
			msgStems[stem(w)] = true
		}
	}
	best, bestN := domain.Recipe{}, 0
	for _, r := range recipes {
		n := 0
		for _, w := range words(r.Title) {
			if len([]rune(w)) >= 4 && msgStems[stem(w)] {
				n++
			}
		}
		if n > bestN {
			best, bestN = r, n
		}
	}
	return best, bestN > 0
}

func adhocRecipe(low string) (domain.Recipe, bool) {
	r := domain.Recipe{Title: "", MainIngredient: "", CookingMethod: domain.MethodOther, Sauce: domain.SauceNone, Richness: 3, Acidity: 2}
	matched := false
	var hit []string
	for _, w := range words(low) {
		for _, k := range dishKeywords {
			if strings.HasPrefix(w, k.stem) {
				k.apply(&r)
				matched = true
				hit = append(hit, w)
				break
			}
		}
	}
	if !matched {
		return r, false
	}
	if r.MainIngredient == "" {
		r.MainIngredient = domain.IngVegetables
	}
	r.Title = strings.Join(hit, " ")
	return r, true
}

// BuildContext renders profile, cellar, recipes, memory and candidates into compact text,
// trimmed to maxTokens. Only bottles with quantity > 0 are included; candidates go first.
func BuildContext(l domain.Locale, p domain.TasteProfile, entries []domain.CellarEntry, recipes []domain.Recipe,
	facts []domain.MemoryFact, f *llm.Facts, year int, maxTokens int) string {
	var b strings.Builder
	h := func(ru, en string) { b.WriteString("## " + pick(l, ru, en) + "\n") }

	h("Кандидаты (движок сочетаний)", "Candidates (pairing engine)")
	if len(f.Candidates) == 0 {
		b.WriteString("-\n")
	}
	for _, c := range f.Candidates {
		fmt.Fprintf(&b, "- [item:%s] %s · score %d · %s · %s\n", c.ItemID, c.Title, c.Score, c.Window, strings.Join(c.Reasons, "; "))
	}
	if len(f.BuyAgain) > 0 {
		h("Купить снова", "Buy again")
		for _, w := range f.BuyAgain {
			b.WriteString("- " + w + "\n")
		}
	}

	h("Профиль вкуса", "Taste profile")
	fmt.Fprintf(&b, "body %d, acidity %d, tannin %d, sweetness %d, dark fruit %d, red fruit %d, oak %d, spice %d/3\n",
		p.Body, p.Acidity, p.Tannin, p.Sweetness, p.FruitDark, p.FruitRed, p.Oak, p.SpiceTolerance)
	if len(p.LikedGrapes) > 0 {
		b.WriteString("liked: " + strings.Join(p.LikedGrapes, ", ") + "\n")
	}
	if len(p.RejectedGrapes) > 0 {
		b.WriteString("rejected: " + strings.Join(p.RejectedGrapes, ", ") + "\n")
	}
	if p.Notes != "" {
		b.WriteString(p.Notes + "\n")
	}

	if len(facts) > 0 {
		h("Память", "Memory")
		for _, fct := range facts {
			b.WriteString("- " + fct.Fact + "\n")
		}
	}

	// Cellar: candidates first, then by urgency.
	candidate := map[string]bool{}
	for _, c := range f.Candidates {
		candidate[c.ItemID] = true
	}
	stock := make([]domain.CellarEntry, 0, len(entries))
	for _, e := range entries {
		if e.Item.Quantity > 0 {
			stock = append(stock, e)
		}
	}
	urg := func(e domain.CellarEntry) int {
		u := domain.DrinkingWindow(e.Item.DrinkFrom, e.Item.DrinkUntil, year).Urgency()
		if candidate[e.Item.ID] {
			u += 10
		}
		return u
	}
	for i := 1; i < len(stock); i++ {
		for j := i; j > 0 && urg(stock[j]) > urg(stock[j-1]); j-- {
			stock[j], stock[j-1] = stock[j-1], stock[j]
		}
	}
	h("Погреб (в наличии)", "Cellar (in stock)")
	for _, e := range stock {
		win := domain.DrinkingWindow(e.Item.DrinkFrom, e.Item.DrinkUntil, year)
		fmt.Fprintf(&b, "- [item:%s] %s · %s · %s · %s · x%d · %s\n", e.Item.ID, WineTitle(e.Wine), e.Wine.Color,
			strings.Join(e.Wine.Grapes, "/"), e.Wine.Region, e.Item.Quantity, win)
	}
	if len(recipes) > 0 {
		h("Рецепты семьи", "Household recipes")
		for _, r := range recipes {
			fmt.Fprintf(&b, "- %s (%s, spice %d)\n", r.Title, r.MainIngredient, r.Spiciness)
		}
	}
	return llm.TrimToTokens(b.String(), maxTokens)
}
