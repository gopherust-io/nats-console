package domain

import "github.com/nats-io/nats.go"

var (
	retentionPolicyNames = map[nats.RetentionPolicy]string{
		nats.LimitsPolicy:    "limits",
		nats.InterestPolicy:  "interest",
		nats.WorkQueuePolicy: "workqueue",
	}
	storageTypeNames = map[nats.StorageType]string{
		nats.FileStorage:   "file",
		nats.MemoryStorage: "memory",
	}
	discardPolicyNames = map[nats.DiscardPolicy]string{
		nats.DiscardOld: "old",
		nats.DiscardNew: "new",
	}
	storeCompressionNames = map[nats.StoreCompression]string{
		nats.NoCompression: "none",
		nats.S2Compression: "s2",
	}
	deliverPolicyNames = map[nats.DeliverPolicy]string{
		nats.DeliverAllPolicy:             "all",
		nats.DeliverLastPolicy:            "last",
		nats.DeliverNewPolicy:             "new",
		nats.DeliverByStartSequencePolicy: "by_start_sequence",
		nats.DeliverByStartTimePolicy:     "by_start_time",
		nats.DeliverLastPerSubjectPolicy:  "last_per_subject",
	}
	ackPolicyNames = map[nats.AckPolicy]string{
		nats.AckNonePolicy:     "none",
		nats.AckAllPolicy:      "all",
		nats.AckExplicitPolicy: "explicit",
	}
	replayPolicyNames = map[nats.ReplayPolicy]string{
		nats.ReplayInstantPolicy:  "instant",
		nats.ReplayOriginalPolicy: "original",
	}
)

func enumString(v any) string {
	switch typed := v.(type) {
	case nats.RetentionPolicy:
		if name, ok := retentionPolicyNames[typed]; ok {
			return name
		}
	case nats.StorageType:
		if name, ok := storageTypeNames[typed]; ok {
			return name
		}
	case nats.DiscardPolicy:
		if name, ok := discardPolicyNames[typed]; ok {
			return name
		}
	case nats.StoreCompression:
		if name, ok := storeCompressionNames[typed]; ok {
			return name
		}
	case nats.DeliverPolicy:
		if name, ok := deliverPolicyNames[typed]; ok {
			return name
		}
	case nats.AckPolicy:
		if name, ok := ackPolicyNames[typed]; ok {
			return name
		}
	case nats.ReplayPolicy:
		if name, ok := replayPolicyNames[typed]; ok {
			return name
		}
	}
	return ""
}

func StorageTypeString(v nats.StorageType) string {
	if name, ok := storageTypeNames[v]; ok {
		return name
	}
	return "file"
}
