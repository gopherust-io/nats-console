package postgres

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func (u *UnitOfWork) RotateEncryptionKeys(ctx context.Context, currentKey, newKey string, dryRun bool) (domain.EncryptionRotationStats, error) {
	stats, err := u.store.RotateEncryptionKeys(ctx, currentKey, newKey, dryRun)
	if err != nil {
		return domain.EncryptionRotationStats{}, err
	}
	return domain.EncryptionRotationStats{ClustersUpdated: stats.ClustersUpdated}, nil
}

func (u *UnitOfWork) QueryMetricSeries(
	ctx context.Context,
	clusterID string,
	metrics []string,
	from, to time.Time,
	step time.Duration,
) (map[string][]domain.MetricPoint, error) {
	return u.store.QueryMetricSeries(ctx, clusterID, metrics, from, to, step)
}

func (u *UnitOfWork) ListBottleneckHourBuckets(
	ctx context.Context,
	clusterID string,
	from, to time.Time,
) ([]domain.BottleneckHourBucket, error) {
	return u.store.ListBottleneckHourBuckets(ctx, clusterID, from, to)
}

func (u *UnitOfWork) ListArchitectureScoreDaily(
	ctx context.Context,
	clusterID string,
	from, to time.Time,
) ([]domain.ArchitectureScoreDailyRow, error) {
	return u.store.ListArchitectureScoreDaily(ctx, clusterID, from, to)
}

func (u *UnitOfWork) GetArchitectureScoreDaily(
	ctx context.Context,
	clusterID string,
	day time.Time,
) (domain.ArchitectureScoreDailyRow, bool, error) {
	return u.store.GetArchitectureScoreDaily(ctx, clusterID, day)
}

func (u *UnitOfWork) UpsertArchitectureScoreDaily(ctx context.Context, row domain.ArchitectureScoreDailyRow) error {
	return u.store.UpsertArchitectureScoreDaily(ctx, row)
}

func (u *UnitOfWork) InsertIncidentAnnotation(ctx context.Context, clusterID string, in domain.IncidentAnnotationCreate) (domain.IncidentAnnotation, error) {
	return u.store.InsertIncidentAnnotation(ctx, clusterID, in)
}

func (u *UnitOfWork) ListIncidentAnnotations(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentAnnotation, error) {
	return u.store.ListIncidentAnnotations(ctx, clusterID, from, to)
}

func (u *UnitOfWork) ListIncidentConsumerSamples(
	ctx context.Context,
	clusterID, stream, consumer string,
	from, to time.Time,
) ([]domain.IncidentConsumerSample, error) {
	return u.store.ListIncidentConsumerSamples(ctx, clusterID, stream, consumer, from, to)
}

func (u *UnitOfWork) ListIncidentNodeEvents(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentNodeEvent, error) {
	return u.store.ListIncidentNodeEvents(ctx, clusterID, from, to)
}

func (u *UnitOfWork) ListAuditInRange(ctx context.Context, clusterID string, from, to time.Time, limit int) ([]domain.IncidentAuditChange, error) {
	return u.store.ListAuditInRange(ctx, clusterID, from, to, limit)
}

func (u *UnitOfWork) ListEventCatalogEntries(ctx context.Context, clusterID string) ([]domain.EventCatalogDoc, error) {
	return u.store.ListEventCatalogEntries(ctx, clusterID)
}

