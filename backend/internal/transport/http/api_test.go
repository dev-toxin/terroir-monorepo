package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/app"
	"github.com/anko-soft-lab/terroir/backend/internal/config"
	"github.com/anko-soft-lab/terroir/backend/internal/seed"
)

type client struct {
	t     *testing.T
	srv   *httptest.Server
	token string
}

func newClient(t *testing.T, mutate ...func(*config.Config)) *client {
	t.Helper()
	cfg, err := config.LoadFrom(func(k string) string {
		return map[string]string{"STORAGE": "memory", "AUTO_SEED": "true", "TERROIR_TODAY": "2026-10-02"}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mutate {
		m(&cfg)
	}
	ctx := context.Background()
	st, err := app.OpenStore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(ctx, cfg, st, log)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.Handler)
	t.Cleanup(func() { srv.Close(); _ = st.Close() })
	return &client{t: t, srv: srv, token: seed.TokenAnton}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	status, raw := c.raw(method, path, body, "")
	var out map[string]any
	if len(raw) > 0 && raw[0] == '{' {
		if err := json.Unmarshal(raw, &out); err != nil {
			c.t.Fatalf("%s %s: bad json %q", method, path, raw)
		}
	}
	return status, out
}

func (c *client) raw(method, path string, body any, contentType string) (int, []byte) {
	c.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	case []byte:
		r = bytes.NewReader(b)
	default:
		buf, _ := json.Marshal(b)
		r = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, r)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

func expect(t *testing.T, got, want int, body map[string]any) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d, want %d: %v", got, want, body)
	}
}

func errCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	s, _ := e["code"].(string)
	return s
}

func items(body map[string]any) []any {
	xs, _ := body["items"].([]any)
	return xs
}

func TestHealthAndAuth(t *testing.T) {
	c := newClient(t)
	st, b := c.do("GET", "/health", nil)
	expect(t, st, 200, b)
	if b["storage"] != "memory" || b["llm_provider"] != "mock" {
		t.Fatalf("health: %v", b)
	}
	st, b = c.do("GET", "/ready", nil)
	expect(t, st, 200, b)

	c.token = ""
	st, b = c.do("GET", "/v1/wines", nil)
	expect(t, st, 401, b)
	if errCode(b) != "unauthorized" {
		t.Fatalf("error format: %v", b)
	}
	c.token = "wrong-token"
	st, _ = c.do("GET", "/v1/wines", nil)
	expect(t, st, 401, nil)
	c.token = seed.TokenElena
	st, b = c.do("GET", "/v1/me", nil)
	expect(t, st, 200, b)
	st, b = c.do("GET", "/v1/nope", nil)
	expect(t, st, 404, b)
	if errCode(b) != "not_found" {
		t.Fatal("404 format")
	}
}

