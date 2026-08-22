package natsclient

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	libnats "github.com/gopherust-io/nats"
	natspkg "github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (c *Client) CaptureIncidentCapsule(ctx context.Context, stream string, req domain.IncidentCapsuleCaptureRequest) (*domain.IncidentCapsuleDetail, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	trigger := libnats.IncidentTrigger(strings.TrimSpace(req.Trigger))
	if commonstrings.IsEmpty(string(trigger)) {
		trigger = libnats.TriggerManual
	}
	capsule, err := c.natsCl.Incidents().Capture(ctx, libnats.IncidentCapture{
		Stream:      stream,
		Consumer:    strings.TrimSpace(req.Consumer),
		FailingSeq:  req.FailingSeq,
		Window:      req.NormalizedWindow(),
		Subject:     strings.TrimSpace(req.Subject),
		Reason:      strings.TrimSpace(req.Reason),
		Trigger:     trigger,
		StoreBucket: domain.DefaultIncidentCapsuleBucket,
		IndexBucket: domain.DefaultIncidentIndexBucket,
		Redact:      defaultConsolCapsuleRedact,
	})
	if err != nil {
		return nil, err
	}

	detail := capsuleToDetail(capsule)
	return &detail, nil
}

func (c *Client) CaptureIncidentCapsuleFromDLQ(ctx context.Context, dlqStream string, seq uint64) (*domain.IncidentCapsuleDetail, error) {
	if seq == 0 {
		return nil, errors.New("seq must be positive")
	}
	if _, err := c.ensureDLQStream(ctx, dlqStream); err != nil {
		return nil, err
	}
	raw, err := c.GetMessage(ctx, dlqStream, seq)
	if err != nil {
		return nil, err
	}
	dlq := domain.DLQMessageFromStream(domain.StreamMessageFromRaw(raw))
	if commonstrings.IsEmpty(dlq.SourceStream) {
		return nil, domain.ErrCapsuleSourceStreamRequired
	}
	if dlq.SourceSeq == 0 {
		return nil, domain.ErrCapsuleSourceSeqRequired
	}
	consumer := strings.TrimSpace(dlq.Consumer)
	if commonstrings.IsEmpty(consumer) {
		return nil, domain.ErrCapsuleConsumerRequired
	}
	reason := dlq.AutopsyError
	if commonstrings.IsEmpty(reason) {
		reason = dlq.Reason
	}
	return c.CaptureIncidentCapsule(ctx, dlq.SourceStream, domain.IncidentCapsuleCaptureRequest{
		Consumer:   consumer,
		FailingSeq: dlq.SourceSeq,
		Subject:    dlq.OriginalSubject,
		Reason:     reason,
		Trigger:    string(libnats.TriggerDLQ),
	})
}

func (c *Client) ListIncidentCapsules(ctx context.Context, stream, consumer string) ([]domain.IncidentCapsuleSummary, error) {
	if commonstrings.IsEmpty(strings.TrimSpace(consumer)) {
		return nil, domain.ErrCapsuleConsumerRequired
	}

	ids, err := c.natsCl.Incidents().List(ctx, stream, consumer, domain.DefaultIncidentIndexBucket)
	if err != nil {
		return nil, err
	}

	out := make([]domain.IncidentCapsuleSummary, 0, len(ids))
	for _, id := range ids {
		sum := domain.IncidentCapsuleSummary{
			ID:       id,
			Stream:   stream,
			Consumer: consumer,
		}
		if loaded, lErr := c.natsCl.Incidents().Load(ctx, domain.DefaultIncidentCapsuleBucket, id); lErr == nil && loaded != nil {
			sum.Trigger = string(loaded.Trigger)
			sum.Reason = loaded.Reason
			sum.FailingSeq = loaded.FailingSeq
			sum.CreatedAt = loaded.CreatedAt
		}
		out = append(out, sum)
	}
	return out, nil
}

func (c *Client) LoadIncidentCapsule(ctx context.Context, id, bucket string) (*domain.IncidentCapsuleDetail, error) {
	if commonstrings.IsEmpty(strings.TrimSpace(id)) {
		return nil, domain.ErrCapsuleIDRequired
	}
	store, _ := domain.CapsuleBuckets(bucket, "")
	capsule, err := c.natsCl.Incidents().Load(ctx, store, id)
	if err != nil {
		return nil, err
	}
	detail := capsuleToDetail(capsule)
	return &detail, nil
}

