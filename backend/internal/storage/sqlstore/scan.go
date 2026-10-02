package sqlstore

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Time formats SQLite may return for DATETIME columns.
var sqliteTimeFormats = []string{
	fixedTimeFormat,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// dbTime scans a timestamp from either driver (time.Time from pgx, time.Time or text from SQLite).
type dbTime struct {
	T     time.Time
	Valid bool
}

func (t *dbTime) Scan(v any) error {
	switch x := v.(type) {
	case nil:
		t.Valid = false
		return nil
	case time.Time:
		t.T, t.Valid = x.UTC(), true
		return nil
	case string:
		return t.parse(x)
	case []byte:
		return t.parse(string(x))
	default:
		return fmt.Errorf("unsupported time type %T", v)
	}
}

func (t *dbTime) parse(s string) error {
	for _, f := range sqliteTimeFormats {
		if p, err := time.Parse(f, s); err == nil {
			t.T, t.Valid = p.UTC(), true
			return nil
		}
	}
	return fmt.Errorf("cannot parse time %q", s)
}

func (t dbTime) ptr() *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.T
	return &v
}

// fixedTimeFormat has constant width, so lexicographic order == chronological order in SQLite.
const fixedTimeFormat = "2006-01-02T15:04:05.000000Z"

// ts converts a time to a driver argument: fixed-width UTC text for SQLite, time.Time for Postgres.
func (s *Store) ts(t time.Time) any {
	if s.dialect == SQLite {
		return t.UTC().Format(fixedTimeFormat)
	}
	return t.UTC()
}

func (s *Store) tsPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return s.ts(*t)
}

// jsonList scans/stores a []string as JSON text.
type jsonList []string

func (l *jsonList) Scan(v any) error {
	var b []byte
	switch x := v.(type) {
	case nil:
		*l = []string{}
		return nil
	case string:
		b = []byte(x)
	case []byte:
		b = x
	default:
		return fmt.Errorf("unsupported list type %T", v)
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return err
	}
	if out == nil {
		out = []string{}
	}
	*l = out
	return nil
}

func (l jsonList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(l))
	return string(b), err
}

func intPtr(n *int64) *int {
	if n == nil {
		return nil
	}
	v := int(*n)
	return &v
}

func int64Ptr(p *int) any {
	if p == nil {
		return nil
	}
	return int64(*p)
}