func TestWinesCRUD(t *testing.T) {
	c := newClient(t)
	st, b := c.do("POST", "/v1/wines", map[string]any{"name": "Кокур", "color": "white", "grapes": []string{"Кокур"}, "vintage": 2022})
	expect(t, st, 201, b)
	id := b["id"].(string)

	st, b = c.do("GET", "/v1/wines/"+id, nil)
	expect(t, st, 200, b)
	st, b = c.do("PATCH", "/v1/wines/"+id, map[string]any{"vintage": nil, "producer": "Долина"})
	expect(t, st, 200, b)
	if b["vintage"] != nil || b["producer"] != "Долина" || b["name"] != "Кокур" {
		t.Fatalf("patch: %v", b)
	}
	st, b = c.do("GET", "/v1/wines", nil)
	expect(t, st, 200, b)
	if len(items(b)) < 20 {
		t.Fatalf("seeded wines: %d", len(items(b)))
	}
	st, _ = c.do("DELETE", "/v1/wines/"+id, nil)
	expect(t, st, 204, nil)
	st, b = c.do("GET", "/v1/wines/"+id, nil)
	expect(t, st, 404, b)

	// errors
	st, b = c.do("POST", "/v1/wines", map[string]any{"name": "", "color": "red"})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/wines", map[string]any{"name": "X", "color": "blue"})
	expect(t, st, 400, b)
	if errCode(b) != "validation_error" {
		t.Fatal(b)
	}
	st, b = c.do("POST", "/v1/wines", map[string]any{"name": "X", "color": "red", "typo_field": 1})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/wines", "{not json")
	expect(t, st, 400, b)
	st, b = c.do("PATCH", "/v1/wines/w-saperavi-kakheti", map[string]any{"color": "blue"})
	expect(t, st, 400, b)
	st, b = c.do("DELETE", "/v1/wines/w-saperavi-kakheti", nil) // has bottles
	expect(t, st, 409, b)
	st, b = c.do("PATCH", "/v1/wines/nope", map[string]any{"name": "x"})
	expect(t, st, 404, b)
}

func TestCellarItemsAndFilters(t *testing.T) {
	c := newClient(t)
	st, b := c.do("GET", "/v1/cellar/items", nil)
	expect(t, st, 200, b)
	all := len(items(b))
	if all < 18 {
		t.Fatalf("in stock: %d", all)
	}
	for _, q := range []string{"?color=white", "?country=Грузия", "?grape=saperavi", "?ready_to_drink=true", "?location=холодильник", "?q=квеври"} {
		st, b = c.do("GET", "/v1/cellar/items"+q, nil)
		expect(t, st, 200, b)
		n := len(items(b))
		if n == 0 || n >= all {
			t.Fatalf("%s: %d of %d", q, n, all)
		}
	}
	st, b = c.do("GET", "/v1/cellar/items?ready_to_drink=true", nil)
	for _, x := range items(b) {
		w := x.(map[string]any)["window"]
		if w == "too_early" || w == "unknown" {
			t.Fatalf("ready_to_drink returned %v", w)
		}
	}
	st, b = c.do("GET", "/v1/cellar/items?include_empty=true", nil)
	if len(items(b)) != all+1 {
		t.Fatalf("include_empty: %d", len(items(b)))
	}
	st, b = c.do("GET", "/v1/cellar/items?color=blue", nil)
	expect(t, st, 400, b)
	st, b = c.do("GET", "/v1/cellar/items?ready_to_drink=maybe", nil)
	expect(t, st, 400, b)

	// create with existing wine and with inline wine
	st, b = c.do("POST", "/v1/cellar/items", map[string]any{"wine_id": "w-kokur", "quantity": 3, "storage_location": "Балкон", "drink_from": 2025, "drink_until": 2027})
	expect(t, st, 201, b)
	id := b["item"].(map[string]any)["id"].(string)
	st, b = c.do("POST", "/v1/cellar/items", map[string]any{"wine": map[string]any{"name": "Рислинг", "color": "white"}, "quantity": 1})
	expect(t, st, 201, b)
	st, b = c.do("GET", "/v1/cellar/items/"+id, nil)
	expect(t, st, 200, b)
	st, b = c.do("PATCH", "/v1/cellar/items/"+id, map[string]any{"quantity": 5, "drink_until": nil, "notes": "подарок"})
	expect(t, st, 200, b)
	it := b["item"].(map[string]any)
	if it["quantity"].(float64) != 5 || it["drink_until"] != nil || it["notes"] != "подарок" {
		t.Fatalf("patch item: %v", it)
	}
	// errors
	st, b = c.do("POST", "/v1/cellar/items", map[string]any{"wine_id": "nope"})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/cellar/items", map[string]any{"wine_id": "w-kokur", "quantity": -1})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/cellar/items", map[string]any{"wine": map[string]any{"name": "X", "color": "red"}, "wine_id": "w-kokur"})
	expect(t, st, 400, b)
	st, b = c.do("PATCH", "/v1/cellar/items/"+id, map[string]any{"drink_from": 2030, "drink_until": 2020})
	expect(t, st, 400, b)
	st, b = c.do("GET", "/v1/cellar/items/nope", nil)
	expect(t, st, 404, b)
}

