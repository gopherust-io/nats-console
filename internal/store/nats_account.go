package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nkeys"
)

type NATSUserTimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// goalign:ignore
type NATSAccountUser struct {
	CreatedAt              time.Time           `json:"createdAt"`
	UpdatedAt              time.Time           `json:"updatedAt"`
	ID                     string              `json:"id"`
	ClusterID              string              `json:"clusterId"`
	AccountName            string              `json:"accountName"`
	Name                   string              `json:"name"`
	PublicKey              string              `json:"publicKey"`
	SigningGroup           string              `json:"signingGroup"`
	AssignedUserID         string              `json:"assignedUserId,omitempty"`
	Tags                   []string            `json:"tags,omitempty"`
	PubAllow               []string            `json:"pubAllow,omitempty"`
	PubDeny                []string            `json:"pubDeny,omitempty"`
	SubAllow               []string            `json:"subAllow,omitempty"`
	SubDeny                []string            `json:"subDeny,omitempty"`
	AllowedConnectionTypes []string            `json:"allowedConnectionTypes,omitempty"`
	SrcCIDRs               []string            `json:"srcCidrs,omitempty"`
	TimesLocale            string              `json:"timesLocale,omitempty"`
	TimeRanges             []NATSUserTimeRange `json:"timeRanges,omitempty"`
	MaxSubs                int64               `json:"maxSubs"`
	MaxPayload             int64               `json:"maxPayload"`
	MaxData                int64               `json:"maxData"`
	JWTLifetimeNs          int64               `json:"jwtLifetimeNs"`
	RespMaxMsgs            int                 `json:"respMaxMsgs"`
	RespTTLNs              int64               `json:"respTTLNs"`
	BearerToken            bool                `json:"bearerToken"`
	ProxyRequired          bool                `json:"proxyRequired"`
	HasJWT                 bool                `json:"hasJwt"`
	JWTIssued              bool                `json:"jwtIssued"`
}

// goalign:ignore
type NATSAccountUserCreate struct {
	ClusterID              string
	AccountName            string
	Name                   string
	SigningGroup           string
	Tags                   []string
	PubAllow               []string
	PubDeny                []string
	SubAllow               []string
	SubDeny                []string
	AllowedConnectionTypes []string
	SrcCIDRs               []string
	TimesLocale            string
	TimeRanges             []NATSUserTimeRange
	MaxSubs                int64
	MaxPayload             int64
	MaxData                int64
	JWTLifetimeNs          int64
	RespMaxMsgs            int
	RespTTLNs              int64
	BearerToken            bool
	ProxyRequired          bool
}

// goalign:ignore
type NATSAccountUserUpdate struct {
	SigningGroup           string
	Tags                   []string
	PubAllow               []string
	PubDeny                []string
	SubAllow               []string
	SubDeny                []string
	AllowedConnectionTypes []string
	SrcCIDRs               []string
	TimesLocale            string
	TimeRanges             []NATSUserTimeRange
	MaxSubs                int64
	MaxPayload             int64
	MaxData                int64
	JWTLifetimeNs          int64
	RespMaxMsgs            int
	RespTTLNs              int64
	BearerToken            bool
	ProxyRequired          bool
}

const natsUserSelectCols = `
	id, cluster_id, account_name, name, public_key, signing_group, jwt,
	COALESCE(assigned_user_id::text, ''),
	tags, pub_allow, pub_deny, sub_allow, sub_deny,
	max_subs, max_payload, jwt_lifetime_ns,
	bearer_token, proxy_required, allowed_connection_types, src_cidrs,
	times_locale, time_ranges, resp_max_msgs, resp_ttl_ns, max_data,
	created_at, updated_at`

func scanNATSAccountUser(scan func(dest ...any) error) (NATSAccountUser, string, error) {
	var item NATSAccountUser
	var jwtStr string
	var timeRangesJSON []byte
	err := scan(
		&item.ID, &item.ClusterID, &item.AccountName, &item.Name, &item.PublicKey, &item.SigningGroup, &jwtStr,
		&item.AssignedUserID,
		&item.Tags, &item.PubAllow, &item.PubDeny, &item.SubAllow, &item.SubDeny,
		&item.MaxSubs, &item.MaxPayload, &item.JWTLifetimeNs,
		&item.BearerToken, &item.ProxyRequired, &item.AllowedConnectionTypes, &item.SrcCIDRs,
		&item.TimesLocale, &timeRangesJSON, &item.RespMaxMsgs, &item.RespTTLNs, &item.MaxData,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return NATSAccountUser{}, "", err
	}
	item.TimeRanges = decodeTimeRanges(timeRangesJSON)
	normalizeNATSUserConfig(&item)
	item.HasJWT = jwtStr != ""
	item.JWTIssued = jwtStr != ""
	return item, jwtStr, nil
}

