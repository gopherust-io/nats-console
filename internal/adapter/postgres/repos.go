package postgres

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func (u *DB) RotateEncryptionKeys(ctx context.Context, currentKey, newKey string, dryRun bool) (domain.EncryptionRotationStats, error) {
	stats, err := u.db.RotateEncryptionKeys(ctx, currentKey, newKey, dryRun)
	if err != nil {
		return domain.EncryptionRotationStats{}, err
	}
	return domain.EncryptionRotationStats{ClustersUpdated: stats.ClustersUpdated}, nil
}

func (u *DB) QueryMetricSeries(
	ctx context.Context,
	clusterID string,
	metrics []string,
	from, to time.Time,
	step time.Duration,
) (map[string][]domain.MetricPoint, error) {
	return u.db.QueryMetricSeries(ctx, clusterID, metrics, from, to, step)
}

func (u *DB) ListBottleneckHourBuckets(
	ctx context.Context,
	clusterID string,
	from, to time.Time,
) ([]domain.BottleneckHourBucket, error) {
	return u.db.ListBottleneckHourBuckets(ctx, clusterID, from, to)
}

func (u *DB) ListArchitectureScoreDaily(
	ctx context.Context,
	clusterID string,
	from, to time.Time,
) ([]domain.ArchitectureScoreDailyRow, error) {
	return u.db.ListArchitectureScoreDaily(ctx, clusterID, from, to)
}

func (u *DB) GetArchitectureScoreDaily(
	ctx context.Context,
	clusterID string,
	day time.Time,
) (domain.ArchitectureScoreDailyRow, bool, error) {
	return u.db.GetArchitectureScoreDaily(ctx, clusterID, day)
}

func (u *DB) UpsertArchitectureScoreDaily(ctx context.Context, row domain.ArchitectureScoreDailyRow) error {
	return u.db.UpsertArchitectureScoreDaily(ctx, row)
}

func (u *DB) InsertIncidentAnnotation(ctx context.Context, clusterID string, in domain.IncidentAnnotationCreate) (domain.IncidentAnnotation, error) {
	return u.db.InsertIncidentAnnotation(ctx, clusterID, in)
}

func (u *DB) ListIncidentAnnotations(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentAnnotation, error) {
	return u.db.ListIncidentAnnotations(ctx, clusterID, from, to)
}

func (u *DB) ListIncidentConsumerSamples(
	ctx context.Context,
	clusterID, stream, consumer string,
	from, to time.Time,
) ([]domain.IncidentConsumerSample, error) {
	return u.db.ListIncidentConsumerSamples(ctx, clusterID, stream, consumer, from, to)
}

func (u *DB) ListIncidentNodeEvents(ctx context.Context, clusterID string, from, to time.Time) ([]domain.IncidentNodeEvent, error) {
	return u.db.ListIncidentNodeEvents(ctx, clusterID, from, to)
}

func (u *DB) ListAuditInRange(ctx context.Context, clusterID string, from, to time.Time, limit int) ([]domain.IncidentAuditChange, error) {
	return u.db.ListAuditInRange(ctx, clusterID, from, to, limit)
}

func (u *DB) ListEventCatalogEntries(ctx context.Context, clusterID string) ([]domain.EventCatalogDoc, error) {
	return u.db.ListEventCatalogEntries(ctx, clusterID)
}

