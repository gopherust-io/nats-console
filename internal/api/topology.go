package api

import (
	"bufio"
	"errors"
	"strconv"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

var errMonitoringTooLarge = errors.New("monitoring payload too large")

// Topology builds a stream/consumer tree from the snapshot hub (or live jsz).
func (h *Handler) Topology(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	clusterName := strings.BytesToString(ctx.QueryArgs().Peek("name"))
	if strings.IsEmpty(clusterName) {
		clusterName = "Cluster"
	}
	fresh := strings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	var raw []byte
	if !fresh && h.hub != nil {
		if data, _, ok := h.hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			raw = data
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
		if int64(len(raw)) > h.cfg.MaxMonitoringBodyBytes {
			httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, errMonitoringTooLarge)
			return
		}
	}

	serverName := resolveTopologyServerName(h, clusterID, raw)
	projected := projectJSZForTopology(raw)
	thr := domain.SlowConsumerThresholds{
		PendingThreshold: h.cfg.SlowConsumer.PendingThreshold,
		LagThreshold:     h.cfg.SlowConsumer.LagThreshold,
		AckPendingRatio:  h.cfg.SlowConsumer.AckPendingRatio,
	}
	tree, err := buildTopologyTree(projected, clusterName, serverName, thr)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, tree)
}

// goalign:ignore // JSON DTO mirroring NATS PeerInfo; trailing bool padding is unavoidable.
type topologyPeer struct {
	Lag     uint64 `json:"lag,omitempty"`
	Active  int64  `json:"active,omitempty"`
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Offline bool   `json:"offline,omitempty"`
}

type topologyRaft struct {
	Group       string         `json:"group,omitempty"`
	Leader      string         `json:"leader,omitempty"`
	ClusterSize int            `json:"clusterSize,omitempty"`
	Peers       []topologyPeer `json:"peers,omitempty"`
}

type topologyNode struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Name     string         `json:"name"`
	Meta     []string       `json:"meta,omitempty"`
	Href     string         `json:"href,omitempty"`
	Status   string         `json:"status,omitempty"`
	Role     string         `json:"role,omitempty"`
	Raft     *topologyRaft  `json:"raft,omitempty"`
	Children []topologyNode `json:"children"`
}

// goalign:ignore // JSON DTO mirroring NATS PeerInfo; trailing bool padding is unavoidable.
type jszPeerInfo struct {
	Lag     uint64 `json:"lag,omitempty"`
	Active  int64  `json:"active,omitempty"`
	Name    string `json:"name"`
	Current bool   `json:"current"`
	Offline bool   `json:"offline,omitempty"`
}

type jszClusterInfo struct {
	Name      string        `json:"name"`
	RaftGroup string        `json:"raft_group"`
	Leader    string        `json:"leader"`
	Replicas  []jszPeerInfo `json:"replicas"`
}

type jszMetaCluster struct {
	Name     string        `json:"name"`
	Leader   string        `json:"leader"`
	Peer     string        `json:"peer"`
	Size     int           `json:"cluster_size"`
	Replicas []jszPeerInfo `json:"replicas"`
}

type jszTopologyPayload struct {
	ServerName     string             `json:"server_name"`
	ServerID       string             `json:"server_id"`
	MetaCluster    *jszMetaCluster    `json:"meta_cluster"`
	AccountDetails []jszAccountDetail `json:"account_details"`
}

type jszAccountDetail struct {
	Name         string            `json:"name"`
	StreamDetail []jszStreamDetail `json:"stream_detail"`
}

type jszStreamDetail struct {
	Name   string `json:"name"`
	Config *struct {
		Retention   string   `json:"retention"`
		Storage     string   `json:"storage"`
		Subjects    []string `json:"subjects"`
		NumReplicas int      `json:"num_replicas"`
	} `json:"config"`
	State *struct {
		Messages      uint64 `json:"messages"`
		ConsumerCount int    `json:"consumer_count"`
		Bytes         uint64 `json:"bytes"`
		LastSeq       uint64 `json:"last_seq"`
	} `json:"state"`
	Cluster        *jszClusterInfo     `json:"cluster"`
	ConsumerDetail []jszConsumerDetail `json:"consumer_detail"`
}

