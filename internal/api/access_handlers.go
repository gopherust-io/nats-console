package api

import (
	"errors"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/app/policy"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/ipset"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type AccessHandler struct {
	access *app.AccessService
	users  *app.UserService
	auth   *auth.Service
	cfg    config.Config
}

func NewAccessHandler(access *app.AccessService, users *app.UserService, authSvc *auth.Service, cfg config.Config) *AccessHandler {
	return &AccessHandler{access: access, users: users, auth: authSvc, cfg: cfg}
}

// ListSystemAccess godoc
//
// @Summary List System Access
// @Tags Access
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} AccessGrantListEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/access [get]
func (h *AccessHandler) ListSystemAccess(ctx *fasthttp.RequestCtx) {
	h.listResourceAccess(ctx, domain.ResourceSystem, apikit.ClusterID(ctx))
}

// UpsertSystemAccess godoc
//
// @Summary Upsert System Access
// @Tags Access
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} AccessGrantEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/access [post]
// @Router /api/v1/clusters/{clusterId}/access [put]
func (h *AccessHandler) UpsertSystemAccess(ctx *fasthttp.RequestCtx) {
	h.upsertResourceAccess(ctx, domain.ResourceSystem, apikit.ClusterID(ctx), true)
}

// DeleteSystemAccess godoc
//
// @Summary Delete System Access
// @Tags Access
// @Param clusterId path string true "clusterId"
// @Param grantId path string true "grantId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/access/{grantId} [delete]
func (h *AccessHandler) DeleteSystemAccess(ctx *fasthttp.RequestCtx) {
	h.deleteResourceAccess(ctx, domain.ResourceSystem, apikit.ClusterID(ctx))
}

// ListAccountAccess godoc
//
// @Summary List Account Access
// @Tags Access
// @Param clusterId path string true "clusterId"
// @Param account path string true "account"
// @Produce json
// @Success 200 {object} AccessGrantListEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/accounts/{account}/access [get]
func (h *AccessHandler) ListAccountAccess(ctx *fasthttp.RequestCtx) {
	account := httpctx.RouteParam(ctx, "account")
	if commonstrings.IsEmpty(account) {
		account = "Default"
	}
	if err := domain.ValidateAccountName(account); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.listResourceAccess(ctx, domain.ResourceAccount, domain.AccountResourceKey(apikit.ClusterID(ctx), account))
}

// UpsertAccountAccess godoc
//
// @Summary Upsert Account Access
// @Tags Access
// @Param clusterId path string true "clusterId"
// @Param account path string true "account"
// @Produce json
// @Success 200 {object} AccessGrantEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/accounts/{account}/access [post]
// @Router /api/v1/clusters/{clusterId}/accounts/{account}/access [put]
func (h *AccessHandler) UpsertAccountAccess(ctx *fasthttp.RequestCtx) {
	account := httpctx.RouteParam(ctx, "account")
	if commonstrings.IsEmpty(account) {
		account = "Default"
	}
	if err := domain.ValidateAccountName(account); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.upsertResourceAccess(ctx, domain.ResourceAccount, domain.AccountResourceKey(apikit.ClusterID(ctx), account), false)
}

// DeleteAccountAccess godoc
//
// @Summary Delete Account Access
// @Tags Access
// @Param clusterId path string true "clusterId"
// @Param account path string true "account"
// @Param grantId path string true "grantId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/accounts/{account}/access/{grantId} [delete]
func (h *AccessHandler) DeleteAccountAccess(ctx *fasthttp.RequestCtx) {
	account := httpctx.RouteParam(ctx, "account")
	if commonstrings.IsEmpty(account) {
		account = "Default"
	}
	if err := domain.ValidateAccountName(account); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.deleteResourceAccess(ctx, domain.ResourceAccount, domain.AccountResourceKey(apikit.ClusterID(ctx), account))
}

