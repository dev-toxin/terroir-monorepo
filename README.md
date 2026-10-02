# Terroir

**Персональный винный помощник с памятью** для себя и пары. В нём есть:

- **погреб**: бутылки, места хранения, окна питья, кнопка «выпил» с оценкой;
- **профиль вкуса** у каждого свой, собирается короткими опросами;
- **свои рецепты** со связкой «вино ↔ блюдо» в обе стороны;
- **чат-ассистент**, который отвечает из *вашего* погреба и истории, а не из общих знаний.

Структурированные данные и детерминированная логика (фильтры, окна питья, правила сочетаний) — основа. LLM работает слоем поверх.
Видение: [`docs/product/vision-2026-09.md`](docs/product/vision-2026-09.md) · статус: [`docs/STATUS.md`](docs/STATUS.md) · решения: [`docs/DECISIONS.md`](docs/DECISIONS.md).

## Быстрый старт за 1 минуту

Нужен только Go ≥ 1.26. Docker не нужен.

```bash
make dev                      # SQLite + демо-данные + mock-LLM на :8080
curl localhost:8080/health    # {"status":"ok","storage":"sqlite","llm_provider":"mock",...}
open http://localhost:8080/app   # PWA; токен: dev-anton-token или dev-elena-token
```

```bash
T="Authorization: Bearer dev-anton-token"
curl -H "$T" localhost:8080/v1/cellar/summary
curl -H "$T" "localhost:8080/v1/recipes/r-tteokbokki/wines?limit=3"
curl -H "$T" -X POST localhost:8080/v1/chat -d '{"message":"Что открыть к стейку?"}'
```

Остальные примеры: [`docs/api/examples.sh`](docs/api/examples.sh). Спецификация: [`docs/api/openapi.yaml`](docs/api/openapi.yaml).

| Команда | Что делает |
|---|---|
| `make dev` | API на SQLite (`tmp/terroir-dev.db`), автосид демо-дома, mock-LLM |
| `make reset-dev` | удалить dev-базу (следующий `make dev` создаст её заново) |
| `make seed` | засеять демо-данные (идемпотентно) |
| `make test` | все тесты с `-race` |
| `make lint` | gofmt + go vet (+ staticcheck, если установлен) |
| `make build` | бинарь `bin/terroir-api` |
| `make up` / `make down` | PostgreSQL + API в Docker |
| `make test-pg` | тесты, включая интеграционные с Postgres (нужен `make up`) |

С телефона в той же Wi-Fi сети: `http://<IP-ноутбука>:8080/app`.

## Архитектура

```text
 PWA (/app, vanilla JS)   curl / будущий Telegram-бот
            │                       │
            ▼                       ▼
   transport/http  ── JSON API v1, Bearer-токен, единый формат ошибок {error:{code,message}}
            │
   service         ── сценарии: погреб, «выпил», опрос, рецепты, сводка, CSV, чат
     │      │   └── pairing  ── детерминированный движок сочетаний (score + причины RU/EN)
     │      └────── llm      ── Provider: mock (по умолчанию) | openai_compat; бюджет; промпты (embed)
     ▼
   domain          ── типы и чистая логика (окна питья, валидация, профиль вкуса)
   storage         ── интерфейсы репозиториев
     └ sqlstore    ── одна реализация на database/sql: SQLite (modernc, без CGO) | PostgreSQL (pgx)
                      миграции встроены (embed) и применяются при старте
```

Монолит на Go, стандартный `net/http` (маршруты Go 1.22+), без фреймворков.

## Переменные окружения

Полный список — в [`.env.example`](.env.example). Главные:

| Переменная | По умолчанию | Смысл |
|---|---|---|
| `STORAGE` | `sqlite` (или `postgres`, если задан `DATABASE_URL`) | `sqlite` \| `postgres` \| `memory` |
| `SQLITE_PATH` | `terroir-dev.db` | файл SQLite |
| `DATABASE_URL` | — | строка подключения PostgreSQL |
| `AUTO_SEED` | `false` (`true` в `make dev`) | создать демо-дом при старте |
| `AUTH_TOKENS` | — | `user_id:token,...` — дополнительные токены |
| `TERROIR_TODAY` | — | зафиксировать «сегодня» (YYYY-MM-DD) для демо |
| `LLM_PROVIDER` | `mock` | `mock` \| `openai_compat` |
| `LLM_BASE_URL`, `LLM_MODEL`, `LLM_API_KEY` | — | для `openai_compat` |
| `LLM_AUTH_SCHEME` | `Bearer` | `Bearer` \| `Api-Key` |
| `LLM_EXTRA_HEADERS` | — | `name:value,...`, только из белого списка |
| `LLM_REASONING_EFFORT` | — | не отправляется, если пусто |
| `LLM_TIMEOUT`, `LLM_MAX_RETRIES` | `30s`, `2` | ретраи на 429/5xx с backoff |
| `LLM_MONTHLY_BUDGET`, `LLM_MONTHLY_TOKENS` | `0` (без лимита) | месячный лимит; при превышении чат отвечает `429 llm_budget_exceeded` |

## Как переключить mock на настоящий провайдер

Нужен любой провайдер с OpenAI-совместимым `POST {base_url}/chat/completions`, доступный легально в вашем регионе. Пример — Yandex AI Studio:

```bash
LLM_PROVIDER=openai_compat \
LLM_BASE_URL=https://llm.api.cloud.yandex.net/v1 \
LLM_MODEL="gpt://<FOLDER_ID>/yandexgpt/latest" \
LLM_API_KEY=<ключ> \
LLM_AUTH_SCHEME=Api-Key \
LLM_EXTRA_HEADERS=x-data-logging-enabled:false \
LLM_MONTHLY_BUDGET=500 LLM_PRICE_PER_1K_INPUT=0.4 LLM_PRICE_PER_1K_OUTPUT=0.4 \
make dev
```

`/health` покажет `llm_provider: openai_compat`. Если модель сошлётся на бутылку, которой нет в погребе, или провайдер упадёт, ассистент ответит детерминированно (`"fallback": true`).
> Gemini, Anthropic и OpenAI в проекте не используются: регион не поддерживается. Прокси, VPN и туннели для обхода — тоже нет.

## Как запустить на PostgreSQL

```bash
make up                       # pgvector/pg16 + API (STORAGE=postgres, автосид)
curl localhost:8080/ready     # storage: postgres, database: ok
make test-pg                  # интеграционные тесты репозитория на реальном Postgres
```

Или без Docker, со своим Postgres: `STORAGE=postgres DATABASE_URL=postgres://... AUTO_SEED=true go run ./cmd/api` (из `backend/`).

## Структура

```text
backend/    Go API (cmd/api, internal/*), миграции в internal/storage/sqlstore/migrations
docs/       видение, статус, решения, API, архитектура, эталонные вопросы, отчёты сессий
infra/      init-скрипты PostgreSQL
```

`.env` в git не коммитится (шаблон — `.env.example`). Демо-данные вымышленные.
