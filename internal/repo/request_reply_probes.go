package repo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var ErrRequestReplyProbeNotFound = errors.New("request reply probe not found")

func (db *DB) ListRequestReplyProbes(ctx context.Context, clusterID string) ([]domain.RequestReplyProbe, error) {
	rows, err := db.pool.Query(ctx, queryListRequestReplyProbes, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var probes []domain.RequestReplyProbe
	for rows.Next() {
		probe, scanErr := scanRequestReplyProbe(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		probes = append(probes, probe)
	}
	if probes == nil {
		probes = []domain.RequestReplyProbe{}
	}
	return probes, rows.Err()
}

// ListRequestReplyProbesPublic returns probes with payloads stripped for non-managers.
func (db *DB) ListRequestReplyProbesPublic(ctx context.Context, clusterID string) ([]domain.RequestReplyProbe, error) {
	probes, err := db.ListRequestReplyProbes(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RequestReplyProbe, len(probes))
	for i, p := range probes {
		out[i] = p.WithoutPayload()
	}
	return out, nil
}

func (db *DB) ListEnabledRequestReplyProbes(ctx context.Context, clusterID string) ([]domain.RequestReplyProbe, error) {
	rows, err := db.pool.Query(ctx, queryListEnabledRequestReplyProbes, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var probes []domain.RequestReplyProbe
	for rows.Next() {
		probe, scanErr := scanRequestReplyProbe(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		probes = append(probes, probe)
	}
	return probes, rows.Err()
}

func (db *DB) GetRequestReplyProbe(ctx context.Context, clusterID, probeID string) (domain.RequestReplyProbe, error) {
	row := db.pool.QueryRow(ctx, queryGetRequestReplyProbe, clusterID, probeID)
	probe, err := scanRequestReplyProbe(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RequestReplyProbe{}, ErrRequestReplyProbeNotFound
	}
	return probe, err
}

func (db *DB) CreateRequestReplyProbe(ctx context.Context, clusterID string, in domain.RequestReplyProbeCreate) (domain.RequestReplyProbe, error) {
	if err := in.Validate(); err != nil {
		return domain.RequestReplyProbe{}, err
	}
	subject, err := domain.CanonicalProbeSubject(in.Subject)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	existing, err := db.ListRequestReplyProbes(ctx, clusterID)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	if len(existing) >= domain.MaxRequestReplyProbesPerCluster {
		return domain.RequestReplyProbe{}, fmt.Errorf("at most %d probes per cluster", domain.MaxRequestReplyProbesPerCluster)
	}

	id := newID()
	timeoutMs, err := domain.NormalizeProbeTimeoutMs(in.TimeoutMs)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	format := in.PayloadFormat.Normalize()
	if commonstrings.IsEmpty(format.String()) {
		format = domain.RequestReplyFormatJSON
	}
	payload, err := decodeProbePayload(in.PayloadB64)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	_, err = db.pool.Exec(ctx, queryInsertRequestReplyProbe,
		id, clusterID, subject, payload, format.String(), timeoutMs, enabled,
	)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	return db.GetRequestReplyProbe(ctx, clusterID, id)
}

func (db *DB) UpdateRequestReplyProbe(ctx context.Context, clusterID, probeID string, in domain.RequestReplyProbeUpdate) (domain.RequestReplyProbe, error) {
	current, err := db.GetRequestReplyProbe(ctx, clusterID, probeID)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	if err := in.ValidateUpdate(current.PayloadFormat); err != nil {
		return domain.RequestReplyProbe{}, err
	}

	subject := current.Subject
	if in.Subject != nil {
		canonical, canonErr := domain.CanonicalProbeSubject(*in.Subject)
		if canonErr != nil {
			return domain.RequestReplyProbe{}, canonErr
		}
		subject = canonical
	}
	timeoutMs := current.TimeoutMs
	if in.TimeoutMs != nil {
		normalized, normErr := domain.NormalizeProbeTimeoutMs(*in.TimeoutMs)
		if normErr != nil {
			return domain.RequestReplyProbe{}, normErr
		}
		timeoutMs = normalized
	}
	enabled := current.Enabled
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	format := current.PayloadFormat.Normalize()
	if commonstrings.IsEmpty(format.String()) {
		format = domain.RequestReplyFormatJSON
	}
	if in.PayloadFormat != nil {
		format = in.PayloadFormat.Normalize()
		if commonstrings.IsEmpty(format.String()) {
			format = domain.RequestReplyFormatJSON
		}
	}
	var payload []byte
	if in.PayloadB64 != nil {
		var decodeErr error
		payload, decodeErr = decodeProbePayload(*in.PayloadB64)
		if decodeErr != nil {
			return domain.RequestReplyProbe{}, decodeErr
		}
	} else {
		payload = mustDecodeProbePayload(current.PayloadB64)
	}

	_, err = db.pool.Exec(ctx, queryUpdateRequestReplyProbe,
		clusterID, probeID, subject, payload, format.String(), timeoutMs, enabled,
	)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	return db.GetRequestReplyProbe(ctx, clusterID, probeID)
}

func (db *DB) DeleteRequestReplyProbe(ctx context.Context, clusterID, probeID string) error {
	tag, err := db.pool.Exec(ctx, queryDeleteRequestReplyProbe, clusterID, probeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRequestReplyProbeNotFound
	}
	return nil
}

type requestReplyProbeRow interface {
	Scan(dest ...any) error
}

func scanRequestReplyProbe(row requestReplyProbeRow) (domain.RequestReplyProbe, error) {
	var probe domain.RequestReplyProbe
	var payload []byte
	var format string
	err := row.Scan(
		&probe.ID,
		&probe.ClusterID,
		&probe.Subject,
		&payload,
		&format,
		&probe.TimeoutMs,
		&probe.Enabled,
		&probe.CreatedAt,
		&probe.UpdatedAt,
	)
	if err != nil {
		return domain.RequestReplyProbe{}, err
	}
	probe.PayloadB64 = encodeProbePayload(payload)
	probe.PayloadFormat = domain.RequestReplyPayloadFormat(format).Normalize()
	if commonstrings.IsEmpty(string(probe.PayloadFormat)) {
		probe.PayloadFormat = domain.RequestReplyFormatJSON
	}
	return probe, nil
}

func decodeProbePayload(payloadB64 string) ([]byte, error) {
	if commonstrings.IsEmpty(payloadB64) {
		return []byte{}, nil
	}
	return base64.StdEncoding.DecodeString(payloadB64)
}

func encodeProbePayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(payload)
}

func mustDecodeProbePayload(payloadB64 string) []byte {
	raw, err := decodeProbePayload(payloadB64)
	if err != nil {
		return []byte{}
	}
	return raw
}
