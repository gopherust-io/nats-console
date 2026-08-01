package domain

import (
	"fmt"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"sort"
	"strings"
	"time"
)

const (
	IncidentEventDeploy            = "deploy"
	IncidentEventChange            = "change"
	IncidentEventLagGrowth         = "lag_growth"
	IncidentEventRedeliverySpike   = "redelivery_spike"
	IncidentEventNodeDisconnect    = "node_disconnect"
	IncidentEventProcessingStopped = "processing_stopped"

	IncidentAnnotationTypeDeploy = "deploy"

	IncidentNodeDisconnect = "disconnect"
	IncidentNodeReconnect  = "reconnect"

	// Detection thresholds for metric-derived timeline events.
	IncidentLagGrowthMinAbsolute    = 50.0
	IncidentLagGrowthMinRatio       = 0.25
	IncidentRedeliverySpikeMinDelta = 5.0
	IncidentProcessingFlatSamples   = 2
)

// IncidentAnnotation is a CI/manual change marker (typically a deploy).
type IncidentAnnotation struct {
	OccurredAt time.Time `json:"occurredAt"`
	ID         string    `json:"id,omitempty"`
	ClusterID  string    `json:"clusterId,omitempty"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Details    string    `json:"details,omitempty"`
}

// IncidentAnnotationCreate is the request body for POST …/incident-annotations.
type IncidentAnnotationCreate struct {
	OccurredAt *time.Time `json:"occurredAt,omitempty"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Details    string     `json:"details,omitempty"`
}

