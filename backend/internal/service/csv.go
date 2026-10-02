package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

// CSVHeader is the column order of export.csv / import.csv. Grapes are separated by "|".
var CSVHeader = []string{"name", "producer", "country", "region", "appellation", "grapes", "vintage", "color", "sweetness", "abv",
	"quantity", "storage_location", "purchase_price_minor", "currency", "purchase_place", "drink_from", "drink_until", "notes", "style_notes"}

// ExportCSV writes all bottles (including empty rows, which keep history) as CSV.
func (s *Service) ExportCSV(ctx context.Context, a Actor, w io.Writer) error {
	entries, err := s.store.ListItems(ctx, a.CellarID, storage.CellarFilter{IncludeEmpty: true})
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(CSVHeader); err != nil {
		return err
	}
	for _, e := range entries {
		wn, it := e.Wine, e.Item
		rec := []string{wn.Name, wn.Producer, wn.Country, wn.Region, wn.Appellation, strings.Join(wn.Grapes, "|"), optInt(wn.Vintage),
			string(wn.Color), string(wn.Sweetness), optFloat(wn.ABV), strconv.Itoa(it.Quantity), it.StorageLocation, optInt64(it.PurchasePriceMinor),
			it.Currency, it.PurchasePlace, optInt(it.DrinkFrom), optInt(it.DrinkUntil), it.Notes, wn.StyleNotes}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// ImportResult summarizes an import.
type ImportResult struct {
	Imported int           `json:"imported"`
	Errors   []ImportError `json:"errors"`
}

// ImportError is a per-row problem; valid rows are still imported.
type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportCSV reads rows in CSVHeader format (header required; column order may differ; unknown columns ignored).
// Each row creates a wine card (source=import) and a cellar item.
func (s *Service) ImportCSV(ctx context.Context, a Actor, r io.Reader) (ImportResult, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return ImportResult{}, domain.Invalid("file", "cannot read CSV header: %v", err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = i
	}
	if _, ok := col["name"]; !ok {
		return ImportResult{}, domain.Invalid("file", "CSV must have a 'name' column")
	}
	if _, ok := col["color"]; !ok {
		return ImportResult{}, domain.Invalid("file", "CSV must have a 'color' column")
	}
	res := ImportResult{Errors: []ImportError{}}
	row := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			res.Errors = append(res.Errors, ImportError{Row: row, Message: err.Error()})
			continue
		}
		if row > 5001 {
			res.Errors = append(res.Errors, ImportError{Row: row, Message: "too many rows (max 5000)"})
			break
		}
		get := func(k string) string {
			if i, ok := col[k]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		w, it, perr := parseRow(get)
		if perr != nil {
			res.Errors = append(res.Errors, ImportError{Row: row, Message: perr.Error()})
			continue
		}
		if err := s.importOne(ctx, a, w, it); err != nil {
			if domain.IsValidation(err) {
				res.Errors = append(res.Errors, ImportError{Row: row, Message: err.Error()})
				continue
			}
			return res, err
		}
		res.Imported++
	}
	return res, nil
}

func (s *Service) importOne(ctx context.Context, a Actor, w domain.Wine, it domain.CellarItem) error {
	w.Source = domain.SourceImport
	w.Normalize()
	if err := w.Validate(); err != nil {
		return err
	}
	it.WineID = "pending"
	it.Normalize()
	if err := it.Validate(); err != nil {
		return err
	}
	cw, err := s.CreateWine(ctx, a, w)
	if err != nil {
		return err
	}
	it.WineID = cw.ID
	_, err = s.AddItem(ctx, a, it)
	return err
}

func parseRow(get func(string) string) (domain.Wine, domain.CellarItem, error) {
	w := domain.Wine{Name: get("name"), Producer: get("producer"), Country: get("country"), Region: get("region"),
		Appellation: get("appellation"), Color: domain.Color(strings.ToLower(get("color"))), Sweetness: domain.Sweetness(strings.ToLower(get("sweetness"))),
		StyleNotes: get("style_notes")}
	if g := get("grapes"); g != "" {
		w.Grapes = strings.Split(g, "|")
	}
	var err error
	if w.Vintage, err = parseOptInt(get("vintage"), "vintage"); err != nil {
		return w, domain.CellarItem{}, err
	}
	if v := get("abv"); v != "" {
		f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
		if err != nil {
			return w, domain.CellarItem{}, fmt.Errorf("abv: not a number")
		}
		w.ABV = &f
	}
	it := domain.CellarItem{Quantity: 1, StorageLocation: get("storage_location"), Currency: get("currency"),
		PurchasePlace: get("purchase_place"), Notes: get("notes")}
	if q := get("quantity"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil {
			return w, it, fmt.Errorf("quantity: not an integer")
		}
		it.Quantity = n
	}
	if p := get("purchase_price_minor"); p != "" {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return w, it, fmt.Errorf("purchase_price_minor: not an integer")
		}
		it.PurchasePriceMinor = &n
	}
	if it.DrinkFrom, err = parseOptInt(get("drink_from"), "drink_from"); err != nil {
		return w, it, err
	}
	if it.DrinkUntil, err = parseOptInt(get("drink_until"), "drink_until"); err != nil {
		return w, it, err
	}
	return w, it, nil
}

func parseOptInt(s, field string) (*int, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "nv") || s == "Н/В" || s == "н/в" {
		return nil, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("%s: not an integer", field)
	}
	return &n, nil
}

func optInt(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func optInt64(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

func optFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}
