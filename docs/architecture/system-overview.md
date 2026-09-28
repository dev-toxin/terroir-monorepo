# System overview

## High-level архитектура

```text
Flutter app
  -> Go backend API
    -> PostgreSQL + pgvector
    -> file/object storage
    -> OCR providers
    -> LLM providers
```

## Главный принцип

Клиент не ходит напрямую в базу данных. Вся бизнес-логика находится в Go backend.

## Компоненты

### Flutter frontend

- Mobile UI.
- Camera/image upload.
- Погреб и карточки вин.
- Food pairing UI.
- Premium/paywall UI в будущем.

### Go backend

- Auth/session boundary.
- API для Flutter.
- Лимиты Freemium.
- OCR/LLM pipeline.
- Нормализация вин.
- Работа с PostgreSQL.
- B2B API в будущем.

### PostgreSQL + pgvector

- Основное хранилище данных.
- JSONB для сырого и нормализованного AI payload.
- pgvector для похожих вин и taste profile search.

### Storage

- MVP: локальное хранилище на VPS.
- Production target: S3-compatible storage, если локальный диск станет риском.

## Домены

- Landing: `terroir-app.com`
- API target: `api.terroir-app.com`
- App deep links target: `app.terroir-app.com` или universal links через основной домен.
