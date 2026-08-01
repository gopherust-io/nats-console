package domain

import (
	"errors"
	"fmt"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"strings"
	"time"
)

const (
	ReplayModeReset   = "reset"
	ReplayModeSidecar = "sidecar"

	ReplayFromSeq       = "seq"
	ReplayFromTime      = "time"
	ReplayFromBeginning = "beginning"
	ReplayFromNew       = "new"

	ReplayPolicyInstant  = "instant"
	ReplayPolicyOriginal = "original"

	DefaultMsgRangeMax = 1000
)

// ReplayConsumerRequest repositions or side-cars a durable consumer for stream replay.
type ReplayConsumerRequest struct {
	Mode          string `json:"mode"`
	From          string `json:"from"`
	Time          string `json:"time,omitempty"`      // RFC3339 start
	UntilTime     string `json:"untilTime,omitempty"` // RFC3339 end (inclusive)
	ReplayPolicy  string `json:"replayPolicy,omitempty"`
	FilterSubject string `json:"filterSubject,omitempty"`
	Durable       string `json:"durable,omitempty"` // sidecar target name
	Seq           uint64 `json:"seq,omitempty"`
	UntilSeq      uint64 `json:"untilSeq,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

// ReplayConsumerResult is returned after a successful replay operation.
type ReplayConsumerResult struct {
	StartTime *string `json:"startTime,omitempty"`
	UntilTime *string `json:"untilTime,omitempty"`
	Durable   string  `json:"durable"`
	Mode      string  `json:"mode"`
	StartSeq  uint64  `json:"startSeq,omitempty"`
	UntilSeq  uint64  `json:"untilSeq,omitempty"`
	Limit     int     `json:"limit,omitempty"`
}

// MessageRangeResult is a batch of stored stream messages.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type MessageRangeResult struct {
	Messages  []StreamMessage `json:"messages"`
	Truncated bool            `json:"truncated,omitempty"`
}

func (r ReplayConsumerRequest) Validate() error {
	mode := strings.ToLower(strings.TrimSpace(r.Mode))
	if commonstrings.IsEmpty(mode) {
		mode = ReplayModeReset
	}
	switch mode {
	case ReplayModeReset, ReplayModeSidecar:
	default:
		return fmt.Errorf("mode must be %q or %q", ReplayModeReset, ReplayModeSidecar)
	}

	from := strings.ToLower(strings.TrimSpace(r.From))
	if commonstrings.IsEmpty(from) {
		return errors.New("from is required")
	}
	switch from {
	case ReplayFromSeq:
		if r.Seq == 0 {
			return fmt.Errorf("seq is required when from=%q", ReplayFromSeq)
		}
	case ReplayFromTime:
		if commonstrings.IsEmpty(strings.TrimSpace(r.Time)) {
			return fmt.Errorf("time is required when from=%q", ReplayFromTime)
		}
		if _, err := parseRFC3339(r.Time); err != nil {
			return fmt.Errorf("time must be RFC3339: %w", err)
		}
	case ReplayFromBeginning, ReplayFromNew:
	default:
		return fmt.Errorf("from must be one of %q, %q, %q, %q",
			ReplayFromSeq, ReplayFromTime, ReplayFromBeginning, ReplayFromNew)
	}

	if !commonstrings.IsEmpty(strings.TrimSpace(r.UntilTime)) {
		if _, err := parseRFC3339(r.UntilTime); err != nil {
			return fmt.Errorf("untilTime must be RFC3339: %w", err)
		}
	}

	if r.Limit < 0 {
		return errors.New("limit must be >= 0")
	}

	if from == ReplayFromNew && (r.UntilSeq > 0 || !commonstrings.IsEmpty(strings.TrimSpace(r.UntilTime)) || r.Limit > 0) {
		return errors.New("untilSeq, untilTime, and limit are not valid when from=new")
	}

	if from == ReplayFromSeq && r.UntilSeq > 0 && r.UntilSeq < r.Seq {
		return fmt.Errorf("untilSeq %d < seq %d", r.UntilSeq, r.Seq)
	}

	if from == ReplayFromTime && !commonstrings.IsEmpty(strings.TrimSpace(r.UntilTime)) {
		start, err := parseRFC3339(r.Time)
		if err != nil {
			return err
		}
		end, err := parseRFC3339(r.UntilTime)
		if err != nil {
			return err
		}
		if end.Before(start) {
			return errors.New("untilTime before time")
		}
	}

	policy := strings.ToLower(strings.TrimSpace(r.ReplayPolicy))
	if !commonstrings.IsEmpty(policy) && policy != ReplayPolicyInstant && policy != ReplayPolicyOriginal {
		return fmt.Errorf("replayPolicy must be %q or %q", ReplayPolicyInstant, ReplayPolicyOriginal)
	}
	return nil
}

func (r ReplayConsumerRequest) NormalizedMode() string {
	mode := strings.ToLower(strings.TrimSpace(r.Mode))
	if commonstrings.IsEmpty(mode) {
		return ReplayModeReset
	}
	return mode
}

func (r ReplayConsumerRequest) NormalizedFrom() string {
	return strings.ToLower(strings.TrimSpace(r.From))
}

func (r ReplayConsumerRequest) ParseTime() (time.Time, error) {
	return parseRFC3339(r.Time)
}

func (r ReplayConsumerRequest) ParseUntilTime() (time.Time, error) {
	return parseRFC3339(r.UntilTime)
}

func parseRFC3339(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
