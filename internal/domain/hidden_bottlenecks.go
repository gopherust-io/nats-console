package domain

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Hidden bottleneck finding kinds.
const (
	BottleneckKindCorrelatedPayloadLag   = "correlated_payload_lag"
	BottleneckKindScheduleLagSpike       = "schedule_lag_spike"
	BottleneckKindSchedulePayloadGrowth  = "schedule_payload_growth"
	BottleneckKindScheduleProcessingSlow = "schedule_processing_slow"
)

// Hidden bottleneck severities / verdicts (reuse architecture vocabulary).
const (
	BottleneckSeverityInfo     = ArchSeverityInfo
	BottleneckSeverityWarn     = ArchSeverityWarn
	BottleneckSeverityCritical = ArchSeverityCritical

	BottleneckVerdictHealthy        = ArchVerdictHealthy
	BottleneckVerdictNeedsAttention = ArchVerdictNeedsAttention
	BottleneckVerdictAtRisk         = ArchVerdictAtRisk
)

const (
	bottleneckMinDistinctWeeks   = 2
	bottleneckLagRatio           = 1.5
	bottleneckLagAbsFloor        = 50.0
	bottleneckPayloadRatio       = 1.25
	bottleneckPayloadAbsFloor    = 256.0
	bottleneckProcessingRatio    = 1.5
	bottleneckProcessingAbsFloor = 50.0
	bottleneckMaxFindings        = 20
)

// BottleneckHourBucket is one compact hourly rollup row.
type BottleneckHourBucket struct {
	BucketHour      time.Time
	AvgProcessingMs *float64
	ClusterID       string
	StreamName      string
	ConsumerName    string
	AvgLag          float64
	MaxLag          float64
	AvgPayloadBytes float64
	Samples         int
}

// HiddenBottleneckFinding is one mined schedule/correlation insight.
type HiddenBottleneckFinding struct {
	Kind       string   `json:"kind"`
	Severity   string   `json:"severity"`
	Title      string   `json:"title"`
	Suggestion string   `json:"suggestion"`
	Stream     string   `json:"stream,omitempty"`
	Consumer   string   `json:"consumer,omitempty"`
	Schedule   string   `json:"schedule,omitempty"`
	Evidence   []string `json:"evidence"`
	Weekday    int      `json:"weekday,omitempty"`
	HourUTC    int      `json:"hourUtc,omitempty"`
}

// HiddenBottleneckTotals counts findings by kind and severity.
type HiddenBottleneckTotals struct {
	ByKind   map[string]int `json:"byKind,omitempty"`
	Problems int            `json:"problems"`
	Critical int            `json:"critical"`
	Warn     int            `json:"warn"`
	Info     int            `json:"info"`
}

// HiddenBottleneckSnapshot is the API payload for Find Hidden Bottlenecks.
type HiddenBottleneckSnapshot struct {
	CapturedAt  time.Time                 `json:"capturedAt,omitzero"`
	From        time.Time                 `json:"from,omitzero"`
	To          time.Time                 `json:"to,omitzero"`
	Verdict     string                    `json:"verdict"`
	Findings    []HiddenBottleneckFinding `json:"findings"`
	Suggestions []string                  `json:"suggestions"`
	Totals      HiddenBottleneckTotals    `json:"totals"`
	Demo        bool                      `json:"demo,omitempty"`
}

type scheduleCellKey struct {
	stream   string
	consumer string
	weekday  time.Weekday
	hour     int
}

type scheduleCell struct {
	weekNumbers map[int]struct{}
	lags        []float64
	payloads    []float64
	processing  []float64
	bucketCount int
}

