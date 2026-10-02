package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// Markdown cellar import.
//
// The owner keeps the cellar in a notebook as a Markdown table:
//
//	| Вино | Винтаж | Сорт | Стиль | С чем подавать | Кол-во |
//	|------|--------|------|-------|----------------|--------|
//	| Саперави | 2019 | Саперави | красное сухое | стейк | 2 |
//
// EN headers are accepted too: Wine | Vintage | Grape | Style | Pairs with | Qty.
// Column order may differ; unknown columns are ignored. Rules:
//   - Vintage: "Н/В", "NV" or empty → non-vintage;
//   - Grape: separated by "," or "/";
//   - Style: color keyword is required (красное/белое/розовое/оранжевое/игристое/десертное/креплёное
//     or red/white/rose/orange/sparkling/dessert/fortified); sweetness keyword is optional;
//   - "Pairs with" goes to bottle notes; Qty defaults to 1.

var mdHeaderAliases = map[string]string{
	"вино": "name", "wine": "name", "название": "name", "name": "name",
	"винтаж": "vintage", "vintage": "vintage", "год": "vintage", "year": "vintage",
	"сорт": "grapes", "сорта": "grapes", "grape": "grapes", "grapes": "grapes",
	"стиль": "style", "style": "style",
	"с чем подавать": "pairs", "pairs with": "pairs", "pairing": "pairs", "к чему": "pairs",
	"кол-во": "qty", "количество": "qty", "qty": "qty", "quantity": "qty",
}

// Order matters: longer/more specific keywords first ("полусухое" before "сухое").
var mdSweetness = []struct {
	kw string
	s  domain.Sweetness
}{
	{"полусладк", domain.SweetnessMedium}, {"полусух", domain.SweetnessOffDry}, {"сладк", domain.SweetnessSweet}, {"сух", domain.SweetnessDry},
	{"off-dry", domain.SweetnessOffDry}, {"off dry", domain.SweetnessOffDry}, {"medium", domain.SweetnessMedium},
	{"semi-sweet", domain.SweetnessMedium}, {"sweet", domain.SweetnessSweet}, {"dry", domain.SweetnessDry},
}

var mdColors = []struct {
	kw string
	c  domain.Color
}{
	{"игрист", domain.ColorSparkling}, {"sparkling", domain.ColorSparkling}, {"брют", domain.ColorSparkling}, {"brut", domain.ColorSparkling},
	{"десерт", domain.ColorDessert}, {"dessert", domain.ColorDessert},
	{"креплён", domain.ColorFortified}, {"креплен", domain.ColorFortified}, {"fortified", domain.ColorFortified},
	{"оранжев", domain.ColorOrange}, {"янтарн", domain.ColorOrange}, {"orange", domain.ColorOrange}, {"amber", domain.ColorOrange},
	{"розов", domain.ColorRose}, {"rosé", domain.ColorRose}, {"rose", domain.ColorRose},
	{"красн", domain.ColorRed}, {"red", domain.ColorRed},
	{"бел", domain.ColorWhite}, {"white", domain.ColorWhite},
}

// ParseStyle maps a free-text style ("красное полусухое") to color and sweetness.
func ParseStyle(style string) (domain.Color, domain.Sweetness, bool) {
	low := strings.ToLower(style)
	var color domain.Color
	for _, c := range mdColors {
		if strings.Contains(low, c.kw) {
			color = c.c
			break
		}
	}
	var sweet domain.Sweetness
	for _, s := range mdSweetness {
		if strings.Contains(low, s.kw) {
			sweet = s.s
			break
		}
	}
	return color, sweet, color != ""
}

func splitMDRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isMDSeparator(cells []string) bool {
	for _, c := range cells {
		c = strings.Trim(c, ":- ")
		if c != "" {
			return false
		}
	}
	return len(cells) > 0
}

// ImportMarkdown imports the first Markdown table found in r. Valid rows are imported,
// invalid rows are reported with their line number.
func (s *Service) ImportMarkdown(ctx context.Context, a Actor, r io.Reader) (ImportResult, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	res := ImportResult{Errors: []ImportError{}}
	var col map[string]int
	lineNo := 0
	rows := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "|") {
			if col != nil && line != "" {
				break // table ended
			}
			continue
		}
		cells := splitMDRow(line)
		if col == nil {
			col = map[string]int{}
			for i, h := range cells {
				if k, ok := mdHeaderAliases[strings.ToLower(strings.Trim(h, "* "))]; ok {
					col[k] = i
				}
			}
			if _, ok := col["name"]; !ok {
				return res, domain.Invalid("file", "markdown table must have a 'Вино'/'Wine' column")
			}
			if _, ok := col["style"]; !ok {
				return res, domain.Invalid("file", "markdown table must have a 'Стиль'/'Style' column")
			}
			continue
		}
		if isMDSeparator(cells) {
			continue
		}
		rows++
		if rows > 5000 {
			res.Errors = append(res.Errors, ImportError{Row: lineNo, Message: "too many rows (max 5000)"})
			break
		}
		get := func(k string) string {
			if i, ok := col[k]; ok && i < len(cells) {
				return cells[i]
			}
			return ""
		}
		w, it, err := parseMDRow(get)
		if err != nil {
			res.Errors = append(res.Errors, ImportError{Row: lineNo, Message: err.Error()})
			continue
		}
		if err := s.importOne(ctx, a, w, it); err != nil {
			if domain.IsValidation(err) {
				res.Errors = append(res.Errors, ImportError{Row: lineNo, Message: err.Error()})
				continue
			}
			return res, err
		}
		res.Imported++
	}
	if err := sc.Err(); err != nil {
		return res, domain.Invalid("file", "cannot read markdown: %v", err)
	}
	if col == nil {
		return res, domain.Invalid("file", "no markdown table found")
	}
	return res, nil
}

func parseMDRow(get func(string) string) (domain.Wine, domain.CellarItem, error) {
	w := domain.Wine{Name: get("name")}
	if w.Name == "" {
		return w, domain.CellarItem{}, fmt.Errorf("name: is required")
	}
	color, sweet, ok := ParseStyle(get("style"))
	if !ok {
		return w, domain.CellarItem{}, fmt.Errorf("style: cannot detect wine color in %q", get("style"))
	}
	w.Color, w.Sweetness = color, sweet
	if g := get("grapes"); g != "" {
		w.Grapes = strings.FieldsFunc(g, func(r rune) bool { return r == ',' || r == '/' || r == ';' })
	}
	var err error
	if w.Vintage, err = parseOptInt(get("vintage"), "vintage"); err != nil {
		return w, domain.CellarItem{}, err
	}
	it := domain.CellarItem{Quantity: 1}
	if pairs := get("pairs"); pairs != "" {
		it.Notes = "С чем подавать: " + pairs
	}
	if q := strings.TrimSpace(get("qty")); q != "" {
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(q, " шт"), "шт"))
		if err != nil {
			return w, it, fmt.Errorf("qty: not an integer")
		}
		it.Quantity = n
	}
	return w, it, nil
}
