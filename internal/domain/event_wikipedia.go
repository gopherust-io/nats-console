package domain

import (
	"sort"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// EventWikipediaHistory is auto-assembled history for a subject page.
type EventWikipediaHistory struct {
	CreatedAt time.Time `json:"createdAt,omitzero"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	Streams   []string  `json:"streams"`
}

// EventWikipediaDeprecation is curated deprecation status.
// goalign:ignore // JSON DTO; trailing bool padding is unavoidable
type EventWikipediaDeprecation struct {
	SuccessorSubject string `json:"successorSubject,omitempty"`
	Note             string `json:"note,omitempty"`
	Deprecated       bool   `json:"deprecated"`
}

// EventWikipediaIncidentLink is a deep-link target for consumer incident reconstruction.
type EventWikipediaIncidentLink struct {
	Stream   string `json:"stream"`
	Consumer string `json:"consumer"`
	Service  string `json:"service,omitempty"`
}

// EventWikipediaPage is one auto-assembled Wikipedia article for a subject.
type EventWikipediaPage struct {
	Deprecation    EventWikipediaDeprecation    `json:"deprecation"`
	Subject        string                       `json:"subject"`
	Purpose        string                       `json:"purpose,omitempty"`
	Owner          string                       `json:"owner,omitempty"`
	History        EventWikipediaHistory        `json:"history"`
	Consumers      []EventCatalogConsumer       `json:"consumers"`
	Example        []byte                       `json:"example,omitempty"`
	Schema         []byte                       `json:"schema,omitempty"`
	RelatedEvents  []string                     `json:"relatedEvents"`
	KnownIncidents []EventWikipediaIncidentLink `json:"knownIncidents"`
	Documented     bool                         `json:"documented"`
	Orphan         bool                         `json:"orphan"`
}

// EventWikipediaTotals summarizes Wikipedia inventory.
type EventWikipediaTotals struct {
	Total      int `json:"total"`
	Documented int `json:"documented"`
	Deprecated int `json:"deprecated"`
	Orphan     int `json:"orphan"`
}

// EventWikipediaSnapshot is the assembled Docs Wikipedia response.
type EventWikipediaSnapshot struct {
	CapturedAt time.Time            `json:"capturedAt"`
	Pages      []EventWikipediaPage `json:"pages"`
	Totals     EventWikipediaTotals `json:"totals"`
}

// BuildEventWikipedia merges catalog entries with genome peers into Wikipedia pages.
// Read-only assembly for Docs UI: no JetStream writes; consumers/streams are
// displayed from live inventory, and deprecation/examples stay in Postgres.
func BuildEventWikipedia(catalog EventCatalogSnapshot, genome EventGenomeSnapshot) EventWikipediaSnapshot {
	relatedBySubject := genomeRelatedBySubject(genome)

	pages := make([]EventWikipediaPage, 0, len(catalog.Entries))
	var totals EventWikipediaTotals
	for _, e := range catalog.Entries {
		related := relatedBySubject[e.Subject]
		if related == nil {
			related = []string{}
		}
		incidents := make([]EventWikipediaIncidentLink, 0, len(e.Consumers))
		for _, c := range e.Consumers {
			incidents = append(incidents, EventWikipediaIncidentLink{
				Stream:   c.Stream,
				Consumer: c.Name,
				Service:  c.Service,
			})
		}
		consumers := e.Consumers
		if consumers == nil {
			consumers = []EventCatalogConsumer{}
		}
		streams := e.Streams
		if streams == nil {
			streams = []string{}
		}
		page := EventWikipediaPage{
			Subject: e.Subject,
			Purpose: e.Description,
			History: EventWikipediaHistory{
				CreatedAt: e.CreatedAt,
				UpdatedAt: e.UpdatedAt,
				UpdatedBy: e.UpdatedBy,
				Streams:   streams,
			},
			Owner:          e.Owner,
			Consumers:      consumers,
			Example:        e.Example,
			Schema:         e.Schema,
			RelatedEvents:  related,
			KnownIncidents: incidents,
			Deprecation: EventWikipediaDeprecation{
				Deprecated:       e.Deprecated,
				SuccessorSubject: e.SuccessorSubject,
				Note:             e.DeprecationNote,
			},
			Documented: e.Documented,
			Orphan:     e.Orphan,
		}
		if page.Documented {
			totals.Documented++
		}
		if page.Deprecation.Deprecated {
			totals.Deprecated++
		}
		if page.Orphan {
			totals.Orphan++
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Subject < pages[j].Subject
	})
	totals.Total = len(pages)
	return EventWikipediaSnapshot{
		CapturedAt: catalog.CapturedAt,
		Pages:      pages,
		Totals:     totals,
	}
}

// FilterEventWikipediaPages keeps pages whose subject equals subject (trimmed).
func FilterEventWikipediaPages(snap EventWikipediaSnapshot, subject string) EventWikipediaSnapshot {
	subject = strings.TrimSpace(subject)
	if commonstrings.IsEmpty(subject) {
		return snap
	}
	out := make([]EventWikipediaPage, 0, 1)
	for _, p := range snap.Pages {
		if p.Subject == subject {
			out = append(out, p)
			break
		}
	}
	snap.Pages = out
	var totals EventWikipediaTotals
	for _, p := range out {
		totals.Total++
		if p.Documented {
			totals.Documented++
		}
		if p.Deprecation.Deprecated {
			totals.Deprecated++
		}
		if p.Orphan {
			totals.Orphan++
		}
	}
	snap.Totals = totals
	return snap
}

func genomeRelatedBySubject(genome EventGenomeSnapshot) map[string][]string {
	clusters := make(map[string][]string)
	for _, f := range genome.Findings {
		key := strings.TrimSpace(f.Genome)
		if commonstrings.IsEmpty(key) {
			key = strings.TrimSpace(f.Subject)
		}
		if commonstrings.IsEmpty(key) {
			continue
		}
		if _, ok := clusters[key]; ok {
			continue
		}
		literals := make([]string, 0, len(f.Cluster))
		seen := make(map[string]struct{})
		for _, peer := range f.Cluster {
			peer = strings.TrimSpace(peer)
			if commonstrings.IsEmpty(peer) {
				continue
			}
			if _, ok := seen[peer]; ok {
				continue
			}
			seen[peer] = struct{}{}
			literals = append(literals, peer)
		}
		sort.Strings(literals)
		if len(literals) >= 2 {
			clusters[key] = literals
		}
	}

	out := make(map[string][]string)
	for _, literals := range clusters {
		for _, subject := range literals {
			peers := make([]string, 0, len(literals)-1)
			for _, peer := range literals {
				if peer != subject {
					peers = append(peers, peer)
				}
			}
			out[subject] = peers
		}
	}
	return out
}
