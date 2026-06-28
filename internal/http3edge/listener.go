package http3edge

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/log"
	"github.com/quic-go/quic-go/http3"
)

type Listener struct {
	server *http3.Server
	wg     sync.WaitGroup
}

func Start(cfg config.Config) (*Listener, error) {
	if !cfg.HTTP3Enabled {
		return nil, nil
	}
	if cfg.HTTP3CertFile == "" || cfg.HTTP3KeyFile == "" {
		return nil, errors.New("HTTP3_CERT_FILE and HTTP3_KEY_FILE are required when HTTP3_ENABLED=true")
	}

	backend, err := url.Parse("http://" + cfg.HTTP3BackendAddr())
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).Str("component", "http3").Str("path", r.URL.Path).Msg("reverse proxy error")
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	cert, err := tls.LoadX509KeyPair(cfg.HTTP3CertFile, cfg.HTTP3KeyFile)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{http3.NextProtoH3},
	}

	l := &Listener{
		server: &http3.Server{
			Addr:      cfg.HTTP3Addr,
			Handler:   proxy,
			TLSConfig: tlsConfig,
		},
	}

	l.wg.Go(func() {
		log.Info().Str("component", "http3").Str("addr", cfg.HTTP3Addr).Msg("HTTP/3 listener started")
		if err := l.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Str("component", "http3").Msg("HTTP/3 listener failed")
		}
	})

	return l, nil
}

func (l *Listener) Stop() {
	if l == nil || l.server == nil {
		return
	}
	_ = l.server.Close()
	l.wg.Wait()
}
