package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	OIDCSub   string    `json:"oidc_sub,omitempty"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

type UserCreate struct {
	Username     string
	Email        string
	Password     string
	OIDCSub      string
	Roles        []string
	PasswordHash string
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, string, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.password_hash, u.oidc_sub, u.created_at
		FROM users u WHERE u.username = $1`, username)

	var u User
	var passwordHash string
	err := row.Scan(&u.ID, &u.Username, &u.Email, &passwordHash, &u.OIDCSub, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}

	roles, err := s.listUserRoles(ctx, u.ID)
	if err != nil {
		return User{}, "", err
	}
	u.Roles = roles
	return u, passwordHash, nil
}

func (s *Store) GetUserByOIDCSub(ctx context.Context, sub string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.oidc_sub, u.created_at
		FROM users u WHERE u.oidc_sub = $1`, sub)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	roles, err := s.listUserRoles(ctx, u.ID)
	if err != nil {
		return User{}, err
	}
	u.Roles = roles
	return u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.oidc_sub, u.created_at
		FROM users u WHERE u.id = $1`, id)

	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	roles, err := s.listUserRoles(ctx, u.ID)
	if err != nil {
		return User{}, err
	}
	u.Roles = roles
	return u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.username, u.email, u.oidc_sub, u.created_at
		FROM users u ORDER BY u.username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.CreatedAt); err != nil {
			return nil, err
		}
		roles, err := s.listUserRoles(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		u.Roles = roles
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, in UserCreate) (User, error) {
	id := uuid.New().String()
	passwordHash := in.PasswordHash
	if passwordHash == "" && in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, err
		}
		passwordHash = string(hash)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()
	row := tx.QueryRow(ctx, `
		INSERT INTO users (id, username, email, password_hash, oidc_sub, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, username, email, oidc_sub, created_at`,
		id, in.Username, in.Email, passwordHash, in.OIDCSub, now)

	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.CreatedAt); err != nil {
		return User{}, err
	}

	roles := in.Roles
	if len(roles) == 0 {
		roles = []string{RoleViewer}
	}
	if err := s.setUserRolesTx(ctx, tx, u.ID, roles); err != nil {
		return User{}, err
	}
	u.Roles = roles

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Store) SetUserRoles(ctx context.Context, userID string, roles []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.setUserRolesTx(ctx, tx, userID, roles); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) setUserRolesTx(ctx context.Context, tx pgx.Tx, userID string, roles []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, role := range roles {
		var roleID int
		if err := tx.QueryRow(ctx, `SELECT id FROM roles WHERE name = $1`, role).Scan(&roleID); err != nil {
			return fmt.Errorf("unknown role %q: %w", role, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) listUserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.name FROM roles r
		JOIN user_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1
		ORDER BY r.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func HighestRole(roles []string) string {
	if HasRole(roles, RoleAdmin) {
		return RoleAdmin
	}
	if HasRole(roles, RoleOperator) {
		return RoleOperator
	}
	if HasRole(roles, RoleViewer) {
		return RoleViewer
	}
	return ""
}
