package port

import (
	"context"
	"time"

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

type EncryptionRepository interface {
	RotateEncryptionKeys(ctx context.Context, currentKey, newKey string, dryRun bool) (domain.EncryptionRotationStats, error)
}

type MetricsRepository interface {
	QueryMetricSeries(ctx context.Context, clusterID string, metrics []string, from, to time.Time, step time.Duration) (map[string][]domain.MetricPoint, error)
	ListBottleneckHourBuckets(ctx context.Context, clusterID string, from, to time.Time) ([]domain.BottleneckHourBucket, error)
	ListArchitectureScoreDaily(ctx context.Context, clusterID string, from, to time.Time) ([]domain.ArchitectureScoreDailyRow, error)
	GetArchitectureScoreDaily(ctx context.Context, clusterID string, day time.Time) (domain.ArchitectureScoreDailyRow, bool, error)
	UpsertArchitectureScoreDaily(ctx context.Context, row domain.ArchitectureScoreDailyRow) error
}

type IncidentRepository interface {
	InsertIncidentAnnotation(ctx context.Context, clusterID string, in domain.IncidentAnnotationCreate) (domain.IncidentAnnotation, error)
	ListIncidentAnnotations(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentAnnotation, error)
	ListIncidentConsumerSamples(ctx context.Context, clusterID, stream, consumer string, from, to time.Time) ([]domain.IncidentConsumerSample, error)
	ListIncidentNodeEvents(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentNodeEvent, error)
	ListAuditInRange(ctx context.Context, clusterID string, from, to time.Time, limit int) ([]domain.IncidentAuditChange, error)
}

type EventCatalogRepository interface {
	ListEventCatalogEntries(ctx context.Context, clusterID string) ([]domain.EventCatalogDoc, error)
	UpsertEventCatalogEntry(ctx context.Context, clusterID, subject, updatedBy string, in domain.EventCatalogUpsert) (domain.EventCatalogDoc, error)
	DeleteEventCatalogEntry(ctx context.Context, clusterID, subject string) error
}

type AlertRepository interface {
	ListAlerts(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, int, error)
	ListOpenUnacknowledged(ctx context.Context, clusterIDs []string, limit int) ([]domain.Alert, int, error)
	GetAlert(ctx context.Context, id string) (domain.Alert, error)
	AcknowledgeAlert(ctx context.Context, id, actor string) (domain.Alert, error)
	ListAlertRules(ctx context.Context, clusterID string, enabledOnly bool) ([]domain.AlertRule, error)
	GetAlertRule(ctx context.Context, id string) (domain.AlertRule, error)
	CreateAlertRule(ctx context.Context, in domain.AlertRuleCreate, createdBy string) (domain.AlertRule, error)
	UpdateAlertRule(ctx context.Context, id string, in domain.AlertRuleUpdate) (domain.AlertRule, error)
	DeleteAlertRule(ctx context.Context, id string) error
}

type AccessRepository interface {
	CreateUserInvite(ctx context.Context, userID string, ttl time.Duration) (domain.UserInvite, error)
	GetUserInvite(ctx context.Context, token string) (domain.UserInvite, error)
	AcceptUserInvite(ctx context.Context, token, password string) (domain.User, error)
	ListAccessGrantsByResource(ctx context.Context, resourceType, resourceKey string) ([]domain.AccessGrant, error)
	UpsertAccessGrant(ctx context.Context, in domain.AccessGrantUpsert) (domain.AccessGrant, error)
	DeleteAccessGrantScoped(ctx context.Context, id, resourceType, resourceKey string) (userID string, err error)
	DeleteAccessGrantByResource(ctx context.Context, userID, resourceType, resourceKey string) error
}

type NATSAccountRepository interface {
	ListNATSAccountUsers(ctx context.Context, clusterID, accountName string) ([]domain.NATSAccountUser, error)
	GetNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUser, error)
	CreateNATSAccountUserWithSeed(ctx context.Context, in domain.NATSAccountUserCreate, accountSeed string) (domain.NATSAccountUserCreds, error)
	UpdateNATSAccountUser(ctx context.Context, clusterID, accountName, userID string, in domain.NATSAccountUserUpdate, accountSeed string) (domain.NATSAccountUser, error)
	DeleteNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) error
	GetNATSAccountUserCreds(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUserCreds, error)
	RotateNATSAccountUser(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error)
	MintNATSAccountUserJWT(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error)
	AssignNATSAccountUserPerson(ctx context.Context, clusterID, accountName, natsUserID, personUserID string) (domain.NATSAccountUser, error)
	ListSubjectPermissions(ctx context.Context, clusterID, accountName, subject string) (domain.SubjectPermissionsResult, error)
	ListSigningGroups(ctx context.Context, clusterID, accountName string) ([]domain.SigningGroup, error)
	CreateSigningGroup(ctx context.Context, in domain.SigningGroupCreate) (domain.SigningGroup, error)
	UpdateSigningGroup(ctx context.Context, clusterID, accountName, groupID string, in domain.SigningGroupUpdate) (domain.SigningGroup, error)
	DeleteSigningGroup(ctx context.Context, clusterID, accountName, groupID string) error
	ListNATSAccountExports(ctx context.Context, clusterID, accountName, kind string) ([]domain.NATSAccountExport, error)
	CreateNATSAccountExport(ctx context.Context, in domain.NATSAccountExportCreate) (domain.NATSAccountExport, error)
	UpdateNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string, in domain.NATSAccountExportUpdate) (domain.NATSAccountExport, error)
	DeleteNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string) error
}

type UnitOfWork interface {
	ClusterRepository
	UserRepository
	AuditRepository
	EncryptionRepository
	MetricsRepository
	IncidentRepository
	EventCatalogRepository
	AlertRepository
	AccessRepository
	NATSAccountRepository
	Close()
}
