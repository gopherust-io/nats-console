package domain

import (
	"testing"

	"github.com/gopherust-io/nats/dlq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDLQStream(t *testing.T) {
	assert.True(t, IsDLQStream("ORDERS_DLQ", nil))
	assert.True(t, IsDLQStream("orders_DLQ", nil))
	assert.False(t, IsDLQStream("ORDERS", nil))
	assert.True(t, IsDLQStream("POISON", map[string]string{MetadataRoleKey: MetadataRoleDLQ}))
	assert.False(t, IsDLQStream("POISON", map[string]string{MetadataRoleKey: "work"}))
}

func TestDLQMessageFromStream(t *testing.T) {
	msg := StreamMessage{
		Seq:     7,
		Subject: "orders.dlq.poison",
		Time:    "2026-01-02T03:04:05Z",
		Data:    "YQ==",
		Headers: map[string]string{
			dlq.HeaderReason:          "handler_requested",
			dlq.HeaderOriginalSubject: "orders.created",
			dlq.HeaderStream:          "ORDERS",
			dlq.HeaderSequence:        "42",
			dlq.HeaderConsumer:        "worker",
			dlq.HeaderNumDelivered:    "5",
			dlq.HeaderAutopsyError:    "boom",
			dlq.HeaderAutopsyHash:     "abc",
			dlq.HeaderAutopsyStack:    "stack",
		},
	}
	out := DLQMessageFromStream(msg)
	assert.Equal(t, uint64(7), out.Seq)
	assert.Equal(t, "handler_requested", out.Reason)
	assert.Equal(t, "orders.created", out.OriginalSubject)
	assert.Equal(t, "ORDERS", out.SourceStream)
	assert.Equal(t, uint64(42), out.SourceSeq)
	assert.Equal(t, "worker", out.Consumer)
	assert.Equal(t, uint64(5), out.NumDelivered)
	assert.Equal(t, "boom", out.AutopsyError)
	assert.Equal(t, "boom", out.DisplayError())
}

func TestRetryPublishHeaders(t *testing.T) {
	headers := map[string]string{
		dlq.HeaderReason:          "max_deliver",
		dlq.HeaderOriginalSubject: "orders.created",
		dlq.HeaderAutopsyError:    "boom",
		headerTraceID:             "trace-1",
		headerContentType:         "application/json",
		headerMsgID:               "msg-1",
		"X-Custom":                "keep-me-not",
	}
	out := RetryPublishHeaders(headers, "ORDERS_DLQ", 42)
	require.NotNil(t, out)
	assert.Equal(t, "trace-1", out[headerTraceID])
	assert.Equal(t, "application/json", out[headerContentType])
	assert.Equal(t, "nats-consol-dlq-retry:ORDERS_DLQ:42", out[headerMsgID])
	_, hasReason := out[dlq.HeaderReason]
	assert.False(t, hasReason)
	_, hasCustom := out["X-Custom"]
	assert.False(t, hasCustom)
}

func TestDLQRetryRequestValidate(t *testing.T) {
	require.ErrorIs(t, (DLQRetryRequest{}).Validate(), ErrDLQRetryEmpty)
	require.NoError(t, (DLQRetryRequest{All: true}).Validate())
	require.NoError(t, (DLQRetryRequest{Seqs: []uint64{1, 2}}).Validate())
	require.Error(t, (DLQRetryRequest{Seqs: []uint64{0}}).Validate())
}

func TestNormalizeDLQListLimit(t *testing.T) {
	assert.Equal(t, DefaultDLQListLimit, NormalizeDLQListLimit(0))
	assert.Equal(t, MaxDLQListLimit, NormalizeDLQListLimit(10_000))
	assert.Equal(t, 50, NormalizeDLQListLimit(50))
}
