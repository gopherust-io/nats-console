package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateDatabaseURL(t *testing.T) {
	t.Parallel()

	for _, u := range []string{
		"postgres://u:p@db.example:5432/natsconsol?sslmode=require",
		"postgres://u:p@db.example:5432/natsconsol?sslmode=verify-ca",
		"postgres://u:p@db.example:5432/natsconsol?sslmode=verify-full",
		"postgres://u:p@db.example:5432/natsconsol?sslmode=Verify-Full",
	} {
		require.NoError(t, ValidateDatabaseURL(u), "%q", u)
	}

	for _, u := range []string{
		"postgres://u:p@db.example:5432/natsconsol?sslmode=disable",
		"postgres://u:p@db.example:5432/natsconsol?sslmode=allow",
		"postgres://u:p@db.example:5432/natsconsol",
		"://bad",
	} {
		require.Error(t, ValidateDatabaseURL(u), "%q", u)
	}

	err := ValidateDatabaseURL("postgres://u:p@db.example:5432/natsconsol?sslmode=disable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sslmode")
}
