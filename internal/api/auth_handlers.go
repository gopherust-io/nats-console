package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
)

type AuthHandler struct {
	auth *auth.Service
}

func NewAuthHandler(authSvc *auth.Service) *AuthHandler {
	return &AuthHandler{auth: authSvc}
}

func (h *AuthHandler) Config(ctx *fasthttp.RequestCtx) {
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"oidc_enabled":  h.auth.OIDCEnabled(),
		"basic_enabled": h.auth.BasicAuthEnabled(),
		"auth_enabled":  h.auth.AuthEnabled(),
	})
}

func (h *AuthHandler) Me(ctx *fasthttp.RequestCtx) {
	c := requestContext(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	writeUserJSON(ctx, fasthttp.StatusOK, user)
}

func (h *AuthHandler) Login(ctx *fasthttp.RequestCtx) {
	if !h.auth.BasicAuthEnabled() {
		writeError(ctx, fasthttp.StatusNotFound, auth.ErrUnauthorized)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	user, err := h.auth.AuthenticateBasic(requestContext(ctx), req.Username, req.Password)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		ctx.SetBodyString("unauthorized")
		return
	}
	token, err := h.auth.CreateSession(user)
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	setCookie(ctx, h.auth.SessionCookie(token))
	writeUserJSON(ctx, fasthttp.StatusOK, user)
}

func (h *AuthHandler) Logout(ctx *fasthttp.RequestCtx) {
	setCookie(ctx, h.auth.ClearSessionCookie())
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AuthHandler) OIDCLogin(ctx *fasthttp.RequestCtx) {
	if !h.auth.OIDCEnabled() {
		writeError(ctx, fasthttp.StatusNotFound, auth.ErrUnauthorized)
		return
	}
	state, err := auth.NewOAuthState()
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	setCookie(ctx, h.auth.OAuthStateCookie(state))
	ctx.Redirect(h.auth.OIDCAuthURL(state), fasthttp.StatusFound)
}

func (h *AuthHandler) OIDCCallback(ctx *fasthttp.RequestCtx) {
	if !h.auth.OIDCEnabled() {
		writeError(ctx, fasthttp.StatusNotFound, auth.ErrUnauthorized)
		return
	}
	if errMsg := string(ctx.QueryArgs().Peek("error")); errMsg != "" {
		setCookie(ctx, h.auth.ClearOAuthStateCookie())
		ctx.Redirect("/login?error="+url.QueryEscape(errMsg), fasthttp.StatusFound)
		return
	}
	state := string(ctx.QueryArgs().Peek("state"))
	cookieState := string(ctx.Request.Header.Cookie("nats_consol_oauth_state"))
	if state == "" || cookieState == "" || state != cookieState {
		setCookie(ctx, h.auth.ClearOAuthStateCookie())
		ctx.Redirect("/login?error=invalid_state", fasthttp.StatusFound)
		return
	}
	code := string(ctx.QueryArgs().Peek("code"))
	if code == "" {
		setCookie(ctx, h.auth.ClearOAuthStateCookie())
		ctx.Redirect("/login?error=missing_code", fasthttp.StatusFound)
		return
	}
	user, err := h.auth.HandleOIDCCallback(requestContext(ctx), code)
	if err != nil {
		setCookie(ctx, h.auth.ClearOAuthStateCookie())
		ctx.Redirect("/login?error="+url.QueryEscape("oidc_failed"), fasthttp.StatusFound)
		return
	}
	token, err := h.auth.CreateSession(user)
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	setCookie(ctx, h.auth.SessionCookie(token))
	setCookie(ctx, h.auth.ClearOAuthStateCookie())
	ctx.Redirect("/", fasthttp.StatusFound)
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
