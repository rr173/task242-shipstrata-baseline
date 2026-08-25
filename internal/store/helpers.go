package store

import (
	"time"
)

// rowScanner 兼容 *sql.Row 与 *sql.Rows 的 Scan。
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func nullTimeStr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func mustParse(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
