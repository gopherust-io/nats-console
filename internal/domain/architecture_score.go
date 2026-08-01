package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Architecture score factor ids (stable for i18n / persistence).
const (
	ArchScoreFactorNaming            = "naming"
	ArchScoreFactorConsumerExplosion = "consumer_explosion"
	ArchScoreFactorDuplicateEvents   = "duplicate_events"
	ArchScoreFactorPayloadSize       = "payload_size"
	ArchScoreFactorLatency           = "latency"
	ArchScoreFactorCoupling          = "coupling"
)

const (
	ArchScoreSignPlus  = "plus"
	ArchScoreSignMinus = "minus"
)

const (
	archScoreMax                 = 100
	archScorePenaltyCritical     = 12
	archScorePenaltyWarn         = 6
	archScorePenaltyInfo         = 2
	archScoreCapPerKind          = 24
	archScoreNamingCleanBonus    = 3
	archScoreLatencyImproveBonus = 4
	archScoreLatencyWorsenPen    = 4
	archScoreLatencyImproveRatio = 0.10
)

// ArchitectureScoreFactor is one +/- contribution shown on the score card.
type ArchitectureScoreFactor struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Sign  string `json:"sign"`
	Delta int    `json:"delta"`
}

// ArchitectureScoreTrendPoint is one day or month on the trend chart.
type ArchitectureScoreTrendPoint struct {
	Period string `json:"period"` // day (YYYY-MM-DD) or month (YYYY-MM)
	Kind   string `json:"kind"`   // day | month
	Score  int    `json:"score"`
}

// ArchitectureScorePrior holds the previous day's score inputs for deltas.
// goalign:ignore // trailing bool padding is unavoidable
type ArchitectureScorePrior struct {
	Score  int
	AvgLag float64
	HasLag bool
}

// ArchitectureScoreHints carries optional live signals beyond inventory analysis.
// goalign:ignore // trailing bool padding is unavoidable
type ArchitectureScoreHints struct {
	Prior  *ArchitectureScorePrior
	AvgLag float64
	HasLag bool
}

// ArchitectureScoreDailyRow is one persisted daily score (storage / trend).
type ArchitectureScoreDailyRow struct {
	ScoreDay   time.Time
	CapturedAt time.Time
	ClusterID  string
	Factors    []ArchitectureScoreFactor
	Score      int
	AvgLag     float64
}

// ArchitectureScoreSnapshot is the API payload for Architecture Score.
type ArchitectureScoreSnapshot struct {
	CapturedAt time.Time                     `json:"capturedAt,omitzero"`
	Verdict    string                        `json:"verdict"`
	Factors    []ArchitectureScoreFactor     `json:"factors"`
	Trend      []ArchitectureScoreTrendPoint `json:"trend"`
	Score      int                           `json:"score"`
	MaxScore   int                           `json:"maxScore"`
	AvgLag     float64                       `json:"avgLag,omitempty"`
	Demo       bool                          `json:"demo,omitempty"`
}

