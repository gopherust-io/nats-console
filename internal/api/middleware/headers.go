package middleware

const (
	HeaderRequestID                     = "X-Request-ID"
	HeaderCSRF                          = "X-CSRF-Token"
	HeaderContentType                   = "X-Content-Type-Options"
	HeaderFrameOptions                  = "X-Frame-Options"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderOrigin                        = "Origin"
	HeaderVary                          = "Vary"
	HeaderAuthorization                 = "Authorization"
	HeaderReferrerPolicy                = "Referrer-Policy"
	HeaderPermissionPolicy              = "Permissions-Policy"
	HeaderContentSecurityPolicy         = "Content-Security-Policy"
	HeaderStrictTransportSecurity       = "Strict-Transport-Security"
)

const (
	ctxKey          = "context"
	requestIDCtxKey = "request_id"
)
