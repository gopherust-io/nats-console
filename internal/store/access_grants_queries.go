package store

const (
	queryListAccessGrantsByUser = `
		SELECT id, user_id, resource_type, resource_key, role, created_at, updated_at
		FROM access_grants WHERE user_id = $1
		ORDER BY resource_type, resource_key`

	queryListAccessGrantsByResource = `
		SELECT g.id, g.user_id, g.resource_type, g.resource_key, g.role, g.created_at, g.updated_at,
		       u.username, u.email
		FROM access_grants g
		JOIN users u ON u.id = g.user_id
		WHERE g.resource_type = $1 AND g.resource_key = $2
		ORDER BY u.username`

	queryUpsertAccessGrant = `
		INSERT INTO access_grants (id, user_id, resource_type, resource_key, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (user_id, resource_type, resource_key) DO UPDATE
		SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at
		RETURNING id, user_id, resource_type, resource_key, role, created_at, updated_at`

	queryDeleteAccessGrant = `DELETE FROM access_grants WHERE id = $1`

	queryDeleteAccessGrantScoped = `
		DELETE FROM access_grants
		WHERE id = $1 AND resource_type = $2 AND resource_key = $3
		RETURNING user_id`

	queryDeleteAccessGrantByResource = `
		DELETE FROM access_grants
		WHERE user_id = $1 AND resource_type = $2 AND resource_key = $3`

	queryDeleteAccessGrantsByResourceKey = `
		DELETE FROM access_grants
		WHERE resource_type = $1 AND resource_key = $2
		RETURNING user_id`

	queryInsertUserInvite = `
		INSERT INTO user_invites (token, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)`

	queryGetUserInvite = `
		SELECT i.token, i.user_id, i.expires_at, i.accepted_at, i.created_at, u.username, u.email
		FROM user_invites i
		JOIN users u ON u.id = i.user_id
		WHERE i.token = $1`

	queryAcceptUserInvite = `
		UPDATE user_invites SET accepted_at = $2
		WHERE token = $1 AND accepted_at IS NULL AND expires_at > $2`

	queryListAccessGrantsByUserIDs = `
		SELECT id, user_id, resource_type, resource_key, role, created_at, updated_at
		FROM access_grants WHERE user_id = ANY($1)
		ORDER BY user_id, resource_type, resource_key`

	queryUpsertAccessGrantNoReturning = `
			INSERT INTO access_grants (id, user_id, resource_type, resource_key, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			ON CONFLICT (user_id, resource_type, resource_key) DO UPDATE
			SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at`
)
