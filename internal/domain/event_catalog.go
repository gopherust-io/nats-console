package domain

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// EventCatalogLiveStream is JetStream inventory used to discover catalog events.
type EventCatalogLiveStream struct {
	Name      string
	Subjects  []string
	Consumers []EventCatalogLiveConsumer
}

// EventCatalogLiveConsumer is a consumer on a live stream.
type EventCatalogLiveConsumer struct {
	Metadata       map[string]string
	Name           string
	DurableName    string
	FilterSubject  string
	FilterSubjects []string
}

// EventCatalogDoc is a persisted enrichment for a subject.
type EventCatalogDoc struct {
	CreatedAt        time.Time `json:"createdAt,omitzero"`
	UpdatedAt        time.Time `json:"updatedAt,omitzero"`
	Subject          string    `json:"subject"`
	Owner            string    `json:"owner"`
	Description      string    `json:"description"`
	SuccessorSubject string    `json:"successorSubject,omitempty"`
	DeprecationNote  string    `json:"deprecationNote,omitempty"`
	UpdatedBy        string    `json:"updatedBy,omitempty"`
	Schema           []byte    `json:"schema,omitempty"`
	Example          []byte    `json:"example,omitempty"`
	Deprecated       bool      `json:"deprecated"`
}

// EventCatalogConsumer is a live consumer attached to an event.
type EventCatalogConsumer struct {
	Name    string `json:"name"`
	Stream  string `json:"stream"`
	Service string `json:"service,omitempty"`
}

