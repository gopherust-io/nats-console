package insights

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/app/monitoring"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// replicasPreferredBase remembers the last monitoring base that answered /routez
// per cluster so SSE scrapes do not keep paying failover cost on a dead primary.
var replicasPreferredBase sync.Map // clusterID -> string

// replicasPeerIDNames maps JetStream meta peer IDs to last-known server names so
// "Server name unknown at this time (peerID: …)" placeholders collapse onto real peers.
var replicasPeerIDNames sync.Map // peerID -> serverName

// replicasLastGood remembers the last successful replicas projection per cluster
// so a total monitoring outage can still return an all-offline view instead of 502.
var replicasLastGood sync.Map // clusterID -> []byte

// Replicas godoc
//
// @Summary Replicas
// @Tags Ops
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.DataMetaEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/replicas [get]
func (h *Handler) Replicas(ctx *fasthttp.RequestCtx) {
	clusterID := apikit.ClusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	c := httpctx.FromRequest(ctx)
	snap, err := h.buildReplicasSnapshotForCluster(c, clusterID, fresh)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			apikit.WriteAPIError(ctx, err)
			return
		}
		if degraded, ok := replicasDegradedFromLastGood(clusterID); ok {
			httpstatus.WriteData(ctx, fasthttp.StatusOK, degraded)
			return
		}
		apikit.WriteNATSError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	rememberReplicasSnapshot(clusterID, snap)
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

func (h *Handler) buildReplicasSnapshotForCluster(c context.Context, clusterID string, fresh bool) (replicasSnapshot, error) {
	varzs, routez, jsz, bases, err := loadReplicasMonitoring(c, h.Svc, h.Hub, clusterID, fresh, h.Cfg.MaxMonitoringBodyBytes)
	if err != nil {
		return replicasSnapshot{}, err
	}
	snap, err := buildReplicasSnapshot(varzs, routez, jsz, bases)
	if err != nil {
		return replicasSnapshot{}, err
	}
	snap.CapturedAt = time.Now().UTC()
	rememberReplicasSnapshot(clusterID, snap)
	return snap, nil
}

func fetchReplicasSnapshotJSON(ctx context.Context, svc *app.Services, hub *snapshot.Hub, clusterID string, maxBody int64) ([]byte, error) {
	if svc == nil {
		return nil, errors.New("services unavailable")
	}
	// Live /jsz (fresh) so SSE meta offline/leader tracks raft within the scrape interval.
	// Leave CapturedAt zero so broker SHA can skip SSE notify when peers are unchanged.
	varzs, routez, jsz, bases, err := loadReplicasMonitoring(ctx, svc, hub, clusterID, true, maxBody)
	if err != nil {
		return nil, err
	}
	snap, err := buildReplicasSnapshot(varzs, routez, jsz, bases)
	if err != nil {
		return nil, err
	}
	rememberReplicasSnapshot(clusterID, snap)
	return serializer.Marshal(snap)
}

func rememberReplicasSnapshot(clusterID string, snap replicasSnapshot) {
	if commonstrings.IsEmpty(clusterID) || len(snap.Peers) == 0 {
		return
	}
	raw, err := serializer.Marshal(snap)
	if err != nil {
		return
	}
	replicasLastGood.Store(clusterID, raw)
}

func replicasDegradedFromLastGood(clusterID string) (replicasSnapshot, bool) {
	v, ok := replicasLastGood.Load(clusterID)
	if !ok {
		return replicasSnapshot{}, false
	}
	raw, ok := v.([]byte)
	if !ok || len(raw) == 0 {
		return replicasSnapshot{}, false
	}
	offline := replicasOfflineFromLatest(clusterID, raw)
	if offline == nil {
		var snap replicasSnapshot
		if err := serializer.Unmarshal(raw, &snap); err != nil {
			return replicasSnapshot{}, false
		}
		for i := range snap.Peers {
			snap.Peers[i].Online = false
			snap.Peers[i].Leader = false
		}
		snap.OnlineCount = 0
		snap.JetStreamLeader = ""
		snap.CapturedAt = time.Now().UTC()
		return snap, len(snap.Peers) > 0
	}
	var snap replicasSnapshot
	if err := serializer.Unmarshal(offline, &snap); err != nil {
		return replicasSnapshot{}, false
	}
	snap.CapturedAt = time.Now().UTC()
	replicasLastGood.Store(clusterID, offline)
	return snap, true
}