func (u *UnitOfWork) UpsertEventCatalogEntry(
	ctx context.Context,
	clusterID, subject, updatedBy string,
	in domain.EventCatalogUpsert,
) (domain.EventCatalogDoc, error) {
	row, err := u.store.UpsertEventCatalogEntry(ctx, clusterID, subject, updatedBy, in)
	if err != nil {
		return domain.EventCatalogDoc{}, mapError(err)
	}
	return domain.EventCatalogDoc{
		Subject:          row.Subject,
		Owner:            row.Owner,
		Description:      row.Description,
		Schema:           row.Schema,
		Example:          row.Example,
		Deprecated:       row.Deprecated,
		SuccessorSubject: row.SuccessorSubject,
		DeprecationNote:  row.DeprecationNote,
		UpdatedBy:        row.UpdatedBy,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

func (u *UnitOfWork) DeleteEventCatalogEntry(ctx context.Context, clusterID, subject string) error {
	return mapError(u.store.DeleteEventCatalogEntry(ctx, clusterID, subject))
}

func (u *UnitOfWork) ListAlerts(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, int, error) {
	return u.store.ListAlerts(ctx, filter)
}

func (u *UnitOfWork) ListOpenUnacknowledged(ctx context.Context, clusterIDs []string, limit int) ([]domain.Alert, int, error) {
	return u.store.ListOpenUnacknowledged(ctx, clusterIDs, limit)
}

func (u *UnitOfWork) GetAlert(ctx context.Context, id string) (domain.Alert, error) {
	alert, err := u.store.GetAlert(ctx, id)
	return alert, mapError(err)
}

func (u *UnitOfWork) AcknowledgeAlert(ctx context.Context, id, actor string) (domain.Alert, error) {
	alert, err := u.store.AcknowledgeAlert(ctx, id, actor)
	return alert, mapError(err)
}

func (u *UnitOfWork) ListAlertRules(ctx context.Context, clusterID string, enabledOnly bool) ([]domain.AlertRule, error) {
	return u.store.ListAlertRules(ctx, clusterID, enabledOnly)
}

func (u *UnitOfWork) GetAlertRule(ctx context.Context, id string) (domain.AlertRule, error) {
	rule, err := u.store.GetAlertRule(ctx, id)
	return rule, mapError(err)
}

func (u *UnitOfWork) CreateAlertRule(ctx context.Context, in domain.AlertRuleCreate, createdBy string) (domain.AlertRule, error) {
	return u.store.CreateAlertRule(ctx, in, createdBy)
}

func (u *UnitOfWork) UpdateAlertRule(ctx context.Context, id string, in domain.AlertRuleUpdate) (domain.AlertRule, error) {
	rule, err := u.store.UpdateAlertRule(ctx, id, in)
	return rule, mapError(err)
}

func (u *UnitOfWork) DeleteAlertRule(ctx context.Context, id string) error {
	return mapError(u.store.DeleteAlertRule(ctx, id))
}

func (u *UnitOfWork) CreateUserInvite(ctx context.Context, userID string, ttl time.Duration) (domain.UserInvite, error) {
	return u.store.CreateUserInvite(ctx, userID, ttl)
}

func (u *UnitOfWork) GetUserInvite(ctx context.Context, token string) (domain.UserInvite, error) {
	inv, err := u.store.GetUserInvite(ctx, token)
	return inv, mapError(err)
}

func (u *UnitOfWork) AcceptUserInvite(ctx context.Context, token, password string) (domain.User, error) {
	user, err := u.store.AcceptUserInvite(ctx, token, password)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(user), nil
}

func (u *UnitOfWork) ListAccessGrantsByResource(ctx context.Context, resourceType, resourceKey string) ([]domain.AccessGrant, error) {
	return u.store.ListAccessGrantsByResource(ctx, resourceType, resourceKey)
}

func (u *UnitOfWork) UpsertAccessGrant(ctx context.Context, in domain.AccessGrantUpsert) (domain.AccessGrant, error) {
	return u.store.UpsertAccessGrant(ctx, in)
}

func (u *UnitOfWork) DeleteAccessGrantScoped(ctx context.Context, id, resourceType, resourceKey string) (string, error) {
	userID, err := u.store.DeleteAccessGrantScoped(ctx, id, resourceType, resourceKey)
	return userID, mapError(err)
}

func (u *UnitOfWork) DeleteAccessGrantByResource(ctx context.Context, userID, resourceType, resourceKey string) error {
	return mapError(u.store.DeleteAccessGrantByResource(ctx, userID, resourceType, resourceKey))
}

func (u *UnitOfWork) ListNATSAccountUsers(ctx context.Context, clusterID, accountName string) ([]domain.NATSAccountUser, error) {
	return u.store.ListNATSAccountUsers(ctx, clusterID, accountName)
}

func (u *UnitOfWork) GetNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUser, error) {
	user, err := u.store.GetNATSAccountUser(ctx, clusterID, accountName, userID)
	return user, mapError(err)
}

func (u *UnitOfWork) CreateNATSAccountUserWithSeed(ctx context.Context, in domain.NATSAccountUserCreate, accountSeed string) (domain.NATSAccountUserCreds, error) {
	return u.store.CreateNATSAccountUserWithSeed(ctx, in, accountSeed)
}

func (u *UnitOfWork) UpdateNATSAccountUser(
	ctx context.Context,
	clusterID, accountName, userID string,
	in domain.NATSAccountUserUpdate,
	accountSeed string,
) (domain.NATSAccountUser, error) {
	user, err := u.store.UpdateNATSAccountUser(ctx, clusterID, accountName, userID, in, accountSeed)
	return user, mapError(err)
}

func (u *UnitOfWork) DeleteNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) ([]string, error) {
	ids, err := u.store.DeleteNATSAccountUser(ctx, clusterID, accountName, userID)
	return ids, mapError(err)
}

