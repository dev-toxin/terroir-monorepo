# Terroir — статус репозитория

> Снимок на начало сессии 2026-10-02 (этап 0). Итоговая таблица по функциям — в конце файла обновляется в этапе 8.

## Что было на старте

| Область | Состояние | Где |
|---|---|---|
| Go API skeleton | есть: slog, graceful shutdown, `/health`, `/ready` | `backend/cmd/api`, `backend/internal/transport/http` |
| Конфиг | `DATABASE_URL` обязателен → без Postgres не стартует | `backend/internal/config` |
| БД | pgxpool, миграции **нигде не применяются** | `backend/internal/database`, `backend/migrations/000001_initial_schema.sql` |
| Схема | под сканер этикеток: `users(email)`, `wines` c `VECTOR(1536)` (привязка к OpenAI), `cellars`, `cellar_items`, `label_scans` | `backend/migrations` |
| Инфраструктура | docker-compose: pgvector:pg16 + `golang:1.22 go run` | `docker-compose.yml`, `infra/postgres/init` |
| Тесты | ноль | — |
| Клиент | нет (Flutter не создан) | — |
| Документация | ~850 строк под старую идею «AI-сомелье: фото → карточка, OCR, B2B» | `docs/product`, `docs/architecture`, `docs/prompts` |

## Что устарело относительно видения 2026-09-28

- Продукт — **персональный винный помощник с памятью**, а не сканер. OCR/фото этикетки — вне MVP.
- `label_scans`, промпты `wine-label-parser`, `wine-card-scanner`, OCR-провайдеры (Google/Yandex Vision) — устарели.
- B2B-кабинет, монетизация через бренды — вне горизонта MVP.
- OpenAI/DeepSeek/Google в `.env.example` — заменяются одним OpenAI-совместимым провайдером (по умолчанию mock).
- `VECTOR(1536)` — убрано из MVP-схемы (эмбеддинги не обязательны).
- Совместный погреб пары был post-MVP — теперь ядро.