// ComputeArchitectureScore builds a 0–100 score from inventory findings + lag hints.
func ComputeArchitectureScore(inputs []EventArchitectureInput, hints ArchitectureScoreHints) ArchitectureScoreSnapshot {
	review := AnalyzeEventArchitecture(inputs)
	genome := analyzeGenomeForScore(inputs)

	factors := make([]ArchitectureScoreFactor, 0, 6)
	score := archScoreMax

	apply := func(f ArchitectureScoreFactor) {
		if f.Delta == 0 {
			return
		}
		factors = append(factors, f)
		score += f.Delta
	}

	namingFindings, duplicateFindings := splitNamingAndDuplicates(review.Problems)
	namingPenalty := cappedPenalty(namingFindings)
	if namingPenalty > 0 {
		apply(ArchitectureScoreFactor{
			ID:    ArchScoreFactorNaming,
			Label: "Naming inconsistent",
			Delta: -namingPenalty,
			Sign:  ArchScoreSignMinus,
		})
	} else {
		apply(ArchitectureScoreFactor{
			ID:    ArchScoreFactorNaming,
			Label: "Better naming",
			Delta: archScoreNamingCleanBonus,
			Sign:  ArchScoreSignPlus,
		})
	}

	dupCount := genome.Totals.Duplicates
	if dupCount == 0 {
		dupCount = len(duplicateFindings)
	}
	if dupCount > 0 {
		pen := minInt(dupCount*archScorePenaltyInfo, archScoreCapPerKind)
		apply(ArchitectureScoreFactor{
			ID:    ArchScoreFactorDuplicateEvents,
			Label: "Duplicate events",
			Delta: -pen,
			Sign:  ArchScoreSignMinus,
		})
	}

	consumerFindings := findingsOfKind(review.Problems, ArchKindTooManyConsumers)
	if pen := cappedPenalty(consumerFindings); pen > 0 {
		apply(ArchitectureScoreFactor{
			ID:    ArchScoreFactorConsumerExplosion,
			Label: "Consumer explosion",
			Delta: -pen,
			Sign:  ArchScoreSignMinus,
		})
	}

	payloadFindings := findingsOfKind(review.Problems, ArchKindPayloadTooLarge)
	if pen := cappedPenalty(payloadFindings); pen > 0 {
		apply(ArchitectureScoreFactor{
			ID:    ArchScoreFactorPayloadSize,
			Label: "Growing payload sizes",
			Delta: -pen,
			Sign:  ArchScoreSignMinus,
		})
	}

	coupling := append(
		findingsOfKind(review.Problems, ArchKindCircularDependency),
		findingsOfKind(review.Problems, ArchKindTightCoupling)...,
	)
	if pen := cappedPenalty(coupling); pen > 0 {
		apply(ArchitectureScoreFactor{
			ID:    ArchScoreFactorCoupling,
			Label: "Tight coupling",
			Delta: -pen,
			Sign:  ArchScoreSignMinus,
		})
	}

	if hints.HasLag && hints.Prior != nil && hints.Prior.HasLag && hints.Prior.AvgLag > 0 {
		improved := hints.AvgLag <= hints.Prior.AvgLag*(1-archScoreLatencyImproveRatio)
		worsened := hints.AvgLag >= hints.Prior.AvgLag*(1+archScoreLatencyImproveRatio)
		switch {
		case improved:
			apply(ArchitectureScoreFactor{
				ID:    ArchScoreFactorLatency,
				Label: "Better latency",
				Delta: archScoreLatencyImproveBonus,
				Sign:  ArchScoreSignPlus,
			})
		case worsened:
			apply(ArchitectureScoreFactor{
				ID:    ArchScoreFactorLatency,
				Label: "Worse latency",
				Delta: -archScoreLatencyWorsenPen,
				Sign:  ArchScoreSignMinus,
			})
		}
	}

	if score > archScoreMax {
		score = archScoreMax
	}
	if score < 0 {
		score = 0
	}

	sort.SliceStable(factors, func(i, j int) bool {
		if factors[i].Sign != factors[j].Sign {
			return factors[i].Sign == ArchScoreSignPlus
		}
		return factors[i].ID < factors[j].ID
	})

	return ArchitectureScoreSnapshot{
		Score:    score,
		MaxScore: archScoreMax,
		Verdict:  scoreVerdict(score),
		Factors:  factors,
		Trend:    []ArchitectureScoreTrendPoint{},
		AvgLag:   hints.AvgLag,
	}
}

// AttachArchitectureScoreTrend fills daily + monthly trend points from persisted rows.
func AttachArchitectureScoreTrend(snap ArchitectureScoreSnapshot, rows []ArchitectureScoreDailyRow) ArchitectureScoreSnapshot {
	daily := make([]ArchitectureScoreTrendPoint, 0, len(rows))
	monthSum := map[string]int{}
	monthN := map[string]int{}
	for _, r := range rows {
		day := r.ScoreDay.UTC().Format("2006-01-02")
		daily = append(daily, ArchitectureScoreTrendPoint{Period: day, Kind: "day", Score: r.Score})
		month := r.ScoreDay.UTC().Format("2006-01")
		monthSum[month] += r.Score
		monthN[month]++
	}
	months := make([]string, 0, len(monthSum))
	for m := range monthSum {
		months = append(months, m)
	}
	sort.Strings(months)
	monthly := make([]ArchitectureScoreTrendPoint, 0, len(months))
	for _, m := range months {
		n := monthN[m]
		if n <= 0 {
			continue
		}
		monthly = append(monthly, ArchitectureScoreTrendPoint{
			Period: m,
			Kind:   "month",
			Score:  monthSum[m] / n,
		})
	}
	// Prefer monthly for the card when we have enough history; else daily sparkline.
	if len(monthly) >= 2 {
		snap.Trend = monthly
	} else {
		snap.Trend = daily
	}
	return snap
}

