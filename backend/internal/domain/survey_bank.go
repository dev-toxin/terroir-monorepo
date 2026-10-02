package domain

// SurveyBank is the built-in taste survey. Order matters: batches are served in this order.
// Questions are deliberately concrete ("how do you feel about…") rather than abstract wine jargon.
var SurveyBank = []Question{
	scaleQ("q_body", 1, AxisBody,
		"Какое вино по ощущению во рту вам ближе? 1 — лёгкое, как вода, 5 — плотное, густое",
		"How full should a wine feel? 1 — light as water, 5 — dense and thick"),
	scaleQ("q_acidity", 2, AxisAcidity,
		"Как вы относитесь к кислотности (вино «свежее, слюноотделение»)? 1 — не люблю, 5 — обожаю",
		"How do you like acidity (fresh, mouth-watering)? 1 — dislike, 5 — love"),
	scaleQ("q_tannin", 3, AxisTannin,
		"Терпкость (вяжет, как крепкий чай): 1 — мешает, 5 — чем больше, тем лучше",
		"Tannin (grippy like strong tea): 1 — bothers me, 5 — the more the better"),
	scaleQ("q_sweet", 4, AxisSweetness,
		"Сладость: 1 — только сухое, 5 — люблю сладкие вина",
		"Sweetness: 1 — bone dry only, 5 — I love sweet wines"),
	scaleQ("q_fruit_dark", 5, AxisFruitDark,
		"Тёмные «джемовые» фрукты — ежевика, чернослив, вишнёвое варенье: 1 — не моё, 5 — именно это",
		"Dark jammy fruit — blackberry, prune, cherry jam: 1 — not me, 5 — exactly this"),
	scaleQ("q_fruit_red", 6, AxisFruitRed,
		"Свежие красные ягоды — клюква, малина, брусника: 1 — не моё, 5 — именно это",
		"Fresh red berries — cranberry, raspberry, lingonberry: 1 — not me, 5 — exactly this"),
	scaleQ("q_oak", 7, AxisOak,
		"Тона дуба — ваниль, дым, поджаренный хлеб: 1 — не люблю, 5 — обожаю",
		"Oak — vanilla, smoke, toast: 1 — dislike, 5 — love"),
	{
		ID: "q_spice", Order: 8, Type: AnswerChoice, Axis: AxisSpice,
		TextRU: "Насколько острую еду вы едите?", TextEN: "How spicy is the food you eat?",
		Options: []Option{
			{"0", "Совсем не острую", "Not spicy at all", []Effect{{Axis: AxisSpice, Set: 0}}},
			{"1", "Чуть-чуть перца", "A little pepper", []Effect{{Axis: AxisSpice, Set: 1}}},
			{"2", "Средне — карри, аджика", "Medium — curry, adjika", []Effect{{Axis: AxisSpice, Set: 2}}},
			{"3", "Корейский/тайский уровень", "Korean/Thai level", []Effect{{Axis: AxisSpice, Set: 3}}},
		},
	},
	{
		ID: "q_colors", Order: 9, Type: AnswerMulti, Axis: AxisColorsPrefer,
		TextRU: "Что вы обычно пьёте? (можно несколько)", TextEN: "What do you usually drink? (multiple)",
		Options: []Option{
			colorOpt(ColorRed, "Красное", "Red"), colorOpt(ColorWhite, "Белое", "White"),
			colorOpt(ColorRose, "Розовое", "Rosé"), colorOpt(ColorSparkling, "Игристое", "Sparkling"),
			colorOpt(ColorOrange, "Оранжевое", "Orange"), colorOpt(ColorDessert, "Десертное", "Dessert"),
		},
	},
	{
		ID: "q_grapes_like", Order: 10, Type: AnswerMulti, Axis: AxisLikedGrapes,
		TextRU: "Какие сорта вам нравились? (можно несколько)", TextEN: "Which grapes did you enjoy? (multiple)",
		Options: grapeOptions(AxisLikedGrapes),
	},
	{
		ID: "q_grapes_reject", Order: 11, Type: AnswerMulti, Axis: AxisRejected,
		TextRU: "Какие сорта точно не ваши? (можно несколько)", TextEN: "Which grapes are definitely not for you? (multiple)",
		Options: grapeOptions(AxisRejected),
	},
	{
		ID: "q_regions", Order: 12, Type: AnswerMulti, Axis: AxisRegions,
		TextRU: "Вина каких регионов хочется пить чаще?", TextEN: "Which regions would you like to drink more?",
		Options: []Option{
			regionOpt("Крым", "Crimea"), regionOpt("Кубань", "Kuban"), regionOpt("Дон", "Don"),
			regionOpt("Кахетия", "Kakheti"), regionOpt("Рона", "Rhône"), regionOpt("Бургундия", "Burgundy"),
			regionOpt("Тоскана", "Tuscany"), regionOpt("Риоха", "Rioja"),
		},
	},
	{
		ID: "q_budget", Order: 13, Type: AnswerChoice, Axis: AxisBudget,
		TextRU: "Сколько обычно готовы потратить на бутылку на ужин дома?",
		TextEN: "How much do you usually spend on a bottle for dinner at home?",
		Options: []Option{
			{"low", "До 1000 ₽ / до $12", "Under $12", []Effect{{Axis: AxisBudget, Min: 0, Max: 100000, Value: "RUB"}}},
			{"mid", "1000–2500 ₽ / $12–30", "$12–30", []Effect{{Axis: AxisBudget, Min: 100000, Max: 250000, Value: "RUB"}}},
			{"high", "2500–5000 ₽ / $30–60", "$30–60", []Effect{{Axis: AxisBudget, Min: 250000, Max: 500000, Value: "RUB"}}},
			{"top", "Больше 5000 ₽ / $60+", "$60+", []Effect{{Axis: AxisBudget, Min: 500000, Max: 0, Value: "RUB"}}},
		},
	},
	{
		ID: "q_steak", Order: 14, Type: AnswerChoice, Axis: AxisTannin,
		TextRU: "К стейку вы бы выбрали…", TextEN: "With a steak you would pick…",
		Options: []Option{
			{"light", "Лёгкое красное, Пино Нуар", "A light red like Pinot Noir", []Effect{{Axis: AxisTannin, Set: 2}, {Axis: AxisBody, Set: 3}}},
			{"bold", "Мощное терпкое — Каберне, Красностоп", "A bold grippy red — Cabernet, Krasnostop", []Effect{{Axis: AxisTannin, Set: 4}, {Axis: AxisBody, Set: 5}}},
			{"jammy", "Сочное, фруктовое, мягкое — Сира, Саперави", "Juicy, soft, fruity — Syrah, Saperavi", []Effect{{Axis: AxisTannin, Set: 3}, {Axis: AxisFruitDark, Set: 5}, {Axis: AxisAcidity, Set: 2}}},
		},
	},
	{
		ID: "q_sour_wine", Order: 15, Type: AnswerChoice, Axis: AxisAcidity,
		TextRU: "Вино показалось «кислым». Ваша реакция?", TextEN: "A wine tastes \"sour\". Your reaction?",
		Options: []Option{
			{"bad", "Не допью", "I won't finish it", []Effect{{Axis: AxisAcidity, Set: 1}}},
			{"ok", "Нормально к еде", "Fine with food", []Effect{{Axis: AxisAcidity, Set: 3}}},
			{"great", "Отлично, освежает", "Great, refreshing", []Effect{{Axis: AxisAcidity, Set: 5}}},
		},
	},
}

func scaleQ(id string, order int, axis Axis, ru, en string) Question {
	return Question{ID: id, Order: order, TextRU: ru, TextEN: en, Type: AnswerScale, Axis: axis}
}

func colorOpt(c Color, ru, en string) Option {
	return Option{string(c), ru, en, []Effect{{Axis: AxisColorsPrefer, Add: string(c)}}}
}

func regionOpt(ru, en string) Option {
	return Option{en, ru, en, []Effect{{Axis: AxisRegions, Add: en}}}
}

func grapeOptions(axis Axis) []Option {
	keys := []string{"saperavi", "krasnostop", "tsimlyansky_cherny", "cabernet_sauvignon", "merlot", "syrah", "pinot_noir",
		"rkatsiteli", "kisi", "kokur", "viognier", "chardonnay", "sauvignon_blanc", "riesling"}
	out := make([]Option, 0, len(keys))
	for _, k := range keys {
		g, _ := LookupGrape(k)
		out = append(out, Option{k, g.NameRU, g.NameEN, []Effect{{Axis: axis, Add: k}}})
	}
	return out
}
