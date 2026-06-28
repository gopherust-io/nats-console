package natsclient

import (
	"net/http"
	"sync"
	"time"
)

var (
	monitoringTransport     *http.Transport
	monitoringTransportOnce sync.Once
)

func monitoringHTTPClient(timeout time.Duration) *http.Client {
	monitoringTransportOnce.Do(func() {
		monitoringTransport = &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		}
	})
	return &http.Client{
		Timeout:   timeout,
		Transport: monitoringTransport,
	}
}
