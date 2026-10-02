package service_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

func TestImportMarkdownFixture(t *testing.T) {
	e := newEnvEmpty(t)
	ctx := context.Background()
	f, err := os.Open("testdata/cellar.md")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res, err := e.svc.ImportMarkdown(ctx, e.anton, f)
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 5 || len(res.Errors) != 2 {
		t.Fatalf("result: %+v", res)
	}
	if !strings.Contains(res.Errors[0].Message, "style") || !strings.Contains(res.Errors[1].Message, "qty") {
		t.Fatalf("errors: %+v", res.Errors)
	}
	items, _ := e.svc.ListItems(ctx, e.anton, service.ItemFilter{CellarFilter: storage.CellarFilter{IncludeEmpty: true}})
	byName := map[string]service.ItemView{}
	for _, it := range items {
		byName[it.Wine.Name] = it
	}
	k := byName["Красностоп «Тестовый склон»"]
	if k.Item.Quantity != 2 || *k.Wine.Vintage != 2019 || k.Wine.Color != domain.ColorRed || k.Wine.Sweetness != domain.SweetnessDry ||
		k.Wine.Source != domain.SourceImport || !strings.Contains(k.Item.Notes, "стейк") {
		t.Fatalf("krasnostop: %+v", k)
	}
	kz := byName["Киндзмараули «Демо»"]
	if kz.Wine.Vintage != nil || kz.Wine.Sweetness != domain.SweetnessMedium {
		t.Fatalf("kindzmarauli: %+v", kz.Wine)
	}
	rk := byName["Ркацители «Пример»"]
	if rk.Wine.Vintage != nil || rk.Item.Quantity != 1 || rk.Wine.Color != domain.ColorWhite {
		t.Fatalf("rkatsiteli: %+v", rk)
	}
	sv := byName["Сира — Вионье «Образец»"]
	if len(sv.Wine.Grapes) != 2 || sv.Wine.Grapes[1] != "Вионье" || sv.Wine.Sweetness != "" {
		t.Fatalf("syrah-viognier: %+v", sv.Wine)
	}
	br := byName["Брют «Учебный»"]
	if br.Wine.Color != domain.ColorSparkling || br.Wine.Vintage != nil || len(br.Wine.Grapes) != 2 {
		t.Fatalf("brut: %+v", br.Wine)
	}
}

func TestImportMarkdownEnglishAndErrors(t *testing.T) {
	e := newEnvEmpty(t)
	ctx := context.Background()
	md := "| Wine | Vintage | Grape | Style | Pairs with | Qty |\n|---|---|---|---|---|---|\n| Demo Red | 2020 | Syrah | red dry | steak | 4 |\n| Demo Rosé | | | rose off-dry | salad | |\n"
	res, err := e.svc.ImportMarkdown(ctx, e.anton, strings.NewReader(md))
	if err != nil || res.Imported != 2 || len(res.Errors) != 0 {
		t.Fatalf("en: %v %+v", err, res)
	}
	for name, body := range map[string]string{
		"no table":     "просто текст",
		"no name col":  "| Стиль | Кол-во |\n|---|---|\n| красное | 1 |\n",
		"no style col": "| Вино | Кол-во |\n|---|---|\n| X | 1 |\n",
	} {
		if _, err := e.svc.ImportMarkdown(ctx, e.anton, strings.NewReader(body)); !domain.IsValidation(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	res, _ = e.svc.ImportMarkdown(ctx, e.anton, strings.NewReader("| Вино | Стиль | Винтаж |\n|-|-|-|\n|  | красное | |\n| X | красное | 18xx |\n| Y | красное | 1700 |\n"))
	if res.Imported != 0 || len(res.Errors) != 3 {
		t.Fatalf("bad rows: %+v", res)
	}
}

func TestParseStyle(t *testing.T) {
	cases := []struct {
		in    string
		color domain.Color
		sweet domain.Sweetness
		ok    bool
	}{
		{"Красное сухое", domain.ColorRed, domain.SweetnessDry, true},
		{"белое полусухое", domain.ColorWhite, domain.SweetnessOffDry, true},
		{"розовое полусладкое", domain.ColorRose, domain.SweetnessMedium, true},
		{"оранжевое", domain.ColorOrange, "", true},
		{"янтарное квеври", domain.ColorOrange, "", true},
		{"игристое брют", domain.ColorSparkling, "", true},
		{"десертное сладкое", domain.ColorDessert, domain.SweetnessSweet, true},
		{"креплёное", domain.ColorFortified, "", true},
		{"White off-dry", domain.ColorWhite, domain.SweetnessOffDry, true},
		{"sparkling", domain.ColorSparkling, "", true},
		{"???", "", "", false},
	}
	for _, c := range cases {
		color, sweet, ok := service.ParseStyle(c.in)
		if color != c.color || sweet != c.sweet || ok != c.ok {
			t.Errorf("%q → %s %s %v", c.in, color, sweet, ok)
		}
	}
}
