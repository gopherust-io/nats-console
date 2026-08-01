package domain

import (
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Subject naming finding kinds.
const (
	SubjectNamingKindWrongCase           = "wrong_case"
	SubjectNamingKindMissingDots         = "missing_dots"
	SubjectNamingKindNonDotSeparator     = "non_dot_separator"
	SubjectNamingKindShallowHierarchy    = "shallow_hierarchy"
	SubjectNamingKindInconsistentVariant = "inconsistent_variant"
)

// Subject naming reason codes.
const (
	SubjectNamingReasonUppercase      = "uppercase"
	SubjectNamingReasonCamelCase      = "camel_case"
	SubjectNamingReasonUnderscoreDash = "underscore_or_dash"
	SubjectNamingReasonFewerThanThree = "fewer_than_three_tokens"
	SubjectNamingReasonVariantCluster = "variant_cluster"
)

// SubjectNamingConsumerInput is one consumer for naming analysis.
type SubjectNamingConsumerInput struct {
	Name           string
	FilterSubject  string
	FilterSubjects []string
}

// SubjectNamingInput is one stream for naming analysis.
type SubjectNamingInput struct {
	Name      string
	Subjects  []string
	Consumers []SubjectNamingConsumerInput
}

// SubjectNamingFinding is one subject naming lint or inconsistency.
type SubjectNamingFinding struct {
	Kind      string   `json:"kind"`
	Stream    string   `json:"stream,omitempty"`
	Consumer  string   `json:"consumer,omitempty"`
	Subject   string   `json:"subject"`
	Suggested string   `json:"suggested"`
	Reasons   []string `json:"reasons"`
	Cluster   []string `json:"cluster,omitempty"`
}

// SubjectNamingTotals counts findings by kind.
type SubjectNamingTotals struct {
	WrongCase            int `json:"wrongCase"`
	MissingDots          int `json:"missingDots"`
	NonDotSeparator      int `json:"nonDotSeparator"`
	ShallowHierarchy     int `json:"shallowHierarchy"`
	InconsistentVariants int `json:"inconsistentVariants"`
	Total                int `json:"total"`
}

// SubjectNamingSnapshot is the API payload for the subject naming auditor.
type SubjectNamingSnapshot struct {
	CapturedAt time.Time              `json:"capturedAt,omitzero"`
	Findings   []SubjectNamingFinding `json:"findings"`
	Totals     SubjectNamingTotals    `json:"totals"`
}

type namingOccurrence struct {
	Stream   string
	Consumer string
	Subject  string
}

// AnalyzeSubjectNaming derives naming findings from stream subjects and consumer filters.
func AnalyzeSubjectNaming(inputs []SubjectNamingInput) SubjectNamingSnapshot {
	out := SubjectNamingSnapshot{
		Findings: []SubjectNamingFinding{},
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
			for _, f := range namingConsumerFilters(c) {
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

	// Per-subject lint findings.
	for _, occ := range occurrences {
		out.Findings = append(out.Findings, lintSubject(occ)...)
	}

	// Inconsistency clusters by fingerprint.
	groups := map[string][]namingOccurrence{}
	for _, occ := range occurrences {
		fp := SubjectFingerprint(occ.Subject)
		if commonstrings.IsEmpty(fp) {
			continue
		}
		groups[fp] = append(groups[fp], occ)
	}
	for _, members := range groups {
		literals := uniqueLiterals(members)
		if len(literals) < 2 {
			continue
		}
		suggested := clusterSuggestion(literals)
		sort.Strings(literals)
		for _, occ := range members {
			if occ.Subject == suggested {
				continue
			}
			out.Findings = append(out.Findings, SubjectNamingFinding{
				Kind:      SubjectNamingKindInconsistentVariant,
				Stream:    occ.Stream,
				Consumer:  occ.Consumer,
				Subject:   occ.Subject,
				Suggested: suggested,
				Reasons:   []string{SubjectNamingReasonVariantCluster},
				Cluster:   append([]string(nil), literals...),
			})
		}
	}

	out.Totals = tallySubjectNamingFindings(out.Findings)
	return out
}

func namingConsumerFilters(c SubjectNamingConsumerInput) []string {
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

func subjectHasWildcard(s string) bool {
	for tok := range strings.SplitSeq(s, ".") {
		if tok == "*" || tok == ">" {
			return true
		}
	}
	return false
}

func lintSubject(occ namingOccurrence) []SubjectNamingFinding {
	s := occ.Subject
	norm := NormalizeSubject(s)
	var findings []SubjectNamingFinding

	hasUpper := false
	for _, r := range s {
		if unicode.IsUpper(r) {
			hasUpper = true
			break
		}
	}
	if hasUpper {
		findings = append(findings, SubjectNamingFinding{
			Kind:      SubjectNamingKindWrongCase,
			Stream:    occ.Stream,
			Consumer:  occ.Consumer,
			Subject:   s,
			Suggested: norm,
			Reasons:   []string{SubjectNamingReasonUppercase},
		})
	}

	if !strings.Contains(s, ".") && looksCamelOrPascal(s) {
		findings = append(findings, SubjectNamingFinding{
			Kind:      SubjectNamingKindMissingDots,
			Stream:    occ.Stream,
			Consumer:  occ.Consumer,
			Subject:   s,
			Suggested: norm,
			Reasons:   []string{SubjectNamingReasonCamelCase},
		})
	}

	if strings.ContainsAny(s, "_-") {
		findings = append(findings, SubjectNamingFinding{
			Kind:      SubjectNamingKindNonDotSeparator,
			Stream:    occ.Stream,
			Consumer:  occ.Consumer,
			Subject:   s,
			Suggested: norm,
			Reasons:   []string{SubjectNamingReasonUnderscoreDash},
		})
	}

	// Shallow: count tokens after normalize for camelCase-only inputs;
	// for dotted subjects use literal dot tokens.
	tokenCount := literalTokenCount(s)
	if tokenCount > 0 && tokenCount < 3 {
		suggested := norm
		if tokens := strings.Split(norm, "."); len(tokens) == 2 {
			suggested = expandShallow(tokens)
		}
		findings = append(findings, SubjectNamingFinding{
			Kind:      SubjectNamingKindShallowHierarchy,
			Stream:    occ.Stream,
			Consumer:  occ.Consumer,
			Subject:   s,
			Suggested: suggested,
			Reasons:   []string{SubjectNamingReasonFewerThanThree},
		})
	}

	return findings
}

func literalTokenCount(s string) int {
	if strings.Contains(s, ".") || strings.ContainsAny(s, "_-") {
		n := 0
		for _, tok := range splitSeparators(s) {
			parts := splitCamel(tok)
			n += len(parts)
		}
		return n
	}
	if looksCamelOrPascal(s) {
		return len(splitCamel(s))
	}
	if commonstrings.IsEmpty(strings.TrimSpace(s)) {
		return 0
	}
	return 1
}

func looksCamelOrPascal(s string) bool {
	hasLower := false
	hasUpper := false
	for _, r := range s {
		if unicode.IsLower(r) {
			hasLower = true
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
	}
	return hasLower && hasUpper
}

// NormalizeSubject converts a subject to dot.lower form.
func NormalizeSubject(s string) string {
	s = strings.TrimSpace(s)
	if commonstrings.IsEmpty(s) {
		return ""
	}
	var tokens []string
	for _, part := range splitSeparators(s) {
		tokens = append(tokens, splitCamel(part)...)
	}
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ToLower(strings.TrimSpace(t))
		if commonstrings.IsEmpty(t) || t == "*" || t == ">" {
			continue
		}
		out = append(out, t)
	}
	return strings.Join(out, ".")
}

// SubjectFingerprint groups naming variants (singularizes first token).
func SubjectFingerprint(s string) string {
	norm := NormalizeSubject(s)
	if commonstrings.IsEmpty(norm) {
		return ""
	}
	tokens := strings.Split(norm, ".")
	if len(tokens) == 0 {
		return ""
	}
	tokens[0] = singularize(tokens[0])
	return strings.Join(tokens, ".")
}

func splitSeparators(s string) []string {
	s = strings.ReplaceAll(s, "_", ".")
	s = strings.ReplaceAll(s, "-", ".")
	parts := strings.Split(s, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if commonstrings.IsEmpty(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func splitCamel(s string) []string {
	if commonstrings.IsEmpty(s) {
		return nil
	}
	runes := []rune(s)
	var tokens []string
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]
		boundary := false
		if unicode.IsUpper(cur) && unicode.IsLower(prev) {
			boundary = true
		}
		if unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(prev) {
			boundary = true
		}
		if boundary {
			tokens = append(tokens, string(runes[start:i]))
			start = i
		}
	}
	tokens = append(tokens, string(runes[start:]))
	return tokens
}

func singularize(tok string) string {
	if len(tok) > 1 && strings.HasSuffix(tok, "s") && !strings.HasSuffix(tok, "ss") {
		return tok[:len(tok)-1]
	}
	return tok
}

func isPlural(tok string) bool {
	return singularize(tok) != tok
}

func expandShallow(tokens []string) string {
	if len(tokens) != 2 {
		return strings.Join(tokens, ".")
	}
	first, action := tokens[0], tokens[1]
	if isPlural(first) {
		return first + "." + singularize(first) + "." + action
	}
	return first + "." + first + "." + action
}

func uniqueLiterals(members []namingOccurrence) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range members {
		if _, ok := seen[m.Subject]; ok {
			continue
		}
		seen[m.Subject] = struct{}{}
		out = append(out, m.Subject)
	}
	return out
}

func clusterSuggestion(literals []string) string {
	// Prefer longest NormalizeSubject among members that are already fully lower + dotted.
	bestNorm := ""
	bestScore := -1
	for _, lit := range literals {
		norm := NormalizeSubject(lit)
		score := len(strings.Split(norm, "."))
		// Prefer already-compliant literals slightly.
		if lit == norm {
			score += 10
		}
		if score > bestScore || (score == bestScore && len(norm) > len(bestNorm)) {
			bestScore = score
			bestNorm = norm
		}
	}
	tokens := strings.Split(bestNorm, ".")
	if len(tokens) == 2 {
		// Prefer plural domain form if any member starts with a plural domain after normalize.
		pluralDomain := ""
		for _, lit := range literals {
			n := NormalizeSubject(lit)
			parts := strings.Split(n, ".")
			if len(parts) >= 1 && isPlural(parts[0]) {
				pluralDomain = parts[0]
				break
			}
		}
		if !commonstrings.IsEmpty(pluralDomain) {
			return pluralDomain + "." + singularize(pluralDomain) + "." + tokens[1]
		}
		return expandShallow(tokens)
	}
	if len(tokens) >= 3 {
		// If first token is singular but a peer has plural, prefer plural domain.
		for _, lit := range literals {
			n := NormalizeSubject(lit)
			parts := strings.Split(n, ".")
			if len(parts) >= 1 && isPlural(parts[0]) {
				out := append([]string{parts[0]}, tokens[1:]...)
				// Ensure entity is singular of domain when matching pattern domain.entity.action
				if len(out) >= 2 && out[1] == parts[0] {
					out[1] = singularize(parts[0])
				}
				return strings.Join(out, ".")
			}
		}
	}
	return bestNorm
}

func tallySubjectNamingFindings(findings []SubjectNamingFinding) SubjectNamingTotals {
	var t SubjectNamingTotals
	t.Total = len(findings)
	for _, f := range findings {
		switch f.Kind {
		case SubjectNamingKindWrongCase:
			t.WrongCase++
		case SubjectNamingKindMissingDots:
			t.MissingDots++
		case SubjectNamingKindNonDotSeparator:
			t.NonDotSeparator++
		case SubjectNamingKindShallowHierarchy:
			t.ShallowHierarchy++
		case SubjectNamingKindInconsistentVariant:
			t.InconsistentVariants++
		}
	}
	return t
}
