# Self-hosted PostgreSQL strategy

## Решение

Terroir использует собственный PostgreSQL на VPS вместо Supabase.

## Почему

- VPS все равно нужен для Go backend.
- PostgreSQL можно держать рядом с backend и контролировать стоимость.
- Нет зависимости от Supabase Auth, Storage, Realtime и Edge Functions.
- Проще сохранить чистую backend-архитектуру: Flutter -> Go API -> PostgreSQL.
- pgvector позволяет реализовать AI-поиск без отдельной vector database на MVP-этапе.

## Базовая схема инфраструктуры

```text
Flutter mobile app
  -> Go backend API
    -> PostgreSQL + pgvector
    -> local/S3-compatible storage
    -> OCR providers
    -> LLM providers
```

## Что PostgreSQL закрывает в Terroir

- Пользователи и профили.
- Вина, винодельни, регионы, сорта винограда.
- Цифровые погреба и дегустационные заметки.
- Результаты OCR/LLM-сканов.
- Подписки и лимиты.
- B2B-аналитика.
- Векторный поиск похожих вин через pgvector.

## Zero Operations правила

1. Все локально запускается через Docker Compose.
2. Все изменения схемы идут через миграции.
3. Секреты хранятся только в `.env`, не в Git.
4. Backup PostgreSQL должен быть автоматизирован до beta-релиза.
5. Flutter не получает прямой доступ к PostgreSQL.

## Ближайшие технические задачи

1. Добавить backend skeleton на Go.
2. Подключить миграции БД.
3. Создать первую схему: users, wines, scans, cellars.
4. Добавить healthcheck endpoint backend.
5. Добавить Makefile или Taskfile для локальных команд.
