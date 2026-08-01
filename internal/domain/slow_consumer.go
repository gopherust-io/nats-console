package domain

const (
	DefaultSlowConsumerPendingThreshold = uint64(1000)
	DefaultSlowConsumerLagThreshold     = uint64(1000)
	DefaultSlowConsumerAckPendingRatio  = 0.9

	SlowReasonPending    = "pending"
	SlowReasonLag        = "lag"
	SlowReasonAckPending = "ack_pending"
)

// SlowConsumerThresholds mirrors the nats library WatchSlowConsumer defaults.
type SlowConsumerThresholds struct {
	PendingThreshold uint64
	LagThreshold     uint64
	AckPendingRatio  float64
}

func (t SlowConsumerThresholds) WithDefaults() SlowConsumerThresholds {
	out := t
	if out.PendingThreshold == 0 {
		out.PendingThreshold = DefaultSlowConsumerPendingThreshold
	}
	if out.LagThreshold == 0 {
		out.LagThreshold = DefaultSlowConsumerLagThreshold
	}
	if out.AckPendingRatio <= 0 {
		out.AckPendingRatio = DefaultSlowConsumerAckPendingRatio
	}
	return out
}

// ConsumerLagMessages returns max(0, streamLastSeq − deliveredStreamSeq).
func ConsumerLagMessages(streamLastSeq, deliveredStreamSeq uint64) uint64 {
	if streamLastSeq <= deliveredStreamSeq {
		return 0
	}
	return streamLastSeq - deliveredStreamSeq
}

// EvaluateSlowConsumer returns whether thresholds are met and why.
func EvaluateSlowConsumer(pending, lag uint64, ackPending, maxAckPending int, thr SlowConsumerThresholds) (slow bool, reasons []string) {
	thr = thr.WithDefaults()
	if pending >= thr.PendingThreshold {
		reasons = append(reasons, SlowReasonPending)
	}
	if lag >= thr.LagThreshold {
		reasons = append(reasons, SlowReasonLag)
	}
	if maxAckPending > 0 {
		limit := max(int(float64(maxAckPending)*thr.AckPendingRatio), 1)
		if ackPending >= limit {
			reasons = append(reasons, SlowReasonAckPending)
		}
	}
	return len(reasons) > 0, reasons
}

// ApplySlowConsumerFlags sets SlowConsumer / SlowReasons on info using stream tip + thresholds.
func ApplySlowConsumerFlags(info *ConsumerInfo, streamLastSeq uint64, thr SlowConsumerThresholds) {
	if info == nil {
		return
	}
	var delivered uint64
	if info.Delivered != nil {
		delivered = info.Delivered.StreamSeq
	}
	lag := ConsumerLagMessages(streamLastSeq, delivered)
	slow, reasons := EvaluateSlowConsumer(info.NumPending, lag, info.NumAckPending, info.Config.MaxAckPending, thr)
	info.SlowConsumer = slow
	info.SlowReasons = reasons
}
