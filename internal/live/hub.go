package live

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/websocket"
	"github.com/nats-io/nats.go"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
)

const (
	defaultLiveWSMaxMessages = 1000
	defaultLiveWSIdleTimeout = 5 * time.Minute
	defaultLiveWSRateLimit   = 100 * time.Millisecond
)

var upgrader = websocket.FastHTTPUpgrader{
	CheckOrigin: func(_ *fasthttp.RequestCtx) bool { return true },
}

type Hub struct {
	gateway port.ClusterGateway
	cfg     config.Config
}

func NewHub(gateway port.ClusterGateway, cfg config.Config) *Hub {
	return &Hub{gateway: gateway, cfg: cfg}
}

func (h *Hub) liveWSMaxMessages() int {
	if h.cfg.LiveWSMaxMessages > 0 {
		return h.cfg.LiveWSMaxMessages
	}
	return defaultLiveWSMaxMessages
}

func (h *Hub) liveWSIdleTimeout() time.Duration {
	if h.cfg.LiveWSIdleTimeout > 0 {
		return h.cfg.LiveWSIdleTimeout
	}
	return defaultLiveWSIdleTimeout
}

func (h *Hub) liveWSRateLimit() time.Duration {
	if h.cfg.LiveWSRateLimit > 0 {
		return h.cfg.LiveWSRateLimit
	}
	return defaultLiveWSRateLimit
}

type controlFrame struct {
	Action string `json:"action"`
}

type liveFrame struct {
	Type    string `json:"type"`
	Subject string `json:"subject,omitempty"`
	Time    string `json:"time,omitempty"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Seq     uint64 `json:"seq,omitempty"`
}

func (h *Hub) Handle(ctx *fasthttp.RequestCtx) {
	clusterID, ok := ctx.UserValue("clusterId").(string)
	if !ok || clusterID == "" {
		ctx.Error("missing clusterId", fasthttp.StatusBadRequest)
		return
	}

	stream := string(ctx.QueryArgs().Peek("stream"))
	if stream == "" {
		ctx.Error("missing stream", fasthttp.StatusBadRequest)
		return
	}
	subjectFilter := string(ctx.QueryArgs().Peek("subject"))
	fromSeq := uint64(0)
	if v := string(ctx.QueryArgs().Peek("fromSeq")); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			ctx.Error("invalid fromSeq", fasthttp.StatusBadRequest)
			return
		}
		fromSeq = parsed
	}

	client, err := h.gateway.GetExecutor(context.Background(), clusterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			ctx.Error("cluster not found", fasthttp.StatusNotFound)
			return
		}
		ctx.Error(err.Error(), fasthttp.StatusBadGateway)
		return
	}

	err = upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		metrics.IncWS()
		defer metrics.DecWS()

		paused := false
		var pauseMu sync.Mutex
		msgCount := 0
		lastSent := time.Time{}

		send := func(frame liveFrame) {
			data, err := sonic.Marshal(frame)
			if err != nil {
				return
			}
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}

		send(liveFrame{Type: "connected", Subject: stream})

		subject := ">"
		subOpts := []nats.SubOpt{nats.BindStream(stream)}
		if subjectFilter != "" {
			subject = subjectFilter
		}
		if fromSeq > 0 {
			subOpts = append(subOpts, nats.StartSequence(fromSeq))
		} else {
			subOpts = append(subOpts, nats.DeliverNew())
		}

		sub, err := client.JetStream().Subscribe(subject, func(msg *nats.Msg) {
			pauseMu.Lock()
			p := paused
			pauseMu.Unlock()
			if p {
				return
			}
			if msgCount >= h.liveWSMaxMessages() {
				send(liveFrame{Type: "error", Error: "max messages reached"})
				return
			}
			if !lastSent.IsZero() && time.Since(lastSent) < h.liveWSRateLimit() {
				return
			}
			lastSent = time.Now()
			msgCount++

			meta, _ := msg.Metadata()
			seq := uint64(0)
			if meta != nil {
				seq = meta.Sequence.Stream
			}
			send(liveFrame{
				Type:    "message",
				Seq:     seq,
				Subject: msg.Subject,
				Time:    time.Now().UTC().Format(time.RFC3339Nano),
				Data:    base64.StdEncoding.EncodeToString(msg.Data),
			})
		}, subOpts...)
		if err != nil {
			send(liveFrame{Type: "error", Error: err.Error()})
			return
		}
		defer func() { _ = sub.Unsubscribe() }()

		idleTimer := time.NewTimer(h.liveWSIdleTimeout())
		defer idleTimer.Stop()

		for {
			select {
			case <-idleTimer.C:
				send(liveFrame{Type: "error", Error: "idle timeout"})
				return
			default:
			}

			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			idleTimer.Reset(h.liveWSIdleTimeout())

			var ctrl controlFrame
			if err := sonic.Unmarshal(data, &ctrl); err != nil {
				continue
			}
			switch ctrl.Action {
			case "pause":
				pauseMu.Lock()
				paused = true
				pauseMu.Unlock()
				send(liveFrame{Type: "paused"})
			case "resume":
				pauseMu.Lock()
				paused = false
				pauseMu.Unlock()
				send(liveFrame{Type: "resumed"})
			case "clear":
				msgCount = 0
				send(liveFrame{Type: "cleared"})
			}
		}
	})
	if err != nil {
		log.Error().Err(err).Str("component", "live").Msg("websocket upgrade failed")
	}
}
