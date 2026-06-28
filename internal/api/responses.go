package api

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
)

type paginationMeta struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type StreamsListResponse struct {
	Streams []domain.StreamInfo `json:"streams"`
	Total   int                 `json:"total"`
	Offset  int                 `json:"offset"`
	Limit   int                 `json:"limit"`
}

type ConsumersListResponse struct {
	Consumers []domain.ConsumerInfo `json:"consumers"`
	Total     int                   `json:"total"`
	Offset    int                   `json:"offset"`
	Limit     int                   `json:"limit"`
}

type KeysListResponse struct {
	Keys   []string `json:"keys"`
	Total  int      `json:"total"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
}

type ObjectsListResponse struct {
	Objects []string `json:"objects"`
	Total   int      `json:"total"`
	Offset  int      `json:"offset"`
	Limit   int      `json:"limit"`
}

type KVBucketsListResponse struct {
	Buckets []domain.KVBucketInfo `json:"buckets"`
	Total   int                   `json:"total"`
}

type ObjectBucketsListResponse struct {
	Buckets []domain.ObjectBucketInfo `json:"buckets"`
	Total   int                       `json:"total"`
}

type KVHistoryResponse struct {
	Entries []domain.KVEntry `json:"entries"`
	Total   int              `json:"total"`
}

type UsersListResponse struct {
	Users []domain.User `json:"users"`
	Total int           `json:"total"`
}

type AuditListResponse struct {
	Entries []domain.AuditEntry `json:"entries"`
	Total   int                 `json:"total"`
}

type ClustersListResponse struct {
	Clusters []domain.Cluster `json:"clusters"`
	Total    int              `json:"total"`
}

type ConnectionsListResponse struct {
	Connections []domain.NATSConnectionStatus `json:"connections"`
	Total       int                           `json:"total"`
}

type AuthConfigResponse struct {
	OIDCProviders []auth.ProviderInfo `json:"oidcProviders"`
	OIDCEnabled   bool                `json:"oidcEnabled"`
	BasicEnabled  bool                `json:"basicEnabled"`
	AuthEnabled   bool                `json:"authEnabled"`
	AIEnabled     bool                `json:"aiEnabled"`
}

type UserResponse struct {
	AccessRules *domain.AccessRules `json:"accessRules,omitempty"`
	ID          string              `json:"id"`
	Username    string              `json:"username"`
	Email       string              `json:"email"`
	CreatedAt   string              `json:"createdAt,omitempty"`
	Roles       []string            `json:"roles"`
	IsRoot      bool                `json:"isRoot"`
}

func toUserResponse(user store.User) UserResponse {
	resp := UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Roles:    nonNilSlice(user.Roles),
		IsRoot:   user.IsRoot,
	}
	if user.AccessRules != nil {
		resp.AccessRules = &domain.AccessRules{
			ClusterIDs:      append([]string(nil), user.AccessRules.ClusterIDs...),
			ManageUsers:     user.AccessRules.ManageUsers,
			ViewAudit:       user.AccessRules.ViewAudit,
			DeleteClusters:  user.AccessRules.DeleteClusters,
			AssignableRoles: append([]string(nil), user.AccessRules.AssignableRoles...),
		}
	}
	if !user.CreatedAt.IsZero() {
		resp.CreatedAt = user.CreatedAt.UTC().Format(time.RFC3339)
	}
	return resp
}

type AssistantConfigResponse struct {
	AIProvider string `json:"aiProvider,omitempty"`
	AIModel    string `json:"aiModel,omitempty"`
	AIEnabled  bool   `json:"aiEnabled"`
}

type AssistantErrorResponse struct {
	Error             string `json:"error"`
	Code              string `json:"code"`
	Retryable         bool   `json:"retryable"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
}
