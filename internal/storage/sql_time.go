package storage

import (
	"fmt"
	"time"
)

// sqlTime accepts both driver-native timestamps and legacy SQLite values written
// from IMAP's fixed numeric timezone. PostgreSQL normally returns time.Time.
type sqlTime struct{ value *time.Time }

func (t sqlTime) Scan(src any) error {
	if v, ok := src.(time.Time); ok {
		*t.value = v
		return nil
	}
	var raw string
	switch v := src.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unsupported timestamp type %T", src)
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999 -0700 -0700", "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999", "2006-01-02"} {
		if v, err := time.Parse(layout, raw); err == nil {
			*t.value = v
			return nil
		}
	}
	return fmt.Errorf("invalid timestamp %q", raw)
}
