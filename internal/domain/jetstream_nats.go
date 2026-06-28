package domain

import "github.com/nats-io/nats.go"

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
	return StreamConfigDTO{
		Name:      cfg.Name,
		Subjects:  append([]string(nil), cfg.Subjects...),
		Retention: enumString(cfg.Retention),
		Storage:   enumString(cfg.Storage),
		MaxMsgs:   cfg.MaxMsgs,
		MaxBytes:  cfg.MaxBytes,
		MaxAge:    int64(cfg.MaxAge),
	}
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
	return ConsumerConfigDTO{
		DurableName:    cfg.Durable,
		Name:           cfg.Name,
		DeliverPolicy:  enumString(cfg.DeliverPolicy),
		AckPolicy:      enumString(cfg.AckPolicy),
		FilterSubject:  cfg.FilterSubject,
		FilterSubjects: append([]string(nil), cfg.FilterSubjects...),
	}
}

func sequenceInfoFromNATS(info nats.SequenceInfo) *SequenceInfoDTO {
	return &SequenceInfoDTO{
		ConsumerSeq: info.Consumer,
		StreamSeq:   info.Stream,
	}
}
