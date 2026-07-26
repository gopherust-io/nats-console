package api

import (
	"errors"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/valyala/fasthttp"
)

type NATSAccountHandler struct {
	store *store.Store
	auth  *auth.Service
	cfg   config.Config
}

func NewNATSAccountHandler(st *store.Store, authSvc *auth.Service, cfg config.Config) *NATSAccountHandler {
	return &NATSAccountHandler{store: st, auth: authSvc, cfg: cfg}
}

func (h *NATSAccountHandler) accountFromCtx(ctx *fasthttp.RequestCtx) string {
	account := string(ctx.QueryArgs().Peek("account"))
	if account == "" {
		account = routeParam(ctx, "account")
	}
	if account == "" {
		account = "Default"
	}
	return account
}

func (h *NATSAccountHandler) ListUsers(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	account := h.accountFromCtx(ctx)
	users, err := h.store.ListNATSAccountUsers(requestContext(ctx), clusterID, account)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"users": nonNilSlice(users),
		"total": len(users),
	})
}

func (h *NATSAccountHandler) GetUser(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	user, err := h.store.GetNATSAccountUser(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"))
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, user)
}

// goalign:ignore
type natsUserConfigRequest struct {
	Name                   string                    `json:"name"`
	AccountName            string                    `json:"accountName"`
	SigningGroup           string                    `json:"signingGroup"`
	Tags                   []string                  `json:"tags"`
	PubAllow               []string                  `json:"pubAllow"`
	PubDeny                []string                  `json:"pubDeny"`
	SubAllow               []string                  `json:"subAllow"`
	SubDeny                []string                  `json:"subDeny"`
	AllowedConnectionTypes []string                  `json:"allowedConnectionTypes"`
	SrcCIDRs               []string                  `json:"srcCidrs"`
	TimesLocale            string                    `json:"timesLocale"`
	TimeRanges             []store.NATSUserTimeRange `json:"timeRanges"`
	MaxSubs                int64                     `json:"maxSubs"`
	MaxPayload             int64                     `json:"maxPayload"`
	MaxData                int64                     `json:"maxData"`
	JWTLifetimeNs          int64                     `json:"jwtLifetimeNs"`
	RespMaxMsgs            int                       `json:"respMaxMsgs"`
	RespTTLNs              int64                     `json:"respTTLNs"`
	BearerToken            bool                      `json:"bearerToken"`
	ProxyRequired          bool                      `json:"proxyRequired"`
}

func (h *NATSAccountHandler) CreateUser(ctx *fasthttp.RequestCtx) {
	var req natsUserConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	if req.AccountName == "" {
		req.AccountName = "Default"
	}
	user, err := h.store.CreateNATSAccountUserWithSeed(requestContext(ctx), store.NATSAccountUserCreate{
		ClusterID:              clusterID(ctx),
		AccountName:            req.AccountName,
		Name:                   req.Name,
		SigningGroup:           req.SigningGroup,
		Tags:                   req.Tags,
		PubAllow:               req.PubAllow,
		PubDeny:                req.PubDeny,
		SubAllow:               req.SubAllow,
		SubDeny:                req.SubDeny,
		AllowedConnectionTypes: req.AllowedConnectionTypes,
		SrcCIDRs:               req.SrcCIDRs,
		TimesLocale:            req.TimesLocale,
		TimeRanges:             req.TimeRanges,
		MaxSubs:                req.MaxSubs,
		MaxPayload:             req.MaxPayload,
		MaxData:                req.MaxData,
		JWTLifetimeNs:          req.JWTLifetimeNs,
		RespMaxMsgs:            req.RespMaxMsgs,
		RespTTLNs:              req.RespTTLNs,
		BearerToken:            req.BearerToken,
		ProxyRequired:          req.ProxyRequired,
	}, h.cfg.NATSAccountSeed)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, user)
}

func (h *NATSAccountHandler) UpdateUser(ctx *fasthttp.RequestCtx) {
	var req natsUserConfigRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	account := h.accountFromCtx(ctx)
	user, err := h.store.UpdateNATSAccountUser(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"), store.NATSAccountUserUpdate{
		SigningGroup:           req.SigningGroup,
		Tags:                   req.Tags,
		PubAllow:               req.PubAllow,
		PubDeny:                req.PubDeny,
		SubAllow:               req.SubAllow,
		SubDeny:                req.SubDeny,
		AllowedConnectionTypes: req.AllowedConnectionTypes,
		SrcCIDRs:               req.SrcCIDRs,
		TimesLocale:            req.TimesLocale,
		TimeRanges:             req.TimeRanges,
		MaxSubs:                req.MaxSubs,
		MaxPayload:             req.MaxPayload,
		MaxData:                req.MaxData,
		JWTLifetimeNs:          req.JWTLifetimeNs,
		RespMaxMsgs:            req.RespMaxMsgs,
		RespTTLNs:              req.RespTTLNs,
		BearerToken:            req.BearerToken,
		ProxyRequired:          req.ProxyRequired,
	}, h.cfg.NATSAccountSeed)
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, user)
}

