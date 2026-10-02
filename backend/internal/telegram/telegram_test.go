package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/llm"
	"github.com/anko-soft-lab/terroir/backend/internal/seed"
	"github.com/anko-soft-lab/terroir/backend/internal/service"
	"github.com/anko-soft-lab/terroir/backend/internal/storage/sqlstore"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		in   string
		want Command
	}{
		{"", Command{Kind: CmdUnknown, Error: "empty message"}},
		{"/help", Command{Kind: CmdHelp}},
		{"/start", Command{Kind: CmdHelp}},
		{"/cellar", Command{Kind: CmdCellar}},
		{"/cellar@TerroirBot", Command{Kind: CmdCellar}},
		{"/CELLAR", Command{Kind: CmdCellar}},
		{"/drink i-tsimla", Command{Kind: CmdDrink, ItemID: "i-tsimla"}},
		{"/drink i-tsimla 5", Command{Kind: CmdDrink, ItemID: "i-tsimla", Rating: 5}},
		{"/drink", Command{Kind: CmdDrink, Error: "usage: /drink <item_id> [rating 1-5]"}},
		{"/drink i-x 9", Command{Kind: CmdDrink, ItemID: "i-x", Error: "rating must be 1..5"}},
		{"/drink i-x abc", Command{Kind: CmdDrink, ItemID: "i-x", Error: "rating must be 1..5"}},
		{"/drink i-x 4 extra", Command{Kind: CmdDrink, ItemID: "i-x", Rating: 4, Error: "usage: /drink <item_id> [rating 1-5]"}},
		{"/pair свиная шея на углях", Command{Kind: CmdPair, Text: "свиная шея на углях"}},
		{"/pair", Command{Kind: CmdPair, Error: "usage: /pair <dish or recipe>"}},
		{"  что пора выпить?  ", Command{Kind: CmdChat, Text: "что пора выпить?"}},
		{"/weird", Command{Kind: CmdUnknown, Error: "unknown command /weird"}},
	}
	for _, c := range cases {
		if got := ParseCommand(c.in); got != c.want {
			t.Errorf("%q: got %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestClientHTTP(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":7,"message":{"message_id":1,"from":{"id":42},"chat":{"id":99},"text":"/cellar"}}]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			_ = json.Unmarshal(body, &sent)
			_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
		default:
			_, _ = w.Write([]byte(`{"ok":false,"description":"Not Found"}`))
		}
	}))
	defer srv.Close()

	if _, err := NewClient("", ""); err == nil {
		t.Fatal("empty token accepted")
	}
	c, err := NewClient(srv.URL+"/", "TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	ups, err := c.GetUpdates(context.Background(), 0, time.Second)
	if err != nil || len(ups) != 1 || ups[0].Message.Text != "/cellar" || ups[0].Message.From.ID != 42 {
		t.Fatalf("updates: %v %+v", err, ups)
	}
	if err := c.SendMessage(context.Background(), 99, "привет"); err != nil {
		t.Fatal(err)
	}
	if sent["text"] != "привет" || sent["chat_id"].(float64) != 99 {
		t.Fatalf("sent: %v", sent)
	}
	if paths[0] != "/botTOKEN/getUpdates" {
		t.Fatalf("path: %s", paths[0])
	}
	if err := c.call(context.Background(), "nope", map[string]any{}, nil); err == nil || !strings.Contains(err.Error(), "Not Found") {
		t.Fatalf("api error: %v", err)
	}
	bad := &Client{BaseURL: "http://127.0.0.1:1", Token: "SECRET", HTTP: &http.Client{Timeout: time.Second}}
	if err := bad.SendMessage(context.Background(), 1, "x"); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("network error must not leak token: %v", err)
	}
	if def, _ := NewClient("", "T"); def.BaseURL != DefaultBaseURL {
		t.Fatal("default base url")
	}
}

type fakeHandler struct{ calls []string }

func (f *fakeHandler) Cellar(_ context.Context, tok string) (string, error) {
	f.calls = append(f.calls, "cellar:"+tok)
	return "list", nil
}
func (f *fakeHandler) Drink(_ context.Context, tok, id string, r int) (string, error) {
	if id == "boom" {
		return "", errors.New("no bottles left")
	}
	f.calls = append(f.calls, "drink:"+id)
	return "ok", nil
}
func (f *fakeHandler) Ask(_ context.Context, tok, text string) (string, error) {
	f.calls = append(f.calls, "ask:"+text)
	return "answer", nil
}

