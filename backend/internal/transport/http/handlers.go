package http

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

type actorHandler = func(http.ResponseWriter, *http.Request, service.Actor)

var _ actorHandler = (*api)(nil).me

func (a *api) me(w http.ResponseWriter, r *http.Request, act service.Actor) {
	writeJSON(w, http.StatusOK, map[string]any{"user": act.User, "cellar_id": act.CellarID, "role": act.Role})
}

// ---------- wines ----------

func (a *api) listWines(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.ListWines(r.Context(), act)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

// wineInput is the create body (subset of domain.Wine that clients may set).
type wineInput struct {
	Name        string           `json:"name"`
	Producer    string           `json:"producer"`
	Country     string           `json:"country"`
	Region      string           `json:"region"`
	Appellation string           `json:"appellation"`
	Grapes      []string         `json:"grapes"`
	Vintage     *int             `json:"vintage"`
	Color       domain.Color     `json:"color"`
	Sweetness   domain.Sweetness `json:"sweetness"`
	ABV         *float64         `json:"abv"`
	Body        int              `json:"body"`
	Acidity     int              `json:"acidity"`
	Tannin      int              `json:"tannin"`
	StyleNotes  string           `json:"style_notes"`
}

func (in wineInput) wine() domain.Wine {
	return domain.Wine{Name: in.Name, Producer: in.Producer, Country: in.Country, Region: in.Region, Appellation: in.Appellation,
		Grapes: in.Grapes, Vintage: in.Vintage, Color: in.Color, Sweetness: in.Sweetness, ABV: in.ABV, Body: in.Body,
		Acidity: in.Acidity, Tannin: in.Tannin, StyleNotes: in.StyleNotes, Source: domain.SourceManual}
}

func (a *api) createWine(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var in wineInput
	if err := decode(r, &in); err != nil {
		a.fail(w, err)
		return
	}
	wine, err := a.svc.CreateWine(r.Context(), act, in.wine())
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wine)
}

func (a *api) getWine(w http.ResponseWriter, r *http.Request, act service.Actor) {
	wine, err := a.svc.GetWine(r.Context(), act, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wine)
}

func (a *api) patchWine(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var p service.WinePatch
	if err := decode(r, &p); err != nil {
		a.fail(w, err)
		return
	}
	wine, err := a.svc.UpdateWine(r.Context(), act, r.PathValue("id"), p)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wine)
}

