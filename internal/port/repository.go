package port

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

type ClusterRepository interface {
	CountClusters(ctx context.Context) (int, error)
	ListClusters(ctx context.Context) ([]domain.Cluster, error)
	GetCluster(ctx context.Context, id string) (domain.Cluster, error)
	GetDefaultCluster(ctx context.Context) (domain.Cluster, error)
	CreateCluster(ctx context.Context, in domain.ClusterCreate) (domain.Cluster, error)
	UpdateCluster(ctx context.Context, id string, in domain.ClusterUpdate) (domain.Cluster, error)
	DeleteCluster(ctx context.Context, id string) error
	Ping(ctx context.Context) error
}

type UserRepository interface {
	List(ctx context.Context) ([]domain.User, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	GetByUsername(ctx context.Context, username string) (domain.User, string, error)
	GetByOIDCSub(ctx context.Context, sub string) (domain.User, error)
	CreateUser(ctx context.Context, in domain.UserCreate) (domain.User, error)
	UpdateUser(ctx context.Context, userID string, in domain.UserUpdate) (domain.User, error)
	DeleteUser(ctx context.Context, userID string) error
	SetRoles(ctx context.Context, userID string, roles []string) error
	CountUsers(ctx context.Context) (int, error)
	HasRootUser(ctx context.Context) (bool, error)
}

type AuditRepository interface {
	Insert(ctx context.Context, in domain.AuditCreate) error
	ListAudit(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, int, error)
}

type UnitOfWork interface {
	ClusterRepository
	UserRepository
	AuditRepository
	Close()
}
