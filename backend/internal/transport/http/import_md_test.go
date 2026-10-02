package http_test

import (
	"bytes"
	"testing"
)

func TestImportMarkdownEndpoint(t *testing.T) {
	c := newClient(t)
	md := "| Вино | Винтаж | Сорт | Стиль | С чем подавать | Кол-во |\n|---|---|---|---|---|---|\n" +
		"| Тест Саперави | 2020 | Саперави | красное сухое | хинкали | 2 |\n" +
		"| Тест Странное | Н/В | ? | непонятно | — | 1 |\n"
	st, body := c.raw("POST", "/v1/import.md", md, "text/markdown")
	if st != 200 || !bytes.Contains(body, []byte(`"imported":1`)) || !bytes.Contains(body, []byte(`"row":4`)) {
		t.Fatalf("import.md: %d %s", st, body)
	}
	st, b := c.do("GET", "/v1/cellar/items?q=Тест Саперави", nil)
	expect(t, st, 200, b)
	if len(items(b)) != 1 {
		t.Fatalf("imported bottle not found: %v", b)
	}
	st, b = c.do("POST", "/v1/import.md", "нет таблицы")
	expect(t, st, 400, b)
	c.token = ""
	st, _ = c.raw("POST", "/v1/import.md", md, "text/markdown")
	if st != 401 {
		t.Fatalf("auth: %d", st)
	}
}
