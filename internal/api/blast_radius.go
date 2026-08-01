package api

import (
	"context"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const blastRadiusPageSize = 500

// GetStreamImpact returns blast-radius analysis for deleting a stream.
func (h *Handler) GetStreamImpact(ctx *fasthttp.RequestCtx) {
	name := httpctx.RouteParam(ctx, "name")
	if err := validateResourceName(name); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cluster := clusterID(ctx)
	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		info, err := client.StreamInfo(c, name)
		if err != nil {
			return nil, fasthttp.StatusNotFound, err
		}
		target := domain.StreamInfoFromNATS(info)

		consumers, err := listAllConsumers(c, client, name, blastRadiusPageSize)
		if err != nil {
			return nil, 0, err
		}

		streams, err := h.blastRadiusStreams(c, client, cluster)
		if err != nil {
			return nil, 0, err
		}

		return domain.ComputeBlastRadius(target, consumers, streams), fasthttp.StatusOK, nil
	})
}

func (h *Handler) blastRadiusStreams(ctx context.Context, client port.JetStreamExecutor, clusterID string) ([]domain.StreamInfo, error) {
	if h.hub != nil && !commonstrings.IsEmpty(clusterID) {
		if raw, _, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			if streams, err := streamsFromTopologyJSZ(raw); err == nil && len(streams) > 0 {
				return streams, nil
			}
		}
	}
	return listAllStreams(ctx, client, blastRadiusPageSize)
}

func streamsFromTopologyJSZ(raw []byte) ([]domain.StreamInfo, error) {
	var payload struct {
		AccountDetails []struct {
			StreamDetail []struct {
				Config *struct {
					Metadata map[string]string `json:"metadata"`
					Mirror   *struct {
						Name string `json:"name"`
					} `json:"mirror"`
					Sources []struct {
						Name string `json:"name"`
					} `json:"sources"`
				} `json:"config"`
				Name string `json:"name"`
			} `json:"stream_detail"`
		} `json:"account_details"`
	}
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.StreamInfo, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			if commonstrings.IsEmpty(stream.Name) {
				continue
			}
			si := domain.StreamInfo{Config: domain.StreamConfigDTO{Name: stream.Name}}
			if stream.Config != nil {
				si.Config.Metadata = stream.Config.Metadata
				if stream.Config.Mirror != nil {
					si.Config.Mirror = &domain.StreamSourceDTO{Name: stream.Config.Mirror.Name}
				}
				if len(stream.Config.Sources) > 0 {
					si.Config.Sources = make([]domain.StreamSourceDTO, 0, len(stream.Config.Sources))
					for _, src := range stream.Config.Sources {
						si.Config.Sources = append(si.Config.Sources, domain.StreamSourceDTO{Name: src.Name})
					}
				}
			}
			out = append(out, si)
		}
	}
	return out, nil
}

func listAllConsumers(ctx context.Context, client port.JetStreamExecutor, stream string, pageSize int) ([]domain.ConsumerInfo, error) {
	if pageSize <= 0 {
		pageSize = blastRadiusPageSize
	}
	var out []domain.ConsumerInfo
	offset := 0
	for {
		page, total, err := client.ListConsumers(ctx, stream, offset, pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.ConsumerInfosFromNATS(page)...)
		offset += len(page)
		if len(page) == 0 || offset >= total {
			break
		}
	}
	return out, nil
}

func listAllStreams(ctx context.Context, client port.JetStreamExecutor, pageSize int) ([]domain.StreamInfo, error) {
	if pageSize <= 0 {
		pageSize = blastRadiusPageSize
	}
	var out []domain.StreamInfo
	offset := 0
	for {
		page, total, err := client.ListStreams(ctx, offset, pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.StreamInfosFromNATS(page)...)
		offset += len(page)
		if len(page) == 0 || offset >= total {
			break
		}
	}
	return out, nil
}
