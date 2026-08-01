package domain

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	archExportMaxStreams   = 40
	archExportMaxConsumers = 80
)

// ArchitectureInventory is the cluster graph used for architecture export.
type ArchitectureInventory struct {
	CapturedAt  time.Time
	ClusterName string
	Streams     []EventArchitectureInput
	Demo        bool
	Truncated   bool
}

// ArchitectureExportFile is one path + content inside the export zip.
type ArchitectureExportFile struct {
	Path    string
	Content string
}

// ArchitectureExportBundle is the full set of generated architecture artifacts.
type ArchitectureExportBundle struct {
	Files []ArchitectureExportFile
}

// BuildArchitectureInventory truncates large inventories for export.
func BuildArchitectureInventory(clusterName string, capturedAt time.Time, streams []EventArchitectureInput) ArchitectureInventory {
	inv := ArchitectureInventory{
		ClusterName: clusterName,
		CapturedAt:  capturedAt,
		Streams:     append([]EventArchitectureInput(nil), streams...),
	}
	if inv.CapturedAt.IsZero() {
		inv.CapturedAt = time.Now().UTC()
	}
	if commonstrings.IsEmpty(inv.ClusterName) {
		inv.ClusterName = "cluster"
	}
	sort.SliceStable(inv.Streams, func(i, j int) bool {
		return inv.Streams[i].Name < inv.Streams[j].Name
	})
	if len(inv.Streams) > archExportMaxStreams {
		inv.Streams = inv.Streams[:archExportMaxStreams]
		inv.Truncated = true
	}
	totalCons := 0
	for i := range inv.Streams {
		if totalCons >= archExportMaxConsumers {
			inv.Streams[i].Consumers = nil
			inv.Truncated = true
			continue
		}
		remain := archExportMaxConsumers - totalCons
		if len(inv.Streams[i].Consumers) > remain {
			inv.Streams[i].Consumers = inv.Streams[i].Consumers[:remain]
			inv.Truncated = true
		}
		totalCons += len(inv.Streams[i].Consumers)
	}
	return inv
}

// DemoArchitectureInventory returns a canned inventory for Docs sample export.
func DemoArchitectureInventory() ArchitectureInventory {
	inv := BuildArchitectureInventory("demo-cluster", time.Now().UTC(), []EventArchitectureInput{
		{
			Name:     "ORDERS",
			Subjects: []string{"orders.created", "orders.updated", "orders.shipped"},
			Messages: 1000,
			Bytes:    2_000_000,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "billing", FilterSubject: "orders.>"},
				{Name: "shipping", FilterSubject: "orders.shipped"},
				{Name: "analytics", FilterSubject: "orders.>"},
			},
		},
		{
			Name:     "BILLING",
			Subjects: []string{"billing.charged", "billing.refunded"},
			Messages: 400,
			Bytes:    400_000,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "ledger", FilterSubject: "billing.>"},
			},
		},
		{
			Name:     "NOTIFY",
			Subjects: []string{"notify.email", "notify.sms"},
			Messages: 200,
			Bytes:    80_000,
			Consumers: []EventArchitectureConsumerInput{
				{Name: "mailer", FilterSubject: "notify.email"},
			},
		},
	})
	inv.Demo = true
	return inv
}

// GenerateArchitectureExport builds all export files for the inventory.
// adrOverrides optionally replaces ADR file contents (path → markdown).
func GenerateArchitectureExport(inv ArchitectureInventory, adrOverrides map[string]string) ArchitectureExportBundle {
	files := []ArchitectureExportFile{
		{Path: "MANIFEST.md", Content: renderManifest(inv)},
		{Path: "c4/context.puml", Content: renderC4PlantUML(inv)},
		{Path: "c4/containers.mmd", Content: renderC4Mermaid(inv)},
		{Path: "diagrams/architecture.mmd", Content: renderMermaidFlow(inv)},
		{Path: "diagrams/architecture.puml", Content: renderPlantUML(inv)},
		{Path: "diagrams/architecture.excalidraw", Content: renderExcalidraw(inv)},
		{Path: "diagrams/architecture.drawio", Content: renderDrawIO(inv)},
		{Path: "docs/architecture.md", Content: renderArchitectureMarkdown(inv)},
		{Path: "adr/0001-jetstream-topology.md", Content: renderADRTopology(inv)},
		{Path: "adr/0002-subject-boundaries.md", Content: renderADRBoundaries(inv)},
	}
	if len(adrOverrides) > 0 {
		for i := range files {
			if override, ok := adrOverrides[files[i].Path]; ok && !commonstrings.IsEmpty(strings.TrimSpace(override)) {
				files[i].Content = override
			}
		}
	}
	return ArchitectureExportBundle{Files: files}
}

