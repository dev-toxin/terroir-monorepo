# Frontend architecture

## Стек

- Flutter / Dart
- iOS + Android на старте
- Web возможно позже для B2B кабинета

## Принципы

- Flutter не работает напрямую с PostgreSQL.
- Все данные идут через Go API.
- Сканирование этикетки должно быть главным быстрым сценарием.
- Формы ручного ввода — fallback, а не основной UX.

## MVP screens

- Onboarding.
- Auth.
- Home.
- Scan label.
- Wine details.
- Cellar.
- Tasting note.
- Food pairing.
- Profile/settings.

## Архитектурное направление

Для solo-разработчика предпочтительна простая feature-first структура:

```text
lib/
  core/
  features/
    auth/
    scan/
    wines/
    cellar/
    pairing/
```
