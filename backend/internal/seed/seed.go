// Package seed creates the demo household: two users (Anton, Elena), a shared cellar with
// ~20 fictional bottles (Russian and Georgian focus), ~10 home recipes, a taste profile and history.
//
// All producers and bottles are fictional or generic. No real personal data.
package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

// Fixed ids/tokens so that docs/api/examples.sh works after `make reset-dev`.
const (
	UserAnton  = "u-anton"
	UserElena  = "u-elena"
	CellarID   = "c-home"
	TokenAnton = "dev-anton-token"
	TokenElena = "dev-elena-token"
)

type bottle struct {
	key                     string
	name, producer, country string
	region                  string
	grapes                  []string
	vintage                 int // 0 = NV
	color                   domain.Color
	sweet                   domain.Sweetness
	abv                     float64
	body, acid, tannin      int
	notes                   string
	qty                     int
	loc                     string
	from, until             int
	priceRub                int64
}

var bottles = []bottle{
	{"krasnostop-don", "Красностоп Золотовский", "Винодельня «Донской склон»", "Россия", "Дон", []string{"Красностоп"}, 2019, domain.ColorRed, domain.SweetnessDry, 13.5, 4, 4, 5, "Ежевика, чёрная смородина, плотные танины", 2, "Стеллаж A, полка 1", 2023, 2030, 2200},
	{"tsimla", "Цимлянский чёрный", "Хутор «Старая станица»", "Россия", "Дон", []string{"Цимлянский чёрный"}, 2020, domain.ColorRed, domain.SweetnessDry, 13, 4, 3, 3, "Вишнёвое варенье, чернослив, мягкие танины", 3, "Стеллаж A, полка 1", 2022, 2027, 1600},
	{"saperavi-kakheti", "Саперави", "Марани «Алазани»", "Грузия", "Кахетия", []string{"Саперави"}, 2018, domain.ColorRed, domain.SweetnessDry, 14, 5, 4, 4, "Чернослив, тёмная вишня, специи", 2, "Стеллаж A, полка 2", 2021, 2026, 1800},
	{"saperavi-qvevri", "Саперави квеври", "Марани «Алазани»", "Грузия", "Кахетия", []string{"Саперави"}, 2021, domain.ColorRed, domain.SweetnessDry, 13.5, 5, 4, 5, "Квеври, ежевика, земля", 1, "Стеллаж A, полка 2", 2026, 2034, 2600},
	{"kindzmarauli", "Киндзмараули", "Марани «Алазани»", "Грузия", "Кахетия", []string{"Саперави"}, 0, domain.ColorRed, domain.SweetnessMedium, 11.5, 4, 3, 2, "Полусладкое, вишнёвый джем", 2, "Холодильник", 0, 0, 1100},
	{"kokur", "Кокур", "Долина «Солнечная»", "Россия", "Крым", []string{"Кокур"}, 2022, domain.ColorWhite, domain.SweetnessDry, 12, 2, 4, 0, "Айва, зелёное яблоко, свежо", 2, "Холодильник", 2023, 2025, 900},
	{"kisi", "Киси", "Марани «Цинандали-Гора»", "Грузия", "Кахетия", []string{"Киси"}, 2022, domain.ColorWhite, domain.SweetnessDry, 13, 3, 3, 0, "Груша, мёд, абрикос", 1, "Холодильник", 2023, 2027, 1500},
	{"kisi-amber", "Киси янтарное", "Марани «Цинандали-Гора»", "Грузия", "Кахетия", []string{"Киси"}, 2021, domain.ColorOrange, domain.SweetnessDry, 13, 4, 3, 2, "Квеври, курага, чай, лёгкая терпкость", 1, "Стеллаж B", 2023, 2029, 2100},
	{"rkatsiteli", "Ркацители", "Марани «Алазани»", "Грузия", "Кахетия", []string{"Ркацители"}, 2023, domain.ColorWhite, domain.SweetnessDry, 12.5, 3, 4, 0, "Цитрус, айва, травы", 3, "Холодильник", 2024, 2026, 800},
	{"rkatsiteli-amber", "Ркацители янтарное", "Марани «Алазани»", "Грузия", "Кахетия", []string{"Ркацители"}, 2020, domain.ColorOrange, domain.SweetnessDry, 12.5, 3, 4, 3, "Квеври, грецкий орех, сухофрукты", 1, "Стеллаж B", 2022, 2026, 1700},
	{"syrah-kuban", "Сира", "Усадьба «Тамань-Ветер»", "Россия", "Кубань", []string{"Сира"}, 2020, domain.ColorRed, domain.SweetnessDry, 14, 4, 3, 4, "Ежевика, перец, фиалка", 2, "Стеллаж A, полка 3", 2022, 2028, 2400},
	{"syrah-viognier", "Сира — Вионье", "Усадьба «Тамань-Ветер»", "Россия", "Кубань", []string{"Сира", "Вионье"}, 2021, domain.ColorRed, domain.SweetnessDry, 13.5, 4, 3, 3, "Тёмные ягоды, абрикосовая косточка", 1, "Стеллаж A, полка 3", 2023, 2029, 2800},
	{"viognier", "Вионье", "Усадьба «Тамань-Ветер»", "Россия", "Кубань", []string{"Вионье"}, 2022, domain.ColorWhite, domain.SweetnessDry, 13.5, 4, 2, 0, "Абрикос, персик, масляное", 1, "Холодильник", 2023, 2026, 1900},
	{"pinot-crimea", "Пино Нуар", "Долина «Солнечная»", "Россия", "Крым", []string{"Пино Нуар"}, 2021, domain.ColorRed, domain.SweetnessDry, 12.5, 2, 4, 2, "Клюква, вишня, лесная подстилка", 2, "Стеллаж A, полка 4", 2023, 2027, 2300},
	{"pinot-burgundy", "Бургундия Пино Нуар", "Домен «Деревенский»", "Франция", "Бургундия", []string{"Пино Нуар"}, 2019, domain.ColorRed, domain.SweetnessDry, 13, 2, 4, 2, "Малина, грибы", 1, "Стеллаж A, полка 4", 2022, 2026, 4500},
	{"cabernet-kuban", "Каберне Совиньон резерв", "Шато «Кубанская терраса»", "Россия", "Кубань", []string{"Каберне Совиньон"}, 2017, domain.ColorRed, domain.SweetnessDry, 14.5, 5, 3, 5, "Смородина, кедр, дуб", 1, "Стеллаж A, полка 1", 2020, 2025, 3200},
	{"merlot-crimea", "Мерло", "Долина «Солнечная»", "Россия", "Крым", []string{"Мерло"}, 2016, domain.ColorRed, domain.SweetnessDry, 13.5, 4, 3, 3, "Слива, шоколад; окно прошло", 1, "Стеллаж A, полка 3", 2018, 2023, 1400},
	{"riesling-off", "Рислинг полусухой", "Хутор «Абрау-Склон»", "Россия", "Кубань", []string{"Рислинг"}, 2022, domain.ColorWhite, domain.SweetnessOffDry, 11, 2, 5, 0, "Лайм, персик, лёгкая сладость", 2, "Холодильник", 2023, 2028, 1100},
	{"sparkling-brut", "Игристое брют", "Хутор «Абрау-Склон»", "Россия", "Кубань", []string{"Шардоне", "Пино Нуар"}, 0, domain.ColorSparkling, domain.SweetnessDry, 12, 2, 5, 0, "Классический метод, бриошь", 2, "Холодильник", 0, 0, 1500},
	{"rose-don", "Розе", "Винодельня «Донской склон»", "Россия", "Дон", []string{"Цимлянский чёрный"}, 2023, domain.ColorRose, domain.SweetnessDry, 12, 2, 4, 0, "Клубника, арбуз", 1, "Холодильник", 2024, 2025, 900},
	{"muscat-dessert", "Мускат белый десертный", "Долина «Солнечная»", "Россия", "Крым", []string{"Мускат"}, 2015, domain.ColorDessert, domain.SweetnessSweet, 16, 4, 3, 0, "Мёд, чайная роза", 1, "Стеллаж B", 2018, 2035, 2000},
	{"saperavi-empty", "Саперави резерв", "Марани «Алазани»", "Грузия", "Кахетия", []string{"Саперави"}, 2017, domain.ColorRed, domain.SweetnessDry, 14, 5, 4, 4, "Закончилось — оставлено ради истории", 0, "", 2020, 2027, 2500},
}

