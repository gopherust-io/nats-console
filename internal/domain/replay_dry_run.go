package domain

import (
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"strings"
)

// DefaultReplayEstimateRate is the assumed instant-replay throughput (msgs/s)
// used when estimating duration. Tunable later if live rates are available.
const DefaultReplayEstimateRate = 1000

// ReplayDryRun is a preview of replay impact without mutating JetStream.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type ReplayDryRun struct {
	PotentialDuplicates []string `json:"potentialDuplicates"`
	Messages            uint64   `json:"messages"`
	EstimatedDurationMs int64    `json:"estimatedDurationMs"`
	ConsumersAffected   int      `json:"consumersAffected"`
	Unbounded           bool     `json:"unbounded,omitempty"`
	Approximate         bool     `json:"approximate,omitempty"`
}

// ComputeReplayDryRun estimates message count, duration, and duplicate risk
// for a replay request against the current stream/consumer snapshot.
func ComputeReplayDryRun(req ReplayConsumerRequest, stream StreamInfo, target ConsumerInfo) (ReplayDryRun, error) {
	if err := req.Validate(); err != nil {
		return ReplayDryRun{}, err
	}

	out := ReplayDryRun{
		ConsumersAffected:   1,
		PotentialDuplicates: []string{},
	}
	if id := ServiceID(target); !commonstrings.IsEmpty(id) {
		out.PotentialDuplicates = []string{id}
	}

	from := req.NormalizedFrom()
	policy := strings.ToLower(strings.TrimSpace(req.ReplayPolicy))
	if commonstrings.IsEmpty(policy) {
		policy = ReplayPolicyInstant
	}

	firstSeq := stream.State.FirstSeq
	lastSeq := stream.State.LastSeq
	if firstSeq == 0 && lastSeq > 0 {
		firstSeq = 1
	}

	switch {
	case req.Limit > 0:
		out.Messages = uint64(req.Limit)
	case from == ReplayFromNew:
		out.Messages = 0
		out.Unbounded = true
	case from == ReplayFromTime:
		out.Approximate = true
		if commonstrings.IsEmpty(strings.TrimSpace(req.UntilTime)) && req.UntilSeq == 0 {
			out.Unbounded = true
		}
	default:
		startSeq := resolveReplayStartSeq(from, req.Seq, firstSeq)
		if req.UntilSeq > 0 {
			end := req.UntilSeq
			if lastSeq > 0 && end > lastSeq {
				end = lastSeq
			}
			if startSeq > 0 && end >= startSeq {
				out.Messages = end - startSeq + 1
			}
		} else {
			out.Unbounded = true
			if startSeq > 0 && lastSeq >= startSeq {
				out.Messages = lastSeq - startSeq + 1
			}
		}
	}

	if policy == ReplayPolicyOriginal {
		if d := originalReplayDurationMs(req); d > 0 {
			out.EstimatedDurationMs = d
			return out, nil
		}
	}
	out.EstimatedDurationMs = EstimateReplayDurationMs(out.Messages, DefaultReplayEstimateRate)
	return out, nil
}

func resolveReplayStartSeq(from string, seq, firstSeq uint64) uint64 {
	switch from {
	case ReplayFromBeginning:
		if firstSeq == 0 {
			return 1
		}
		return firstSeq
	case ReplayFromSeq:
		return seq
	default:
		return 0
	}
}

func originalReplayDurationMs(req ReplayConsumerRequest) int64 {
	if commonstrings.IsEmpty(strings.TrimSpace(req.Time)) || commonstrings.IsEmpty(strings.TrimSpace(req.UntilTime)) {
		return 0
	}
	start, err := req.ParseTime()
	if err != nil {
		return 0
	}
	end, err := req.ParseUntilTime()
	if err != nil {
		return 0
	}
	ms := end.Sub(start).Milliseconds()
	if ms < 0 {
		return 0
	}
	return ms
}

// EstimateReplayDurationMs returns ceil(messages/rate) seconds as milliseconds.
func EstimateReplayDurationMs(messages uint64, ratePerSec int) int64 {
	if messages == 0 {
		return 0
	}
	if ratePerSec <= 0 {
		ratePerSec = DefaultReplayEstimateRate
	}
	secs := (messages + uint64(ratePerSec) - 1) / uint64(ratePerSec)
	if secs > uint64(^uint64(0)>>1)/1000 {
		return int64(^uint64(0) >> 1)
	}
	return int64(secs) * 1000 //nolint:gosec // G115: secs clamped above
}
