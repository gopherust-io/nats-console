package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrRootProtected = errors.New("root user protected")
)

type Cluster struct {
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	NATSURL       string    `json:"nats_url"`
	MonitoringURL string    `json:"monitoring_url"`
	CredsFilePath string    `json:"-"`
	Token         string    `json:"-"`
	HasCreds      bool      `json:"has_creds"`
	HasToken      bool      `json:"has_token"`
	IsDefault     bool      `json:"is_default"`
}

type ClusterCreate struct {
	Name          string
	NATSURL       string
	MonitoringURL string
	CredsFilePath string
	Token         string
	IsDefault     bool
}

type ClusterUpdate struct {
	Name          *string
	NATSURL       *string
	MonitoringURL *string
	CredsFilePath *string
	Token         *string
	IsDefault     *bool
}

func (s *Store) CountClusters(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM clusters`).Scan(&count)
	return count, err
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at
		FROM clusters
		ORDER BY is_default DESC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []Cluster
	for rows.Next() {
		c, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}
	if clusters == nil {
		return []Cluster{}, rows.Err()
	}
	return clusters, rows.Err()
}

func (s *Store) GetCluster(ctx context.Context, id string) (Cluster, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at
		FROM clusters WHERE id = $1`, id)
	return scanClusterRow(row)
}

func (s *Store) GetDefaultCluster(ctx context.Context) (Cluster, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at
		FROM clusters WHERE is_default = TRUE LIMIT 1`)
	return scanClusterRow(row)
}

func (s *Store) CreateCluster(ctx context.Context, in ClusterCreate) (Cluster, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Cluster{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.IsDefault {
		if _, err := tx.Exec(ctx, `UPDATE clusters SET is_default = FALSE, updated_at = $1`, now); err != nil {
			return Cluster{}, err
		}
	}

	token, err := s.encryptToken(in.Token)
	if err != nil {
		return Cluster{}, fmt.Errorf("encrypt token: %w", err)
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO clusters (id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at`,
		id, in.Name, in.NATSURL, in.MonitoringURL, in.CredsFilePath, token, in.IsDefault, now, now)

	c, err := scanClusterRow(row)
	if err != nil {
		return Cluster{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Cluster{}, err
	}
	return c, nil
}

func (s *Store) UpdateCluster(ctx context.Context, id string, in ClusterUpdate) (Cluster, error) {
	current, err := s.GetCluster(ctx, id)
	if err != nil {
		return Cluster{}, err
	}

	if in.Name != nil {
		current.Name = *in.Name
	}
	if in.NATSURL != nil {
		current.NATSURL = *in.NATSURL
	}
	if in.MonitoringURL != nil {
		current.MonitoringURL = *in.MonitoringURL
	}
	if in.CredsFilePath != nil {
		current.CredsFilePath = *in.CredsFilePath
	}
	if in.Token != nil {
		token, err := s.encryptToken(*in.Token)
		if err != nil {
			return Cluster{}, fmt.Errorf("encrypt token: %w", err)
		}
		current.Token = token
	}
	if in.IsDefault != nil {
		current.IsDefault = *in.IsDefault
	}
	current.UpdatedAt = time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Cluster{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if current.IsDefault {
		if _, err := tx.Exec(ctx, `UPDATE clusters SET is_default = FALSE, updated_at = $1 WHERE id <> $2`, current.UpdatedAt, id); err != nil {
			return Cluster{}, err
		}
	}

	row := tx.QueryRow(ctx, `
		UPDATE clusters
		SET name = $2, nats_url = $3, monitoring_url = $4, creds_file_path = $5, token = $6,
		    is_default = $7, updated_at = $8
		WHERE id = $1
		RETURNING id, name, nats_url, monitoring_url, creds_file_path, token, is_default, created_at, updated_at`,
		id, current.Name, current.NATSURL, current.MonitoringURL, current.CredsFilePath, current.Token, current.IsDefault, current.UpdatedAt)

	c, err := scanClusterRow(row)
	if err != nil {
		return Cluster{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Cluster{}, err
	}
	return c, nil
}

func (s *Store) GetClusterCredentials(ctx context.Context, id string) (Cluster, error) {
	cluster, err := s.GetCluster(ctx, id)
	if err != nil {
		return Cluster{}, err
	}
	token, err := s.decryptToken(cluster.Token)
	if err != nil {
		return Cluster{}, fmt.Errorf("decrypt token: %w", err)
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Store) DeleteCluster(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM clusters WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCluster(rows pgx.Rows) (Cluster, error) {
	var c Cluster
	err := rows.Scan(
		&c.ID, &c.Name, &c.NATSURL, &c.MonitoringURL, &c.CredsFilePath, &c.Token,
		&c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Cluster{}, err
	}
	c.HasCreds = c.CredsFilePath != ""
	c.HasToken = c.Token != ""
	return c, nil
}

func scanClusterRow(row pgx.Row) (Cluster, error) {
	var c Cluster
	err := row.Scan(
		&c.ID, &c.Name, &c.NATSURL, &c.MonitoringURL, &c.CredsFilePath, &c.Token,
		&c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Cluster{}, ErrNotFound
	}
	if err != nil {
		return Cluster{}, fmt.Errorf("scan cluster: %w", err)
	}
	c.HasCreds = c.CredsFilePath != ""
	c.HasToken = c.Token != ""
	return c, nil
}
