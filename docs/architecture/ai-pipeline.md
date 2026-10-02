> ⚠️ Устарело с 2026-09-28: продукт пересмотрен, см. [docs/product/vision-2026-09.md](../product/vision-2026-09.md).

# AI pipeline

## Label scan pipeline

```text
Image upload
  -> OCR provider
  -> raw OCR text
  -> LLM parser
  -> normalized wine JSON
  -> validation
  -> wine matching/deduplication
  -> save wine + scan result
```

## OCR strategy

- Yandex Vision для кириллицы и российского рынка.
- Google Vision как международный provider.
- Tesseract как возможный fallback/локальный вариант.

## LLM strategy

- DeepSeek как cost-efficient вариант.
- ChatGPT/OpenAI как quality fallback.
- Все ответы LLM должны валидироваться схемой.

## Что сохраняем

- URI изображения.
- OCR provider.
- Raw OCR text.
- LLM provider.
- Parsed JSON.
- Статус пайплайна.
- Связь с карточкой вина.

## Главный риск

AI может галлюцинировать. Поэтому поля должны иметь confidence/source, а критичные значения нужно валидировать.