type recipe struct {
	key     string
	r       domain.Recipe
	pairsOK []string // bottle keys that worked before (feedback 5)
	pairsKO []string // bottle keys that disappointed (feedback 2)
}

var recipes = []recipe{
	{"tteokbokki", domain.Recipe{Title: "Токпокки с кимчи", Cuisine: "корейская", MainIngredient: domain.IngVegetables, CookingMethod: domain.MethodStewed, Sauce: domain.SauceSweet, Spiciness: 3, Richness: 3, Acidity: 3, TimeMinutes: 30, Tags: []string{"острое", "азия"}}, []string{"riesling-off"}, []string{"saperavi-kakheti"}},
	{"kimchi-pork", domain.Recipe{Title: "Свинина с кимчи (чеюк-поккым)", Cuisine: "корейская", MainIngredient: domain.IngPork, CookingMethod: domain.MethodFried, Sauce: domain.SauceSoy, Spiciness: 3, Richness: 4, Acidity: 3, TimeMinutes: 25, Tags: []string{"острое", "азия"}}, nil, nil},
	{"thai-curry", domain.Recipe{Title: "Тайский красный карри с курицей", Cuisine: "тайская", MainIngredient: domain.IngPoultry, CookingMethod: domain.MethodStewed, Sauce: domain.SauceCurry, Spiciness: 2, Richness: 4, Acidity: 2, TimeMinutes: 35, Tags: []string{"острое", "азия"}}, nil, nil},
	{"khachapuri", domain.Recipe{Title: "Грузинский ужин: хачапури по-аджарски и пхали", Cuisine: "грузинская", MainIngredient: domain.IngCheese, CookingMethod: domain.MethodBaked, Sauce: domain.SauceNone, Spiciness: 0, Richness: 5, Acidity: 2, TimeMinutes: 60, Tags: []string{"грузия", "гости"}}, []string{"kisi-amber"}, nil},
	{"bolognese", domain.Recipe{Title: "Паста болоньезе", Cuisine: "итальянская", MainIngredient: domain.IngPasta, CookingMethod: domain.MethodStewed, Sauce: domain.SauceTomato, Spiciness: 0, Richness: 4, Acidity: 4, TimeMinutes: 90, Tags: []string{"паста"}}, []string{"tsimla"}, nil},
	{"beef-cheeks", domain.Recipe{Title: "Говяжьи щёки, томлёные в красном вине", Cuisine: "европейская", MainIngredient: domain.IngBeef, CookingMethod: domain.MethodBraised, Sauce: domain.SauceRed, Spiciness: 0, Richness: 5, Acidity: 3, TimeMinutes: 240, Tags: []string{"мясо", "выходные"}}, []string{"saperavi-kakheti"}, nil},
	{"pork-neck", domain.Recipe{Title: "Свиная шея на углях", Cuisine: "домашняя", MainIngredient: domain.IngPork, CookingMethod: domain.MethodGrilled, Sauce: domain.SauceNone, Spiciness: 1, Richness: 4, Acidity: 2, TimeMinutes: 120, Tags: []string{"мангал", "мясо"}}, []string{"krasnostop-don"}, nil},
	{"salmon", domain.Recipe{Title: "Лосось в духовке с лимоном и укропом", Cuisine: "домашняя", MainIngredient: domain.IngFattyFish, CookingMethod: domain.MethodBaked, Sauce: domain.SauceCitrus, Spiciness: 0, Richness: 3, Acidity: 3, TimeMinutes: 30, Tags: []string{"рыба", "будни"}}, nil, nil},
	{"tandoori", domain.Recipe{Title: "Курица тандури", Cuisine: "индийская", MainIngredient: domain.IngPoultry, CookingMethod: domain.MethodGrilled, Sauce: domain.SauceCurry, Spiciness: 2, Richness: 3, Acidity: 3, TimeMinutes: 60, Tags: []string{"острое"}}, nil, nil},
	{"steak", domain.Recipe{Title: "Стейк рибай", Cuisine: "домашняя", MainIngredient: domain.IngBeef, CookingMethod: domain.MethodGrilled, Sauce: domain.SauceNone, Spiciness: 0, Richness: 5, Acidity: 1, TimeMinutes: 25, Tags: []string{"мясо"}}, nil, nil},
	{"fish-cream", domain.Recipe{Title: "Треска в сливочном соусе", Cuisine: "европейская", MainIngredient: domain.IngFish, CookingMethod: domain.MethodBaked, Sauce: domain.SauceCream, Spiciness: 0, Richness: 4, Acidity: 2, TimeMinutes: 40, Tags: []string{"рыба"}}, nil, nil},
}

