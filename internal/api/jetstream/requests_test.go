package jetstream

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
)

func TestStreamConfigRequestToNATSSynadiaFields(t *testing.T) {
	t.Parallel()

	allowRollup := false
	denyDelete := true
	denyPurge := true
	allowDirect := true
	discardNewPer := true
	cfg, err := (streamConfigRequest{
		Name:                 "ORDERS",
		Description:          "order events",
		Subjects:             []string{"orders.>"},
		Retention:            "interest",
		Storage:              "memory",
		Discard:              "new",
		Compression:          "s2",
		Replicas:             3,
		MaxMsgs:              1000,
		MaxBytes:             1 << 30,
		MaxAge:               int64(time.Hour),
		MaxConsumers:         10,
		MaxMsgSize:           1024,
		MaxMsgsPerSubject:    50,
		Duplicates:           int64(2 * time.Minute),
		FirstSeq:             100,
		AllowRollup:          &allowRollup,
		DenyDelete:           &denyDelete,
		DenyPurge:            &denyPurge,
		DiscardNewPerSubject: &discardNewPer,
		AllowDirect:          &allowDirect,
		AllowMsgTTL:          new(true),
		Placement: &apikit.StreamPlacementRequest{
			Cluster: "east",
			Tags:    []string{"ssd"},
		},
		Sources: []apikit.StreamSourceRequest{
			{Name: "ORIGIN", FilterSubject: "orders.*"},
		},
		SubjectTransform: &apikit.SubjectTransformRequest{Source: "in.>", Destination: "out.>"},
		RePublish:        &apikit.RePublishRequest{Destination: "audit.>", HeadersOnly: true},
		ConsumerLimits: &apikit.ConsumerLimitsRequest{
			InactiveThreshold: int64(time.Minute),
			MaxAckPending:     20,
		},
		Metadata: map[string]string{"owner": "ops"},
	}).toNATS()
	require.NoError(t, err)
	assert.Equal(t, "ORDERS", cfg.Name)
	assert.Equal(t, "order events", cfg.Description)
	assert.Equal(t, []string{"orders.>"}, cfg.Subjects)
	assert.Equal(t, nats.InterestPolicy, cfg.Retention)
	assert.Equal(t, nats.MemoryStorage, cfg.Storage)
	assert.Equal(t, nats.DiscardNew, cfg.Discard)
	assert.Equal(t, nats.S2Compression, cfg.Compression)
	assert.Equal(t, 3, cfg.Replicas)
	assert.Equal(t, int64(1000), cfg.MaxMsgs)
	assert.Equal(t, int64(1<<30), cfg.MaxBytes)
	assert.Equal(t, time.Hour, cfg.MaxAge)
	assert.Equal(t, 2*time.Minute, cfg.Duplicates)
	assert.Equal(t, uint64(100), cfg.FirstSeq)
	assert.False(t, cfg.AllowRollup)
	assert.True(t, cfg.DenyDelete)
	assert.True(t, cfg.DenyPurge)
	assert.True(t, cfg.DiscardNewPerSubject)
	assert.True(t, cfg.AllowDirect)
	assert.True(t, cfg.AllowMsgTTL)
	require.NotNil(t, cfg.Placement)
	assert.Equal(t, "east", cfg.Placement.Cluster)
	require.Len(t, cfg.Sources, 1)
	assert.Equal(t, "ORIGIN", cfg.Sources[0].Name)
	require.NotNil(t, cfg.SubjectTransform)
	assert.Equal(t, "out.>", cfg.SubjectTransform.Destination)
	require.NotNil(t, cfg.RePublish)
	assert.True(t, cfg.RePublish.HeadersOnly)
	assert.Equal(t, time.Minute, cfg.ConsumerLimits.InactiveThreshold)
	assert.Equal(t, 20, cfg.ConsumerLimits.MaxAckPending)
	assert.Equal(t, "ops", cfg.Metadata["owner"])
}

func TestStreamConfigRequestToNATSDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := (streamConfigRequest{Name: "S"}).toNATS()
	require.NoError(t, err)
	assert.Equal(t, nats.FileStorage, cfg.Storage)
	assert.Equal(t, 1, cfg.Replicas)
	assert.True(t, cfg.AllowRollup)
	assert.False(t, cfg.DenyDelete)
	assert.False(t, cfg.DenyPurge)
	assert.Nil(t, cfg.Placement)
}

func TestStreamConfigRequestMirrorClearsSubjects(t *testing.T) {
	t.Parallel()

	cfg, err := (streamConfigRequest{
		Name:     "M",
		Subjects: []string{"should.clear"},
		Mirror:   &apikit.StreamSourceRequest{Name: "ORIGIN", FilterSubject: "x.*"},
	}).toNATS()
	require.NoError(t, err)
	require.NotNil(t, cfg.Mirror)
	assert.Equal(t, "ORIGIN", cfg.Mirror.Name)
	assert.Equal(t, "x.*", cfg.Mirror.FilterSubject)
	assert.Nil(t, cfg.Subjects)
}

func TestConsumerConfigRequestToNATSFullFields(t *testing.T) {
	t.Parallel()

	cfg, err := (consumerConfigRequest{
		DurableName:         "worker",
		Description:         "orders worker",
		DeliverPolicy:       "by_start_sequence",
		AckPolicy:           "explicit",
		ReplayPolicy:        "instant",
		OptStartSeq:         42,
		FilterSubject:       "orders.>",
		AckWaitNs:           int64(30 * time.Second),
		MaxDeliver:          5,
		BackoffNs:           []int64{int64(time.Second), int64(2 * time.Second)},
		MaxAckPending:       100,
		MaxWaiting:          512,
		MaxRequestBatch:     10,
		MaxRequestExpiresNs: int64(time.Minute),
		DeliverSubject:      "deliver.orders",
		DeliverGroup:        "g1",
		FlowControl:         true,
		HeartbeatNs:         int64(5 * time.Second),
		HeadersOnly:         true,
		Replicas:            1,
		MemoryStorage:       true,
		InactiveThresholdNs: int64(time.Hour),
		RateLimitBps:        1024,
		SampleFreq:          "100",
		Metadata:            map[string]string{"owner": "ops"},
	}).toNATS()
	require.NoError(t, err)
	assert.Equal(t, "worker", cfg.Durable)
	assert.Equal(t, "orders worker", cfg.Description)
	assert.Equal(t, nats.DeliverByStartSequencePolicy, cfg.DeliverPolicy)
	assert.Equal(t, nats.AckExplicitPolicy, cfg.AckPolicy)
	assert.Equal(t, uint64(42), cfg.OptStartSeq)
	assert.Equal(t, "orders.>", cfg.FilterSubject)
	assert.Equal(t, 30*time.Second, cfg.AckWait)
	assert.Equal(t, 5, cfg.MaxDeliver)
	require.Len(t, cfg.BackOff, 2)
	assert.Equal(t, time.Second, cfg.BackOff[0])
	assert.Equal(t, "deliver.orders", cfg.DeliverSubject)
	assert.True(t, cfg.FlowControl)
	assert.True(t, cfg.MemoryStorage)
	assert.Equal(t, "ops", cfg.Metadata["owner"])
}

func TestConsumerConfigRequestRequiresStartFields(t *testing.T) {
	t.Parallel()

	_, err := (consumerConfigRequest{
		DurableName:   "w",
		DeliverPolicy: "by_start_sequence",
	}).toNATS()
	require.Error(t, err)

	_, err = (consumerConfigRequest{
		DurableName:   "w",
		DeliverPolicy: "by_start_time",
	}).toNATS()
	require.Error(t, err)
}
