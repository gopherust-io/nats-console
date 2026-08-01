package domain

import "testing"

func TestEvaluateSlowConsumer(t *testing.T) {
	t.Parallel()
	thr := SlowConsumerThresholds{PendingThreshold: 100, LagThreshold: 50, AckPendingRatio: 0.9}

	slow, reasons := EvaluateSlowConsumer(99, 0, 0, 0, thr)
	if slow || len(reasons) != 0 {
		t.Fatalf("expected not slow, got %v %v", slow, reasons)
	}

	slow, reasons = EvaluateSlowConsumer(100, 0, 0, 0, thr)
	if !slow || len(reasons) != 1 || reasons[0] != SlowReasonPending {
		t.Fatalf("pending: got %v %v", slow, reasons)
	}

	slow, reasons = EvaluateSlowConsumer(0, 50, 0, 0, thr)
	if !slow || len(reasons) != 1 || reasons[0] != SlowReasonLag {
		t.Fatalf("lag: got %v %v", slow, reasons)
	}

	slow, reasons = EvaluateSlowConsumer(0, 0, 90, 100, thr)
	if !slow || len(reasons) != 1 || reasons[0] != SlowReasonAckPending {
		t.Fatalf("ack: got %v %v", slow, reasons)
	}

	slow, _ = EvaluateSlowConsumer(0, 0, 1000, 0, thr)
	if slow {
		t.Fatal("max ack pending 0 should disable ack check")
	}
}

func TestApplySlowConsumerFlags(t *testing.T) {
	t.Parallel()
	info := &ConsumerInfo{
		NumPending:    2000,
		NumAckPending: 0,
		Delivered:     &SequenceInfoDTO{StreamSeq: 100},
		Config:        ConsumerConfigDTO{MaxAckPending: 1000},
	}
	ApplySlowConsumerFlags(info, 3100, SlowConsumerThresholds{})
	if !info.SlowConsumer {
		t.Fatal("expected slow")
	}
	if len(info.SlowReasons) < 2 {
		t.Fatalf("expected pending+lag reasons, got %v", info.SlowReasons)
	}
}
