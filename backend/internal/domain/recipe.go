package domain

import (
	"strings"
	"time"
)

// CookingMethod of a recipe.
type CookingMethod string

const (
	MethodRaw     CookingMethod = "raw"
	MethodSteamed CookingMethod = "steamed"
	MethodBoiled  CookingMethod = "boiled"
	MethodBaked   CookingMethod = "baked"
	MethodFried   CookingMethod = "fried"
	MethodGrilled CookingMethod = "grilled" // гриль / угли
	MethodBraised CookingMethod = "braised" // тушение
	MethodStewed  CookingMethod = "stewed"
	MethodSmoked  CookingMethod = "smoked"
	MethodOther   CookingMethod = "other"
)

// MainIngredient is the dominant protein/base.
type MainIngredient string

const (
	IngBeef       MainIngredient = "beef"
	IngLamb       MainIngredient = "lamb"
	IngPork       MainIngredient = "pork"
	IngPoultry    MainIngredient = "poultry"
	IngFish       MainIngredient = "fish"
	IngFattyFish  MainIngredient = "fatty_fish" // лосось, скумбрия
	IngSeafood    MainIngredient = "seafood"
	IngCheese     MainIngredient = "cheese"
	IngVegetables MainIngredient = "vegetables"
	IngMushrooms  MainIngredient = "mushrooms"
	IngPasta      MainIngredient = "pasta"
	IngGame       MainIngredient = "game"
	IngDessert    MainIngredient = "dessert"
)

// Sauce is the dominant sauce/flavour base.
type Sauce string

const (
	SauceNone   Sauce = "none"
	SauceCream  Sauce = "cream"
	SauceTomato Sauce = "tomato"
	SauceSoy    Sauce = "soy" // соевый / umami
	SauceCitrus Sauce = "citrus"
	SauceSweet  Sauce = "sweet"
	SauceHerb   Sauce = "herb"
	SauceRed    Sauce = "red_wine"
	SauceCurry  Sauce = "curry"
)

// Recipe is a household recipe.
// Spiciness 0..3, Richness 1..5 (fat), Acidity 1..5 (sauce sourness).
type Recipe struct {
	ID             string         `json:"id"`
	CellarID       string         `json:"cellar_id"`
	Title          string         `json:"title"`
	Cuisine        string         `json:"cuisine"`
	MainIngredient MainIngredient `json:"main_ingredient"`
	CookingMethod  CookingMethod  `json:"cooking_method"`
	Sauce          Sauce          `json:"sauce"`
	Spiciness      int            `json:"spiciness"`
	Richness       int            `json:"richness"`
	Acidity        int            `json:"acidity"`
	TimeMinutes    int            `json:"time_minutes"`
	Notes          string         `json:"notes"`
	Tags           []string       `json:"tags"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

var knownIngredients = map[MainIngredient]bool{
	IngBeef: true, IngLamb: true, IngPork: true, IngPoultry: true, IngFish: true, IngFattyFish: true, IngSeafood: true,
	IngCheese: true, IngVegetables: true, IngMushrooms: true, IngPasta: true, IngGame: true, IngDessert: true,
}

var knownMethods = map[CookingMethod]bool{
	MethodRaw: true, MethodSteamed: true, MethodBoiled: true, MethodBaked: true, MethodFried: true,
	MethodGrilled: true, MethodBraised: true, MethodStewed: true, MethodSmoked: true, MethodOther: true,
}

var knownSauces = map[Sauce]bool{
	SauceNone: true, SauceCream: true, SauceTomato: true, SauceSoy: true, SauceCitrus: true,
	SauceSweet: true, SauceHerb: true, SauceRed: true, SauceCurry: true,
}

// Normalize trims strings and fills defaults.
func (r *Recipe) Normalize() {
	r.Title = strings.TrimSpace(r.Title)
	r.Cuisine = strings.TrimSpace(r.Cuisine)
	r.Notes = strings.TrimSpace(r.Notes)
	if r.CookingMethod == "" {
		r.CookingMethod = MethodOther
	}
	if r.Sauce == "" {
		r.Sauce = SauceNone
	}
	if r.Richness == 0 {
		r.Richness = 3
	}
	if r.Acidity == 0 {
		r.Acidity = 2
	}
	tags := make([]string, 0, len(r.Tags))
	for _, t := range r.Tags {
		if t = strings.TrimSpace(strings.ToLower(t)); t != "" {
			tags = append(tags, t)
		}
	}
	r.Tags = tags
}

// Validate checks recipe invariants.
func (r *Recipe) Validate() error {
	if r.Title == "" {
		return Invalid("title", "is required")
	}
	if !knownIngredients[r.MainIngredient] {
		return Invalid("main_ingredient", "unknown value %q", r.MainIngredient)
	}
	if !knownMethods[r.CookingMethod] {
		return Invalid("cooking_method", "unknown value %q", r.CookingMethod)
	}
	if !knownSauces[r.Sauce] {
		return Invalid("sauce", "unknown value %q", r.Sauce)
	}
	if r.Spiciness < 0 || r.Spiciness > 3 {
		return Invalid("spiciness", "must be 0..3")
	}
	if r.Richness < 1 || r.Richness > 5 {
		return Invalid("richness", "must be 1..5")
	}
	if r.Acidity < 1 || r.Acidity > 5 {
		return Invalid("acidity", "must be 1..5")
	}
	if r.TimeMinutes < 0 || r.TimeMinutes > 24*60*3 {
		return Invalid("time_minutes", "must be 0..4320")
	}
	return nil
}

// PairingFeedback records how well a wine went with a recipe (learned from history).
type PairingFeedback struct {
	ID        string    `json:"id"`
	CellarID  string    `json:"cellar_id"`
	RecipeID  string    `json:"recipe_id"`
	WineID    string    `json:"wine_id"`
	UserID    string    `json:"user_id"`
	Rating    int       `json:"rating"` // 1..5
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryFact is a short fact the assistant remembers about a user.
type MemoryFact struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Fact      string    `json:"fact"`
	Source    string    `json:"source"` // chat | drink | survey | manual
	CreatedAt time.Time `json:"created_at"`
}

// ChatMessage in a conversation.
type ChatMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	Role           string    `json:"role"` // user | assistant
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// LLMUsage is one accounted LLM call.
type LLMUsage struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CostMicros       int64     `json:"cost_micros"` // in budget currency * 1e6
	CreatedAt        time.Time `json:"created_at"`
}
