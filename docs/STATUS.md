# Terroir — статус (2026-10-02)

Видение: [`product/vision-2026-09.md`](product/vision-2026-09.md). Решения: [`DECISIONS.md`](DECISIONS.md).

## Функции

| Функция | Статус | Где в коде | Как проверить |
|---|---|---|---|
| Запуск без Docker одной командой | ✅ готово | `Makefile`, `backend/cmd/api`, `internal/app` | `make dev` → `curl localhost:8080/health` |
| SQLite-хранилище (dev/тесты), `memory` | ✅ готово | `internal/storage/sqlstore` | `make test` |
| PostgreSQL-хранилище | 🟡 частично: код и миграции есть, интеграционные тесты **не запускались** (Docker выключен) | `sqlstore/db.go`, `migrations/postgres` | `make up && make test-pg` |
| Встроенные миграции при старте | ✅ | `sqlstore.migrate` | `TestMigrationsIdempotent` |
| Совместный погреб пары (дом + роли) | ✅ | `domain/cellar.go`, `cellar_members` | `TestSharedCellarSeparateProfiles` |
| Вина CRUD | ✅ | `service/cellar.go`, `/v1/wines` | `TestWinesCRUD` |
| Бутылки, фильтры (цвет, страна, сорт, место, q, «пора пить») | ✅ | `/v1/cellar/items` | `TestCellarItemsAndFilters` |
| Окна питья (рано / пора / пик / последний год / поздно) | ✅ | `domain.DrinkingWindow` | `TestDrinkingWindow` |
| «Выпил» атомарно, без ухода в минус, строка с нулём остаётся | ✅ | `sqlstore.Drink` | `TestConcurrentDrinkLastBottle`, `TestDrinkLastBottleRace` |
| Сводка погреба + «открыть скоро» | ✅ | `service.Summary`, `/v1/cellar/summary` | `TestEndToEndAddDrinkSummary` |
| Опрос вкуса пачками 5–7, детерминированное обновление профиля | ✅ (15 вопросов) | `domain/survey_bank.go`, `domain/taste.go` | `TestSurveyBatches` |
| Рецепты CRUD | ✅ | `/v1/recipes` | `TestRecipesAndPairing` |
| Движок сочетаний (score + причины RU/EN, профиль, окно, история) | ✅ | `internal/pairing` | `go test ./internal/pairing` (19 кейсов), `docs/architecture/pairing.md` |
| «Что открыть к рецепту» / «Что приготовить к бутылке» | ✅ | `/v1/recipes/{id}/wines`, `/v1/cellar/items/{id}/recipes` | examples.sh |
| Обучение на удачных парах (pairing_feedback, «выпил» с рецептом) | ✅ | `applyFeedback` | `TestDrinkWithRecipeTeachesPairing` |
| Чат-ассистент из погреба, валидация ссылок, детерминированный фолбэк | ✅ на mock | `service/assistant.go`, `service/intent.go` | `TestAssistant*`, `POST /v1/chat` |
| Память (memory_facts) | 🟡 частично: извлечение по шаблонам «не люблю / обожаю …», без LLM-суммаризации | `service.ExtractFact` | `TestAssistantMemoryAndLocale` |
| LLM: mock (по умолчанию) | ✅ | `llm/mock.go` | `/health` → `llm_provider: mock` |
| LLM: OpenAI-совместимый провайдер (Bearer/Api-Key, заголовки, ретраи, таймаут) | 🟡 протестирован на httptest, **без реального ключа** | `llm/openaicompat.go` | `go test ./internal/llm` |
| Месячный бюджет LLM, учёт в llm_usage | ✅ | `llm/budget.go` | `TestChatBudgetExceeded` |
| CSV экспорт/импорт | ✅ | `service/csv.go` | `TestCSVExportImport` |
| Импорт из Markdown-таблицы | ✅ | `service/markdown_import.go`, `/v1/import.md` | `markdown_import_test.go` |
| «Купить снова» (оценки ≥4) | ✅ | `service.BuyAgain`, `/v1/cellar/buy-again` | `TestBuyAgain` |
| Напоминания «пора открыть» (выборка, без рассылки) | 🟡 только выборка | `service.Reminders`, `/v1/reminders` | `TestReadyFilterAndSummary` |
| Эталонные вопросы для слепого сравнения | ✅ 10/10 на mock | `internal/eval`, `docs/eval/blind-questions.md` | `go test -v ./internal/eval` |
| PWA `/app` | 🟡 погреб, «выпил», карточка, рецепты, опрос, чат, RU/EN; **нет форм добавления вин/рецептов**; ручной клик-тест не делался | `internal/webapp/static` | открыть `http://localhost:8080/app` |
| Telegram-бот | 🟡 каркас (клиент, разбор команд, адаптер), не подключён в `main` | `internal/telegram` | `go test ./internal/telegram` |
| Фото этикетки → карточка | ❌ вне MVP, только интерфейс | `llm/label.go` | — |
| Dockerfile (distroless), compose без `go run` | 🟡 написан, не собирался (Docker выключен) | `backend/Dockerfile`, `docker-compose.yml` | `make up` |
| CI (GitHub Actions) | 🟡 файл создан, не запускался | `.github/workflows/ci.yml` | push в GitHub |
| Регистрация, пароли, OAuth | ❌ сознательно нет: статические токены (D-007) | `service.Authenticate` | — |

## Покрытие тестами (go test -cover)

domain 96% · pairing 98% · llm 97% · config 99% · service 86% · telegram 94% · http 82% · sqlstore 79% · eval 100%.

## Что устарело

Документы эпохи сканера (`docs/product/*`, `docs/architecture/ai-pipeline.md`, `docs/terroir_features.md`) помечены баннером «Устарело с 2026-09-28». Промпты сканера в `docs/prompts/` помечены как archived. `docs/architecture/{backend,database,system-overview,...}.md` ещё не переписаны — верить коду и README.
