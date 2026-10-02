package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// CompatConfig configures an OpenAI-compatible provider.
type CompatConfig struct {
	BaseURL         string // without trailing slash, e.g. https://llm.api.cloud.yandex.net/v1
	Model           string
	APIKey          string
	AuthScheme      string            // "Bearer" or "Api-Key"
	ExtraHeaders    map[string]string // already whitelisted by config
	ReasoningEffort string            // omitted when empty
	Temperature     *float64          // omitted when nil
	MaxTokens       int               // omitted when 0
	Timeout         time.Duration
	MaxRetries      int
	// Backoff returns the delay before retry attempt n (1-based). Defaults to 500ms * 2^(n-1).
	Backoff func(n int) time.Duration
	Client  *http.Client
}

// OpenAICompat calls POST {base_url}/chat/completions.
type OpenAICompat struct {
	cfg CompatConfig
}

// NewOpenAICompat creates the provider.
func NewOpenAICompat(cfg CompatConfig) *OpenAICompat {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Backoff == nil {
		cfg.Backoff = func(n int) time.Duration { return time.Duration(500*(1<<(n-1))) * time.Millisecond }
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: cfg.Timeout}
	}
	if cfg.AuthScheme == "" {
		cfg.AuthScheme = "Bearer"
	}
	return &OpenAICompat{cfg: cfg}
}

// Name implements Provider.
func (p *OpenAICompat) Name() string { return "openai_compat" }

// Model implements Provider.
func (p *OpenAICompat) Model() string { return p.cfg.Model }

type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Temperature     *float64  `json:"temperature,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

// HTTPError is a non-2xx answer.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("llm http %d: %s", e.Status, e.Body) }

func retryable(status int) bool { return status == http.StatusTooManyRequests || status >= 500 }

// Complete implements Provider with retries on 429/5xx and network errors.
func (p *OpenAICompat) Complete(ctx context.Context, req Request) (Response, error) {
	body, err := json.Marshal(chatRequest{
		Model: p.cfg.Model, Messages: req.Messages, Temperature: p.cfg.Temperature,
		MaxTokens: p.cfg.MaxTokens, ReasoningEffort: p.cfg.ReasoningEffort,
	})
	if err != nil {
		return Response{}, err
	}
	var lastErr error
	for attempt := 0; attempt <= p.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := p.cfg.Backoff(attempt)
			var he *HTTPError
			if errors.As(lastErr, &he) && he.Status == http.StatusTooManyRequests {
				delay = max(delay, retryAfter(he))
			}
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(delay):
			}
		}
		resp, err := p.once(ctx, body)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		var he *HTTPError
		if errors.As(err, &he) && !retryable(he.Status) {
			return Response{}, err
		}
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
	}
	return Response{}, fmt.Errorf("llm: giving up after %d attempts: %w", p.cfg.MaxRetries+1, lastErr)
}

func retryAfter(*HTTPError) time.Duration { return 0 }

func (p *OpenAICompat) once(ctx context.Context, body []byte) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", p.cfg.AuthScheme+" "+p.cfg.APIKey)
	for k, v := range p.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}
	res, err := p.cfg.Client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Response{}, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		snippet := string(raw)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return Response{}, &HTTPError{Status: res.StatusCode, Body: snippet}
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Response{}, fmt.Errorf("llm: decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return Response{}, errors.New("llm: empty choices")
	}
	model := cr.Model
	if model == "" {
		model = p.cfg.Model
	}
	return Response{Content: cr.Choices[0].Message.Content, Usage: cr.Usage, Model: model}, nil
}

var _ = strconv.Itoa
