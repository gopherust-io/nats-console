package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

type AuthHandler struct {
	auth *auth.Service
	cfg  config.Config
}

func NewAuthHandler(authSvc *auth.Service, cfg config.Config) *AuthHandler {
	return &AuthHandler{auth: authSvc, cfg: cfg}
}

func (h *AuthHandler) Config(ctx *fasthttp.RequestCtx) {
	serializer.WriteJSON(ctx, fasthttp.StatusOK, AuthConfigResponse{
		BasicEnabled: h.auth.BasicAuthEnabled(),
		AuthEnabled:  h.auth.AuthEnabled(),
		AIEnabled:    h.cfg.AIActive(),
	})
}

func (h *AuthHandler) Me(ctx *fasthttp.RequestCtx) {
	c := httpctx.FromRequest(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	writeUserJSON(ctx, fasthttp.StatusOK, user)
}

func (h *AuthHandler) Login(ctx *fasthttp.RequestCtx) {
	if !h.auth.BasicAuthEnabled() {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, auth.ErrUnauthorized)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	user, err := h.auth.AuthenticateBasic(httpctx.FromRequest(ctx), req.Username, req.Password)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetBodyString("unauthorized")
		return
	}
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

func (h *AuthHandler) Logout(ctx *fasthttp.RequestCtx) {
	if token := string(ctx.Request.Header.Cookie(auth.SessionCookie)); token != "" {
		h.auth.InvalidateSession(token)
	}
	setCookie(ctx, h.auth.ClearSessionCookie())
	setCookie(ctx, h.auth.ClearCSRFCookie())
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func setCookie(ctx *fasthttp.RequestCtx, cookie *http.Cookie) {
	var b strings.Builder
	b.WriteString(cookie.Name)
	b.WriteString("=")
	b.WriteString(cookie.Value)
	b.WriteString("; Path=")
	b.WriteString(cookie.Path)
	if cookie.HttpOnly {
		b.WriteString("; HttpOnly")
	}
	if cookie.MaxAge != 0 {
		b.WriteString("; Max-Age=")
		b.WriteString(strconv.Itoa(cookie.MaxAge))
	}
	if cookie.Secure {
		b.WriteString("; Secure")
	}
	switch cookie.SameSite {
	case http.SameSiteLaxMode:
		b.WriteString("; SameSite=Lax")
	case http.SameSiteStrictMode:
		b.WriteString("; SameSite=Strict")
	}
	ctx.Response.Header.Set("Set-Cookie", b.String())
}
