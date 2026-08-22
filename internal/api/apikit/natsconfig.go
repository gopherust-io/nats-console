package apikit

import (
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Request fragments shared by the stream, consumer, KV, and object-store
// config bodies. They live here because both the jetstream and kvobj packages
// accept them.

type StreamPlacementRequest struct {
	Cluster string   `json:"cluster,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type StreamExternalRequest struct {
	APIPrefix     string `json:"api,omitempty"`
	DeliverPrefix string `json:"deliver,omitempty"`
}

type StreamSourceRequest struct {
	External      *StreamExternalRequest `json:"external,omitempty"`
	Name          string                 `json:"name"`
	FilterSubject string                 `json:"filterSubject,omitempty"`
	OptStartTime  string                 `json:"optStartTime,omitempty"`
	OptStartSeq   uint64                 `json:"optStartSeq,omitempty"`
}

type SubjectTransformRequest struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
}

// goalign:ignore
type RePublishRequest struct {
	Source      string `json:"src,omitempty"`
	Destination string `json:"dest"`
	HeadersOnly bool   `json:"headersOnly,omitempty"`
}

type ConsumerLimitsRequest struct {
	InactiveThreshold int64 `json:"inactiveThreshold,omitempty"` // nanoseconds
	MaxAckPending     int   `json:"maxAckPending,omitempty"`
}

func (r StreamSourceRequest) ToNATS() (*nats.StreamSource, error) {
	src := &nats.StreamSource{
		Name:          r.Name,
		FilterSubject: r.FilterSubject,
		OptStartSeq:   r.OptStartSeq,
	}
	if !commonstrings.IsEmpty(strings.TrimSpace(r.OptStartTime)) {
		t, err := time.Parse(time.RFC3339Nano, r.OptStartTime)
		if err != nil {
			t, err = time.Parse(time.RFC3339, r.OptStartTime)
			if err != nil {
				return nil, fmt.Errorf("optStartTime: %w", err)
			}
		}
		src.OptStartTime = &t
	}
	if r.External != nil && (!commonstrings.IsEmpty(r.External.APIPrefix) || !commonstrings.IsEmpty(r.External.DeliverPrefix)) {
		src.External = &nats.ExternalStream{
			APIPrefix:     r.External.APIPrefix,
			DeliverPrefix: r.External.DeliverPrefix,
		}
	}
	return src, nil
}

// ToNATSPlacement returns the nats placement for a request fragment, or nil
// when neither a cluster nor tags were supplied.
func (r *StreamPlacementRequest) ToNATSPlacement() *nats.Placement {
	if r == nil || (commonstrings.IsEmpty(r.Cluster) && len(r.Tags) == 0) {
		return nil
	}
	return &nats.Placement{
		Cluster: r.Cluster,
		Tags:    append([]string(nil), r.Tags...),
	}
}

// ToNATSRePublish returns the nats republish config, or nil when no destination
// was supplied.
func (r *RePublishRequest) ToNATSRePublish() *nats.RePublish {
	if r == nil || commonstrings.IsEmpty(r.Destination) {
		return nil
	}
	return &nats.RePublish{
		Source:      r.Source,
		Destination: r.Destination,
		HeadersOnly: r.HeadersOnly,
	}
}

// SourcesToNATS converts a source list, skipping blank names.
func SourcesToNATS(sources []StreamSourceRequest) ([]*nats.StreamSource, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	out := make([]*nats.StreamSource, 0, len(sources))
	for i, srcReq := range sources {
		if commonstrings.IsEmpty(strings.TrimSpace(srcReq.Name)) {
			continue
		}
		src, err := srcReq.ToNATS()
		if err != nil {
			return nil, fmt.Errorf("sources[%d]: %w", i, err)
		}
		out = append(out, src)
	}
	return out, nil
}

func CloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

// UnmarshalEnum decodes a bare enum string into a nats.go enum type, which only
// implements JSON unmarshalling.
func UnmarshalEnum[T any](value string, target *T) error {
	return serializer.Unmarshal(commonstrings.StringToBytes(`"`+value+`"`), target)
}
