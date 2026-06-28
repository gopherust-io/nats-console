package api

import (
	"strconv"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/store"
)

type UsersHandler struct {
	store *store.Store
}

func NewUsersHandler(st *store.Store) *UsersHandler {
	return &UsersHandler{store: st}
}

func (h *UsersHandler) List(ctx *fasthttp.RequestCtx) {
	users, err := h.store.ListUsers(requestContext(ctx))
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"users": users,
		"total": len(users),
	})
}

func (h *UsersHandler) SetRoles(ctx *fasthttp.RequestCtx) {
	userID := param(ctx, "userId")
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if len(req.Roles) == 0 {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
		return
	}
	if err := h.store.SetUserRoles(requestContext(ctx), userID, req.Roles); err != nil {
		writeStoreError(ctx, err)
		return
	}
	user, err := h.store.GetUserByID(requestContext(ctx), userID)
	if err != nil {
		writeStoreError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, user)
}

type AuditHandler struct {
	store *store.Store
}

func NewAuditHandler(st *store.Store) *AuditHandler {
	return &AuditHandler{store: st}
}

func (h *AuditHandler) List(ctx *fasthttp.RequestCtx) {
	limit, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("limit")))
	offset, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("offset")))
	clusterID := string(ctx.QueryArgs().Peek("cluster_id"))

	entries, total, err := h.store.ListAudit(requestContext(ctx), store.AuditFilter{
		ClusterID: clusterID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
	})
}