func TestBotReply(t *testing.T) {
	h := &fakeHandler{}
	b := &Bot{Handler: h, Users: map[int64]string{1: "tok"}}
	ctx := context.Background()
	msg := func(from int64, text string) *Message { return &Message{From: &User{ID: from}, Text: text} }
	if b.Reply(ctx, nil) != "" || b.Reply(ctx, &Message{Text: "x"}) != "" {
		t.Fatal("nil message")
	}
	if b.Reply(ctx, msg(2, "/help")) != HelpText {
		t.Fatal("help for unknown user")
	}
	if !strings.Contains(b.Reply(ctx, msg(2, "/cellar")), "не подключены") {
		t.Fatal("unknown user")
	}
	if b.Reply(ctx, msg(1, "/cellar")) != "list" || b.Reply(ctx, msg(1, "/drink i-1 4")) != "ok" ||
		b.Reply(ctx, msg(1, "/pair стейк")) != "answer" || b.Reply(ctx, msg(1, "привет")) != "answer" {
		t.Fatal("routing")
	}
	if !strings.Contains(b.Reply(ctx, msg(1, "/drink")), "usage") || !strings.Contains(b.Reply(ctx, msg(1, "/drink boom")), "Ошибка") {
		t.Fatal("errors")
	}
	if b.Reply(ctx, msg(1, "/weird")) == "" {
		t.Fatal("unknown command")
	}
	if len(h.calls) != 4 || h.calls[2] != "ask:Что открыть к блюду: стейк?" {
		t.Fatalf("calls: %v", h.calls)
	}
}

type fakeAPI struct {
	mu   sync.Mutex
	sent []string
	done context.CancelFunc
	n    int
}

func (f *fakeAPI) GetUpdates(ctx context.Context, offset int64, _ time.Duration) ([]Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	if f.n == 1 {
		return []Update{{UpdateID: 1, Message: &Message{From: &User{ID: 1}, Chat: Chat{ID: 5}, Text: "/help"}}}, nil
	}
	f.done()
	return nil, ctx.Err()
}

func (f *fakeAPI) SendMessage(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}

func TestBotRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	api := &fakeAPI{done: cancel}
	b := &Bot{API: api, Handler: &fakeHandler{}, Users: map[int64]string{}}
	if err := b.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("run: %v", err)
	}
	if len(api.sent) != 1 || api.sent[0] != HelpText {
		t.Fatalf("sent: %v", api.sent)
	}
}

func TestParseUsers(t *testing.T) {
	m, err := ParseUsers("123:dev-anton-token, 456:dev-elena-token")
	if err != nil || m[123] != "dev-anton-token" || m[456] != "dev-elena-token" {
		t.Fatalf("%v %v", err, m)
	}
	if m, _ := ParseUsers(""); len(m) != 0 {
		t.Fatal("empty")
	}
	for _, bad := range []string{"abc:tok", "123", "123:"} {
		if _, err := ParseUsers(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestServiceHandler(t *testing.T) {
	ctx := context.Background()
	st, err := sqlstore.OpenSQLite(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	if err := seed.Run(ctx, st, now); err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, domain.FixedClock{T: now}, sqlstore.NewID)
	h := ServiceHandler{Svc: svc, Assistant: service.NewAssistant(svc, llm.Mock{}, nil, 0, slog.New(slog.NewTextHandler(io.Discard, nil)))}
	out, err := h.Cellar(ctx, seed.TokenAnton)
	if err != nil || !strings.Contains(out, "/drink i-") {
		t.Fatalf("cellar: %v %q", err, out)
	}
	out, err = h.Drink(ctx, seed.TokenAnton, "i-tsimla", 4)
	if err != nil || !strings.Contains(out, "2") {
		t.Fatalf("drink: %v %q", err, out)
	}
	out, err = h.Ask(ctx, seed.TokenElena, "Что открыть к стейку?")
	if err != nil || !strings.Contains(out, "[item:") {
		t.Fatalf("ask: %v %q", err, out)
	}
	for _, f := range []func() error{
		func() error { _, err := h.Cellar(ctx, "bad"); return err },
		func() error { _, err := h.Drink(ctx, "bad", "x", 0); return err },
		func() error { _, err := h.Ask(ctx, "bad", "x"); return err },
		func() error { _, err := h.Drink(ctx, seed.TokenAnton, "nope", 0); return err },
	} {
		if f() == nil {
			t.Fatal("expected error")
		}
	}
}
