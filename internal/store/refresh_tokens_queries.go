package store

const (
	//nolint:gosec // G101: SQL column names, not credentials
	queryInsertRefreshToken = `
		INSERT INTO auth_refresh_tokens (id, user_id, token_hash, fingerprint_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`

	queryGetRefreshTokenByHash = `
		SELECT id, user_id, token_hash, fingerprint_hash, expires_at, created_at, replaced_by
		FROM auth_refresh_tokens
		WHERE token_hash = $1`

	queryMarkRefreshTokenReplaced = `
		UPDATE auth_refresh_tokens
		SET replaced_by = $2
		WHERE id = $1 AND replaced_by IS NULL`

	queryDeleteRefreshTokensByUser = `DELETE FROM auth_refresh_tokens WHERE user_id = $1`

	queryDeleteRefreshTokenByHash = `DELETE FROM auth_refresh_tokens WHERE token_hash = $1`

	queryPurgeExpiredRefreshTokens = `DELETE FROM auth_refresh_tokens WHERE expires_at < now()`
)
