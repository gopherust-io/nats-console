package apikit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gopherust-io/nats-consol/pkg/common/safe"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const (
	// Per-request client ceiling; per-hop contexts use replicasMonitorHopTimeout.
	replicasMonitorTimeout = 3 * time.Second
	// Failover hop budget — keep short so a dead primary cannot stall SSE ~tens of seconds.
	replicasMonitorHopTimeout = time.Second
	// defaultMonitoringBodyLimit matches config MaxMonitoringBodyBytes default (8 MiB).
	defaultMonitoringBodyLimit int64 = 8 << 20
)

var monitoringHTTPClient = &http.Client{
	Timeout: replicasMonitorTimeout,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialMonitoringContext,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("monitoring redirect: stopped after 5 redirects")
		}
		if req.URL == nil {
			return errors.New("monitoring redirect: empty url")
		}
		if err := ValidateMonitoringURL(req.URL.String()); err != nil {
			return err
		}
		return validateMonitoringHostResolved(req.Context(), req.URL.Hostname())
	},
}

func dialMonitoringContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("monitoring dns: %w", err)
	}
	var dialer net.Dialer
	var lastErr error
	for _, a := range ips {
		if isBlockedMonitoringIP(a.IP) {
			lastErr = errors.New("monitoring url host not allowed")
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(a.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("monitoring url host not allowed")
	}
	return nil, lastErr
}

func isBlockedMonitoringIP(ip net.IP) bool {
	return ip == nil || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func validateMonitoringHostResolved(ctx context.Context, host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if commonstrings.IsEmpty(host) {
		return errors.New("monitoring redirect: host not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedMonitoringIP(ip) {
			return errors.New("monitoring redirect: host not allowed")
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("monitoring dns: %w", err)
	}
	for _, a := range addrs {
		if isBlockedMonitoringIP(a.IP) {
			return errors.New("monitoring redirect: host not allowed")
		}
	}
	return nil
}

// MonitoringCandidates builds failover HTTP bases from the primary monitoring URL
// and each NATS client URL (client port + 4000 → monitor port, e.g. 4222→8222).
func MonitoringCandidates(natsURL, monitoringURL string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	add := func(raw string) {
		raw = strings.TrimRight(strings.TrimSpace(raw), "/")
		if commonstrings.IsEmpty(raw) {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}

	add(monitoringURL)

	scheme := "http"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(monitoringURL)), "https://") {
		scheme = "https"
	}

	for part := range strings.SplitSeq(natsURL, ",") {
		part = strings.TrimSpace(part)
		if commonstrings.IsEmpty(part) {
			continue
		}
		u, err := url.Parse(part)
		if err != nil || commonstrings.IsEmpty(u.Hostname()) {
			continue
		}
		port := u.Port()
		if commonstrings.IsEmpty(port) {
			port = "4222"
		}
		clientPort, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		monPort := clientPort + 4000
		host := u.Hostname()
		if strings.Contains(host, ":") {
			host = net.JoinHostPort(host, strconv.Itoa(monPort))
			add(fmt.Sprintf("%s://%s", scheme, host))
			continue
		}
		add(fmt.Sprintf("%s://%s:%d", scheme, host, monPort))
	}
	return out
}

func FetchMonitoringWithFailover(ctx context.Context, bases []string, path string) (body []byte, used string, err error) {
	if len(bases) == 0 {
		return nil, "", errors.New("no monitoring URLs")
	}
	path = normalizeMonitorPath(path)

	var lastErr error
	for _, base := range bases {
		base = strings.TrimRight(base, "/")
		hopCtx, cancel := context.WithTimeout(ctx, replicasMonitorHopTimeout)
		raw, doErr := getMonitoring(hopCtx, monitoringHTTPClient, base, path, defaultMonitoringBodyLimit)
		cancel()
		if doErr != nil {
			lastErr = doErr
			continue
		}
		return raw, base, nil
	}
	if lastErr == nil {
		lastErr = errors.New("monitoring unreachable")
	}
	return nil, "", lastErr
}

// FetchMonitoringAll scrapes path from every base in parallel. Results keep
// candidate order; unreachable bases are omitted. Returns an error only when
// every base fails.
func FetchMonitoringAll(ctx context.Context, bases []string, path string) ([][]byte, error) {
	if len(bases) == 0 {
		return nil, errors.New("no monitoring URLs")
	}
	path = normalizeMonitorPath(path)

	type result struct {
		err  error
		body []byte
		idx  int
	}
	ch := make(chan result, len(bases))
	var wg sync.WaitGroup
	for i, base := range bases {
		wg.Go(func() {
			// WaitGroup.Go already Done()s when f returns — do not call Done again.
			defer func() {
				if rec := recover(); rec != nil {
					safe.Log("monitoring", rec)
					ch <- result{idx: i, err: fmt.Errorf("panic: %v", rec)}
				}
			}()
			base = strings.TrimRight(base, "/")
			hopCtx, cancel := context.WithTimeout(ctx, replicasMonitorHopTimeout)
			body, err := getMonitoring(hopCtx, monitoringHTTPClient, base, path, defaultMonitoringBodyLimit)
			cancel()
			ch <- result{idx: i, body: body, err: err}
		})
	}
	wg.Wait()
	close(ch)

	byIdx := make([][]byte, len(bases))
	var lastErr error
	ok := 0
	for r := range ch {
		if r.err != nil {
			lastErr = r.err
			continue
		}
		byIdx[r.idx] = r.body
		ok++
	}
	if ok == 0 {
		if lastErr == nil {
			lastErr = errors.New("monitoring unreachable")
		}
		return nil, lastErr
	}
	out := make([][]byte, 0, ok)
	for _, body := range byIdx {
		if len(body) == 0 {
			continue
		}
		out = append(out, body)
	}
	return out, nil
}

func normalizeMonitorPath(path string) string {
	if commonstrings.IsEmpty(path) {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func getMonitoring(ctx context.Context, client *http.Client, base, path string, maxBody int64) ([]byte, error) {
	if maxBody <= 0 {
		maxBody = defaultMonitoringBodyLimit
	}
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, base+path, http.NoBody)
	if reqErr != nil {
		return nil, reqErr
	}
	resp, doErr := client.Do(req)
	if doErr != nil {
		return nil, doErr
	}
	defer func() { _ = resp.Body.Close() }()

	limited := io.LimitReader(resp.Body, maxBody+1)
	raw, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, readErr
	}
	if int64(len(raw)) > maxBody {
		return nil, fmt.Errorf("monitoring %s: body exceeds %d bytes", path, maxBody)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("monitoring %s: status %d", path, resp.StatusCode)
	}
	return raw, nil
}
