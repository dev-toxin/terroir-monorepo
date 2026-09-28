# Backend architecture

## Стек

- Go 1.22+
- net/http на старте
- pgx для PostgreSQL
- goose/golang-migrate для миграций на следующем шаге

## Структура

```text
backend/
  cmd/api/main.go
  internal/config
  internal/database
  internal/transport/http
  migrations
```

## Принципы

- Простая архитектура без лишнего фреймворка.
- Явные зависимости.
- Вся бизнес-логика вне HTTP handlers.
- Контексты и таймауты обязательны для внешних вызовов.
- JSON API для Flutter.

## Текущие endpoints

- `GET /health` — процесс жив.
- `GET /ready` — процесс жив и есть подключение к БД.

## Ближайшие модули

- Auth.
- Users.
- Wines.
- Cellars.
- Label scans.
- AI parsing.
- Food pairing.
