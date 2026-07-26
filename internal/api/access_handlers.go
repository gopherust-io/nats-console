package api

import (
	"errors"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

type AccessHandler struct {
	store *store.Store
	auth  *auth.Service
	cfg   config.Config
}

func NewAccessHandler(st *store.Store, authSvc *auth.Service, cfg config.Config) *AccessHandler {
	return &AccessHandler{store: st, auth: authSvc, cfg: cfg}
}

func (h *AccessHandler) ListSystemAccess(ctx *fasthttp.RequestCtx) {
	h.listResourceAccess(ctx, store.ResourceSystem, clusterID(ctx))
}

func (h *AccessHandler) UpsertSystemAccess(ctx *fasthttp.RequestCtx) {
	h.upsertResourceAccess(ctx, store.ResourceSystem, clusterID(ctx), true)
}

func (h *AccessHandler) DeleteSystemAccess(ctx *fasthttp.RequestCtx) {
	h.deleteResourceAccess(ctx, store.ResourceSystem, clusterID(ctx))
}

func (h *AccessHandler) ListAccountAccess(ctx *fasthttp.RequestCtx) {
	account := routeParam(ctx, "account")
	if account == "" {
		account = "Default"
	}
	h.listResourceAccess(ctx, store.ResourceAccount, domain.AccountResourceKey(clusterID(ctx), account))
}

func (h *AccessHandler) UpsertAccountAccess(ctx *fasthttp.RequestCtx) {
	account := routeParam(ctx, "account")
	if account == "" {
		account = "Default"
	}
	h.upsertResourceAccess(ctx, store.ResourceAccount, domain.AccountResourceKey(clusterID(ctx), account), false)
}

func (h *AccessHandler) DeleteAccountAccess(ctx *fasthttp.RequestCtx) {
	account := routeParam(ctx, "account")
	if account == "" {
		account = "Default"
	}
	h.deleteResourceAccess(ctx, store.ResourceAccount, domain.AccountResourceKey(clusterID(ctx), account))
}

func (h *AccessHandler) InvitePerson(ctx *fasthttp.RequestCtx) {
	actor, ok := storeActor(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if !auth.CanManageUsers(actor) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		ctx.SetBodyString("forbidden")
		return
	}
	var req struct {
		Username string   `json:"username"`
		Email    string   `json:"email"`
		Roles    []string `json:"roles"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("username"))
		return
	}
	if len(req.Roles) == 0 {
		req.Roles = []string{store.RoleViewer}
	}
	if req.Email == "" {
		req.Email = req.Username + "@local"
	}
	user, err := h.store.CreateUser(requestContext(ctx), store.UserCreate{
		Username: req.Username,
		Email:    req.Email,
		Roles:    req.Roles,
		AccessRules: &store.AccessRules{
			ClusterIDs:      []string{},
			AssignableRoles: []string{store.RoleViewer},
		},
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			serializer.WriteError(ctx, fasthttp.StatusConflict, err)
			return
		}
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	inv, err := h.store.CreateUserInvite(requestContext(ctx), user.ID, 7*24*time.Hour)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	base := strings.TrimSuffix(h.cfg.PublicBaseURL, "/")
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, map[string]any{
		"user":      toUserResponse(user),
		"inviteUrl": base + "/invite/" + inv.Token,
		"expiresAt": inv.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *AccessHandler) GetInvite(ctx *fasthttp.RequestCtx) {
	token := routeParam(ctx, "token")
	inv, err := h.store.GetUserInvite(requestContext(ctx), token)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if inv.AcceptedAt != nil || time.Now().UTC().After(inv.ExpiresAt) {
		serializer.WriteError(ctx, fasthttp.StatusGone, errors.New("invite expired or already used"))
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"username":  inv.Username,
		"email":     inv.Email,
		"expiresAt": inv.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *AccessHandler) AcceptInvite(ctx *fasthttp.RequestCtx) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Token == "" || req.Password == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("token and password required"))
		return
	}
	user, err := h.store.AcceptUserInvite(requestContext(ctx), req.Token, req.Password)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		if errors.Is(err, store.ErrConflict) {
			serializer.WriteError(ctx, fasthttp.StatusConflict, err)
			return
		}
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.auth.InvalidateUser(user.ID)
	token, err := h.auth.CreateSession(user)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	csrf, err := h.auth.NewCSRFToken()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	setCookie(ctx, h.auth.SessionCookie(token))
	setCookie(ctx, h.auth.CSRFCookie(csrf))
	writeUserJSON(ctx, fasthttp.StatusOK, user)
}

func (h *AccessHandler) listResourceAccess(ctx *fasthttp.RequestCtx, resourceType, resourceKey string) {
	actor, ok := storeActor(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if !h.canManage(actor, resourceType, resourceKey) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		ctx.SetBodyString("forbidden")
		return
	}
	grants, err := h.store.ListAccessGrantsByResource(requestContext(ctx), resourceType, resourceKey)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"grants": nonNilSlice(grants),
		"total":  len(grants),
	})
}

func (h *AccessHandler) upsertResourceAccess(ctx *fasthttp.RequestCtx, resourceType, resourceKey string, systemOnly bool) {
	actor, ok := storeActor(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if !h.canManage(actor, resourceType, resourceKey) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		ctx.SetBodyString("forbidden")
		return
	}
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if systemOnly && req.Role == store.GrantCredentialDownloader {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("credential_downloader is account-scoped"))
		return
	}
	grant, err := h.store.UpsertAccessGrant(requestContext(ctx), store.AccessGrantUpsert{
		UserID:       req.UserID,
		ResourceType: resourceType,
		ResourceKey:  resourceKey,
		Role:         req.Role,
	})
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.auth.InvalidateUser(req.UserID)
	serializer.WriteJSON(ctx, fasthttp.StatusOK, grant)
}

func (h *AccessHandler) deleteResourceAccess(ctx *fasthttp.RequestCtx, resourceType, resourceKey string) {
	actor, ok := storeActor(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	if !h.canManage(actor, resourceType, resourceKey) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		ctx.SetBodyString("forbidden")
		return
	}
	grantID := routeParam(ctx, "grantId")
	userID := string(ctx.QueryArgs().Peek("userId"))
	c := requestContext(ctx)
	if grantID != "" {
		if err := h.store.DeleteAccessGrant(c, grantID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
				return
			}
			serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
			return
		}
		if userID != "" {
			h.auth.InvalidateUser(userID)
		}
		ctx.SetStatusCode(fasthttp.StatusNoContent)
		return
	}
	if userID == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("userId"))
		return
	}
	if err := h.store.DeleteAccessGrantByResource(c, userID, resourceType, resourceKey); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	h.auth.InvalidateUser(userID)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AccessHandler) canManage(actor store.User, resourceType, resourceKey string) bool {
	switch resourceType {
	case store.ResourceSystem:
		return auth.CanManageSystemAccess(actor, resourceKey)
	case store.ResourceAccount:
		clusterID, account, _ := strings.Cut(resourceKey, ":")
		return auth.CanManageAccountAccess(actor, clusterID, account)
	default:
		return auth.CanManageUsers(actor) || actor.IsRoot
	}
}

func storeActor(ctx *fasthttp.RequestCtx) (store.User, bool) {
	c := requestContext(ctx)
	return auth.UserFromContext(c)
}
