package natsclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/port"
)

var errSuperclusterUnavailable = errors.New("supercluster monitoring unavailable")

func BuildSuperclusterOverview(ctx context.Context, client port.JetStreamExecutor) (domain.SuperclusterOverview, error) {
	out := domain.SuperclusterOverview{
		FetchedAt:    time.Now().UTC(),
		SourceErrors: map[string]string{},
	}
	successes := 0

	record := func(source string, err error) {
		if err != nil {
			out.SourceErrors[source] = err.Error()
			return
		}
		successes++
	}

	if raw, err := client.Monitoring(ctx, "/varz"); err != nil {
		record("varz", err)
	} else {
		record("varz", applyVarz(raw, &out))
	}

	if raw, err := client.Monitoring(ctx, "/gatewayz"); err != nil {
		record("gatewayz", err)
	} else {
		gateways, parseErr := parseGateways(raw)
		if parseErr == nil {
			out.Gateways = gateways
		}
		record("gatewayz", parseErr)
	}

	if raw, err := client.Monitoring(ctx, "/routez"); err != nil {
		record("routez", err)
	} else {
		routes, parseErr := parseRoutes(raw)
		if parseErr == nil {
			out.Routes = routes
		}
		record("routez", parseErr)
	}

	if raw, err := client.Monitoring(ctx, "/leafz"); err != nil {
		record("leafz", err)
	} else {
		leafnodes, parseErr := parseLeafnodes(raw)
		if parseErr == nil {
			out.Leafnodes = leafnodes
		}
		record("leafz", parseErr)
	}

	if raw, err := client.Monitoring(ctx, "/jsz?raft=1&streams=1&config=1&leader-only=1"); err != nil {
		record("jsz", err)
	} else {
		record("jsz", applyJSZ(raw, &out))
	}

	if successes == 0 {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		return out, fmt.Errorf("%w: %v", errSuperclusterUnavailable, out.SourceErrors)
	}

	if len(out.SourceErrors) > 0 {
		out.Warnings = make([]string, 0, len(out.SourceErrors))
		for source, msg := range out.SourceErrors {
			out.Warnings = append(out.Warnings, source+": "+msg)
		}
	}

	normalizeSuperclusterOverview(&out)
	if len(out.SourceErrors) == 0 {
		out.SourceErrors = nil
	}
	return out, nil
}

func normalizeSuperclusterOverview(out *domain.SuperclusterOverview) {
	if out.Gateways == nil {
		out.Gateways = []domain.SuperclusterGateway{}
	}
	if out.Routes == nil {
		out.Routes = []domain.SuperclusterRoute{}
	}
	if out.Leafnodes == nil {
		out.Leafnodes = []domain.SuperclusterLeafnode{}
	}
	if out.StreamReplication == nil {
		out.StreamReplication = []domain.StreamReplication{}
	}
}

