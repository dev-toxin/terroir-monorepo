# Terroir features catalog

## Резюме

На текущий момент репозиторий содержит foundation-слой будущего приложения Terroir: документацию продукта/архитектуры, self-hosted PostgreSQL + pgvector, Go backend skeleton и начальную SQL-схему. Пользовательские mobile-фичи еще не реализованы в UI, но домены MVP зафиксированы в документации и миграциях.

## Таблица фич

| Категория | Фича | Что умеет сейчас | Evidence-файлы | Уверенность |
|---|---|---|---|---|
| Infrastructure | Self-hosted PostgreSQL | Локальный PostgreSQL 16 с pgvector через Docker Compose | `docker-compose.yml`, `infra/postgres/init/001_extensions.sql` | High |
| Backend | Go API skeleton | Запускает HTTP-сервер, читает env, подключается к PostgreSQL | `backend/cmd/api/main.go`, `backend/internal/config/config.go`, `backend/internal/database/postgres.go` | High |
| Backend | Healthcheck | `GET /health` возвращает статус процесса | `backend/internal/transport/http/router.go` | High |
| Backend | Readiness check | `GET /ready` проверяет доступность PostgreSQL | `backend/internal/transport/http/router.go` | High |
| Database | Initial wine domain schema | Есть таблицы users, wines, cellars, cellar_items, label_scans | `backend/migrations/000001_initial_schema.sql` | High |
| AI foundation | Label scan storage model | Таблица хранит OCR/LLM provider, raw OCR text, parsed JSON, статус | `backend/migrations/000001_initial_schema.sql` | Medium |
| AI foundation | Wine vector profile | В `wines` есть `tasting_embedding VECTOR(1536)` | `backend/migrations/000001_initial_schema.sql` | Medium |
| Product docs | B2C roadmap | Зафиксированы MVP и post-MVP B2C функции | `docs/product/roadmap.md`, `docs/product/b2c-features.md` | High |
| Product docs | B2B roadmap | Зафиксированы B2B SaaS направления | `docs/product/b2b-features.md`, `docs/product/monetization.md` | High |
| AI prompts | Label parser prompt | Черновик strict JSON промпта для парсинга этикеток | `docs/prompts/wine-label-parser.md` | Medium |
| AI prompts | Food pairing prompt | Черновик strict JSON промпта для pairing | `docs/prompts/food-pairing.md` | Medium |
| AI prompts | Wine card scanner prompt | Черновик strict JSON промпта для винных карт | `docs/prompts/wine-card-scanner.md` | Medium |
| Product/Marketing | Основной домен | Зафиксирован `terroir-app.com` как landing domain | `README.md`, `docs/product/vision.md` | High |

## Технические/платформенные возможности

- Docker Compose для локальной инфраструктуры.
- PostgreSQL с расширениями `vector`, `pg_trgm`, `unaccent`, `pgcrypto`.
- Go backend без тяжелого фреймворка.
- JSON health/readiness endpoints.
- Шаблон env-переменных через `.env.example`.
- Git ignore для секретов, Go, Flutter и локальных данных.

## Интеграции и внешние сервисы

Запланированы, но не реализованы в коде:

- DeepSeek API.
- OpenAI/ChatGPT API.
- Yandex Vision.
- Google Vision.
- Локальный/S3-compatible storage.

## Возможные скрытые или частично реализованные фичи

- OCR/LLM pipeline частично отражен в схеме `label_scans`, но сервисной реализации пока нет.
- Vector search частично заложен через `tasting_embedding`, но запросов similarity search пока нет.
- Digital cellar частично заложен через `cellars` и `cellar_items`, но API/UI пока нет.
- B2B аналитика описана в документации, но схемы и API пока нет.

## Замечания для сравнения с другим приложением

Текущий репозиторий находится на foundation-этапе. Для честного сравнения с готовым приложением нужно учитывать, что пользовательские UI-фичи пока отсутствуют, а реализованы архитектурные заготовки.

## Checklist для сравнения других проектов

- README и продуктовые документы.
- Dependency config: `go.mod`, `pubspec.yaml`, `package.json`.
- Точка входа backend/frontend.
- HTTP routes/controllers.
- Database migrations.
- Domain models.
- Screens/pages/routes frontend.
- Services для внешних API.
- Env/config файлы.
- Docker/infra файлы.
