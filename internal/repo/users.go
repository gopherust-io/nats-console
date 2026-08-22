package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	RoleAdmin    = domain.RoleAdmin
	RoleOperator = domain.RoleOperator
	RoleViewer   = domain.RoleViewer
)

// goalign:ignore
type AccessRules struct {
	ClusterIDs      []string `json:"clusterIds,omitempty"`
	AssignableRoles []string `json:"assignableRoles,omitempty"`
	ManageUsers     bool     `json:"manageUsers"`
	ViewAudit       bool     `json:"viewAudit"`
	DeleteClusters  bool     `json:"deleteClusters"`
}

// goalign:ignore
type User struct {
	CreatedAt      time.Time     `json:"created_at"`
	AccessRules    *AccessRules  `json:"access_rules,omitempty"`
	Grants         []AccessGrant `json:"grants,omitempty"`
	ID             string        `json:"id"`
	Username       string        `json:"username"`
	Email          string        `json:"email"`
	OIDCSub        string        `json:"oidc_sub,omitempty"`
	Roles          []string      `json:"roles"`
	IsRoot         bool          `json:"is_root"`
	SessionVersion int64         `json:"-"`
}

// goalign:ignore
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

// goalign:ignore
type UserUpdate struct {
	Email       *string
	Password    *string
	AccessRules *AccessRules
	Roles       []string
	SetRoles    bool
	SetRules    bool
	ClearRules  bool
}

func (db *DB) GetUserByUsername(ctx context.Context, username string) (User, string, error) {
	row := db.pool.QueryRow(ctx, queryGetUserByUsername, username)

	var u User
	var passwordHash string
	var rulesJSON, grantsJSON []byte
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.IsRoot, &rulesJSON, &u.CreatedAt,
		&u.Roles, &grantsJSON, &passwordHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}
	if err := decodeAccessRules(rulesJSON, &u.AccessRules); err != nil {
		return User{}, "", err
	}
	if err := decodeUserGrants(grantsJSON, &u.Grants); err != nil {
		return User{}, "", err
	}
	return u, passwordHash, nil
}

func (db *DB) GetUserByOIDCSub(ctx context.Context, sub string) (User, error) {
	return db.getUserWhere(ctx, "u.oidc_sub = $1", sub)
}

func (db *DB) GetUserByID(ctx context.Context, id string) (User, error) {
	return db.getUserWhere(ctx, "u.id = $1", id)
}

func (db *DB) getUserWhere(ctx context.Context, where string, arg any) (User, error) {
	row := db.pool.QueryRow(ctx, userSelectWithRolesGrants+queryGetUserFromWhere+where, arg)

	var u User
	var rulesJSON, grantsJSON []byte
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.OIDCSub, &u.IsRoot, &rulesJSON, &u.CreatedAt,
		&u.Roles, &grantsJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if err := decodeAccessRules(rulesJSON, &u.AccessRules); err != nil {
		return User{}, err
	}
	if err := decodeUserGrants(grantsJSON, &u.Grants); err != nil {
		return User{}, err
	}
	return u, nil
}

func (db *DB) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := db.pool.Query(ctx, queryListUsers)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := db.attachGrants(ctx, users); err != nil {
		return nil, err
	}
	return users, nil
}

