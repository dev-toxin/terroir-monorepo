# ADR-0002: Go + Flutter + PostgreSQL

## Status

Accepted

## Context

Terroir должен быть кроссплатформенным, дешевым в эксплуатации и готовым к AI-функциям.

## Decision

- Flutter для мобильного клиента.
- Go для backend API.
- Self-hosted PostgreSQL + pgvector для данных и vector search.

## Why

- Flutter ускоряет mobile-разработку.
- Go дает простой и дешевый backend.
- PostgreSQL закрывает реляционные данные, JSONB и embeddings.

## Consequences

- Не используем Supabase как платформу.
- Больше ответственности за backup и эксплуатацию БД.
- Меньше vendor lock-in и лучше контроль стоимости.
