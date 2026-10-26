// Package dbtype holds SQLite column types that database/sql does not handle the way we want.
//
// Column declarations in migrations use type names that keep SQLite's TEXT affinity and let sqlc
// map them to these Go types:
//
//	DECIMAL_TEXT   → money.Dec
//	TIMESTAMP_TEXT → dbtype.Time   (UTC, fixed width, sorts as text)
//	JSON_TEXT      → dbtype.JSON
//	BOOLEAN        → bool          (stored as 0/1)
package dbtype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// TimeLayout matches strftime('%Y-%m-%dT%H:%M:%fZ', 'now') used in column defaults, so values
// written by Go and by SQLite compare correctly as strings.
const TimeLayout = "2006-01-02T15:04:05.000Z"

// Time is a UTC timestamp stored as fixed-width TEXT.
type Time struct {
	time.Time
}

func NewTime(t time.Time) Time { return Time{t.UTC()} }

func Now() Time { return NewTime(time.Now()) }

func (t Time) Value() (driver.Value, error) { return t.UTC().Format(TimeLayout), nil }

func (t *Time) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	case time.Time:
		t.Time = v.UTC()
		return nil
	default:
		return fmt.Errorf("dbtype.Time: unsupported type %T", src)
	}
	for _, layout := range []string{TimeLayout, time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if p, err := time.Parse(layout, s); err == nil {
			t.Time = p.UTC()
			return nil
		}
	}
	return fmt.Errorf("dbtype.Time: cannot parse %q", s)
}

func (t Time) String() string { return t.UTC().Format(TimeLayout) }

// FormatTime formats for TEXT comparisons in queries (e.g. "WHERE expires_at > ?").
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// JSON is a JSON document stored as TEXT.
type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "null", nil
	}
	return string(j), nil
}

func (j *JSON) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*j = JSON(v)
	case []byte:
		*j = append((*j)[:0], v...)
	default:
		return fmt.Errorf("dbtype.JSON: unsupported type %T", src)
	}
	return nil
}

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(b []byte) error {
	*j = append((*j)[:0], b...)
	return nil
}

// MarshalJSONValue encodes v for a JSON_TEXT column.
func MarshalJSONValue(v any) (JSON, error) {
	b, err := json.Marshal(v)
	return JSON(b), err
}
