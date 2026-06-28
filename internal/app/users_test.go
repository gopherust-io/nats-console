package app_test

import (
	"context"
	"testing"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUsers struct {
	users map[string]domain.User
}

func (m *mockUsers) List(ctx context.Context) ([]domain.User, error) {
	out := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, nil
}

func (m *mockUsers) GetByID(ctx context.Context, id string) (domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (m *mockUsers) GetByUsername(ctx context.Context, username string) (domain.User, string, error) {
	return domain.User{}, "", domain.ErrNotFound
}

func (m *mockUsers) GetByOIDCSub(ctx context.Context, sub string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (m *mockUsers) CreateUser(ctx context.Context, in domain.UserCreate) (domain.User, error) {
	u := domain.User{
		ID:          "new-user",
		Username:    in.Username,
		Email:       in.Email,
		Roles:       in.Roles,
		AccessRules: in.AccessRules,
	}
	m.users[u.ID] = u
	return u, nil
}

func (m *mockUsers) UpdateUser(ctx context.Context, userID string, in domain.UserUpdate) (domain.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	if in.SetRoles {
		u.Roles = in.Roles
	}
	if in.SetRules {
		u.AccessRules = in.AccessRules
	}
	m.users[userID] = u
	return u, nil
}

func (m *mockUsers) DeleteUser(ctx context.Context, userID string) error {
	if _, ok := m.users[userID]; !ok {
		return domain.ErrNotFound
	}
	delete(m.users, userID)
	return nil
}

func (m *mockUsers) SetRoles(ctx context.Context, userID string, roles []string) error {
	_, err := m.UpdateUser(ctx, userID, domain.UserUpdate{Roles: roles, SetRoles: true})
	return err
}

func (m *mockUsers) CountUsers(ctx context.Context) (int, error) {
	return len(m.users), nil
}

func (m *mockUsers) HasRootUser(ctx context.Context) (bool, error) {
	for _, u := range m.users {
		if u.IsRoot {
			return true, nil
		}
	}
	return false, nil
}

func TestUserServiceProtectsRoot(t *testing.T) {
	repo := &mockUsers{users: map[string]domain.User{
		"root": {ID: "root", Username: "root", IsRoot: true, Roles: []string{domain.RoleAdmin}},
		"bob":  {ID: "bob", Username: "bob", Roles: []string{domain.RoleAdmin}},
	}}
	svc := app.NewUserService(repo)
	admin := domain.User{
		ID:    "bob",
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ManageUsers:     true,
			AssignableRoles: []string{domain.RoleViewer},
		},
	}

	_, err := svc.SetRoles(context.Background(), admin, "root", []string{domain.RoleViewer})
	require.ErrorIs(t, err, domain.ErrRootProtected)

	err = svc.Delete(context.Background(), admin, "root")
	require.ErrorIs(t, err, domain.ErrRootProtected)
}

func TestUserServiceRootCanCreateDelegatedAdmin(t *testing.T) {
	repo := &mockUsers{users: map[string]domain.User{}}
	svc := app.NewUserService(repo)
	root := domain.User{ID: "root", IsRoot: true, Roles: []string{domain.RoleAdmin}}

	created, err := svc.Create(context.Background(), root, domain.UserCreate{
		Username: "delegate",
		Email:    "delegate@example.com",
		Password: "secret",
		Roles:    []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ManageUsers:     true,
			ViewAudit:       true,
			AssignableRoles: []string{domain.RoleOperator, domain.RoleViewer},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "delegate", created.Username)
}
