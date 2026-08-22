package natsclient

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/internal/domain"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func (c *Client) DeleteMessage(_ context.Context, stream string, seq uint64) error {
	js := c.JetStream()
	if js == nil {
		return errors.New("JetStream not available")
	}
	return js.DeleteMsg(stream, seq)
}

func (c *Client) ensureDLQStream(ctx context.Context, stream string) (*nats.StreamInfo, error) {
	info, err := c.StreamInfo(ctx, stream)
	if err != nil {
		return nil, err
	}
	if !domain.IsDLQStream(info.Config.Name, info.Config.Metadata) {
		return nil, domain.ErrNotDLQStream
	}
	return info, nil
}

func (c *Client) ListDLQMessages(ctx context.Context, stream string, startSeq uint64, limit int) (*domain.DLQListResult, error) {
	info, err := c.ensureDLQStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	limit = domain.NormalizeDLQListLimit(limit)
	if info.State.Msgs == 0 || info.State.LastSeq == 0 {
		return &domain.DLQListResult{Messages: []domain.DLQMessage{}}, nil
	}

	if startSeq == 0 || startSeq < info.State.FirstSeq {
		startSeq = info.State.FirstSeq
	}
	if startSeq > info.State.LastSeq {
		return &domain.DLQListResult{Messages: []domain.DLQMessage{}}, nil
	}

	rangeRes, err := c.GetMessageRange(ctx, stream, startSeq, info.State.LastSeq, limit)
	if err != nil {
		return nil, err
	}

	out := &domain.DLQListResult{
		Messages:  make([]domain.DLQMessage, 0, len(rangeRes.Messages)),
		Truncated: rangeRes.Truncated,
	}
	var lastSeq uint64
	for _, msg := range rangeRes.Messages {
		out.Messages = append(out.Messages, domain.DLQMessageFromStream(msg))
		lastSeq = msg.Seq
	}
	// Prefer Truncated from the range helper, but also advance when a full page
	// was returned before LastSeq (some range backends omit Truncated)
	moreRemain := lastSeq > 0 && lastSeq < info.State.LastSeq
	fullPage := len(rangeRes.Messages) >= limit
	if moreRemain && (rangeRes.Truncated || fullPage) {
		next := lastSeq + 1
		out.NextSeq = &next
		out.Truncated = true
	}
	return out, nil
}

func (c *Client) RetryDLQMessages(ctx context.Context, stream string, req domain.DLQRetryRequest) (*domain.DLQRetryResult, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	info, err := c.ensureDLQStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	var (
		remaining *int
		truncated bool
		sequences = req.Seqs
	)

	if req.All {
		sequences, truncated, remaining, err = c.collectDLQRetryAllSequences(ctx, stream, info, req.NormalizedLimit())
		if err != nil {
			return nil, err
		}
	}

	result := &domain.DLQRetryResult{
		Failed:    make([]domain.DLQRetryFailure, 0),
		Truncated: truncated,
		Remaining: remaining,
	}
	for _, seq := range sequences {
		if err := c.retryOneDLQMessage(ctx, stream, seq); err != nil {
			result.Failed = append(result.Failed, domain.DLQRetryFailure{
				Seq:   seq,
				Error: err.Error(),
			})
			continue
		}
		result.Retried++
	}
	return result, nil
}

func (c *Client) collectDLQRetryAllSequences(
	ctx context.Context,
	stream string,
	info *nats.StreamInfo,
	safetyMax int,
) (sequences []uint64, truncated bool, remaining *int, err error) {
	if safetyMax <= 0 {
		safetyMax = domain.DefaultDLQRetryAllLimit
	}
	capHint := safetyMax
	if messages := info.State.Msgs; messages > 0 && messages < uint64(capHint) { //nolint:gosec // G115: capHint is positive int bound
		capHint = int(messages) //nolint:gosec // G115: messages < uint64(capHint)
	}

	sequences = make([]uint64, 0, capHint)
	startSeq := uint64(0)

	for len(sequences) < safetyMax {
		pageLimit := domain.MaxDLQListLimit
		if left := safetyMax - len(sequences); left < pageLimit {
			pageLimit = left
		}
		listed, listErr := c.ListDLQMessages(ctx, stream, startSeq, pageLimit)
		if listErr != nil {
			return nil, false, nil, listErr
		}
		if len(listed.Messages) == 0 {
			break
		}
		for _, msg := range listed.Messages {
			sequences = append(sequences, msg.Seq)
		}
		if listed.NextSeq == nil {
			break
		}
		startSeq = *listed.NextSeq
		if len(sequences) >= safetyMax {
			if listed.NextSeq != nil || (len(listed.Messages) > 0 && listed.Messages[len(listed.Messages)-1].Seq < info.State.LastSeq) {
				truncated = true
				left := 0
				if messages := info.State.Msgs; messages <= uint64(^uint(0)>>1) { //nolint:gosec
					left = max(int(messages)-len(sequences), 0)
				}
				remaining = &left
			}
			break
		}
	}
	return sequences, truncated, remaining, nil
}

func (c *Client) retryOneDLQMessage(ctx context.Context, stream string, seq uint64) error {
	raw, err := c.GetMessage(ctx, stream, seq)
	if err != nil {
		return err
	}
	msg := domain.StreamMessageFromRaw(raw)
	dlqMsg := domain.DLQMessageFromStream(msg)
	original := strings.TrimSpace(dlqMsg.OriginalSubject)
	if commonstrings.IsEmpty(original) {
		return domain.ErrDLQMissingOriginalSub
	}

	data, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return fmt.Errorf("decode data: %w", err)
	}

	headers := domain.RetryPublishHeaders(msg.Headers, stream, seq)
	if _, err := c.natsCl.PublishRaw(ctx, original, data, headers); err != nil {
		return fmt.Errorf("republish subject=%q: %w", original, err)
	}
	if err := c.DeleteMessage(ctx, stream, seq); err != nil {
		return fmt.Errorf("%w: seq=%d: %w", domain.ErrDLQRepublishedNotDeleted, seq, err)
	}
	return nil
}