func TestDrinkEndpoint(t *testing.T) {
	c := newClient(t)
	st, b := c.do("POST", "/v1/cellar/items/i-cabernet-kuban/drink", map[string]any{"rating": 5, "note": "к стейку", "paired_recipe_id": "r-steak"})
	expect(t, st, 200, b)
	if b["item"].(map[string]any)["quantity"].(float64) != 0 {
		t.Fatal("quantity")
	}
	st, b = c.do("POST", "/v1/cellar/items/i-cabernet-kuban/drink", nil)
	expect(t, st, 409, b)
	if errCode(b) != "out_of_stock" {
		t.Fatal(b)
	}
	st, b = c.do("POST", "/v1/cellar/items/i-tsimla/drink", map[string]any{"rating": 7})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/cellar/items/i-tsimla/drink", map[string]any{"paired_recipe_id": "nope"})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/cellar/items/nope/drink", map[string]any{})
	expect(t, st, 404, b)
	st, b = c.do("GET", "/v1/cellar/history", nil)
	expect(t, st, 200, b)
	if len(items(b)) < 5 {
		t.Fatalf("history: %d", len(items(b)))
	}
}

// End-to-end: add wine → add to cellar → drink → summary reflects it.
func TestEndToEndAddDrinkSummary(t *testing.T) {
	c := newClient(t)
	_, before := c.do("GET", "/v1/cellar/summary", nil)
	total := before["total_bottles"].(float64)
	red := before["by_color"].(map[string]any)["red"].(float64)

	_, w := c.do("POST", "/v1/wines", map[string]any{"name": "Красностоп Тест", "color": "red", "grapes": []string{"Красностоп"}, "vintage": 2020})
	st, it := c.do("POST", "/v1/cellar/items", map[string]any{"wine_id": w["id"], "quantity": 2, "drink_from": 2025, "drink_until": 2026})
	expect(t, st, 201, it)
	itemID := it["item"].(map[string]any)["id"].(string)
	if it["window"] != "last_call" {
		t.Fatalf("window: %v", it["window"])
	}
	_, mid := c.do("GET", "/v1/cellar/summary", nil)
	if mid["total_bottles"].(float64) != total+2 || mid["by_color"].(map[string]any)["red"].(float64) != red+2 {
		t.Fatalf("after add: %v", mid)
	}
	st, _ = c.do("POST", "/v1/cellar/items/"+itemID+"/drink", map[string]any{"rating": 4})
	expect(t, st, 200, nil)
	st, after := c.do("GET", "/v1/cellar/summary?top=50", nil)
	expect(t, st, 200, after)
	if after["total_bottles"].(float64) != total+1 {
		t.Fatalf("after drink: %v", after["total_bottles"])
	}
	if after["drunk_last_30_days"].(float64) != mid["drunk_last_30_days"].(float64)+1 {
		t.Fatal("recent drinks not updated")
	}
	found := false
	for _, x := range after["open_soon"].([]any) {
		if x.(map[string]any)["item"].(map[string]any)["id"] == itemID {
			found = true
		}
	}
	if !found {
		t.Fatal("last-call bottle must be in open_soon")
	}
}

func TestTasteSurvey(t *testing.T) {
	c := newClient(t)
	st, b := c.do("GET", "/v1/taste/survey/next", nil)
	expect(t, st, 200, b)
	qs := b["questions"].([]any)
	if len(qs) < 5 || len(qs) > 7 {
		t.Fatalf("batch size %d", len(qs))
	}
	first := qs[0].(map[string]any)
	if !strings.Contains(first["text"].(string), "вино") {
		t.Fatalf("RU text expected: %v", first["text"])
	}
	_, en := c.do("GET", "/v1/taste/survey/next?lang=en", nil)
	if !strings.Contains(en["questions"].([]any)[0].(map[string]any)["text"].(string), "wine") {
		t.Fatal("EN text expected")
	}
	st, b = c.do("POST", "/v1/taste/survey/answers", map[string]any{"answers": []map[string]any{
		{"question_id": "q_body", "values": []string{"4"}},
		{"question_id": "q_acidity", "values": []string{"2"}},
		{"question_id": "q_spice", "values": []string{"3"}},
	}})
	expect(t, st, 200, b)
	if b["body"].(float64) != 4 || b["spice_tolerance"].(float64) != 3 {
		t.Fatalf("profile: %v", b)
	}
	_, next := c.do("GET", "/v1/taste/survey/next", nil)
	for _, q := range next["questions"].([]any) {
		if q.(map[string]any)["id"] == "q_body" {
			t.Fatal("answered question served again")
		}
	}
	st, b = c.do("GET", "/v1/taste/profile", nil)
	expect(t, st, 200, b)
	st, b = c.do("POST", "/v1/taste/survey/answers", map[string]any{"answers": []map[string]any{{"question_id": "nope", "values": []string{"1"}}}})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/taste/survey/answers", map[string]any{"answers": []map[string]any{}})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/taste/survey/answers", map[string]any{"answers": []map[string]any{{"question_id": "q_body", "values": []string{"9"}}}})
	expect(t, st, 400, b)
}