type jszConsumerDetail struct {
	Config *struct {
		FilterSubject  string            `json:"filter_subject"`
		FilterSubjects []string          `json:"filter_subjects"`
		DurableName    string            `json:"durable_name"`
		DeliverPolicy  string            `json:"deliver_policy"`
		AckPolicy      string            `json:"ack_policy"`
		NumReplicas    int               `json:"num_replicas"`
		MaxAckPending  int               `json:"max_ack_pending"`
		Metadata       map[string]string `json:"metadata"`
	} `json:"config"`
	Delivered *struct {
		ConsumerSeq uint64 `json:"consumer_seq"`
		StreamSeq   uint64 `json:"stream_seq"`
	} `json:"delivered"`
	Name           string          `json:"name"`
	NumPending     int             `json:"num_pending"`
	NumAckPending  int             `json:"num_ack_pending"`
	NumWaiting     int             `json:"num_waiting"`
	NumRedelivered int             `json:"num_redelivered"`
	Cluster        *jszClusterInfo `json:"cluster"`
}

func buildTopologyTree(raw []byte, clusterName, serverName string, thr domain.SlowConsumerThresholds) (topologyNode, error) {
	thr = thr.WithDefaults()
	var payload jszTopologyPayload
	if err := serializer.Unmarshal(raw, &payload); err != nil {
		return topologyNode{}, err
	}
	if strings.IsEmpty(serverName) {
		serverName = payload.ServerName
	}

	streams := make([]topologyNode, 0)
	for _, acct := range payload.AccountDetails {
		for _, stream := range acct.StreamDetail {
			subjects := []string{}
			storage, retention := "", ""
			numReplicas := 1
			if stream.Config != nil {
				subjects = append([]string(nil), stream.Config.Subjects...)
				storage = stream.Config.Storage
				retention = stream.Config.Retention
				if stream.Config.NumReplicas > 0 {
					numReplicas = stream.Config.NumReplicas
				}
			}
			messages, consumerCount, lastSeq := uint64(0), 0, uint64(0)
			if stream.State != nil {
				messages = stream.State.Messages
				consumerCount = stream.State.ConsumerCount
				lastSeq = stream.State.LastSeq
			}

			streamRaft := raftFromCluster(stream.Cluster, numReplicas)
			streamStatus := raftStatus(streamRaft, numReplicas, false)
			streamRole := raftRole(streamRaft, numReplicas, serverName)

			consumers := make([]topologyNode, 0, len(stream.ConsumerDetail))
			for _, c := range stream.ConsumerDetail {
				filter, deliver := "", ""
				cReplicas := 0
				maxAckPending := 0
				if c.Config != nil {
					filter = c.Config.FilterSubject
					deliver = c.Config.DeliverPolicy
					cReplicas = c.Config.NumReplicas
					maxAckPending = c.Config.MaxAckPending
				}
				if cReplicas <= 0 {
					cReplicas = numReplicas
				}
				deliveredStreamSeq := uint64(0)
				if c.Delivered != nil {
					deliveredStreamSeq = c.Delivered.StreamSeq
				}
				lag := domain.ConsumerLagMessages(lastSeq, deliveredStreamSeq)
				pending := uint64(0)
				if c.NumPending > 0 {
					pending = uint64(c.NumPending)
				}
				slow, reasons := domain.EvaluateSlowConsumer(pending, lag, c.NumAckPending, maxAckPending, thr)
				cRaft := raftFromCluster(c.Cluster, cReplicas)
				status := raftStatus(cRaft, cReplicas, slow)
				role := raftRole(cRaft, cReplicas, serverName)

				meta := []string{}
				if !strings.IsEmpty(filter) {
					meta = append(meta, "filter "+filter)
				}
				if !strings.IsEmpty(deliver) {
					meta = append(meta, deliver)
				}
				if slow {
					meta = append(meta, "slow")
				}
				for _, reason := range reasons {
					switch reason {
					case domain.SlowReasonPending:
						meta = append(meta, "pending")
					case domain.SlowReasonAckPending:
						meta = append(meta, "ack pending")
					case domain.SlowReasonLag:
						meta = append(meta, "lag "+strconv.FormatUint(lag, 10))
					}
				}
				if c.NumWaiting > 0 {
					meta = append(meta, "waiting")
				}
				if c.NumRedelivered > 0 {
					meta = append(meta, "redelivered")
				}
				meta = appendRaftMeta(meta, cRaft, role)

				consumers = append(consumers, topologyNode{
					ID:       "consumer:" + stream.Name + ":" + c.Name,
					Kind:     "consumer",
					Name:     c.Name,
					Meta:     meta,
					Status:   status,
					Role:     role,
					Raft:     cRaft,
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
			if !strings.IsEmpty(storage) {
				meta = append(meta, storage)
			}
			if !strings.IsEmpty(retention) {
				meta = append(meta, retention)
			}
			meta = appendRaftMeta(meta, streamRaft, streamRole)

			children := append(subjectNodes, consumers...)
			streams = append(streams, topologyNode{
				ID:       "stream:" + stream.Name,
				Kind:     "stream",
				Name:     stream.Name,
				Meta:     meta,
				Href:     "/streams/" + stream.Name,
				Status:   streamStatus,
				Role:     streamRole,
				Raft:     streamRaft,
				Children: children,
			})
		}
	}

	metaRaft := raftFromMeta(payload.MetaCluster)
	metaReplicas := 1
	if payload.MetaCluster != nil && payload.MetaCluster.Size > 0 {
		metaReplicas = payload.MetaCluster.Size
	}
	rootMeta := []string{strconv.Itoa(len(streams)) + " streams"}
	rootRole := raftRole(metaRaft, metaReplicas, serverName)
	rootMeta = appendRaftMeta(rootMeta, metaRaft, rootRole)
	if metaRaft != nil && !strings.IsEmpty(metaRaft.Leader) {
		rootMeta = append(rootMeta, "meta leader "+metaRaft.Leader)
	}

	return topologyNode{
		ID:       "cluster:root",
		Kind:     "cluster",
		Name:     clusterName,
		Meta:     rootMeta,
		Status:   raftStatus(metaRaft, metaReplicas, false),
		Role:     rootRole,
		Raft:     metaRaft,
		Children: streams,
	}, nil
}

func raftFromCluster(ci *jszClusterInfo, numReplicas int) *topologyRaft {
	if ci == nil && numReplicas <= 1 {
		return nil
	}
	r := &topologyRaft{
		ClusterSize: numReplicas,
	}
	if ci != nil {
		r.Group = ci.RaftGroup
		r.Leader = ci.Leader
		if !strings.IsEmpty(ci.Name) && strings.IsEmpty(r.Group) {
			r.Group = ci.Name
		}
		r.Peers = peersFromJSZ(ci.Replicas)
		if r.ClusterSize <= 1 && len(r.Peers) > 0 {
			r.ClusterSize = len(r.Peers) + 1
		}
	}
	if strings.IsEmpty(r.Leader) && len(r.Peers) == 0 && r.ClusterSize <= 1 {
		return nil
	}
	return r
}

func raftFromMeta(meta *jszMetaCluster) *topologyRaft {
	if meta == nil {
		return nil
	}
	size := meta.Size
	if size <= 0 {
		size = 1
		if !strings.IsEmpty(meta.Leader) {
			size = max(1, len(meta.Replicas)+1)
		}
	}
	if size <= 1 && strings.IsEmpty(meta.Leader) && len(meta.Replicas) == 0 {
		return nil
	}
	return &topologyRaft{
		Group:       meta.Name,
		Leader:      meta.Leader,
		ClusterSize: size,
		Peers:       peersFromJSZ(meta.Replicas),
	}
}

func peersFromJSZ(in []jszPeerInfo) []topologyPeer {
	if len(in) == 0 {
		return nil
	}
	out := make([]topologyPeer, 0, len(in))
	for _, p := range in {
		out = append(out, topologyPeer(p))
	}
	return out
}

func raftStatus(r *topologyRaft, numReplicas int, pendingWarn bool) string {
	clustered := numReplicas > 1 || (r != nil && r.ClusterSize > 1)
	if r != nil {
		for _, p := range r.Peers {
			if p.Offline || (!p.Current && p.Lag > 0) {
				return "unhealthy"
			}
		}
		if clustered && strings.IsEmpty(r.Leader) {
			return "unhealthy"
		}
	} else if clustered {
		return "unhealthy"
	}
	if pendingWarn {
		return "warning"
	}
	return "healthy"
}

func raftRole(r *topologyRaft, numReplicas int, serverName string) string {
	clustered := numReplicas > 1 || (r != nil && r.ClusterSize > 1)
	if !clustered {
		return "standalone"
	}
	if r == nil {
		return "replica"
	}
	if !strings.IsEmpty(serverName) && !strings.IsEmpty(r.Leader) {
		if serverName == r.Leader {
			return "leader"
		}
		return "replica"
	}
	// Without a local server name, still mark clustered streams as replica unless
	// peer list is empty and leader is known (queried from leader often omits self).
	if !strings.IsEmpty(r.Leader) && len(r.Peers) > 0 {
		return "replica"
	}
	if !strings.IsEmpty(r.Leader) {
		return "leader"
	}
	return "replica"
}

func appendRaftMeta(meta []string, r *topologyRaft, role string) []string {
	if r == nil {
		return meta
	}
	if role == "leader" || role == "replica" {
		meta = append(meta, role)
	}
	if r.ClusterSize > 1 {
		meta = append(meta, "R"+strconv.Itoa(r.ClusterSize))
	}
	if !strings.IsEmpty(r.Leader) && role != "leader" {
		meta = append(meta, "leader "+r.Leader)
	}
	return meta
}

func resolveTopologyServerName(h *Handler, clusterID string, jszRaw []byte) string {
	var tip struct {
		ServerName string `json:"server_name"`
	}
	if serializer.Unmarshal(jszRaw, &tip) == nil && !strings.IsEmpty(tip.ServerName) {
		return tip.ServerName
	}
	if h == nil || h.hub == nil {
		return ""
	}
	raw, _, ok := h.hub.MonitoringPayload(clusterID, "/varz")
	if !ok || len(raw) == 0 {
		return ""
	}
	var varz struct {
		ServerName string `json:"server_name"`
	}
	if serializer.Unmarshal(raw, &varz) != nil {
		return ""
	}
	return varz.ServerName
}

// projectJSZForTopology previously remashaled a trimmed DTO. Typed unmarshal in
// buildTopologyTree / zombies / etc. already ignores unused fields, so remashal
// only burned CPU. Kept as identity for call-site compatibility.
func projectJSZForTopology(raw []byte) []byte {
	return raw
}

// SnapshotEventsSSE pushes snapshot refresh notifications to the browser.
func (h *Handler) SnapshotEventsSSE(ctx *fasthttp.RequestCtx) {
	if h.hub == nil {
		ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		ctx.SetBodyString("snapshot hub unavailable")
		return
	}
	clusterID := clusterID(ctx)
	ch, unsub := h.hub.SubscribeCluster(clusterID, 16)

	ctx.SetContentType("text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Connection", "keep-alive")
	// unsub must run inside the stream writer: SetBodyStreamWriter returns
	// immediately after starting the writer goroutine, so a handler-level
	// defer would unsubscribe before any events are delivered.
	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer unsub()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
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
