package domain

import (
	"slices"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Zombie finding kinds for JetStream dead-code style detection.
const (
	ZombieKindEmptyStream        = "empty_stream"
	ZombieKindIdleConsumer       = "idle_consumer"
	ZombieKindUnconsumedSubject  = "unconsumed_subject"
	ZombieKindUnpublishedSubject = "unpublished_subject"
	ZombieKindUnboundConsumer    = "unbound_consumer"
)

// Zombie reason codes attached to findings.
const (
	ZombieReasonNeverReceived = "never_received"
	ZombieReasonZeroDelivered = "zero_delivered"
	ZombieReasonNoConsumer    = "no_matching_consumer"
	ZombieReasonEmptyStream   = "empty_stream"
	ZombieReasonOrphanFilter  = "orphan_filter"
)

// ZombieConsumerInput is one consumer for zombie analysis.
type ZombieConsumerInput struct {
	Name             string
	FilterSubject    string
	FilterSubjects   []string
	DeliveredConsSeq uint64
	DeliveredStrSeq  uint64
}

// ZombieStreamInput is one stream for zombie analysis.
type ZombieStreamInput struct {
	Name      string
	Subjects  []string
	Consumers []ZombieConsumerInput
	Messages  uint64
	LastSeq   uint64
}

// ZombieFinding is one unused/idle JetStream entity.
type ZombieFinding struct {
	Kind     string   `json:"kind"`
	Stream   string   `json:"stream,omitempty"`
	Consumer string   `json:"consumer,omitempty"`
	Subject  string   `json:"subject,omitempty"`
	Reasons  []string `json:"reasons"`
}

// ZombieTotals counts findings by kind.
type ZombieTotals struct {
	EmptyStreams        int `json:"emptyStreams"`
	IdleConsumers       int `json:"idleConsumers"`
	UnconsumedSubjects  int `json:"unconsumedSubjects"`
	UnpublishedSubjects int `json:"unpublishedSubjects"`
	UnboundConsumers    int `json:"unboundConsumers"`
	Total               int `json:"total"`
}

// ZombieSnapshot is the API payload for zombie detection.
type ZombieSnapshot struct {
	CapturedAt time.Time       `json:"capturedAt,omitzero"`
	Findings   []ZombieFinding `json:"findings"`
	Totals     ZombieTotals    `json:"totals"`
}

// AnalyzeZombies derives zombie findings from a JetStream topology view.
func AnalyzeZombies(streams []ZombieStreamInput) ZombieSnapshot {
	out := ZombieSnapshot{
		Findings: []ZombieFinding{},
	}
	for _, stream := range streams {
		name := strings.TrimSpace(stream.Name)
		if commonstrings.IsEmpty(name) {
			continue
		}
		empty := stream.Messages == 0 && stream.LastSeq == 0
		if empty {
			out.Findings = append(out.Findings, ZombieFinding{
				Kind:    ZombieKindEmptyStream,
				Stream:  name,
				Reasons: []string{ZombieReasonNeverReceived},
			})
			for _, subj := range stream.Subjects {
				subj = strings.TrimSpace(subj)
				if commonstrings.IsEmpty(subj) {
					continue
				}
				out.Findings = append(out.Findings, ZombieFinding{
					Kind:    ZombieKindUnpublishedSubject,
					Stream:  name,
					Subject: subj,
					Reasons: []string{ZombieReasonEmptyStream, ZombieReasonNeverReceived},
				})
			}
		}

		filters := consumerFilters(stream.Consumers)
		coversAll := slices.ContainsFunc(filters, commonstrings.IsEmpty)

		if !coversAll {
			for _, subj := range stream.Subjects {
				subj = strings.TrimSpace(subj)
				if commonstrings.IsEmpty(subj) {
					continue
				}
				if !subjectCoveredByFilters(subj, filters) {
					out.Findings = append(out.Findings, ZombieFinding{
						Kind:    ZombieKindUnconsumedSubject,
						Stream:  name,
						Subject: subj,
						Reasons: []string{ZombieReasonNoConsumer},
					})
				}
			}
		}

		for _, c := range stream.Consumers {
			cname := strings.TrimSpace(c.Name)
			if commonstrings.IsEmpty(cname) {
				continue
			}
			cf := consumerFilterList(c)
			if !empty && c.DeliveredConsSeq == 0 {
				out.Findings = append(out.Findings, ZombieFinding{
					Kind:     ZombieKindIdleConsumer,
					Stream:   name,
					Consumer: cname,
					Reasons:  []string{ZombieReasonZeroDelivered},
				})
			}
			if len(cf) == 0 {
				continue
			}
			for _, f := range cf {
				f = strings.TrimSpace(f)
				if commonstrings.IsEmpty(f) {
					continue
				}
				if !filterMatchesAnySubject(f, stream.Subjects) {
					out.Findings = append(out.Findings, ZombieFinding{
						Kind:     ZombieKindUnboundConsumer,
						Stream:   name,
						Consumer: cname,
						Subject:  f,
						Reasons:  []string{ZombieReasonOrphanFilter},
					})
				}
			}
		}
	}
	out.Totals = tallyZombieFindings(out.Findings)
	return out
}

func tallyZombieFindings(findings []ZombieFinding) ZombieTotals {
	var t ZombieTotals
	t.Total = len(findings)
	for _, f := range findings {
		switch f.Kind {
		case ZombieKindEmptyStream:
			t.EmptyStreams++
		case ZombieKindIdleConsumer:
			t.IdleConsumers++
		case ZombieKindUnconsumedSubject:
			t.UnconsumedSubjects++
		case ZombieKindUnpublishedSubject:
			t.UnpublishedSubjects++
		case ZombieKindUnboundConsumer:
			t.UnboundConsumers++
		}
	}
	return t
}

func consumerFilters(consumers []ZombieConsumerInput) []string {
	var out []string
	for _, c := range consumers {
		list := consumerFilterList(c)
		if len(list) == 0 {
			out = append(out, "")
			continue
		}
		out = append(out, list...)
	}
	return out
}

func consumerFilterList(c ZombieConsumerInput) []string {
	var out []string
	if s := strings.TrimSpace(c.FilterSubject); !commonstrings.IsEmpty(s) {
		out = append(out, s)
	}
	for _, s := range c.FilterSubjects {
		s = strings.TrimSpace(s)
		if commonstrings.IsEmpty(s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// subjectCoveredByFilters reports whether any consumer filter covers streamSubject.
// Empty filter entries mean "all subjects".
func subjectCoveredByFilters(streamSubject string, filters []string) bool {
	for _, f := range filters {
		if commonstrings.IsEmpty(f) {
			return true
		}
		if subjectsOverlap(streamSubject, f) {
			return true
		}
	}
	return false
}

func filterMatchesAnySubject(filter string, subjects []string) bool {
	for _, s := range subjects {
		s = strings.TrimSpace(s)
		if commonstrings.IsEmpty(s) {
			continue
		}
		if subjectsOverlap(s, filter) {
			return true
		}
	}
	return false
}

// subjectsOverlap reports whether two NATS subject patterns can share traffic.
func subjectsOverlap(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if commonstrings.IsEmpty(a) || commonstrings.IsEmpty(b) {
		return false
	}
	if a == b {
		return true
	}
	return matchSubjectPattern(a, b) || matchSubjectPattern(b, a)
}

// matchSubjectPattern reports whether subject matches pattern (* and > wildcards).
// Duplicated lightly from subjectauth to keep domain free of that import cycle risk.
func matchSubjectPattern(subject, pattern string) bool {
	subTokens := strings.Split(subject, ".")
	patTokens := strings.Split(pattern, ".")
	for len(patTokens) > 0 {
		if len(patTokens) == 1 && patTokens[0] == ">" {
			return len(subTokens) > 0
		}
		if len(subTokens) == 0 {
			return false
		}
		switch patTokens[0] {
		case "*":
			subTokens, patTokens = subTokens[1:], patTokens[1:]
		case ">":
			return true
		default:
			if subTokens[0] != patTokens[0] {
				return false
			}
			subTokens, patTokens = subTokens[1:], patTokens[1:]
		}
	}
	return len(subTokens) == 0
}
