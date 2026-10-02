package service

import (
	"context"
	"fmt"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/pairing"
)

// ---------- taste survey ----------

// QuestionView is a localized survey question.
type QuestionView struct {
	ID      string            `json:"id"`
	Text    string            `json:"text"`
	Type    domain.AnswerType `json:"type"`
	Axis    domain.Axis       `json:"axis"`
	Options []OptionView      `json:"options,omitempty"`
}

// OptionView is a localized option.
type OptionView struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SurveyBatch is the next batch of unanswered questions.
type SurveyBatch struct {
	Questions []QuestionView `json:"questions"`
	Answered  int            `json:"answered"`
	Total     int            `json:"total"`
	Done      bool           `json:"done"`
}

// NextSurveyBatch returns up to size (5..7, default 6) unanswered questions.
func (s *Service) NextSurveyBatch(ctx context.Context, a Actor, size int) (SurveyBatch, error) {
	if size < 5 || size > 7 {
		size = 6
	}
	answered, err := s.store.AnsweredQuestions(ctx, a.User.ID)
	if err != nil {
		return SurveyBatch{}, err
	}
	qs := domain.NextBatch(domain.SurveyBank, answered, size)
	out := SurveyBatch{Questions: make([]QuestionView, 0, len(qs)), Answered: len(answered), Total: len(domain.SurveyBank), Done: len(qs) == 0}
	for _, q := range qs {
		out.Questions = append(out.Questions, localizeQuestion(q, a.User.Locale))
	}
	return out, nil
}

func localizeQuestion(q domain.Question, l domain.Locale) QuestionView {
	v := QuestionView{ID: q.ID, Text: q.TextRU, Type: q.Type, Axis: q.Axis}
	if l == domain.LocaleEN {
		v.Text = q.TextEN
	}
	for _, o := range q.Options {
		label := o.LabelRU
		if l == domain.LocaleEN {
			label = o.LabelEN
		}
		v.Options = append(v.Options, OptionView{Value: o.Value, Label: label})
	}
	return v
}

// AnswerInput is one answer in POST /v1/taste/survey/answers.
type AnswerInput struct {
	QuestionID string   `json:"question_id"`
	Values     []string `json:"values"`
}

// AnswerSurvey validates all answers, applies them to the profile and stores everything atomically.
func (s *Service) AnswerSurvey(ctx context.Context, a Actor, answers []AnswerInput) (domain.TasteProfile, error) {
	if len(answers) == 0 {
		return domain.TasteProfile{}, domain.Invalid("answers", "at least one answer is required")
	}
	p, err := s.store.GetProfile(ctx, a.User.ID)
	if err != nil {
		return p, err
	}
	now := s.clock.Now()
	stored := make([]domain.SurveyAnswer, 0, len(answers))
	for _, in := range answers {
		q, ok := domain.FindQuestion(domain.SurveyBank, in.QuestionID)
		if !ok {
			return p, domain.Invalid("question_id", "unknown question %q", in.QuestionID)
		}
		ans := domain.SurveyAnswer{QuestionID: q.ID, Values: in.Values, AnsweredAt: now}
		if err := domain.ApplyAnswer(&p, q, ans); err != nil {
			return p, err
		}
		stored = append(stored, ans)
	}
	p.UserID, p.UpdatedAt = a.User.ID, now
	if err := s.store.SaveProfileWithAnswers(ctx, p, stored); err != nil {
		return p, err
	}
	return p, nil
}

// Profile returns the taste profile of the actor.
func (s *Service) Profile(ctx context.Context, a Actor) (domain.TasteProfile, error) {
	return s.store.GetProfile(ctx, a.User.ID)
}

// ---------- recipes ----------

// RecipePatch carries optional recipe fields.
type RecipePatch struct {
	Title          Opt[string]                `json:"title"`
	Cuisine        Opt[string]                `json:"cuisine"`
	MainIngredient Opt[domain.MainIngredient] `json:"main_ingredient"`
	CookingMethod  Opt[domain.CookingMethod]  `json:"cooking_method"`
	Sauce          Opt[domain.Sauce]          `json:"sauce"`
	Spiciness      Opt[int]                   `json:"spiciness"`
	Richness       Opt[int]                   `json:"richness"`
	Acidity        Opt[int]                   `json:"acidity"`
	TimeMinutes    Opt[int]                   `json:"time_minutes"`
	Notes          Opt[string]                `json:"notes"`
	Tags           Opt[[]string]              `json:"tags"`
}

