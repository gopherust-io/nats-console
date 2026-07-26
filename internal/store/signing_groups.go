package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrSigningGroupProtected = errors.New("default signing group cannot be deleted")
	ErrSigningGroupInUse     = errors.New("signing group is still used by one or more NATS users")
)

// goalign:ignore
type SigningGroup struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ID          string    `json:"id"`
	ClusterID   string    `json:"clusterId"`
	AccountName string    `json:"accountName"`
	Name        string    `json:"name"`
	PubAllow    []string  `json:"pubAllow"`
	PubDeny     []string  `json:"pubDeny"`
	SubAllow    []string  `json:"subAllow"`
	SubDeny     []string  `json:"subDeny"`
	MaxData     int64     `json:"maxData"`
	MaxPayload  int64     `json:"maxPayload"`
	MaxSubs     int64     `json:"maxSubs"`
	Scoped      bool      `json:"scoped"`
}

// goalign:ignore
type SigningGroupCreate struct {
	ClusterID   string
	AccountName string
	Name        string
	PubAllow    []string
	PubDeny     []string
	SubAllow    []string
	SubDeny     []string
	MaxData     int64
	MaxPayload  int64
	MaxSubs     int64
	Scoped      bool
}

// goalign:ignore
type SigningGroupUpdate struct {
	PubAllow   []string
	PubDeny    []string
	SubAllow   []string
	SubDeny    []string
	MaxData    int64
	MaxPayload int64
	MaxSubs    int64
	Scoped     bool
}