// EventCatalogEntry is one Swagger-style event row (live and/or documented).
type EventCatalogEntry struct {
	CreatedAt        time.Time              `json:"createdAt,omitzero"`
	UpdatedAt        time.Time              `json:"updatedAt,omitzero"`
	SuccessorSubject string                 `json:"successorSubject,omitempty"`
	Subject          string                 `json:"subject"`
	DeprecationNote  string                 `json:"deprecationNote,omitempty"`
	UpdatedBy        string                 `json:"updatedBy,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Owner            string                 `json:"owner,omitempty"`
	Schema           []byte                 `json:"schema,omitempty"`
	Example          []byte                 `json:"example,omitempty"`
	Streams          []string               `json:"streams"`
	Consumers        []EventCatalogConsumer `json:"consumers"`
	Deprecated       bool                   `json:"deprecated"`
	Documented       bool                   `json:"documented"`
	Orphan           bool                   `json:"orphan"`
}

// EventCatalogTotals summarizes catalog inventory.
type EventCatalogTotals struct {
	Total        int `json:"total"`
	Documented   int `json:"documented"`
	Undocumented int `json:"undocumented"`
	Orphan       int `json:"orphan"`
}

// EventCatalogSnapshot is the merged live + docs catalog response.
type EventCatalogSnapshot struct {
	CapturedAt time.Time           `json:"capturedAt"`
	Entries    []EventCatalogEntry `json:"entries"`
	Totals     EventCatalogTotals  `json:"totals"`
}

// EventCatalogUpsert is the PUT body for catalog enrichments.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type EventCatalogUpsert struct {
	Owner            string `json:"owner"`
	Description      string `json:"description"`
	SuccessorSubject string `json:"successorSubject"`
	DeprecationNote  string `json:"deprecationNote"`
	Schema           []byte `json:"schema"`
	Example          []byte `json:"example"`
	Deprecated       bool   `json:"deprecated"`
}

// IsConcreteCatalogSubject reports whether s can be a catalog event name
// (no NATS wildcards).
func IsConcreteCatalogSubject(s string) bool {
	s = strings.TrimSpace(s)
	if commonstrings.IsEmpty(s) {
		return false
	}
	return !strings.ContainsAny(s, "*>")
}

// ValidateEventCatalogSubject rejects empty subjects and wildcards.
func ValidateEventCatalogSubject(subject string) error {
	subject = strings.TrimSpace(subject)
	if commonstrings.IsEmpty(subject) {
		return errors.New("subject required")
	}
	if strings.ContainsAny(subject, "*>") {
		return errors.New("catalog subject cannot contain wildcards")
	}
	if strings.ContainsAny(subject, " \t\n\r") {
		return errors.New("catalog subject cannot contain whitespace")
	}
	return nil
}

// ValidateEventCatalogSchema requires schema to be absent/null or a JSON object.
func ValidateEventCatalogSchema(raw []byte) error {
	return validateCatalogJSONObject(raw, "schema")
}

// ValidateEventCatalogExample requires example to be absent/null or a JSON object.
func ValidateEventCatalogExample(raw []byte) error {
	return validateCatalogJSONObject(raw, "example")
}

func validateCatalogJSONObject(raw []byte, field string) error {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(commonstrings.BytesToString(raw))
	if commonstrings.IsEmpty(trimmed) || trimmed == "null" {
		return nil
	}
	var v any
	if err := serializer.Unmarshal(raw, &v); err != nil {
		return errors.New(field + " must be valid JSON")
	}
	if _, ok := v.(map[string]any); !ok {
		return errors.New(field + " must be a JSON object")
	}
	return nil
}

// CanonicalEventCatalogSubject trims and validates a catalog subject.
func CanonicalEventCatalogSubject(subject string) (string, error) {
	subject = strings.TrimSpace(subject)
	if err := ValidateEventCatalogSubject(subject); err != nil {
		return "", err
	}
	return subject, nil
}

// BuildEventCatalog merges live JetStream inventory with Postgres docs.
// Docs are a documentation overlay only: this never creates, updates, or
// deletes streams, consumers, subjects, or any other JetStream resource.
func BuildEventCatalog(live []EventCatalogLiveStream, docs []EventCatalogDoc) EventCatalogSnapshot {
	subjects := make(map[string]struct{})
	docsBySubject := make(map[string]EventCatalogDoc)

	for _, stream := range live {
		for _, subj := range stream.Subjects {
			subj = strings.TrimSpace(subj)
			if IsConcreteCatalogSubject(subj) {
				subjects[subj] = struct{}{}
			}
		}
		for _, c := range stream.Consumers {
			for _, filter := range catalogConsumerFilters(c) {
				if IsConcreteCatalogSubject(filter) {
					subjects[filter] = struct{}{}
				}
			}
		}
	}

	for _, doc := range docs {
		subject, err := CanonicalEventCatalogSubject(doc.Subject)
		if err != nil {
			continue
		}
		subjects[subject] = struct{}{}
		cp := doc
		cp.Subject = subject
		docsBySubject[subject] = cp
	}

	entries := make([]EventCatalogEntry, 0, len(subjects))
	var totals EventCatalogTotals
	for subject := range subjects {
		streams, consumers := attachLiveCatalog(live, subject)
		entry := EventCatalogEntry{
			Subject:   subject,
			Streams:   streams,
			Consumers: consumers,
			Orphan:    len(streams) == 0 && len(consumers) == 0,
		}
		if doc, ok := docsBySubject[subject]; ok {
			entry.Owner = strings.TrimSpace(doc.Owner)
			entry.Description = strings.TrimSpace(doc.Description)
			entry.Schema = normalizeCatalogJSON(doc.Schema)
			entry.Example = normalizeCatalogJSON(doc.Example)
			entry.Deprecated = doc.Deprecated
			entry.SuccessorSubject = strings.TrimSpace(doc.SuccessorSubject)
			entry.DeprecationNote = strings.TrimSpace(doc.DeprecationNote)
			entry.UpdatedBy = strings.TrimSpace(doc.UpdatedBy)
			entry.CreatedAt = doc.CreatedAt
			entry.UpdatedAt = doc.UpdatedAt
			entry.Documented = catalogDocHasContent(entry)
		}
		if entry.Documented {
			totals.Documented++
		} else {
			totals.Undocumented++
		}
		if entry.Orphan {
			totals.Orphan++
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Subject < entries[j].Subject
	})
	totals.Total = len(entries)
	return EventCatalogSnapshot{
		Entries: entries,
		Totals:  totals,
	}
}

func attachLiveCatalog(live []EventCatalogLiveStream, subject string) ([]string, []EventCatalogConsumer) {
	streamSet := make(map[string]struct{})
	consumerSet := make(map[string]EventCatalogConsumer)

	for _, stream := range live {
		streamName := strings.TrimSpace(stream.Name)
		if commonstrings.IsEmpty(streamName) {
			continue
		}
		covers := streamCoversSubject(stream.Subjects, subject)
		matched := false
		for _, c := range stream.Consumers {
			if !catalogConsumerMatchesSubject(c, subject) {
				continue
			}
			if !covers && !concreteFilterEquals(c, subject) {
				continue
			}
			name := catalogConsumerName(c)
			if commonstrings.IsEmpty(name) {
				continue
			}
			key := streamName + "\x00" + name
			consumerSet[key] = EventCatalogConsumer{
				Name:    name,
				Stream:  streamName,
				Service: catalogConsumerService(c),
			}
			streamSet[streamName] = struct{}{}
			matched = true
		}
		if covers && !matched {
			streamSet[streamName] = struct{}{}
		}
	}

	return sortedKeys(streamSet), sortedCatalogConsumers(consumerSet)
}

func catalogConsumerName(c EventCatalogLiveConsumer) string {
	if s := strings.TrimSpace(c.Name); !commonstrings.IsEmpty(s) {
		return s
	}
	if s := strings.TrimSpace(c.DurableName); !commonstrings.IsEmpty(s) {
		return s
	}
	return ""
}

func catalogConsumerService(c EventCatalogLiveConsumer) string {
	if c.Metadata != nil {
		if s := strings.TrimSpace(c.Metadata[MetadataServiceKey]); !commonstrings.IsEmpty(s) {
			return s
		}
	}
	if s := strings.TrimSpace(c.DurableName); !commonstrings.IsEmpty(s) {
		return s
	}
	return catalogConsumerName(c)
}

func catalogConsumerFilters(c EventCatalogLiveConsumer) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(f string) {
		f = strings.TrimSpace(f)
		if commonstrings.IsEmpty(f) {
			return
		}
		if _, ok := seen[f]; ok {
			return
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	add(c.FilterSubject)
	for _, f := range c.FilterSubjects {
		add(f)
	}
	return out
}

func concreteFilterEquals(c EventCatalogLiveConsumer, subject string) bool {
	for _, f := range catalogConsumerFilters(c) {
		if IsConcreteCatalogSubject(f) && f == subject {
			return true
		}
	}
	return false
}

func catalogConsumerMatchesSubject(c EventCatalogLiveConsumer, subject string) bool {
	filters := catalogConsumerFilters(c)
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f == subject || matchSubjectPattern(subject, f) {
			return true
		}
	}
	return false
}

func streamCoversSubject(patterns []string, subject string) bool {
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if commonstrings.IsEmpty(p) {
			continue
		}
		if p == subject || matchSubjectPattern(subject, p) {
			return true
		}
	}
	return false
}

func sortedCatalogConsumers(set map[string]EventCatalogConsumer) []EventCatalogConsumer {
	if len(set) == 0 {
		return []EventCatalogConsumer{}
	}
	out := make([]EventCatalogConsumer, 0, len(set))
	for _, c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Stream != out[j].Stream {
			return out[i].Stream < out[j].Stream
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeCatalogJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(commonstrings.BytesToString(raw))
	if commonstrings.IsEmpty(trimmed) || trimmed == "null" {
		return nil
	}
	return commonstrings.StringToBytes(trimmed)
}

func catalogDocHasContent(e EventCatalogEntry) bool {
	return !commonstrings.IsEmpty(e.Owner) ||
		!commonstrings.IsEmpty(e.Description) ||
		len(e.Schema) > 0 ||
		len(e.Example) > 0 ||
		e.Deprecated ||
		!commonstrings.IsEmpty(e.SuccessorSubject) ||
		!commonstrings.IsEmpty(e.DeprecationNote)
}
