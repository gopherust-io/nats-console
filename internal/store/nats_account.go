package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nkeys"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type (
	NATSUserTimeRange      = domain.NATSUserTimeRange
	NATSAccountUser        = domain.NATSAccountUser
	NATSAccountUserCreate  = domain.NATSAccountUserCreate
	NATSAccountUserUpdate  = domain.NATSAccountUserUpdate
	NATSAccountUserCreds   = domain.NATSAccountUserCreds
	NATSAccountExport      = domain.NATSAccountExport
	NATSAccountExportCreate = domain.NATSAccountExportCreate
	NATSAccountExportUpdate = domain.NATSAccountExportUpdate
)

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
	item.HasJWT = !strings.IsEmpty(jwtStr)
	item.JWTIssued = !strings.IsEmpty(jwtStr)
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
	if err := serializer.Unmarshal(raw, &out); err != nil || out == nil {
		return []NATSUserTimeRange{}
	}
	return out
}

func encodeTimeRanges(in []NATSUserTimeRange) []byte {
	if in == nil {
		in = []NATSUserTimeRange{}
	}
	raw, err := serializer.Marshal(in)
	if err != nil {
		return strings.StringToBytes("[]")
	}
	return raw
}

func (s *Store) ListNATSAccountUsers(ctx context.Context, clusterID, accountName string) ([]NATSAccountUser, error) {
	rows, err := s.pool.Query(ctx, queryListNATSAccountUsers, clusterID, accountName)
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
	if strings.IsEmpty(in.SigningGroup) {
		in.SigningGroup = "Default"
	}
	if strings.IsEmpty(in.AccountName) {
		in.AccountName = "Default"
	}
	normalizeUserCreateLimits(&in)
	if err := s.EnsureDefaultSigningGroup(ctx, in.ClusterID, in.AccountName); err != nil {
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
	encSeed, err := s.encryptToken(strings.BytesToString(seed))
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
	if !strings.IsEmpty(accountSeed) {
		userJWT, err = mintUserJWT(ctx, s, in.ClusterID, in.AccountName, user, strings.BytesToString(seed), accountSeed)
		if err != nil {
			return NATSAccountUserCreds{}, err
		}
	}

	id := newID()
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, queryInsertNATSAccountUser,
		id, in.ClusterID, in.AccountName, in.Name, pub, encSeed, userJWT, in.SigningGroup,
		in.Tags, in.PubAllow, in.PubDeny, in.SubAllow, in.SubDeny, in.MaxSubs, in.MaxPayload, in.JWTLifetimeNs,
		in.BearerToken, in.ProxyRequired, in.AllowedConnectionTypes, in.SrcCIDRs,
		in.TimesLocale, encodeTimeRanges(in.TimeRanges), in.RespMaxMsgs, in.RespTTLNs, in.MaxData, now)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}

	user.ID = id
	user.PublicKey = pub
	user.HasJWT = !strings.IsEmpty(userJWT)
	user.JWTIssued = !strings.IsEmpty(userJWT)
	user.CreatedAt = now
	user.UpdatedAt = now
	out := NATSAccountUserCreds{NATSAccountUser: user, Seed: strings.BytesToString(seed), JWT: userJWT}
	out.Cred = formatCreds(userJWT, strings.BytesToString(seed))
	return out, nil
}

