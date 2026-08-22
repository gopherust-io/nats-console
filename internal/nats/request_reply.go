package natsclient

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// RequestReplyConnzPath scrapes connz with subscription detail so queue groups
// (qgroup) are available. NATS keeps the subscription *count* in
// "subscriptions" and puts subjects in subscriptions_list(_detail).
const RequestReplyConnzPath = "/connz?limit=1024&subs=detail"

type connzPayload struct {
	Connections []connzConnection `json:"connections"`
}

type connzConnection struct {
	Name                    string           `json:"name"`
	Account                 string           `json:"account"`
	Rtt                     string           `json:"rtt"`
	SubscriptionsRaw        json.RawMessage  `json:"subscriptions"`
	SubscriptionsList       []string         `json:"subscriptions_list"`
	SubscriptionsListDetail []connzSubDetail `json:"subscriptions_list_detail"`
	CID                     int              `json:"cid"`
	PendingBytes            int              `json:"pending_bytes"`
}

type connzSubDetail struct {
	Subject string `json:"subject"`
	Queue   string `json:"queue"`
	QGroup  string `json:"qgroup"`
}

type connzSubscription struct {
	Subject string `json:"subject"`
	Queue   string `json:"queue"`
}

// subscriptions resolves the several shapes NATS /connz has used over time:
//   - legacy: "subscriptions": [{subject, queue}]
//   - subs=1: "subscriptions": <count>, "subscriptions_list": ["subj", ...]
//   - subs=detail: "subscriptions": <count>, "subscriptions_list_detail": [{subject, qgroup}]
func (c connzConnection) subscriptions() []connzSubscription {
	if len(c.SubscriptionsListDetail) > 0 {
		out := make([]connzSubscription, 0, len(c.SubscriptionsListDetail))
		for _, d := range c.SubscriptionsListDetail {
			queue := d.QGroup
			if commonstrings.IsEmpty(queue) {
				queue = d.Queue
			}
			out = append(out, connzSubscription{Subject: d.Subject, Queue: queue})
		}
		return out
	}
	if len(c.SubscriptionsList) > 0 {
		out := make([]connzSubscription, 0, len(c.SubscriptionsList))
		for _, subject := range c.SubscriptionsList {
			out = append(out, connzSubscription{Subject: subject})
		}
		return out
	}
	raw := c.SubscriptionsRaw
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	// Count-only form ("subscriptions": 2) — ignore; lists above carry subjects.
	if raw[0] != '[' {
		return nil
	}
	var legacy []connzSubscription
	if err := serializer.Unmarshal(raw, &legacy); err != nil {
		return nil
	}
	return legacy
}

type patternKey struct {
	subject string
	queue   string
}

// ptrSlab batches optional JSON pointer values so addresses stay valid
// when capacity is reserved up front (no realloc during take*)
type ptrSlab struct {
	floats []float64
	bools  []bool
}

func (s *ptrSlab) f64(v float64) *float64 {
	// Never grow past reserved capacity: append realloc would invalidate prior pointers
	if len(s.floats) == cap(s.floats) {
		p := new(float64)
		*p = v
		return p
	}
	s.floats = append(s.floats, v)
	return &s.floats[len(s.floats)-1]
}

func (s *ptrSlab) boolean(v bool) *bool {
	if len(s.bools) == cap(s.bools) {
		p := new(bool)
		*p = v
		return p
	}
	s.bools = append(s.bools, v)
	return &s.bools[len(s.bools)-1]
}

