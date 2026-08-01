package api

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// GetBehaviorFingerprint returns the latest worker-reported fingerprint from KV.
// Missing bucket/key yields available=false (200), not an error — UI shows idle state.
func (h *Handler) GetBehaviorFingerprint(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	if err := validateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateResourceName(consumer); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	bucket := strings.TrimSpace(h.cfg.BehaviorFingerprintKVBucket)
	if commonstrings.IsEmpty(bucket) {
		bucket = domain.DefaultBehaviorFingerprintKVBucket
	}
	key := domain.BehaviorFingerprintKVKey(stream, consumer)

	h.natsAction(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entry, err := client.GetKVEntry(c, bucket, key)
		if err != nil {
			if isNATSNotFound(err) || errors.Is(err, domain.ErrNotFound) {
				return domain.BehaviorFingerprintReport{Available: false}, fasthttp.StatusOK, nil
			}
			return nil, fasthttp.StatusBadGateway, err
		}
		raw, decodeErr := decodeKVEntryValue(entry)
		if decodeErr != nil {
			return domain.BehaviorFingerprintReport{Available: false}, fasthttp.StatusOK, nil
		}
		return domain.ParseBehaviorFingerprintKV(raw, stream, consumer), fasthttp.StatusOK, nil
	})
}

func decodeKVEntryValue(entry *domain.KVEntry) ([]byte, error) {
	if entry == nil || commonstrings.IsEmpty(entry.Value) {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(entry.Value)
}
