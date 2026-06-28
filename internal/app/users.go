package app

import (
	"context"
	"errors"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

type UserService struct {
	users port.UserRepository
}

func NewUserService(users port.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) List(ctx context.Context, actor domain.User) ([]domain.User, error) {
	perms := domain.PermissionsFor(actor)
	if !perms.ManageUsers && !perms.IsRoot {
		return nil, domain.ErrForbidden
	}
	users, err := s.users.List(ctx)
	if err != nil {
		return nil, err
	}
	if perms.IsRoot {
		return users, nil
	}
	filtered := make([]domain.User, 0, len(users))
	for _, user := range users {
		if user.IsRoot {
			continue
		}
		if domain.PermissionsFor(user).SupersetOf(perms) || actor.ID == user.ID {
			filtered = append(filtered, user)
		}
	}
	return filtered, nil
}

func (s *UserService) Create(ctx context.Context, actor domain.User, in domain.UserCreate) (domain.User, error) {
	perms := domain.PermissionsFor(actor)
	if !perms.ManageUsers {
		return domain.User{}, domain.ErrForbidden
	}
	if in.IsRoot {
		return domain.User{}, domain.ErrForbidden
	}
	if err := validateUserInput(in.Roles, in.AccessRules); err != nil {
		return domain.User{}, err
	}
	if !perms.IsRoot {
		if domain.HasRole(in.Roles, domain.RoleAdmin) && in.AccessRules == nil {
			return domain.User{}, domain.ErrCannotEscalate
		}
		if !perms.AllowsRoles(in.Roles) {
			return domain.User{}, domain.ErrCannotEscalate
		}
		if !perms.CanAssignAccessRules(in.AccessRules) {
			return domain.User{}, domain.ErrCannotEscalate
		}
	}
	return s.users.CreateUser(ctx, in)
}

func (s *UserService) Update(ctx context.Context, actor domain.User, userID string, in domain.UserUpdate) (domain.User, error) {
	target, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.authorizeUserMutation(actor, target); err != nil {
		return domain.User{}, err
	}
	perms := domain.PermissionsFor(actor)
	if in.SetRoles {
		if err := validateUserInput(in.Roles, nil); err != nil {
			return domain.User{}, err
		}
		if !perms.AllowsRoles(in.Roles) {
			return domain.User{}, domain.ErrCannotEscalate
		}
	}
	if in.SetRules {
		if err := domain.ValidateAccessRules(in.AccessRules); err != nil {
			return domain.User{}, err
		}
		if !perms.CanAssignAccessRules(in.AccessRules) {
			return domain.User{}, domain.ErrCannotEscalate
		}
	}
	return s.users.UpdateUser(ctx, userID, in)
}

func (s *UserService) Delete(ctx context.Context, actor domain.User, userID string) error {
	if actor.ID == userID {
		return domain.ErrForbidden
	}
	target, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.authorizeUserMutation(actor, target); err != nil {
		return err
	}
	return s.users.DeleteUser(ctx, userID)
}

func (s *UserService) SetRoles(ctx context.Context, actor domain.User, userID string, roles []string) (domain.User, error) {
	return s.Update(ctx, actor, userID, domain.UserUpdate{
		Roles:    roles,
		SetRoles: true,
	})
}

func (s *UserService) authorizeUserMutation(actor, target domain.User) error {
	if target.IsRoot && !actor.IsRoot {
		return domain.ErrRootProtected
	}
	if actor.ID == target.ID && !actor.IsRoot {
		return domain.ErrForbidden
	}
	perms := domain.PermissionsFor(actor)
	if !perms.ManageUsers {
		return domain.ErrForbidden
	}
	if perms.IsRoot {
		return nil
	}
	if !domain.PermissionsFor(target).SupersetOf(perms) && actor.ID != target.ID {
		return domain.ErrForbidden
	}
	return nil
}

func validateUserInput(roles []string, rules *domain.AccessRules) error {
	if len(roles) == 0 {
		return errors.New("roles required")
	}
	for _, role := range roles {
		switch role {
		case domain.RoleAdmin, domain.RoleOperator, domain.RoleViewer:
		default:
			return errors.New("invalid role: " + role)
		}
	}
	if domain.HasRole(roles, domain.RoleAdmin) && rules == nil {
		return domain.ValidateAccessRules(rules)
	}
	if rules == nil {
		return errors.New("accessRules required")
	}
	return domain.ValidateAccessRules(rules)
}
