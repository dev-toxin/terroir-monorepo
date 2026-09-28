# Database architecture

## Стек

- PostgreSQL 16
- pgvector
- pg_trgm
- unaccent

## Почему PostgreSQL

- Сильная реляционная модель для вина, пользователей, погребов и B2B.
- JSONB для AI payload.
- pgvector для similarity search.
- Можно self-host на VPS без vendor lock-in.

## MVP таблицы

- `users`
- `wines`
- `cellars`
- `cellar_items`
- `label_scans`

## Vector search

Поле `wines.tasting_embedding` хранит embedding вкусового профиля вина.

Примеры будущих сценариев:

- Похожие вина.
- Альтернативы дешевле.
- Вина под похожие блюда.
- Поиск по дегустационной заметке.

## Правило миграций

Схема БД не меняется вручную на production. Все изменения проходят через миграции в `backend/migrations`.
