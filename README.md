# Terroir

Terroir — кроссплатформенное wine AI-приложение для любителей вина, виноделов и дистрибьюторов.

## Домены

- Основной landing: `https://terroir-app.com`
- Будущий API: `https://api.terroir-app.com`

## Цель

Создать B2C/B2B продукт с цифровым погребом, AI-сомелье, OCR-сканированием этикеток, food pairing и будущей B2B-аналитикой для виноделен и дистрибьюторов.

## Технологический стек

- Frontend: Flutter / Dart
- Backend: Go
- Database: self-hosted PostgreSQL + pgvector
- AI/OCR: OCR providers + LLM API
- Infra: Docker Compose на бюджетном VPS

## Структура

```text
backend/    Go API
frontend/   Flutter app, будет добавлен позже
infra/      PostgreSQL init и будущая VPS-инфраструктура
docs/       продуктовая и архитектурная память
```

## Почему `.env.example`, а не `.env`

`.env.example` — безопасный шаблон переменных окружения, который хранится в Git.

Реальный `.env` создается локально из шаблона и не коммитится, потому что в нем будут пароли, API-ключи OCR/LLM и другие секреты.

## Локальный запуск

1. Скопировать переменные окружения:

```bash
cp .env.example .env
```

2. Запустить PostgreSQL с pgvector:

```bash
docker compose up -d postgres
```

3. Запустить backend:

```bash
docker compose up backend
```

4. Проверить health endpoints:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

## Архитектурный принцип

Flutter не работает напрямую с базой данных. Клиент общается с Go backend, а backend отвечает за бизнес-логику, лимиты подписки, OCR/LLM pipeline, хранение данных и интеграции.
