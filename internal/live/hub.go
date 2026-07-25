package live

import (
	"context"
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
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
)

const (
	defaultLiveWSMaxMessages = 1000
	defaultLiveWSIdleTimeout = 5 * time.Minute
	defaultLiveWSRateLimit   = 100 * time.Millisecond
	connPollInterval         = 2 * time.Second
)

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

func (h *Hub) checkOrigin(ctx *fasthttp.RequestCtx) bool {
	origins := h.cfg.CORSOrigins()
	if len(origins) == 0 {
		return true
	}
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return true
	}
	for _, allowed := range origins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
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

func requestContext(ctx *fasthttp.RequestCtx) context.Context {
	return httpctx.FromRequest(ctx)
}

func (h *Hub) Handle(ctx *fasthttp.RequestCtx) {
	clusterID, ok := ctx.UserValue("clusterId").(string)
	if !ok || clusterID == "" {
		ctx.Error("missing clusterId", fasthttp.StatusBadRequest)
		return
	}

	if !h.checkOrigin(ctx) {
		ctx.Error("origin not allowed", fasthttp.StatusForbidden)
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

	reqCtx := requestContext(ctx)
	client, err := h.gateway.GetExecutor(reqCtx, clusterID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			ctx.Error("cluster not found", fasthttp.StatusNotFound)
			return
		}
		ctx.Error(err.Error(), fasthttp.StatusBadGateway)
		return
	}

	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(c *fasthttp.RequestCtx) bool {
			return h.checkOrigin(c)
		},
	}

	err = upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		h.serveConn(conn, client, clusterID, stream, subjectFilter, fromSeq)
	})
	if err != nil {
		log.Error().Err(err).Str("component", "live").Msg("websocket upgrade failed")
	}
}

func (h *Hub) serveConn(conn *websocket.Conn, client port.JetStreamExecutor, clusterID, stream, subjectFilter string, fromSeq uint64) {
	defer func() { _ = conn.Close() }()
	metrics.IncWS()
	defer metrics.DecWS()

	sessionCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu         sync.Mutex
		writeMu    sync.Mutex
		idleMu     sync.Mutex
		paused     bool
		msgCount   int
		lastSent   time.Time
		maxReached bool
		closed     bool
	)

	idleTimer := time.NewTimer(h.liveWSIdleTimeout())
	defer idleTimer.Stop()

	resetIdle := func() {
		idleMu.Lock()
		defer idleMu.Unlock()
		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
		idleTimer.Reset(h.liveWSIdleTimeout())
	}

	writeFrameOnce := func(frame liveFrame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return h.writeFrame(conn, frame)
	}

	closeSession := func(message string) {
		mu.Lock()
		if closed {
			mu.Unlock()
			return
		}
		closed = true
		mu.Unlock()
		cancel()
		if message != "" {
			_ = writeFrameOnce(liveFrame{Type: "error", Error: message})
		}
		_ = conn.Close()
	}

	send := func(frame liveFrame) bool {
		mu.Lock()
		if closed {
			mu.Unlock()
			return false
		}
		mu.Unlock()
		if err := writeFrameOnce(frame); err != nil {
			closeSession("")
			return false
		}
		resetIdle()
		h.gateway.Touch(clusterID)
		return true
	}

	if !send(liveFrame{Type: "connected", Subject: stream}) {
		return
	}

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
		mu.Lock()
		if closed || paused || maxReached {
			mu.Unlock()
			return
		}
		if msgCount >= h.liveWSMaxMessages() {
			maxReached = true
			mu.Unlock()
			send(liveFrame{Type: "error", Error: "max messages reached"})
			closeSession("")
			return
		}
		if !lastSent.IsZero() && time.Since(lastSent) < h.liveWSRateLimit() {
			metrics.IncLiveWSFramesDropped()
			mu.Unlock()
			return
		}
		msgCount++
		lastSent = time.Now()
		mu.Unlock()

		seq := uint64(0)
		if meta, metaErr := msg.Metadata(); metaErr == nil && meta != nil {
			seq = meta.Sequence.Stream
		}
		if !send(messageLiveFrame(seq, msg.Subject, msg.Data, time.Now())) {
			return
		}
	}, subOpts...)
	if err != nil {
		closeSession(err.Error())
		return
	}
	defer func() { _ = sub.Unsubscribe() }()

	connPoll := time.NewTicker(connPollInterval)
	defer connPoll.Stop()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-sessionCtx.Done():
				return
			default:
			}
			// Idle is enforced by idleTimer (reset on send/recv), not read deadline.
			_ = conn.SetReadDeadline(time.Time{})
			_, data, readErr := conn.ReadMessage()
			if readErr != nil {
				closeSession("")
				return
			}

			resetIdle()
			h.gateway.Touch(clusterID)

			var ctrl controlFrame
			if err := sonic.Unmarshal(data, &ctrl); err != nil {
				continue
			}
			switch ctrl.Action {
			case "pause":
				mu.Lock()
				paused = true
				mu.Unlock()
				send(liveFrame{Type: "paused"})
			case "resume":
				mu.Lock()
				paused = false
				mu.Unlock()
				send(liveFrame{Type: "resumed"})
			case "clear":
				mu.Lock()
				msgCount = 0
				maxReached = false
				mu.Unlock()
				send(liveFrame{Type: "cleared"})
			}
		}
	}()

	for {
		select {
		case <-sessionCtx.Done():
			return
		case <-idleTimer.C:
			closeSession("idle timeout")
			return
		case <-readDone:
			return
		case <-connPoll.C:
			h.gateway.Touch(clusterID)
			nc := client.Conn()
			if nc == nil || !nc.IsConnected() {
				closeSession("nats disconnected")
				return
			}
		}
	}
}

func (h *Hub) writeFrame(conn *websocket.Conn, frame liveFrame) error {
	data, err := encodeLiveFrame(frame)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
