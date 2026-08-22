package live

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/tel"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/metrics"
	"github.com/gopherust-io/nats-consol/internal/port"
	"github.com/gopherust-io/nats-consol/pkg/common/safe"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
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
	if h.cfg.LiveWS.MaxMessages > 0 {
		return h.cfg.LiveWS.MaxMessages
	}
	return defaultLiveWSMaxMessages
}

func (h *Hub) liveWSIdleTimeout() time.Duration {
	if h.cfg.LiveWS.IdleTimeout > 0 {
		return h.cfg.LiveWS.IdleTimeout
	}
	return defaultLiveWSIdleTimeout
}

func (h *Hub) liveWSRateLimit() time.Duration {
	if h.cfg.LiveWS.RateLimit > 0 {
		return h.cfg.LiveWS.RateLimit
	}
	return defaultLiveWSRateLimit
}

func (h *Hub) gatewayTouchInterval() time.Duration {
	ttl := h.cfg.NATS.ClientCacheTTL
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
	origin := strings.BytesToString(ctx.Request.Header.Peek("Origin"))
	if strings.IsEmpty(origin) {
		return true
	}
	for _, allowed := range origins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

type FrameAction string

const (
	Pause  FrameAction = "pause"
	Resume FrameAction = "resume"
	Clear  FrameAction = "clear"
)

type FrameType string

const (
	Paused    FrameType = "paused"
	Resumed   FrameType = "resumed"
	Cleared   FrameType = "cleared"
	Error     FrameType = "error"
	Connected FrameType = "connected"
)

func (ft FrameType) String() string {
	return string(ft)
}

type liveFrame struct {
	Type    FrameType         `json:"type"`
	Subject string            `json:"subject,omitempty"`
	Time    string            `json:"time,omitempty"`
	Data    string            `json:"data,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Error   string            `json:"error,omitempty"`
	Seq     uint64            `json:"seq,omitempty"`
}

// Handle godoc
//
// @Summary Handle
// @Tags Live
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/live/ws [get]
func (h *Hub) Handle(ctx *fasthttp.RequestCtx) {
	clusterID, ok := ctx.UserValue("clusterId").(string)
	if !ok || strings.IsEmpty(clusterID) {
		ctx.Error("missing clusterId", fasthttp.StatusBadRequest)
		return
	}

	if !h.checkOrigin(ctx) {
		ctx.Error("origin not allowed", fasthttp.StatusForbidden)
		return
	}

	stream := strings.BytesToString(ctx.QueryArgs().Peek("stream"))
	if strings.IsEmpty(stream) {
		ctx.Error("missing stream", fasthttp.StatusBadRequest)
		return
	}
	subjectFilter := strings.BytesToString(ctx.QueryArgs().Peek("subject"))
	fromSeq := uint64(0)
	if v := strings.BytesToString(ctx.QueryArgs().Peek("fromSeq")); !strings.IsEmpty(v) {
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
		tel.Error().Err(err).Str("component", "live").Msg("live websocket gateway failed")
		ctx.Error("NATS is unavailable", fasthttp.StatusBadGateway)
		return
	}

	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(c *fasthttp.RequestCtx) bool {
			return h.checkOrigin(c)
		},
	}

	err = upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		h.serveConn(ctx, conn, client, clusterID, stream, subjectFilter, fromSeq)
	})
	if err != nil {
		tel.Error().Err(err).Str("component", "live").Msg("websocket upgrade failed")
	}
}

func (h *Hub) serveConn(
	ctx context.Context,
	conn *websocket.Conn,
	client port.JetStreamExecutor,
	clusterID,
	stream,
	subjectFilter string,
	fromSeq uint64) {

	defer func() { _ = conn.Close() }()
	metrics.IncWS()
	defer metrics.DecWS()

	sessionCtx, cancel := context.WithCancel(ctx)
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
		if !strings.IsEmpty(message) {
			_ = writeFrameOnce(liveFrame{Type: Error, Error: message})
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

	sendMessageFrame := func(frame []byte) bool {
		mu.Lock()
		if closed.Load() {
			mu.Unlock()
			return false
		}
		if msgCount >= h.liveWSMaxMessages() {
			mu.Unlock()
			send(liveFrame{Type: Error, Error: "max messages reached"})
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

		if err := writeBytesOnce(frame); err != nil {
			closeSession("")
			return false
		}
		resetIdle()
		touchGateway()
		return true
	}

	if !send(liveFrame{Type: Connected, Subject: stream}) {
		return
	}

	viewer := newMuxViewer(sendMessageFrame, &paused, &closed, h.cfg.LiveWS.PayloadTruncateBytes)
	unsub, err := h.mux.attach(client, muxKey{cluster: clusterID, stream: stream, filter: subjectFilter, fromSeq: fromSeq}, fromSeq, viewer)
	if err != nil {
		closeSession(err.Error())
		return
	}
	defer unsub()

	connPoll := time.NewTicker(connPollInterval)
	defer connPoll.Stop()

	readDone := make(chan struct{})
	const component = "live_ws"

	go func() {
		defer close(readDone)
		defer safe.Recover(component)
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

			switch parseControlAction(data) {
			case Pause:
				paused.Store(true)
				send(liveFrame{Type: Paused})
			case Resume:
				paused.Store(false)
				send(liveFrame{Type: Resumed})
			case Clear:
				mu.Lock()
				msgCount = 0
				mu.Unlock()
				send(liveFrame{Type: Cleared})
			}
		}
	}()

	defer safe.Recover("live_ws")
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
