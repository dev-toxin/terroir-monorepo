# Wine card scanner prompt

## Назначение

Парсинг OCR-текста винной карты ресторана в список вин.

## Prompt draft

```text
OCR text from restaurant wine card:
{{ocr_text}}

Return strict JSON:
{
  "restaurant_name": "",
  "currency": "",
  "items": [
    {
      "raw_line": "",
      "wine_name": "",
      "producer": "",
      "country": "",
      "region": "",
      "vintage": null,
      "price": null,
      "volume_ml": null,
      "confidence": 0.0
    }
  ],
  "warnings": []
}
```

## Правила

- Сохраняй `raw_line` для аудита.
- Не объединяй разные позиции без уверенности.
- Не выдумывай винтаж и цену.