// replicasOfflineFromLatest marks every peer offline when monitoring is unreachable.
func replicasOfflineFromLatest(clusterID string, latest []byte) []byte {
	if len(latest) == 0 {
		if v, ok := replicasLastGood.Load(clusterID); ok {
			if raw, ok := v.([]byte); ok {
				latest = raw
			}
		}
	}
	if len(latest) == 0 {
		return nil
	}
	var snap replicasSnapshot
	if err := serializer.Unmarshal(latest, &snap); err != nil {
		return nil
	}
	if len(snap.Peers) == 0 {
		return nil
	}
	alreadyOffline := true
	for i := range snap.Peers {
		if snap.Peers[i].Online {
			alreadyOffline = false
		}
		snap.Peers[i].Online = false
		snap.Peers[i].Leader = false
		snap.Peers[i].Current = nil
	}
	if alreadyOffline && snap.OnlineCount == 0 && commonstrings.IsEmpty(snap.JetStreamLeader) {
		return nil
	}
	snap.OnlineCount = 0
	snap.PeerCount = len(snap.Peers)
	snap.JetStreamLeader = ""
	if !commonstrings.IsEmpty(clusterID) {
		replicasPreferredBase.Delete(clusterID)
	}
	out, err := serializer.Marshal(snap)
	if err != nil {
		return nil
	}
	return out
}

// jszHasMetaCluster reports whether raw /jsz JSON includes a meta_cluster object.
// Slim hub /jsz (streams/consumers/messages only) returns false.
func jszHasMetaCluster(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var probe struct {
		MetaCluster *struct{} `json:"meta_cluster"`
	}
	if err := serializer.Unmarshal(raw, &probe); err != nil {
		return false
	}
	return probe.MetaCluster != nil
}

// hubJSZForReplicas returns cached topology /jsz only when it carries meta_cluster.
// Slim hub /jsz is never used — it omits raft meta.
func hubJSZForReplicas(hub *snapshot.Hub, clusterID string) []byte {
	if hub == nil || commonstrings.IsEmpty(clusterID) {
		return nil
	}
	data, _, ok := hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath)
	if !ok || !jszHasMetaCluster(data) {
		return nil
	}
	return data
}

func loadReplicasMonitoring(
	c context.Context,
	svc *app.Services,
	hub *snapshot.Hub,
	clusterID string,
	fresh bool,
	maxBody int64,
) (varzs [][]byte, routez, jsz []byte, monitorBases int, err error) {
	if svc == nil || svc.Cluster == nil {
		return nil, nil, nil, 0, errors.New("cluster service unavailable")
	}
	cluster, err := svc.Cluster.Get(c, clusterID)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	bases := apikit.MonitoringCandidates(cluster.NATSURL, cluster.MonitoringURL)
	if len(bases) == 0 {
		return nil, nil, nil, 0, errors.New("no monitoring URL configured")
	}
	if v, ok := replicasPreferredBase.Load(clusterID); ok {
		if preferred, ok := v.(string); ok {
			bases = preferBase(bases, preferred)
		}
	}

	if !fresh {
		jsz = hubJSZForReplicas(hub, clusterID)
	}

	// Resolve routez first so varz/jsz prefer the same monitoring base.
	data, usedBase, monErr := apikit.FetchMonitoringWithFailover(c, bases, "/routez")
	if monErr != nil {
		return nil, nil, nil, len(bases), monErr
	}
	routez = data
	if !commonstrings.IsEmpty(usedBase) {
		replicasPreferredBase.Store(clusterID, usedBase)
	}
	ordered := preferBase(bases, usedBase)

	// Fan-out /varz; first successful candidate is treated as monitored.
	varzs, err = apikit.FetchMonitoringAll(c, ordered, "/varz")
	if err != nil {
		return nil, nil, nil, len(bases), err
	}

	if jsz == nil {
		// Live /jsz — replicas need meta_cluster (slim hub cache omits it).
		data, _, monErr = apikit.FetchMonitoringWithFailover(c, ordered, "/jsz")
		if monErr != nil {
			jsz = nil
		} else {
			jsz = data
		}
	}

	total := int64(len(routez) + len(jsz))
	for _, v := range varzs {
		total += int64(len(v))
	}
	if total > maxBody {
		return nil, nil, nil, len(bases), apikit.ErrMonitoringTooLarge
	}
	return varzs, routez, jsz, len(bases), nil
}

