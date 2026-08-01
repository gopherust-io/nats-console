package store

const (
	userSelectWithRolesGrants = `
	SELECT u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at,
	       COALESCE((
	         SELECT array_agg(r.name ORDER BY r.name)
	         FROM user_roles ur
	         JOIN roles r ON r.id = ur.role_id
	         WHERE ur.user_id = u.id
	       ), '{}'),
	       COALESCE((
	         SELECT json_agg(json_build_object(
	           'id', g.id,
	           'user_id', g.user_id,
	           'resource_type', g.resource_type,
	           'resource_key', g.resource_key,
	           'role', g.role,
	           'created_at', g.created_at,
	           'updated_at', g.updated_at
	         ) ORDER BY g.resource_type, g.resource_key)
	         FROM access_grants g WHERE g.user_id = u.id
	       ), '[]'::json)`

	queryGetUserByUsername = userSelectWithRolesGrants + `, u.password_hash
		FROM users u WHERE u.username = $1`

	queryListUsers = `
		SELECT u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at,
		       COALESCE(array_agg(r.name ORDER BY r.name) FILTER (WHERE r.name IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		GROUP BY u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at
		ORDER BY u.is_root DESC, u.username ASC`

	queryInsertUser = `
		INSERT INTO users (id, username, email, password_hash, oidc_sub, is_root, access_rules, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, username, email, oidc_sub, is_root, access_rules, created_at`

	queryUpdateUserEmailPassword = `
				UPDATE users SET email = $2, password_hash = $3 WHERE id = $1`

	queryUpdateUserEmail = `UPDATE users SET email = $2 WHERE id = $1`

	queryUpdateUserPassword = `UPDATE users SET password_hash = $2 WHERE id = $1`

	queryUpdateUserAccessRules = `UPDATE users SET access_rules = $2 WHERE id = $1`

	queryDeleteUser = `DELETE FROM users WHERE id = $1 AND is_root = false`

	queryDeleteUserRoles = `DELETE FROM user_roles WHERE user_id = $1`

	queryGetRoleIDByName = `SELECT id FROM roles WHERE name = $1`

	queryInsertUserRole = `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`

	queryCountUsers = `SELECT COUNT(*) FROM users`

	queryHasRootUser = `SELECT EXISTS (SELECT 1 FROM users WHERE is_root = true)`

	queryGetUserFromWhere = `
		FROM users u WHERE `
)
