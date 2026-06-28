package assistant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"

	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
)

const (
	defaultContextCacheTTL = 45 * time.Second
	maxContextCacheEntries = 100
)

var (
	ErrNotEnabled         = errors.New("assistant is not enabled")
	ErrContextUnavailable = errors.New("cluster context unavailable")
)

type PageContext struct {
	Route    string `json:"route,omitempty"`
	Stream   string `json:"stream,omitempty"`
	Consumer string `json:"consumer,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
	Key      string `json:"key,omitempty"`
}

type ContextBuilder struct {
	store    *store.Store
	nats     *natsclient.Manager
	cache    map[string]contextCacheEntry
	cacheTTL time.Duration
	mu       sync.Mutex
}

type contextCacheEntry struct {
	expiresAt time.Time
	formatted string
}

func NewContextBuilder(st *store.Store, nats *natsclient.Manager, cacheTTL time.Duration) *ContextBuilder {
	if cacheTTL <= 0 {
		cacheTTL = defaultContextCacheTTL
	}
	return &ContextBuilder{
		store:    st,
		nats:     nats,
		cacheTTL: cacheTTL,
		cache:    make(map[string]contextCacheEntry),
	}
}

func (b *ContextBuilder) BuildContextBlock(ctx context.Context, clusterID string, page PageContext) (string, error) {
	key := contextCacheKey(clusterID, page)
	now := time.Now()

	b.mu.Lock()
	if entry, ok := b.cache[key]; ok && now.Before(entry.expiresAt) {
		formatted := entry.formatted
		b.mu.Unlock()
		return formatted, nil
	}
	b.purgeExpiredLocked(now)
	b.mu.Unlock()

	raw, err := b.buildFresh(ctx, clusterID, page)
	if err != nil {
		return "", err
	}
	formatted, err := FormatContextBlock(raw)
	if err != nil {
		return "", err
	}

	b.mu.Lock()
	b.purgeExpiredLocked(now)
	if len(b.cache) >= maxContextCacheEntries {
		b.evictOldestLocked()
	}
	b.cache[key] = contextCacheEntry{
		formatted: formatted,
		expiresAt: now.Add(b.cacheTTL),
	}
	b.mu.Unlock()

	return formatted, nil
}

func contextCacheKey(clusterID string, page PageContext) string {
	return clusterID + "|" + page.Route + "|" + page.Stream + "|" + page.Consumer + "|" + page.Bucket + "|" + page.Key
}

func (b *ContextBuilder) purgeExpiredLocked(now time.Time) {
	for key, entry := range b.cache {
		if !now.Before(entry.expiresAt) {
			delete(b.cache, key)
		}
	}
}

func (b *ContextBuilder) evictOldestLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	first := true
	for key, entry := range b.cache {
		if first || entry.expiresAt.Before(oldestExpiry) {
			oldestKey = key
			oldestExpiry = entry.expiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(b.cache, oldestKey)
	}
}

func (b *ContextBuilder) buildFresh(ctx context.Context, clusterID string, page PageContext) (map[string]any, error) {
	cluster, err := b.store.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	client, err := b.nats.Get(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"cluster": map[string]any{
			"id":             cluster.ID,
			"name":           cluster.Name,
			"nats_url":       redactURL(cluster.NATSURL),
			"monitoring_url": cluster.MonitoringURL,
			"is_default":     cluster.IsDefault,
			"has_creds":      cluster.HasCreds,
			"has_token":      cluster.HasToken,
		},
		"page": page,
	}

	if account, err := client.AccountInfo(ctx); err == nil {
		out["account"] = map[string]any{
			"streams":   account.Streams,
			"consumers": account.Consumers,
			"memory":    account.Memory,
			"storage":   account.Store,
			"limits":    account.Limits,
		}
	}

	if jsz, err := client.Monitoring(ctx, "/jsz?streams=1&consumers=1&accounts=1"); err == nil {
		out["jetstream"] = compactJSON(jsz, 12_000)
	}

	if varz, err := client.Monitoring(ctx, "/varz"); err == nil {
		out["server"] = compactJSON(varz, 4_000)
	}

	streams, total, err := client.ListStreams(ctx, 0, 30)
	if err == nil {
		summaries := make([]map[string]any, 0, len(streams))
		for _, info := range streams {
			summaries = append(summaries, streamSummary(info))
		}
		out["streams"] = summaries
		if total > len(streams) {
			out["streams_truncated"] = total - len(streams)
		}
	}

	if page.Stream != "" {
		if info, err := client.StreamInfo(ctx, page.Stream); err == nil {
			out["current_stream"] = streamSummary(info)
			consumers, total, err := client.ListConsumers(ctx, page.Stream, 0, 20)
			if err == nil {
				summaries := make([]map[string]any, 0, len(consumers))
				for _, cInfo := range consumers {
					summaries = append(summaries, consumerSummary(cInfo))
				}
				out["current_consumers"] = summaries
				if total > len(consumers) {
					out["consumers_truncated"] = total - len(consumers)
				}
			}
		}
	}

	if page.Stream != "" && page.Consumer != "" {
		if cInfo, err := client.ConsumerInfo(ctx, page.Stream, page.Consumer); err == nil {
			out["current_consumer"] = consumerSummary(cInfo)
		}
	}

	return SanitizeContext(out), nil
}

func streamSummary(info *nats.StreamInfo) map[string]any {
	return map[string]any{
		"name": info.Config.Name,
		"config": map[string]any{
			"subjects":  info.Config.Subjects,
			"retention": fmt.Sprintf("%v", info.Config.Retention),
			"storage":   fmt.Sprintf("%v", info.Config.Storage),
			"max_msgs":  info.Config.MaxMsgs,
			"max_bytes": info.Config.MaxBytes,
			"max_age":   info.Config.MaxAge.String(),
		},
		"state": map[string]any{
			"messages":       info.State.Msgs,
			"bytes":          info.State.Bytes,
			"first_seq":      info.State.FirstSeq,
			"last_seq":       info.State.LastSeq,
			"consumer_count": info.State.Consumers,
		},
	}
}

func consumerSummary(info *nats.ConsumerInfo) map[string]any {
	return map[string]any{
		"name":            info.Name,
		"stream":          info.Stream,
		"num_pending":     info.NumPending,
		"num_ack_pending": info.NumAckPending,
		"delivered":       info.Delivered,
		"config": map[string]any{
			"durable_name":   info.Config.Durable,
			"deliver_policy": fmt.Sprintf("%v", info.Config.DeliverPolicy),
			"ack_policy":     fmt.Sprintf("%v", info.Config.AckPolicy),
			"filter_subject": info.Config.FilterSubject,
		},
	}
}

func compactJSON(raw []byte, maxLen int) any {
	if len(raw) > maxLen {
		raw = raw[:maxLen]
	}
	var v any
	if err := sonic.Unmarshal(raw, &v); err != nil {
		s := redactString(string(raw))
		if len(s) > maxLen {
			return s[:maxLen] + "…"
		}
		return s
	}
	safe := redactValue(v)
	encoded, err := sonic.Marshal(safe)
	if err != nil || len(encoded) <= maxLen {
		return safe
	}
	return map[string]any{"truncated": true, "preview": string(encoded[:maxLen]) + "…"}
}

func FormatContextBlock(ctx map[string]any) (string, error) {
	data, err := sonic.MarshalString(SanitizeContext(ctx))
	if err != nil {
		return "", err
	}
	return "Live NATS JetStream cluster context (JSON). Sensitive fields are redacted:\n```json\n" + data + "\n```", nil
}
