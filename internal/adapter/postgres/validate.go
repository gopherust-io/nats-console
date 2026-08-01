package postgres

import (
	"errors"
	"net/url"
	"strings"
)

// ValidateDatabaseURL requires a production-safe Postgres SSL mode.
func ValidateDatabaseURL(databaseURL string) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return errors.New("DATABASE_URL is not a valid URL")
	}
	mode := strings.ToLower(strings.TrimSpace(u.Query().Get("sslmode")))
	switch mode {
	case "require", "verify-ca", "verify-full":
		return nil
	default:
		return errors.New("DATABASE_URL must set sslmode=require, verify-ca, or verify-full")
	}
}
