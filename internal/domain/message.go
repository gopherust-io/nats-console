package domain

import (
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
	"github.com/nats-io/nats.go"
)

type MessageResult struct {
	Navigation MessageNavigation `json:"navigation"`
	Message    StreamMessage     `json:"message"`
}

type MessageNavigation struct {
	PrevSeq *uint64 `json:"prevSeq,omitempty"`
	NextSeq *uint64 `json:"nextSeq,omitempty"`
}

type StreamMessage struct {
	Subject string `json:"subject"`
	Time    string `json:"time"`
	Data    string `json:"data"`
	Seq     uint64 `json:"seq"`
}

type PublishMessageRequest struct {
	Headers map[string]string `json:"headers,omitempty"`
	Subject string            `json:"subject"`
	Data    string            `json:"data"`
}

type PublishMessageResult struct {
	Stream  string `json:"stream"`
	Subject string `json:"subject"`
	Seq     uint64 `json:"seq"`
}

func StreamMessageFromRaw(msg *nats.RawStreamMsg) StreamMessage {
	if msg == nil {
		return StreamMessage{}
	}
	out := StreamMessage{
		Seq:     msg.Sequence,
		Subject: msg.Subject,
		Time:    msg.Time.UTC().Format(time.RFC3339Nano),
		Data:    b64util.EncodeToString(msg.Data),
	}
	return out
}
