//go:build ignore

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fasthttp/websocket"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ws_check BASE_URL CLUSTER_ID [COOKIE_JAR]")
		os.Exit(2)
	}
	baseURL := strings.TrimRight(os.Args[1], "/")
	clusterID := os.Args[2]

	headers := http.Header{}
	if auth := os.Getenv("AUTH"); auth != "" {
		headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)))
	}
	if len(os.Args) >= 4 && os.Args[3] != "" {
		if cookie := netscapeCookieHeader(os.Args[3]); cookie != "" {
			headers.Set("Cookie", cookie)
		}
	}

	wsURL := strings.Replace(baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = fmt.Sprintf("%s/api/v1/clusters/%s/live/ws?stream=LIVE_SMOKE", wsURL, clusterID)

	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		exitErr(fmt.Errorf("websocket dial: %w", err))
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		exitErr(err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		exitErr(fmt.Errorf("read connected frame: %w", err))
	}

	var frame struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &frame); err != nil {
		exitErr(fmt.Errorf("parse frame: %w", err))
	}
	if frame.Type != "connected" {
		exitErr(fmt.Errorf("expected connected frame, got %q", frame.Type))
	}
	fmt.Println("live websocket connected")
}

func netscapeCookieHeader(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var parts []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}
		parts = append(parts, fields[5]+"="+fields[6])
	}
	return strings.Join(parts, "; ")
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