// DemoArchitectureScoreSnapshot returns the product-brief showcase (92/100).
func DemoArchitectureScoreSnapshot() ArchitectureScoreSnapshot {
	now := time.Now().UTC()
	factors := []ArchitectureScoreFactor{
		{ID: ArchScoreFactorNaming, Label: "Better naming", Delta: 3, Sign: ArchScoreSignPlus},
		{ID: ArchScoreFactorLatency, Label: "Better latency", Delta: 4, Sign: ArchScoreSignPlus},
		{ID: ArchScoreFactorConsumerExplosion, Label: "Consumer explosion", Delta: -6, Sign: ArchScoreSignMinus},
		{ID: ArchScoreFactorDuplicateEvents, Label: "Duplicate events", Delta: -2, Sign: ArchScoreSignMinus},
		{ID: ArchScoreFactorPayloadSize, Label: "Growing payload sizes", Delta: -6, Sign: ArchScoreSignMinus},
	}
	trend := make([]ArchitectureScoreTrendPoint, 0, 6)
	scores := []int{84, 86, 88, 90, 91, 92}
	for i, s := range scores {
		month := now.AddDate(0, i-5, 0).Format("2006-01")
		trend = append(trend, ArchitectureScoreTrendPoint{Period: month, Kind: "month", Score: s})
	}
	return ArchitectureScoreSnapshot{
		CapturedAt: now,
		Score:      92,
		MaxScore:   archScoreMax,
		Verdict:    "Architecture score 92/100 — naming and latency improved; watch consumer fan-out and payloads",
		Factors:    factors,
		Trend:      trend,
		Demo:       true,
	}
}

// AverageConsumerLag returns mean lag across samples (0 if empty).
func AverageConsumerLag(samples []IncidentConsumerSample) (avg float64, ok bool) {
	if len(samples) == 0 {
		return 0, false
	}
	var sum float64
	for _, s := range samples {
		sum += s.Lag
	}
	return sum / float64(len(samples)), true
}

func scoreVerdict(score int) string {
	switch {
	case score >= 90:
		return fmt.Sprintf("Architecture score %d/100 — healthy with room to polish", score)
	case score >= 75:
		return fmt.Sprintf("Architecture score %d/100 — needs attention on a few factors", score)
	default:
		return fmt.Sprintf("Architecture score %d/100 — at risk; address high-impact factors", score)
	}
}

func cappedPenalty(findings []EventArchitectureFinding) int {
	pen := 0
	for _, f := range findings {
		switch f.Severity {
		case ArchSeverityCritical:
			pen += archScorePenaltyCritical
		case ArchSeverityWarn:
			pen += archScorePenaltyWarn
		default:
			pen += archScorePenaltyInfo
		}
	}
	return minInt(pen, archScoreCapPerKind)
}

func findingsOfKind(problems []EventArchitectureFinding, kind string) []EventArchitectureFinding {
	out := make([]EventArchitectureFinding, 0)
	for _, p := range problems {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

func splitNamingAndDuplicates(problems []EventArchitectureFinding) (naming, duplicates []EventArchitectureFinding) {
	for _, p := range problems {
		if p.Kind != ArchKindNamingInconsistent {
			continue
		}
		if isGenomeDuplicateFinding(p) {
			duplicates = append(duplicates, p)
			continue
		}
		naming = append(naming, p)
	}
	return naming, duplicates
}

func isGenomeDuplicateFinding(p EventArchitectureFinding) bool {
	if strings.HasPrefix(p.Title, "Semantic duplicate") {
		return true
	}
	for _, e := range p.Evidence {
		if strings.HasPrefix(e, "genome=") {
			return true
		}
	}
	return false
}

func analyzeGenomeForScore(inputs []EventArchitectureInput) EventGenomeSnapshot {
	genomeIn := make([]EventGenomeInput, 0, len(inputs))
	for _, s := range inputs {
		gi := EventGenomeInput{Name: s.Name, Subjects: s.Subjects}
		for _, c := range s.Consumers {
			gi.Consumers = append(gi.Consumers, EventGenomeConsumerInput(c))
		}
		genomeIn = append(genomeIn, gi)
	}
	return AnalyzeEventGenome(genomeIn)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatArchitectureScoreFactorsJSON is used by storage; factors must be non-nil for JSON.
func NormalizeArchitectureScoreFactors(factors []ArchitectureScoreFactor) []ArchitectureScoreFactor {
	if factors == nil {
		return []ArchitectureScoreFactor{}
	}
	out := make([]ArchitectureScoreFactor, 0, len(factors))
	for _, f := range factors {
		if commonstrings.IsEmpty(f.ID) {
			continue
		}
		out = append(out, f)
	}
	return out
}
