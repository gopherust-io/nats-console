package api

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
)

// LoginRequest is the body for POST /api/v1/auth/login.
//
// goalign:ignore
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password" format:"password"`
}

// HealthEnvelope wraps HealthStatus in the standard API envelope (for swag).
//
// goalign:ignore
type HealthEnvelope struct {
	Data app.HealthStatus `json:"data"`
}

// AuthConfigEnvelope wraps AuthConfigResponse (for swag).
//
// goalign:ignore
type AuthConfigEnvelope struct {
	Data AuthConfigResponse `json:"data"`
}

// UserEnvelope wraps UserResponse (for swag).
//
// goalign:ignore
type UserEnvelope struct {
	Data UserResponse `json:"data"`
}

// ErrorBody is the API error payload (mirrored for swag; see httpstatus.ErrorBody).
//
// goalign:ignore
type ErrorBody struct {
	Message           string `json:"message"`
	Code              string `json:"code"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
	Retryable         bool   `json:"retryable,omitempty"`
}

// ErrorEnvelope wraps ErrorBody (for swag).
//
// goalign:ignore
type ErrorEnvelope struct {
	Error *ErrorBody `json:"error"`
}

// goalign:ignore
type AuthConfigResponse struct {
	BasicEnabled bool `json:"basicEnabled"`
	AuthEnabled  bool `json:"authEnabled"`
	AIEnabled    bool `json:"aiEnabled"`
}

// goalign:ignore
type UserResponse struct {
	AccessRules *domain.AccessRules  `json:"accessRules,omitempty"`
	Grants      []domain.AccessGrant `json:"grants,omitempty"`
	ID          string               `json:"id"`
	Username    string               `json:"username"`
	Email       string               `json:"email"`
	CreatedAt   string               `json:"createdAt,omitempty"`
	Roles       []string             `json:"roles"`
	IsRoot      bool                 `json:"isRoot"`
}

func toUserResponse(user domain.User) UserResponse {
	return userResponseFromDomain(user)
}

func userResponseFromDomain(user domain.User) UserResponse {
	resp := UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Roles:    apikit.NonNilSlice(user.Roles),
		IsRoot:   user.IsRoot,
		Grants:   apikit.NonNilSlice(user.Grants),
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

func toUserResponses(users []domain.User) []UserResponse {
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, userResponseFromDomain(u))
	}
	return out
}

// goalign:ignore
type AssistantConfigResponse struct {
	AIProvider string `json:"aiProvider,omitempty"`
	AIModel    string `json:"aiModel,omitempty"`
	AIEnabled  bool   `json:"aiEnabled"`
}