func normalizeNATSUserConfig(item *NATSAccountUser) {
	item.Tags = nonNilStrings(item.Tags)
	item.PubAllow = nonNilStrings(item.PubAllow)
	item.PubDeny = nonNilStrings(item.PubDeny)
	item.SubAllow = nonNilStrings(item.SubAllow)
	item.SubDeny = nonNilStrings(item.SubDeny)
	item.AllowedConnectionTypes = nonNilStrings(item.AllowedConnectionTypes)
	item.SrcCIDRs = nonNilStrings(item.SrcCIDRs)
	if item.TimeRanges == nil {
		item.TimeRanges = []NATSUserTimeRange{}
	}
}

func normalizeUserCreateLimits(in *NATSAccountUserCreate) {
	in.Tags = nonNilStrings(in.Tags)
	in.PubAllow = nonNilStrings(in.PubAllow)
	in.PubDeny = nonNilStrings(in.PubDeny)
	in.SubAllow = nonNilStrings(in.SubAllow)
	in.SubDeny = nonNilStrings(in.SubDeny)
	in.AllowedConnectionTypes = nonNilStrings(in.AllowedConnectionTypes)
	in.SrcCIDRs = nonNilStrings(in.SrcCIDRs)
	if in.TimeRanges == nil {
		in.TimeRanges = []NATSUserTimeRange{}
	}
	if in.MaxSubs == 0 {
		in.MaxSubs = -1
	}
	if in.MaxPayload == 0 {
		in.MaxPayload = -1
	}
	if in.MaxData == 0 {
		in.MaxData = -1
	}
}

func normalizeUserUpdateLimits(in *NATSAccountUserUpdate) {
	in.Tags = nonNilStrings(in.Tags)
	in.PubAllow = nonNilStrings(in.PubAllow)
	in.PubDeny = nonNilStrings(in.PubDeny)
	in.SubAllow = nonNilStrings(in.SubAllow)
	in.SubDeny = nonNilStrings(in.SubDeny)
	in.AllowedConnectionTypes = nonNilStrings(in.AllowedConnectionTypes)
	in.SrcCIDRs = nonNilStrings(in.SrcCIDRs)
	if in.TimeRanges == nil {
		in.TimeRanges = []NATSUserTimeRange{}
	}
	if in.MaxSubs == 0 {
		in.MaxSubs = -1
	}
	if in.MaxPayload == 0 {
		in.MaxPayload = -1
	}
	if in.MaxData == 0 {
		in.MaxData = -1
	}
}

func decodeTimeRanges(raw []byte) []NATSUserTimeRange {
	if len(raw) == 0 {
		return []NATSUserTimeRange{}
	}
	var out []NATSUserTimeRange
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []NATSUserTimeRange{}
	}
	return out
}

