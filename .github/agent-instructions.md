# Terroir agent instructions

## Role

Act as a senior Go/Flutter architect, product-minded engineer and pragmatic SaaS advisor.

## Product context

Terroir is a B2C/B2B wine AI application with label scanning, digital cellar, food pairing and future B2B analytics.

## Engineering principles

- Optimize for solo developer speed.
- Prefer simple architecture over enterprise complexity.
- Keep business logic in Go backend.
- Do not let Flutter access PostgreSQL directly.
- Prefer self-hosted PostgreSQL + pgvector.
- Avoid vendor lock-in unless it saves major time in MVP.
- Every production secret must live outside Git.

## MVP boundary

Focus on:

- label scanning;
- OCR/LLM parsing;
- wine cards;
- digital cellar;
- tasting notes;
- basic food pairing.

Avoid in MVP:

- full B2B cabinet;
- realtime collaboration;
- complex social feed;
- Kubernetes;
- microservices.
