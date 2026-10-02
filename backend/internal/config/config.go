// Package config loads Terroir settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Storage kinds.
const (
	StorageSQLite   = "sqlite"
	StoragePostgres = "postgres"
	StorageMemory   = "memory"
)

// LLM holds settings of the single OpenAI-compatible provider.
type LLM struct {
	Provider        string            // mock | openai_compat
	BaseURL         string            // e.g. https://llm.api.cloud.yandex.net/v1
	Model           string            // e.g. gpt://<FOLDER_ID>/yandexgpt/latest
	APIKey          string            // never logged
	AuthScheme      string            // Bearer | Api-Key
	ExtraHeaders    map[string]string // whitelisted, e.g. x-data-logging-enabled: false
	ReasoningEffort string            // sent only when non-empty
	Temperature     *float64
	MaxTokens       int
	Timeout         time.Duration
	MaxRetries      int
	MonthlyTokens   int64 // 0 = unlimited
	MonthlyBudget   int64 // money, in micros of budget currency; 0 = unlimited
	PricePer1KIn    int64 // micros per 1K prompt tokens
	PricePer1KOut   int64 // micros per 1K completion tokens
	ContextTokens   int   // max tokens of assembled context
}

// Config is the full app config.
type Config struct {
	AppEnv      string
	HTTPAddr    string
	Storage     string
	DatabaseURL string
	SQLitePath  string
	AutoSeed    bool
	AuthTokens  map[string]string // token -> user id
	FixedDate   *time.Time        // TERROIR_TODAY=YYYY-MM-DD for demos/tests
	LLM         LLM
}

// AllowedExtraHeaders is the whitelist for LLM_EXTRA_HEADERS. Anything else is rejected
// so that a typo cannot override Authorization or leak data to the provider.
var AllowedExtraHeaders = map[string]bool{
	"x-data-logging-enabled": true, // Yandex AI Studio: disable request logging
	"x-folder-id":            true, // Yandex: folder for billing
	"openai-project":         true,
	"openai-organization":    true,
}

// Load reads configuration from the environment.
func Load() (Config, error) { return LoadFrom(os.Getenv) }

// LoadFrom reads configuration using the given lookup (testable).
func LoadFrom(getenv func(string) string) (Config, error) {
	get := func(k, def string) string {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			return v
		}
		return def
	}
	cfg := Config{
		AppEnv:      get("APP_ENV", "local"),
		HTTPAddr:    get("BACKEND_HTTP_ADDR", ":8080"),
		DatabaseURL: get("DATABASE_URL", ""),
		SQLitePath:  get("SQLITE_PATH", "terroir-dev.db"),
	}
	cfg.Storage = get("STORAGE", "")
	if cfg.Storage == "" {
		if cfg.DatabaseURL != "" {
			cfg.Storage = StoragePostgres
		} else {
			cfg.Storage = StorageSQLite
		}
	}
	switch cfg.Storage {
	case StorageSQLite, StorageMemory:
	case StoragePostgres:
		if cfg.DatabaseURL == "" {
			return cfg, errors.New("DATABASE_URL is required when STORAGE=postgres")
		}
	default:
		return cfg, fmt.Errorf("STORAGE must be sqlite, postgres or memory, got %q", cfg.Storage)
	}
	var err error
	if cfg.AutoSeed, err = parseBool(get("AUTO_SEED", "false")); err != nil {
		return cfg, fmt.Errorf("AUTO_SEED: %w", err)
	}
	if cfg.AuthTokens, err = parseTokens(get("AUTH_TOKENS", "")); err != nil {
		return cfg, err
	}
	if d := get("TERROIR_TODAY", ""); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return cfg, fmt.Errorf("TERROIR_TODAY must be YYYY-MM-DD: %w", err)
		}
		t = t.Add(12 * time.Hour)
		cfg.FixedDate = &t
	}
	cfg.LLM, err = loadLLM(get)
	return cfg, err
}

