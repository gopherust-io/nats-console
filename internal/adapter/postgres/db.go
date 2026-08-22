package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/repo"
)

type DB struct {
	db *repo.DB
}

var _ port.DB = (*DB)(nil)

func Open(ctx context.Context, databaseURL string, encryptor *crypto.Encryptor, pool repo.PoolConfig) (*DB, error) {
	if err := ValidateDatabaseURL(databaseURL); err != nil {
		return nil, err
	}
	st, err := repo.Open(ctx, databaseURL, encryptor, pool)
	if err != nil {
		return nil, err
	}
	return WrapStore(st), nil
}

func OpenWithConfig(ctx context.Context, cfg config.Config, encryptor *crypto.Encryptor) (*DB, error) {
	if cfg.TLSEnabled() {
		if err := ValidateDatabaseURL(cfg.DB.URL); err != nil {
			return nil, err
		}
	}
	st, err := repo.Open(ctx, cfg.DB.URL, encryptor, poolConfigFrom(cfg))
	if err != nil {
		return nil, err
	}
	return WrapStore(st), nil
}

func poolConfigFrom(cfg config.Config) repo.PoolConfig {
	return repo.PoolConfig{
		MaxConns:          boundedInt32(cfg.DB.MaxConns),
		MinConns:          boundedInt32(cfg.DB.MinConns),
		MaxConnLifetime:   cfg.DB.MaxConnLifetime,
		MaxConnIdleTime:   cfg.DB.MaxConnIdleTime,
		HealthCheckPeriod: cfg.DB.HealthCheckPeriod,
	}
}

func boundedInt32(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < math.MinInt32 {
		return math.MinInt32
	}
	return int32(n)
}

func WrapStore(st *repo.DB) *DB {
	return &DB{db: st}
}

func (u *DB) Stop() {
	if u != nil && u.db != nil {
		u.db.Stop()
	}
}

func (u *DB) DB() *repo.DB {
	return u.db
}

func (u *DB) CountClusters(ctx context.Context) (int, error) {
	return u.db.CountClusters(ctx)
}

func (u *DB) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	items, err := u.db.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Cluster, len(items))
	for i, item := range items {
		out[i] = toDomainCluster(item)
	}
	return out, nil
}

func (u *DB) GetCluster(ctx context.Context, id string) (domain.Cluster, error) {
	item, err := u.db.GetCluster(ctx, id)
	if err != nil {
		return domain.Cluster{}, mapError(err)
	}
	return toDomainCluster(item), nil
}

func (u *DB) GetDefaultCluster(ctx context.Context) (domain.Cluster, error) {
	item, err := u.db.GetDefaultCluster(ctx)
	if err != nil {
		return domain.Cluster{}, mapError(err)
	}
	return toDomainCluster(item), nil
}

func (u *DB) CreateCluster(ctx context.Context, in domain.ClusterCreate) (domain.Cluster, error) {
	item, err := u.db.CreateCluster(ctx, repo.ClusterCreate{
		Name:          in.Name,
		NATSURL:       in.NATSURL,
		MonitoringURL: in.MonitoringURL,
		CredsFilePath: in.CredsFilePath,
		Token:         in.Token,
		IsDefault:     in.IsDefault,
	})
	if err != nil {
		return domain.Cluster{}, err
	}
	return toDomainCluster(item), nil
}

func (u *DB) UpdateCluster(ctx context.Context, id string, in domain.ClusterUpdate) (domain.Cluster, error) {
	item, err := u.db.UpdateCluster(ctx, id, repo.ClusterUpdate{
		Name:          in.Name,
		NATSURL:       in.NATSURL,
		MonitoringURL: in.MonitoringURL,
		CredsFilePath: in.CredsFilePath,
		Token:         in.Token,
		IsDefault:     in.IsDefault,
	})
	if err != nil {
		return domain.Cluster{}, mapError(err)
	}
	return toDomainCluster(item), nil
}

func (u *DB) DeleteCluster(ctx context.Context, id string) error {
	return mapError(u.db.DeleteCluster(ctx, id))
}

func (u *DB) Ping(ctx context.Context) error {
	return u.db.Ping(ctx)
}

func (u *DB) List(ctx context.Context) ([]domain.User, error) {
	items, err := u.db.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, len(items))
	for i, item := range items {
		out[i] = toDomainUser(item)
	}
	return out, nil
}

func (u *DB) GetByID(ctx context.Context, id string) (domain.User, error) {
	item, err := u.db.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(item), nil
}

func (u *DB) GetByUsername(ctx context.Context, username string) (domain.User, string, error) {
	item, hash, err := u.db.GetUserByUsername(ctx, username)
	if err != nil {
		return domain.User{}, "", mapError(err)
	}
	return toDomainUser(item), hash, nil
}

func (u *DB) GetByOIDCSub(ctx context.Context, sub string) (domain.User, error) {
	item, err := u.db.GetUserByOIDCSub(ctx, sub)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(item), nil
}

func (u *DB) CreateUser(ctx context.Context, in domain.UserCreate) (domain.User, error) {
	item, err := u.db.CreateUser(ctx, repo.UserCreate{
		Username:     in.Username,
		Email:        in.Email,
		Password:     in.Password,
		OIDCSub:      in.OIDCSub,
		Roles:        in.Roles,
		PasswordHash: in.PasswordHash,
		IsRoot:       in.IsRoot,
		AccessRules:  toStoreAccessRules(in.AccessRules),
	})
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(item), nil
}

