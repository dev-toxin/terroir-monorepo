package llm

import (
	"context"
	"fmt"
	"strings"
)

// Mock is a deterministic provider: no network, answers from Request.Facts with templates.
// It references bottles as [item:<id>] exactly like the real prompt asks the model to.
type Mock struct{}

// Name implements Provider.
func (Mock) Name() string { return "mock" }

// Model implements Provider.
func (Mock) Model() string { return "mock-sommelier-1" }

// Complete implements Provider.
func (Mock) Complete(_ context.Context, req Request) (Response, error) {
	f := req.Facts
	if f == nil {
		f = &Facts{Locale: "ru", Intent: "general"}
	}
	en := f.Locale == "en"
	var b strings.Builder
	switch {
	case f.Intent == "buy_again":
		if len(f.BuyAgain) == 0 {
			b.WriteString(pick(en, "Пока нет вин с оценкой 4+ — отмечайте «выпил» с оценкой, и я соберу список.",
				"No wines rated 4+ yet — rate bottles when you drink them and I'll build the list."))
		} else {
			b.WriteString(pick(en, "Стоит купить снова (по вашим оценкам 4+):\n", "Worth buying again (your ratings 4+):\n"))
			for _, w := range f.BuyAgain {
				fmt.Fprintf(&b, "• %s\n", w)
			}
		}
	case len(f.Candidates) == 0:
		b.WriteString(pick(en, "В погребе сейчас нет подходящих бутылок. Если хотите — подскажу, что купить.",
			"There are no suitable bottles in your cellar right now. I can suggest what to buy."))
	default:
		top := f.Candidates[0]
		switch f.Intent {
		case "pair_recipe":
			fmt.Fprintf(&b, pick(en, "К блюду «%s» я бы открыл %s [item:%s].", "To \"%s\" I would open %s [item:%s]."), f.RecipeName, top.Title, top.ItemID)
		case "drink_now":
			fmt.Fprintf(&b, pick(en, "Пора открыть %s [item:%s] — %s.", "Time to open %s [item:%s] — %s."), top.Title, top.ItemID, top.Window)
		default:
			fmt.Fprintf(&b, pick(en, "Из вашего погреба я бы выбрал %s [item:%s].", "From your cellar I'd pick %s [item:%s]."), top.Title, top.ItemID)
		}
		if len(top.Reasons) > 0 {
			b.WriteString(pick(en, " Почему: ", " Why: "))
			b.WriteString(strings.Join(top.Reasons, "; "))
			b.WriteString(".")
		}
		if len(f.Candidates) > 1 {
			b.WriteString(pick(en, "\nЗапасные варианты: ", "\nAlternatives: "))
			alts := make([]string, 0, 2)
			for _, c := range f.Candidates[1:min(3, len(f.Candidates))] {
				alts = append(alts, fmt.Sprintf("%s [item:%s]", c.Title, c.ItemID))
			}
			b.WriteString(strings.Join(alts, ", "))
			b.WriteString(".")
		}
	}
	prompt := 0
	for _, m := range req.Messages {
		prompt += EstimateTokens(m.Content)
	}
	out := b.String()
	return Response{Content: out, Model: Mock{}.Model(), Usage: Usage{PromptTokens: prompt, CompletionTokens: EstimateTokens(out)}}, nil
}

func pick(en bool, ru, eng string) string {
	if en {
		return eng
	}
	return ru
}