func (s *Store) DeleteNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) error {
	tag, err := s.pool.Exec(ctx, queryDeleteNATSAccountUser, clusterID, accountName, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) (NATSAccountUser, error) {
	item, _, err := scanNATSAccountUser(s.pool.QueryRow(ctx, queryGetNATSAccountUser, clusterID, accountName, userID).Scan)
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
	err := s.pool.QueryRow(ctx, queryGetNATSAccountUserCreds, clusterID, accountName, userID).
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
	item.HasJWT = !strings.IsEmpty(jwtStr)
	item.JWTIssued = !strings.IsEmpty(jwtStr)
	seed, err := s.decryptToken(encSeed)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	out := NATSAccountUserCreds{NATSAccountUser: item, Seed: seed, JWT: jwtStr}
	out.Cred = formatCreds(jwtStr, seed)
	return out, nil
}

func (s *Store) UpdateNATSAccountUser(ctx context.Context, clusterID, accountName, userID string, in NATSAccountUserUpdate, accountSeed string) (NATSAccountUser, error) {
	if strings.IsEmpty(in.SigningGroup) {
		in.SigningGroup = "Default"
	}
	normalizeUserUpdateLimits(&in)
	existing, err := s.GetNATSAccountUserCreds(ctx, clusterID, accountName, userID)
	if err != nil {
		return NATSAccountUser{}, err
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, queryUpdateNATSAccountUser,
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
	if !strings.IsEmpty(accountSeed) {
		userJWT, mintErr := mintUserJWT(ctx, s, clusterID, accountName, updated, existing.Seed, accountSeed)
		if mintErr != nil {
			return NATSAccountUser{}, mintErr
		}
		_, err = s.pool.Exec(ctx, queryUpdateNATSAccountUserJWT,
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
	encSeed, err := s.encryptToken(strings.BytesToString(seed))
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	userJWT := ""
	if !strings.IsEmpty(accountSeed) {
		userJWT, err = mintUserJWT(ctx, s, clusterID, accountName, existing, strings.BytesToString(seed), accountSeed)
		if err != nil {
			return NATSAccountUserCreds{}, err
		}
	}
	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx, queryRotateNATSAccountUser,
		clusterID, accountName, userID, pub, encSeed, userJWT, now)
	if err != nil {
		return NATSAccountUserCreds{}, err
	}
	out := NATSAccountUserCreds{
		NATSAccountUser: existing,
		Seed:            strings.BytesToString(seed),
		JWT:             userJWT,
	}
	out.PublicKey = pub
	out.HasJWT = !strings.IsEmpty(userJWT)
	out.JWTIssued = !strings.IsEmpty(userJWT)
	out.UpdatedAt = now
	out.Cred = formatCreds(userJWT, strings.BytesToString(seed))
	return out, nil
}

func (s *Store) MintNATSAccountUserJWT(ctx context.Context, clusterID, accountName, userID, accountSeed string) (NATSAccountUserCreds, error) {
	if strings.IsEmpty(accountSeed) {
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
	_, err = s.pool.Exec(ctx, queryUpdateNATSAccountUserJWT,
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
	current, err := s.GetNATSAccountUser(ctx, clusterID, accountName, natsUserID)
	if err != nil {
		return NATSAccountUser{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NATSAccountUser{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var assigned any
	if strings.IsEmpty(consoleUserID) {
		assigned = nil
	} else {
		assigned = consoleUserID
	}
	tag, err := tx.Exec(ctx, queryAssignNATSAccountUserPerson,
		clusterID, accountName, natsUserID, assigned, now)
	if err != nil {
		return NATSAccountUser{}, err
	}
	if tag.RowsAffected() == 0 {
		return NATSAccountUser{}, ErrNotFound
	}

	resourceKey := domainAccountNATSUserKey(clusterID, accountName, natsUserID)
	prevAssignee := current.AssignedUserID
	if !strings.IsEmpty(prevAssignee) && prevAssignee != consoleUserID {
		if _, err := tx.Exec(ctx, queryDeleteAccessGrantByResource,
			prevAssignee, ResourceNATSUser, resourceKey); err != nil {
			return NATSAccountUser{}, err
		}
	}
	if !strings.IsEmpty(consoleUserID) {
		grantID := newID()
		if _, err := tx.Exec(ctx, queryUpsertAccessGrantNoReturning,
			grantID, consoleUserID, ResourceNATSUser, resourceKey, GrantAdmin, now); err != nil {
			return NATSAccountUser{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return NATSAccountUser{}, err
	}
	return s.GetNATSAccountUser(ctx, clusterID, accountName, natsUserID)
}

func domainAccountNATSUserKey(clusterID, accountName, natsUserID string) string {
	return clusterID + ":" + accountName + ":" + natsUserID
}

func formatCreds(userJWT, seed string) string {
	if strings.IsEmpty(userJWT) {
		return fmt.Sprintf("-----BEGIN NATS USER SEED-----\n%s\n-----END NATS USER SEED-----\n", seed)
	}
	return fmt.Sprintf("-----BEGIN NATS USER JWT-----\n%s\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n%s\n------END USER NKEY SEED------\n", userJWT, seed)
}

func (s *Store) ListNATSAccountExports(ctx context.Context, clusterID, accountName, kind string) ([]NATSAccountExport, error) {
	rows, err := s.pool.Query(ctx, queryListNATSAccountExports, clusterID, accountName, kind)
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
	if strings.IsEmpty(in.AccountName) {
		in.AccountName = "Default"
	}
	id := newID()
	now := time.Now().UTC()
	var item NATSAccountExport
	err := s.pool.QueryRow(ctx, queryInsertNATSAccountExport,
		id, in.ClusterID, in.AccountName, in.Kind, in.Name, in.Subject, in.Description, now, now).
		Scan(&item.ID, &item.ClusterID, &item.AccountName, &item.Kind, &item.Name, &item.Subject, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return NATSAccountExport{}, err
	}
	return item, nil
}

func (s *Store) UpdateNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string, in NATSAccountExportUpdate) (NATSAccountExport, error) {
	now := time.Now().UTC()
	var item NATSAccountExport
	err := s.pool.QueryRow(ctx, queryUpdateNATSAccountExport,
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
	tag, err := s.pool.Exec(ctx, queryDeleteNATSAccountExport, clusterID, accountName, exportID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