func (c IncidentAnnotationCreate) Validate() error {
	typ := strings.TrimSpace(c.Type)
	if commonstrings.IsEmpty(typ) {
		typ = IncidentAnnotationTypeDeploy
	}
	if typ != IncidentAnnotationTypeDeploy {
		return fmt.Errorf("%w: unsupported annotation type %q", ErrInvalidInput, c.Type)
	}
	if commonstrings.IsEmpty(strings.TrimSpace(c.Title)) {
		return fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	return nil
}

func (c IncidentAnnotationCreate) NormalizedType() string {
	typ := strings.TrimSpace(c.Type)
	if commonstrings.IsEmpty(typ) {
		return IncidentAnnotationTypeDeploy
	}
	return typ
}

// IncidentConsumerSample is a per-consumer scrape used for lag/redelivery/stop detection.
type IncidentConsumerSample struct {
	CapturedAt     time.Time
	StreamName     string
	ConsumerName   string
	Lag            float64
	NumRedelivered float64
	DeliveredSeq   float64
	AckFloorSeq    float64
}

// IncidentNodeEvent is a named route/node membership transition.
type IncidentNodeEvent struct {
	OccurredAt time.Time
	NodeName   string
	EventType  string
}

// IncidentAuditChange is a consol mutation used as a Deploy fallback.
type IncidentAuditChange struct {
	Timestamp    time.Time
	Action       string
	ResourceType string
	ResourceName string
	Actor        string
}

// IncidentTimelineEvent is one ordered reconstruction marker.
type IncidentTimelineEvent struct {
	At       time.Time `json:"at"`
	Category string    `json:"category"`
	Label    string    `json:"label"`
	Source   string    `json:"source"`
	Evidence string    `json:"evidence,omitempty"`
}

// IncidentReconstruction is the auto-generated timeline for a consumer window.
type IncidentReconstruction struct {
	ClusterID  string                  `json:"clusterId"`
	Stream     string                  `json:"stream"`
	Consumer   string                  `json:"consumer"`
	From       time.Time               `json:"from"`
	To         time.Time               `json:"to"`
	Events     []IncidentTimelineEvent `json:"events"`
	EventCount int                     `json:"eventCount"`
	UsedDeploy bool                    `json:"usedDeployAnnotations"`
	UsedAudit  bool                    `json:"usedAuditFallback"`
}

// IncidentReconstructionInput feeds ComputeIncidentTimeline.
type IncidentReconstructionInput struct {
	ClusterID   string
	Stream      string
	Consumer    string
	From        time.Time
	To          time.Time
	Annotations []IncidentAnnotation
	Samples     []IncidentConsumerSample
	NodeEvents  []IncidentNodeEvent
	Audit       []IncidentAuditChange
}

// ComputeIncidentTimeline merges annotations (or audit fallback), metric
// inflection points, and named disconnects into a de-duplicated, time-ordered list.
func ComputeIncidentTimeline(in IncidentReconstructionInput) IncidentReconstruction {
	out := IncidentReconstruction{
		ClusterID: in.ClusterID,
		Stream:    in.Stream,
		Consumer:  in.Consumer,
		From:      in.From.UTC(),
		To:        in.To.UTC(),
		Events:    []IncidentTimelineEvent{},
	}

	deployAnns := filterDeployAnnotations(in.Annotations, in.From, in.To)
	if len(deployAnns) > 0 {
		out.UsedDeploy = true
		for _, a := range deployAnns {
			label := strings.TrimSpace(a.Title)
			if commonstrings.IsEmpty(label) {
				label = "Deploy"
			}
			out.Events = append(out.Events, IncidentTimelineEvent{
				At:       a.OccurredAt.UTC(),
				Category: IncidentEventDeploy,
				Label:    label,
				Source:   "annotation",
				Evidence: strings.TrimSpace(a.Details),
			})
		}
	} else {
		for _, a := range filterAuditInWindow(in.Audit, in.From, in.To) {
			out.UsedAudit = true
			label := formatAuditChangeLabel(a)
			out.Events = append(out.Events, IncidentTimelineEvent{
				At:       a.Timestamp.UTC(),
				Category: IncidentEventChange,
				Label:    label,
				Source:   "audit",
				Evidence: strings.TrimSpace(a.Actor),
			})
		}
	}

	samples := filterConsumerSamples(in.Samples, in.Stream, in.Consumer, in.From, in.To)
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].CapturedAt.Before(samples[j].CapturedAt)
	})
	out.Events = append(out.Events, detectLagGrowth(samples)...)
	out.Events = append(out.Events, detectRedeliverySpike(samples)...)
	out.Events = append(out.Events, detectProcessingStopped(samples)...)

	for _, n := range in.NodeEvents {
		if n.EventType != IncidentNodeDisconnect {
			continue
		}
		if n.OccurredAt.Before(in.From) || n.OccurredAt.After(in.To) {
			continue
		}
		name := strings.TrimSpace(n.NodeName)
		if commonstrings.IsEmpty(name) {
			name = "Node"
		}
		out.Events = append(out.Events, IncidentTimelineEvent{
			At:       n.OccurredAt.UTC(),
			Category: IncidentEventNodeDisconnect,
			Label:    name + " disconnects",
			Source:   "routez",
			Evidence: "node membership lost",
		})
	}

	out.Events = dedupeTimelineEvents(out.Events)
	sort.SliceStable(out.Events, func(i, j int) bool {
		if out.Events[i].At.Equal(out.Events[j].At) {
			return out.Events[i].Category < out.Events[j].Category
		}
		return out.Events[i].At.Before(out.Events[j].At)
	})
	out.EventCount = len(out.Events)
	return out
}

