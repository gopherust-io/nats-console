package domain

import "github.com/nats-io/nats.go"

var (
	retentionPolicyNames = map[nats.RetentionPolicy]string{
		nats.LimitsPolicy:      "limits",
		nats.InterestPolicy:    "interest",
		nats.WorkQueuePolicy:   "workqueue",
	}
	storageTypeNames = map[nats.StorageType]string{
		nats.FileStorage:   "file",
		nats.MemoryStorage: "memory",
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
	case nats.DeliverPolicy:
		if name, ok := deliverPolicyNames[typed]; ok {
			return name
		}
	case nats.AckPolicy:
		if name, ok := ackPolicyNames[typed]; ok {
			return name
		}
	}
	return ""
}
