package api

import (
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
)

// PaginationMeta mirrors httpstatus.Meta for OpenAPI Models.
//
// goalign:ignore
type PaginationMeta struct {
	Total  int `json:"total"`
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// Swagger response envelopes and model aliases. Referenced by // @Success
// comments so definitions appear in the OpenAPI Models section.

// goalign:ignore
type DataMetaEnvelope struct {
	Data any             `json:"data,omitempty"`
	Meta *PaginationMeta `json:"meta,omitempty"`
}

// goalign:ignore
type StreamListEnvelope struct {
	Data []domain.StreamInfo `json:"data"`
	Meta *PaginationMeta     `json:"meta,omitempty"`
}

// goalign:ignore
type ConsumerListEnvelope struct {
	Data []domain.ConsumerInfo `json:"data"`
	Meta *PaginationMeta       `json:"meta,omitempty"`
}

// goalign:ignore
type AccountInfoEnvelope struct {
	Data domain.AccountInfo `json:"data"`
}

// goalign:ignore
type StreamInfoEnvelope struct {
	Data domain.StreamInfo `json:"data"`
}

// goalign:ignore
type ConsumerInfoEnvelope struct {
	Data domain.ConsumerInfo `json:"data"`
}

// goalign:ignore
type ClusterEnvelope struct {
	Data domain.Cluster `json:"data"`
}

// goalign:ignore
type ClusterListEnvelope struct {
	Data []domain.Cluster `json:"data"`
}

// goalign:ignore
type ClusterTestEnvelope struct {
	Data domain.ClusterTestResult `json:"data"`
}

// goalign:ignore
type ConnectionStatusEnvelope struct {
	Data domain.NATSConnectionStatus `json:"data"`
}

// goalign:ignore
type ConnectionStatusListEnvelope struct {
	Data []domain.NATSConnectionStatus `json:"data"`
}

// goalign:ignore
type AlertListEnvelope struct {
	Data []domain.Alert `json:"data"`
}

// goalign:ignore
type AlertEnvelope struct {
	Data domain.Alert `json:"data"`
}

// goalign:ignore
type AlertRuleListEnvelope struct {
	Data []domain.AlertRule `json:"data"`
}

// goalign:ignore
type AlertRuleEnvelope struct {
	Data domain.AlertRule `json:"data"`
}

// goalign:ignore
type AlertOpenSummaryEnvelope struct {
	Data domain.AlertOpenSummary `json:"data"`
}

// goalign:ignore
type UserListEnvelope struct {
	Data []UserResponse `json:"data"`
}

// goalign:ignore
type AuditListEnvelope struct {
	Data []domain.AuditEntry `json:"data"`
}

// goalign:ignore
type AccessGrantListEnvelope struct {
	Data []domain.AccessGrant `json:"data"`
}

// goalign:ignore
type AccessGrantEnvelope struct {
	Data domain.AccessGrant `json:"data"`
}

// goalign:ignore
type NATSUserListEnvelope struct {
	Data []domain.NATSAccountUser `json:"data"`
}

// goalign:ignore
type NATSUserEnvelope struct {
	Data domain.NATSAccountUser `json:"data"`
}

// goalign:ignore
type NATSCredsEnvelope struct {
	Data domain.NATSAccountUserCreds `json:"data"`
}

// goalign:ignore
type SigningGroupListEnvelope struct {
	Data []domain.SigningGroup `json:"data"`
}

// goalign:ignore
type SigningGroupEnvelope struct {
	Data domain.SigningGroup `json:"data"`
}

// goalign:ignore
type ExportListEnvelope struct {
	Data []domain.NATSAccountExport `json:"data"`
}

// goalign:ignore
type ExportEnvelope struct {
	Data domain.NATSAccountExport `json:"data"`
}

// goalign:ignore
type SubjectPermissionsEnvelope struct {
	Data domain.SubjectPermissionsResult `json:"data"`
}

// goalign:ignore
type KVBucketListEnvelope struct {
	Data []domain.KVBucketInfo `json:"data"`
}

// goalign:ignore
type KVBucketEnvelope struct {
	Data domain.KVBucketInfo `json:"data"`
}

// goalign:ignore
type KVEntryEnvelope struct {
	Data domain.KVEntry `json:"data"`
}

// goalign:ignore
type KVEntryListEnvelope struct {
	Data []domain.KVEntry `json:"data"`
}

// goalign:ignore
type ObjectBucketListEnvelope struct {
	Data []domain.ObjectBucketInfo `json:"data"`
}

// goalign:ignore
type ObjectBucketEnvelope struct {
	Data domain.ObjectBucketInfo `json:"data"`
}

// goalign:ignore
type ObjectInfoEnvelope struct {
	Data domain.ObjectInfo `json:"data"`
}

// goalign:ignore
type ObjectInfoListEnvelope struct {
	Data []domain.ObjectInfo `json:"data"`
}

// goalign:ignore
type StreamMessageEnvelope struct {
	Data domain.StreamMessage `json:"data"`
}

// goalign:ignore
type PublishMessageEnvelope struct {
	Data domain.PublishMessageResult `json:"data"`
}

// goalign:ignore
type MessageRangeEnvelope struct {
	Data domain.MessageRangeResult `json:"data"`
}

// goalign:ignore
type DLQListEnvelope struct {
	Data domain.DLQListResult `json:"data"`
}

// goalign:ignore
type DLQRetryEnvelope struct {
	Data domain.DLQRetryResult `json:"data"`
}

// goalign:ignore
type ReplayEnvelope struct {
	Data domain.ReplayConsumerResult `json:"data"`
}

// goalign:ignore
type ReplayDryRunEnvelope struct {
	Data domain.ReplayDryRun `json:"data"`
}

// goalign:ignore
type BlastRadiusEnvelope struct {
	Data domain.BlastRadius `json:"data"`
}

// goalign:ignore
type MetricsHistoryEnvelope struct {
	Data domain.MetricsHistoryResponse `json:"data"`
}

// goalign:ignore
type ZombieEnvelope struct {
	Data domain.ZombieSnapshot `json:"data"`
}

// goalign:ignore
type SubjectNamingEnvelope struct {
	Data domain.SubjectNamingSnapshot `json:"data"`
}

// goalign:ignore
type EventGenomeEnvelope struct {
	Data domain.EventGenomeSnapshot `json:"data"`
}

// goalign:ignore
type EventArchitectureEnvelope struct {
	Data domain.EventArchitectureSnapshot `json:"data"`
}

// goalign:ignore
type ArchitectureRefactorEnvelope struct {
	Data domain.ArchitectureRefactorPlan `json:"data"`
}

// goalign:ignore
type ArchitectureScoreEnvelope struct {
	Data domain.ArchitectureScoreSnapshot `json:"data"`
}

// goalign:ignore
type HiddenBottlenecksEnvelope struct {
	Data domain.HiddenBottleneckSnapshot `json:"data"`
}

// goalign:ignore
type ChaosStoryEnvelope struct {
	Data domain.ChaosStory `json:"data"`
}

// goalign:ignore
type ChaosStorySeedEnvelope struct {
	Data domain.ChaosStorySeed `json:"data"`
}

// goalign:ignore
type ArchitectureInventoryEnvelope struct {
	Data domain.ArchitectureInventory `json:"data"`
}

// goalign:ignore
type EventCatalogEnvelope struct {
	Data domain.EventCatalogSnapshot `json:"data"`
}

// goalign:ignore
type EventCatalogDocEnvelope struct {
	Data domain.EventCatalogDoc `json:"data"`
}

// goalign:ignore
type EventWikipediaEnvelope struct {
	Data domain.EventWikipediaSnapshot `json:"data"`
}

// goalign:ignore
type RequestReplyEnvelope struct {
	Data domain.RequestReplySnapshot `json:"data"`
}

// goalign:ignore
type IncidentReconstructionEnvelope struct {
	Data domain.IncidentReconstruction `json:"data"`
}

// goalign:ignore
type IncidentAnnotationEnvelope struct {
	Data domain.IncidentAnnotation `json:"data"`
}

// goalign:ignore
type IncidentCapsuleEnvelope struct {
	Data domain.IncidentCapsuleDetail `json:"data"`
}

// goalign:ignore
type IncidentCapsuleListEnvelope struct {
	Data []domain.IncidentCapsuleSummary `json:"data"`
}

// goalign:ignore
type IncidentCapsuleDryRunEnvelope struct {
	Data domain.IncidentCapsuleDryRun `json:"data"`
}

// goalign:ignore
type BehaviorFingerprintEnvelope struct {
	Data domain.BehaviorFingerprintReport `json:"data"`
}

// goalign:ignore
type PprofConfigEnvelope struct {
	Data domain.PprofConfig `json:"data"`
}

// goalign:ignore
type PprofRuntimeEnvelope struct {
	Data domain.PprofRuntimeStats `json:"data"`
}

// goalign:ignore
type PprofSummaryEnvelope struct {
	Data domain.PprofProfileSummary `json:"data"`
}

// goalign:ignore
type RotateKeyEnvelope struct {
	Data domain.RotateEncryptionKeyResult `json:"data"`
}

// goalign:ignore
type UserInviteEnvelope struct {
	Data domain.UserInvite `json:"data"`
}

// goalign:ignore
type AssistantConfigEnvelope struct {
	Data AssistantConfigResponse `json:"data"`
}

// goalign:ignore
type HealthStatusModel = app.HealthStatus

// ModelCatalog is referenced once so nested domain schemas stay in definitions.
//
// goalign:ignore
type ModelCatalog struct {
	Health               app.HealthStatus                 `json:"health"`
	Account              domain.AccountInfo               `json:"account"`
	Stream               domain.StreamInfo                `json:"stream"`
	StreamConfig         domain.StreamConfigDTO           `json:"streamConfig"`
	Consumer             domain.ConsumerInfo              `json:"consumer"`
	ConsumerConfig       domain.ConsumerConfigDTO         `json:"consumerConfig"`
	Cluster              domain.Cluster                   `json:"cluster"`
	ClusterTest          domain.ClusterTestResult         `json:"clusterTest"`
	Connection           domain.NATSConnectionStatus      `json:"connection"`
	Alert                domain.Alert                     `json:"alert"`
	AlertRule            domain.AlertRule                 `json:"alertRule"`
	AlertOpenSummary     domain.AlertOpenSummary          `json:"alertOpenSummary"`
	User                 domain.User                      `json:"user"`
	AccessGrant          domain.AccessGrant               `json:"accessGrant"`
	AccessRules          domain.AccessRules               `json:"accessRules"`
	AuditEntry           domain.AuditEntry                `json:"auditEntry"`
	NATSUser             domain.NATSAccountUser           `json:"natsUser"`
	NATSCreds            domain.NATSAccountUserCreds      `json:"natsCreds"`
	SigningGroup         domain.SigningGroup              `json:"signingGroup"`
	Export               domain.NATSAccountExport         `json:"export"`
	SubjectPermissions   domain.SubjectPermissionsResult  `json:"subjectPermissions"`
	KVBucket             domain.KVBucketInfo              `json:"kvBucket"`
	KVEntry              domain.KVEntry                   `json:"kvEntry"`
	ObjectBucket         domain.ObjectBucketInfo          `json:"objectBucket"`
	ObjectInfo           domain.ObjectInfo                `json:"objectInfo"`
	StreamMessage        domain.StreamMessage             `json:"streamMessage"`
	PublishResult        domain.PublishMessageResult      `json:"publishResult"`
	MessageRange         domain.MessageRangeResult        `json:"messageRange"`
	DLQList              domain.DLQListResult             `json:"dlqList"`
	DLQRetry             domain.DLQRetryResult            `json:"dlqRetry"`
	Replay               domain.ReplayConsumerResult      `json:"replay"`
	ReplayDryRun         domain.ReplayDryRun              `json:"replayDryRun"`
	BlastRadius          domain.BlastRadius               `json:"blastRadius"`
	MetricsHistory       domain.MetricsHistoryResponse    `json:"metricsHistory"`
	Zombies              domain.ZombieSnapshot            `json:"zombies"`
	SubjectNaming        domain.SubjectNamingSnapshot     `json:"subjectNaming"`
	EventGenome          domain.EventGenomeSnapshot       `json:"eventGenome"`
	EventArchitecture    domain.EventArchitectureSnapshot `json:"eventArchitecture"`
	ArchitectureRefactor domain.ArchitectureRefactorPlan  `json:"architectureRefactor"`
	ArchitectureScore    domain.ArchitectureScoreSnapshot `json:"architectureScore"`
	HiddenBottlenecks    domain.HiddenBottleneckSnapshot  `json:"hiddenBottlenecks"`
	ChaosStory           domain.ChaosStory                `json:"chaosStory"`
	ArchitectureInv      domain.ArchitectureInventory     `json:"architectureInventory"`
	EventCatalog         domain.EventCatalogSnapshot      `json:"eventCatalog"`
	EventWikipedia       domain.EventWikipediaSnapshot    `json:"eventWikipedia"`
	RequestReply         domain.RequestReplySnapshot      `json:"requestReply"`
	IncidentRecon        domain.IncidentReconstruction    `json:"incidentReconstruction"`
	IncidentCapsule      domain.IncidentCapsuleDetail     `json:"incidentCapsule"`
	BehaviorFingerprint  domain.BehaviorFingerprintReport `json:"behaviorFingerprint"`
	PprofConfig          domain.PprofConfig               `json:"pprofConfig"`
	PprofRuntime         domain.PprofRuntimeStats         `json:"pprofRuntime"`
	RotateKey            domain.RotateEncryptionKeyResult `json:"rotateKey"`
	UserInvite           domain.UserInvite                `json:"userInvite"`
	LoginRequest         LoginRequest                     `json:"loginRequest"`
	UserResponse         UserResponse                     `json:"userResponse"`
	AuthConfig           AuthConfigResponse               `json:"authConfig"`
}