func (u *UnitOfWork) GetNATSAccountUserCreds(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUserCreds, error) {
	creds, err := u.store.GetNATSAccountUserCreds(ctx, clusterID, accountName, userID)
	return creds, mapError(err)
}

func (u *UnitOfWork) RotateNATSAccountUser(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error) {
	creds, err := u.store.RotateNATSAccountUser(ctx, clusterID, accountName, userID, accountSeed)
	return creds, mapError(err)
}

func (u *UnitOfWork) MintNATSAccountUserJWT(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error) {
	creds, err := u.store.MintNATSAccountUserJWT(ctx, clusterID, accountName, userID, accountSeed)
	return creds, mapError(err)
}

func (u *UnitOfWork) AssignNATSAccountUserPerson(ctx context.Context, clusterID, accountName, natsUserID, personUserID string) (domain.NATSAccountUser, error) {
	user, err := u.store.AssignNATSAccountUserPerson(ctx, clusterID, accountName, natsUserID, personUserID)
	return user, mapError(err)
}

func (u *UnitOfWork) ListSubjectPermissions(ctx context.Context, clusterID, accountName, subject string) (domain.SubjectPermissionsResult, error) {
	return u.store.ListSubjectPermissions(ctx, clusterID, accountName, subject)
}

func (u *UnitOfWork) ListSigningGroups(ctx context.Context, clusterID, accountName string) ([]domain.SigningGroup, error) {
	return u.store.ListSigningGroups(ctx, clusterID, accountName)
}

func (u *UnitOfWork) CreateSigningGroup(ctx context.Context, in domain.SigningGroupCreate) (domain.SigningGroup, error) {
	return u.store.CreateSigningGroup(ctx, in)
}

func (u *UnitOfWork) UpdateSigningGroup(
	ctx context.Context,
	clusterID, accountName, groupID string,
	in domain.SigningGroupUpdate,
) (domain.SigningGroup, error) {
	group, err := u.store.UpdateSigningGroup(ctx, clusterID, accountName, groupID, in)
	return group, mapError(err)
}

func (u *UnitOfWork) DeleteSigningGroup(ctx context.Context, clusterID, accountName, groupID string) error {
	return mapError(u.store.DeleteSigningGroup(ctx, clusterID, accountName, groupID))
}

func (u *UnitOfWork) ListNATSAccountExports(ctx context.Context, clusterID, accountName, kind string) ([]domain.NATSAccountExport, error) {
	return u.store.ListNATSAccountExports(ctx, clusterID, accountName, kind)
}

func (u *UnitOfWork) CreateNATSAccountExport(ctx context.Context, in domain.NATSAccountExportCreate) (domain.NATSAccountExport, error) {
	return u.store.CreateNATSAccountExport(ctx, in)
}

func (u *UnitOfWork) UpdateNATSAccountExport(
	ctx context.Context,
	clusterID, accountName, exportID string,
	in domain.NATSAccountExportUpdate,
) (domain.NATSAccountExport, error) {
	item, err := u.store.UpdateNATSAccountExport(ctx, clusterID, accountName, exportID, in)
	return item, mapError(err)
}

func (u *UnitOfWork) DeleteNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string) error {
	return mapError(u.store.DeleteNATSAccountExport(ctx, clusterID, accountName, exportID))
}
