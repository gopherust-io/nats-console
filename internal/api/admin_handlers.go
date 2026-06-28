package api

import (
	"errors"
	"strings"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/crypto"
	"github.com/gopherust-io/nats-consol/internal/domain"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/valyala/fasthttp"
)

type AdminHandler struct {
	store *store.Store
}

func NewAdminHandler(st *store.Store) *AdminHandler {
	return &AdminHandler{store: st}
}

func (h *AdminHandler) RotateEncryptionKey(ctx *fasthttp.RequestCtx) {
	c := requestContext(ctx)
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
		JWTUpdated:      stats.JWTUpdated,
		DryRun:          dryRun,
		Message:         msg,
	})
}

type ResolverHandler struct {
	store *store.Store
}

func NewResolverHandler(st *store.Store) *ResolverHandler {
	return &ResolverHandler{store: st}
}

func (h *ResolverHandler) ListAccounts(ctx *fasthttp.RequestCtx) {
	clusterID := routeParam(ctx, "clusterId")
	accounts, err := h.store.ListJWTAccounts(requestContext(ctx), clusterID)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}
	out := make([]domain.JWTAccount, 0, len(accounts))
	for _, item := range accounts {
		out = append(out, jwtAccountFromStore(item))
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"accounts": out,
		"total":    len(out),
	})
}

func (h *ResolverHandler) ImportAccount(ctx *fasthttp.RequestCtx) {
	clusterID := routeParam(ctx, "clusterId")
	var req domain.JWTAccountImport
	if err := serializer.UnmarshalRequest(ctx.PostBody(), &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	parsed, err := natsclient.ParseAccountJWT(req.JWT)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = parsed.Name
	}
	if name == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, domain.ErrInvalidInput)
		return
	}

	created, err := h.store.CreateJWTAccount(requestContext(ctx), store.JWTAccountCreate{
		ClusterID: clusterID,
		Name:      name,
		JWT:       strings.TrimSpace(req.JWT),
		ExpiresAt: parsed.ExpiresAt,
	})
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, jwtAccountFromStore(created))
}

func (h *ResolverHandler) DeleteAccount(ctx *fasthttp.RequestCtx) {
	clusterID := routeParam(ctx, "clusterId")
	name := routeParam(ctx, "name")
	if err := h.store.DeleteJWTAccount(requestContext(ctx), clusterID, name); err != nil {
		writeStoreError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *ResolverHandler) ExportAccounts(ctx *fasthttp.RequestCtx) {
	clusterID := routeParam(ctx, "clusterId")
	accounts, err := h.store.ExportJWTAccounts(requestContext(ctx), clusterID)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
		return
	}

	out := domain.JWTResolverExport{Accounts: make([]domain.JWTExportEntry, 0, len(accounts))}
	for _, item := range accounts {
		jwtValue, err := h.store.DecryptCredential(item.JWT)
		if err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
			return
		}
		out.Accounts = append(out.Accounts, domain.JWTExportEntry{Name: item.Name, JWT: jwtValue})
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, out)
}

func jwtAccountFromStore(item store.JWTAccount) domain.JWTAccount {
	return domain.JWTAccount{
		ID:        item.ID,
		ClusterID: item.ClusterID,
		Name:      item.Name,
		HasJWT:    item.JWT != "",
		ExpiresAt: item.ExpiresAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func writeStoreError(ctx *fasthttp.RequestCtx, err error) {
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, domain.ErrNotFound)
		return
	}
	serializer.WriteError(ctx, fasthttp.StatusBadGateway, err)
}
