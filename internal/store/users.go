package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

type AccessRules struct {
	ClusterIDs      []string `json:"clusterIds,omitempty"`
	AssignableRoles []string `json:"assignableRoles,omitempty"`
	ManageUsers     bool     `json:"manageUsers"`
	ViewAudit       bool     `json:"viewAudit"`
	DeleteClusters  bool     `json:"deleteClusters"`
}

type User struct {
	CreatedAt   time.Time    `json:"created_at"`
	AccessRules *AccessRules `json:"access_rules,omitempty"`
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	Email       string       `json:"email"`
	OIDCSub     string       `json:"oidc_sub,omitempty"`
	Roles       []string     `json:"roles"`
	IsRoot      bool         `json:"is_root"`
}

type UserCreate struct {
	AccessRules  *AccessRules
	Username     string
	Email        string
	Password     string
	OIDCSub      string
	PasswordHash string
	Roles        []string
	IsRoot       bool
}

type UserUpdate struct {
	Email       *string
	Password    *string
	AccessRules *AccessRules
	Roles       []string
	SetRoles    bool
	SetRules    bool
	ClearRules  bool
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, string, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.email, u.password_hash, u.oidc_sub, u.is_root, u.access_rules, u.created_at
		FROM users u WHERE u.username = $1`, username)

	var u User
	var passwordHash string
	var rulesJSON []byte
	err := row.Scan(&u.ID, &u.Username, &u.Email, &passwordHash, &u.OIDCSub, &u.IsRoot, &rulesJSON, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}
	if err := decodeAccessRules(rulesJSON, &u.AccessRules); err != nil {
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
		SELECT u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at
		FROM users u WHERE u.oidc_sub = $1`, sub)

	var u User
	var rulesJSON []byte
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.IsRoot, &rulesJSON, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if err := decodeAccessRules(rulesJSON, &u.AccessRules); err != nil {
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
		SELECT u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at
		FROM users u WHERE u.id = $1`, id)

	var u User
	var rulesJSON []byte
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.IsRoot, &rulesJSON, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if err := decodeAccessRules(rulesJSON, &u.AccessRules); err != nil {
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
		SELECT u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at,
		       COALESCE(array_agg(r.name ORDER BY r.name) FILTER (WHERE r.name IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		GROUP BY u.id, u.username, u.email, u.oidc_sub, u.is_root, u.access_rules, u.created_at
		ORDER BY u.is_root DESC, u.username ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var rulesJSON []byte
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.IsRoot, &rulesJSON, &u.CreatedAt, &u.Roles); err != nil {
			return nil, err
		}
		if err := decodeAccessRules(rulesJSON, &u.AccessRules); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, in UserCreate) (User, error) {
	if in.IsRoot {
		exists, err := s.HasRootUser(ctx)
		if err != nil {
			return User{}, err
		}
		if exists {
			return User{}, ErrConflict
		}
	}

	id := uuid.New().String()
	passwordHash := in.PasswordHash
	if passwordHash == "" && in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, err
		}
		passwordHash = string(hash)
	}

	rulesJSON, err := encodeAccessRules(in.AccessRules)
	if err != nil {
		return User{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	row := tx.QueryRow(ctx, `
		INSERT INTO users (id, username, email, password_hash, oidc_sub, is_root, access_rules, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, username, email, oidc_sub, is_root, access_rules, created_at`,
		id, in.Username, in.Email, passwordHash, in.OIDCSub, in.IsRoot, rulesJSON, now)

	var u User
	var storedRules []byte
	if err := row.Scan(&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.IsRoot, &storedRules, &u.CreatedAt); err != nil {
		return User{}, err
	}
	if err := decodeAccessRules(storedRules, &u.AccessRules); err != nil {
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

func (s *Store) UpdateUser(ctx context.Context, userID string, in UserUpdate) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.Email != nil || in.Password != nil {
		if in.Email != nil && in.Password != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
			if err != nil {
				return User{}, err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE users SET email = $2, password_hash = $3 WHERE id = $1`,
				userID, *in.Email, string(hash)); err != nil {
				return User{}, err
			}
		} else if in.Email != nil {
			if _, err := tx.Exec(ctx, `UPDATE users SET email = $2 WHERE id = $1`, userID, *in.Email); err != nil {
				return User{}, err
			}
		} else if in.Password != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
			if err != nil {
				return User{}, err
			}
			if _, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, string(hash)); err != nil {
				return User{}, err
			}
		}
	}

	if in.SetRules {
		var rulesJSON []byte
		if in.ClearRules {
			rulesJSON = nil
		} else {
			var err error
			rulesJSON, err = encodeAccessRules(in.AccessRules)
			if err != nil {
				return User{}, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET access_rules = $2 WHERE id = $1`, userID, rulesJSON); err != nil {
			return User{}, err
		}
	}

	if in.SetRoles {
		if err := s.setUserRolesTx(ctx, tx, userID, in.Roles); err != nil {
			return User{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return s.GetUserByID(ctx, userID)
}

func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsRoot {
		return ErrRootProtected
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1 AND is_root = false`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetUserRoles(ctx context.Context, userID string, roles []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

func (s *Store) HasRootUser(ctx context.Context) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE is_root = true)`).Scan(&exists)
	return exists, err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HasRole(roles []string, role string) bool {
	return slices.Contains(roles, role)
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

func encodeAccessRules(rules *AccessRules) ([]byte, error) {
	if rules == nil {
		return nil, nil
	}
	return json.Marshal(rules)
}

func decodeAccessRules(data []byte, out **AccessRules) error {
	if len(data) == 0 {
		*out = nil
		return nil
	}
	var rules AccessRules
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}
	*out = &rules
	return nil
}
