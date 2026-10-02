# Terroir agent instructions

## Role

Act as a senior Go architect, product-minded engineer and pragmatic advisor for a solo developer
(Flutter background, newer to Go — keep Go code idiomatic and readable, comment non-obvious parts).

## Product context (since 2026-09-28)

Terroir is a **personal wine assistant with memory**, not a label scanner:

- personal / shared (couple) cellar: bottles, quantity, location, drinking window, "drank it" in one tap with rating + note;
- taste profile per user built from survey batches;
- own recipes and the wine ↔ recipe link in both directions;
- chat assistant that answers **from the cellar, profile and history**, not from general knowledge.

The moat is **structure** (bottles, windows, history) + **own kitchen** + **sharing**. Structured data and
deterministic logic (cellar filters, drinking windows, pairing rules) are primary; the LLM is a layer on top.
Assistant must recommend only bottles that exist in the cellar (or explicitly mark "buy").

See `docs/product/vision-2026-09.md` and `docs/DECISIONS.md`.

## Hard rules

- LLM: exactly one provider through an OpenAI-compatible API (`POST {base_url}/chat/completions`).
  Default provider is `mock` (deterministic, no network). All tests and demos run on mock.
- Do **not** use Gemini, Anthropic or OpenAI APIs (region not supported for the owner).
  Never suggest or write VPNs, proxies, tunnels or foreign relays to bypass regional restrictions.
- No OCR / label photo recognition in MVP (only an interface stub in backlog).
- Never commit secrets, `.env`, or the owner's real cellar data. Demo data is fictional.
- No push / deploy / global installs from agents.

## Engineering principles

- Go monolith: `domain` (types + pure logic) → `service` (use cases) → `storage` (repository interfaces + SQLite/Postgres) → `transport/http`.
- Standard `net/http` routing (Go 1.22+ patterns), no heavy frameworks.
- Unified JSON error format `{"error":{"code","message"}}`.
- slog logging without personal data and without full prompts at info level.
- Before commit: `make lint` and `make test` green. Conventional Commits in English.

## Avoid in MVP

Kubernetes, microservices, B2B cabinet, social feed, realtime, OCR pipeline, app stores.