func (h *NATSAccountHandler) DeleteUser(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	err := h.store.DeleteNATSAccountUser(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"))
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *NATSAccountHandler) DownloadCreds(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if user, ok := storeActor(ctx); ok {
		if !auth.CanDownloadCreds(user, clusterID(ctx), account) {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString("forbidden")
			return
		}
	}
	creds, err := h.store.GetNATSAccountUserCreds(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, creds)
}

func (h *NATSAccountHandler) RotateUser(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	creds, err := h.store.RotateNATSAccountUser(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"), h.cfg.NATSAccountSeed)
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, creds)
}

func (h *NATSAccountHandler) MintJWT(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	creds, err := h.store.MintNATSAccountUserJWT(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"), h.cfg.NATSAccountSeed)
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, creds)
}

func (h *NATSAccountHandler) AssignPerson(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	var req struct {
		UserID string `json:"userId"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	user, err := h.store.AssignNATSAccountUserPerson(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "userId"), req.UserID)
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.UserID != "" && h.auth != nil {
		h.auth.InvalidateUser(req.UserID)
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, user)
}

func (h *NATSAccountHandler) ListSigningGroups(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	groups, err := h.store.ListSigningGroups(requestContext(ctx), clusterID(ctx), account)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"groups": nonNilSlice(groups),
		"total":  len(groups),
	})
}

// goalign:ignore
type createSigningGroupRequest struct {
	Name        string   `json:"name"`
	AccountName string   `json:"accountName"`
	PubAllow    []string `json:"pubAllow"`
	PubDeny     []string `json:"pubDeny"`
	SubAllow    []string `json:"subAllow"`
	SubDeny     []string `json:"subDeny"`
	MaxData     int64    `json:"maxData"`
	MaxPayload  int64    `json:"maxPayload"`
	MaxSubs     int64    `json:"maxSubs"`
	Scoped      bool     `json:"scoped"`
}

func (h *NATSAccountHandler) CreateSigningGroup(ctx *fasthttp.RequestCtx) {
	var req createSigningGroupRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.AccountName == "" {
		req.AccountName = h.accountFromCtx(ctx)
	}
	group, err := h.store.CreateSigningGroup(requestContext(ctx), store.SigningGroupCreate{
		ClusterID:   clusterID(ctx),
		AccountName: req.AccountName,
		Name:        req.Name,
		Scoped:      req.Scoped,
		PubAllow:    req.PubAllow,
		PubDeny:     req.PubDeny,
		SubAllow:    req.SubAllow,
		SubDeny:     req.SubDeny,
		MaxData:     req.MaxData,
		MaxPayload:  req.MaxPayload,
		MaxSubs:     req.MaxSubs,
	})
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, group)
}

func (h *NATSAccountHandler) UpdateSigningGroup(ctx *fasthttp.RequestCtx) {
	var req createSigningGroupRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	account := h.accountFromCtx(ctx)
	group, err := h.store.UpdateSigningGroup(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "groupId"), store.SigningGroupUpdate{
		Scoped:     req.Scoped,
		PubAllow:   req.PubAllow,
		PubDeny:    req.PubDeny,
		SubAllow:   req.SubAllow,
		SubDeny:    req.SubDeny,
		MaxData:    req.MaxData,
		MaxPayload: req.MaxPayload,
		MaxSubs:    req.MaxSubs,
	})
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, group)
}

func (h *NATSAccountHandler) DeleteSigningGroup(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	err := h.store.DeleteSigningGroup(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "groupId"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
	case errors.Is(err, store.ErrSigningGroupProtected), errors.Is(err, store.ErrSigningGroupInUse):
		serializer.WriteError(ctx, fasthttp.StatusConflict, err)
	case err != nil:
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
	default:
		ctx.SetStatusCode(fasthttp.StatusNoContent)
	}
}

func (h *NATSAccountHandler) ListExports(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	kind := string(ctx.QueryArgs().Peek("kind"))
	items, err := h.store.ListNATSAccountExports(requestContext(ctx), clusterID(ctx), account, kind)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"exports": nonNilSlice(items),
		"total":   len(items),
	})
}

type createExportRequest struct {
	AccountName string `json:"accountName"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

func (h *NATSAccountHandler) CreateExport(ctx *fasthttp.RequestCtx) {
	var req createExportRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" || req.Kind == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name/kind"))
		return
	}
	if req.AccountName == "" {
		req.AccountName = "Default"
	}
	item, err := h.store.CreateNATSAccountExport(requestContext(ctx), store.NATSAccountExportCreate{
		ClusterID:   clusterID(ctx),
		AccountName: req.AccountName,
		Kind:        req.Kind,
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, item)
}

func (h *NATSAccountHandler) UpdateExport(ctx *fasthttp.RequestCtx) {
	var req createExportRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	account := h.accountFromCtx(ctx)
	item, err := h.store.UpdateNATSAccountExport(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "exportId"), store.NATSAccountExportUpdate{
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, item)
}

func (h *NATSAccountHandler) DeleteExport(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	err := h.store.DeleteNATSAccountExport(requestContext(ctx), clusterID(ctx), account, routeParam(ctx, "exportId"))
	if errors.Is(err, store.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
