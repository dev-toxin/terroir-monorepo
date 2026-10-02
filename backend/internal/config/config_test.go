package config

import (
	"strings"
	"testing"
	"time"
)

func env(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }

func TestDefaults(t *testing.T) {
	c, err := LoadFrom(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Storage != StorageSQLite || c.HTTPAddr != ":8080" || c.AppEnv != "local" || c.SQLitePath == "" {
		t.Fatalf("%+v", c)
	}
	if c.LLM.Provider != "mock" || c.LLM.AuthScheme != "Bearer" || c.LLM.Timeout != 30*time.Second || c.LLM.MaxRetries != 2 {
		t.Fatalf("%+v", c.LLM)
	}
	if c.LLM.ReasoningEffort != "" || c.LLM.Temperature != nil || c.LLM.MonthlyBudget != 0 || c.AutoSeed || c.FixedDate != nil {
		t.Fatalf("%+v", c)
	}
}

func TestStorage(t *testing.T) {
	if _, err := LoadFrom(env(map[string]string{"STORAGE": "postgres"})); err == nil {
		t.Fatal("postgres without DATABASE_URL must fail")
	}
	c, err := LoadFrom(env(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil || c.Storage != StoragePostgres {
		t.Fatalf("DATABASE_URL implies postgres: %v %s", err, c.Storage)
	}
	c, err = LoadFrom(env(map[string]string{"STORAGE": "memory"}))
	if err != nil || c.Storage != StorageMemory {
		t.Fatal(err)
	}
	if _, err := LoadFrom(env(map[string]string{"STORAGE": "mongo"})); err == nil {
		t.Fatal("unknown storage must fail")
	}
}

func TestExtraHeaders(t *testing.T) {
	c, err := LoadFrom(env(map[string]string{"LLM_EXTRA_HEADERS": "X-Data-Logging-Enabled: false, x-folder-id:abc"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.ExtraHeaders["x-data-logging-enabled"] != "false" || c.LLM.ExtraHeaders["x-folder-id"] != "abc" {
		t.Fatalf("%v", c.LLM.ExtraHeaders)
	}
	for _, bad := range []string{"Authorization:Bearer x", "x-evil:1", "novalue", ":v"} {
		if _, err := LoadFrom(env(map[string]string{"LLM_EXTRA_HEADERS": bad})); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}
}

func TestAuthSchemeAndProvider(t *testing.T) {
	if _, err := LoadFrom(env(map[string]string{"LLM_AUTH_SCHEME": "Basic"})); err == nil {
		t.Fatal("Basic must be rejected")
	}
	c, err := LoadFrom(env(map[string]string{"LLM_AUTH_SCHEME": "Api-Key"}))
	if err != nil || c.LLM.AuthScheme != "Api-Key" {
		t.Fatal(err)
	}
	if _, err := LoadFrom(env(map[string]string{"LLM_PROVIDER": "openai_compat", "LLM_BASE_URL": "https://h/v1", "LLM_MODEL": "m"})); err == nil {
		t.Fatal("missing key must fail")
	}
	if _, err := LoadFrom(env(map[string]string{"LLM_PROVIDER": "gemini"})); err == nil {
		t.Fatal("unknown provider must fail")
	}
	c, err = LoadFrom(env(map[string]string{"LLM_PROVIDER": "OpenAI-Compatible", "LLM_BASE_URL": "https://llm.api.cloud.yandex.net/v1/",
		"LLM_MODEL": "gpt://f/yandexgpt/latest", "LLM_API_KEY": "secret", "LLM_REASONING_EFFORT": "low", "LLM_TEMPERATURE": "0.2"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.Provider != "openai_compat" || c.LLM.BaseURL != "https://llm.api.cloud.yandex.net/v1" || c.LLM.ReasoningEffort != "low" || *c.LLM.Temperature != 0.2 {
		t.Fatalf("%+v", c.LLM)
	}
}

func TestNumbers(t *testing.T) {
	c, err := LoadFrom(env(map[string]string{"LLM_MONTHLY_BUDGET": "12.5", "LLM_PRICE_PER_1K_INPUT": "0.4",
		"LLM_MONTHLY_TOKENS": "100000", "LLM_TIMEOUT": "5s", "LLM_MAX_RETRIES": "0"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM.MonthlyBudget != 12_500_000 || c.LLM.PricePer1KIn != 400_000 || c.LLM.MonthlyTokens != 100000 ||
		c.LLM.Timeout != 5*time.Second || c.LLM.MaxRetries != 0 {
		t.Fatalf("%+v", c.LLM)
	}
	for k, v := range map[string]string{"LLM_MONTHLY_BUDGET": "-1", "LLM_MONTHLY_TOKENS": "x", "LLM_TIMEOUT": "5",
		"LLM_MAX_TOKENS": "-3", "LLM_TEMPERATURE": "3", "AUTO_SEED": "maybe", "LLM_PRICE_PER_1K_OUTPUT": "abc", "LLM_PRICE_PER_1K_INPUT": "-2"} {
		if _, err := LoadFrom(env(map[string]string{k: v})); err == nil {
			t.Fatalf("%s=%s must fail", k, v)
		}
	}
}

func TestAuthTokensAndToday(t *testing.T) {
	c, err := LoadFrom(env(map[string]string{"AUTH_TOKENS": "u-anton:longtoken1, u-elena:longtoken2", "TERROIR_TODAY": "2026-10-02", "AUTO_SEED": "true"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthTokens["longtoken1"] != "u-anton" || c.AuthTokens["longtoken2"] != "u-elena" || !c.AutoSeed {
		t.Fatalf("%v", c.AuthTokens)
	}
	if c.FixedDate == nil || c.FixedDate.Format("2006-01-02") != "2026-10-02" {
		t.Fatalf("%v", c.FixedDate)
	}
	for _, bad := range []string{"u:short", "notoken", ":longtoken1"} {
		if _, err := LoadFrom(env(map[string]string{"AUTH_TOKENS": bad})); err == nil {
			t.Fatalf("%q must fail", bad)
		}
	}
	if _, err := LoadFrom(env(map[string]string{"TERROIR_TODAY": "02.10.2026"})); err == nil || !strings.Contains(err.Error(), "TERROIR_TODAY") {
		t.Fatal("bad date must fail")
	}
}
