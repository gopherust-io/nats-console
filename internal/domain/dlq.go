package domain

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats/dlq"
)

const (
	// MetadataRoleKey marks a stream as a dead-letter queue in stream metadata.
	MetadataRoleKey = "nats-consol/role"
	// MetadataRoleDLQ is the metadata value for DLQ streams.
	MetadataRoleDLQ = "dlq"

	// DLQNameSuffix is the conventional stream name suffix (e.g. ORDERS_DLQ).
	DLQNameSuffix = "_DLQ"

	// DefaultDLQListLimit is the default page size for listing DLQ messages.
	DefaultDLQListLimit = 100
	// MaxDLQListLimit caps a single DLQ list request.
	MaxDLQListLimit = 500
	// DefaultDLQRetryAllLimit is the default safety max for retry-all (paged).
	DefaultDLQRetryAllLimit = 10_000
	// MaxDLQRetryAllLimit caps retry-all in one API call (paged until exhausted or this max).
	MaxDLQRetryAllLimit = 10_000

	headerTraceID     = "Trace-Id"
	headerContentType = "Nats-Content-Type"
	headerMsgID       = "Nats-Msg-Id"
)

var (
	ErrNotDLQStream             = errors.New("stream is not a dead letter queue")
	ErrDLQMissingOriginalSub    = errors.New("DLQ message missing X-NATS-Original-Subject")
	ErrDLQRetryEmpty            = errors.New("provide seqs or set all=true")
	ErrDLQRepublishedNotDeleted = errors.New("republished but failed to delete from DLQ")
)

// DLQMessage is a stored DLQ message with parsed poison metadata.
type DLQMessage struct {
	Headers         map[string]string `json:"headers,omitempty"`
	Subject         string            `json:"subject"`
	Time            string            `json:"time"`
	Data            string            `json:"data"`
	Reason          string            `json:"reason,omitempty"`
	OriginalSubject string            `json:"originalSubject,omitempty"`
	SourceStream    string            `json:"sourceStream,omitempty"`
	Consumer        string            `json:"consumer,omitempty"`
	AutopsyError    string            `json:"autopsyError,omitempty"`
	AutopsyHash     string            `json:"autopsyHash,omitempty"`
	AutopsyStack    string            `json:"autopsyStack,omitempty"`
	Seq             uint64            `json:"seq"`
	SourceSeq       uint64            `json:"sourceSeq,omitempty"`
	NumDelivered    uint64            `json:"numDelivered,omitempty"`
}

// DLQListResult is a page of DLQ messages.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type DLQListResult struct {
	NextSeq   *uint64      `json:"nextSeq,omitempty"`
	Messages  []DLQMessage `json:"messages"`
	Truncated bool         `json:"truncated,omitempty"`
}

// DLQRetryRequest selects messages to republish to their original subjects.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type DLQRetryRequest struct {
	Seqs  []uint64 `json:"seqs,omitempty"`
	Limit int      `json:"limit,omitempty"`
	All   bool     `json:"all,omitempty"`
}

// DLQRetryFailure records a per-sequence retry error.
type DLQRetryFailure struct {
	Error string `json:"error"`
	Seq   uint64 `json:"seq"`
}

// DLQRetryResult summarizes a retry batch.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type DLQRetryResult struct {
	Remaining *int              `json:"remaining,omitempty"`
	Failed    []DLQRetryFailure `json:"failed,omitempty"`
	Retried   int               `json:"retried"`
	Truncated bool              `json:"truncated,omitempty"`
}

// IsDLQStream reports whether a stream should be treated as a dead-letter queue.
func IsDLQStream(name string, metadata map[string]string) bool {
	if strings.HasSuffix(name, DLQNameSuffix) {
		return true
	}
	if metadata != nil && metadata[MetadataRoleKey] == MetadataRoleDLQ {
		return true
	}
	return false
}

// NormalizeDLQListLimit clamps a DLQ list page size.
func NormalizeDLQListLimit(limit int) int {
	if limit <= 0 {
		return DefaultDLQListLimit
	}
	if limit > MaxDLQListLimit {
		return MaxDLQListLimit
	}
	return limit
}

// Validate checks a DLQ retry request.
func (r DLQRetryRequest) Validate() error {
	if r.All {
		return nil
	}
	if len(r.Seqs) == 0 {
		return ErrDLQRetryEmpty
	}
	if slices.Contains(r.Seqs, 0) {
		return errors.New("seqs must be positive")
	}
	return nil
}

// NormalizedLimit returns the retry-all batch limit.
func (r DLQRetryRequest) NormalizedLimit() int {
	if r.Limit <= 0 {
		return DefaultDLQRetryAllLimit
	}
	if r.Limit > MaxDLQRetryAllLimit {
		return MaxDLQRetryAllLimit
	}
	return r.Limit
}

// DLQMessageFromStream maps a stored stream message into DLQ shape.
func DLQMessageFromStream(msg StreamMessage) DLQMessage {
	out := DLQMessage{
		Seq:     msg.Seq,
		Subject: msg.Subject,
		Time:    msg.Time,
		Data:    msg.Data,
		Headers: msg.Headers,
	}
	if msg.Headers == nil {
		return out
	}
	out.Reason = msg.Headers[dlq.HeaderReason]
	out.OriginalSubject = msg.Headers[dlq.HeaderOriginalSubject]
	out.SourceStream = msg.Headers[dlq.HeaderStream]
	out.Consumer = msg.Headers[dlq.HeaderConsumer]
	out.AutopsyError = msg.Headers[dlq.HeaderAutopsyError]
	out.AutopsyHash = msg.Headers[dlq.HeaderAutopsyHash]
	out.AutopsyStack = msg.Headers[dlq.HeaderAutopsyStack]
	if v := msg.Headers[dlq.HeaderSequence]; !commonstrings.IsEmpty(v) {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			out.SourceSeq = n
		}
	}
	if v := msg.Headers[dlq.HeaderNumDelivered]; !commonstrings.IsEmpty(v) {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			out.NumDelivered = n
		}
	}
	return out
}

// RetryPublishHeaders returns headers safe to attach when republishing from the DLQ.
// DLQ/autopsy headers are stripped. Trace-Id and content-type are kept.
// A stable Nats-Msg-Id derived from the DLQ stream+seq is set so a retry after
// "republished but not deleted" is deduplicated by JetStream duplicate windows.
func RetryPublishHeaders(headers map[string]string, dlqStream string, dlqSeq uint64) map[string]string {
	out := make(map[string]string)
	if tid := headers[headerTraceID]; !commonstrings.IsEmpty(tid) {
		out[headerTraceID] = tid
	}
	if ct := headers[headerContentType]; !commonstrings.IsEmpty(ct) {
		out[headerContentType] = ct
	}
	out[headerMsgID] = fmt.Sprintf("nats-consol-dlq-retry:%s:%d", dlqStream, dlqSeq)
	return out
}

// DisplayError prefers autopsy error text, falling back to DLQ reason.
func (m DLQMessage) DisplayError() string {
	if !commonstrings.IsEmpty(m.AutopsyError) {
		return m.AutopsyError
	}
	return m.Reason
}