func TestRecipesAndPairing(t *testing.T) {
	c := newClient(t)
	st, b := c.do("POST", "/v1/recipes", map[string]any{"title": "Утка с вишней", "main_ingredient": "poultry", "cooking_method": "baked", "sauce": "sweet", "richness": 4})
	expect(t, st, 201, b)
	id := b["id"].(string)
	st, b = c.do("PATCH", "/v1/recipes/"+id, map[string]any{"spiciness": 1, "tags": []string{"праздник"}})
	expect(t, st, 200, b)
	st, b = c.do("GET", "/v1/recipes/"+id, nil)
	expect(t, st, 200, b)
	st, b = c.do("GET", "/v1/recipes", nil)
	expect(t, st, 200, b)
	if len(items(b)) < 11 {
		t.Fatal("recipes")
	}
	st, _ = c.do("DELETE", "/v1/recipes/"+id, nil)
	expect(t, st, 204, nil)
	st, b = c.do("DELETE", "/v1/recipes/"+id, nil)
	expect(t, st, 404, b)
	st, b = c.do("POST", "/v1/recipes", map[string]any{"title": "X", "main_ingredient": "tofu"})
	expect(t, st, 400, b)
	st, b = c.do("PATCH", "/v1/recipes/r-steak", map[string]any{"spiciness": 9})
	expect(t, st, 400, b)

	// what to open for the steak: a tannic red, with reasons
	st, b = c.do("GET", "/v1/recipes/r-steak/wines?limit=3", nil)
	expect(t, st, 200, b)
	xs := items(b)
	if len(xs) != 3 {
		t.Fatalf("suggestions %d", len(xs))
	}
	top := xs[0].(map[string]any)
	if top["bottle"].(map[string]any)["wine"].(map[string]any)["color"] != "red" || len(top["reasons"].([]any)) == 0 {
		t.Fatalf("steak top: %v", top)
	}
	// spicy Korean: top must not be a tannic red
	_, b = c.do("GET", "/v1/recipes/r-tteokbokki/wines?limit=1", nil)
	spicyTop := items(b)[0].(map[string]any)["bottle"].(map[string]any)["wine"].(map[string]any)
	if spicyTop["tannin"].(float64) >= 4 {
		t.Fatalf("spicy top is tannic: %v", spicyTop["name"])
	}
	st, b = c.do("GET", "/v1/recipes/nope/wines", nil)
	expect(t, st, 404, b)

	// what to cook for a bottle
	st, b = c.do("GET", "/v1/cellar/items/i-krasnostop-don/recipes?limit=3", nil)
	expect(t, st, 200, b)
	if len(items(b)) != 3 {
		t.Fatal("recipes for bottle")
	}
	first := items(b)[0].(map[string]any)["recipe"].(map[string]any)["main_ingredient"]
	if first != "beef" && first != "pork" && first != "lamb" {
		t.Fatalf("krasnostop → %v", first)
	}
	st, b = c.do("GET", "/v1/cellar/items/nope/recipes", nil)
	expect(t, st, 404, b)

	st, b = c.do("POST", "/v1/recipes/r-salmon/feedback", map[string]any{"wine_id": "w-pinot-crimea", "rating": 5})
	expect(t, st, 201, b)
	st, b = c.do("POST", "/v1/recipes/r-salmon/feedback", map[string]any{"wine_id": "w-pinot-crimea", "rating": 0})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/recipes/r-salmon/feedback", map[string]any{"wine_id": "nope", "rating": 3})
	expect(t, st, 400, b)
}