func (u *DB) UpdateUser(ctx context.Context, userID string, in domain.UserUpdate) (domain.User, error) {
	item, err := u.db.UpdateUser(ctx, userID, repo.UserUpdate{
		Email:       in.Email,
		Password:    in.Password,
		Roles:       in.Roles,
		SetRoles:    in.SetRoles,
		AccessRules: toStoreAccessRules(in.AccessRules),
		SetRules:    in.SetRules,
		ClearRules:  in.SetRules && in.AccessRules == nil,
	})
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(item), nil
}

func (u *DB) DeleteUser(ctx context.Context, userID string) error {
	return mapError(u.db.DeleteUser(ctx, userID))
}

func (u *DB) SetRoles(ctx context.Context, userID string, roles []string) error {
	return mapError(u.db.SetUserRoles(ctx, userID, roles))
}

func (u *DB) CountUsers(ctx context.Context) (int, error) {
	return u.db.CountUsers(ctx)
}

func (u *DB) HasRootUser(ctx context.Context) (bool, error) {
	return u.db.HasRootUser(ctx)
}

func (u *DB) Insert(ctx context.Context, in domain.AuditCreate) error {
	return u.db.InsertAudit(ctx, repo.AuditCreate{
		Actor:        in.Actor,
		Action:       in.Action,
		ClusterID:    in.ClusterID,
		ResourceType: in.ResourceType,
		ResourceName: in.ResourceName,
		RequestID:    in.RequestID,
		Details: repo.AuditRequestDetails{
			Method: in.Details.Method,
			Path:   in.Details.Path,
			Status: in.Details.Status,
		},
		IP: in.IP,
	})
}

func (u *DB) ListAudit(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	items, total, err := u.db.ListAudit(ctx, repo.AuditFilter{
		ClusterID:  filter.ClusterID,
		ClusterIDs: filter.ClusterIDs,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.AuditEntry, len(items))
	for i, item := range items {
		out[i] = toDomainAudit(item)
	}
	return out, total, nil
}

func toDomainCluster(c repo.Cluster) domain.Cluster {
	return domain.Cluster{
		ID:            c.ID,
		Name:          c.Name,
		NATSURL:       c.NATSURL,
		MonitoringURL: c.MonitoringURL,
		HasCreds:      c.HasCreds,
		HasToken:      c.HasToken,
		IsDefault:     c.IsDefault,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

func toDomainUser(u repo.User) domain.User {
	grants := make([]domain.AccessGrant, 0, len(u.Grants))
	for _, g := range u.Grants {
		grants = append(grants, domain.AccessGrant(g))
	}
	return domain.User{
		ID:             u.ID,
		Username:       u.Username,
		Email:          u.Email,
		OIDCSub:        u.OIDCSub,
		Roles:          u.Roles,
		IsRoot:         u.IsRoot,
		AccessRules:    toDomainAccessRules(u.AccessRules),
		Grants:         grants,
		CreatedAt:      u.CreatedAt,
		SessionVersion: u.SessionVersion,
	}
}

func toDomainAccessRules(rules *repo.AccessRules) *domain.AccessRules {
	if rules == nil {
		return nil
	}
	return &domain.AccessRules{
		ClusterIDs:      append([]string(nil), rules.ClusterIDs...),
		ManageUsers:     rules.ManageUsers,
		ViewAudit:       rules.ViewAudit,
		DeleteClusters:  rules.DeleteClusters,
		AssignableRoles: append([]string(nil), rules.AssignableRoles...),
	}
}

func toStoreAccessRules(rules *domain.AccessRules) *repo.AccessRules {
	if rules == nil {
		return nil
	}
	return &repo.AccessRules{
		ClusterIDs:      append([]string(nil), rules.ClusterIDs...),
		ManageUsers:     rules.ManageUsers,
		ViewAudit:       rules.ViewAudit,
		DeleteClusters:  rules.DeleteClusters,
		AssignableRoles: append([]string(nil), rules.AssignableRoles...),
	}
}

func toDomainAudit(e repo.AuditEntry) domain.AuditEntry {
	return domain.AuditEntry{
		ID:           e.ID,
		Timestamp:    e.Timestamp,
		Actor:        e.Actor,
		Action:       e.Action,
		ClusterID:    e.ClusterID,
		ResourceType: e.ResourceType,
		ResourceName: e.ResourceName,
		RequestID:    e.RequestID,
		Details:      e.Details,
		IP:           e.IP,
	}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repo.ErrNotFound) {
		return domain.ErrNotFound
	}
	if errors.Is(err, repo.ErrRootProtected) {
		return domain.ErrRootProtected
	}
	if errors.Is(err, repo.ErrConflict) {
		return domain.ErrConflict
	}
	if errors.Is(err, repo.ErrAlertNotFound) {
		return domain.ErrAlertNotFound
	}
	if errors.Is(err, repo.ErrAlertRuleNotFound) {
		return domain.ErrAlertRuleNotFound
	}
	if errors.Is(err, repo.ErrEventCatalogEntryNotFound) {
		return domain.ErrEventCatalogEntryNotFound
	}
	if errors.Is(err, repo.ErrSigningGroupProtected) {
		return domain.ErrSigningGroupProtected
	}
	if errors.Is(err, repo.ErrSigningGroupInUse) {
		return domain.ErrSigningGroupInUse
	}
	return err
}
