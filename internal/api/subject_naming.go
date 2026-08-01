package api

import (
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// SubjectNaming returns subject naming findings derived from topology jsz.
func (h *Handler) SubjectNaming(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	var capturedAt time.Time
	if !fresh && h.hub != nil {
		if data, at, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
			capturedAt = at
		}
	}
	if raw == nil {
		c := httpctx.FromRequest(ctx)
		client, err := h.svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			writeAPIError(ctx, err)
			return
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return
		}
		raw = data
		capturedAt = time.Now().UTC()
		if int64(len(raw)) > h.cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return
		}
	}

	projected := projectJSZForTopology(raw)
	inputs, err := subjectNamingInputsFromJSZ(projected)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	snap := domain.AnalyzeSubjectNaming(inputs)
	if !capturedAt.IsZero() {
		snap.CapturedAt = capturedAt
	} else {
		snap.CapturedAt = time.Now().UTC()
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

func subjectNamingInputsFromJSZ(raw []byte) ([]domain.SubjectNamingInput, error) {
	var payload jszTopologyPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.SubjectNamingInput, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			in := domain.SubjectNamingInput{Name: stream.Name}
			if stream.Config != nil {
				in.Subjects = append([]string(nil), stream.Config.Subjects...)
			}
			for _, c := range stream.ConsumerDetail {
				cin := domain.SubjectNamingConsumerInput{Name: c.Name}
				if c.Config != nil {
					cin.FilterSubject = c.Config.FilterSubject
					cin.FilterSubjects = append([]string(nil), c.Config.FilterSubjects...)
				}
				in.Consumers = append(in.Consumers, cin)
			}
			out = append(out, in)
		}
	}
	return out, nil
}
