package middleware

import (
	"strings"

	"github.com/valyala/fasthttp"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

const pathCtxKey = "path"

const (
	pathPrefixAuthLogin        = "/api/v1/auth/login"
	pathPrefixAuthLogout       = "/api/v1/auth/logout"
	pathPrefixAuthRefresh      = "/api/v1/auth/refresh"
	pathPrefixAuthConfig       = "/api/v1/auth/config"
	pathPrefixAuthInvite       = "/api/v1/auth/invite/"
	pathPrefixAuthAcceptInvite = "/api/v1/auth/invite/accept"
	pathPrefixAssistant        = "/api/v1/assistant"

	PathPrefixPprof           = "/debug/pprof"
	pathPrefixPprofProfile    = "/api/v1/pprof/profile/profile"
	pathPrefixPprofProfileCPU = "/api/v1/pprof/profile/cpu"

	pathPrefixHealth  = "/api/health"
	pathPrefixOpenAPI = "/api/openapi.yaml"
)

func requestPath(ctx *fasthttp.RequestCtx) string {
	if path, ok := ctx.UserValue(pathCtxKey).(string); ok && !commonstrings.IsEmpty(path) {
		return path
	}
	return commonstrings.BytesToString(ctx.Path())
}

func isLongRunningProfilePath(path string) bool {
	if isPprofPath(path) {
		return strings.HasPrefix(path, PathPrefixPprof+"/profile")
	}
	return strings.HasPrefix(path, pathPrefixPprofProfileCPU) || strings.HasPrefix(path, pathPrefixPprofProfile)
}

func isPprofPath(path string) bool {
	return path == PathPrefixPprof || strings.HasPrefix(path, PathPrefixPprof+"/")
}

func isPublicPath(path string) bool {
	switch path {
	case pathPrefixHealth,
		pathPrefixOpenAPI,
		pathPrefixAuthConfig,
		pathPrefixAuthLogin,
		pathPrefixAuthLogout,
		pathPrefixAuthRefresh,
		pathPrefixAuthAcceptInvite:
		return true
	default:
		return strings.HasPrefix(path, pathPrefixAuthInvite)
	}
}