func (a *api) deleteWine(w http.ResponseWriter, r *http.Request, act service.Actor) {
	if err := a.svc.DeleteWine(r.Context(), act, r.PathValue("id")); err != nil {
		a.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- cellar ----------

func (a *api) listItems(w http.ResponseWriter, r *http.Request, act service.Actor) {
	q := r.URL.Query()
	f := service.ItemFilter{CellarFilter: storage.CellarFilter{
		Color: domain.Color(q.Get("color")), Country: q.Get("country"), Grape: q.Get("grape"),
		Location: q.Get("location"), Query: q.Get("q"), IncludeEmpty: q.Get("include_empty") == "true",
	}}
	if f.Color != "" && !f.Color.Valid() {
		a.fail(w, domain.Invalid("color", "unknown color"))
		return
	}
	switch q.Get("ready_to_drink") {
	case "", "false":
	case "true":
		f.ReadyToDrink = true
	default:
		a.fail(w, domain.Invalid("ready_to_drink", "must be true or false"))
		return
	}
	xs, err := a.svc.ListItems(r.Context(), act, f)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

// itemInput creates bottles; either wine_id or an inline "wine" object is required.
type itemInput struct {
	WineID             string     `json:"wine_id"`
	Wine               *wineInput `json:"wine"`
	Quantity           *int       `json:"quantity"`
	StorageLocation    string     `json:"storage_location"`
	PurchasePriceMinor *int64     `json:"purchase_price_minor"`
	Currency           string     `json:"currency"`
	PurchasePlace      string     `json:"purchase_place"`
	DrinkFrom          *int       `json:"drink_from"`
	DrinkUntil         *int       `json:"drink_until"`
	Notes              string     `json:"notes"`
}

func (a *api) createItem(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var in itemInput
	if err := decode(r, &in); err != nil {
		a.fail(w, err)
		return
	}
	if in.Wine != nil && in.WineID != "" {
		a.fail(w, domain.Invalid("wine", "send either wine_id or wine, not both"))
		return
	}
	qty := 1
	if in.Quantity != nil {
		qty = *in.Quantity
	}
	it := domain.CellarItem{WineID: in.WineID, Quantity: qty, StorageLocation: in.StorageLocation, PurchasePriceMinor: in.PurchasePriceMinor,
		Currency: in.Currency, PurchasePlace: in.PurchasePlace, DrinkFrom: in.DrinkFrom, DrinkUntil: in.DrinkUntil, Notes: in.Notes}
	if in.Wine != nil {
		// validate the bottle part before creating the wine so a bad request leaves no orphan card
		probe := it
		probe.WineID = "pending"
		probe.Normalize()
		if err := probe.Validate(); err != nil {
			a.fail(w, err)
			return
		}
		wine, err := a.svc.CreateWine(r.Context(), act, in.Wine.wine())
		if err != nil {
			a.fail(w, err)
			return
		}
		it.WineID = wine.ID
	}
	v, err := a.svc.AddItem(r.Context(), act, it)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *api) getItem(w http.ResponseWriter, r *http.Request, act service.Actor) {
	v, err := a.svc.GetItem(r.Context(), act, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *api) patchItem(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var p service.ItemPatch
	if err := decode(r, &p); err != nil {
		a.fail(w, err)
		return
	}
	v, err := a.svc.UpdateItem(r.Context(), act, r.PathValue("id"), p)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *api) drink(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var req service.DrinkRequest
	if r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			a.fail(w, err)
			return
		}
	}
	res, err := a.svc.Drink(r.Context(), act, r.PathValue("id"), req)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *api) recipesForItem(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.RecipesForItem(r.Context(), act, r.PathValue("id"), queryInt(r, "limit", 5))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

func (a *api) summary(w http.ResponseWriter, r *http.Request, act service.Actor) {
	s, err := a.svc.Summary(r.Context(), act, queryInt(r, "top", 5))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *api) history(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.History(r.Context(), act, queryInt(r, "limit", 50))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

func (a *api) buyAgain(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.BuyAgain(r.Context(), act)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

func (a *api) reminders(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.Reminders(r.Context(), act)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

// ---------- taste ----------

func (a *api) surveyNext(w http.ResponseWriter, r *http.Request, act service.Actor) {
	b, err := a.svc.NextSurveyBatch(r.Context(), act, queryInt(r, "size", 6))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *api) surveyAnswers(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var body struct {
		Answers []service.AnswerInput `json:"answers"`
	}
	if err := decode(r, &body); err != nil {
		a.fail(w, err)
		return
	}
	p, err := a.svc.AnswerSurvey(r.Context(), act, body.Answers)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *api) profile(w http.ResponseWriter, r *http.Request, act service.Actor) {
	p, err := a.svc.Profile(r.Context(), act)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ---------- recipes ----------

type recipeInput struct {
	Title          string                `json:"title"`
	Cuisine        string                `json:"cuisine"`
	MainIngredient domain.MainIngredient `json:"main_ingredient"`
	CookingMethod  domain.CookingMethod  `json:"cooking_method"`
	Sauce          domain.Sauce          `json:"sauce"`
	Spiciness      int                   `json:"spiciness"`
	Richness       int                   `json:"richness"`
	Acidity        int                   `json:"acidity"`
	TimeMinutes    int                   `json:"time_minutes"`
	Notes          string                `json:"notes"`
	Tags           []string              `json:"tags"`
}

func (a *api) listRecipes(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.ListRecipes(r.Context(), act)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

func (a *api) createRecipe(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var in recipeInput
	if err := decode(r, &in); err != nil {
		a.fail(w, err)
		return
	}
	rec, err := a.svc.CreateRecipe(r.Context(), act, domain.Recipe{Title: in.Title, Cuisine: in.Cuisine, MainIngredient: in.MainIngredient,
		CookingMethod: in.CookingMethod, Sauce: in.Sauce, Spiciness: in.Spiciness, Richness: in.Richness, Acidity: in.Acidity,
		TimeMinutes: in.TimeMinutes, Notes: in.Notes, Tags: in.Tags})
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rec)
}

func (a *api) getRecipe(w http.ResponseWriter, r *http.Request, act service.Actor) {
	rec, err := a.svc.GetRecipe(r.Context(), act, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (a *api) patchRecipe(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var p service.RecipePatch
	if err := decode(r, &p); err != nil {
		a.fail(w, err)
		return
	}
	rec, err := a.svc.UpdateRecipe(r.Context(), act, r.PathValue("id"), p)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (a *api) deleteRecipe(w http.ResponseWriter, r *http.Request, act service.Actor) {
	if err := a.svc.DeleteRecipe(r.Context(), act, r.PathValue("id")); err != nil {
		a.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) winesForRecipe(w http.ResponseWriter, r *http.Request, act service.Actor) {
	xs, err := a.svc.WinesForRecipe(r.Context(), act, r.PathValue("id"), queryInt(r, "limit", 5))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newList(xs))
}

func (a *api) pairingFeedback(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var in service.PairingFeedbackInput
	if err := decode(r, &in); err != nil {
		a.fail(w, err)
		return
	}
	f, err := a.svc.AddPairingFeedback(r.Context(), act, r.PathValue("id"), in)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// ---------- chat ----------

func (a *api) chat(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var req service.ChatRequest
	if err := decode(r, &req); err != nil {
		a.fail(w, err)
		return
	}
	res, err := a.ast.Chat(r.Context(), act, req)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------- csv ----------

func (a *api) exportCSV(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var buf bytes.Buffer
	if err := a.svc.ExportCSV(r.Context(), act, &buf); err != nil {
		a.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="terroir-cellar.csv"`)
	_, _ = w.Write(buf.Bytes())
}

func (a *api) importCSV(w http.ResponseWriter, r *http.Request, act service.Actor) {
	var src io.Reader = io.LimitReader(r.Body, maxCSVBytes)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxCSVBytes); err != nil {
			a.fail(w, domain.Invalid("file", "bad multipart form"))
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			a.fail(w, domain.Invalid("file", "form field 'file' is required"))
			return
		}
		defer f.Close()
		src = f
	}
	res, err := a.svc.ImportCSV(r.Context(), act, src)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// importMarkdown imports a Markdown table (body: text/markdown or text/plain).
func (a *api) importMarkdown(w http.ResponseWriter, r *http.Request, act service.Actor) {
	res, err := a.svc.ImportMarkdown(r.Context(), act, io.LimitReader(r.Body, maxCSVBytes))
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
