package api

import (
	"time"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
)

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

func toUserResponse(user store.User) UserResponse {
	return userResponseFromDomain(auth.StoreUserToDomain(user))
}

func userResponseFromDomain(user domain.User) UserResponse {
	resp := UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Roles:    nonNilSlice(user.Roles),
		IsRoot:   user.IsRoot,
		Grants:   nonNilSlice(user.Grants),
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