// CreateRecipe stores a recipe in the actor's cellar.
func (s *Service) CreateRecipe(ctx context.Context, a Actor, r domain.Recipe) (domain.Recipe, error) {
	now := s.clock.Now()
	r.ID, r.CellarID, r.CreatedAt, r.UpdatedAt = s.newID(), a.CellarID, now, now
	r.Normalize()
	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, s.store.CreateRecipe(ctx, r)
}

// GetRecipe returns a recipe.
func (s *Service) GetRecipe(ctx context.Context, a Actor, id string) (domain.Recipe, error) {
	return s.store.GetRecipe(ctx, a.CellarID, id)
}

// ListRecipes lists recipes.
func (s *Service) ListRecipes(ctx context.Context, a Actor) ([]domain.Recipe, error) {
	return s.store.ListRecipes(ctx, a.CellarID)
}

// UpdateRecipe applies a patch.
func (s *Service) UpdateRecipe(ctx context.Context, a Actor, id string, p RecipePatch) (domain.Recipe, error) {
	r, err := s.store.GetRecipe(ctx, a.CellarID, id)
	if err != nil {
		return r, err
	}
	p.Title.Apply(&r.Title)
	p.Cuisine.Apply(&r.Cuisine)
	p.MainIngredient.Apply(&r.MainIngredient)
	p.CookingMethod.Apply(&r.CookingMethod)
	p.Sauce.Apply(&r.Sauce)
	p.Spiciness.Apply(&r.Spiciness)
	p.Richness.Apply(&r.Richness)
	p.Acidity.Apply(&r.Acidity)
	p.TimeMinutes.Apply(&r.TimeMinutes)
	p.Notes.Apply(&r.Notes)
	p.Tags.Apply(&r.Tags)
	r.Normalize()
	if err := r.Validate(); err != nil {
		return r, err
	}
	r.UpdatedAt = s.clock.Now()
	return r, s.store.UpdateRecipe(ctx, r)
}

// DeleteRecipe deletes a recipe.
func (s *Service) DeleteRecipe(ctx context.Context, a Actor, id string) error {
	return s.store.DeleteRecipe(ctx, a.CellarID, id)
}

// ---------- pairing ----------