func (u *DB) UpsertEventCatalogEntry(
	ctx context.Context,
	clusterID, subject, updatedBy string,
	in domain.EventCatalogUpsert,
) (domain.EventCatalogDoc, error) {
	row, err := u.db.UpsertEventCatalogEntry(ctx, clusterID, subject, updatedBy, in)
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

func (u *DB) DeleteEventCatalogEntry(ctx context.Context, clusterID, subject string) error {
	return mapError(u.db.DeleteEventCatalogEntry(ctx, clusterID, subject))
}

func (u *DB) ListAlerts(ctx context.Context, filter domain.AlertFilter) ([]domain.Alert, int, error) {
	return u.db.ListAlerts(ctx, filter)
}

func (u *DB) ListOpenUnacknowledged(ctx context.Context, clusterIDs []string, limit int) ([]domain.Alert, int, error) {
	return u.db.ListOpenUnacknowledged(ctx, clusterIDs, limit)
}

func (u *DB) GetAlert(ctx context.Context, id string) (domain.Alert, error) {
	alert, err := u.db.GetAlert(ctx, id)
	return alert, mapError(err)
}

func (u *DB) AcknowledgeAlert(ctx context.Context, id, actor string) (domain.Alert, error) {
	alert, err := u.db.AcknowledgeAlert(ctx, id, actor)
	return alert, mapError(err)
}

func (u *DB) ListAlertRules(ctx context.Context, clusterID string, enabledOnly bool) ([]domain.AlertRule, error) {
	return u.db.ListAlertRules(ctx, clusterID, enabledOnly)
}

func (u *DB) GetAlertRule(ctx context.Context, id string) (domain.AlertRule, error) {
	rule, err := u.db.GetAlertRule(ctx, id)
	return rule, mapError(err)
}

func (u *DB) CreateAlertRule(ctx context.Context, in domain.AlertRuleCreate, createdBy string) (domain.AlertRule, error) {
	return u.db.CreateAlertRule(ctx, in, createdBy)
}

func (u *DB) UpdateAlertRule(ctx context.Context, id string, in domain.AlertRuleUpdate) (domain.AlertRule, error) {
	rule, err := u.db.UpdateAlertRule(ctx, id, in)
	return rule, mapError(err)
}

func (u *DB) DeleteAlertRule(ctx context.Context, id string) error {
	return mapError(u.db.DeleteAlertRule(ctx, id))
}

func (u *DB) CreateUserInvite(ctx context.Context, userID string, ttl time.Duration) (domain.UserInvite, error) {
	return u.db.CreateUserInvite(ctx, userID, ttl)
}

func (u *DB) GetUserInvite(ctx context.Context, token string) (domain.UserInvite, error) {
	inv, err := u.db.GetUserInvite(ctx, token)
	return inv, mapError(err)
}

func (u *DB) AcceptUserInvite(ctx context.Context, token, password string) (domain.User, error) {
	user, err := u.db.AcceptUserInvite(ctx, token, password)
	if err != nil {
		return domain.User{}, mapError(err)
	}
	return toDomainUser(user), nil
}

func (u *DB) ListAccessGrantsByResource(ctx context.Context, resourceType, resourceKey string) ([]domain.AccessGrant, error) {
	return u.db.ListAccessGrantsByResource(ctx, resourceType, resourceKey)
}

func (u *DB) UpsertAccessGrant(ctx context.Context, in domain.AccessGrantUpsert) (domain.AccessGrant, error) {
	return u.db.UpsertAccessGrant(ctx, in)
}

func (u *DB) DeleteAccessGrantScoped(ctx context.Context, id, resourceType, resourceKey string) (string, error) {
	userID, err := u.db.DeleteAccessGrantScoped(ctx, id, resourceType, resourceKey)
	return userID, mapError(err)
}

func (u *DB) DeleteAccessGrantByResource(ctx context.Context, userID, resourceType, resourceKey string) error {
	return mapError(u.db.DeleteAccessGrantByResource(ctx, userID, resourceType, resourceKey))
}

func (u *DB) ListNATSAccountUsers(ctx context.Context, clusterID, accountName string) ([]domain.NATSAccountUser, error) {
	return u.db.ListNATSAccountUsers(ctx, clusterID, accountName)
}

func (u *DB) GetNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUser, error) {
	user, err := u.db.GetNATSAccountUser(ctx, clusterID, accountName, userID)
	return user, mapError(err)
}

func (u *DB) CreateNATSAccountUserWithSeed(ctx context.Context, in domain.NATSAccountUserCreate, accountSeed string) (domain.NATSAccountUserCreds, error) {
	return u.db.CreateNATSAccountUserWithSeed(ctx, in, accountSeed)
}

