package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type captured struct {
	method, path, auth, extra string
	body                      map[string]any
}

func newServer(t *testing.T, statuses []int, got *captured, calls *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(calls, 1)
		if got != nil {
			got.method, got.path = r.Method, r.URL.Path
			got.auth = r.Header.Get("Authorization")
			got.extra = r.Header.Get("x-data-logging-enabled")
			raw, _ := io.ReadAll(r.Body)
			got.body = map[string]any{}
			_ = json.Unmarshal(raw, &got.body)
		}
		if int(n) <= len(statuses) && statuses[n-1] != http.StatusOK {
			w.WriteHeader(statuses[n-1])
			_, _ = w.Write([]byte(`{"error":"x"}`))
			return
		}
		_, _ = w.Write([]byte(`{"model":"srv-model","choices":[{"message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":11,"completion_tokens":7}}`))
	}))
}

func zeroBackoff(int) time.Duration { return 0 }

func req() Request { return Request{Messages: []Message{{Role: "user", Content: "hi"}}} }

func TestOpenAICompat_RequestShapeBearer(t *testing.T) {
	var got captured
	var calls int32
	srv := newServer(t, nil, &got, &calls)
	defer srv.Close()
	p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m1", APIKey: "k1",
		ExtraHeaders: map[string]string{"x-data-logging-enabled": "false"}})
	resp, err := p.Complete(context.Background(), req())
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/chat/completions" {
		t.Fatalf("got %s %s", got.method, got.path)
	}
	if got.auth != "Bearer k1" {
		t.Fatalf("auth %q", got.auth)
	}
	if got.extra != "false" {
		t.Fatalf("extra header %q", got.extra)
	}
	for _, k := range []string{"reasoning_effort", "temperature", "max_tokens"} {
		if _, ok := got.body[k]; ok {
			t.Fatalf("%s must be omitted by default: %v", k, got.body)
		}
	}
	if got.body["model"] != "m1" {
		t.Fatalf("model %v", got.body["model"])
	}
	if resp.Content != "hello" || resp.Model != "srv-model" || resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 7 {
		t.Fatalf("resp %+v", resp)
	}
	if p.Name() != "openai_compat" || p.Model() != "m1" {
		t.Fatal("name/model")
	}
}

func TestOpenAICompat_ApiKeyAndOptionalParams(t *testing.T) {
	var got captured
	var calls int32
	srv := newServer(t, nil, &got, &calls)
	defer srv.Close()
	temp := 0.3
	p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k2", AuthScheme: "Api-Key",
		ReasoningEffort: "low", Temperature: &temp, MaxTokens: 100})
	if _, err := p.Complete(context.Background(), req()); err != nil {
		t.Fatal(err)
	}
	if got.auth != "Api-Key k2" {
		t.Fatalf("auth %q", got.auth)
	}
	if got.body["reasoning_effort"] != "low" || got.body["temperature"] != 0.3 || got.body["max_tokens"] != float64(100) {
		t.Fatalf("body %v", got.body)
	}
}

func TestOpenAICompat_ModelFallbackWhenServerOmitsIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"x"}}]}`))
	}))
	defer srv.Close()
	resp, err := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "cfg-model", APIKey: "k"}).Complete(context.Background(), req())
	if err != nil || resp.Model != "cfg-model" {
		t.Fatalf("%v %+v", err, resp)
	}
}

func TestOpenAICompat_RetriesThenSuccess(t *testing.T) {
	for _, st := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		var calls int32
		srv := newServer(t, []int{st, st}, nil, &calls)
		p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k", MaxRetries: 2, Backoff: zeroBackoff})
		resp, err := p.Complete(context.Background(), req())
		srv.Close()
		if err != nil || resp.Content != "hello" {
			t.Fatalf("status %d: %v", st, err)
		}
		if calls != 3 {
			t.Fatalf("status %d: calls %d", st, calls)
		}
	}
}

func TestOpenAICompat_NoRetryOnClientErrors(t *testing.T) {
	for _, st := range []int{http.StatusBadRequest, http.StatusUnauthorized} {
		var calls int32
		srv := newServer(t, []int{st, st, st}, nil, &calls)
		p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k", MaxRetries: 3, Backoff: zeroBackoff})
		_, err := p.Complete(context.Background(), req())
		srv.Close()
		var he *HTTPError
		if !errors.As(err, &he) || he.Status != st {
			t.Fatalf("status %d: err %v", st, err)
		}
		if he.Error() == "" {
			t.Fatal("empty error text")
		}
		if calls != 1 {
			t.Fatalf("status %d: calls %d", st, calls)
		}
	}
}

func TestOpenAICompat_RetriesExhausted(t *testing.T) {
	var calls int32
	srv := newServer(t, []int{500, 502, 503, 504}, nil, &calls)
	defer srv.Close()
	p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k", MaxRetries: 2, Backoff: zeroBackoff})
	_, err := p.Complete(context.Background(), req())
	var he *HTTPError
	if err == nil || !errors.As(err, &he) {
		t.Fatalf("want wrapped HTTPError, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("calls %d", calls)
	}
}

func TestOpenAICompat_RetryAfterHeaderParsed(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k", MaxRetries: 0})
	_, err := p.Complete(context.Background(), req())
	var he *HTTPError
	if !errors.As(err, &he) || he.RetryAfter != 30*time.Second {
		t.Fatalf("want RetryAfter capped at 30s, got %v", err)
	}
}

func TestOpenAICompat_ContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k", MaxRetries: 3, Backoff: zeroBackoff})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.Complete(ctx, req())
	if err == nil {
		t.Fatal("want error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("did not respect context: %v", time.Since(start))
	}
}

func TestOpenAICompat_ProviderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	p := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k", Timeout: 50 * time.Millisecond, MaxRetries: 0})
	if _, err := p.Complete(context.Background(), req()); err == nil {
		t.Fatal("want timeout error")
	}
}

func TestOpenAICompat_BadResponses(t *testing.T) {
	for name, body := range map[string]string{"invalid json": `{`, "empty choices": `{"choices":[]}`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		_, err := NewOpenAICompat(CompatConfig{BaseURL: srv.URL, Model: "m", APIKey: "k"}).Complete(context.Background(), req())
		srv.Close()
		if err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
}

func TestOpenAICompat_DefaultBackoff(t *testing.T) {
	p := NewOpenAICompat(CompatConfig{})
	if p.cfg.Backoff(1) != 500*time.Millisecond || p.cfg.Backoff(3) != 2*time.Second {
		t.Fatal("unexpected default backoff")
	}
	if p.cfg.AuthScheme != "Bearer" || p.cfg.Timeout != 30*time.Second {
		t.Fatal("unexpected defaults")
	}
}
