# Zero Operations strategy

## Цель

Сделать инфраструктуру, которую может поддерживать один разработчик без постоянного ручного администрирования.

## Принципы

- Один VPS на старте.
- Docker Compose вместо Kubernetes.
- Self-hosted PostgreSQL.
- Минимум внешних managed-сервисов.
- Автоматические backup до beta.
- Простые healthchecks.
- Логи в stdout.

## Что автоматизировать первым

- Database backup.
- Deployment script.
- TLS через Caddy или Traefik.
- Monitoring uptime.
- Error logging.

## Чего избегать на MVP

- Kubernetes.
- Microservices.
- Сложный event streaming.
- Отдельная vector database.
- Преждевременная B2B CRM.
