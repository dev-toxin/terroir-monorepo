package sqlstore

import (
	"context"
	"database/sql"

	"github.com/anko-soft-lab/terroir/backend/internal/domain"
)

// CreateUser inserts a user.
func (s *Store) CreateUser(ctx context.Context, u domain.User) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO users (id, display_name, locale, telegram_id, created_at) VALUES (?, ?, ?, ?, ?)`),
		u.ID, u.DisplayName, string(u.Locale), u.TelegramID, s.ts(u.CreatedAt))
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

func scanUser(row interface{ Scan(...any) error }) (domain.User, error) {
	var u domain.User
	var locale string
	var tg sql.NullInt64
	var created dbTime
	if err := row.Scan(&u.ID, &u.DisplayName, &locale, &tg, &created); err != nil {
		return u, notFound(err)
	}
	u.Locale = domain.Locale(locale)
	if tg.Valid {
		v := tg.Int64
		u.TelegramID = &v
	}
	u.CreatedAt = created.T
	return u, nil
}

const userCols = `u.id, u.display_name, u.locale, u.telegram_id, u.created_at`

// GetUser returns a user by id.
func (s *Store) GetUser(ctx context.Context, id string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, s.q(`SELECT `+userCols+` FROM users u WHERE u.id = ?`), id))
}

// AddToken registers a token hash for a user (idempotent).
func (s *Store) AddToken(ctx context.Context, userID, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO user_tokens (token_hash, user_id, created_at) VALUES (?, ?, ?) ON CONFLICT (token_hash) DO NOTHING`),
		tokenHash, userID, s.ts(nowUTC()))
	return err
}

// UserByToken resolves a token hash to a user.
func (s *Store) UserByToken(ctx context.Context, tokenHash string) (domain.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, s.q(`SELECT `+userCols+` FROM users u JOIN user_tokens t ON t.user_id = u.id WHERE t.token_hash = ?`), tokenHash))
}

// CreateCellar inserts a cellar.
func (s *Store) CreateCellar(ctx context.Context, c domain.Cellar) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO cellars (id, name, created_at) VALUES (?, ?, ?)`), c.ID, c.Name, s.ts(c.CreatedAt))
	if isUniqueViolation(err) {
		return domain.ErrConflict
	}
	return err
}

// AddMember adds a user to a cellar (idempotent).
func (s *Store) AddMember(ctx context.Context, m domain.Membership) error {
	_, err := s.db.ExecContext(ctx, s.q(`INSERT INTO cellar_members (cellar_id, user_id, role) VALUES (?, ?, ?) ON CONFLICT (cellar_id, user_id) DO NOTHING`),
		m.CellarID, m.UserID, string(m.Role))
	return err
}

// MembershipsOf lists cellars of a user, owners first.
func (s *Store) MembershipsOf(ctx context.Context, userID string) ([]domain.Membership, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT cellar_id, user_id, role FROM cellar_members WHERE user_id = ? ORDER BY role DESC, cellar_id`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		var m domain.Membership
		var role string
		if err := rows.Scan(&m.CellarID, &m.UserID, &role); err != nil {
			return nil, err
		}
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	return out, rows.Err()
}

// MembersOf lists users of a cellar.
func (s *Store) MembersOf(ctx context.Context, cellarID string) ([]domain.User, error) {
	rows, err := s.db.QueryContext(ctx, s.q(`SELECT `+userCols+` FROM users u JOIN cellar_members m ON m.user_id = u.id WHERE m.cellar_id = ? ORDER BY u.display_name`), cellarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
