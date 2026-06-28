//go:build integration

package integration_test

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/fasthttp/websocket"
	"github.com/nats-io/nats.go"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiveWS(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"LIVE_SMOKE","subjects":["live.>"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create stream: %s", string(respBody))

	dialer := websocket.Dialer{
		NetDialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return srv.DialConn()
		},
	}
	wsURL := "ws://nats-consol.local/api/v1/clusters/" + clusterID + "/live/ws?stream=LIVE_SMOKE"
	conn, _, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	connected := readLiveFrame(t, conn, 5*time.Second)
	require.Equal(t, "connected", connected["type"])
	assert.Equal(t, "LIVE_SMOKE", connected["subject"])

	nc, err := nats.Connect(stack.Cfg.NATSURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })
	js, err := nc.JetStream()
	require.NoError(t, err)

	_, err = js.Publish("live.test", []byte("hello-live"))
	require.NoError(t, err)

	msg := readLiveFrame(t, conn, 5*time.Second)
	require.Equal(t, "message", msg["type"])
	assert.Equal(t, "live.test", msg["subject"])
	decoded, err := base64.StdEncoding.DecodeString(msg["data"].(string))
	require.NoError(t, err)
	assert.Equal(t, "hello-live", string(decoded))

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"pause"}`)))
	paused := readLiveFrame(t, conn, 2*time.Second)
	require.Equal(t, "paused", paused["type"])

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"resume"}`)))
	resumed := readLiveFrame(t, conn, 2*time.Second)
	require.Equal(t, "resumed", resumed["type"])
}

func readLiveFrame(t *testing.T, conn *websocket.Conn, timeout time.Duration) map[string]any {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(timeout)))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	var frame map[string]any
	require.NoError(t, sonic.Unmarshal(data, &frame))
	return frame
}
