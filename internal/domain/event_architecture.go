package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Event architecture finding kinds.
const (
	ArchKindTooManyConsumers   = "too_many_consumers"
	ArchKindCircularDependency = "circular_dependency"
	ArchKindTightCoupling      = "tight_coupling"
	ArchKindNamingInconsistent = "naming_inconsistent"
	ArchKindPayloadTooLarge    = "payload_too_large"
)

// Event architecture severities.
const (
	ArchSeverityInfo     = "info"
	ArchSeverityWarn     = "warn"
	ArchSeverityCritical = "critical"
)

// Event architecture verdicts.
const (
	ArchVerdictHealthy        = "healthy"
	ArchVerdictNeedsAttention = "needs_attention"
	ArchVerdictAtRisk         = "at_risk"
)

const (
	archTooManyConsumersPerSubject = 8
	archTooManyConsumersPerStream  = 12
	archTightCouplingMinStreams    = 3
	archPayloadAvgBytesThreshold   = 256 * 1024
	archMaxNamingFindings          = 8
	archMaxGenomeFindings          = 8
)

// EventArchitectureConsumerInput is one consumer for architecture analysis.
type EventArchitectureConsumerInput struct {
	Name           string
	FilterSubject  string
	FilterSubjects []string
}

// EventArchitectureInput is one stream for architecture analysis.
type EventArchitectureInput struct {
	Name      string
	Subjects  []string
	Consumers []EventArchitectureConsumerInput
	Messages  uint64
	Bytes     uint64
}

// EventArchitectureFinding is one architecture problem with a suggestion.
type EventArchitectureFinding struct {
	Kind       string   `json:"kind"`
	Severity   string   `json:"severity"`
	Title      string   `json:"title"`
	Suggestion string   `json:"suggestion"`
	Stream     string   `json:"stream,omitempty"`
	Subject    string   `json:"subject,omitempty"`
	Consumer   string   `json:"consumer,omitempty"`
	Evidence   []string `json:"evidence"`
}

// EventArchitectureTotals counts findings by kind and severity.
type EventArchitectureTotals struct {
	ByKind   map[string]int `json:"byKind,omitempty"`
	Problems int            `json:"problems"`
	Critical int            `json:"critical"`
	Warn     int            `json:"warn"`
	Info     int            `json:"info"`
}

// EventArchitectureSnapshot is the API payload for architecture review.
type EventArchitectureSnapshot struct {
	CapturedAt  time.Time                  `json:"capturedAt,omitzero"`
	Verdict     string                     `json:"verdict"`
	Problems    []EventArchitectureFinding `json:"problems"`
	Suggestions []string                   `json:"suggestions"`
	Totals      EventArchitectureTotals    `json:"totals"`
	Demo        bool                       `json:"demo,omitempty"`
}

// AnalyzeEventArchitecture derives architecture problems from stream/consumer inventory.
func AnalyzeEventArchitecture(inputs []EventArchitectureInput) EventArchitectureSnapshot {
	out := EventArchitectureSnapshot{
		Problems:    []EventArchitectureFinding{},
		Suggestions: []string{},
		Verdict:     ArchVerdictHealthy,
		Totals: EventArchitectureTotals{
			ByKind: map[string]int{},
		},
	}

	out.Problems = append(out.Problems, findTooManyConsumers(inputs)...)
	out.Problems = append(out.Problems, findCircularDependencies(inputs)...)
	out.Problems = append(out.Problems, findTightCoupling(inputs)...)
	out.Problems = append(out.Problems, findLargePayloads(inputs)...)
	out.Problems = append(out.Problems, mapNamingFindings(inputs)...)
	out.Problems = append(out.Problems, mapGenomeFindings(inputs)...)

	sort.SliceStable(out.Problems, func(i, j int) bool {
		si, sj := severityRank(out.Problems[i].Severity), severityRank(out.Problems[j].Severity)
		if si != sj {
			return si > sj
		}
		return out.Problems[i].Kind < out.Problems[j].Kind ||
			(out.Problems[i].Kind == out.Problems[j].Kind && out.Problems[i].Title < out.Problems[j].Title)
	})

	seenSug := map[string]struct{}{}
	for _, p := range out.Problems {
		out.Totals.Problems++
		out.Totals.ByKind[p.Kind]++
		switch p.Severity {
		case ArchSeverityCritical:
			out.Totals.Critical++
		case ArchSeverityWarn:
			out.Totals.Warn++
		default:
			out.Totals.Info++
		}
		s := strings.TrimSpace(p.Suggestion)
		if commonstrings.IsEmpty(s) {
			continue
		}
		if _, ok := seenSug[s]; ok {
			continue
		}
		seenSug[s] = struct{}{}
		out.Suggestions = append(out.Suggestions, s)
	}

	switch {
	case out.Totals.Critical > 0:
		out.Verdict = ArchVerdictAtRisk
	case out.Totals.Warn > 0 || out.Totals.Info > 0:
		out.Verdict = ArchVerdictNeedsAttention
	default:
		out.Verdict = ArchVerdictHealthy
	}
	return out
}

