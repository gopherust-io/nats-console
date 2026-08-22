package repo

const (
	natsUserSelectCols = `
	id, cluster_id, account_name, name, public_key, signing_group, jwt,
	COALESCE(assigned_user_id::text, ''),
	tags, pub_allow, pub_deny, sub_allow, sub_deny,
	max_subs, max_payload, jwt_lifetime_ns,
	bearer_token, proxy_required, allowed_connection_types, src_cidrs,
	times_locale, time_ranges, resp_max_msgs, resp_ttl_ns, max_data,
	created_at, updated_at`

	queryListNATSAccountUsers = `
		SELECT ` + natsUserSelectCols + `
		FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2
		ORDER BY name ASC`

	queryInsertNATSAccountUser = `
		INSERT INTO nats_account_users
			(id, cluster_id, account_name, name, public_key, seed_encrypted, jwt, signing_group,
			 tags, pub_allow, pub_deny, sub_allow, sub_deny, max_subs, max_payload, jwt_lifetime_ns,
			 bearer_token, proxy_required, allowed_connection_types, src_cidrs,
			 times_locale, time_ranges, resp_max_msgs, resp_ttl_ns, max_data,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$26)`

	queryDeleteNATSAccountUser = `
		DELETE FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryGetNATSAccountUser = `
		SELECT ` + natsUserSelectCols + `
		FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	//nolint:gosec // G101: SQL column names, not credentials
	queryGetNATSAccountUserCreds = `
		SELECT id, cluster_id, account_name, name, public_key, seed_encrypted, jwt, signing_group,
		       COALESCE(assigned_user_id::text, ''),
		       tags, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_subs, max_payload, jwt_lifetime_ns,
		       bearer_token, proxy_required, allowed_connection_types, src_cidrs,
		       times_locale, time_ranges, resp_max_msgs, resp_ttl_ns, max_data,
		       created_at, updated_at
		FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryUpdateNATSAccountUser = `
		UPDATE nats_account_users SET
			signing_group = $4, tags = $5, pub_allow = $6, pub_deny = $7, sub_allow = $8, sub_deny = $9,
			max_subs = $10, max_payload = $11, jwt_lifetime_ns = $12,
			bearer_token = $13, proxy_required = $14, allowed_connection_types = $15, src_cidrs = $16,
			times_locale = $17, time_ranges = $18, resp_max_msgs = $19, resp_ttl_ns = $20, max_data = $21,
			updated_at = $22
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryUpdateNATSAccountUserJWT = `
			UPDATE nats_account_users SET jwt = $4, updated_at = $5
			WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryRotateNATSAccountUser = `
		UPDATE nats_account_users
		SET public_key = $4, seed_encrypted = $5, jwt = $6, updated_at = $7
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryAssignNATSAccountUserPerson = `
		UPDATE nats_account_users SET assigned_user_id = $4, updated_at = $5
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryListNATSAccountExports = `
		SELECT id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at
		FROM nats_account_exports
		WHERE cluster_id = $1 AND account_name = $2 AND ($3 = '' OR kind = $3)
		ORDER BY name ASC`

	queryInsertNATSAccountExport = `
		INSERT INTO nats_account_exports
			(id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at`

	queryUpdateNATSAccountExport = `
		UPDATE nats_account_exports SET
			name = $4, subject = $5, description = $6, updated_at = $7
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3
		RETURNING id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at`

	queryDeleteNATSAccountExport = `
		DELETE FROM nats_account_exports
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`
)