func loadLLM(get func(string, string) string) (LLM, error) {
	l := LLM{
		Provider:        strings.ToLower(get("LLM_PROVIDER", "mock")),
		BaseURL:         strings.TrimRight(get("LLM_BASE_URL", ""), "/"),
		Model:           get("LLM_MODEL", ""),
		APIKey:          get("LLM_API_KEY", ""),
		AuthScheme:      get("LLM_AUTH_SCHEME", "Bearer"),
		ReasoningEffort: get("LLM_REASONING_EFFORT", ""),
	}
	var err error
	if l.ExtraHeaders, err = parseHeaders(get("LLM_EXTRA_HEADERS", "")); err != nil {
		return l, err
	}
	if v := get("LLM_TEMPERATURE", ""); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 || f > 2 {
			return l, errors.New("LLM_TEMPERATURE must be a number 0..2")
		}
		l.Temperature = &f
	}
	ints := []struct {
		key string
		def string
		dst *int
	}{
		{"LLM_MAX_TOKENS", "800", &l.MaxTokens},
		{"LLM_MAX_RETRIES", "2", &l.MaxRetries},
		{"LLM_CONTEXT_TOKENS", "3000", &l.ContextTokens},
	}
	for _, it := range ints {
		n, err := strconv.Atoi(get(it.key, it.def))
		if err != nil || n < 0 {
			return l, fmt.Errorf("%s must be a non-negative integer", it.key)
		}
		*it.dst = n
	}
	if l.Timeout, err = time.ParseDuration(get("LLM_TIMEOUT", "30s")); err != nil {
		return l, fmt.Errorf("LLM_TIMEOUT: %w", err)
	}
	if l.MonthlyTokens, err = parseInt64(get("LLM_MONTHLY_TOKENS", "0"), "LLM_MONTHLY_TOKENS"); err != nil {
		return l, err
	}
	if l.MonthlyBudget, err = parseMoneyMicros(get("LLM_MONTHLY_BUDGET", "0")); err != nil {
		return l, fmt.Errorf("LLM_MONTHLY_BUDGET: %w", err)
	}
	if l.PricePer1KIn, err = parseMoneyMicros(get("LLM_PRICE_PER_1K_INPUT", "0")); err != nil {
		return l, fmt.Errorf("LLM_PRICE_PER_1K_INPUT: %w", err)
	}
	if l.PricePer1KOut, err = parseMoneyMicros(get("LLM_PRICE_PER_1K_OUTPUT", "0")); err != nil {
		return l, fmt.Errorf("LLM_PRICE_PER_1K_OUTPUT: %w", err)
	}
	switch l.AuthScheme {
	case "Bearer", "Api-Key":
	default:
		return l, errors.New("LLM_AUTH_SCHEME must be Bearer or Api-Key")
	}
	switch l.Provider {
	case "mock":
	case "openai_compat", "openai-compatible", "openaicompat":
		l.Provider = "openai_compat"
		if l.BaseURL == "" || l.Model == "" || l.APIKey == "" {
			return l, errors.New("LLM_BASE_URL, LLM_MODEL and LLM_API_KEY are required for LLM_PROVIDER=openai_compat")
		}
	default:
		return l, fmt.Errorf("LLM_PROVIDER must be mock or openai_compat, got %q", l.Provider)
	}
	return l, nil
}

func parseBool(s string) (bool, error) { return strconv.ParseBool(s) }

func parseInt64(s, name string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return n, nil
}

// parseMoneyMicros parses "12.5" into 12_500_000 micros.
func parseMoneyMicros(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, errors.New("must be a non-negative number")
	}
	return int64(f*1_000_000 + 0.5), nil
}

// parseHeaders parses "k1:v1,k2:v2" and enforces the whitelist.
func parseHeaders(s string) (map[string]string, error) {
	out := map[string]string{}
	if s == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(pair, ":")
		k = strings.ToLower(strings.TrimSpace(k))
		if !ok || k == "" {
			return nil, fmt.Errorf("LLM_EXTRA_HEADERS: bad pair %q (want name:value)", pair)
		}
		if !AllowedExtraHeaders[k] {
			return nil, fmt.Errorf("LLM_EXTRA_HEADERS: header %q is not in the whitelist", k)
		}
		out[k] = strings.TrimSpace(v)
	}
	return out, nil
}

// parseTokens parses "user_id:token,user_id:token" into token -> user id.
func parseTokens(s string) (map[string]string, error) {
	out := map[string]string{}
	if s == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		u, t, ok := strings.Cut(strings.TrimSpace(pair), ":")
		if !ok || u == "" || len(t) < 8 {
			return nil, errors.New("AUTH_TOKENS: want user_id:token pairs, token at least 8 chars")
		}
		out[t] = u
	}
	return out, nil
}
