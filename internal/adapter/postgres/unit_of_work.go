package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/store"
)

type UnitOfWork struct {
	store *store.Store
}

var _ port.UnitOfWork = (*UnitOfWork)(nil)

func Open(ctx context.Context, databaseURL, migrationsDir string, encryptor *crypto.Encryptor, pool store.PoolConfig) (*UnitOfWork, error) {
	st, err := store.Open(ctx, databaseURL, migrationsDir, encryptor, pool)
	if err != nil {
		return nil, err
	}
	return WrapStore(st), nil
}

func OpenWithConfig(ctx context.Context, cfg config.Config, encryptor *crypto.Encryptor) (*UnitOfWork, error) {
	return Open(ctx, cfg.DatabaseURL, "migrations", encryptor, poolConfigFrom(cfg))
}

func poolConfigFrom(cfg config.Config) store.PoolConfig {
	return store.PoolConfig{
		MaxConns:          boundedInt32(cfg.DBMaxConns),
		MinConns:          boundedInt32(cfg.DBMinConns),
		MaxConnLifetime:   cfg.DBMaxConnLifetime,
		MaxConnIdleTime:   cfg.DBMaxConnIdleTime,
		HealthCheckPeriod: cfg.DBHealthCheckPeriod,
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

func WrapStore(st *store.Store) *UnitOfWork {
	return &UnitOfWork{store: st}
}

func (u *UnitOfWork) Close() {
	if u != nil && u.store != nil {
		u.store.Close()
	}
}

func (u *UnitOfWork) Raw() *store.Store {
	return u.store
}

func (u *UnitOfWork) CountClusters(ctx context.Context) (int, error) {
	return u.store.CountClusters(ctx)
}

func (u *UnitOfWork) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	items, err := u.store.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Cluster, len(items))
	for i, item := range items {
		out[i] = toDomainCluster(item)
	}
	return out, nil
}

func (u *UnitOfWork) GetCluster(ctx context.Context, id string) (domain.Cluster, error) {
	item, err := u.store.GetCluster(ctx, id)
	if err != nil {
		return domain.Cluster{}, mapError(err)
	}
	return toDomainCluster(item), nil
}

func (u *UnitOfWork) GetDefaultCluster(ctx context.Context) (domain.Cluster, error) {
	item, err := u.store.GetDefaultCluster(ctx)
	if err != nil {
		return domain.Cluster{}, mapError(err)
	}
	return toDomainCluster(item), nil
}

func (u *UnitOfWork) CreateCluster(ctx context.Context, in domain.ClusterCreate) (domain.Cluster, error) {
	item, err := u.store.CreateCluster(ctx, store.ClusterCreate{
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

func (u *UnitOfWork) UpdateCluster(ctx context.Context, id string, in domain.ClusterUpdate) (domain.Cluster, error) {
	item, err := u.store.UpdateCluster(ctx, id, store.ClusterUpdate{
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

func (u *UnitOfWork) DeleteCluster(ctx context.Context, id string) error {
	return mapError(u.store.DeleteCluster(ctx, id))
}

func (u *UnitOfWork) Ping(ctx context.Context) error {
	return u.store.Ping(ctx)
}

func (u *UnitOfWork) List(ctx context.Context) ([]domain.User, error) {
	items, err := u.store.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, len(items))
	for i, item := range items {
		out[i] = toDomainUser(item)
	}
	return out, nil
}

func (u *UnitOfWork) GetByID(ctx context.Context, id string) (domain.User, error) {
	item, err := u.store.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(item), nil
}

func (u *UnitOfWork) GetByUsername(ctx context.Context, username string) (domain.User, string, error) {
	item, hash, err := u.store.GetUserByUsername(ctx, username)
	if err != nil {
		return domain.User{}, "", mapError(err)
	}
	return toDomainUser(item), hash, nil
}

func (u *UnitOfWork) GetByOIDCSub(ctx context.Context, sub string) (domain.User, error) {
	item, err := u.store.GetUserByOIDCSub(ctx, sub)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(item), nil
}

func (u *UnitOfWork) CreateUser(ctx context.Context, in domain.UserCreate) (domain.User, error) {
	item, err := u.store.CreateUser(ctx, store.UserCreate{
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

func (u *UnitOfWork) UpdateUser(ctx context.Context, userID string, in domain.UserUpdate) (domain.User, error) {
	item, err := u.store.UpdateUser(ctx, userID, store.UserUpdate{
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

func (u *UnitOfWork) DeleteUser(ctx context.Context, userID string) error {
	return mapError(u.store.DeleteUser(ctx, userID))
}

func (u *UnitOfWork) SetRoles(ctx context.Context, userID string, roles []string) error {
	return mapError(u.store.SetUserRoles(ctx, userID, roles))
}

func (u *UnitOfWork) CountUsers(ctx context.Context) (int, error) {
	return u.store.CountUsers(ctx)
}

func (u *UnitOfWork) HasRootUser(ctx context.Context) (bool, error) {
	return u.store.HasRootUser(ctx)
}

func (u *UnitOfWork) Insert(ctx context.Context, in domain.AuditCreate) error {
	return u.store.InsertAudit(ctx, store.AuditCreate{
		Actor:        in.Actor,
		Action:       in.Action,
		ClusterID:    in.ClusterID,
		ResourceType: in.ResourceType,
		ResourceName: in.ResourceName,
		RequestID:    in.RequestID,
		Details: store.AuditRequestDetails{
			Method: in.Details.Method,
			Path:   in.Details.Path,
			Status: in.Details.Status,
		},
		IP: in.IP,
	})
}

func (u *UnitOfWork) ListAudit(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, int, error) {
	items, total, err := u.store.ListAudit(ctx, store.AuditFilter{
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

func toDomainCluster(c store.Cluster) domain.Cluster {
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

func toDomainUser(u store.User) domain.User {
	return domain.User{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		OIDCSub:     u.OIDCSub,
		Roles:       u.Roles,
		IsRoot:      u.IsRoot,
		AccessRules: toDomainAccessRules(u.AccessRules),
		CreatedAt:   u.CreatedAt,
	}
}

func toDomainAccessRules(rules *store.AccessRules) *domain.AccessRules {
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

func toStoreAccessRules(rules *domain.AccessRules) *store.AccessRules {
	if rules == nil {
		return nil
	}
	return &store.AccessRules{
		ClusterIDs:      append([]string(nil), rules.ClusterIDs...),
		ManageUsers:     rules.ManageUsers,
		ViewAudit:       rules.ViewAudit,
		DeleteClusters:  rules.DeleteClusters,
		AssignableRoles: append([]string(nil), rules.AssignableRoles...),
	}
}

func toDomainAudit(e store.AuditEntry) domain.AuditEntry {
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
	if errors.Is(err, store.ErrNotFound) {
		return domain.ErrNotFound
	}
	if errors.Is(err, store.ErrRootProtected) {
		return domain.ErrRootProtected
	}
	if errors.Is(err, store.ErrConflict) {
		return domain.ErrRootExists
	}
	return err
}
