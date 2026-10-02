package sqlstore

import (
	"context"
	"database/sql"
	"time"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

func nowUTC() time.Time { return time.Now().UTC() }

const wineCols = `w.id, w.cellar_id, w.name, w.producer, w.country, w.region, w.appellation, w.grapes, w.vintage, w.color,
w.sweetness, w.abv, w.body, w.acidity, w.tannin, w.style_notes, w.source, w.created_at, w.updated_at`

func wineDest(w *domain.Wine, grapes *jsonList, vintage *sql.NullInt64, abv *sql.NullFloat64, color, sweet, source *string, created, updated *dbTime) []any {
	return []any{&w.ID, &w.CellarID, &w.Name, &w.Producer, &w.Country, &w.Region, &w.Appellation, grapes, vintage, color,
		sweet, abv, &w.Body, &w.Acidity, &w.Tannin, &w.StyleNotes, source, created, updated}
}

type wineScan struct {
	w                    domain.Wine
	grapes               jsonList
	vintage              sql.NullInt64
	abv                  sql.NullFloat64
	color, sweet, source string
	created, updated     dbTime
}

func (ws *wineScan) dest() []any {
	return wineDest(&ws.w, &ws.grapes, &ws.vintage, &ws.abv, &ws.color, &ws.sweet, &ws.source, &ws.created, &ws.updated)
}

func (ws *wineScan) wine() domain.Wine {
	w := ws.w
	w.Grapes = []string(ws.grapes)
	if ws.vintage.Valid {
		v := int(ws.vintage.Int64)
		w.Vintage = &v
	}
	if ws.abv.Valid {
		v := ws.abv.Float64
		w.ABV = &v
	}
	w.Color = domain.Color(ws.color)
	w.Sweetness = domain.Sweetness(ws.sweet)
	w.Source = domain.Source(ws.source)
	w.CreatedAt, w.UpdatedAt = ws.created.T, ws.updated.T
	return w
}

// CreateWine inserts a wine card.
func (s *Store) CreateWine(ctx context.Context, w domain.Wine) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO wines (id, cellar_id, name, producer, country, region, appellation, grapes, vintage,
color, sweetness, abv, body, acidity, tannin, style_notes, source, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		w.ID, w.CellarID, w.Name, w.Producer, w.Country, w.Region, w.Appellation, jsonList(w.Grapes), int64Ptr(w.Vintage),
		string(w.Color), string(w.Sweetness), w.ABV, w.Body, w.Acidity, w.Tannin, w.StyleNotes, string(w.Source),
		s.ts(w.CreatedAt), s.ts(w.UpdatedAt))
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

// GetWine returns a wine in a cellar.
func (s *Store) GetWine(ctx context.Context, cellarID, id string) (domain.Wine, error) {
	var ws wineScan
	err := s.db.QueryRowContext(ctx, s.q(`SELECT `+wineCols+` FROM wines w WHERE w.cellar_id = ? AND w.id = ?`), cellarID, id).Scan(ws.dest()...)
	if err != nil {
		return domain.Wine{}, notFound(err)
	}
	return ws.wine(), nil
}

// ListWines lists all wine cards in a cellar ordered by name.
func (s *Store) ListWines(ctx context.Context, cellarID string) ([]domain.Wine, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+wineCols+` FROM wines w WHERE w.cellar_id = ? ORDER BY w.name, w.id`), cellarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Wine{}
	for rows.Next() {
		var ws wineScan
		if err := rows.Scan(ws.dest()...); err != nil {
			return nil, err
		}
		out = append(out, ws.wine())
	}
	return out, rows.Err()
}

// UpdateWine overwrites a wine card.
func (s *Store) UpdateWine(ctx context.Context, w domain.Wine) error {
	res, err := s.db.ExecContext(ctx, s.q(`UPDATE wines SET name = ?, producer = ?, country = ?, region = ?, appellation = ?, grapes = ?,
vintage = ?, color = ?, sweetness = ?, abv = ?, body = ?, acidity = ?, tannin = ?, style_notes = ?, source = ?, updated_at = ?
WHERE cellar_id = ? AND id = ?`),
		w.Name, w.Producer, w.Country, w.Region, w.Appellation, jsonList(w.Grapes), int64Ptr(w.Vintage), string(w.Color),
		string(w.Sweetness), w.ABV, w.Body, w.Acidity, w.Tannin, w.StyleNotes, string(w.Source), s.ts(w.UpdatedAt), w.CellarID, w.ID)
	return affected(res, err)
}

// DeleteWine deletes a wine card that has no bottles referencing it.
func (s *Store) DeleteWine(ctx context.Context, cellarID, id string) error {
	var n int
	if err := s.db.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM cellar_items WHERE wine_id = ?`), id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrConflict
	}
	res, err := s.db.ExecContext(ctx, s.q(`DELETE FROM wines WHERE cellar_id = ? AND id = ?`), cellarID, id)
	if isFKViolation(err) {
		return domain.ErrConflict
	}
	return affected(res, err)
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