func (db *DB) CreateUser(ctx context.Context, in UserCreate) (User, error) {
	if in.IsRoot {
		exists, err := db.HasRootUser(ctx)
		if err != nil {
			return User{}, err
		}
		if exists {
			return User{}, ErrConflict
		}
	}

	id := newID()
	passwordHash := in.PasswordHash
	if strings.IsEmpty(passwordHash) && !strings.IsEmpty(in.Password) {
		hash, err := bcrypt.GenerateFromPassword(strings.StringToBytes(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return User{}, err
		}
		passwordHash = strings.BytesToString(hash)
	}

	rulesJSON, err := encodeAccessRules(in.AccessRules)
	if err != nil {
		return User{}, err
	}

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	row := tx.QueryRow(ctx, queryInsertUser,
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
	if err := db.setUserRolesTx(ctx, tx, u.ID, roles); err != nil {
		return User{}, err
	}
	u.Roles = roles

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

func (db *DB) UpdateUser(ctx context.Context, userID string, in UserUpdate) (User, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.Email != nil || in.Password != nil {
		if in.Email != nil && in.Password != nil {
			hash, err := bcrypt.GenerateFromPassword(strings.StringToBytes(*in.Password), bcrypt.DefaultCost)
			if err != nil {
				return User{}, err
			}
			if _, err := tx.Exec(ctx, queryUpdateUserEmailPassword,
				userID, *in.Email, strings.BytesToString(hash)); err != nil {
				return User{}, err
			}
		} else if in.Email != nil {
			if _, err := tx.Exec(ctx, queryUpdateUserEmail, userID, *in.Email); err != nil {
				return User{}, err
			}
		} else if in.Password != nil {
			hash, err := bcrypt.GenerateFromPassword(strings.StringToBytes(*in.Password), bcrypt.DefaultCost)
			if err != nil {
				return User{}, err
			}
			if _, err := tx.Exec(ctx, queryUpdateUserPassword, userID, strings.BytesToString(hash)); err != nil {
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
		if _, err := tx.Exec(ctx, queryUpdateUserAccessRules, userID, rulesJSON); err != nil {
			return User{}, err
		}
	}

	if in.SetRoles {
		if err := db.setUserRolesTx(ctx, tx, userID, in.Roles); err != nil {
			return User{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return db.GetUserByID(ctx, userID)
}

func (db *DB) DeleteUser(ctx context.Context, userID string) error {
	user, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.IsRoot {
		return ErrRootProtected
	}
	tag, err := db.pool.Exec(ctx, queryDeleteUser, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) SetUserRoles(ctx context.Context, userID string, roles []string) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := db.setUserRolesTx(ctx, tx, userID, roles); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) setUserRolesTx(ctx context.Context, tx pgx.Tx, userID string, roles []string) error {
	if _, err := tx.Exec(ctx, queryDeleteUserRoles, userID); err != nil {
		return err
	}
	for _, role := range roles {
		var roleID int
		if err := tx.QueryRow(ctx, queryGetRoleIDByName, role).Scan(&roleID); err != nil {
			return fmt.Errorf("unknown role %q: %w", role, err)
		}
		if _, err := tx.Exec(ctx, queryInsertUserRole, userID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := db.pool.QueryRow(ctx, queryCountUsers).Scan(&count)
	return count, err
}

func (db *DB) HasRootUser(ctx context.Context) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx, queryHasRootUser).Scan(&exists)
	return exists, err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword(strings.StringToBytes(hash), strings.StringToBytes(password)) == nil
}

func HighestRole(roles []string) string {
	return domain.HighestRole(roles)
}

func encodeAccessRules(rules *AccessRules) ([]byte, error) {
	if rules == nil {
		return nil, nil
	}
	return serializer.Marshal(rules)
}

func decodeAccessRules(data []byte, out **AccessRules) error {
	if len(data) == 0 {
		*out = nil
		return nil
	}
	var rules AccessRules
	if err := serializer.Unmarshal(data, &rules); err != nil {
		return err
	}
	*out = &rules
	return nil
}

type grantJSON struct {
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ResourceType string    `json:"resource_type"`
	ResourceKey  string    `json:"resource_key"`
	Role         string    `json:"role"`
}

func decodeUserGrants(data []byte, out *[]AccessGrant) error {
	if len(data) == 0 || strings.BytesToString(data) == "[]" || strings.BytesToString(data) == "null" {
		*out = []AccessGrant{}
		return nil
	}
	var rows []grantJSON
	if err := serializer.Unmarshal(data, &rows); err != nil {
		return err
	}
	grants := make([]AccessGrant, 0, len(rows))
	for _, g := range rows {
		grants = append(grants, AccessGrant{
			ID:           g.ID,
			UserID:       g.UserID,
			ResourceType: g.ResourceType,
			ResourceKey:  g.ResourceKey,
			Role:         g.Role,
			CreatedAt:    g.CreatedAt,
			UpdatedAt:    g.UpdatedAt,
		})
	}
	*out = grants
	return nil
}
