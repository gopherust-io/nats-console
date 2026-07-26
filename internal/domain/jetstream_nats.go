package domain

import (
	"maps"
	"time"

	"github.com/nats-io/nats.go"
)

func AccountInfoFromNATS(info *nats.AccountInfo) AccountInfo {
	if info == nil {
		return AccountInfo{}
	}
	return AccountInfo{
		Memory:    info.Memory,
		Storage:   info.Store,
		Streams:   info.Streams,
		Consumers: info.Consumers,
		Limits: AccountLimits{
			MaxMemory:    info.Limits.MaxMemory,
			MaxStorage:   info.Limits.MaxStore,
			MaxStreams:   info.Limits.MaxStreams,
			MaxConsumers: info.Limits.MaxConsumers,
		},
	}
}

func StreamInfoFromNATS(info *nats.StreamInfo) StreamInfo {
	if info == nil {
		return StreamInfo{}
	}
	return StreamInfo{
		Config:  streamConfigFromNATS(info.Config),
		State:   streamStateFromNATS(info.State),
		Created: info.Created,
	}
}

func StreamInfosFromNATS(items []*nats.StreamInfo) []StreamInfo {
	if len(items) == 0 {
		return []StreamInfo{}
	}
	out := make([]StreamInfo, len(items))
	for i, item := range items {
		out[i] = StreamInfoFromNATS(item)
	}
	return out
}

func streamConfigFromNATS(cfg nats.StreamConfig) StreamConfigDTO {
	out := StreamConfigDTO{
		Name:                   cfg.Name,
		Description:            cfg.Description,
		Subjects:               append([]string(nil), cfg.Subjects...),
		Retention:              enumString(cfg.Retention),
		Storage:                enumString(cfg.Storage),
		Discard:                enumString(cfg.Discard),
		Compression:            enumString(cfg.Compression),
		MaxMsgs:                cfg.MaxMsgs,
		MaxBytes:               cfg.MaxBytes,
		MaxAge:                 int64(cfg.MaxAge),
		MaxConsumers:           cfg.MaxConsumers,
		MaxMsgSize:             cfg.MaxMsgSize,
		MaxMsgsPerSubject:      cfg.MaxMsgsPerSubject,
		Replicas:               cfg.Replicas,
		Duplicates:             int64(cfg.Duplicates),
		FirstSeq:               cfg.FirstSeq,
		SubjectDeleteMarkerTTL: int64(cfg.SubjectDeleteMarkerTTL),
		AllowRollup:            cfg.AllowRollup,
		DenyDelete:             cfg.DenyDelete,
		DenyPurge:              cfg.DenyPurge,
		DiscardNewPerSubject:   cfg.DiscardNewPerSubject,
		NoAck:                  cfg.NoAck,
		Sealed:                 cfg.Sealed,
		AllowDirect:            cfg.AllowDirect,
		MirrorDirect:           cfg.MirrorDirect,
		AllowMsgTTL:            cfg.AllowMsgTTL,
		Metadata:               cloneStringMap(cfg.Metadata),
	}
	if cfg.Placement != nil && (cfg.Placement.Cluster != "" || len(cfg.Placement.Tags) > 0) {
		out.Placement = &StreamPlacementDTO{
			Cluster: cfg.Placement.Cluster,
			Tags:    append([]string(nil), cfg.Placement.Tags...),
		}
	}
	if cfg.Mirror != nil {
		out.Mirror = streamSourceFromNATS(cfg.Mirror)
	}
	if len(cfg.Sources) > 0 {
		out.Sources = make([]StreamSourceDTO, 0, len(cfg.Sources))
		for _, src := range cfg.Sources {
			if src == nil {
				continue
			}
			out.Sources = append(out.Sources, *streamSourceFromNATS(src))
		}
	}
	if cfg.SubjectTransform != nil && cfg.SubjectTransform.Destination != "" {
		out.SubjectTransform = &SubjectTransformDTO{
			Source:      cfg.SubjectTransform.Source,
			Destination: cfg.SubjectTransform.Destination,
		}
	}
	if cfg.RePublish != nil && cfg.RePublish.Destination != "" {
		out.RePublish = &RePublishDTO{
			Source:      cfg.RePublish.Source,
			Destination: cfg.RePublish.Destination,
			HeadersOnly: cfg.RePublish.HeadersOnly,
		}
	}
	if cfg.ConsumerLimits.InactiveThreshold > 0 || cfg.ConsumerLimits.MaxAckPending != 0 {
		out.ConsumerLimits = &StreamConsumerLimitsDTO{
			InactiveThreshold: int64(cfg.ConsumerLimits.InactiveThreshold),
			MaxAckPending:     cfg.ConsumerLimits.MaxAckPending,
		}
	}
	return out
}