// WineSuggestion is a ranked bottle for a recipe.
type WineSuggestion struct {
	Item    ItemView `json:"bottle"`
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

// RecipeSuggestion is a ranked recipe for a bottle.
type RecipeSuggestion struct {
	Recipe  domain.Recipe `json:"recipe"`
	Score   int           `json:"score"`
	Reasons []string      `json:"reasons"`
}

// WinesForRecipe ranks in-stock bottles for a recipe ("что открыть из погреба к рецепту").
func (s *Service) WinesForRecipe(ctx context.Context, a Actor, recipeID string, limit int) ([]WineSuggestion, error) {
	r, err := s.store.GetRecipe(ctx, a.CellarID, recipeID)
	if err != nil {
		return nil, err
	}
	return s.winesFor(ctx, a, r, limit)
}

func (s *Service) winesFor(ctx context.Context, a Actor, r domain.Recipe, limit int) ([]WineSuggestion, error) {
	entries, err := s.store.ListItems(ctx, a.CellarID, ItemFilter{}.CellarFilter)
	if err != nil {
		return nil, err
	}
	p, err := s.store.GetProfile(ctx, a.User.ID)
	if err != nil {
		return nil, err
	}
	fb, err := s.store.ListPairingFeedback(ctx, a.CellarID)
	if err != nil {
		return nil, err
	}
	cands := pairing.RankWines(entries, r, &p, s.clock.Now().Year(), fb)
	if limit <= 0 {
		limit = 5
	}
	if len(cands) > limit {
		cands = cands[:limit]
	}
	out := make([]WineSuggestion, 0, len(cands))
	for _, c := range cands {
		out = append(out, WineSuggestion{Item: s.view(c.Entry, a.User.Locale), Score: c.Result.Score, Reasons: pairing.TopReasons(c.Result, a.User.Locale, 4)})
	}
	return out, nil
}

// RecipesForItem ranks household recipes for a bottle ("что приготовить к бутылке").
func (s *Service) RecipesForItem(ctx context.Context, a Actor, itemID string, limit int) ([]RecipeSuggestion, error) {
	e, err := s.store.GetItem(ctx, a.CellarID, itemID)
	if err != nil {
		return nil, err
	}
	recipes, err := s.store.ListRecipes(ctx, a.CellarID)
	if err != nil {
		return nil, err
	}
	p, err := s.store.GetProfile(ctx, a.User.ID)
	if err != nil {
		return nil, err
	}
	fb, err := s.store.ListPairingFeedback(ctx, a.CellarID)
	if err != nil {
		return nil, err
	}
	cands := pairing.RankRecipes(e, recipes, &p, fb)
	if limit <= 0 {
		limit = 5
	}
	if len(cands) > limit {
		cands = cands[:limit]
	}
	out := make([]RecipeSuggestion, 0, len(cands))
	for _, c := range cands {
		out = append(out, RecipeSuggestion{Recipe: c.Recipe, Score: c.Result.Score, Reasons: pairing.TopReasons(c.Result, a.User.Locale, 4)})
	}
	return out, nil
}

// PairingFeedbackInput is the body of POST /v1/recipes/{id}/feedback.
type PairingFeedbackInput struct {
	WineID string `json:"wine_id"`
	Rating int    `json:"rating"`
	Note   string `json:"note"`
}

// AddPairingFeedback stores how well a wine went with a recipe.
func (s *Service) AddPairingFeedback(ctx context.Context, a Actor, recipeID string, in PairingFeedbackInput) (domain.PairingFeedback, error) {
	if in.Rating < 1 || in.Rating > 5 {
		return domain.PairingFeedback{}, domain.Invalid("rating", "must be 1..5")
	}
	if _, err := s.store.GetRecipe(ctx, a.CellarID, recipeID); err != nil {
		return domain.PairingFeedback{}, err
	}
	if _, err := s.store.GetWine(ctx, a.CellarID, in.WineID); err != nil {
		if err == domain.ErrNotFound {
			return domain.PairingFeedback{}, domain.Invalid("wine_id", "wine not found")
		}
		return domain.PairingFeedback{}, err
	}
	f := domain.PairingFeedback{ID: s.newID(), CellarID: a.CellarID, RecipeID: recipeID, WineID: in.WineID, UserID: a.User.ID,
		Rating: in.Rating, Note: in.Note, CreatedAt: s.clock.Now()}
	return f, s.store.AddPairingFeedback(ctx, f)
}

// BuyAgain lists wines rated >= 4 by the actor, best first ("вернуться и купить снова").
func (s *Service) BuyAgain(ctx context.Context, a Actor) ([]BuyAgainItem, error) {
	events, err := s.store.ListConsumption(ctx, a.CellarID, a.User.ID, 1000)
	if err != nil {
		return nil, err
	}
	type agg struct{ sum, n int }
	byWine := map[string]*agg{}
	var order []string
	for _, e := range events {
		if e.Rating == 0 {
			continue
		}
		g, ok := byWine[e.WineID]
		if !ok {
			g = &agg{}
			byWine[e.WineID] = g
			order = append(order, e.WineID)
		}
		g.sum += e.Rating
		g.n++
	}
	out := []BuyAgainItem{}
	for _, id := range order {
		g := byWine[id]
		avg := float64(g.sum) / float64(g.n)
		if avg < 4 {
			continue
		}
		w, err := s.store.GetWine(ctx, a.CellarID, id)
		if err != nil {
			continue
		}
		inStock := 0
		items, _ := s.store.ListItems(ctx, a.CellarID, ItemFilter{}.CellarFilter)
		for _, it := range items {
			if it.Wine.ID == id {
				inStock += it.Item.Quantity
			}
		}
		out = append(out, BuyAgainItem{Wine: w, AvgRating: avg, Times: g.n, InStock: inStock})
	}
	sortBuyAgain(out)
	return out, nil
}

// BuyAgainItem is an entry of the buy-again list.
type BuyAgainItem struct {
	Wine      domain.Wine `json:"wine"`
	AvgRating float64     `json:"avg_rating"`
	Times     int         `json:"times_rated"`
	InStock   int         `json:"in_stock"`
}

func sortBuyAgain(xs []BuyAgainItem) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && (xs[j].AvgRating > xs[j-1].AvgRating || (xs[j].AvgRating == xs[j-1].AvgRating && xs[j].InStock < xs[j-1].InStock)); j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// WineTitle formats "Name Producer 2019" for messages.
func WineTitle(w domain.Wine) string {
	t := w.Name
	if w.Producer != "" && w.Producer != w.Name {
		t += ", " + w.Producer
	}
	return fmt.Sprintf("%s %s", t, w.VintageLabel())
}