func encodeTimeRanges(in []NATSUserTimeRange) []byte {
	if in == nil {
		in = []NATSUserTimeRange{}
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

//nolint:govet // fieldalignment conflicts with embedded-first rule for NATSAccountUser
type NATSAccountUserCreds struct {
	NATSAccountUser

	Seed string `json:"seed,omitempty"`
	JWT  string `json:"jwt,omitempty"`
	Cred string `json:"creds,omitempty"`
}

func (s *Store) ListNATSAccountUsers(ctx context.Context, clusterID, accountName string) ([]NATSAccountUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+natsUserSelectCols+`
		FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2
		ORDER BY name ASC`, clusterID, accountName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]NATSAccountUser, 0)
	for rows.Next() {
		item, _, err := scanNATSAccountUser(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateNATSAccountUser(ctx context.Context, in NATSAccountUserCreate) (NATSAccountUserCreds, error) {
	return s.CreateNATSAccountUserWithSeed(ctx, in, "")
}

func (s *Store) CreateNATSAccountUserWithSeed(ctx context.Context, in NATSAccountUserCreate, accountSeed string) (NATSAccountUserCreds, error) {
	if in.SigningGroup == "" {
		in.SigningGroup = "Default"
	}
	if in.AccountName == "" {
		in.AccountName = "Default"
	}
	normalizeUserCreateLimits(&in)
	_ = s.EnsureDefaultSigningGroup(ctx, in.ClusterID, in.AccountName)
	kp, err := nkeys.CreateUser()
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	pub, err := kp.PublicKey()
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	seed, err := kp.Seed()
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	encSeed, err := s.encryptToken(string(seed))
	if err != nil {
		return NATSAccountUserCreds{}, fmt.Errorf("encrypt seed: %w", err)
	}

	user := NATSAccountUser{
		ClusterID: in.ClusterID, AccountName: in.AccountName, Name: in.Name, SigningGroup: in.SigningGroup,
		Tags: in.Tags, PubAllow: in.PubAllow, PubDeny: in.PubDeny, SubAllow: in.SubAllow, SubDeny: in.SubDeny,
		AllowedConnectionTypes: in.AllowedConnectionTypes, SrcCIDRs: in.SrcCIDRs,
		TimesLocale: in.TimesLocale, TimeRanges: in.TimeRanges,
		MaxSubs: in.MaxSubs, MaxPayload: in.MaxPayload, MaxData: in.MaxData, JWTLifetimeNs: in.JWTLifetimeNs,
		RespMaxMsgs: in.RespMaxMsgs, RespTTLNs: in.RespTTLNs,
		BearerToken: in.BearerToken, ProxyRequired: in.ProxyRequired,
	}

	userJWT := ""
	if accountSeed != "" {
		userJWT, err = mintUserJWT(ctx, s, in.ClusterID, in.AccountName, user, string(seed), accountSeed)
		if err != nil {
			return NATSAccountUserCreds{}, err
		}
	}

	id := uuid.NewString()
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		INSERT INTO nats_account_users
			(id, cluster_id, account_name, name, public_key, seed_encrypted, jwt, signing_group,
			 tags, pub_allow, pub_deny, sub_allow, sub_deny, max_subs, max_payload, jwt_lifetime_ns,
			 bearer_token, proxy_required, allowed_connection_types, src_cidrs,
			 times_locale, time_ranges, resp_max_msgs, resp_ttl_ns, max_data,
			 created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$26)`,
		id, in.ClusterID, in.AccountName, in.Name, pub, encSeed, userJWT, in.SigningGroup,
		in.Tags, in.PubAllow, in.PubDeny, in.SubAllow, in.SubDeny, in.MaxSubs, in.MaxPayload, in.JWTLifetimeNs,
		in.BearerToken, in.ProxyRequired, in.AllowedConnectionTypes, in.SrcCIDRs,
		in.TimesLocale, encodeTimeRanges(in.TimeRanges), in.RespMaxMsgs, in.RespTTLNs, in.MaxData, now)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}

	user.ID = id
	user.PublicKey = pub
	user.HasJWT = userJWT != ""
	user.JWTIssued = userJWT != ""
	user.CreatedAt = now
	user.UpdatedAt = now
	out := NATSAccountUserCreds{NATSAccountUser: user, Seed: string(seed), JWT: userJWT}
	out.Cred = formatCreds(userJWT, string(seed))
	return out, nil
}

func (s *Store) DeleteNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`, clusterID, accountName, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) (NATSAccountUser, error) {
	item, _, err := scanNATSAccountUser(s.pool.QueryRow(ctx, `
		SELECT `+natsUserSelectCols+`
		FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`, clusterID, accountName, userID).Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return NATSAccountUser{}, ErrNotFound
	}
	if err != nil {
		return NATSAccountUser{}, err
	}
	return item, nil
}

func (s *Store) GetNATSAccountUserCreds(ctx context.Context, clusterID, accountName, userID string) (NATSAccountUserCreds, error) {
	var item NATSAccountUser
	var encSeed, jwtStr string
	var timeRangesJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, account_name, name, public_key, seed_encrypted, jwt, signing_group,
		       COALESCE(assigned_user_id::text, ''),
		       tags, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_subs, max_payload, jwt_lifetime_ns,
		       bearer_token, proxy_required, allowed_connection_types, src_cidrs,
		       times_locale, time_ranges, resp_max_msgs, resp_ttl_ns, max_data,
		       created_at, updated_at
		FROM nats_account_users
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`, clusterID, accountName, userID).
		Scan(
			&item.ID, &item.ClusterID, &item.AccountName, &item.Name, &item.PublicKey, &encSeed, &jwtStr, &item.SigningGroup,
			&item.AssignedUserID,
			&item.Tags, &item.PubAllow, &item.PubDeny, &item.SubAllow, &item.SubDeny,
			&item.MaxSubs, &item.MaxPayload, &item.JWTLifetimeNs,
			&item.BearerToken, &item.ProxyRequired, &item.AllowedConnectionTypes, &item.SrcCIDRs,
			&item.TimesLocale, &timeRangesJSON, &item.RespMaxMsgs, &item.RespTTLNs, &item.MaxData,
			&item.CreatedAt, &item.UpdatedAt,
		)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NATSAccountUserCreds{}, ErrNotFound
		}
		return NATSAccountUserCreds{}, err
	}
	item.TimeRanges = decodeTimeRanges(timeRangesJSON)
	normalizeNATSUserConfig(&item)
	item.HasJWT = jwtStr != ""
	item.JWTIssued = jwtStr != ""
	seed, err := s.decryptToken(encSeed)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	out := NATSAccountUserCreds{NATSAccountUser: item, Seed: seed, JWT: jwtStr}
	out.Cred = formatCreds(jwtStr, seed)
	return out, nil
}

func (s *Store) UpdateNATSAccountUser(ctx context.Context, clusterID, accountName, userID string, in NATSAccountUserUpdate, accountSeed string) (NATSAccountUser, error) {
	if in.SigningGroup == "" {
		in.SigningGroup = "Default"
	}
	normalizeUserUpdateLimits(&in)
	existing, err := s.GetNATSAccountUserCreds(ctx, clusterID, accountName, userID)
	if err != nil {
		return NATSAccountUser{}, err
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		UPDATE nats_account_users SET
			signing_group = $4, tags = $5, pub_allow = $6, pub_deny = $7, sub_allow = $8, sub_deny = $9,
			max_subs = $10, max_payload = $11, jwt_lifetime_ns = $12,
			bearer_token = $13, proxy_required = $14, allowed_connection_types = $15, src_cidrs = $16,
			times_locale = $17, time_ranges = $18, resp_max_msgs = $19, resp_ttl_ns = $20, max_data = $21,
			updated_at = $22
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, userID, in.SigningGroup,
		in.Tags, in.PubAllow, in.PubDeny, in.SubAllow, in.SubDeny,
		in.MaxSubs, in.MaxPayload, in.JWTLifetimeNs,
		in.BearerToken, in.ProxyRequired, in.AllowedConnectionTypes, in.SrcCIDRs,
		in.TimesLocale, encodeTimeRanges(in.TimeRanges), in.RespMaxMsgs, in.RespTTLNs, in.MaxData, now)
	if err != nil {
		return NATSAccountUser{}, err
	}
	updated, err := s.GetNATSAccountUser(ctx, clusterID, accountName, userID)
	if err != nil {
		return NATSAccountUser{}, err
	}
	if accountSeed != "" {
		userJWT, mintErr := mintUserJWT(ctx, s, clusterID, accountName, updated, existing.Seed, accountSeed)
		if mintErr != nil {
			return NATSAccountUser{}, mintErr
		}
		_, err = s.pool.Exec(ctx, `
			UPDATE nats_account_users SET jwt = $4, updated_at = $5
			WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
			clusterID, accountName, userID, userJWT, now)
		if err != nil {
			return NATSAccountUser{}, err
		}
		updated.HasJWT = true
		updated.JWTIssued = true
		updated.UpdatedAt = now
	}
	return updated, nil
}

func (s *Store) RotateNATSAccountUser(ctx context.Context, clusterID, accountName, userID, accountSeed string) (NATSAccountUserCreds, error) {
	existing, err := s.GetNATSAccountUser(ctx, clusterID, accountName, userID)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	kp, err := nkeys.CreateUser()
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	pub, err := kp.PublicKey()
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	seed, err := kp.Seed()
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	encSeed, err := s.encryptToken(string(seed))
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	userJWT := ""
	if accountSeed != "" {
		userJWT, err = mintUserJWT(ctx, s, clusterID, accountName, existing, string(seed), accountSeed)
		if err != nil {
			return NATSAccountUserCreds{}, err
		}
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		UPDATE nats_account_users
		SET public_key = $4, seed_encrypted = $5, jwt = $6, updated_at = $7
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, userID, pub, encSeed, userJWT, now)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	out := NATSAccountUserCreds{
		NATSAccountUser: existing,
		Seed:            string(seed),
		JWT:             userJWT,
	}
	out.PublicKey = pub
	out.HasJWT = userJWT != ""
	out.JWTIssued = userJWT != ""
	out.UpdatedAt = now
	out.Cred = formatCreds(userJWT, string(seed))
	return out, nil
}

func (s *Store) MintNATSAccountUserJWT(ctx context.Context, clusterID, accountName, userID, accountSeed string) (NATSAccountUserCreds, error) {
	if accountSeed == "" {
		return NATSAccountUserCreds{}, errors.New("NATS_ACCOUNT_SEED is not configured")
	}
	creds, err := s.GetNATSAccountUserCreds(ctx, clusterID, accountName, userID)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	userJWT, err := mintUserJWT(ctx, s, clusterID, accountName, creds.NATSAccountUser, creds.Seed, accountSeed)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, `
		UPDATE nats_account_users SET jwt = $4, updated_at = $5
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, userID, userJWT, now)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	creds.JWT = userJWT
	creds.HasJWT = true
	creds.JWTIssued = true
	creds.UpdatedAt = now
	creds.Cred = formatCreds(userJWT, creds.Seed)
	return creds, nil
}

func (s *Store) AssignNATSAccountUserPerson(ctx context.Context, clusterID, accountName, natsUserID, consoleUserID string) (NATSAccountUser, error) {
	now := time.Now().UTC()
	var assigned any
	if consoleUserID == "" {
		assigned = nil
	} else {
		assigned = consoleUserID
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE nats_account_users SET assigned_user_id = $4, updated_at = $5
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, natsUserID, assigned, now)
	if err != nil {
		return NATSAccountUser{}, err
	}
	if tag.RowsAffected() == 0 {
		return NATSAccountUser{}, ErrNotFound
	}
	if consoleUserID != "" {
		if _, err := s.UpsertAccessGrant(ctx, AccessGrantUpsert{
			UserID:       consoleUserID,
			ResourceType: ResourceNATSUser,
			ResourceKey:  domainAccountNATSUserKey(clusterID, accountName, natsUserID),
			Role:         GrantAdmin,
		}); err != nil {
			return NATSAccountUser{}, err
		}
	}
	return s.GetNATSAccountUser(ctx, clusterID, accountName, natsUserID)
}

func domainAccountNATSUserKey(clusterID, accountName, natsUserID string) string {
	return clusterID + ":" + accountName + ":" + natsUserID
}

func formatCreds(userJWT, seed string) string {
	if userJWT == "" {
		return fmt.Sprintf("-----BEGIN NATS USER SEED-----\n%s\n-----END NATS USER SEED-----\n", seed)
	}
	return fmt.Sprintf("-----BEGIN NATS USER JWT-----\n%s\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n%s\n------END USER NKEY SEED------\n", userJWT, seed)
}

type NATSAccountExport struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ID          string    `json:"id"`
	ClusterID   string    `json:"clusterId"`
	AccountName string    `json:"accountName"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
}

type NATSAccountExportCreate struct {
	ClusterID   string
	AccountName string
	Kind        string
	Name        string
	Subject     string
	Description string
}

func (s *Store) ListNATSAccountExports(ctx context.Context, clusterID, accountName, kind string) ([]NATSAccountExport, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at
		FROM nats_account_exports
		WHERE cluster_id = $1 AND account_name = $2 AND ($3 = '' OR kind = $3)
		ORDER BY name ASC`, clusterID, accountName, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]NATSAccountExport, 0)
	for rows.Next() {
		var item NATSAccountExport
		if err := rows.Scan(&item.ID, &item.ClusterID, &item.AccountName, &item.Kind, &item.Name, &item.Subject, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CreateNATSAccountExport(ctx context.Context, in NATSAccountExportCreate) (NATSAccountExport, error) {
	if in.AccountName == "" {
		in.AccountName = "Default"
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	var item NATSAccountExport
	err := s.pool.QueryRow(ctx, `
		INSERT INTO nats_account_exports
			(id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at`,
		id, in.ClusterID, in.AccountName, in.Kind, in.Name, in.Subject, in.Description, now, now).
		Scan(&item.ID, &item.ClusterID, &item.AccountName, &item.Kind, &item.Name, &item.Subject, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return NATSAccountExport{}, err
	}
	return item, nil
}

type NATSAccountExportUpdate struct {
	Name        string
	Subject     string
	Description string
}

func (s *Store) UpdateNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string, in NATSAccountExportUpdate) (NATSAccountExport, error) {
	now := time.Now().UTC()
	var item NATSAccountExport
	err := s.pool.QueryRow(ctx, `
		UPDATE nats_account_exports SET
			name = $4, subject = $5, description = $6, updated_at = $7
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3
		RETURNING id, cluster_id, account_name, kind, name, subject, description, created_at, updated_at`,
		clusterID, accountName, exportID, in.Name, in.Subject, in.Description, now).
		Scan(&item.ID, &item.ClusterID, &item.AccountName, &item.Kind, &item.Name, &item.Subject, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return NATSAccountExport{}, ErrNotFound
	}
	if err != nil {
		return NATSAccountExport{}, err
	}
	return item, nil
}

func (s *Store) DeleteNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM nats_account_exports
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`, clusterID, accountName, exportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
