package sqlstore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
	"github.com/anko-soft-lab/terroir/backend/internal/storage"
)

const itemCols = `i.id, i.cellar_id, i.wine_id, i.quantity, i.storage_location, i.purchase_price_minor, i.currency,
i.purchase_place, i.purchased_at, i.drink_from, i.drink_until, i.notes, i.created_at, i.updated_at`

type itemScan struct {
	it               domain.CellarItem
	price            sql.NullInt64
	purchased        dbTime
	from, until      sql.NullInt64
	created, updated dbTime
}

func (is *itemScan) dest() []any {
	return []any{&is.it.ID, &is.it.CellarID, &is.it.WineID, &is.it.Quantity, &is.it.StorageLocation, &is.price, &is.it.Currency,
		&is.it.PurchasePlace, &is.purchased, &is.from, &is.until, &is.it.Notes, &is.created, &is.updated}
}

func (is *itemScan) item() domain.CellarItem {
	it := is.it
	if is.price.Valid {
		v := is.price.Int64
		it.PurchasePriceMinor = &v
	}
	it.PurchasedAt = is.purchased.ptr()
	if is.from.Valid {
		it.DrinkFrom = intPtr(&is.from.Int64)
	}
	if is.until.Valid {
		it.DrinkUntil = intPtr(&is.until.Int64)
	}
	it.CreatedAt, it.UpdatedAt = is.created.T, is.updated.T
	return it
}

// CreateItem inserts bottles. The wine must belong to the same cellar.
func (s *Store) CreateItem(ctx context.Context, it domain.CellarItem) error {
	if _, err := s.GetWine(ctx, it.CellarID, it.WineID); err != nil {
		if err == domain.ErrNotFound {
			return domain.Invalid("wine_id", "wine not found in this cellar")
		}
		return err
	}
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO cellar_items (id, cellar_id, wine_id, quantity, storage_location, purchase_price_minor,
currency, purchase_place, purchased_at, drink_from, drink_until, notes, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		it.ID, it.CellarID, it.WineID, it.Quantity, it.StorageLocation, it.PurchasePriceMinor, it.Currency, it.PurchasePlace,
		s.tsPtr(it.PurchasedAt), int64Ptr(it.DrinkFrom), int64Ptr(it.DrinkUntil), it.Notes, s.ts(it.CreatedAt), s.ts(it.UpdatedAt))
	return err
}

// GetItem returns a cellar item with its wine.
func (s *Store) GetItem(ctx context.Context, cellarID, id string) (domain.CellarEntry, error) {
	var is itemScan
	var ws wineScan
	err := s.db.QueryRowContext(ctx, s.q(`SELECT `+itemCols+`, `+wineCols+` FROM cellar_items i JOIN wines w ON w.id = i.wine_id
WHERE i.cellar_id = ? AND i.id = ?`), cellarID, id).Scan(append(is.dest(), ws.dest()...)...)
	if err != nil {
		return domain.CellarEntry{}, notFound(err)
	}
	return domain.CellarEntry{Item: is.item(), Wine: ws.wine()}, nil
}

// UpdateItem overwrites editable fields of a cellar item.
func (s *Store) UpdateItem(ctx context.Context, it domain.CellarItem) error {
	res, err := s.db.ExecContext(ctx, s.q(`UPDATE cellar_items SET quantity = ?, storage_location = ?, purchase_price_minor = ?, currency = ?,
purchase_place = ?, purchased_at = ?, drink_from = ?, drink_until = ?, notes = ?, updated_at = ? WHERE cellar_id = ? AND id = ?`),
		it.Quantity, it.StorageLocation, it.PurchasePriceMinor, it.Currency, it.PurchasePlace, s.tsPtr(it.PurchasedAt),
		int64Ptr(it.DrinkFrom), int64Ptr(it.DrinkUntil), it.Notes, s.ts(it.UpdatedAt), it.CellarID, it.ID)
	return affected(res, err)
}

