package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ArchitectureRefactorSeed optionally focuses the plan on a review finding.
type ArchitectureRefactorSeed struct {
	Kind    string
	Stream  string
	Subject string
}

// ArchitectureRefactorNode is one labeled node in a before/after graph.
type ArchitectureRefactorNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // stream | consumer | event
}

// ArchitectureRefactorEdge is a directed edge in a before/after graph.
type ArchitectureRefactorEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// ArchitectureRefactorGraph is a simple directed graph for UI rendering.
type ArchitectureRefactorGraph struct {
	Label string                     `json:"label,omitempty"`
	Nodes []ArchitectureRefactorNode `json:"nodes"`
	Edges []ArchitectureRefactorEdge `json:"edges"`
}

// ArchitectureRefactorStep is one migration step.
type ArchitectureRefactorStep struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Order  int    `json:"order"`
}

// ArchitectureRefactorPlan is the API payload for coupling reduction.
type ArchitectureRefactorPlan struct {
	CapturedAt   time.Time                  `json:"capturedAt,omitzero"`
	Seed         *ArchitectureRefactorSeed  `json:"seed,omitempty"`
	Before       ArchitectureRefactorGraph  `json:"before"`
	After        ArchitectureRefactorGraph  `json:"after"`
	ClusterName  string                     `json:"clusterName,omitempty"`
	Question     string                     `json:"question"`
	Verdict      string                     `json:"verdict"`
	Rationale    string                     `json:"rationale"`
	EventSubject string                     `json:"eventSubject"`
	Narrative    string                     `json:"narrative,omitempty"`
	Steps        []ArchitectureRefactorStep `json:"steps"`
	Demo         bool                       `json:"demo,omitempty"`
}

// BuildArchitectureRefactorPlan derives a coupling-reduction plan from inventory.
func BuildArchitectureRefactorPlan(inv ArchitectureInventory, seed ArchitectureRefactorSeed) ArchitectureRefactorPlan {
	plan := ArchitectureRefactorPlan{
		CapturedAt:  inv.CapturedAt,
		ClusterName: inv.ClusterName,
		Question:    "Reduce coupling.",
		Demo:        inv.Demo,
	}
	if !commonstrings.IsEmpty(seed.Kind) || !commonstrings.IsEmpty(seed.Stream) || !commonstrings.IsEmpty(seed.Subject) {
		s := seed
		plan.Seed = &s
	}

	chain := pickCouplingChain(inv.Streams, seed)
	if len(chain) < 2 {
		plan.Verdict = "No strong coupling chain detected"
		plan.Rationale = "Inventory does not show a clear A→B→C or multi-consumer fan-out to refactor. Keep monitoring Architecture Review."
		plan.EventSubject = "domain.event"
		plan.Before = ArchitectureRefactorGraph{Label: "Before"}
		plan.After = ArchitectureRefactorGraph{Label: "After"}
		plan.Steps = defaultHealthySteps()
		return plan
	}

	eventSubject := suggestedEventSubject(chain, seed)
	plan.EventSubject = eventSubject
	plan.Before = buildBeforeGraph(chain)
	plan.After = buildAfterGraph(chain, eventSubject)
	plan.Steps = migrationSteps(chain, eventSubject)
	plan.Verdict = fmt.Sprintf("Decouple %s via event %s", strings.Join(chain, " → "), eventSubject)
	plan.Rationale = fmt.Sprintf(
		"Observed synchronous-style coupling across %s. Introduce a JetStream subject so producers publish once and consumers subscribe independently.",
		strings.Join(chain, ", "),
	)
	return plan
}

// DemoArchitectureRefactorPlan returns the classic A→B→C sample.
func DemoArchitectureRefactorPlan() ArchitectureRefactorPlan {
	inv := ArchitectureInventory{
		ClusterName: "demo-cluster",
		CapturedAt:  time.Now().UTC(),
		Demo:        true,
		Streams: []EventArchitectureInput{
			{
				Name:     "A",
				Subjects: []string{"orders.created"},
				Consumers: []EventArchitectureConsumerInput{
					{Name: "to-b", FilterSubject: "orders.created"},
				},
			},
			{
				Name:     "B",
				Subjects: []string{"orders.enriched"},
				Consumers: []EventArchitectureConsumerInput{
					{Name: "from-a", FilterSubject: "orders.created"},
					{Name: "to-c", FilterSubject: "orders.enriched"},
				},
			},
			{
				Name:     "C",
				Subjects: []string{"orders.fulfilled"},
				Consumers: []EventArchitectureConsumerInput{
					{Name: "from-b", FilterSubject: "orders.enriched"},
				},
			},
		},
	}
	return BuildArchitectureRefactorPlan(inv, ArchitectureRefactorSeed{
		Kind:    ArchKindTightCoupling,
		Subject: "orders",
	})
}

