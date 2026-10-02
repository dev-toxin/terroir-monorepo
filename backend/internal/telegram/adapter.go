package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/anko-soft-lab/terroir/backend/internal/service"
)

// ServiceHandler implements Handler over the Terroir services.
type ServiceHandler struct {
	Svc       *service.Service
	Assistant *service.Assistant
}

// Cellar lists bottles that are ready to open (top 10).
func (h ServiceHandler) Cellar(ctx context.Context, token string) (string, error) {
	act, err := h.Svc.Authenticate(ctx, token)
	if err != nil {
		return "", err
	}
	items, err := h.Svc.ListItems(ctx, act, service.ItemFilter{ReadyToDrink: true})
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "Сейчас нечего срочно открывать.", nil
	}
	var b strings.Builder
	b.WriteString("Пора открыть:\n")
	for i, v := range items {
		if i == 10 {
			break
		}
		fmt.Fprintf(&b, "• %s ×%d — %s (/drink %s)\n", service.WineTitle(v.Wine), v.Item.Quantity, v.WindowLabel, v.Item.ID)
	}
	return b.String(), nil
}

// Drink records one bottle.
func (h ServiceHandler) Drink(ctx context.Context, token, itemID string, rating int) (string, error) {
	act, err := h.Svc.Authenticate(ctx, token)
	if err != nil {
		return "", err
	}
	res, err := h.Svc.Drink(ctx, act, itemID, service.DrinkRequest{Rating: rating})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Записал. Осталось бутылок: %d", res.Item.Quantity), nil
}

// Ask forwards text to the assistant.
func (h ServiceHandler) Ask(ctx context.Context, token, text string) (string, error) {
	act, err := h.Svc.Authenticate(ctx, token)
	if err != nil {
		return "", err
	}
	res, err := h.Assistant.Chat(ctx, act, service.ChatRequest{Message: text})
	if err != nil {
		return "", err
	}
	return res.Answer, nil
}
