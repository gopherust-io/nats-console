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

func requestContext(ctx *fasthttp.RequestCtx) context.Context {
	if c, ok := ctx.UserValue("context").(context.Context); ok && c != nil {
		return c
	}
	return context.Background()
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

	err = upgrader.Upgrade(ctx, func(conn *websocket.Conn) {
		h.serveConn(conn, client, stream, subjectFilter, fromSeq)
	})
	if err != nil {
		log.Error().Err(err).Str("component", "live").Msg("websocket upgrade failed")
	}
}

func (h *Hub) serveConn(conn *websocket.Conn, client port.JetStreamExecutor, stream, subjectFilter string, fromSeq uint64) {
	defer func() { _ = conn.Close() }()
	metrics.IncWS()
	defer metrics.DecWS()

	sessionCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu         sync.Mutex
		writeMu    sync.Mutex
		paused     bool
		msgCount   int
		lastSent   time.Time
		maxReached bool
		closed     bool
	)

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
		if !send(liveFrame{
			Type:    "message",
			Seq:     seq,
			Subject: msg.Subject,
			Time:    time.Now().UTC().Format(time.RFC3339Nano),
			Data:    base64.StdEncoding.EncodeToString(msg.Data),
		}) {
			return
		}
	}, subOpts...)
	if err != nil {
		closeSession(err.Error())
		return
	}
	defer func() { _ = sub.Unsubscribe() }()

	if nc := client.Conn(); nc != nil {
		if prev := nc.DisconnectErrHandler(); prev != nil {
			nc.SetDisconnectErrHandler(func(c *nats.Conn, err error) {
				prev(c, err)
				closeSession("nats disconnected")
			})
		} else {
			nc.SetDisconnectErrHandler(func(_ *nats.Conn, _ error) {
				closeSession("nats disconnected")
			})
		}
	}

	idleTimer := time.NewTimer(h.liveWSIdleTimeout())
	defer idleTimer.Stop()

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			select {
			case <-sessionCtx.Done():
				return
			default:
			}
			_ = conn.SetReadDeadline(time.Now().Add(h.liveWSIdleTimeout()))
			_, data, readErr := conn.ReadMessage()
			if readErr != nil {
				closeSession("")
				return
			}

			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(h.liveWSIdleTimeout())

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

	select {
	case <-sessionCtx.Done():
	case <-idleTimer.C:
		closeSession("idle timeout")
	case <-readDone:
	}
}

func (h *Hub) writeFrame(conn *websocket.Conn, frame liveFrame) error {
	data, err := sonic.Marshal(frame)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
