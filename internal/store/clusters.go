package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrRootProtected = errors.New("root user protected")
)

// goalign:ignore
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

// goalign:ignore
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
	err := s.pool.QueryRow(ctx, queryCountClusters).Scan(&count)
	return count, err
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	rows, err := s.pool.Query(ctx, queryListClusters)
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
	return scanClusterRow(s.pool.QueryRow(ctx, queryGetClusterByID, id))
}

func (s *Store) GetDefaultCluster(ctx context.Context) (Cluster, error) {
	return scanClusterRow(s.pool.QueryRow(ctx, queryGetDefaultCluster))
}

func (s *Store) CreateCluster(ctx context.Context, in ClusterCreate) (Cluster, error) {
	id := newID()
	now := time.Now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Cluster{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if in.IsDefault {
		if _, err := tx.Exec(ctx, queryClearDefaultClusters, now); err != nil {
			return Cluster{}, err
		}
	}

	token, err := s.encryptToken(in.Token)
	if err != nil {
		return Cluster{}, fmt.Errorf("encrypt token: %w", err)
	}

	c, err := scanClusterRow(tx.QueryRow(ctx, queryInsertCluster,
		id, in.Name, in.NATSURL, in.MonitoringURL, in.CredsFilePath, token, in.IsDefault, now, now))
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
		if _, err := tx.Exec(ctx, queryClearDefaultClustersExcept, current.UpdatedAt, id); err != nil {
			return Cluster{}, err
		}
	}

	c, err := scanClusterRow(tx.QueryRow(ctx, queryUpdateCluster,
		id, current.Name, current.NATSURL, current.MonitoringURL, current.CredsFilePath, current.Token, current.IsDefault, current.UpdatedAt))
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
	tag, err := s.pool.Exec(ctx, queryDeleteCluster, id)
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
	c.HasCreds = !strings.IsEmpty(c.CredsFilePath)
	c.HasToken = !strings.IsEmpty(c.Token)
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
	c.HasCreds = !strings.IsEmpty(c.CredsFilePath)
	c.HasToken = !strings.IsEmpty(c.Token)
	return c, nil
}