func TestBuyAgainAndReminders(t *testing.T) {
	c := newClient(t)
	st, b := c.do("GET", "/v1/cellar/buy-again", nil)
	expect(t, st, 200, b)
	if len(items(b)) == 0 {
		t.Fatal("buy-again empty")
	}
	st, b = c.do("GET", "/v1/reminders", nil)
	expect(t, st, 200, b)
	if len(items(b)) == 0 {
		t.Fatal("reminders empty")
	}
}

func TestChat(t *testing.T) {
	c := newClient(t)
	st, b := c.do("POST", "/v1/chat", map[string]any{"message": "Что открыть к свиной шее на углях?"})
	expect(t, st, 200, b)
	bottles := b["bottles"].([]any)
	if b["intent"] != "pair_recipe" || len(bottles) == 0 || b["provider"] != "mock" {
		t.Fatalf("chat: %v", b)
	}
	// every referenced bottle must exist in stock
	for _, x := range bottles {
		id := x.(map[string]any)["item_id"].(string)
		st, item := c.do("GET", "/v1/cellar/items/"+id, nil)
		expect(t, st, 200, item)
		if item["item"].(map[string]any)["quantity"].(float64) <= 0 {
			t.Fatalf("referenced empty bottle %s", id)
		}
	}
	conv := b["conversation_id"].(string)
	st, b = c.do("POST", "/v1/chat", map[string]any{"message": "А что пора выпить? Я не люблю кислые вина", "conversation_id": conv})
	expect(t, st, 200, b)
	if b["intent"] != "drink_now" || b["conversation_id"] != conv {
		t.Fatalf("chat 2: %v", b)
	}
	st, b = c.do("POST", "/v1/chat", map[string]any{"message": ""})
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/chat", map[string]any{"message": strings.Repeat("я", 2001)})
	expect(t, st, 400, b)
}

func TestChatBudgetExceeded(t *testing.T) {
	c := newClient(t, func(cfg *config.Config) { cfg.LLM.MonthlyTokens = 10 })
	st, b := c.do("POST", "/v1/chat", map[string]any{"message": "Что открыть к стейку?"})
	expect(t, st, 429, b)
	if errCode(b) != "llm_budget_exceeded" {
		t.Fatal(b)
	}
}

func TestCSVExportImport(t *testing.T) {
	c := newClient(t)
	st, raw := c.raw("GET", "/v1/export.csv", nil, "")
	if st != 200 || !bytes.HasPrefix(raw, []byte("name,producer")) {
		t.Fatalf("export: %d %q", st, raw[:min(len(raw), 80)])
	}
	lines := bytes.Count(raw, []byte("\n"))
	if lines < 22 {
		t.Fatalf("export lines %d", lines)
	}

	// round trip into a fresh household (Elena's cellar is the same, so import into the same and count)
	csvBody := "name,color,grapes,vintage,quantity,drink_from,drink_until\n" +
		"Тестовое,red,Саперави|Мерло,2020,2,2024,2030\n" +
		",red,,,1,,\n" + // missing name → row error
		"Плохое,blue,,,1,,\n" + // bad color → row error
		"Н/В брют,sparkling,,Н/В,1,,\n"
	st, b := c.do("POST", "/v1/import.csv", csvBody)
	expect(t, st, 200, b)
	if b["imported"].(float64) != 2 || len(b["errors"].([]any)) != 2 {
		t.Fatalf("import: %v", b)
	}
	// multipart upload of the export itself
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	fw, _ := mw.CreateFormFile("file", "cellar.csv")
	_, _ = fw.Write(raw)
	_ = mw.Close()
	st, body := c.raw("POST", "/v1/import.csv", mp.Bytes(), mw.FormDataContentType())
	if st != 200 || !bytes.Contains(body, []byte(`"imported":22`)) {
		t.Fatalf("multipart import: %d %s", st, body)
	}
	st, b = c.do("POST", "/v1/import.csv", "producer\nX\n")
	expect(t, st, 400, b)
	st, b = c.do("POST", "/v1/import.csv", "")
	expect(t, st, 400, b)
}

func TestStaticApp(t *testing.T) {
	c := newClient(t)
	st, raw := c.raw("GET", "/app/", nil, "")
	if st != 200 || !bytes.Contains(raw, []byte("Terroir")) {
		t.Fatalf("app: %d", st)
	}
	_ = time.Now
}