func (u *DB) UpdateNATSAccountUser(
	ctx context.Context,
	clusterID, accountName, userID string,
	in domain.NATSAccountUserUpdate,
	accountSeed string,
) (domain.NATSAccountUser, error) {
	user, err := u.db.UpdateNATSAccountUser(ctx, clusterID, accountName, userID, in, accountSeed)
	return user, mapError(err)
}

func (u *DB) DeleteNATSAccountUser(ctx context.Context, clusterID, accountName, userID string) ([]string, error) {
	ids, err := u.db.DeleteNATSAccountUser(ctx, clusterID, accountName, userID)
	return ids, mapError(err)
}

func (u *DB) GetNATSAccountUserCreds(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUserCreds, error) {
	creds, err := u.db.GetNATSAccountUserCreds(ctx, clusterID, accountName, userID)
	return creds, mapError(err)
}

func (u *DB) RotateNATSAccountUser(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error) {
	creds, err := u.db.RotateNATSAccountUser(ctx, clusterID, accountName, userID, accountSeed)
	return creds, mapError(err)
}

func (u *DB) MintNATSAccountUserJWT(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error) {
	creds, err := u.db.MintNATSAccountUserJWT(ctx, clusterID, accountName, userID, accountSeed)
	return creds, mapError(err)
}

func (u *DB) AssignNATSAccountUserPerson(ctx context.Context, clusterID, accountName, natsUserID, personUserID string) (domain.NATSAccountUser, error) {
	user, err := u.db.AssignNATSAccountUserPerson(ctx, clusterID, accountName, natsUserID, personUserID)
	return user, mapError(err)
}

func (u *DB) ListSubjectPermissions(ctx context.Context, clusterID, accountName, subject string) (domain.SubjectPermissionsResult, error) {
	return u.db.ListSubjectPermissions(ctx, clusterID, accountName, subject)
}

func (u *DB) ListSigningGroups(ctx context.Context, clusterID, accountName string) ([]domain.SigningGroup, error) {
	return u.db.ListSigningGroups(ctx, clusterID, accountName)
}

func (u *DB) CreateSigningGroup(ctx context.Context, in domain.SigningGroupCreate) (domain.SigningGroup, error) {
	return u.db.CreateSigningGroup(ctx, in)
}

func (u *DB) UpdateSigningGroup(
	ctx context.Context,
	clusterID, accountName, groupID string,
	in domain.SigningGroupUpdate,
) (domain.SigningGroup, error) {
	group, err := u.db.UpdateSigningGroup(ctx, clusterID, accountName, groupID, in)
	return group, mapError(err)
}

func (u *DB) DeleteSigningGroup(ctx context.Context, clusterID, accountName, groupID string) error {
	return mapError(u.db.DeleteSigningGroup(ctx, clusterID, accountName, groupID))
}

func (u *DB) ListNATSAccountExports(ctx context.Context, clusterID, accountName, kind string) ([]domain.NATSAccountExport, error) {
	return u.db.ListNATSAccountExports(ctx, clusterID, accountName, kind)
}

func (u *DB) CreateNATSAccountExport(ctx context.Context, in domain.NATSAccountExportCreate) (domain.NATSAccountExport, error) {
	return u.db.CreateNATSAccountExport(ctx, in)
}

func (u *DB) UpdateNATSAccountExport(
	ctx context.Context,
	clusterID, accountName, exportID string,
	in domain.NATSAccountExportUpdate,
) (domain.NATSAccountExport, error) {
	item, err := u.db.UpdateNATSAccountExport(ctx, clusterID, accountName, exportID, in)
	return item, mapError(err)
}

func (u *DB) DeleteNATSAccountExport(ctx context.Context, clusterID, accountName, exportID string) error {
	return mapError(u.db.DeleteNATSAccountExport(ctx, clusterID, accountName, exportID))
}