// Exists reports whether the demo household is already present.
func Exists(ctx context.Context, st storage.Store) (bool, error) {
	_, err := st.GetUser(ctx, UserAnton)
	if err == domain.ErrNotFound {
		return false, nil
	}
	return err == nil, err
}

// Run seeds the demo household. now drives purchase dates and history; idempotent via Exists.
func Run(ctx context.Context, st storage.Store, now time.Time) error {
	if ok, err := Exists(ctx, st); err != nil || ok {
		return err
	}
	n := 0
	id := func(prefix string) string { n++; return fmt.Sprintf("%s-%03d", prefix, n) }
	created := now.AddDate(0, -2, 0)

	for _, u := range []domain.User{
		{ID: UserAnton, DisplayName: "Anton", Locale: domain.LocaleRU, CreatedAt: created},
		{ID: UserElena, DisplayName: "Elena", Locale: domain.LocaleRU, CreatedAt: created},
	} {
		if err := st.CreateUser(ctx, u); err != nil {
			return err
		}
	}
	if err := st.AddToken(ctx, UserAnton, service.HashToken(TokenAnton)); err != nil {
		return err
	}
	if err := st.AddToken(ctx, UserElena, service.HashToken(TokenElena)); err != nil {
		return err
	}
	if err := st.CreateCellar(ctx, domain.Cellar{ID: CellarID, Name: "Домашний погреб", CreatedAt: created}); err != nil {
		return err
	}
	if err := st.AddMember(ctx, domain.Membership{CellarID: CellarID, UserID: UserAnton, Role: domain.RoleOwner}); err != nil {
		return err
	}
	if err := st.AddMember(ctx, domain.Membership{CellarID: CellarID, UserID: UserElena, Role: domain.RoleMember}); err != nil {
		return err
	}

	// Drinking windows (and vintages) in the table are written as of 2026; shift them so the
	// demo keeps the same mix of "too early / peak / last call / late" in any year.
	shift := now.Year() - 2026
	wineIDs := map[string]string{}
	itemIDs := map[string]string{}
	for _, b := range bottles {
		if b.vintage != 0 {
			b.vintage += shift
		}
		if b.from != 0 {
			b.from += shift
		}
		if b.until != 0 {
			b.until += shift
		}
		w := domain.Wine{ID: "w-" + b.key, CellarID: CellarID, Name: b.name, Producer: b.producer, Country: b.country,
			Region: b.region, Grapes: b.grapes, Color: b.color, Sweetness: b.sweet, Body: b.body, Acidity: b.acid, Tannin: b.tannin,
			StyleNotes: b.notes, Source: domain.SourceSeed, CreatedAt: created, UpdatedAt: created}
		if b.vintage != 0 {
			v := b.vintage
			w.Vintage = &v
		}
		abv := b.abv
		w.ABV = &abv
		if err := w.Validate(); err != nil {
			return fmt.Errorf("seed wine %s: %w", b.key, err)
		}
		if err := st.CreateWine(ctx, w); err != nil {
			return err
		}
		price := b.priceRub * 100
		bought := created.AddDate(0, 0, -len(itemIDs)*7)
		it := domain.CellarItem{ID: "i-" + b.key, CellarID: CellarID, WineID: w.ID, Quantity: b.qty, StorageLocation: b.loc,
			PurchasePriceMinor: &price, Currency: "RUB", PurchasePlace: "винотека", PurchasedAt: &bought, CreatedAt: created, UpdatedAt: created}
		if b.from != 0 {
			f := b.from
			it.DrinkFrom = &f
		}
		if b.until != 0 {
			u := b.until
			it.DrinkUntil = &u
		}
		if err := it.Validate(); err != nil {
			return fmt.Errorf("seed item %s: %w", b.key, err)
		}
		if err := st.CreateItem(ctx, it); err != nil {
			return err
		}
		wineIDs[b.key], itemIDs[b.key] = w.ID, it.ID
	}

	for _, rc := range recipes {
		r := rc.r
		r.ID, r.CellarID, r.CreatedAt, r.UpdatedAt = "r-"+rc.key, CellarID, created, created
		r.Normalize()
		if err := r.Validate(); err != nil {
			return fmt.Errorf("seed recipe %s: %w", rc.key, err)
		}
		if err := st.CreateRecipe(ctx, r); err != nil {
			return err
		}
		for i, keys := range [][]string{rc.pairsOK, rc.pairsKO} {
			rating := 5
			if i == 1 {
				rating = 2
			}
			for _, k := range keys {
				f := domain.PairingFeedback{ID: id("pf"), CellarID: CellarID, RecipeID: r.ID, WineID: wineIDs[k], UserID: UserAnton,
					Rating: rating, Note: "из истории", CreatedAt: created.AddDate(0, 0, 3)}
				if err := st.AddPairingFeedback(ctx, f); err != nil {
					return err
				}
			}
		}
	}

	// History: the empty Saperavi reserve was drunk (rated 5) — feeds "buy again".
	// Drink() needs stock, so add a bottle, drink it, and the row stays at quantity 0.
	emptyItem := itemIDs["saperavi-empty"]
	if err := st.UpdateItem(ctx, withQty(ctx, st, emptyItem, 2)); err != nil {
		return err
	}
	history := []struct {
		item, user string
		daysAgo    int
		rating     int
		note       string
	}{
		{emptyItem, UserAnton, 40, 5, "Плотное, джемовое — то, что любим"},
		{emptyItem, UserElena, 20, 5, "Купить ещё"},
		{itemIDs["krasnostop-don"], UserAnton, 25, 4, "К шее на углях — отлично"},
		{itemIDs["rkatsiteli"], UserElena, 10, 3, "Кисловато для меня"},
	}
	for _, h := range history {
		ev := domain.ConsumptionEvent{ID: id("ev"), CellarItemID: h.item, UserID: h.user, Bottles: 1,
			DrunkAt: now.AddDate(0, 0, -h.daysAgo), Rating: h.rating, Note: h.note}
		if _, err := st.Drink(ctx, CellarID, ev); err != nil {
			return fmt.Errorf("seed drink: %w", err)
		}
	}
	// restore quantities for bottles drunk in history (seed declares the current stock)
	for _, k := range []string{"krasnostop-don", "rkatsiteli"} {
		for _, b := range bottles {
			if b.key == k {
				if err := st.UpdateItem(ctx, withQty(ctx, st, itemIDs[k], b.qty)); err != nil {
					return err
				}
			}
		}
	}

	// Taste profiles: Anton — dark jammy fruit, low acidity, Korean-level heat. Elena — similar, fewer tannins.
	profiles := []domain.TasteProfile{
		{UserID: UserAnton, Acidity: 2, Tannin: 3, Body: 4, Sweetness: 1, FruitDark: 5, FruitRed: 2, Oak: 3, SpiceTolerance: 3,
			LikedGrapes: []string{"saperavi", "syrah", "tsimlyansky_cherny", "krasnostop"}, RejectedGrapes: []string{"sauvignon_blanc"},
			Regions: []string{"Kakheti", "Don", "Kuban"}, Colors: []domain.Color{domain.ColorRed, domain.ColorOrange},
			BudgetMinMinor: 100000, BudgetMaxMinor: 300000, BudgetCurrency: "RUB",
			Notes: "Тёмные джемовые фрукты, низкая кислотность, острота корейского уровня", Version: 1, UpdatedAt: created},
		{UserID: UserElena, Acidity: 2, Tannin: 2, Body: 3, Sweetness: 2, FruitDark: 4, FruitRed: 3, Oak: 2, SpiceTolerance: 3,
			LikedGrapes: []string{"saperavi", "kisi"}, RejectedGrapes: []string{},
			Regions: []string{"Kakheti", "Crimea"}, Colors: []domain.Color{domain.ColorRed, domain.ColorWhite},
			Notes: "Любит мягкое, фруктовое; к острому — что-то освежающее", Version: 1, UpdatedAt: created},
	}
	for _, p := range profiles {
		if err := st.SaveProfileWithAnswers(ctx, p, nil); err != nil {
			return err
		}
	}
	facts := []domain.MemoryFact{
		{ID: id("mf"), UserID: UserAnton, Fact: "Не любит кислые белые вина", Source: "seed", CreatedAt: created},
		{ID: id("mf"), UserID: UserAnton, Fact: "По пятницам часто готовим корейское", Source: "seed", CreatedAt: created},
		{ID: id("mf"), UserID: UserElena, Fact: "Предпочитает вина помягче и пофруктовее", Source: "seed", CreatedAt: created},
	}
	for _, f := range facts {
		if err := st.AddFact(ctx, f); err != nil {
			return err
		}
	}
	return nil
}

func withQty(ctx context.Context, st storage.Store, itemID string, qty int) domain.CellarItem {
	e, err := st.GetItem(ctx, CellarID, itemID)
	if err != nil {
		return domain.CellarItem{ID: itemID, CellarID: CellarID, Quantity: qty}
	}
	it := e.Item
	it.Quantity = qty
	return it
}