func (c *Client) PreviewIncidentCapsule(ctx context.Context, id, bucket string) (*domain.IncidentCapsuleDryRun, error) {
	detail, err := c.LoadIncidentCapsule(ctx, id, bucket)
	if err != nil {
		return nil, err
	}
	store, _ := domain.CapsuleBuckets(bucket, "")
	capsule, err := c.natsCl.Incidents().Load(ctx, store, id)
	if err != nil {
		return nil, err
	}

	subjects := make([]string, 0, len(detail.Messages))
	seen := map[string]struct{}{}
	for _, m := range detail.Messages {
		if _, ok := seen[m.Subject]; ok {
			continue
		}
		seen[m.Subject] = struct{}{}
		subjects = append(subjects, m.Subject)
	}

	preview := detail.Messages
	if len(preview) > domain.MaxCapsulePreviewMessages {
		preview = preview[:domain.MaxCapsulePreviewMessages]
	}

	invoked := 0
	_ = c.natsCl.Incidents().ReplayLocal(ctx, capsule, func(_ context.Context, _ *natspkg.Msg) error {
		invoked++
		return nil
	})

	return &domain.IncidentCapsuleDryRun{
		ID:           detail.ID,
		Stream:       detail.Stream,
		Consumer:     detail.Consumer,
		FailingSeq:   detail.FailingSeq,
		MessageCount: detail.MessageCount,
		Subjects:     subjects,
		Preview:      preview,
		Invoked:      invoked,
	}, nil
}

func defaultConsolCapsuleRedact(msg *libnats.CapsuleMessage) {
	if msg == nil || msg.Header == nil {
		return
	}
	for _, k := range []string{"Authorization", "Nats-Api-Token", "Cookie", "X-Api-Key"} {
		delete(msg.Header, k)
	}
}

func capsuleToDetail(c *libnats.Capsule) domain.IncidentCapsuleDetail {
	if c == nil {
		return domain.IncidentCapsuleDetail{}
	}
	messages := make([]domain.IncidentCapsuleMessage, 0, len(c.Messages))
	for _, m := range c.Messages {
		messages = append(messages, capsuleMessageToAPI(m))
	}
	timeline := make([]domain.IncidentEventPreview, 0, len(c.FlightTimeline))
	for _, ev := range c.FlightTimeline {
		timeline = append(timeline, domain.IncidentEventPreview{
			At:      ev.Time,
			Kind:    fmt.Sprintf("%d", ev.Kind),
			Subject: ev.Subject,
			Detail:  truncateString(ev.Detail, domain.MaxCapsulePreviewBytes),
		})
	}
	return domain.IncidentCapsuleDetail{
		CreatedAt:      c.CreatedAt,
		ID:             c.ID,
		Stream:         c.Stream,
		Consumer:       c.Consumer,
		Trigger:        string(c.Trigger),
		Subject:        c.Subject,
		Reason:         c.Reason,
		Messages:       messages,
		FlightTimeline: timeline,
		FailingSeq:     c.FailingSeq,
		SchemaVersion:  c.SchemaVersion,
		MessageCount:   len(c.Messages),
		HasFingerprint: c.Fingerprint != nil,
	}
}

func capsuleMessageToAPI(m libnats.CapsuleMessage) domain.IncidentCapsuleMessage {
	data := m.Data
	truncated := false
	if len(data) > domain.MaxCapsulePreviewBytes {
		data = data[:domain.MaxCapsulePreviewBytes]
		truncated = true
	}
	headers := make(map[string]string, len(m.Header))
	for k, vs := range m.Header {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}
	return domain.IncidentCapsuleMessage{
		Time:      m.Time,
		Headers:   headers,
		Subject:   m.Subject,
		Data:      base64.StdEncoding.EncodeToString(data),
		Sequence:  m.Sequence,
		Truncated: truncated,
	}
}

func truncateString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("…(%d more)", len(s)-n)
}
