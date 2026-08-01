package domain

import (
	"fmt"
	"testing"
	"time"
)

func TestComputeArchitectureScore_CleanBonus(t *testing.T) {
	snap := ComputeArchitectureScore(nil, ArchitectureScoreHints{})
	if snap.Score != 100 {
		t.Fatalf("score=%d want 100", snap.Score)
	}
	foundNaming := false
	for _, f := range snap.Factors {
		if f.ID == ArchScoreFactorNaming && f.Sign == ArchScoreSignPlus {
			foundNaming = true
		}
	}
	if !foundNaming {
		t.Fatal("expected better naming bonus")
	}
}

func TestComputeArchitectureScore_ConsumerExplosion(t *testing.T) {
	consumers := make([]EventArchitectureConsumerInput, 0, 14)
	for i := range 14 {
		consumers = append(consumers, EventArchitectureConsumerInput{Name: fmt.Sprintf("c%d", i)})
	}
	inputs := []EventArchitectureInput{{
		Name:      "ORDERS",
		Subjects:  []string{"orders.created"},
		Consumers: consumers,
	}}
	snap := ComputeArchitectureScore(inputs, ArchitectureScoreHints{})
	if snap.Score >= 100 {
		t.Fatalf("expected penalty, score=%d", snap.Score)
	}
	var found bool
	for _, f := range snap.Factors {
		if f.ID == ArchScoreFactorConsumerExplosion && f.Sign == ArchScoreSignMinus {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing consumer explosion factor: %+v", snap.Factors)
	}
}

func TestComputeArchitectureScore_LatencyImprove(t *testing.T) {
	hints := ArchitectureScoreHints{
		AvgLag: 50,
		HasLag: true,
		Prior:  &ArchitectureScorePrior{AvgLag: 100, HasLag: true},
	}
	snap := ComputeArchitectureScore(nil, hints)
	var found bool
	for _, f := range snap.Factors {
		if f.ID == ArchScoreFactorLatency && f.Sign == ArchScoreSignPlus {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected better latency: %+v", snap.Factors)
	}
}

func TestDemoArchitectureScoreSnapshot(t *testing.T) {
	demo := DemoArchitectureScoreSnapshot()
	if demo.Score != 92 || !demo.Demo {
		t.Fatalf("demo=%+v", demo)
	}
	if len(demo.Factors) < 5 {
		t.Fatalf("factors=%d", len(demo.Factors))
	}
	if len(demo.Trend) < 2 {
		t.Fatal("expected multi-month trend")
	}
}

func TestAttachArchitectureScoreTrend_Monthly(t *testing.T) {
	rows := []ArchitectureScoreDailyRow{
		{ScoreDay: mustDay("2026-01-01"), Score: 80},
		{ScoreDay: mustDay("2026-01-15"), Score: 90},
		{ScoreDay: mustDay("2026-02-01"), Score: 92},
	}
	snap := AttachArchitectureScoreTrend(ArchitectureScoreSnapshot{Score: 92}, rows)
	if len(snap.Trend) != 2 {
		t.Fatalf("trend=%+v", snap.Trend)
	}
	if snap.Trend[0].Kind != "month" || snap.Trend[0].Score != 85 {
		t.Fatalf("jan=%+v", snap.Trend[0])
	}
}

func mustDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
