package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
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
		OIDCEnabled:   h.auth.OIDCEnabled(),
		OIDCProviders: h.auth.SSOProviders(),
		BasicEnabled:  h.auth.BasicAuthEnabled(),
		AuthEnabled:   h.auth.AuthEnabled(),
		AIEnabled:     h.cfg.AIActive(),
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
	user, err := h.auth.AuthenticateBasic(requestContext(ctx), req.Username, req.Password)
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
	setCookie(ctx, h.auth.ClearSessionCookie())
	setCookie(ctx, h.auth.ClearCSRFCookie())
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AuthHandler) OIDCLogin(ctx *fasthttp.RequestCtx) {
	h.ssoLogin(ctx, auth.ProviderLegacy)
}

func (h *AuthHandler) SSOProviderLogin(ctx *fasthttp.RequestCtx) {
	provider, _ := ctx.UserValue("provider").(string)
	h.ssoLogin(ctx, provider)
}

func (h *AuthHandler) ssoLogin(ctx *fasthttp.RequestCtx, provider string) {
	if !h.auth.OIDCEnabled() {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, auth.ErrUnauthorized)
		return
	}
	state, err := auth.NewOAuthState()
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	authURL, err := h.auth.SSOAuthURL(provider, state)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	setCookie(ctx, h.auth.OAuthStateCookie(state))
	setCookie(ctx, h.auth.OAuthProviderCookie(provider))
	ctx.Redirect(authURL, fasthttp.StatusFound)
}

func (h *AuthHandler) OIDCCallback(ctx *fasthttp.RequestCtx) {
	h.ssoCallback(ctx, auth.ProviderLegacy)
}

func (h *AuthHandler) SSOProviderCallback(ctx *fasthttp.RequestCtx) {
	provider, _ := ctx.UserValue("provider").(string)
	h.ssoCallback(ctx, provider)
}

func (h *AuthHandler) ssoCallback(ctx *fasthttp.RequestCtx, provider string) {
	if !h.auth.OIDCEnabled() {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, auth.ErrUnauthorized)
		return
	}
	if errMsg := string(ctx.QueryArgs().Peek("error")); errMsg != "" {
		h.clearOAuthCookies(ctx)
		ctx.Redirect("/login?error="+url.QueryEscape(errMsg), fasthttp.StatusFound)
		return
	}
	state := string(ctx.QueryArgs().Peek("state"))
	cookieState := string(ctx.Request.Header.Cookie("nats_consol_oauth_state"))
	cookieProvider := string(ctx.Request.Header.Cookie("nats_consol_oauth_provider"))
	if state == "" || cookieState == "" || state != cookieState || cookieProvider != provider {
		h.clearOAuthCookies(ctx)
		ctx.Redirect("/login?error=invalid_state", fasthttp.StatusFound)
		return
	}
	code := string(ctx.QueryArgs().Peek("code"))
	if code == "" {
		h.clearOAuthCookies(ctx)
		ctx.Redirect("/login?error=missing_code", fasthttp.StatusFound)
		return
	}
	user, err := h.auth.HandleSSOCallback(requestContext(ctx), provider, code)
	if err != nil {
		h.clearOAuthCookies(ctx)
		ctx.Redirect("/login?error="+url.QueryEscape("oidc_failed"), fasthttp.StatusFound)
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
	h.clearOAuthCookies(ctx)
	ctx.Redirect("/", fasthttp.StatusFound)
}

func (h *AuthHandler) clearOAuthCookies(ctx *fasthttp.RequestCtx) {
	setCookie(ctx, h.auth.ClearOAuthStateCookie())
	setCookie(ctx, h.auth.ClearOAuthProviderCookie())
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
