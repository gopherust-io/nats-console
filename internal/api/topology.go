package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/valyala/fasthttp"
)

var errMonitoringTooLarge = errors.New("monitoring payload too large")

// Topology builds a stream/consumer tree from the snapshot hub (or live jsz).
func (h *Handler) Topology(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	clusterName := string(ctx.QueryArgs().Peek("name"))
	if clusterName == "" {
		clusterName = "Cluster"
	}
	fresh := string(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	if !fresh && h.hub != nil {
		if data, _, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
		}
	}
	if raw == nil {
		c := requestContext(ctx)
		client, err := h.svc.JetStream.GetExecutor(c, clusterID)
		if err != nil {
			writeDomainError(ctx, err)
			return
		}
		data, err := client.Monitoring(c, snapshot.TopologyJSZPath)
		if err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return
		}
		raw = data
		if int64(len(raw)) > h.cfg.MaxMonitoringBytes() {
			serializer.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return
		}
	}

	projected := projectJSZForTopology(raw)
	tree, err := buildTopologyTree(projected, clusterName)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, tree)
}

type topologyNode struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Name     string         `json:"name"`
	Meta     []string       `json:"meta,omitempty"`
	Href     string         `json:"href,omitempty"`
	Status   string         `json:"status,omitempty"`
	Children []topologyNode `json:"children"`
}

type jszTopologyPayload struct {
	AccountDetails []struct {
		Name         string `json:"name"`
		StreamDetail []struct {
			Name   string `json:"name"`
			Config *struct {
				Retention string   `json:"retention"`
				Storage   string   `json:"storage"`
				Subjects  []string `json:"subjects"`
			} `json:"config"`
			State *struct {
				Messages      uint64 `json:"messages"`
				ConsumerCount int    `json:"consumer_count"`
				Bytes         uint64 `json:"bytes"`
			} `json:"state"`
			ConsumerDetail []struct {
				Config *struct {
					FilterSubject string `json:"filter_subject"`
					DurableName   string `json:"durable_name"`
					DeliverPolicy string `json:"deliver_policy"`
					AckPolicy     string `json:"ack_policy"`
				} `json:"config"`
				Name          string `json:"name"`
				NumPending    int    `json:"num_pending"`
				NumAckPending int    `json:"num_ack_pending"`
			} `json:"consumer_detail"`
		} `json:"stream_detail"`
	} `json:"account_details"`
}

func buildTopologyTree(raw []byte, clusterName string) (topologyNode, error) {
	var payload jszTopologyPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return topologyNode{}, err
	}
	streams := make([]topologyNode, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			subjects := []string{}
			storage, retention := "", ""
			if stream.Config != nil {
				subjects = append([]string(nil), stream.Config.Subjects...)
				storage = stream.Config.Storage
				retention = stream.Config.Retention
			}
			messages, consumerCount := uint64(0), 0
			if stream.State != nil {
				messages = stream.State.Messages
				consumerCount = stream.State.ConsumerCount
			}
			consumers := make([]topologyNode, 0, len(stream.ConsumerDetail))
			for _, c := range stream.ConsumerDetail {
				filter, deliver := "", ""
				if c.Config != nil {
					filter = c.Config.FilterSubject
					deliver = c.Config.DeliverPolicy
				}
				status := "healthy"
				if c.NumPending > 0 || c.NumAckPending > 0 {
					status = "warning"
				}
				meta := []string{}
				if filter != "" {
					meta = append(meta, "filter "+filter)
				}
				if deliver != "" {
					meta = append(meta, deliver)
				}
				if c.NumPending > 0 {
					meta = append(meta, "pending")
				}
				consumers = append(consumers, topologyNode{
					ID:       "consumer:" + stream.Name + ":" + c.Name,
					Kind:     "consumer",
					Name:     c.Name,
					Meta:     meta,
					Status:   status,
					Href:     "/streams/" + stream.Name + "/consumers/" + c.Name,
					Children: []topologyNode{},
				})
			}
			subjectNodes := make([]topologyNode, 0, len(subjects))
			for _, subj := range subjects {
				subjectNodes = append(subjectNodes, topologyNode{
					ID:       "subject:" + stream.Name + ":" + subj,
					Kind:     "subject",
					Name:     subj,
					Children: []topologyNode{},
				})
			}
			cc := max(consumerCount, len(consumers))
			meta := []string{
				strconv.FormatUint(messages, 10) + " msgs",
				strconv.Itoa(cc) + " consumers",
			}
			if storage != "" {
				meta = append(meta, storage)
			}
			if retention != "" {
				meta = append(meta, retention)
			}
			children := append(subjectNodes, consumers...)
			streams = append(streams, topologyNode{
				ID:       "stream:" + stream.Name,
				Kind:     "stream",
				Name:     stream.Name,
				Meta:     meta,
				Href:     "/streams/" + stream.Name,
				Status:   "healthy",
				Children: children,
			})
		}
	}
	return topologyNode{
		ID:       "cluster:root",
		Kind:     "cluster",
		Name:     clusterName,
		Meta:     []string{strconv.Itoa(len(streams)) + " streams"},
		Children: streams,
	}, nil
}

// projectJSZForTopology keeps account/stream/consumer fields needed for topology
// and drops bulky unused monitoring fields by re-encoding the trimmed struct.
func projectJSZForTopology(raw []byte) []byte {
	var payload jszTopologyPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return out
}

// SnapshotEventsSSE pushes snapshot refresh notifications to the browser.
func (h *Handler) SnapshotEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.hub == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("snapshot hub unavailable")
		return
	}
	clusterID := clusterID(ctx)
	ch, unsub := h.hub.Subscribe(16)
	defer unsub()

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				if ev.ClusterID != "" && ev.ClusterID != clusterID {
					continue
				}
				_, _ = w.WriteString("event: snapshot\ndata: {\"clusterId\":\"")
				_, _ = w.WriteString(ev.ClusterID)
				_, _ = w.WriteString("\",\"capturedAt\":\"")
				_, _ = w.WriteString(ev.CapturedAt.UTC().Format(time.RFC3339Nano))
				_, _ = w.WriteString("\"}\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				_, _ = w.WriteString(": ping\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}