func streamSourceFromNATS(src *nats.StreamSource) *StreamSourceDTO {
	if src == nil {
		return nil
	}
	out := &StreamSourceDTO{
		Name:          src.Name,
		FilterSubject: src.FilterSubject,
		OptStartSeq:   src.OptStartSeq,
	}
	if src.OptStartTime != nil {
		out.OptStartTime = src.OptStartTime.UTC().Format(time.RFC3339Nano)
	}
	if src.External != nil {
		out.External = &StreamExternalDTO{
			APIPrefix:     src.External.APIPrefix,
			DeliverPrefix: src.External.DeliverPrefix,
		}
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func streamStateFromNATS(state nats.StreamState) StreamStateDTO {
	return StreamStateDTO{
		Messages:      state.Msgs,
		Bytes:         state.Bytes,
		FirstSeq:      state.FirstSeq,
		LastSeq:       state.LastSeq,
		ConsumerCount: state.Consumers,
	}
}

func ConsumerInfoFromNATS(info *nats.ConsumerInfo) ConsumerInfo {
	if info == nil {
		return ConsumerInfo{}
	}
	out := ConsumerInfo{
		Name:           info.Name,
		StreamName:     info.Stream,
		Config:         consumerConfigFromNATS(info.Config),
		NumPending:     info.NumPending,
		NumAckPending:  info.NumAckPending,
		NumRedelivered: info.NumRedelivered,
		NumWaiting:     info.NumWaiting,
		Created:        info.Created,
	}
	if info.Delivered.Consumer != 0 || info.Delivered.Stream != 0 {
		out.Delivered = sequenceInfoFromNATS(info.Delivered)
	}
	if info.AckFloor.Consumer != 0 || info.AckFloor.Stream != 0 {
		out.AckFloor = sequenceInfoFromNATS(info.AckFloor)
	}
	return out
}

func ConsumerInfosFromNATS(items []*nats.ConsumerInfo) []ConsumerInfo {
	if len(items) == 0 {
		return []ConsumerInfo{}
	}
	out := make([]ConsumerInfo, len(items))
	for i, item := range items {
		out[i] = ConsumerInfoFromNATS(item)
	}
	return out
}

func consumerConfigFromNATS(cfg nats.ConsumerConfig) ConsumerConfigDTO {
	out := ConsumerConfigDTO{
		DurableName:         cfg.Durable,
		Name:                cfg.Name,
		Description:         cfg.Description,
		DeliverPolicy:       enumString(cfg.DeliverPolicy),
		AckPolicy:           enumString(cfg.AckPolicy),
		FilterSubject:       cfg.FilterSubject,
		FilterSubjects:      append([]string(nil), cfg.FilterSubjects...),
		OptStartSeq:         cfg.OptStartSeq,
		ReplayPolicy:        enumString(cfg.ReplayPolicy),
		AckWaitNs:           int64(cfg.AckWait),
		MaxDeliver:          cfg.MaxDeliver,
		RateLimitBps:        cfg.RateLimit,
		SampleFreq:          cfg.SampleFrequency,
		MaxWaiting:          cfg.MaxWaiting,
		MaxAckPending:       cfg.MaxAckPending,
		FlowControl:         cfg.FlowControl,
		HeartbeatNs:         int64(cfg.Heartbeat),
		HeadersOnly:         cfg.HeadersOnly,
		MaxRequestBatch:     cfg.MaxRequestBatch,
		MaxRequestExpiresNs: int64(cfg.MaxRequestExpires),
		MaxRequestMaxBytes:  cfg.MaxRequestMaxBytes,
		DeliverSubject:      cfg.DeliverSubject,
		DeliverGroup:        cfg.DeliverGroup,
		InactiveThresholdNs: int64(cfg.InactiveThreshold),
		Replicas:            cfg.Replicas,
		MemoryStorage:       cfg.MemoryStorage,
	}
	if cfg.OptStartTime != nil {
		out.OptStartTime = cfg.OptStartTime.UTC().Format(time.RFC3339Nano)
	}
	if len(cfg.BackOff) > 0 {
		out.BackoffNs = make([]int64, len(cfg.BackOff))
		for i, d := range cfg.BackOff {
			out.BackoffNs[i] = int64(d)
		}
	}
	if len(cfg.Metadata) > 0 {
		out.Metadata = make(map[string]string, len(cfg.Metadata))
		maps.Copy(out.Metadata, cfg.Metadata)
	}
	return out
}

func sequenceInfoFromNATS(info nats.SequenceInfo) *SequenceInfoDTO {
	return &SequenceInfoDTO{
		ConsumerSeq: info.Consumer,
		StreamSeq:   info.Stream,
	}
}
