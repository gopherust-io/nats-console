package domain

import (
	"errors"
	"fmt"
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
)

// ReplayConsumerRequest repositions or side-cars a durable consumer for stream replay.
type ReplayConsumerRequest struct {
	Mode          string `json:"mode"`
	From          string `json:"from"`
	Time          string `json:"time,omitempty"` // RFC3339
	ReplayPolicy  string `json:"replayPolicy,omitempty"`
	FilterSubject string `json:"filterSubject,omitempty"`
	Durable       string `json:"durable,omitempty"` // sidecar target name
	Seq           uint64 `json:"seq,omitempty"`
}

// ReplayConsumerResult is returned after a successful replay operation.
type ReplayConsumerResult struct {
	Durable string `json:"durable"`
	Mode    string `json:"mode"`
}

func (r ReplayConsumerRequest) Validate() error {
	mode := strings.ToLower(strings.TrimSpace(r.Mode))
	if mode == "" {
		mode = ReplayModeReset
	}
	switch mode {
	case ReplayModeReset, ReplayModeSidecar:
	default:
		return fmt.Errorf("mode must be %q or %q", ReplayModeReset, ReplayModeSidecar)
	}

	from := strings.ToLower(strings.TrimSpace(r.From))
	if from == "" {
		return errors.New("from is required")
	}
	switch from {
	case ReplayFromSeq:
		if r.Seq == 0 {
			return fmt.Errorf("seq is required when from=%q", ReplayFromSeq)
		}
	case ReplayFromTime:
		if strings.TrimSpace(r.Time) == "" {
			return fmt.Errorf("time is required when from=%q", ReplayFromTime)
		}
		if _, err := time.Parse(time.RFC3339, r.Time); err != nil {
			if _, err2 := time.Parse(time.RFC3339Nano, r.Time); err2 != nil {
				return fmt.Errorf("time must be RFC3339: %w", err)
			}
		}
	case ReplayFromBeginning, ReplayFromNew:
	default:
		return fmt.Errorf("from must be one of %q, %q, %q, %q",
			ReplayFromSeq, ReplayFromTime, ReplayFromBeginning, ReplayFromNew)
	}

	policy := strings.ToLower(strings.TrimSpace(r.ReplayPolicy))
	if policy != "" && policy != ReplayPolicyInstant && policy != ReplayPolicyOriginal {
		return fmt.Errorf("replayPolicy must be %q or %q", ReplayPolicyInstant, ReplayPolicyOriginal)
	}
	return nil
}

func (r ReplayConsumerRequest) NormalizedMode() string {
	mode := strings.ToLower(strings.TrimSpace(r.Mode))
	if mode == "" {
		return ReplayModeReset
	}
	return mode
}

func (r ReplayConsumerRequest) NormalizedFrom() string {
	return strings.ToLower(strings.TrimSpace(r.From))
}

func (r ReplayConsumerRequest) ParseTime() (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, r.Time); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, r.Time)
}