// DiscoverHiddenBottlenecks mines recurring weekday×hour anomalies and correlations.
func DiscoverHiddenBottlenecks(buckets []BottleneckHourBucket) HiddenBottleneckSnapshot {
	out := HiddenBottleneckSnapshot{
		Findings:    []HiddenBottleneckFinding{},
		Suggestions: []string{},
		Verdict:     BottleneckVerdictHealthy,
		Totals: HiddenBottleneckTotals{
			ByKind: map[string]int{},
		},
	}
	if len(buckets) == 0 {
		return out
	}

	cells := buildScheduleCells(buckets)
	var findings []HiddenBottleneckFinding

	// Consumer cells (non-empty durable).
	for key, cell := range cells {
		if commonstrings.IsEmpty(key.consumer) {
			continue
		}
		if len(cell.weekNumbers) < bottleneckMinDistinctWeeks {
			continue
		}
		schedule := formatSchedule(key.weekday, key.hour)
		lagBase := baselineOtherSchedules(cells, key, true, func(c *scheduleCell) []float64 { return c.lags })
		payBase := streamPayloadBaseline(cells, key.stream, key.weekday, key.hour)
		procBase := baselineOtherSchedules(cells, key, true, func(c *scheduleCell) []float64 { return c.processing })

		lagCur := median(cell.lags)
		payCur := median(cell.payloads)
		procCur := median(cell.processing)

		lagElevated := elevated(lagCur, lagBase, bottleneckLagRatio, bottleneckLagAbsFloor)
		payElevated := elevated(payCur, payBase, bottleneckPayloadRatio, bottleneckPayloadAbsFloor)
		procElevated := elevated(procCur, procBase, bottleneckProcessingRatio, bottleneckProcessingAbsFloor)

		if lagElevated && payElevated {
			findings = append(findings, HiddenBottleneckFinding{
				Kind:     BottleneckKindCorrelatedPayloadLag,
				Severity: severityForRatio(lagCur, lagBase, payCur, payBase),
				Title: fmt.Sprintf("%s %s slows when %s payload grows",
					schedule, key.consumer, key.stream),
				Evidence: []string{
					"schedule=" + schedule,
					"consumer=" + key.consumer,
					"stream=" + key.stream,
					fmt.Sprintf("lag=%.0f (baseline %.0f)", lagCur, lagBase),
					fmt.Sprintf("avgPayload=%.0fB (baseline %.0fB)", payCur, payBase),
					fmt.Sprintf("weeks=%d", len(cell.weekNumbers)),
				},
				Suggestion: "Inspect producers that enlarge payloads on this schedule; consider compression, schema trimming, or scaling the consumer before the window.",
				Stream:     key.stream,
				Consumer:   key.consumer,
				Schedule:   schedule,
				Weekday:    int(key.weekday),
				HourUTC:    key.hour,
			})
			continue
		}
		if lagElevated {
			findings = append(findings, HiddenBottleneckFinding{
				Kind:     BottleneckKindScheduleLagSpike,
				Severity: BottleneckSeverityWarn,
				Title:    fmt.Sprintf("%s: %s lag spikes on a schedule", schedule, key.consumer),
				Evidence: []string{
					"schedule=" + schedule,
					"consumer=" + key.consumer,
					"stream=" + key.stream,
					fmt.Sprintf("lag=%.0f (baseline %.0f)", lagCur, lagBase),
					fmt.Sprintf("weeks=%d", len(cell.weekNumbers)),
				},
				Suggestion: "Look for batch jobs, cron publishers, or downstream slowdowns that recur at this weekday/hour.",
				Stream:     key.stream,
				Consumer:   key.consumer,
				Schedule:   schedule,
				Weekday:    int(key.weekday),
				HourUTC:    key.hour,
			})
		}
		if procElevated {
			findings = append(findings, HiddenBottleneckFinding{
				Kind:     BottleneckKindScheduleProcessingSlow,
				Severity: BottleneckSeverityWarn,
				Title:    fmt.Sprintf("%s: %s processing latency elevated", schedule, key.consumer),
				Evidence: []string{
					"schedule=" + schedule,
					"consumer=" + key.consumer,
					fmt.Sprintf("processingMs=%.0f (baseline %.0f)", procCur, procBase),
					fmt.Sprintf("weeks=%d", len(cell.weekNumbers)),
				},
				Suggestion: "Correlate worker fingerprints with dependency latency (DB, HTTP) during this window.",
				Stream:     key.stream,
				Consumer:   key.consumer,
				Schedule:   schedule,
				Weekday:    int(key.weekday),
				HourUTC:    key.hour,
			})
		}
	}

	// Stream-only payload growth (consumer empty).
	for key, cell := range cells {
		if !commonstrings.IsEmpty(key.consumer) {
			continue
		}
		if len(cell.weekNumbers) < bottleneckMinDistinctWeeks {
			continue
		}
		payBase := streamPayloadBaseline(cells, key.stream, key.weekday, key.hour)
		payCur := median(cell.payloads)
		if !elevated(payCur, payBase, bottleneckPayloadRatio, bottleneckPayloadAbsFloor) {
			continue
		}
		// Skip if a correlated finding already covers this stream+schedule.
		schedule := formatSchedule(key.weekday, key.hour)
		dup := false
		for _, f := range findings {
			if f.Kind == BottleneckKindCorrelatedPayloadLag && f.Stream == key.stream && f.Schedule == schedule {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		findings = append(findings, HiddenBottleneckFinding{
			Kind:     BottleneckKindSchedulePayloadGrowth,
			Severity: BottleneckSeverityInfo,
			Title:    fmt.Sprintf("%s: %s average payload grows", schedule, key.stream),
			Evidence: []string{
				"schedule=" + schedule,
				"stream=" + key.stream,
				fmt.Sprintf("avgPayload=%.0fB (baseline %.0fB)", payCur, payBase),
				fmt.Sprintf("weeks=%d", len(cell.weekNumbers)),
			},
			Suggestion: "Identify which subjects enlarge on this schedule; trim fields or split large events.",
			Stream:     key.stream,
			Schedule:   schedule,
			Weekday:    int(key.weekday),
			HourUTC:    key.hour,
		})
	}

	sort.SliceStable(findings, func(i, j int) bool {
		ki, kj := kindRank(findings[i].Kind), kindRank(findings[j].Kind)
		if ki != kj {
			return ki > kj
		}
		si, sj := severityRank(findings[i].Severity), severityRank(findings[j].Severity)
		if si != sj {
			return si > sj
		}
		return findings[i].Title < findings[j].Title
	})
	if len(findings) > bottleneckMaxFindings {
		findings = findings[:bottleneckMaxFindings]
	}

	seenSug := map[string]struct{}{}
	for _, f := range findings {
		out.Totals.Problems++
		out.Totals.ByKind[f.Kind]++
		switch f.Severity {
		case BottleneckSeverityCritical:
			out.Totals.Critical++
		case BottleneckSeverityWarn:
			out.Totals.Warn++
		default:
			out.Totals.Info++
		}
		s := strings.TrimSpace(f.Suggestion)
		if commonstrings.IsEmpty(s) {
			continue
		}
		if _, ok := seenSug[s]; ok {
			continue
		}
		seenSug[s] = struct{}{}
		out.Suggestions = append(out.Suggestions, s)
	}
	out.Findings = findings

	switch {
	case out.Totals.Critical > 0:
		out.Verdict = BottleneckVerdictAtRisk
	case out.Totals.Warn > 0 || out.Totals.Info > 0:
		out.Verdict = BottleneckVerdictNeedsAttention
	default:
		out.Verdict = BottleneckVerdictHealthy
	}
	return out
}

func kindRank(kind string) int {
	switch kind {
	case BottleneckKindCorrelatedPayloadLag:
		return 4
	case BottleneckKindScheduleProcessingSlow:
		return 3
	case BottleneckKindScheduleLagSpike:
		return 2
	default:
		return 1
	}
}

func severityForRatio(lagCur, lagBase, payCur, payBase float64) string {
	lagR := safeRatio(lagCur, lagBase)
	payR := safeRatio(payCur, payBase)
	if lagR >= 3 || payR >= 2.5 {
		return BottleneckSeverityCritical
	}
	if lagR >= 2 || payR >= 1.8 {
		return BottleneckSeverityWarn
	}
	return BottleneckSeverityInfo
}

func safeRatio(cur, base float64) float64 {
	if base <= 0 {
		if cur > 0 {
			return math.Inf(1)
		}
		return 1
	}
	return cur / base
}

func elevated(cur, base, ratio, absFloor float64) bool {
	if cur < absFloor {
		return false
	}
	if base <= 0 {
		return cur >= absFloor*ratio
	}
	return cur >= base*ratio && (cur-base) >= absFloor*0.25
}

func buildScheduleCells(buckets []BottleneckHourBucket) map[scheduleCellKey]*scheduleCell {
	out := make(map[scheduleCellKey]*scheduleCell)
	for _, b := range buckets {
		h := b.BucketHour.UTC()
		key := scheduleCellKey{
			stream:   strings.TrimSpace(b.StreamName),
			consumer: strings.TrimSpace(b.ConsumerName),
			weekday:  h.Weekday(),
			hour:     h.Hour(),
		}
		if commonstrings.IsEmpty(key.stream) {
			continue
		}
		cell := out[key]
		if cell == nil {
			cell = &scheduleCell{weekNumbers: map[int]struct{}{}}
			out[key] = cell
		}
		cell.bucketCount++
		_, week := h.ISOWeek()
		// Mix year into week key so multi-year retention still works.
		cell.weekNumbers[h.Year()*100+week] = struct{}{}
		if !commonstrings.IsEmpty(key.consumer) {
			cell.lags = append(cell.lags, b.AvgLag)
		}
		if b.AvgPayloadBytes > 0 {
			cell.payloads = append(cell.payloads, b.AvgPayloadBytes)
		}
		if b.AvgProcessingMs != nil && *b.AvgProcessingMs > 0 {
			cell.processing = append(cell.processing, *b.AvgProcessingMs)
		}
	}
	return out
}

func baselineOtherSchedules(
	cells map[scheduleCellKey]*scheduleCell,
	match scheduleCellKey,
	requireSameConsumer bool,
	pick func(*scheduleCell) []float64,
) float64 {
	var vals []float64
	for key, cell := range cells {
		if key.stream != match.stream {
			continue
		}
		if requireSameConsumer && key.consumer != match.consumer {
			continue
		}
		if key.weekday == match.weekday && key.hour == match.hour {
			continue
		}
		vals = append(vals, pick(cell)...)
	}
	return median(vals)
}

func streamPayloadBaseline(cells map[scheduleCellKey]*scheduleCell, stream string, weekday time.Weekday, hour int) float64 {
	var vals []float64
	for key, cell := range cells {
		if key.stream != stream {
			continue
		}
		if key.weekday == weekday && key.hour == hour {
			continue
		}
		// Prefer stream-only rows; fall back to consumer payloads for that stream.
		if !commonstrings.IsEmpty(key.consumer) && hasStreamOnlyPayload(cells, stream) {
			continue
		}
		vals = append(vals, cell.payloads...)
	}
	return median(vals)
}

func hasStreamOnlyPayload(cells map[scheduleCellKey]*scheduleCell, stream string) bool {
	for key, cell := range cells {
		if key.stream == stream && commonstrings.IsEmpty(key.consumer) && len(cell.payloads) > 0 {
			return true
		}
	}
	return false
}

func formatSchedule(weekday time.Weekday, hour int) string {
	return fmt.Sprintf("%s %02d:00 UTC", weekday.String(), hour)
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

// DemoHiddenBottlenecksSnapshot returns the Friday 18:00 / payload-doubles showcase.
func DemoHiddenBottlenecksSnapshot() HiddenBottleneckSnapshot {
	now := time.Now().UTC().Truncate(time.Hour)
	// Build synthetic history: Fridays 18:00 elevated lag+payload; other hours normal.
	var buckets []BottleneckHourBucket
	for weeksAgo := range 4 {
		// Find Friday 18:00 of each prior week.
		friday := now.AddDate(0, 0, -int(now.Weekday())-6-7*weeksAgo).Truncate(24 * time.Hour).Add(18 * time.Hour)
		if friday.Weekday() != time.Friday {
			friday = friday.AddDate(0, 0, int(time.Friday)-int(friday.Weekday()))
		}
		friday = time.Date(friday.Year(), friday.Month(), friday.Day(), 18, 0, 0, 0, time.UTC)
		proc := 2400.0
		buckets = append(buckets,
			BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "billing-worker", BucketHour: friday,
				AvgLag: 420, MaxLag: 800, AvgPayloadBytes: 8192, AvgProcessingMs: &proc, Samples: 12,
			},
			BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "", BucketHour: friday,
				AvgPayloadBytes: 8192, Samples: 12,
			},
		)
		// Baseline hours same week.
		for _, hour := range []int{10, 14} {
			t := time.Date(friday.Year(), friday.Month(), friday.Day(), hour, 0, 0, 0, time.UTC)
			p := 200.0
			buckets = append(buckets,
				BottleneckHourBucket{
					StreamName: "ORDERS", ConsumerName: "billing-worker", BucketHour: t,
					AvgLag: 40, MaxLag: 60, AvgPayloadBytes: 4096, AvgProcessingMs: &p, Samples: 12,
				},
				BottleneckHourBucket{
					StreamName: "ORDERS", ConsumerName: "", BucketHour: t,
					AvgPayloadBytes: 4096, Samples: 12,
				},
			)
		}
		// Mid-week baseline.
		wed := friday.AddDate(0, 0, -2)
		wed = time.Date(wed.Year(), wed.Month(), wed.Day(), 18, 0, 0, 0, time.UTC)
		p := 210.0
		buckets = append(buckets,
			BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "billing-worker", BucketHour: wed,
				AvgLag: 45, MaxLag: 70, AvgPayloadBytes: 4100, AvgProcessingMs: &p, Samples: 12,
			},
			BottleneckHourBucket{
				StreamName: "ORDERS", ConsumerName: "", BucketHour: wed,
				AvgPayloadBytes: 4100, Samples: 12,
			},
		)
	}
	snap := DiscoverHiddenBottlenecks(buckets)
	snap.Demo = true
	snap.CapturedAt = now
	if snap.Verdict == BottleneckVerdictHealthy {
		// Ensure demo always shows the hero finding even if thresholds drift.
		snap = HiddenBottleneckSnapshot{
			CapturedAt: now,
			Verdict:    BottleneckVerdictNeedsAttention,
			Demo:       true,
			Findings: []HiddenBottleneckFinding{{
				Kind:     BottleneckKindCorrelatedPayloadLag,
				Severity: BottleneckSeverityWarn,
				Title:    "Friday 18:00 UTC billing-worker slows when ORDERS payload grows",
				Evidence: []string{
					"schedule=Friday 18:00 UTC",
					"consumer=billing-worker",
					"stream=ORDERS",
					"lag=420 (baseline 42)",
					"avgPayload=8192B (baseline 4096B)",
					"weeks=4",
				},
				Suggestion: "Inspect producers that enlarge payloads on this schedule; consider compression, schema trimming, or scaling the consumer before the window.",
				Stream:     "ORDERS",
				Consumer:   "billing-worker",
				Schedule:   "Friday 18:00 UTC",
				Weekday:    int(time.Friday),
				HourUTC:    18,
			}},
			Suggestions: []string{
				"Inspect producers that enlarge payloads on this schedule; consider compression, schema trimming, or scaling the consumer before the window.",
			},
			Totals: HiddenBottleneckTotals{
				Problems: 1,
				Warn:     1,
				ByKind:   map[string]int{BottleneckKindCorrelatedPayloadLag: 1},
			},
		}
	}
	return snap
}