func applyVarz(raw []byte, out *domain.SuperclusterOverview) error {
	var payload struct {
		ServerName string `json:"server_name"`
		Cluster    struct {
			Name string `json:"name"`
		} `json:"cluster"`
		Gateway struct {
			Name string `json:"name"`
		} `json:"gateway"`
		Routes    int `json:"routes"`
		Leafnodes int `json:"leafnodes"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	out.ServerName = payload.ServerName
	out.ClusterName = payload.Cluster.Name
	out.GatewayEnabled = payload.Gateway.Name != ""
	out.RouteCount = payload.Routes
	out.LeafCount = payload.Leafnodes
	return nil
}

func parseGateways(raw []byte) ([]domain.SuperclusterGateway, error) {
	var payload struct {
		OutboundGateways []gatewayEntry `json:"outbound_gateways"`
		InboundGateways  []gatewayEntry `json:"inbound_gateways"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var out []domain.SuperclusterGateway
	for _, gw := range payload.OutboundGateways {
		out = append(out, gatewayToDomain(gw, "outbound"))
	}
	for _, gw := range payload.InboundGateways {
		out = append(out, gatewayToDomain(gw, "inbound"))
	}
	return out, nil
}

type gatewayEntry struct {
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Connections   int    `json:"connections"`
	Subscriptions int    `json:"subscriptions"`
}

func gatewayToDomain(gw gatewayEntry, direction string) domain.SuperclusterGateway {
	return domain.SuperclusterGateway{
		Name:          gw.Name,
		Direction:     direction,
		Host:          gw.Host,
		Port:          gw.Port,
		Connections:   gw.Connections,
		Subscriptions: gw.Subscriptions,
	}
}

func parseRoutes(raw []byte) ([]domain.SuperclusterRoute, error) {
	var payload struct {
		Routes []struct {
			RemoteID  string `json:"remote_id"`
			URL       string `json:"url"`
			DidRemove bool   `json:"did_remove"`
			InMsgs    uint64 `json:"in_msgs"`
			OutMsgs   uint64 `json:"out_msgs"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.SuperclusterRoute, 0, len(payload.Routes))
	for _, route := range payload.Routes {
		out = append(out, domain.SuperclusterRoute{
			RemoteID:  route.RemoteID,
			URL:       route.URL,
			Connected: !route.DidRemove,
			InMsgs:    route.InMsgs,
			OutMsgs:   route.OutMsgs,
		})
	}
	return out, nil
}

func parseLeafnodes(raw []byte) ([]domain.SuperclusterLeafnode, error) {
	var payload struct {
		Leafs []struct {
			Name      string `json:"name"`
			Remote    string `json:"remote"`
			RTT       string `json:"rtt"`
			DidRemove bool   `json:"did_remove"`
		} `json:"leafs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	out := make([]domain.SuperclusterLeafnode, 0, len(payload.Leafs))
	for _, leaf := range payload.Leafs {
		out = append(out, domain.SuperclusterLeafnode{
			Name:      leaf.Name,
			Remote:    leaf.Remote,
			Connected: !leaf.DidRemove,
			RTT:       leaf.RTT,
		})
	}
	return out, nil
}

func applyJSZ(raw []byte, out *domain.SuperclusterOverview) error {
	var payload struct {
		Domain         string `json:"domain"`
		AccountDetails []struct {
			StreamDetail []jszStreamDetail `json:"stream_detail"`
		} `json:"account_details"`
		Meta        metaJSZ `json:"meta"`
		MetaCluster metaJSZ `json:"meta_cluster"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	out.JetStreamDomain = payload.Domain
	meta := payload.MetaCluster
	if meta.Name == "" && len(meta.Replicas) == 0 {
		meta = payload.Meta
	}
	if meta.Name != "" || len(meta.Replicas) > 0 {
		replicas := make([]domain.SuperclusterReplica, 0, len(meta.Replicas))
		for _, rep := range meta.Replicas {
			replicas = append(replicas, domain.SuperclusterReplica{
				Name:    rep.Name,
				ID:      rep.ID,
				Leader:  rep.Leader,
				Current: rep.Current,
				Online:  rep.Active != 0 || rep.Current,
				Active:  rep.ActiveStr,
				Lag:     rep.Lag,
			})
		}
		out.MetaCluster = &domain.SuperclusterMeta{
			Leader:      meta.Leader,
			ClusterSize: meta.ClusterSize,
			Replicas:    replicas,
		}
	}

	for _, account := range payload.AccountDetails {
		for _, stream := range account.StreamDetail {
			out.StreamReplication = append(out.StreamReplication, replicationFromStream(stream)...)
		}
	}
	return nil
}

type metaJSZ struct {
	Name     string `json:"name"`
	Leader   string `json:"leader"`
	Replicas []struct {
		Name      string `json:"name"`
		ID        string `json:"id"`
		ActiveStr string `json:"active_str"`
		Active    int64  `json:"active"`
		Lag       uint64 `json:"lag"`
		Leader    bool   `json:"leader"`
		Current   bool   `json:"current"`
	} `json:"replicas"`
	ClusterSize int `json:"cluster_size"`
}

type jszStreamDetail struct {
	Name    string          `json:"name"`
	Mirror  *jszSourceInfo  `json:"mirror"`
	Sources []jszSourceInfo `json:"sources"`
}

type jszSourceInfo struct {
	External *struct {
		APIPrefix string `json:"api_prefix"`
	} `json:"external"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Error  struct {
		APIError struct {
			Description string `json:"description"`
		} `json:"api_error"`
	} `json:"error"`
	Lag    uint64 `json:"lag"`
	Active bool   `json:"active"`
}

func replicationFromStream(stream jszStreamDetail) []domain.StreamReplication {
	var out []domain.StreamReplication
	if stream.Mirror != nil {
		out = append(out, sourceInfoToReplication(stream.Name, "mirror", *stream.Mirror))
	}
	for _, source := range stream.Sources {
		out = append(out, sourceInfoToReplication(stream.Name, "source", source))
	}
	return out
}

func sourceInfoToReplication(streamName, kind string, info jszSourceInfo) domain.StreamReplication {
	targetDomain := info.Domain
	if targetDomain == "" && info.External != nil {
		targetDomain = info.External.APIPrefix
	}
	errMsg := info.Error.APIError.Description
	return domain.StreamReplication{
		StreamName:   streamName,
		Kind:         kind,
		TargetName:   info.Name,
		TargetDomain: targetDomain,
		Active:       info.Active,
		Lag:          info.Lag,
		Error:        errMsg,
	}
}
