package performance_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

func BenchmarkStreamInfoFromNATSAllocs(b *testing.B) {
	info := &nats.StreamInfo{
		Config: nats.StreamConfig{
			Name:      "ORDERS",
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
			MaxMsgs:   100,
		},
		State: nats.StreamState{
			Msgs:      10,
			FirstSeq:  1,
			LastSeq:   10,
			Consumers: 2,
		},
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = domain.StreamInfoFromNATS(info)
	}
}

func BenchmarkWriteJSONAllocs(b *testing.B) {
	payload := domain.StreamInfo{
		Config: domain.StreamConfigDTO{Name: "ORDERS", Retention: "limits", Storage: "file"},
		State:  domain.StreamStateDTO{Messages: 10, FirstSeq: 1, LastSeq: 10, ConsumerCount: 2},
	}
	ctx := &fasthttp.RequestCtx{}
	b.ReportAllocs()
	for b.Loop() {
		serializer.WriteJSON(ctx, fasthttp.StatusOK, payload)
	}
}
