package domain

import "time"

type SuperclusterOverview struct {
	FetchedAt         time.Time              `json:"fetchedAt"`
	MetaCluster       *SuperclusterMeta      `json:"metaCluster,omitempty"`
	ServerName        string                 `json:"serverName"`
	ClusterName       string                 `json:"clusterName,omitempty"`
	JetStreamDomain   string                 `json:"jetstreamDomain,omitempty"`
	Gateways          []SuperclusterGateway  `json:"gateways"`
	Routes            []SuperclusterRoute    `json:"routes"`
	Leafnodes         []SuperclusterLeafnode `json:"leafnodes"`
	StreamReplication []StreamReplication    `json:"streamReplication"`
	RouteCount        int                    `json:"routeCount"`
	LeafCount         int                    `json:"leafCount"`
	GatewayEnabled    bool                   `json:"gatewayEnabled"`
}

type SuperclusterGateway struct {
	Name          string `json:"name"`
	Direction     string `json:"direction"`
	Host          string `json:"host,omitempty"`
	Port          int    `json:"port,omitempty"`
	Connections   int    `json:"connections,omitempty"`
	Subscriptions int    `json:"subscriptions,omitempty"`
}

type SuperclusterRoute struct {
	RemoteID  string `json:"remoteId,omitempty"`
	URL       string `json:"url,omitempty"`
	Connected bool   `json:"connected"`
	InMsgs    uint64 `json:"inMsgs,omitempty"`
	OutMsgs   uint64 `json:"outMsgs,omitempty"`
}

type SuperclusterLeafnode struct {
	Name      string `json:"name,omitempty"`
	Remote    string `json:"remote,omitempty"`
	RTT       string `json:"rtt,omitempty"`
	Connected bool   `json:"connected"`
}

type SuperclusterMeta struct {
	Leader      string                `json:"leader,omitempty"`
	Replicas    []SuperclusterReplica `json:"replicas,omitempty"`
	ClusterSize int                   `json:"clusterSize,omitempty"`
}

type SuperclusterReplica struct {
	Name    string `json:"name"`
	ID      string `json:"id,omitempty"`
	Active  string `json:"active,omitempty"`
	Lag     uint64 `json:"lag,omitempty"`
	Leader  bool   `json:"leader"`
	Current bool   `json:"current"`
	Online  bool   `json:"online"`
}

type StreamReplication struct {
	StreamName   string `json:"streamName"`
	Kind         string `json:"kind"`
	TargetName   string `json:"targetName"`
	TargetDomain string `json:"targetDomain,omitempty"`
	Error        string `json:"error,omitempty"`
	Lag          uint64 `json:"lag,omitempty"`
	Active       bool   `json:"active"`
}
