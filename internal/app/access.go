package app

import (
	"context"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type AccessService struct {
	access port.AccessRepository
}

func NewAccessService(access port.AccessRepository) *AccessService {
	return &AccessService{access: access}
}

func (s *AccessService) CreateInvite(ctx context.Context, userID string, ttl time.Duration) (domain.UserInvite, error) {
	return s.access.CreateUserInvite(ctx, userID, ttl)
}

func (s *AccessService) GetInvite(ctx context.Context, token string) (domain.UserInvite, error) {
	return s.access.GetUserInvite(ctx, token)
}

func (s *AccessService) AcceptInvite(ctx context.Context, token, password string) (domain.User, error) {
	return s.access.AcceptUserInvite(ctx, token, password)
}

func (s *AccessService) ListGrantsByResource(ctx context.Context, resourceType, resourceKey string) ([]domain.AccessGrant, error) {
	return s.access.ListAccessGrantsByResource(ctx, resourceType, resourceKey)
}

func (s *AccessService) UpsertGrant(ctx context.Context, in domain.AccessGrantUpsert) (domain.AccessGrant, error) {
	return s.access.UpsertAccessGrant(ctx, in)
}

func (s *AccessService) DeleteGrantScoped(ctx context.Context, id, resourceType, resourceKey string) (string, error) {
	return s.access.DeleteAccessGrantScoped(ctx, id, resourceType, resourceKey)
}

func (s *AccessService) DeleteGrantByResource(ctx context.Context, userID, resourceType, resourceKey string) error {
	return s.access.DeleteAccessGrantByResource(ctx, userID, resourceType, resourceKey)
}