func pickCouplingChain(streams []EventArchitectureInput, seed ArchitectureRefactorSeed) []string {
	byName := map[string]EventArchitectureInput{}
	names := make([]string, 0, len(streams))
	for _, s := range streams {
		byName[s.Name] = s
		names = append(names, s.Name)
	}
	sort.Strings(names)

	// Prefer seed stream + neighbors that cross-consume.
	if !commonstrings.IsEmpty(seed.Stream) {
		if s, ok := byName[seed.Stream]; ok {
			chain := []string{s.Name}
			for _, other := range streams {
				if other.Name == s.Name {
					continue
				}
				if streamsCrossConsume(s, other) || streamsCrossConsume(other, s) {
					chain = append(chain, other.Name)
				}
			}
			if len(chain) >= 2 {
				return uniquePreserve(chain)
			}
		}
	}

	// Soft cycle pair → expand to third if possible.
	for i, a := range streams {
		for j, b := range streams {
			if i >= j {
				continue
			}
			if streamsCrossConsume(a, b) && streamsCrossConsume(b, a) {
				chain := []string{a.Name, b.Name}
				for _, c := range streams {
					if c.Name == a.Name || c.Name == b.Name {
						continue
					}
					if streamsCrossConsume(a, c) || streamsCrossConsume(b, c) || streamsCrossConsume(c, a) || streamsCrossConsume(c, b) {
						chain = append(chain, c.Name)
						break
					}
				}
				return uniquePreserve(chain)
			}
		}
	}

	// Prefix coupling: streams sharing subject prefix.
	prefixStreams := map[string][]string{}
	seedTok := firstSubjectToken(seed.Subject)
	for _, s := range streams {
		for _, subj := range s.Subjects {
			tok := firstSubjectToken(subj)
			if commonstrings.IsEmpty(tok) {
				continue
			}
			if !commonstrings.IsEmpty(seedTok) && tok != seedTok {
				continue
			}
			prefixStreams[tok] = appendUnique(prefixStreams[tok], s.Name)
		}
	}
	best := []string{}
	for _, list := range prefixStreams {
		if len(list) > len(best) {
			best = append([]string(nil), list...)
		}
	}
	if len(best) >= 2 {
		sort.Strings(best)
		if len(best) > 3 {
			best = best[:3]
		}
		return best
	}

	// Fan-out: stream with many consumers → treat producer + top consumers as chain labels.
	for _, s := range streams {
		if len(s.Consumers) < 2 {
			continue
		}
		chain := []string{s.Name}
		for i, c := range s.Consumers {
			if i >= 2 {
				break
			}
			chain = append(chain, c.Name)
		}
		return chain
	}

	if len(names) >= 2 {
		if len(names) > 3 {
			return names[:3]
		}
		return names
	}
	return nil
}

func uniquePreserve(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func suggestedEventSubject(chain []string, seed ArchitectureRefactorSeed) string {
	if !commonstrings.IsEmpty(seed.Subject) {
		tok := firstSubjectToken(seed.Subject)
		if !commonstrings.IsEmpty(tok) {
			return tok + ".changed"
		}
		if !subjectHasWildcard(seed.Subject) {
			return seed.Subject
		}
	}
	if len(chain) > 0 {
		return strings.ToLower(chain[0]) + ".changed"
	}
	return "domain.event"
}

func buildBeforeGraph(chain []string) ArchitectureRefactorGraph {
	g := ArchitectureRefactorGraph{Label: "Before: " + strings.Join(chain, " → ")}
	for _, name := range chain {
		id := mermaidID(name)
		g.Nodes = append(g.Nodes, ArchitectureRefactorNode{ID: id, Label: name, Kind: "stream"})
	}
	for i := 0; i+1 < len(chain); i++ {
		g.Edges = append(g.Edges, ArchitectureRefactorEdge{
			From:  mermaidID(chain[i]),
			To:    mermaidID(chain[i+1]),
			Label: "sync / direct",
		})
	}
	return g
}

func buildAfterGraph(chain []string, eventSubject string) ArchitectureRefactorGraph {
	g := ArchitectureRefactorGraph{Label: "After: " + chain[0] + " → Event → " + strings.Join(chain[1:], ",")}
	producer := chain[0]
	eventID := mermaidID("event_" + eventSubject)
	g.Nodes = append(g.Nodes, ArchitectureRefactorNode{ID: mermaidID(producer), Label: producer, Kind: "stream"})
	g.Nodes = append(g.Nodes, ArchitectureRefactorNode{ID: eventID, Label: eventSubject, Kind: "event"})
	g.Edges = append(g.Edges, ArchitectureRefactorEdge{From: mermaidID(producer), To: eventID, Label: "publish"})
	for _, name := range chain[1:] {
		id := mermaidID(name)
		g.Nodes = append(g.Nodes, ArchitectureRefactorNode{ID: id, Label: name, Kind: "stream"})
		g.Edges = append(g.Edges, ArchitectureRefactorEdge{From: eventID, To: id, Label: "consume"})
	}
	return g
}

func migrationSteps(chain []string, eventSubject string) []ArchitectureRefactorStep {
	producer := chain[0]
	consumers := strings.Join(chain[1:], ", ")
	return []ArchitectureRefactorStep{
		{Order: 1, Title: "Introduce event subject", Detail: fmt.Sprintf("Create or extend a JetStream stream that owns subject `%s` (and document it in Event Catalog).", eventSubject)},
		{Order: 2, Title: "Dual-publish from producer", Detail: fmt.Sprintf("Update `%s` to publish `%s` alongside the existing direct call path so nothing breaks yet.", producer, eventSubject)},
		{Order: 3, Title: "Add consumers on the event", Detail: fmt.Sprintf("Point `%s` at durable consumers filtered on `%s` (queue groups if competing).", consumers, eventSubject)},
		{Order: 4, Title: "Shadow / verify", Detail: "Run both paths in parallel; compare outcomes and lag via Consol Topology / Live until parity is proven."},
		{Order: 5, Title: "Remove direct coupling", Detail: fmt.Sprintf("Delete the sync/direct edges %s → %s once consumers are healthy on the event.", producer, consumers)},
		{Order: 6, Title: "Cut over and monitor", Detail: "Disable dual-publish leftovers, purge unused filters, and watch Architecture Review for residual coupling."},
	}
}

func defaultHealthySteps() []ArchitectureRefactorStep {
	return []ArchitectureRefactorStep{
		{Order: 1, Title: "Keep reviewing", Detail: "Re-run Architecture Review after inventory changes."},
		{Order: 2, Title: "Document public subjects", Detail: "Use Event Catalog so ownership stays explicit before coupling grows."},
	}
}
