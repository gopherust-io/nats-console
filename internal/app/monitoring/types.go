package monitoring

// JSZPeerInfo mirrors NATS /jsz PeerInfo for topology and insight extractors.
//
// goalign:ignore // JSON DTO mirroring NATS PeerInfo; trailing bool padding is unavoidable.
type JSZPeerInfo struct {
	Lag     uint64 `json:"lag,omitempty"`
	Active  int64  `json:"active,omitempty"`
	Name    string `json:"name"`
	Peer    string `json:"peer,omitempty"`
	Current bool   `json:"current"`
	Offline bool   `json:"offline,omitempty"`
}

// JSZClusterInfo is the raft cluster block on a stream or consumer in /jsz.
type JSZClusterInfo struct {
	Name      string        `json:"name"`
	RaftGroup string        `json:"raft_group"`
	Leader    string        `json:"leader"`
	Replicas  []JSZPeerInfo `json:"replicas"`
}

// JSZMetaCluster is the JetStream meta cluster block in /jsz.
type JSZMetaCluster struct {
	Name     string        `json:"name"`
	Leader   string        `json:"leader"`
	Peer     string        `json:"peer"`
	Size     int           `json:"cluster_size"`
	Replicas []JSZPeerInfo `json:"replicas"`
}

// JSZTopologyPayload is the typed subset of /jsz used by topology and insight engines.
type JSZTopologyPayload struct {
	ServerName     string             `json:"server_name"`
	ServerID       string             `json:"server_id"`
	MetaCluster    *JSZMetaCluster    `json:"meta_cluster"`
	AccountDetails []JSZAccountDetail `json:"account_details"`
}

// JSZAccountDetail holds per-account stream detail from /jsz.
type JSZAccountDetail struct {
	Name         string            `json:"name"`
	StreamDetail []JSZStreamDetail `json:"stream_detail"`
}

// JSZStreamDetail holds stream config/state/consumers from /jsz.
type JSZStreamDetail struct {
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
	Cluster        *JSZClusterInfo     `json:"cluster"`
	ConsumerDetail []JSZConsumerDetail `json:"consumer_detail"`
}

// JSZConsumerDetail holds consumer config/delivered/pending from /jsz.
type JSZConsumerDetail struct {
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
	Cluster        *JSZClusterInfo `json:"cluster"`
}
