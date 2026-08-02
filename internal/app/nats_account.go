package app

import (
	"context"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type NATSAccountService struct {
	accounts port.NATSAccountRepository
}

func NewNATSAccountService(accounts port.NATSAccountRepository) *NATSAccountService {
	return &NATSAccountService{accounts: accounts}
}

func (s *NATSAccountService) ListUsers(ctx context.Context, clusterID, accountName string) ([]domain.NATSAccountUser, error) {
	return s.accounts.ListNATSAccountUsers(ctx, clusterID, accountName)
}

func (s *NATSAccountService) GetUser(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUser, error) {
	return s.accounts.GetNATSAccountUser(ctx, clusterID, accountName, userID)
}

func (s *NATSAccountService) CreateUser(ctx context.Context, in domain.NATSAccountUserCreate, accountSeed string) (domain.NATSAccountUserCreds, error) {
	return s.accounts.CreateNATSAccountUserWithSeed(ctx, in, accountSeed)
}

func (s *NATSAccountService) UpdateUser(
	ctx context.Context,
	clusterID, accountName, userID string,
	in domain.NATSAccountUserUpdate,
	accountSeed string,
) (domain.NATSAccountUser, error) {
	return s.accounts.UpdateNATSAccountUser(ctx, clusterID, accountName, userID, in, accountSeed)
}

func (s *NATSAccountService) DeleteUser(ctx context.Context, clusterID, accountName, userID string) ([]string, error) {
	return s.accounts.DeleteNATSAccountUser(ctx, clusterID, accountName, userID)
}

func (s *NATSAccountService) GetCreds(ctx context.Context, clusterID, accountName, userID string) (domain.NATSAccountUserCreds, error) {
	return s.accounts.GetNATSAccountUserCreds(ctx, clusterID, accountName, userID)
}

func (s *NATSAccountService) RotateUser(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error) {
	return s.accounts.RotateNATSAccountUser(ctx, clusterID, accountName, userID, accountSeed)
}

func (s *NATSAccountService) MintJWT(ctx context.Context, clusterID, accountName, userID, accountSeed string) (domain.NATSAccountUserCreds, error) {
	return s.accounts.MintNATSAccountUserJWT(ctx, clusterID, accountName, userID, accountSeed)
}

func (s *NATSAccountService) AssignPerson(ctx context.Context, clusterID, accountName, natsUserID, personUserID string) (domain.NATSAccountUser, error) {
	return s.accounts.AssignNATSAccountUserPerson(ctx, clusterID, accountName, natsUserID, personUserID)
}

func (s *NATSAccountService) SubjectPermissions(ctx context.Context, clusterID, accountName, subject string) (domain.SubjectPermissionsResult, error) {
	return s.accounts.ListSubjectPermissions(ctx, clusterID, accountName, subject)
}

func (s *NATSAccountService) ListSigningGroups(ctx context.Context, clusterID, accountName string) ([]domain.SigningGroup, error) {
	return s.accounts.ListSigningGroups(ctx, clusterID, accountName)
}

func (s *NATSAccountService) CreateSigningGroup(ctx context.Context, in domain.SigningGroupCreate) (domain.SigningGroup, error) {
	return s.accounts.CreateSigningGroup(ctx, in)
}

func (s *NATSAccountService) UpdateSigningGroup(
	ctx context.Context,
	clusterID, accountName, groupID string,
	in domain.SigningGroupUpdate,
) (domain.SigningGroup, error) {
	return s.accounts.UpdateSigningGroup(ctx, clusterID, accountName, groupID, in)
}

func (s *NATSAccountService) DeleteSigningGroup(ctx context.Context, clusterID, accountName, groupID string) error {
	return s.accounts.DeleteSigningGroup(ctx, clusterID, accountName, groupID)
}

func (s *NATSAccountService) ListExports(ctx context.Context, clusterID, accountName, kind string) ([]domain.NATSAccountExport, error) {
	return s.accounts.ListNATSAccountExports(ctx, clusterID, accountName, kind)
}

func (s *NATSAccountService) CreateExport(ctx context.Context, in domain.NATSAccountExportCreate) (domain.NATSAccountExport, error) {
	return s.accounts.CreateNATSAccountExport(ctx, in)
}

func (s *NATSAccountService) UpdateExport(
	ctx context.Context,
	clusterID, accountName, exportID string,
	in domain.NATSAccountExportUpdate,
) (domain.NATSAccountExport, error) {
	return s.accounts.UpdateNATSAccountExport(ctx, clusterID, accountName, exportID, in)
}

func (s *NATSAccountService) DeleteExport(ctx context.Context, clusterID, accountName, exportID string) error {
	return s.accounts.DeleteNATSAccountExport(ctx, clusterID, accountName, exportID)
}
