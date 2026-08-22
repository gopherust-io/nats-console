package repo

const (
	queryEnsureDefaultSigningGroup = `
		INSERT INTO nats_signing_groups
			(id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny, max_data, max_payload, max_subs, created_at, updated_at)
		VALUES ($1, $2, $3, 'Default', false, '{}', '{}', '{}', '{}', -1, -1, -1, NOW(), NOW())
		ON CONFLICT (cluster_id, account_name, name) DO NOTHING`

	queryListSigningGroups = `
		SELECT id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_data, max_payload, max_subs, created_at, updated_at
		FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2
		ORDER BY name ASC`

	queryInsertSigningGroup = `
		INSERT INTO nats_signing_groups
			(id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny, max_data, max_payload, max_subs, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`

	queryGetSigningGroupByName = `
		SELECT id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_data, max_payload, max_subs, created_at, updated_at
		FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND name = $3`

	queryUpdateSigningGroup = `
		UPDATE nats_signing_groups SET
			scoped = $4, pub_allow = $5, pub_deny = $6, sub_allow = $7, sub_deny = $8,
			max_data = $9, max_payload = $10, max_subs = $11, updated_at = $12
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryGetSigningGroupByID = `
		SELECT id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_data, max_payload, max_subs, created_at, updated_at
		FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	queryGetSigningGroupName = `
		SELECT name FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`

	querySigningGroupInUse = `
		SELECT EXISTS (
			SELECT 1 FROM nats_account_users
			WHERE cluster_id = $1 AND account_name = $2 AND signing_group = $3
		)`

	queryDeleteSigningGroup = `
		DELETE FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3 AND name <> 'Default'`
)
