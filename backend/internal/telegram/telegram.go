// Package telegram is a SKELETON of a Telegram bot adapter (backlog item, not wired into cmd/api).
//
// It talks to the Telegram Bot API via plain net/http (long polling getUpdates + sendMessage).
// The base URL is configurable (default https://api.telegram.org) — it is a normal adapter,
// not a way around anything. The bot does nothing unless TELEGRAM_BOT_TOKEN is set.
//
// TODO(auth): mapping Telegram user id → Terroir user is a config map tgID → API token for now
// (TELEGRAM_USERS="123456:dev-anton-token,..."). Replace with Telegram Login / users.telegram_id.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL of the Bot API.
const DefaultBaseURL = "https://api.telegram.org"

// Update is the subset of a Telegram update we use.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message is a Telegram message.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

// User is a Telegram user.
type User struct {
	ID           int64  `json:"id"`
	LanguageCode string `json:"language_code"`
}

// Chat is a Telegram chat.
type Chat struct {
	ID int64 `json:"id"`
}

// API is the Bot API surface the bot needs.
type API interface {
	GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// Client implements API over HTTP.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewClient creates a client. baseURL "" → DefaultBaseURL.
func NewClient(baseURL, token string) (*Client, error) {
	if token == "" {
		return nil, errors.New("telegram: TELEGRAM_BOT_TOKEN is not set")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: &http.Client{Timeout: 70 * time.Second}}, nil
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/bot"+c.Token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		// never include the URL (it contains the token) in errors
		var ue *url.Error
		if errors.As(err, &ue) {
			return fmt.Errorf("telegram %s: %w", method, ue.Err)
		}
		return fmt.Errorf("telegram %s: request failed", method)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return fmt.Errorf("telegram %s: http %d, bad response", method, res.StatusCode)
	}
	if !ar.OK {
		return fmt.Errorf("telegram %s: %s", method, ar.Description)
	}
	if out != nil {
		return json.Unmarshal(ar.Result, out)
	}
	return nil
}

// GetUpdates long-polls for updates.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]Update, error) {
	var ups []Update
	err := c.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": int(timeout.Seconds()), "allowed_updates": []string{"message"}}, &ups)
	return ups, err
}

// SendMessage sends a plain-text message.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.call(ctx, "sendMessage", map[string]any{"chat_id": chatID, "text": text}, nil)
}

// ---------- commands ----------

// CommandKind is a bot command.
type CommandKind string

const (
	CmdHelp    CommandKind = "help"
	CmdCellar  CommandKind = "cellar"
	CmdDrink   CommandKind = "drink"
	CmdPair    CommandKind = "pair"
	CmdChat    CommandKind = "chat" // free text → assistant
	CmdUnknown CommandKind = "unknown"
)

// Command is a parsed message.
type Command struct {
	Kind   CommandKind
	ItemID string // /drink
	Rating int    // /drink, 0 = none
	Text   string // /pair, chat
	Error  string // human-readable parse error (Kind stays the intended command)
}

// ParseCommand parses /cellar, /drink <item_id> [rating 1-5], /pair <recipe text>, /help, /start
// and free text. "/cmd@BotName" is accepted.
func ParseCommand(text string) Command {
	text = strings.TrimSpace(text)
	if text == "" {
		return Command{Kind: CmdUnknown, Error: "empty message"}
	}
	if !strings.HasPrefix(text, "/") {
		return Command{Kind: CmdChat, Text: text}
	}
	fields := strings.Fields(text)
	name := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	args := fields[1:]
	switch name {
	case "help", "start":
		return Command{Kind: CmdHelp}
	case "cellar":
		return Command{Kind: CmdCellar}
	case "drink":
		c := Command{Kind: CmdDrink}
		if len(args) == 0 {
			c.Error = "usage: /drink <item_id> [rating 1-5]"
			return c
		}
		c.ItemID = args[0]
		if len(args) > 1 {
			r, err := strconv.Atoi(args[1])
			if err != nil || r < 1 || r > 5 {
				c.Error = "rating must be 1..5"
				return c
			}
			c.Rating = r
		}
		if len(args) > 2 {
			c.Error = "usage: /drink <item_id> [rating 1-5]"
		}
		return c
	case "pair":
		c := Command{Kind: CmdPair, Text: strings.Join(args, " ")}
		if c.Text == "" {
			c.Error = "usage: /pair <dish or recipe>"
		}
		return c
	default:
		return Command{Kind: CmdUnknown, Error: "unknown command /" + name}
	}
}

// ---------- routing ----------

// Handler executes commands for an authenticated Terroir user (identified by API token).
// An adapter over *service.Service and *service.Assistant implements it (see adapter.go).
type Handler interface {
	Cellar(ctx context.Context, token string) (string, error)
	Drink(ctx context.Context, token, itemID string, rating int) (string, error)
	Ask(ctx context.Context, token, text string) (string, error)
}

// HelpText is the /help reply.
const HelpText = "Terroir бот:\n/cellar — что пора открыть\n/drink <id> [оценка 1-5] — выпил бутылку\n/pair <блюдо> — что открыть к блюду\nИли просто напишите вопрос."

// Bot routes Telegram messages to a Handler.
type Bot struct {
	API     API
	Handler Handler
	Users   map[int64]string // Telegram user id → Terroir API token (TODO: real login)
	Poll    time.Duration
}

// Reply computes the answer for one message (pure routing; no network besides the Handler).
func (b *Bot) Reply(ctx context.Context, m *Message) string {
	if m == nil || m.From == nil {
		return ""
	}
	cmd := ParseCommand(m.Text)
	if cmd.Kind == CmdHelp {
		return HelpText
	}
	token, ok := b.Users[m.From.ID]
	if !ok {
		return "Вы не подключены к погребу. Попросите владельца добавить ваш Telegram id."
	}
	if cmd.Error != "" {
		return cmd.Error
	}
	var (
		out string
		err error
	)
	switch cmd.Kind {
	case CmdCellar:
		out, err = b.Handler.Cellar(ctx, token)
	case CmdDrink:
		out, err = b.Handler.Drink(ctx, token, cmd.ItemID, cmd.Rating)
	case CmdPair:
		out, err = b.Handler.Ask(ctx, token, "Что открыть к блюду: "+cmd.Text+"?")
	case CmdChat:
		out, err = b.Handler.Ask(ctx, token, cmd.Text)
	default:
		return HelpText
	}
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	return out
}

// Run long-polls until ctx is done.
func (b *Bot) Run(ctx context.Context) error {
	poll := b.Poll
	if poll == 0 {
		poll = 50 * time.Second
	}
	var offset int64
	for ctx.Err() == nil {
		ups, err := b.API.GetUpdates(ctx, offset, poll)
		if err != nil {
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range ups {
			offset = u.UpdateID + 1
			if reply := b.Reply(ctx, u.Message); reply != "" {
				_ = b.API.SendMessage(ctx, u.Message.Chat.ID, reply)
			}
		}
	}
	return ctx.Err()
}

// ParseUsers parses "tgID:token,tgID:token".
func ParseUsers(s string) (map[int64]string, error) {
	out := map[int64]string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		id, tok, ok := strings.Cut(strings.TrimSpace(pair), ":")
		n, err := strconv.ParseInt(id, 10, 64)
		if !ok || err != nil || tok == "" {
			return nil, errors.New("TELEGRAM_USERS: want tgID:token pairs")
		}
		out[n] = tok
	}
	return out, nil
}
