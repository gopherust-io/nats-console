package api

import (
	"errors"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type NATSAccountHandler struct {
	accounts *app.NATSAccountService
	auth     *auth.Service
	cfg      config.Config
}

func NewNATSAccountHandler(accounts *app.NATSAccountService, authSvc *auth.Service, cfg config.Config) *NATSAccountHandler {
	return &NATSAccountHandler{accounts: accounts, auth: authSvc, cfg: cfg}
}

func (h *NATSAccountHandler) accountFromCtx(ctx *fasthttp.RequestCtx) string {
	account := commonstrings.BytesToString(ctx.QueryArgs().Peek("account"))
	if commonstrings.IsEmpty(account) {
		account = httpctx.RouteParam(ctx, "account")
	}
	if commonstrings.IsEmpty(account) {
		account = "Default"
	}
	return account
}

// requireAccountAccess enforces per-account narrowing for account-scoped routes.
func requireAccountAccess(ctx *fasthttp.RequestCtx, account string) bool {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	if !auth.CanAccessAccount(user, clusterID(ctx), account) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}

// natsUserIDFromCtx returns the route userId after UUID validation.
// Non-UUID values (e.g. static path segments mis-routed as {userId}) get a 400.
func natsUserIDFromCtx(ctx *fasthttp.RequestCtx) (string, bool) {
	userID := httpctx.RouteParam(ctx, "userId")
	if err := validateUUID(userID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return "", false
	}
	return userID, true
}

func (h *NATSAccountHandler) ListUsers(ctx *fasthttp.RequestCtx) {
	clusterID := clusterID(ctx)
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	users, err := h.accounts.ListUsers(httpctx.FromRequest(ctx), clusterID, account)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, nonNilSlice(users), totalMeta(len(users)))
}

func (h *NATSAccountHandler) SubjectPermissions(ctx *fasthttp.RequestCtx) {
	subject := strings.TrimSpace(commonstrings.BytesToString(ctx.QueryArgs().Peek("subject")))
	if commonstrings.IsEmpty(subject) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("subject"))
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	result, err := h.accounts.SubjectPermissions(httpctx.FromRequest(ctx), clusterID(ctx), account, subject)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, result)
}

func (h *NATSAccountHandler) GetUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	user, err := h.accounts.GetUser(httpctx.FromRequest(ctx), clusterID(ctx), account, userID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, user)
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
	TimeRanges             []domain.NATSUserTimeRange `json:"timeRanges"`
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
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	if commonstrings.IsEmpty(req.AccountName) {
		req.AccountName = "Default"
	}
	if !requireAccountAccess(ctx, req.AccountName) {
		return
	}
	user, err := h.accounts.CreateUser(httpctx.FromRequest(ctx), domain.NATSAccountUserCreate{
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
	}, h.cfg.NATS.AccountSeed)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, redactNATSCredsUnlessAllowed(ctx, req.AccountName, user))
}

func requireDownloadCreds(ctx *fasthttp.RequestCtx, account, natsUserID string) bool {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	if !auth.CanDownloadCreds(user, clusterID(ctx), account, natsUserID) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}

func redactNATSCredsUnlessAllowed(ctx *fasthttp.RequestCtx, account string, creds domain.NATSAccountUserCreds) domain.NATSAccountUserCreds {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok || !auth.CanDownloadCreds(user, clusterID(ctx), account, creds.ID) {
		creds.Seed = ""
		creds.Cred = ""
		creds.JWT = ""
	}
	return creds
}

func (h *NATSAccountHandler) UpdateUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	var req natsUserConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	user, err := h.accounts.UpdateUser(httpctx.FromRequest(ctx), clusterID(ctx), account, userID, domain.NATSAccountUserUpdate{
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
	}, h.cfg.NATS.AccountSeed)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, user)
}

func (h *NATSAccountHandler) DeleteUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	err := h.accounts.DeleteUser(httpctx.FromRequest(ctx), clusterID(ctx), account, userID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *NATSAccountHandler) DownloadCreds(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	if !requireDownloadCreds(ctx, account, userID) {
		return
	}
	creds, err := h.accounts.GetCreds(httpctx.FromRequest(ctx), clusterID(ctx), account, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, creds)
}