func preferBase(bases []string, preferred string) []string {
	preferred = strings.TrimRight(strings.TrimSpace(preferred), "/")
	if commonstrings.IsEmpty(preferred) {
		return bases
	}
	out := make([]string, 0, len(bases))
	out = append(out, preferred)
	for _, b := range bases {
		if strings.TrimRight(b, "/") == preferred {
			continue
		}
		out = append(out, b)
	}
	return out
}

type replicasSnapshot struct {
	// CapturedAt is set on REST responses; omitted on SSE payloads so broker SHA dedup works.
	CapturedAt      time.Time     `json:"capturedAt,omitzero"`
	ClusterName     string        `json:"clusterName,omitempty"`
	MonitoredServer string        `json:"monitoredServer,omitempty"`
	JetStreamLeader string        `json:"jetstreamLeader,omitempty"`
	Peers           []replicaPeer `json:"peers"`
	ClusterSize     int           `json:"clusterSize"`
	PeerCount       int           `json:"peerCount"`
	OnlineCount     int           `json:"onlineCount"`
}

// goalign:ignore // JSON DTO; mixed optional monitoring fields.
type replicaPeer struct {
	InMsgs      *int64   `json:"inMsgs,omitempty"`
	OutMsgs     *int64   `json:"outMsgs,omitempty"`
	Pending     *int64   `json:"pending,omitempty"`
	Connections *int     `json:"connections,omitempty"`
	CPU         *float64 `json:"cpu,omitempty"`
	Mem         *int64   `json:"mem,omitempty"`
	Current     *bool    `json:"current,omitempty"` // nil = unknown; false = lagging; true = caught up
	Lag         *int64   `json:"lag,omitempty"`
	Name        string   `json:"name"`
	PeerID      string   `json:"peerId,omitempty"`
	Role        string   `json:"role"`
	Uptime      string   `json:"uptime,omitempty"`
	RTT         string   `json:"rtt,omitempty"`
	Idle        string   `json:"idle,omitempty"`
	Version     string   `json:"version,omitempty"`
	IP          string   `json:"ip,omitempty"`
	Online      bool     `json:"online"`
	Leader      bool     `json:"leader,omitempty"`
}

type replicasVarzPayload struct {
	Cluster *struct {
		Name string `json:"name"`
	} `json:"cluster"`
	ServerName  string  `json:"server_name"`
	Version     string  `json:"version"`
	Uptime      string  `json:"uptime"`
	CPU         float64 `json:"cpu"`
	Mem         int64   `json:"mem"`
	Connections int     `json:"connections"`
}

type replicasRoutezPayload struct {
	ServerName string `json:"server_name"`
	Routes     []struct {
		RemoteName  string `json:"remote_name"`
		Name        string `json:"name"`
		RemoteID    string `json:"remote_id"`
		IP          string `json:"ip"`
		Uptime      string `json:"uptime"`
		Idle        string `json:"idle"`
		RTT         string `json:"rtt"`
		PendingSize int64  `json:"pending_size"`
		InMsgs      int64  `json:"in_msgs"`
		OutMsgs     int64  `json:"out_msgs"`
	} `json:"routes"`
}

type replicasJSZPayload struct {
	MetaCluster *monitoring.JSZMetaCluster `json:"meta_cluster"`
	ServerName  string                     `json:"server_name"`
}

