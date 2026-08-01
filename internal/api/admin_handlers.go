package api

import (
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type AdminHandler struct {
	admin *app.AdminService
}

func NewAdminHandler(admin *app.AdminService) *AdminHandler {
	return &AdminHandler{admin: admin}
}

func (h *AdminHandler) RotateEncryptionKey(ctx *fasthttp.RequestCtx) {
	c := httpctx.FromRequest(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok || !user.IsRoot {
		httpstatus.WriteForbidden(ctx)
		return
	}

	var req domain.RotateEncryptionKeyRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.CurrentKey) || commonstrings.IsEmpty(req.NewKey) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrInvalidInput)
		return
	}
	if len(req.NewKey) < 16 {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, crypto.ErrInvalidKey)
		return
	}

	dryRun := strings.EqualFold(commonstrings.BytesToString(ctx.QueryArgs().Peek("dryRun")), "true")
	stats, err := h.admin.RotateEncryptionKeys(c, req.CurrentKey, req.NewKey, dryRun)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	msg := "Restart the server with ENCRYPTION_KEY set to the new key."
	if dryRun {
		msg = "Dry run only — no data was modified."
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.RotateEncryptionKeyResult{
		ClustersUpdated: stats.ClustersUpdated,
		DryRun:          dryRun,
		Message:         msg,
	})
}
