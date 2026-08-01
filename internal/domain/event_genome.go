package domain

import (
	"sort"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Event genome reason codes.
const (
	EventGenomeReasonActionSynonym = "action_synonym"
	EventGenomeReasonEntityVariant = "entity_variant"
)

// EventGenomeConsumerInput is one consumer for genome analysis.
type EventGenomeConsumerInput struct {
	Name           string
	FilterSubject  string
	FilterSubjects []string
}

// EventGenomeInput is one stream for genome analysis.
type EventGenomeInput struct {
	Name      string
	Subjects  []string
	Consumers []EventGenomeConsumerInput
}

// EventGenomeFinding is one subject that should converge to a cluster suggestion.
type EventGenomeFinding struct {
	Subject   string   `json:"subject"`
	Suggested string   `json:"suggested"`
	Genome    string   `json:"genome"`
	Cluster   []string `json:"cluster"`
	Stream    string   `json:"stream,omitempty"`
	Consumer  string   `json:"consumer,omitempty"`
	Reasons   []string `json:"reasons"`
}

// EventGenomeTotals counts genome duplicate clusters and findings.
type EventGenomeTotals struct {
	Clusters   int `json:"clusters"`
	Duplicates int `json:"duplicates"`
	Total      int `json:"total"`
}

// EventGenomeSnapshot is the API payload for event genome analysis.
type EventGenomeSnapshot struct {
	CapturedAt time.Time            `json:"capturedAt,omitzero"`
	Findings   []EventGenomeFinding `json:"findings"`
	Totals     EventGenomeTotals    `json:"totals"`
}

// actionSynonyms maps action tokens to a canonical form.
var actionSynonyms = map[string]string{
	"created":   "created",
	"new":       "created",
	"added":     "created",
	"insert":    "created",
	"inserted":  "created",
	"updated":   "updated",
	"changed":   "updated",
	"modified":  "updated",
	"patched":   "updated",
	"deleted":   "deleted",
	"removed":   "deleted",
	"cancelled": "deleted",
	"canceled":  "deleted",
}

// AnalyzeEventGenome clusters semantically duplicate subjects from stream/consumer inputs.
func AnalyzeEventGenome(inputs []EventGenomeInput) EventGenomeSnapshot {
	out := EventGenomeSnapshot{
		Findings: []EventGenomeFinding{},
	}

	var occurrences []namingOccurrence
	seen := map[string]struct{}{}

	for _, stream := range inputs {
		name := strings.TrimSpace(stream.Name)
		for _, subj := range stream.Subjects {
			subj = strings.TrimSpace(subj)
			if commonstrings.IsEmpty(subj) || subjectHasWildcard(subj) {
				continue
			}
			key := name + "\x00\x00" + subj
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			occurrences = append(occurrences, namingOccurrence{Stream: name, Subject: subj})
		}
		for _, c := range stream.Consumers {
			cname := strings.TrimSpace(c.Name)
			for _, f := range genomeConsumerFilters(c) {
				if subjectHasWildcard(f) {
					continue
				}
				key := name + "\x00" + cname + "\x00" + f
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				occurrences = append(occurrences, namingOccurrence{Stream: name, Consumer: cname, Subject: f})
			}
		}
	}

	groups := map[string][]namingOccurrence{}
	for _, occ := range occurrences {
		fp := EventGenomeKey(occ.Subject)
		if commonstrings.IsEmpty(fp) {
			continue
		}
		groups[fp] = append(groups[fp], occ)
	}

	clusters := 0
	for genome, members := range groups {
		literals := uniqueLiterals(members)
		if len(literals) < 2 {
			continue
		}
		clusters++
		suggested := withCanonicalAction(clusterSuggestion(literals))
		sort.Strings(literals)
		for _, occ := range members {
			if occ.Subject == suggested {
				continue
			}
			out.Findings = append(out.Findings, EventGenomeFinding{
				Subject:   occ.Subject,
				Suggested: suggested,
				Genome:    genome,
				Cluster:   append([]string(nil), literals...),
				Stream:    occ.Stream,
				Consumer:  occ.Consumer,
				Reasons:   genomeReasons(occ.Subject, suggested),
			})
		}
	}

	sort.Slice(out.Findings, func(i, j int) bool {
		if out.Findings[i].Genome != out.Findings[j].Genome {
			return out.Findings[i].Genome < out.Findings[j].Genome
		}
		if out.Findings[i].Stream != out.Findings[j].Stream {
			return out.Findings[i].Stream < out.Findings[j].Stream
		}
		if out.Findings[i].Consumer != out.Findings[j].Consumer {
			return out.Findings[i].Consumer < out.Findings[j].Consumer
		}
		return out.Findings[i].Subject < out.Findings[j].Subject
	})

	out.Totals = EventGenomeTotals{
		Clusters:   clusters,
		Duplicates: len(out.Findings),
		Total:      len(out.Findings),
	}
	return out
}

// EventGenomeKey returns the semantic fingerprint for a subject.
func EventGenomeKey(s string) string {
	norm := NormalizeSubject(s)
	if commonstrings.IsEmpty(norm) {
		return ""
	}
	tokens := strings.Split(norm, ".")
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) == 1 {
		tokens[0] = canonicalizeAction(singularize(tokens[0]))
		return tokens[0]
	}
	for i := range len(tokens) - 1 {
		tokens[i] = singularize(tokens[i])
	}
	tokens[len(tokens)-1] = canonicalizeAction(tokens[len(tokens)-1])
	return strings.Join(tokens, ".")
}

func canonicalizeAction(tok string) string {
	tok = strings.ToLower(strings.TrimSpace(tok))
	if canon, ok := actionSynonyms[tok]; ok {
		return canon
	}
	return tok
}

func withCanonicalAction(suggested string) string {
	suggested = strings.TrimSpace(suggested)
	if commonstrings.IsEmpty(suggested) {
		return suggested
	}
	tokens := strings.Split(suggested, ".")
	if len(tokens) == 0 {
		return suggested
	}
	tokens[len(tokens)-1] = canonicalizeAction(tokens[len(tokens)-1])
	return strings.Join(tokens, ".")
}

func genomeReasons(subject, suggested string) []string {
	var reasons []string
	tokens := strings.Split(NormalizeSubject(subject), ".")
	if len(tokens) > 0 {
		action := tokens[len(tokens)-1]
		if canonicalizeAction(action) != action {
			reasons = append(reasons, EventGenomeReasonActionSynonym)
		}
	}
	for i := range len(tokens) - 1 {
		if singularize(tokens[i]) != tokens[i] {
			reasons = append(reasons, EventGenomeReasonEntityVariant)
			break
		}
	}
	if subject != suggested && len(reasons) == 0 {
		reasons = append(reasons, EventGenomeReasonEntityVariant)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, EventGenomeReasonEntityVariant)
	}
	return reasons
}

func genomeConsumerFilters(c EventGenomeConsumerInput) []string {
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