func buildReplicasSnapshot(varzRaws [][]byte, routezRaw, jszRaw []byte, monitorBases int) (replicasSnapshot, error) {
	var out replicasSnapshot
	peers := make(map[string]*replicaPeer)
	varzSeen := make(map[string]struct{})
	routezServer := ""

	upsert := func(name string) *replicaPeer {
		name = strings.TrimSpace(name)
		if commonstrings.IsEmpty(name) {
			return nil
		}
		if p, ok := peers[name]; ok {
			return p
		}
		p := &replicaPeer{Name: name, Role: "meta", Online: false}
		peers[name] = p
		return p
	}

	applyVarz := func(vz replicasVarzPayload, primary bool) {
		if vz.Cluster != nil && commonstrings.IsEmpty(out.ClusterName) {
			out.ClusterName = vz.Cluster.Name
		}
		if primary && !commonstrings.IsEmpty(vz.ServerName) {
			out.MonitoredServer = vz.ServerName
		}
		p := upsert(vz.ServerName)
		if p == nil {
			return
		}
		if primary {
			p.Role = "monitored"
		} else if p.Role != "monitored" && p.Role != "route" {
			p.Role = "route"
		}
		p.Online = true
		varzSeen[p.Name] = struct{}{}
		p.Uptime = firstNonEmpty(vz.Uptime, p.Uptime)
		p.Version = firstNonEmpty(vz.Version, p.Version)
		conn := vz.Connections
		p.Connections = &conn
		cpu := vz.CPU
		p.CPU = &cpu
		mem := vz.Mem
		p.Mem = &mem
	}

	primary := true
	for _, raw := range varzRaws {
		if len(raw) == 0 {
			continue
		}
		var vz replicasVarzPayload
		if err := serializer.Unmarshal(raw, &vz); err != nil {
			return out, err
		}
		applyVarz(vz, primary)
		if primary && !commonstrings.IsEmpty(vz.ServerName) {
			primary = false
		}
	}

	if len(routezRaw) > 0 {
		var rz replicasRoutezPayload
		if err := serializer.Unmarshal(routezRaw, &rz); err != nil {
			return out, err
		}
		routezServer = rz.ServerName
		if commonstrings.IsEmpty(out.MonitoredServer) {
			out.MonitoredServer = rz.ServerName
		}
		if p := upsert(rz.ServerName); p != nil {
			if p.Role != "monitored" && out.MonitoredServer == rz.ServerName {
				p.Role = "monitored"
			}
			p.Online = true
		}
		for _, r := range rz.Routes {
			name := r.RemoteName
			if commonstrings.IsEmpty(name) {
				name = r.Name
			}
			if commonstrings.IsEmpty(name) {
				name = r.RemoteID
			}
			p := upsert(name)
			if p == nil {
				continue
			}
			if p.Role != "monitored" {
				p.Role = "route"
			}
			p.Online = true
			p.IP = r.IP
			p.Uptime = firstNonEmpty(p.Uptime, r.Uptime)
			p.Idle = r.Idle
			p.RTT = r.RTT
			in := r.InMsgs
			outMsgs := r.OutMsgs
			pending := r.PendingSize
			p.InMsgs = &in
			p.OutMsgs = &outMsgs
			p.Pending = &pending
		}
	}

	// Peers that no longer answer /varz are offline even when routez still lists them.
	// Only when multi-monitor + single varz still trusts routez until meta offline
	// (cannot scrape peer varz without more bases).
	// Skip the routez reporter itself to avoid false-offline on a flaky varz hop (M4).
	if len(varzSeen) > 0 && (monitorBases > 1 || len(varzSeen) > 1) {
		for _, p := range peers {
			if _, ok := varzSeen[p.Name]; ok {
				continue
			}
			if p.Name == routezServer || p.Name == out.MonitoredServer {
				continue
			}
			p.Online = false
		}
	}

	if len(jszRaw) > 0 {
		var jz replicasJSZPayload
		if err := serializer.Unmarshal(jszRaw, &jz); err != nil {
			return out, err
		}
		if jz.MetaCluster != nil {
			meta := jz.MetaCluster
			if commonstrings.IsEmpty(out.ClusterName) {
				out.ClusterName = meta.Name
			}
			out.JetStreamLeader = resolveMetaPeerName(meta.Leader, meta.Peer)
			out.ClusterSize = meta.Size
			if p := upsert(out.JetStreamLeader); p != nil {
				p.Leader = true
				if !commonstrings.IsEmpty(meta.Peer) {
					rememberPeerIDName(meta.Peer, p.Name)
					p.PeerID = meta.Peer
				}
				// Current comes from meta.replicas when present — do not invent caught-up.
			}
			for _, r := range meta.Replicas {
				name := resolveMetaPeerName(r.Name, r.Peer)
				if commonstrings.IsEmpty(name) {
					continue
				}
				// Drop ghost placeholder keys if we remapped onto a real peer name.
				if name != r.Name {
					delete(peers, r.Name)
				}
				p := upsert(name)
				if p == nil {
					continue
				}
				if peerID := firstNonEmpty(r.Peer, peerIDFromUnknownName(r.Name)); !commonstrings.IsEmpty(peerID) {
					p.PeerID = peerID
					if !isUnknownServerName(name) {
						rememberPeerIDName(peerID, name)
					}
				}
				cur := r.Current
				p.Current = &cur
				if r.Lag > 0 {
					lag := int64(min(r.Lag, math.MaxInt64))
					p.Lag = &lag
				}
				if r.Offline {
					// Live meta offline wins over lingering routez/varz (SSE scrapes fresh /jsz).
					// Preserve route link metrics (RTT/in–out) so meta-offline does not erase evidence.
					p.Online = false
				} else if p.Role == "meta" {
					// Meta-only peers (no monitor candidate) stay online when raft says so,
					// unless we already demoted via varz absence for a multi-peer mesh.
					if _, seen := varzSeen[p.Name]; !seen && len(varzSeen) == 0 {
						p.Online = true
					}
				}
				if p.Role != "monitored" && p.Role != "route" {
					p.Role = "meta"
				}
			}
			if p := upsert(jz.ServerName); p != nil && commonstrings.IsEmpty(out.MonitoredServer) {
				out.MonitoredServer = jz.ServerName
			}
		}
	}

	if out.ClusterSize == 0 {
		out.ClusterSize = len(peers)
	}

	list := make([]replicaPeer, 0, len(peers))
	online := 0
	for _, p := range peers {
		if isUnknownServerName(p.Name) {
			// Prefer compact peer-id label when we never learned a real name.
			if id := firstNonEmpty(p.PeerID, peerIDFromUnknownName(p.Name)); !commonstrings.IsEmpty(id) {
				p.Name = "peer:" + id
				p.PeerID = id
			}
		}
		if p.Online {
			online++
		}
		if !commonstrings.IsEmpty(out.JetStreamLeader) && p.Name == out.JetStreamLeader {
			p.Leader = true
		}
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Leader != list[j].Leader {
			return list[i].Leader
		}
		if list[i].Role == "monitored" && list[j].Role != "monitored" {
			return true
		}
		if list[j].Role == "monitored" && list[i].Role != "monitored" {
			return false
		}
		return list[i].Name < list[j].Name
	})
	out.Peers = list
	out.PeerCount = len(list)
	out.OnlineCount = online
	return out, nil
}