// ListItems lists cellar entries with SQL-side filters (window filters are applied in the service).
func (s *Store) ListItems(ctx context.Context, cellarID string, f storage.CellarFilter) ([]domain.CellarEntry, error) {
	var where []string
	args := []any{cellarID}
	where = append(where, "i.cellar_id = ?")
	if f.InStockOnly || !f.IncludeEmpty {
		where = append(where, "i.quantity > 0")
	}
	if f.Color != "" {
		where = append(where, "w.color = ?")
		args = append(args, string(f.Color))
	}

	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+itemCols+`, `+wineCols+` FROM cellar_items i JOIN wines w ON w.id = i.wine_id
WHERE `+strings.Join(where, " AND ")+` ORDER BY w.name, w.vintage, i.id`), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.CellarEntry{}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for rows.Next() {
		var is itemScan
		var ws wineScan
		if err := rows.Scan(append(is.dest(), ws.dest()...)...); err != nil {
			return nil, err
		}
		e := domain.CellarEntry{Item: is.item(), Wine: ws.wine()}
		// Grape, country, location and free-text filters are done in Go: grapes are JSON and need
		// alias matching, and SQLite LOWER() does not fold Cyrillic. Household cellars are small.
		if f.Grape != "" && !e.Wine.HasGrape(f.Grape) {
			continue
		}
		if q != "" && !matchesQuery(e, q) {
			continue
		}
		if f.Country != "" && !strings.EqualFold(strings.TrimSpace(e.Wine.Country), strings.TrimSpace(f.Country)) {
			continue
		}
		if f.Location != "" && !strings.Contains(strings.ToLower(e.Item.StorageLocation), strings.ToLower(strings.TrimSpace(f.Location))) {
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func matchesQuery(e domain.CellarEntry, q string) bool {
	hay := strings.ToLower(strings.Join(append([]string{e.Wine.Name, e.Wine.Producer, e.Wine.Region, e.Wine.Country,
		e.Wine.Appellation, e.Wine.StyleNotes, e.Item.Notes, e.Item.StorageLocation}, e.Wine.Grapes...), " "))
	return strings.Contains(hay, q)
}

// Drink atomically decrements the quantity and records the event.
//
// The conditional UPDATE ("... WHERE quantity >= n") is the guard against going negative:
// two concurrent requests for the last bottle cannot both match the row.
func (s *Store) Drink(ctx context.Context, cellarID string, ev domain.ConsumptionEvent) (domain.CellarItem, error) {
	var updated domain.CellarItem
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, s.q(`UPDATE cellar_items SET quantity = quantity - ?, updated_at = ?
WHERE cellar_id = ? AND id = ? AND quantity >= ?`), ev.Bottles, s.ts(ev.DrunkAt), cellarID, ev.CellarItemID, ev.Bottles)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			var exists int
			if err := tx.QueryRowContext(ctx, s.q(`SELECT COUNT(*) FROM cellar_items WHERE cellar_id = ? AND id = ?`), cellarID, ev.CellarItemID).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return domain.ErrNotFound
			}
			return domain.ErrOutOfStock
		}
		var wineID string
		if err := tx.QueryRowContext(ctx, s.q(`SELECT wine_id FROM cellar_items WHERE id = ?`), ev.CellarItemID).Scan(&wineID); err != nil {
			return err
		}
		ev.WineID = wineID
		_, err = tx.ExecContext(ctx, s.q(`INSERT INTO consumption_events (id, cellar_item_id, wine_id, user_id, bottles, drunk_at, rating, note, occasion, paired_recipe_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`), ev.ID, ev.CellarItemID, ev.WineID, ev.UserID, ev.Bottles, s.ts(ev.DrunkAt), ev.Rating, ev.Note, ev.Occasion, ev.PairedRecipeID)
		if err != nil {
			return err
		}
		if ev.PairedRecipeID != nil && ev.Rating > 0 {
			_, err = tx.ExecContext(ctx, s.q(`INSERT INTO pairing_feedback (id, cellar_id, recipe_id, wine_id, user_id, rating, note, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), NewID(), cellarID, *ev.PairedRecipeID, ev.WineID, ev.UserID, ev.Rating, ev.Note, s.ts(ev.DrunkAt))
			if isFKViolation(err) {
				return domain.Invalid("paired_recipe_id", "recipe not found")
			}
			if err != nil {
				return err
			}
		}
		var is itemScan
		if err := tx.QueryRowContext(ctx, s.q(`SELECT `+itemCols+` FROM cellar_items i WHERE i.id = ?`), ev.CellarItemID).Scan(is.dest()...); err != nil {
			return err
		}
		updated = is.item()
		return nil
	})
	return updated, err
}

// ListConsumption lists drink events in a cellar (optionally for one user), newest first.
func (s *Store) ListConsumption(ctx context.Context, cellarID, userID string, limit int) ([]domain.ConsumptionEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT e.id, e.cellar_item_id, e.wine_id, e.user_id, e.bottles, e.drunk_at, e.rating, e.note, e.occasion, e.paired_recipe_id
FROM consumption_events e JOIN cellar_items i ON i.id = e.cellar_item_id WHERE i.cellar_id = ?`
	args := []any{cellarID}
	if userID != "" {
		query += ` AND e.user_id = ?`
		args = append(args, userID)
	}
	query += ` ORDER BY e.drunk_at DESC, e.id LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, s.q(query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ConsumptionEvent{}
	for rows.Next() {
		var e domain.ConsumptionEvent
		var at dbTime
		var recipe sql.NullString
		if err := rows.Scan(&e.ID, &e.CellarItemID, &e.WineID, &e.UserID, &e.Bottles, &at, &e.Rating, &e.Note, &e.Occasion, &recipe); err != nil {
			return nil, err
		}
		e.DrunkAt = at.T
		if recipe.Valid {
			v := recipe.String
			e.PairedRecipeID = &v
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
