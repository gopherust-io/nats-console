package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type AdminService struct {
	encryption port.EncryptionRepository
}

func NewAdminService(encryption port.EncryptionRepository) *AdminService {
	return &AdminService{encryption: encryption}
}

func (s *AdminService) RotateEncryptionKeys(ctx context.Context, currentKey, newKey string, dryRun bool) (domain.EncryptionRotationStats, error) {
	return s.encryption.RotateEncryptionKeys(ctx, currentKey, newKey, dryRun)
}