// ZipArchitectureExport writes the bundle as a zip archive.
func ZipArchitectureExport(bundle ArchitectureExportBundle) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range bundle.Files {
		fw, err := w.Create(f.Path)
		if err != nil {
			_ = w.Close()
			return nil, err
		}
		if _, err := fw.Write([]byte(f.Content)); err != nil {
			_ = w.Close()
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderManifest(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString("# Architecture export manifest\n\n")
	fmt.Fprintf(&b, "- Cluster: `%s`\n", inv.ClusterName)
	fmt.Fprintf(&b, "- Captured: %s\n", inv.CapturedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "- Streams: %d\n", len(inv.Streams))
	if inv.Demo {
		b.WriteString("- Source: sample / demo inventory\n")
	} else {
		b.WriteString("- Source: live JetStream jsz scan\n")
	}
	if inv.Truncated {
		b.WriteString("- Note: inventory truncated for export size limits\n")
	}
	b.WriteString("\n## Files\n\n")
	b.WriteString("- `c4/context.puml` — C4-PlantUML context\n")
	b.WriteString("- `c4/containers.mmd` — C4-style Mermaid container view\n")
	b.WriteString("- `diagrams/architecture.mmd` — Mermaid flowchart\n")
	b.WriteString("- `diagrams/architecture.puml` — PlantUML components\n")
	b.WriteString("- `diagrams/architecture.excalidraw` — Excalidraw scene\n")
	b.WriteString("- `diagrams/architecture.drawio` — diagrams.net / Draw.io\n")
	b.WriteString("- `docs/architecture.md` — Markdown architecture doc\n")
	b.WriteString("- `adr/0001-jetstream-topology.md` — ADR: JetStream topology\n")
	b.WriteString("- `adr/0002-subject-boundaries.md` — ADR: subject boundaries\n")
	return b.String()
}

func renderC4PlantUML(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString("@startuml\n!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml\n\n")
	fmt.Fprintf(&b, "title C4 Container — %s JetStream\n\n", sanitizeLabel(inv.ClusterName))
	b.WriteString("Person(ops, \"Operator\", \"Uses NATS Consol\")\n")
	b.WriteString("System_Boundary(js, \"JetStream\") {\n")
	for _, s := range inv.Streams {
		id := plantID("S_" + s.Name)
		fmt.Fprintf(&b, "  Container(%s, \"%s\", \"JetStream stream\", \"%s\")\n",
			id, sanitizeLabel(s.Name), sanitizeLabel(strings.Join(clipSubjects(s.Subjects, 3), ", ")))
	}
	b.WriteString("}\n")
	b.WriteString("Rel(ops, js, \"Manages via Consol\")\n")
	for _, s := range inv.Streams {
		sid := plantID("S_" + s.Name)
		for _, c := range s.Consumers {
			cid := plantID("C_" + s.Name + "_" + c.Name)
			fmt.Fprintf(&b, "Container(%s, \"%s\", \"Consumer\", \"%s\")\n",
				cid, sanitizeLabel(c.Name), sanitizeLabel(primaryFilter(c)))
			fmt.Fprintf(&b, "Rel(%s, %s, \"consumes\")\n", cid, sid)
		}
	}
	b.WriteString("@enduml\n")
	return b.String()
}

func renderC4Mermaid(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString("%% C4-style container view (Mermaid flowchart)\n")
	b.WriteString("flowchart TB\n")
	fmt.Fprintf(&b, "  subgraph cluster[\"%s\"]\n", escapeMermaid(inv.ClusterName))
	for _, s := range inv.Streams {
		sid := mermaidID("S_" + s.Name)
		fmt.Fprintf(&b, "    %s[\"Stream: %s\"]\n", sid, escapeMermaid(s.Name))
		for _, c := range s.Consumers {
			cid := mermaidID("C_" + s.Name + "_" + c.Name)
			fmt.Fprintf(&b, "    %s[\"Consumer: %s\"]\n", cid, escapeMermaid(c.Name))
			fmt.Fprintf(&b, "    %s --> %s\n", sid, cid)
		}
	}
	b.WriteString("  end\n")
	b.WriteString("  Operator((Operator)) --> cluster\n")
	return b.String()
}

func renderMermaidFlow(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, s := range inv.Streams {
		sid := mermaidID("S_" + s.Name)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", sid, escapeMermaid(s.Name))
		for _, subj := range clipSubjects(s.Subjects, 4) {
			jid := mermaidID("J_" + s.Name + "_" + subj)
			fmt.Fprintf(&b, "  %s((\"%s\"))\n", jid, escapeMermaid(subj))
			fmt.Fprintf(&b, "  %s --> %s\n", jid, sid)
		}
		for _, c := range s.Consumers {
			cid := mermaidID("C_" + s.Name + "_" + c.Name)
			fmt.Fprintf(&b, "  %s[\"%s\"]\n", cid, escapeMermaid(c.Name))
			fmt.Fprintf(&b, "  %s --> %s\n", sid, cid)
		}
	}
	return b.String()
}

func renderPlantUML(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString("@startuml\nleft to right direction\n")
	fmt.Fprintf(&b, "title %s — JetStream components\n\n", sanitizeLabel(inv.ClusterName))
	for _, s := range inv.Streams {
		fmt.Fprintf(&b, "package \"%s\" {\n", sanitizeLabel(s.Name))
		fmt.Fprintf(&b, "  component [%s] as %s\n", sanitizeLabel(s.Name), plantID("S_"+s.Name))
		for _, c := range s.Consumers {
			fmt.Fprintf(&b, "  component [%s] as %s\n", sanitizeLabel(c.Name), plantID("C_"+s.Name+"_"+c.Name))
			fmt.Fprintf(&b, "  %s --> %s : %s\n",
				plantID("S_"+s.Name), plantID("C_"+s.Name+"_"+c.Name), sanitizeLabel(primaryFilter(c)))
		}
		b.WriteString("}\n")
	}
	b.WriteString("@enduml\n")
	return b.String()
}

func renderExcalidraw(inv ArchitectureInventory) string {
	type elem map[string]any
	elements := make([]elem, 0)
	id := 1
	nextID := func() string {
		s := strconv.Itoa(id)
		id++
		return s
	}
	x, y := 40.0, 40.0
	col := 0
	streamCenters := map[string][2]float64{}
	for _, s := range inv.Streams {
		sid := nextID()
		elements = append(elements, elem{
			"id": sid, "type": "rectangle", "x": x, "y": y, "width": 180, "height": 70,
			"angle": 0, "strokeColor": "#0f766e", "backgroundColor": "#ccfbf1",
			"fillStyle": "solid", "strokeWidth": 2, "roughness": 0, "opacity": 100,
			"groupIds": []string{}, "roundness": map[string]any{"type": 3}, "seed": id, "version": 1, "versionNonce": id,
			"isDeleted": false, "boundElements": []any{}, "updated": 1, "link": nil, "locked": false,
		})
		tid := nextID()
		elements = append(elements, elem{
			"id": tid, "type": "text", "x": x + 12, "y": y + 22, "width": 156, "height": 25,
			"text": s.Name, "originalText": s.Name, "fontSize": 16, "fontFamily": 1,
			"textAlign": "left", "verticalAlign": "top", "strokeColor": "#134e4a",
			"backgroundColor": "transparent", "fillStyle": "solid", "strokeWidth": 1,
			"roughness": 0, "opacity": 100, "angle": 0, "groupIds": []string{},
			"seed": id, "version": 1, "versionNonce": id, "isDeleted": false,
			"boundElements": nil, "updated": 1, "link": nil, "locked": false, "containerId": nil,
		})
		streamCenters[s.Name] = [2]float64{x + 90, y + 70}
		cy := y + 100
		for _, c := range s.Consumers {
			cid := nextID()
			elements = append(elements, elem{
				"id": cid, "type": "rectangle", "x": x + 20, "y": cy, "width": 140, "height": 48,
				"angle": 0, "strokeColor": "#0369a1", "backgroundColor": "#e0f2fe",
				"fillStyle": "solid", "strokeWidth": 1, "roughness": 0, "opacity": 100,
				"groupIds": []string{}, "roundness": map[string]any{"type": 3}, "seed": id, "version": 1, "versionNonce": id,
				"isDeleted": false, "boundElements": []any{}, "updated": 1, "link": nil, "locked": false,
			})
			tt := nextID()
			elements = append(elements, elem{
				"id": tt, "type": "text", "x": x + 28, "y": cy + 14, "width": 120, "height": 20,
				"text": c.Name, "originalText": c.Name, "fontSize": 14, "fontFamily": 1,
				"textAlign": "left", "verticalAlign": "top", "strokeColor": "#0c4a6e",
				"backgroundColor": "transparent", "fillStyle": "solid", "strokeWidth": 1,
				"roughness": 0, "opacity": 100, "angle": 0, "groupIds": []string{},
				"seed": id, "version": 1, "versionNonce": id, "isDeleted": false,
				"boundElements": nil, "updated": 1, "link": nil, "locked": false, "containerId": nil,
			})
			aid := nextID()
			sc := streamCenters[s.Name]
			elements = append(elements, elem{
				"id": aid, "type": "arrow", "x": sc[0], "y": sc[1],
				"width": 0, "height": cy - sc[1],
				"points":      [][]float64{{0, 0}, {0, cy - sc[1]}},
				"strokeColor": "#64748b", "backgroundColor": "transparent",
				"fillStyle": "solid", "strokeWidth": 1, "roughness": 0, "opacity": 100,
				"angle": 0, "groupIds": []string{}, "seed": id, "version": 1, "versionNonce": id,
				"isDeleted": false, "boundElements": nil, "updated": 1, "link": nil, "locked": false,
				"startBinding": nil, "endBinding": nil, "lastCommittedPoint": nil,
				"startArrowhead": nil, "endArrowhead": "arrow",
			})
			cy += 64
		}
		col++
		if col%3 == 0 {
			x = 40
			y += 320
		} else {
			x += 240
		}
	}
	doc := map[string]any{
		"type":     "excalidraw",
		"version":  2,
		"source":   "nats-consol-architecture-export",
		"elements": elements,
		"appState": map[string]any{
			"viewBackgroundColor": "#f8fafc",
			"gridSize":            nil,
		},
		"files": map[string]any{},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(raw) + "\n"
}

func renderDrawIO(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString(`<mxfile host="nats-consol" modified="` + inv.CapturedAt.UTC().Format(time.RFC3339) + `" agent="nats-consol" version="1.0">`)
	b.WriteString(`<diagram id="architecture" name="Architecture">`)
	b.WriteString(`<mxGraphModel dx="1200" dy="800" grid="1" gridSize="10" guides="1" tooltips="1" connect="1" arrows="1" fold="1" page="1" pageScale="1" pageWidth="1169" pageHeight="827"><root>`)
	b.WriteString(`<mxCell id="0"/><mxCell id="1" parent="0"/>`)
	cell := 2
	x, y := 40, 40
	col := 0
	for _, s := range inv.Streams {
		sid := cell
		cell++
		fmt.Fprintf(&b, `<mxCell id="%d" value="%s" style="rounded=1;whiteSpace=wrap;html=1;fillColor=#E6FFFA;strokeColor=#0F766E;" vertex="1" parent="1"><mxGeometry x="%d" y="%d" width="160" height="60" as="geometry"/></mxCell>`,
			sid, html.EscapeString(s.Name), x, y)
		cy := y + 90
		for _, c := range s.Consumers {
			cid := cell
			cell++
			fmt.Fprintf(&b, `<mxCell id="%d" value="%s" style="rounded=1;whiteSpace=wrap;html=1;fillColor=#E0F2FE;strokeColor=#0369A1;" vertex="1" parent="1"><mxGeometry x="%d" y="%d" width="140" height="44" as="geometry"/></mxCell>`,
				cid, html.EscapeString(c.Name), x+10, cy)
			eid := cell
			cell++
			fmt.Fprintf(&b, `<mxCell id="%d" style="edgeStyle=orthogonalEdgeStyle;rounded=0;orthogonalLoop=1;jettySize=auto;html=1;endArrow=block;" edge="1" parent="1" source="%d" target="%d"><mxGeometry relative="1" as="geometry"/></mxCell>`,
				eid, sid, cid)
			cy += 60
		}
		col++
		if col%3 == 0 {
			x = 40
			y += 280
		} else {
			x += 220
		}
	}
	b.WriteString(`</root></mxGraphModel></diagram></mxfile>`)
	b.WriteString("\n")
	return b.String()
}

func renderArchitectureMarkdown(inv ArchitectureInventory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Architecture — %s\n\n", inv.ClusterName)
	fmt.Fprintf(&b, "Generated by NATS Consol Architecture Generator at %s.\n\n", inv.CapturedAt.UTC().Format(time.RFC3339))
	if inv.Truncated {
		b.WriteString("> Inventory was truncated for export limits.\n\n")
	}
	b.WriteString("## Overview diagram\n\n```mermaid\n")
	b.WriteString(renderMermaidFlow(inv))
	b.WriteString("```\n\n## Streams\n\n")
	b.WriteString("| Stream | Subjects | Consumers | Messages | Bytes |\n| --- | --- | ---: | ---: | ---: |\n")
	for _, s := range inv.Streams {
		fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %d |\n",
			s.Name,
			strings.Join(clipSubjects(s.Subjects, 5), ", "),
			len(s.Consumers),
			s.Messages,
			s.Bytes)
	}
	b.WriteString("\n## Consumers\n\n")
	b.WriteString("| Stream | Consumer | Filter |\n| --- | --- | --- |\n")
	for _, s := range inv.Streams {
		for _, c := range s.Consumers {
			fmt.Fprintf(&b, "| `%s` | `%s` | `%s` |\n", s.Name, c.Name, primaryFilter(c))
		}
	}
	b.WriteString("\n## Related artifacts\n\n")
	b.WriteString("- C4: `c4/context.puml`, `c4/containers.mmd`\n")
	b.WriteString("- Diagrams: `diagrams/architecture.*`\n")
	b.WriteString("- ADRs: `adr/`\n")
	return b.String()
}

func renderADRTopology(inv ArchitectureInventory) string {
	var b strings.Builder
	b.WriteString("# ADR 0001: JetStream topology\n\n")
	fmt.Fprintf(&b, "- Status: Accepted\n- Date: %s\n- Cluster: %s\n\n", inv.CapturedAt.UTC().Format("2006-01-02"), inv.ClusterName)
	b.WriteString("## Context\n\n")
	fmt.Fprintf(&b, "The cluster exposes %d JetStream streams observed by NATS Consol.\n\n", len(inv.Streams))
	b.WriteString("## Decision\n\n")
	b.WriteString("Treat each stream as a bounded persistence boundary for its subject set. Consumers attach per stream with explicit filter subjects.\n\n")
	b.WriteString("### Observed streams\n\n")
	for _, s := range inv.Streams {
		fmt.Fprintf(&b, "- `%s` — subjects: %s — consumers: %d\n",
			s.Name, strings.Join(clipSubjects(s.Subjects, 6), ", "), len(s.Consumers))
	}
	b.WriteString("\n## Consequences\n\n")
	b.WriteString("- Stream ownership is the unit of retention, replication, and purge.\n")
	b.WriteString("- Cross-stream coupling should go through subjects, not shared storage.\n")
	b.WriteString("- Topology exports (this zip) should be regenerated after structural changes.\n")
	return b.String()
}

func renderADRBoundaries(inv ArchitectureInventory) string {
	prefixes := map[string][]string{}
	for _, s := range inv.Streams {
		for _, subj := range s.Subjects {
			tok := firstSubjectToken(subj)
			if commonstrings.IsEmpty(tok) {
				continue
			}
			prefixes[tok] = appendUnique(prefixes[tok], s.Name)
		}
	}
	keys := make([]string, 0, len(prefixes))
	for k := range prefixes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("# ADR 0002: Subject boundaries\n\n")
	fmt.Fprintf(&b, "- Status: Proposed\n- Date: %s\n- Cluster: %s\n\n", inv.CapturedAt.UTC().Format("2006-01-02"), inv.ClusterName)
	b.WriteString("## Context\n\n")
	b.WriteString("Subject prefixes hint at domain ownership. Multiple streams sharing a prefix may indicate tight coupling.\n\n")
	b.WriteString("## Decision\n\n")
	b.WriteString("Prefer one owning stream (or explicit bridge stream) per subject prefix family.\n\n")
	b.WriteString("### Prefix → streams\n\n")
	if len(keys) == 0 {
		b.WriteString("- (no concrete subjects observed)\n")
	}
	for _, k := range keys {
		fmt.Fprintf(&b, "- `%s.*` → %s\n", k, strings.Join(prefixes[k], ", "))
	}
	b.WriteString("\n## Consequences\n\n")
	b.WriteString("- Rename or split subjects that span unrelated streams.\n")
	b.WriteString("- Document public subjects in the Event Catalog.\n")
	return b.String()
}

func primaryFilter(c EventArchitectureConsumerInput) string {
	filters := archConsumerFilters(c)
	if len(filters) == 0 {
		return ">"
	}
	return filters[0]
}

func clipSubjects(subjects []string, n int) []string {
	out := make([]string, 0, n)
	for _, s := range subjects {
		s = strings.TrimSpace(s)
		if commonstrings.IsEmpty(s) {
			continue
		}
		out = append(out, s)
		if len(out) >= n {
			break
		}
	}
	return out
}

func appendUnique(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}

func sanitizeLabel(s string) string {
	s = strings.ReplaceAll(s, `"`, "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func escapeMermaid(s string) string {
	s = strings.ReplaceAll(s, `"`, "'")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "[", "(")
	s = strings.ReplaceAll(s, "]", ")")
	return s
}

func mermaidID(s string) string {
	var b strings.Builder
	b.WriteByte('n')
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func plantID(s string) string {
	id := mermaidID(s)
	if id == "n" {
		return "nX"
	}
	return id
}
