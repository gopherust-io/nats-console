package api

import (
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

type AdminHandler struct {
	store *store.Store
}

func NewAdminHandler(st *store.Store) *AdminHandler {
	return &AdminHandler{store: st}
}

func (h *AdminHandler) RotateEncryptionKey(ctx *fasthttp.RequestCtx) {
	c := httpctx.FromRequest(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok || !user.IsRoot {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		ctx.SetBodyString("forbidden")
		return
	}

	var req domain.RotateEncryptionKeyRequest
	if err := serializer.UnmarshalRequest(ctx.PostBody(), &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.CurrentKey == "" || req.NewKey == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrInvalidInput)
		return
	}
	if len(req.NewKey) < 16 {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, crypto.ErrInvalidKey)
		return
	}

	dryRun := strings.EqualFold(string(ctx.QueryArgs().Peek("dryRun")), "true")
	stats, err := h.store.RotateEncryptionKeys(c, req.CurrentKey, req.NewKey, dryRun)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	msg := "Restart the server with ENCRYPTION_KEY set to the new key."
	if dryRun {
		msg = "Dry run only — no data was modified."
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, domain.RotateEncryptionKeyResult{
		ClustersUpdated: stats.ClustersUpdated,
		DryRun:          dryRun,
		Message:         msg,
	})
}