func (s *Store) EnsureDefaultSigningGroup(ctx context.Context, clusterID, accountName string) error {
	if accountName == "" {
		accountName = "Default"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nats_signing_groups
			(id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny, max_data, max_payload, max_subs, created_at, updated_at)
		VALUES ($1, $2, $3, 'Default', false, '{}', '{}', '{}', '{}', -1, -1, -1, NOW(), NOW())
		ON CONFLICT (cluster_id, account_name, name) DO NOTHING`,
		uuid.NewString(), clusterID, accountName)
	return err
}

func (s *Store) ListSigningGroups(ctx context.Context, clusterID, accountName string) ([]SigningGroup, error) {
	if err := s.EnsureDefaultSigningGroup(ctx, clusterID, accountName); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_data, max_payload, max_subs, created_at, updated_at
		FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2
		ORDER BY name ASC`, clusterID, accountName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SigningGroup, 0)
	for rows.Next() {
		var g SigningGroup
		if err := rows.Scan(&g.ID, &g.ClusterID, &g.AccountName, &g.Name, &g.Scoped,
			&g.PubAllow, &g.PubDeny, &g.SubAllow, &g.SubDeny,
			&g.MaxData, &g.MaxPayload, &g.MaxSubs, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.PubAllow = nonNilStrings(g.PubAllow)
		g.PubDeny = nonNilStrings(g.PubDeny)
		g.SubAllow = nonNilStrings(g.SubAllow)
		g.SubDeny = nonNilStrings(g.SubDeny)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) CreateSigningGroup(ctx context.Context, in SigningGroupCreate) (SigningGroup, error) {
	if in.AccountName == "" {
		in.AccountName = "Default"
	}
	if in.Name == "" {
		return SigningGroup{}, errors.New("name required")
	}
	if in.MaxData == 0 {
		in.MaxData = -1
	}
	if in.MaxPayload == 0 {
		in.MaxPayload = -1
	}
	if in.MaxSubs == 0 {
		in.MaxSubs = -1
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nats_signing_groups
			(id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny, max_data, max_payload, max_subs, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13)`,
		id, in.ClusterID, in.AccountName, in.Name, in.Scoped,
		nonNilStrings(in.PubAllow), nonNilStrings(in.PubDeny), nonNilStrings(in.SubAllow), nonNilStrings(in.SubDeny),
		in.MaxData, in.MaxPayload, in.MaxSubs, now)
	if err != nil {
		return SigningGroup{}, err
	}
	return SigningGroup{
		ID: id, ClusterID: in.ClusterID, AccountName: in.AccountName, Name: in.Name,
		Scoped: in.Scoped, PubAllow: nonNilStrings(in.PubAllow), PubDeny: nonNilStrings(in.PubDeny),
		SubAllow: nonNilStrings(in.SubAllow), SubDeny: nonNilStrings(in.SubDeny),
		MaxData: in.MaxData, MaxPayload: in.MaxPayload, MaxSubs: in.MaxSubs,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *Store) GetSigningGroup(ctx context.Context, clusterID, accountName, name string) (SigningGroup, error) {
	var g SigningGroup
	err := s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_data, max_payload, max_subs, created_at, updated_at
		FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND name = $3`, clusterID, accountName, name).
		Scan(&g.ID, &g.ClusterID, &g.AccountName, &g.Name, &g.Scoped,
			&g.PubAllow, &g.PubDeny, &g.SubAllow, &g.SubDeny,
			&g.MaxData, &g.MaxPayload, &g.MaxSubs, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SigningGroup{}, ErrNotFound
	}
	if err != nil {
		return SigningGroup{}, err
	}
	g.PubAllow = nonNilStrings(g.PubAllow)
	g.PubDeny = nonNilStrings(g.PubDeny)
	g.SubAllow = nonNilStrings(g.SubAllow)
	g.SubDeny = nonNilStrings(g.SubDeny)
	return g, nil
}

func (s *Store) UpdateSigningGroup(ctx context.Context, clusterID, accountName, groupID string, in SigningGroupUpdate) (SigningGroup, error) {
	if in.MaxData == 0 {
		in.MaxData = -1
	}
	if in.MaxPayload == 0 {
		in.MaxPayload = -1
	}
	if in.MaxSubs == 0 {
		in.MaxSubs = -1
	}
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE nats_signing_groups SET
			scoped = $4, pub_allow = $5, pub_deny = $6, sub_allow = $7, sub_deny = $8,
			max_data = $9, max_payload = $10, max_subs = $11, updated_at = $12
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, groupID, in.Scoped,
		nonNilStrings(in.PubAllow), nonNilStrings(in.PubDeny), nonNilStrings(in.SubAllow), nonNilStrings(in.SubDeny),
		in.MaxData, in.MaxPayload, in.MaxSubs, now)
	if err != nil {
		return SigningGroup{}, err
	}
	if tag.RowsAffected() == 0 {
		return SigningGroup{}, ErrNotFound
	}
	var g SigningGroup
	err = s.pool.QueryRow(ctx, `
		SELECT id, cluster_id, account_name, name, scoped, pub_allow, pub_deny, sub_allow, sub_deny,
		       max_data, max_payload, max_subs, created_at, updated_at
		FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, groupID).
		Scan(&g.ID, &g.ClusterID, &g.AccountName, &g.Name, &g.Scoped,
			&g.PubAllow, &g.PubDeny, &g.SubAllow, &g.SubDeny,
			&g.MaxData, &g.MaxPayload, &g.MaxSubs, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return SigningGroup{}, err
	}
	g.PubAllow = nonNilStrings(g.PubAllow)
	g.PubDeny = nonNilStrings(g.PubDeny)
	g.SubAllow = nonNilStrings(g.SubAllow)
	g.SubDeny = nonNilStrings(g.SubDeny)
	return g, nil
}

func (s *Store) DeleteSigningGroup(ctx context.Context, clusterID, accountName, groupID string) error {
	var name string
	err := s.pool.QueryRow(ctx, `
		SELECT name FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3`,
		clusterID, accountName, groupID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if name == "Default" {
		return ErrSigningGroupProtected
	}

	var inUse bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM nats_account_users
			WHERE cluster_id = $1 AND account_name = $2 AND signing_group = $3
		)`, clusterID, accountName, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrSigningGroupInUse
	}

	tag, err := s.pool.Exec(ctx, `
		DELETE FROM nats_signing_groups
		WHERE cluster_id = $1 AND account_name = $2 AND id = $3 AND name <> 'Default'`,
		clusterID, accountName, groupID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
