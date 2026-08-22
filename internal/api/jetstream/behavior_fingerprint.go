package jetstream

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/port"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// GetBehaviorFingerprint godoc
//
// @Summary Get Behavior Fingerprint
// @Tags JetStream
// @Param clusterId path string true "clusterId"
// @Param name path string true "name"
// @Param consumer path string true "consumer"
// @Produce json
// @Success 200 {object} api.BehaviorFingerprintEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/streams/{name}/consumers/{consumer}/behavior-fingerprint [get]
func (h *Handler) GetBehaviorFingerprint(ctx *fasthttp.RequestCtx) {
	stream := httpctx.RouteParam(ctx, "name")
	consumer := httpctx.RouteParam(ctx, "consumer")
	if err := apikit.ValidateResourceName(stream); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := apikit.ValidateResourceName(consumer); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	bucket := strings.TrimSpace(h.Cfg.BehaviorFingerprintKVBucket)
	if commonstrings.IsEmpty(bucket) {
		bucket = domain.DefaultBehaviorFingerprintKVBucket
	}
	key := domain.BehaviorFingerprintKVKey(stream, consumer)

	h.Action(ctx, func(c context.Context, client port.JetStreamExecutor) (any, int, error) {
		entry, err := client.GetKVEntry(c, bucket, key)
		if err != nil {
			if apikit.IsNATSNotFound(err) || errors.Is(err, domain.ErrNotFound) {
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
