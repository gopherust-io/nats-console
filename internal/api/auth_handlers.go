package api

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/ipset"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type AuthHandler struct {
	auth *auth.Service
	cfg  config.Config
}

func NewAuthHandler(authSvc *auth.Service, cfg config.Config) *AuthHandler {
	return &AuthHandler{auth: authSvc, cfg: cfg}
}

func (h *AuthHandler) requestFingerprint(ctx *fasthttp.RequestCtx) string {
	trusted := ipset.ParseTrustedProxies(h.cfg.TrustedProxyList())
	ip := httpctx.ClientIP(ctx, trusted)
	ua := commonstrings.BytesToString(ctx.Request.Header.UserAgent())
	return auth.DeviceFingerprint(ua, ip)
}

func (h *AuthHandler) issueSessionCookies(ctx *fasthttp.RequestCtx, accessToken, refreshToken, csrf string) {
	httpctx.SetCookie(ctx, h.auth.SessionCookie(accessToken))
	httpctx.SetCookie(ctx, h.auth.RefreshTokenCookie(refreshToken))
	httpctx.SetCookie(ctx, h.auth.CSRFCookie(csrf))
}

// Config godoc
//
// @Summary Config
// @Tags Auth
// @Produce json
// @Success 200 {object} AuthConfigEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/auth/config [get]
func (h *AuthHandler) Config(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteData(ctx, fasthttp.StatusOK, AuthConfigResponse{
		BasicEnabled: true,
		AuthEnabled:  true,
		AIEnabled:    h.cfg.AIActive(),
	})
}

// Me godoc
//
// @Summary Me
// @Tags Auth
// @Produce json
// @Success 200 {object} UserEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) Me(ctx *fasthttp.RequestCtx) {
	c := httpctx.FromRequest(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, toUserResponse(user))
}

// Login godoc
//
// @Summary Login
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "credentials"
// @Success 200 {object} UserEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(ctx *fasthttp.RequestCtx) {
	var req LoginRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	user, err := h.auth.AuthenticateBasic(httpctx.FromRequest(ctx), req.Username, req.Password)
	if err != nil {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	fph := h.requestFingerprint(ctx)
	token, err := h.auth.CreateSession(httpctx.FromRequest(ctx), user, fph)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	refresh, _, err := h.auth.IssueRefresh(httpctx.FromRequest(ctx), user.ID, fph)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	csrf, err := h.auth.NewCSRFToken()
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	h.issueSessionCookies(ctx, token, refresh, csrf)
	httpstatus.WriteData(ctx, fasthttp.StatusOK, toUserResponse(user))
}

// Refresh godoc
//
// @Summary Refresh
// @Tags Auth
// @Produce json
// @Success 200 {object} UserEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(ctx *fasthttp.RequestCtx) {
	raw := commonstrings.BytesToString(ctx.Request.Header.Cookie(auth.RefreshCookie))
	if commonstrings.IsEmpty(raw) {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	fph := h.requestFingerprint(ctx)
	user, access, refresh, err := h.auth.RotateRefresh(httpctx.FromRequest(ctx), raw, fph)
	if err != nil {
		if errors.Is(err, auth.ErrRefreshReuse) {
			httpctx.SetCookie(ctx, h.auth.ClearSessionCookie())
			httpctx.SetCookie(ctx, h.auth.ClearRefreshCookie())
			httpctx.SetCookie(ctx, h.auth.ClearCSRFCookie())
		}
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	csrf, err := h.auth.NewCSRFToken()
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	h.issueSessionCookies(ctx, access, refresh, csrf)
	httpstatus.WriteData(ctx, fasthttp.StatusOK, toUserResponse(user))
}

// Logout godoc
//
// @Summary Logout
// @Tags Auth
// @Produce json
// @Success 204 "No content"
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(ctx *fasthttp.RequestCtx) {
	c := httpctx.FromRequest(ctx)
	fph := h.requestFingerprint(ctx)
	sessionTok := commonstrings.BytesToString(ctx.Request.Header.Cookie(auth.SessionCookie))
	refreshTok := commonstrings.BytesToString(ctx.Request.Header.Cookie(auth.RefreshCookie))

	if !commonstrings.IsEmpty(sessionTok) {
		if user, err := h.auth.ParseSession(c, sessionTok, fph); err == nil {
			h.auth.RevokeRefreshTokensForUser(c, user.ID)
		} else if !commonstrings.IsEmpty(refreshTok) {
			h.auth.RevokeRefreshToken(c, refreshTok)
		}
		h.auth.InvalidateSession(c, sessionTok)
	} else if !commonstrings.IsEmpty(refreshTok) {
		h.auth.RevokeRefreshToken(c, refreshTok)
	}

	httpctx.SetCookie(ctx, h.auth.ClearSessionCookie())
	httpctx.SetCookie(ctx, h.auth.ClearRefreshCookie())
	httpctx.SetCookie(ctx, h.auth.ClearCSRFCookie())
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