func severityRank(s string) int {
	switch s {
	case ArchSeverityCritical:
		return 3
	case ArchSeverityWarn:
		return 2
	default:
		return 1
	}
}

func archConsumerFilters(c EventArchitectureConsumerInput) []string {
	var out []string
	if f := strings.TrimSpace(c.FilterSubject); !commonstrings.IsEmpty(f) {
		out = append(out, f)
	}
	for _, f := range c.FilterSubjects {
		f = strings.TrimSpace(f)
		if commonstrings.IsEmpty(f) {
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return []string{">"}
	}
	return out
}

func findTooManyConsumers(inputs []EventArchitectureInput) []EventArchitectureFinding {
	var out []EventArchitectureFinding
	// Per-stream consumer fan-out.
	for _, stream := range inputs {
		if len(stream.Consumers) >= archTooManyConsumersPerStream {
			out = append(out, EventArchitectureFinding{
				Kind:       ArchKindTooManyConsumers,
				Severity:   ArchSeverityWarn,
				Title:      fmt.Sprintf("Stream %s has %d consumers", stream.Name, len(stream.Consumers)),
				Evidence:   []string{fmt.Sprintf("stream=%s consumers=%d", stream.Name, len(stream.Consumers))},
				Suggestion: "Split high-fan-out streams by bounded contexts or use queue groups / interest retention instead of many durable consumers.",
				Stream:     stream.Name,
			})
		}
	}

	// Per-subject consumer count (distinct consumer names matching subject).
	type hit struct {
		stream   string
		consumer string
	}
	bySubject := map[string][]hit{}
	for _, stream := range inputs {
		for _, subj := range stream.Subjects {
			subj = strings.TrimSpace(subj)
			if commonstrings.IsEmpty(subj) || subjectHasWildcard(subj) {
				continue
			}
			for _, c := range stream.Consumers {
				for _, f := range archConsumerFilters(c) {
					if !subjectIntersects(subj, f) {
						continue
					}
					bySubject[subj] = append(bySubject[subj], hit{stream: stream.Name, consumer: c.Name})
					break
				}
			}
		}
		// Also count filters that match across other streams' published subjects.
		for _, other := range inputs {
			if other.Name == stream.Name {
				continue
			}
			for _, subj := range other.Subjects {
				subj = strings.TrimSpace(subj)
				if commonstrings.IsEmpty(subj) || subjectHasWildcard(subj) {
					continue
				}
				for _, c := range stream.Consumers {
					for _, f := range archConsumerFilters(c) {
						if !subjectIntersects(subj, f) {
							continue
						}
						bySubject[subj] = append(bySubject[subj], hit{stream: stream.Name, consumer: c.Name})
						break
					}
				}
			}
		}
	}

	for subj, hits := range bySubject {
		seen := map[string]struct{}{}
		var uniq []hit
		for _, h := range hits {
			key := h.stream + "\x00" + h.consumer
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			uniq = append(uniq, h)
		}
		if len(uniq) < archTooManyConsumersPerSubject {
			continue
		}
		evidence := make([]string, 0, len(uniq))
		for _, h := range uniq {
			evidence = append(evidence, h.stream+"/"+h.consumer)
		}
		sort.Strings(evidence)
		out = append(out, EventArchitectureFinding{
			Kind:       ArchKindTooManyConsumers,
			Severity:   ArchSeverityWarn,
			Title:      fmt.Sprintf("Subject %s has %d consumers", subj, len(uniq)),
			Evidence:   evidence,
			Suggestion: "Reduce fan-out on " + subj + " — prefer fewer consumers, shared queue groups, or split into narrower subjects.",
			Subject:    subj,
		})
	}
	return out
}

func findCircularDependencies(inputs []EventArchitectureInput) []EventArchitectureFinding {
	var out []EventArchitectureFinding
	seenPair := map[string]struct{}{}
	for i, a := range inputs {
		for j, b := range inputs {
			if i >= j {
				continue
			}
			if streamsCrossConsume(a, b) && streamsCrossConsume(b, a) {
				key := a.Name + "\x00" + b.Name
				if _, ok := seenPair[key]; ok {
					continue
				}
				seenPair[key] = struct{}{}
				out = append(out, EventArchitectureFinding{
					Kind:     ArchKindCircularDependency,
					Severity: ArchSeverityCritical,
					Title:    fmt.Sprintf("Circular dependency between %s and %s", a.Name, b.Name),
					Evidence: []string{
						fmt.Sprintf("%s subjects consumed by %s", a.Name, b.Name),
						fmt.Sprintf("%s subjects consumed by %s", b.Name, a.Name),
					},
					Suggestion: "Break the cycle — pick a single owner stream for shared subjects, or introduce a dedicated bridge stream with one-way flow.",
					Stream:     a.Name,
				})
			}
		}
	}
	return out
}

func streamsCrossConsume(publisher, consumerStream EventArchitectureInput) bool {
	for _, subj := range publisher.Subjects {
		subj = strings.TrimSpace(subj)
		if commonstrings.IsEmpty(subj) {
			continue
		}
		for _, c := range consumerStream.Consumers {
			for _, f := range archConsumerFilters(c) {
				if subjectIntersects(subj, f) {
					return true
				}
			}
		}
	}
	return false
}

func findTightCoupling(inputs []EventArchitectureInput) []EventArchitectureFinding {
	prefixStreams := map[string]map[string]struct{}{}
	for _, stream := range inputs {
		for _, subj := range stream.Subjects {
			subj = strings.TrimSpace(subj)
			if commonstrings.IsEmpty(subj) || subjectHasWildcard(subj) {
				continue
			}
			tok := firstSubjectToken(subj)
			if commonstrings.IsEmpty(tok) {
				continue
			}
			if prefixStreams[tok] == nil {
				prefixStreams[tok] = map[string]struct{}{}
			}
			prefixStreams[tok][stream.Name] = struct{}{}
		}
	}

	var out []EventArchitectureFinding
	for prefix, streams := range prefixStreams {
		if len(streams) < archTightCouplingMinStreams {
			continue
		}
		// Require that at least one of those streams has consumers (coupling via consumption).
		hasConsumers := false
		names := make([]string, 0, len(streams))
		for name := range streams {
			names = append(names, name)
			for _, s := range inputs {
				if s.Name == name && len(s.Consumers) > 0 {
					hasConsumers = true
				}
			}
		}
		if !hasConsumers {
			continue
		}
		sort.Strings(names)
		out = append(out, EventArchitectureFinding{
			Kind:       ArchKindTightCoupling,
			Severity:   ArchSeverityWarn,
			Title:      fmt.Sprintf("Tight coupling on prefix %s across %d streams", prefix, len(names)),
			Evidence:   names,
			Suggestion: "Consolidate " + prefix + ".* ownership into fewer streams or introduce an anti-corruption layer subject boundary.",
			Subject:    prefix,
		})
	}
	return out
}

func firstSubjectToken(subj string) string {
	parts := strings.Split(subj, ".")
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

func findLargePayloads(inputs []EventArchitectureInput) []EventArchitectureFinding {
	var out []EventArchitectureFinding
	for _, stream := range inputs {
		if stream.Messages == 0 {
			continue
		}
		avg := stream.Bytes / stream.Messages
		if avg < archPayloadAvgBytesThreshold {
			continue
		}
		out = append(out, EventArchitectureFinding{
			Kind:     ArchKindPayloadTooLarge,
			Severity: ArchSeverityWarn,
			Title:    fmt.Sprintf("Stream %s average message size is %s", stream.Name, formatByteSize(avg)),
			Evidence: []string{
				fmt.Sprintf("bytes=%d messages=%d avg=%d", stream.Bytes, stream.Messages, avg),
			},
			Suggestion: "Shrink payloads on " + stream.Name + " — store blobs in Object Store/KV and publish references, or compress large fields.",
			Stream:     stream.Name,
		})
	}
	return out
}

func formatByteSize(n uint64) string {
	const kib = 1024
	const mib = 1024 * 1024
	switch {
	case n >= mib:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func mapNamingFindings(inputs []EventArchitectureInput) []EventArchitectureFinding {
	namingIn := make([]SubjectNamingInput, 0, len(inputs))
	for _, s := range inputs {
		ni := SubjectNamingInput{Name: s.Name, Subjects: s.Subjects}
		for _, c := range s.Consumers {
			ni.Consumers = append(ni.Consumers, SubjectNamingConsumerInput(c))
		}
		namingIn = append(namingIn, ni)
	}
	snap := AnalyzeSubjectNaming(namingIn)
	limit := min(archMaxNamingFindings, len(snap.Findings))
	out := make([]EventArchitectureFinding, 0, limit)
	for i := range limit {
		f := snap.Findings[i]
		sug := f.Suggested
		if commonstrings.IsEmpty(sug) {
			sug = "Normalize subject to lowercase dotted hierarchy (entity.action)."
		} else {
			sug = "Rename " + f.Subject + " toward " + sug
		}
		out = append(out, EventArchitectureFinding{
			Kind:       ArchKindNamingInconsistent,
			Severity:   ArchSeverityInfo,
			Title:      "Inconsistent event naming: " + f.Subject,
			Evidence:   append([]string{f.Kind}, f.Reasons...),
			Suggestion: sug,
			Stream:     f.Stream,
			Subject:    f.Subject,
			Consumer:   f.Consumer,
		})
	}
	return out
}

func mapGenomeFindings(inputs []EventArchitectureInput) []EventArchitectureFinding {
	genomeIn := make([]EventGenomeInput, 0, len(inputs))
	for _, s := range inputs {
		gi := EventGenomeInput{Name: s.Name, Subjects: s.Subjects}
		for _, c := range s.Consumers {
			gi.Consumers = append(gi.Consumers, EventGenomeConsumerInput(c))
		}
		genomeIn = append(genomeIn, gi)
	}
	snap := AnalyzeEventGenome(genomeIn)
	limit := min(archMaxGenomeFindings, len(snap.Findings))
	out := make([]EventArchitectureFinding, 0, limit)
	for i := range limit {
		f := snap.Findings[i]
		sug := "Converge synonyms onto " + f.Suggested
		if commonstrings.IsEmpty(f.Suggested) {
			sug = "Converge synonym subjects onto a single canonical event name."
		}
		out = append(out, EventArchitectureFinding{
			Kind:       ArchKindNamingInconsistent,
			Severity:   ArchSeverityInfo,
			Title:      "Semantic duplicate subject: " + f.Subject,
			Evidence:   append([]string{"genome=" + f.Genome}, f.Cluster...),
			Suggestion: sug,
			Stream:     f.Stream,
			Subject:    f.Subject,
			Consumer:   f.Consumer,
		})
	}
	return out
}

// subjectIntersects reports whether a published subject could be delivered to a filter.
func subjectIntersects(published, filter string) bool {
	published = strings.TrimSpace(published)
	filter = strings.TrimSpace(filter)
	if commonstrings.IsEmpty(published) || commonstrings.IsEmpty(filter) {
		return false
	}
	if filter == ">" {
		return true
	}
	if published == filter {
		return true
	}
	return natsSubjectMatch(filter, published)
}

// natsSubjectMatch implements NATS-style filter matching (* one token, > rest).
func natsSubjectMatch(filter, subject string) bool {
	ft := strings.Split(filter, ".")
	st := strings.Split(subject, ".")
	fi, si := 0, 0
	for fi < len(ft) && si < len(st) {
		tok := ft[fi]
		if tok == ">" {
			return true
		}
		if tok != "*" && tok != st[si] {
			return false
		}
		fi++
		si++
	}
	if fi == len(ft) && si == len(st) {
		return true
	}
	if fi < len(ft) && ft[fi] == ">" {
		return true
	}
	return false
}

// DemoEventArchitectureSnapshot returns a canned review for Docs showcase.
func DemoEventArchitectureSnapshot() EventArchitectureSnapshot {
	snap := AnalyzeEventArchitecture([]EventArchitectureInput{
		{
			Name:     "ORDERS",
			Subjects: []string{"orders.created", "orders.new", "Orders.Updated"},
			Messages: 1000,
			Bytes:    1000 * 400 * 1024,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "billing", FilterSubject: "orders.>"},
				{Name: "shipping", FilterSubject: "orders.>"},
				{Name: "analytics", FilterSubject: "orders.>"},
				{Name: "crm", FilterSubject: "orders.>"},
				{Name: "fraud", FilterSubject: "orders.>"},
				{Name: "loyalty", FilterSubject: "orders.>"},
				{Name: "notify", FilterSubject: "orders.>"},
				{Name: "search", FilterSubject: "orders.>"},
				{Name: "audit", FilterSubject: "orders.>"},
				{Name: "billing-echo", FilterSubject: "billing.>"},
			},
		},
		{
			Name:     "BILLING",
			Subjects: []string{"orders.paid", "billing.charged"},
			Messages: 100,
			Bytes:    50_000,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "orders-sync", FilterSubject: "orders.>"},
			},
		},
		{
			Name:     "FULFILLMENT",
			Subjects: []string{"orders.shipped", "orders.fulfilled"},
			Messages: 200,
			Bytes:    80_000,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "orders-mirror", FilterSubject: "orders.>"},
			},
		},
		{
			Name:     "ORDERS_OUTBOX",
			Subjects: []string{"orders.created"},
			Messages: 50,
			Bytes:    10_000,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "billing-bridge", FilterSubject: "billing.>"},
			},
		},
	})
	// Force a visible circular pair for demo: ORDERS <-> BILLING style.
	// Analyze may already find coupling/naming/payload/too-many; ensure demo flag.
	snap.Demo = true
	if snap.Verdict == ArchVerdictHealthy {
		snap.Verdict = ArchVerdictNeedsAttention
	}
	return snap
}
