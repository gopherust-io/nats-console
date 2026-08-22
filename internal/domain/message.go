package domain

import (
	"time"

	libnats "github.com/gopherust-io/nats"
	"github.com/nats-io/nats.go"

	"github.com/gopherust-io/nats-consol/pkg/common/b64util"
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
	Headers map[string]string `json:"headers,omitempty"`
	Subject string            `json:"subject"`
	Time    string            `json:"time"`
	Data    string            `json:"data"`
	Seq     uint64            `json:"seq"`
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
	if msg.Header != nil {
		headers := make(map[string]string, len(msg.Header))
		for k, vals := range msg.Header {
			if len(vals) > 0 {
				headers[k] = vals[0]
			}
		}
		if len(headers) > 0 {
			out.Headers = headers
		}
	}
	return out
}

func StreamMessageFromStored(msg *libnats.StoredMessage) StreamMessage {
	if msg == nil {
		return StreamMessage{}
	}
	out := StreamMessage{
		Seq:     msg.Sequence,
		Subject: msg.Subject,
		Time:    msg.Time.UTC().Format(time.RFC3339Nano),
		Data:    b64util.EncodeToString(msg.Data),
	}
	if msg.Header != nil {
		headers := make(map[string]string, len(msg.Header))
		for k, vals := range msg.Header {
			if len(vals) > 0 {
				headers[k] = vals[0]
			}
		}
		if len(headers) > 0 {
			out.Headers = headers
		}
	}
	return out
}