// BuildRequestReplySnapshot aggregates connz subscriptions into request/reply patterns
func BuildRequestReplySnapshot(raw []byte, probeResults []domain.RequestReplyProbeResult) domain.RequestReplySnapshot {
	var payload connzPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return domain.RequestReplySnapshot{
			Patterns:    []domain.RequestReplyPattern{},
			Connections: []domain.RequestReplyConnection{},
		}
	}

	resolved := make([][]connzSubscription, len(payload.Connections))
	subCount := 0
	for i, conn := range payload.Connections {
		subs := conn.subscriptions()
		resolved[i] = subs
		subCount += len(subs)
	}
	// Cap covers: per-conn RTT, per-pattern min/med/max/probe latency, snapshot median/maxProbe
	slab := ptrSlab{
		floats: make([]float64, 0, len(payload.Connections)+subCount*4+2),
		bools:  make([]bool, 0, subCount),
	}

	probeBySubject := make(map[string]domain.RequestReplyProbeResult, len(probeResults))
	for _, result := range probeResults {
		probeBySubject[result.Subject] = result
	}

	var (
		requesterCIDs     = make(map[int]struct{})
		responderCIDs     = make(map[int]struct{})
		patternResponders = make(map[patternKey]map[int]struct{})
		patternRtts       = make(map[patternKey][]float64)
		connections       []domain.RequestReplyConnection
		participantRtts   []float64
	)

	for i, conn := range payload.Connections {
		subs := resolved[i]
		inboxSubs, serviceSubs := classifySubscriptions(subs)
		isRequester := len(inboxSubs) > 0
		isResponder := len(serviceSubs) > 0
		if !isRequester && !isResponder {
			continue
		}

		rttMs, hasRtt := ParseRttMs(conn.Rtt)
		if isRequester {
			requesterCIDs[conn.CID] = struct{}{}
		}
		if isResponder {
			responderCIDs[conn.CID] = struct{}{}
		}
		if hasRtt {
			participantRtts = append(participantRtts, rttMs)
		}

		for _, sub := range subs {
			if isInternalSubject(sub.Subject) || isInboxSubject(sub.Subject) {
				continue
			}
			key := patternKey{subject: sub.Subject, queue: sub.Queue}
			if patternResponders[key] == nil {
				patternResponders[key] = make(map[int]struct{})
			}
			patternResponders[key][conn.CID] = struct{}{}
			if hasRtt {
				patternRtts[key] = append(patternRtts[key], rttMs)
			}
		}

		connections = append(connections, domain.RequestReplyConnection{
			CID:          conn.CID,
			Name:         conn.Name,
			Account:      conn.Account,
			RttMs:        optionalFloat(&slab, rttMs, hasRtt),
			InboxSubs:    inboxSubs,
			ServiceSubs:  serviceSubs,
			PendingBytes: conn.PendingBytes,
		})
	}

	requesterCount := len(requesterCIDs)
	patterns := make([]domain.RequestReplyPattern, 0, len(patternResponders))
	for key, cids := range patternResponders {
		rtts := patternRtts[key]
		minMs, medMs, maxMs, ok := rttStats(rtts)
		pattern := domain.RequestReplyPattern{
			Subject:        key.subject,
			Queue:          key.queue,
			RequesterCount: requesterCount,
			ResponderCount: len(cids),
		}
		if ok {
			pattern.RttMinMs = slab.f64(minMs)
			pattern.RttMedianMs = slab.f64(medMs)
			pattern.RttMaxMs = slab.f64(maxMs)
		}
		if probe, ok := probeBySubject[key.subject]; ok {
			pattern.ProbeLatencyMs = slab.f64(probe.LatencyMs)
			pattern.ProbeOk = slab.boolean(probe.OK)
			pattern.ProbeError = probe.Error
		}
		patterns = append(patterns, pattern)
	}

	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Subject != patterns[j].Subject {
			return patterns[i].Subject < patterns[j].Subject
		}
		return patterns[i].Queue < patterns[j].Queue
	})

	sort.Slice(connections, func(i, j int) bool {
		return connections[i].CID < connections[j].CID
	})

	return domain.RequestReplySnapshot{
		Patterns:    patterns,
		Connections: connections,
		Requesters:  requesterCount,
		Responders:  len(responderCIDs),
		MedianRttMs: medianPtr(&slab, participantRtts),
		MaxProbeMs:  maxProbeLatency(&slab, probeResults),
	}
}

func classifySubscriptions(subs []connzSubscription) (inbox []string, service []string) {
	for _, sub := range subs {
		subject := strings.TrimSpace(sub.Subject)
		if commonstrings.IsEmpty(subject) {
			continue
		}
		if isInboxSubject(subject) {
			inbox = append(inbox, subject)
			continue
		}
		if isInternalSubject(subject) {
			continue
		}
		if !commonstrings.IsEmpty(sub.Queue) {
			service = append(service, queueServiceLabel(subject, sub.Queue))
		} else {
			service = append(service, subject)
		}
	}
	return inbox, service
}

func queueServiceLabel(subject, queue string) string {
	var b strings.Builder
	b.Grow(len(subject) + len(queue) + len(" (queue:)"))
	b.WriteString(subject)
	b.WriteString(" (queue:")
	b.WriteString(queue)
	b.WriteByte(')')
	return b.String()
}

func isInboxSubject(subject string) bool {
	return subject == "_INBOX.>" || strings.HasPrefix(subject, "_INBOX.")
}

func isInternalSubject(subject string) bool {
	return strings.HasPrefix(subject, "$JS.") ||
		strings.HasPrefix(subject, "$SYS.") ||
		isInboxSubject(subject)
}

// ParseRttMs converts NATS connz RTT strings (e.g. "1.23ms") to milliseconds
func ParseRttMs(rtt string) (float64, bool) {
	rtt = strings.TrimSpace(rtt)
	if commonstrings.IsEmpty(rtt) {
		return 0, false
	}
	d, err := time.ParseDuration(rtt)
	if err != nil {
		return 0, false
	}
	ms := float64(d) / float64(time.Millisecond)
	if ms < 0 || math.IsNaN(ms) || math.IsInf(ms, 0) {
		return 0, false
	}
	return ms, true
}

func rttStats(values []float64) (minMs, medMs, maxMs float64, ok bool) {
	if len(values) == 0 {
		return 0, 0, 0, false
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	minMs = sorted[0]
	maxMs = sorted[len(sorted)-1]
	medMs = sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		medMs = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	return minMs, medMs, maxMs, true
}

func medianPtr(slab *ptrSlab, values []float64) *float64 {
	_, med, _, ok := rttStats(values)
	if !ok {
		return nil
	}
	return slab.f64(med)
}

func maxProbeLatency(slab *ptrSlab, results []domain.RequestReplyProbeResult) *float64 {
	var max float64
	has := false
	for _, result := range results {
		if !result.OK {
			continue
		}
		if !has || result.LatencyMs > max {
			max = result.LatencyMs
			has = true
		}
	}
	if !has {
		return nil
	}
	return slab.f64(max)
}

func optionalFloat(slab *ptrSlab, value float64, ok bool) *float64 {
	if !ok {
		return nil
	}
	return slab.f64(value)
}
