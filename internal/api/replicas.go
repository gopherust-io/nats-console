package api

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/snapshot"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// Replicas returns a view-only projection of NATS server peers from varz, routez, and jsz.
func (h *Handler) Replicas(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	fresh := commonstrings.BytesToString(ctx.QueryArgs().Peek("fresh")) == "1"

	c := httpctx.FromRequest(ctx)
	snap, err := h.buildReplicasSnapshotForCluster(c, clusterID, fresh)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeAPIError(ctx, err)
			return
		}
		writeNATSError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, snap)
}

func (h *Handler) buildReplicasSnapshotForCluster(c context.Context, clusterID string, fresh bool) (replicasSnapshot, error) {
	varzs, routez, jsz, err := loadReplicasMonitoring(c, h.svc, h.hub, clusterID, fresh, h.cfg.MaxMonitoringBodyBytes)
	if err != nil {
		return replicasSnapshot{}, err
	}
	snap, err := buildReplicasSnapshot(varzs, routez, jsz)
	if err != nil {
		return replicasSnapshot{}, err
	}
	snap.CapturedAt = time.Now().UTC()
	return snap, nil
}

func fetchReplicasSnapshotJSON(ctx context.Context, svc *app.Services, hub *snapshot.Hub, clusterID string, maxBody int64) ([]byte, error) {
	if svc == nil {
		return nil, errors.New("services unavailable")
	}
	// Prefer hub /jsz when present (collector refreshes it). Always scrape routez/varz
	// for peer projection; broker SHA skip avoids SSE notify when unchanged.
	varzs, routez, jsz, err := loadReplicasMonitoring(ctx, svc, hub, clusterID, false, maxBody)
	if err != nil {
		return nil, err
	}
	snap, err := buildReplicasSnapshot(varzs, routez, jsz)
	if err != nil {
		return nil, err
	}
	snap.CapturedAt = time.Now().UTC()
	return serializer.Marshal(snap)
}

func loadReplicasMonitoring(
	c context.Context,
	svc *app.Services,
	hub *snapshot.Hub,
	clusterID string,
	fresh bool,
	maxBody int64,
) (varzs [][]byte, routez, jsz []byte, err error) {
	if svc == nil || svc.Cluster == nil {
		return nil, nil, nil, errors.New("cluster service unavailable")
	}
	cluster, err := svc.Cluster.Get(c, clusterID)
	if err != nil {
		return nil, nil, nil, err
	}

	bases := monitoringCandidates(cluster.NATSURL, cluster.MonitoringURL)
	if len(bases) == 0 {
		return nil, nil, nil, errors.New("no monitoring URL configured")
	}

	if !fresh && hub != nil {
		// Prefer lightweight /jsz (meta_cluster only); fall back to topology cache.
		if data, _, ok := hub.MonitoringPayload(clusterID, "/jsz"); ok {
			jsz = data
		} else if data, _, ok := hub.MonitoringPayload(clusterID, snapshot.TopologyJSZPath); ok {
			jsz = data
		}
	}

	// Resolve routez first so varz/jsz prefer the same monitoring base.
	data, usedBase, monErr := fetchMonitoringWithFailover(c, bases, "/routez")
	if monErr != nil {
		return nil, nil, nil, monErr
	}
	routez = data
	ordered := preferBase(bases, usedBase)

	// Fan-out /varz; first successful candidate is treated as monitored.
	varzs, err = fetchMonitoringAll(c, ordered, "/varz")
	if err != nil {
		return nil, nil, nil, err
	}

	if jsz == nil {
		// Lightweight path — replicas only need meta_cluster, not streams/consumers.
		data, _, monErr = fetchMonitoringWithFailover(c, ordered, "/jsz")
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
		return nil, nil, nil, errMonitoringTooLarge
	}
	return varzs, routez, jsz, nil
}

func preferBase(bases []string, preferred string) []string {
	preferred = strings.TrimRight(strings.TrimSpace(preferred), "/")
	if preferred == "" {
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
	CapturedAt      time.Time     `json:"capturedAt"`
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
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Uptime      string   `json:"uptime,omitempty"`
	RTT         string   `json:"rtt,omitempty"`
	Idle        string   `json:"idle,omitempty"`
	Version     string   `json:"version,omitempty"`
	IP          string   `json:"ip,omitempty"`
	Online      bool     `json:"online"`
	Current     bool     `json:"current"` // must not omitempty — false means lagging
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
	MetaCluster *jszMetaCluster `json:"meta_cluster"`
	ServerName  string          `json:"server_name"`
}

func buildReplicasSnapshot(varzRaws [][]byte, routezRaw, jszRaw []byte) (replicasSnapshot, error) {
	var out replicasSnapshot
	peers := make(map[string]*replicaPeer)

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
			out.JetStreamLeader = meta.Leader
			out.ClusterSize = meta.Size
			if p := upsert(meta.Leader); p != nil {
				p.Leader = true
				p.Current = true
			}
			for _, r := range meta.Replicas {
				p := upsert(r.Name)
				if p == nil {
					continue
				}
				p.Current = r.Current
				if r.Offline {
					// Live /varz presence wins over stale meta offline; routez alone does not.
					if p.CPU == nil && p.Mem == nil {
						p.Online = false
						p.RTT = ""
						p.Idle = ""
						p.InMsgs = nil
						p.OutMsgs = nil
						p.Pending = nil
					}
				} else if p.Role == "meta" {
					p.Online = true
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if !commonstrings.IsEmpty(v) {
			return v
		}
	}
	return ""
}