func filterDeployAnnotations(anns []IncidentAnnotation, from, to time.Time) []IncidentAnnotation {
	out := make([]IncidentAnnotation, 0, len(anns))
	for _, a := range anns {
		typ := strings.TrimSpace(a.Type)
		if commonstrings.IsEmpty(typ) {
			typ = IncidentAnnotationTypeDeploy
		}
		if typ != IncidentAnnotationTypeDeploy {
			continue
		}
		at := a.OccurredAt.UTC()
		if at.Before(from.UTC()) || at.After(to.UTC()) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func filterAuditInWindow(items []IncidentAuditChange, from, to time.Time) []IncidentAuditChange {
	out := make([]IncidentAuditChange, 0, len(items))
	for _, a := range items {
		at := a.Timestamp.UTC()
		if at.Before(from.UTC()) || at.After(to.UTC()) {
			continue
		}
		if commonstrings.IsEmpty(strings.TrimSpace(a.Action)) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func filterConsumerSamples(samples []IncidentConsumerSample, stream, consumer string, from, to time.Time) []IncidentConsumerSample {
	stream = strings.TrimSpace(stream)
	consumer = strings.TrimSpace(consumer)
	out := make([]IncidentConsumerSample, 0, len(samples))
	for _, s := range samples {
		if !commonstrings.IsEmpty(stream) && s.StreamName != stream {
			continue
		}
		if !commonstrings.IsEmpty(consumer) && s.ConsumerName != consumer {
			continue
		}
		at := s.CapturedAt.UTC()
		if at.Before(from.UTC()) || at.After(to.UTC()) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func formatAuditChangeLabel(a IncidentAuditChange) string {
	parts := make([]string, 0, 3)
	if !commonstrings.IsEmpty(a.Action) {
		parts = append(parts, a.Action)
	}
	if !commonstrings.IsEmpty(a.ResourceType) || !commonstrings.IsEmpty(a.ResourceName) {
		res := strings.TrimSpace(a.ResourceType + " " + a.ResourceName)
		parts = append(parts, strings.TrimSpace(res))
	}
	if len(parts) == 0 {
		return "Change"
	}
	return strings.Join(parts, " ")
}

func detectLagGrowth(samples []IncidentConsumerSample) []IncidentTimelineEvent {
	var out []IncidentTimelineEvent
	for i := 1; i < len(samples); i++ {
		prev, cur := samples[i-1], samples[i]
		delta := cur.Lag - prev.Lag
		if delta < IncidentLagGrowthMinAbsolute {
			continue
		}
		ratioOK := prev.Lag <= 0 || delta/prev.Lag >= IncidentLagGrowthMinRatio
		if !ratioOK && delta < IncidentLagGrowthMinAbsolute*2 {
			continue
		}
		out = append(out, IncidentTimelineEvent{
			At:       cur.CapturedAt.UTC(),
			Category: IncidentEventLagGrowth,
			Label:    "Consumer lag grows",
			Source:   "consumer_sample",
			Evidence: fmt.Sprintf("lag %.0f → %.0f", prev.Lag, cur.Lag),
		})
		// One lag-growth event per contiguous growth episode: skip until lag drops or flattens.
		for i+1 < len(samples) && samples[i+1].Lag >= samples[i].Lag {
			i++
		}
	}
	return out
}

func detectRedeliverySpike(samples []IncidentConsumerSample) []IncidentTimelineEvent {
	var out []IncidentTimelineEvent
	for i := 1; i < len(samples); i++ {
		prev, cur := samples[i-1], samples[i]
		delta := cur.NumRedelivered - prev.NumRedelivered
		if delta < IncidentRedeliverySpikeMinDelta {
			continue
		}
		out = append(out, IncidentTimelineEvent{
			At:       cur.CapturedAt.UTC(),
			Category: IncidentEventRedeliverySpike,
			Label:    "Redeliveries spike",
			Source:   "consumer_sample",
			Evidence: fmt.Sprintf("redelivered +%.0f (%.0f → %.0f)", delta, prev.NumRedelivered, cur.NumRedelivered),
		})
		for i+1 < len(samples) && samples[i+1].NumRedelivered >= samples[i].NumRedelivered {
			i++
		}
	}
	return out
}

func detectProcessingStopped(samples []IncidentConsumerSample) []IncidentTimelineEvent {
	if len(samples) < IncidentProcessingFlatSamples+1 {
		return nil
	}
	var out []IncidentTimelineEvent
	flat := 0
	for i := 1; i < len(samples); i++ {
		prev, cur := samples[i-1], samples[i]
		sameDelivered := cur.DeliveredSeq == prev.DeliveredSeq
		sameAck := cur.AckFloorSeq == prev.AckFloorSeq
		if sameDelivered && sameAck && cur.Lag > 0 {
			flat++
			if flat == IncidentProcessingFlatSamples {
				out = append(out, IncidentTimelineEvent{
					At:       cur.CapturedAt.UTC(),
					Category: IncidentEventProcessingStopped,
					Label:    "Processing stops",
					Source:   "consumer_sample",
					Evidence: fmt.Sprintf("delivered/ack flat at %.0f/%.0f with lag %.0f", cur.DeliveredSeq, cur.AckFloorSeq, cur.Lag),
				})
			}
			continue
		}
		flat = 0
	}
	return out
}

func dedupeTimelineEvents(events []IncidentTimelineEvent) []IncidentTimelineEvent {
	if len(events) == 0 {
		return events
	}
	type key struct {
		cat string
		lbl string
		sec int64
	}
	seen := make(map[key]struct{}, len(events))
	out := make([]IncidentTimelineEvent, 0, len(events))
	for _, e := range events {
		k := key{cat: e.Category, sec: e.At.UTC().Unix(), lbl: e.Label}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, e)
	}
	return out
}