func (h *NATSAccountHandler) RotateUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	if !requireDownloadCreds(ctx, account, userID) {
		return
	}
	creds, err := h.accounts.RotateUser(httpctx.FromRequest(ctx), clusterID(ctx), account, userID, h.cfg.NATS.AccountSeed)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, creds)
}

func (h *NATSAccountHandler) MintJWT(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	if !requireDownloadCreds(ctx, account, userID) {
		return
	}
	creds, err := h.accounts.MintJWT(httpctx.FromRequest(ctx), clusterID(ctx), account, userID, h.cfg.NATS.AccountSeed)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, creds)
}

func (h *NATSAccountHandler) AssignPerson(ctx *fasthttp.RequestCtx) {
	natsUserID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	var req struct {
		UserID string `json:"userId"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	prev, err := h.accounts.GetUser(httpctx.FromRequest(ctx), clusterID(ctx), account, natsUserID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	user, err := h.accounts.AssignPerson(httpctx.FromRequest(ctx), clusterID(ctx), account, natsUserID, req.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	c := httpctx.FromRequest(ctx)
	if h.auth != nil {
		if !commonstrings.IsEmpty(prev.AssignedUserID) && prev.AssignedUserID != req.UserID {
			h.auth.InvalidateUser(c, prev.AssignedUserID)
		}
		if !commonstrings.IsEmpty(req.UserID) {
			h.auth.InvalidateUser(c, req.UserID)
		}
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, user)
}

func (h *NATSAccountHandler) ListSigningGroups(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	groups, err := h.accounts.ListSigningGroups(httpctx.FromRequest(ctx), clusterID(ctx), account)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, nonNilSlice(groups), totalMeta(len(groups)))
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
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.AccountName) {
		req.AccountName = h.accountFromCtx(ctx)
	}
	if !requireAccountAccess(ctx, req.AccountName) {
		return
	}
	group, err := h.accounts.CreateSigningGroup(httpctx.FromRequest(ctx), domain.SigningGroupCreate{
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
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, group)
}

func (h *NATSAccountHandler) UpdateSigningGroup(ctx *fasthttp.RequestCtx) {
	var req createSigningGroupRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	group, err := h.accounts.UpdateSigningGroup(httpctx.FromRequest(ctx), clusterID(ctx), account, httpctx.RouteParam(ctx, "groupId"), domain.SigningGroupUpdate{
		Scoped:     req.Scoped,
		PubAllow:   req.PubAllow,
		PubDeny:    req.PubDeny,
		SubAllow:   req.SubAllow,
		SubDeny:    req.SubDeny,
		MaxData:    req.MaxData,
		MaxPayload: req.MaxPayload,
		MaxSubs:    req.MaxSubs,
	})
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, group)
}

func (h *NATSAccountHandler) DeleteSigningGroup(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	err := h.accounts.DeleteSigningGroup(httpctx.FromRequest(ctx), clusterID(ctx), account, httpctx.RouteParam(ctx, "groupId"))
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *NATSAccountHandler) ListExports(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	kind := commonstrings.BytesToString(ctx.QueryArgs().Peek("kind"))
	items, err := h.accounts.ListExports(httpctx.FromRequest(ctx), clusterID(ctx), account, kind)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, nonNilSlice(items), totalMeta(len(items)))
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
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) || commonstrings.IsEmpty(req.Kind) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name/kind"))
		return
	}
	if commonstrings.IsEmpty(req.AccountName) {
		req.AccountName = "Default"
	}
	if !requireAccountAccess(ctx, req.AccountName) {
		return
	}
	item, err := h.accounts.CreateExport(httpctx.FromRequest(ctx), domain.NATSAccountExportCreate{
		ClusterID:   clusterID(ctx),
		AccountName: req.AccountName,
		Kind:        req.Kind,
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, item)
}

func (h *NATSAccountHandler) UpdateExport(ctx *fasthttp.RequestCtx) {
	var req createExportRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	item, err := h.accounts.UpdateExport(httpctx.FromRequest(ctx), clusterID(ctx), account, httpctx.RouteParam(ctx, "exportId"), domain.NATSAccountExportUpdate{
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, item)
}

func (h *NATSAccountHandler) DeleteExport(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	err := h.accounts.DeleteExport(httpctx.FromRequest(ctx), clusterID(ctx), account, httpctx.RouteParam(ctx, "exportId"))
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
