package api

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
)

const (
	// Per-request timeout; broker scrape budget (DefaultReplicasScrapeTimeout) covers failover hops.
	replicasMonitorTimeout = 3 * time.Second
	// defaultMonitoringBodyLimit matches config MaxMonitoringBodyBytes default (8 MiB).
	defaultMonitoringBodyLimit int64 = 8 << 20
)

var monitoringHTTPClient = &http.Client{
	Timeout: replicasMonitorTimeout,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: time.Second,
	},
}

// monitoringCandidates builds failover HTTP bases from the primary monitoring URL
// and each NATS client URL (client port + 4000 → monitor port, e.g. 4222→8222).
func monitoringCandidates(natsURL, monitoringURL string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	add := func(raw string) {
		raw = strings.TrimRight(strings.TrimSpace(raw), "/")
		if raw == "" {
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
		if part == "" {
			continue
		}
		u, err := url.Parse(part)
		if err != nil || u.Hostname() == "" {
			continue
		}
		port := u.Port()
		if port == "" {
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

func fetchMonitoringWithFailover(ctx context.Context, bases []string, path string) (body []byte, used string, err error) {
	if len(bases) == 0 {
		return nil, "", errors.New("no monitoring URLs")
	}
	path = normalizeMonitorPath(path)

	var lastErr error
	for _, base := range bases {
		base = strings.TrimRight(base, "/")
		raw, doErr := getMonitoring(ctx, monitoringHTTPClient, base, path, defaultMonitoringBodyLimit)
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

// fetchMonitoringAll scrapes path from every base in parallel. Results keep
// candidate order; unreachable bases are omitted. Returns an error only when
// every base fails.
func fetchMonitoringAll(ctx context.Context, bases []string, path string) ([][]byte, error) {
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
		wg.Add(1)
		go func(i int, base string) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					safe.Log("monitoring", rec)
					ch <- result{idx: i, err: fmt.Errorf("panic: %v", rec)}
				}
			}()
			base = strings.TrimRight(base, "/")
			body, err := getMonitoring(ctx, monitoringHTTPClient, base, path, defaultMonitoringBodyLimit)
			ch <- result{idx: i, body: body, err: err}
		}(i, base)
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
	if path == "" {
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
