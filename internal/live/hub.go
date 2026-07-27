package live

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
)

const (
	defaultLiveWSMaxMessages = 1000
	defaultLiveWSIdleTimeout = 5 * time.Minute
	defaultLiveWSRateLimit   = 100 * time.Millisecond
	connPollInterval         = 2 * time.Second
	defaultGatewayTouchMax   = 30 * time.Second
)

type Hub struct {
	gateway port.ClusterGateway
	mux     *subMux
	cfg     config.Config
}

func NewHub(gateway port.ClusterGateway, cfg config.Config) *Hub {
	return &Hub{gateway: gateway, cfg: cfg, mux: newSubMux()}
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

func (h *Hub) gatewayTouchInterval() time.Duration {
	ttl := h.cfg.NATSClientCacheTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	interval := min(ttl/3, defaultGatewayTouchMax)
	return max(interval, time.Second)
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

	reqCtx := httpctx.FromRequest(ctx)
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
		tel.Error().Err(err).Str("component", "live").Msg("websocket upgrade failed")
	}
}

func (h *Hub) serveConn(conn *websocket.Conn, client port.JetStreamExecutor, clusterID, stream, subjectFilter string, fromSeq uint64) {
	defer func() { _ = conn.Close() }()
	metrics.IncWS()
	defer metrics.DecWS()

	sessionCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu               sync.Mutex
		writeMu          sync.Mutex
		idleMu           sync.Mutex
		lastGatewayTouch atomic.Int64
		lastActivityNano atomic.Int64
		paused           atomic.Bool
		closed           atomic.Bool
		msgCount         int
		lastSent         time.Time
	)

	idleTimer := time.NewTimer(h.liveWSIdleTimeout())
	defer idleTimer.Stop()
	lastActivityNano.Store(time.Now().UnixNano())
	touchInterval := h.gatewayTouchInterval()
	idleTimeout := h.liveWSIdleTimeout()

	touchGateway := func() {
		now := time.Now().UnixNano()
		prev := lastGatewayTouch.Load()
		if now-prev < int64(touchInterval) {
			return
		}
		if !lastGatewayTouch.CompareAndSwap(prev, now) {
			return
		}
		h.gateway.Touch(clusterID)
	}

	resetIdle := func() {
		now := time.Now()
		lastActivityNano.Store(now.UnixNano())
		idleMu.Lock()
		defer idleMu.Unlock()
		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
		idleTimer.Reset(idleTimeout)
	}

	writeFrameOnce := func(frame liveFrame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return h.writeFrame(conn, frame)
	}

	writeBytesOnce := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	closeSession := func(message string) {
		if !closed.CompareAndSwap(false, true) {
			return
		}
		cancel()
		if message != "" {
			_ = writeFrameOnce(liveFrame{Type: "error", Error: message})
		}
		_ = conn.Close()
	}

	send := func(frame liveFrame) bool {
		if closed.Load() {
			return false
		}
		if err := writeFrameOnce(frame); err != nil {
			closeSession("")
			return false
		}
		resetIdle()
		touchGateway()
		return true
	}

	sendMessage := func(seq uint64, subject string, payload []byte, now time.Time) bool {
		mu.Lock()
		if closed.Load() {
			mu.Unlock()
			return false
		}
		if msgCount >= h.liveWSMaxMessages() {
			mu.Unlock()
			send(liveFrame{Type: "error", Error: "max messages reached"})
			closeSession("")
			return false
		}
		if !lastSent.IsZero() && time.Since(lastSent) < h.liveWSRateLimit() {
			metrics.IncLiveWSFramesDropped()
			mu.Unlock()
			return false
		}
		msgCount++
		lastSent = time.Now()
		mu.Unlock()

		data, err := encodeMessageFrame(seq, subject, payload, now)
		if err != nil {
			closeSession("")
			return false
		}
		if err := writeBytesOnce(data); err != nil {
			closeSession("")
			return false
		}
		resetIdle()
		touchGateway()
		return true
	}

	if !send(liveFrame{Type: "connected", Subject: stream}) {
		return
	}

	viewer := &muxViewer{
		send:     sendMessage,
		paused:   &paused,
		closed:   &closed,
		truncate: h.cfg.LivePayloadTruncate(),
	}
	unsub, err := h.mux.attach(client, muxKey{cluster: clusterID, stream: stream, filter: subjectFilter}, fromSeq, viewer)
	if err != nil {
		closeSession(err.Error())
		return
	}
	defer unsub()

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
			_ = conn.SetReadDeadline(time.Time{})
			_, data, readErr := conn.ReadMessage()
			if readErr != nil {
				closeSession("")
				return
			}

			resetIdle()
			touchGateway()

			var ctrl controlFrame
			if err := sonic.Unmarshal(data, &ctrl); err != nil {
				continue
			}
			switch ctrl.Action {
			case "pause":
				paused.Store(true)
				send(liveFrame{Type: "paused"})
			case "resume":
				paused.Store(false)
				send(liveFrame{Type: "resumed"})
			case "clear":
				mu.Lock()
				msgCount = 0
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
			if time.Since(time.Unix(0, lastActivityNano.Load())) < idleTimeout {
				idleMu.Lock()
				idleTimer.Reset(idleTimeout)
				idleMu.Unlock()
				continue
			}
			closeSession("idle timeout")
			return
		case <-readDone:
			return
		case <-connPoll.C:
			touchGateway()
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