func isUnknownServerName(name string) bool {
	return strings.Contains(name, "Server name unknown at this time")
}

func peerIDFromUnknownName(name string) string {
	const marker = "peerID: "
	_, rest, found := strings.Cut(name, marker)
	if !found {
		return ""
	}
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ")")
	return strings.TrimSpace(rest)
}

func rememberPeerIDName(peerID, name string) {
	peerID = strings.TrimSpace(peerID)
	name = strings.TrimSpace(name)
	if commonstrings.IsEmpty(peerID) || commonstrings.IsEmpty(name) || isUnknownServerName(name) {
		return
	}
	replicasPeerIDNames.Store(peerID, name)
}

func resolveMetaPeerName(name, peerID string) string {
	name = strings.TrimSpace(name)
	peerID = firstNonEmpty(strings.TrimSpace(peerID), peerIDFromUnknownName(name))
	if !isUnknownServerName(name) && !commonstrings.IsEmpty(name) {
		if !commonstrings.IsEmpty(peerID) {
			rememberPeerIDName(peerID, name)
		}
		return name
	}
	if commonstrings.IsEmpty(peerID) {
		return name
	}
	if v, ok := replicasPeerIDNames.Load(peerID); ok {
		if known, ok := v.(string); ok && !commonstrings.IsEmpty(known) {
			return known
		}
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if !commonstrings.IsEmpty(v) {
			return v
		}
	}
	return ""
}
