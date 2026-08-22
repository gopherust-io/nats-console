package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var (
	ErrSigningGroupProtected = domain.ErrSigningGroupProtected
	ErrSigningGroupInUse     = domain.ErrSigningGroupInUse
)

type (
	SigningGroup       = domain.SigningGroup
	SigningGroupCreate = domain.SigningGroupCreate
	SigningGroupUpdate = domain.SigningGroupUpdate
)

func (db *DB) EnsureDefaultSigningGroup(ctx context.Context, clusterID, accountName string) error {
	if strings.IsEmpty(accountName) {
		accountName = "Default"
	}
	_, err := db.pool.Exec(ctx, queryEnsureDefaultSigningGroup, newID(), clusterID, accountName)
	return err
}

func (db *DB) ListSigningGroups(ctx context.Context, clusterID, accountName string) ([]SigningGroup, error) {
	if err := db.EnsureDefaultSigningGroup(ctx, clusterID, accountName); err != nil {
		return nil, err
	}
	rows, err := db.pool.Query(ctx, queryListSigningGroups, clusterID, accountName)
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

func (db *DB) CreateSigningGroup(ctx context.Context, in SigningGroupCreate) (SigningGroup, error) {
	if strings.IsEmpty(in.AccountName) {
		in.AccountName = "Default"
	}
	if strings.IsEmpty(in.Name) {
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
	id := newID()
	now := time.Now().UTC()
	_, err := db.pool.Exec(ctx, queryInsertSigningGroup,
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

func (db *DB) GetSigningGroup(ctx context.Context, clusterID, accountName, name string) (SigningGroup, error) {
	var g SigningGroup
	err := db.pool.QueryRow(ctx, queryGetSigningGroupByName, clusterID, accountName, name).
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

func (db *DB) UpdateSigningGroup(ctx context.Context, clusterID, accountName, groupID string, in SigningGroupUpdate) (SigningGroup, error) {
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
	tag, err := db.pool.Exec(ctx, queryUpdateSigningGroup,
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
	err = db.pool.QueryRow(ctx, queryGetSigningGroupByID, clusterID, accountName, groupID).
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

func (db *DB) DeleteSigningGroup(ctx context.Context, clusterID, accountName, groupID string) error {
	var name string
	err := db.pool.QueryRow(ctx, queryGetSigningGroupName, clusterID, accountName, groupID).Scan(&name)
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
	if err := db.pool.QueryRow(ctx, querySigningGroupInUse, clusterID, accountName, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrSigningGroupInUse
	}

	tag, err := db.pool.Exec(ctx, queryDeleteSigningGroup, clusterID, accountName, groupID)
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
