package store

const (
	queryGetUserVersion = `SELECT version FROM auth_user_versions WHERE user_id = $1`

	queryBumpUserVersion = `
		INSERT INTO auth_user_versions (user_id, version, updated_at)
		VALUES ($1, 2, now())
		ON CONFLICT (user_id) DO UPDATE
		SET version = auth_user_versions.version + 1, updated_at = now()
		RETURNING version`

	queryRevokeSession = `
		INSERT INTO auth_session_revocations (jti, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (jti) DO UPDATE
		SET expires_at = GREATEST(auth_session_revocations.expires_at, EXCLUDED.expires_at)`

	queryGetSessionRevocation = `SELECT expires_at FROM auth_session_revocations WHERE jti = $1`

	queryPurgeExpiredSessionRevocations = `DELETE FROM auth_session_revocations WHERE expires_at < now()`
)