// InvitePerson godoc
//
// @Summary Invite Person
// @Tags Users
// @Produce json
// @Success 200 {object} UserInviteEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/people/invite [post]
func (h *AccessHandler) InvitePerson(ctx *fasthttp.RequestCtx) {
	actor, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	if !policy.AuthorizeManageUsers(actor) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	var req struct {
		Username   string   `json:"username"`
		Email      string   `json:"email"`
		Roles      []string `json:"roles"`
		ClusterIDs []string `json:"clusterIds"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if commonstrings.IsEmpty(req.Username) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("username"))
		return
	}
	if len(req.Roles) == 0 {
		req.Roles = []string{domain.RoleViewer}
	}
	if commonstrings.IsEmpty(req.Email) {
		req.Email = req.Username + "@local"
	}
	clusterIDs := append([]string(nil), req.ClusterIDs...)
	if len(clusterIDs) == 0 {
		if actor.AccessRules != nil {
			clusterIDs = append([]string(nil), actor.AccessRules.ClusterIDs...)
		}
	}
	if len(clusterIDs) == 0 {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("clusterIds required: assign at least one cluster"))
		return
	}
	user, err := h.users.Create(httpctx.FromRequest(ctx), actor, domain.UserCreate{
		Username: req.Username,
		Email:    req.Email,
		Roles:    req.Roles,
		AccessRules: &domain.AccessRules{
			ClusterIDs:      clusterIDs,
			AssignableRoles: []string{domain.RoleViewer},
		},
	})
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}

	invite, err := h.access.CreateInvite(httpctx.FromRequest(ctx), user.ID, h.cfg.InviteTTL)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}

	httpstatus.WriteData(ctx, fasthttp.StatusCreated, struct {
		User      UserResponse `json:"user"`
		InviteURL string       `json:"inviteUrl"`
		ExpiredAt string       `json:"expiresAt"`
	}{
		User:      toUserResponse(user),
		InviteURL: strings.TrimSuffix(h.cfg.PublicBaseURL, "/") + "/invite/" + invite.Token,
		ExpiredAt: invite.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// GetInvite godoc
//
// @Summary Get Invite
// @Tags Auth
// @Param token path string true "token"
// @Produce json
// @Success 200 {object} UserInviteEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/auth/invite/{token} [get]
func (h *AccessHandler) GetInvite(ctx *fasthttp.RequestCtx) {
	token := httpctx.RouteParam(ctx, "token")
	invite, err := h.access.GetInvite(httpctx.FromRequest(ctx), token)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if invite.AcceptedAt != nil || time.Now().UTC().After(invite.ExpiresAt) {
		httpstatus.WriteErrorMessage(ctx, fasthttp.StatusGone, httpstatus.CodeGone, "invite expired or already used")
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		ExpiredAt string `json:"expiresAt"`
	}{
		Username:  invite.Username,
		Email:     invite.Email,
		ExpiredAt: invite.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// AcceptInvite godoc
//
// @Summary Accept Invite
// @Tags Auth
// @Produce json
// @Success 200 {object} UserEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Router /api/v1/auth/invite/accept [post]
func (h *AccessHandler) AcceptInvite(ctx *fasthttp.RequestCtx) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Token) || commonstrings.IsEmpty(req.Password) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("token and password required"))
		return
	}
	user, err := h.access.AcceptInvite(httpctx.FromRequest(ctx), req.Token, req.Password)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		if errors.Is(err, domain.ErrConflict) {
			httpstatus.WriteError(ctx, fasthttp.StatusConflict, err)
			return
		}
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.auth.InvalidateUser(httpctx.FromRequest(ctx), user.ID)
	trusted := ipset.ParseTrustedProxies(h.cfg.TrustedProxyList())
	fph := auth.DeviceFingerprint(
		commonstrings.BytesToString(ctx.Request.Header.UserAgent()),
		httpctx.ClientIP(ctx, trusted),
	)
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
	httpctx.SetCookie(ctx, h.auth.SessionCookie(token))
	httpctx.SetCookie(ctx, h.auth.RefreshTokenCookie(refresh))
	httpctx.SetCookie(ctx, h.auth.CSRFCookie(csrf))
	httpstatus.WriteData(ctx, fasthttp.StatusOK, toUserResponse(user))
}

func (h *AccessHandler) listResourceAccess(ctx *fasthttp.RequestCtx, resourceType, resourceKey string) {
	actor, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	if !h.canManage(actor, resourceType, resourceKey) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	grants, err := h.access.ListGrantsByResource(httpctx.FromRequest(ctx), resourceType, resourceKey)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, apikit.NonNilSlice(grants), apikit.TotalMeta(len(grants)))
}

func (h *AccessHandler) upsertResourceAccess(ctx *fasthttp.RequestCtx, resourceType, resourceKey string, systemOnly bool) {
	actor, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	if !h.canManage(actor, resourceType, resourceKey) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	var req struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if systemOnly && req.Role == domain.GrantCredentialDownloader {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("credential_downloader is account-scoped"))
		return
	}
	if req.Role == domain.GrantAdmin && !policy.AuthorizeMintAdminGrant(actor, resourceType, resourceKey) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	grant, err := h.access.UpsertGrant(httpctx.FromRequest(ctx), domain.AccessGrantUpsert{
		UserID:       req.UserID,
		ResourceType: resourceType,
		ResourceKey:  resourceKey,
		Role:         req.Role,
	})
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.auth.InvalidateUser(httpctx.FromRequest(ctx), req.UserID)
	httpstatus.WriteData(ctx, fasthttp.StatusOK, grant)
}

func (h *AccessHandler) deleteResourceAccess(ctx *fasthttp.RequestCtx, resourceType, resourceKey string) {
	actor, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	if !h.canManage(actor, resourceType, resourceKey) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	grantID := httpctx.RouteParam(ctx, "grantId")
	userID := commonstrings.BytesToString(ctx.QueryArgs().Peek("userId"))
	c := httpctx.FromRequest(ctx)
	if !commonstrings.IsEmpty(grantID) {
		deletedUserID, err := h.access.DeleteGrantScoped(c, grantID, resourceType, resourceKey)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
				return
			}
			apikit.WriteAPIError(ctx, err)
			return
		}
		h.auth.InvalidateUser(c, deletedUserID)
		ctx.SetStatusCode(fasthttp.StatusNoContent)
		return
	}
	if commonstrings.IsEmpty(userID) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("userId"))
		return
	}
	if err := h.access.DeleteGrantByResource(c, userID, resourceType, resourceKey); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		apikit.WriteAPIError(ctx, err)
		return
	}
	h.auth.InvalidateUser(c, userID)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *AccessHandler) canManage(actor domain.User, resourceType, resourceKey string) bool {
	switch resourceType {
	case domain.ResourceSystem:
		return policy.AuthorizeManageSystemAccess(actor, resourceKey)
	case domain.ResourceAccount:
		clusterID, account, _ := strings.Cut(resourceKey, ":")
		return policy.AuthorizeManageAccountAccess(actor, clusterID, account)
	default:
		return policy.AuthorizeManageUsers(actor) || actor.IsRoot
	}
}
