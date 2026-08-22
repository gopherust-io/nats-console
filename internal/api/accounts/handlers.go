// Package accounts serves the NATS account bounded context: NATS users and
// their credentials, signing groups, and cross-account sharing exports.
package accounts

import (
	"errors"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/app/policy"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type Handler struct {
	accounts *app.NATSAccountService
	auth     *auth.Service
	cfg      config.Config
}

func NewHandler(accounts *app.NATSAccountService, authSvc *auth.Service, cfg config.Config) *Handler {
	return &Handler{accounts: accounts, auth: authSvc, cfg: cfg}
}

func (h *Handler) accountFromCtx(ctx *fasthttp.RequestCtx) string {
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
	if !policy.AuthorizeAccessAccount(user, apikit.ClusterID(ctx), account) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}

// requireManageAccountAccess gates assign/share flows that mint credential grants.
func requireManageAccountAccess(ctx *fasthttp.RequestCtx, account string) bool {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	if !policy.AuthorizeManageAccountAccess(user, apikit.ClusterID(ctx), account) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}

// requireMutateAccountAccess gates create/update/delete of NATS users, signing
// groups, and exports. Account observers and nats_user-only grants may read
// (via requireAccountAccess) but must not mutate account control-plane data.
// Cluster-write operators and account admins are allowed.
func requireMutateAccountAccess(ctx *fasthttp.RequestCtx, account string) bool {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	cid := apikit.ClusterID(ctx)
	if !policy.AuthorizeAccessAccount(user, cid, account) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	if policy.AuthorizeManageAccountAccess(user, cid, account) || policy.AuthorizeWriteCluster(user, cid) {
		return true
	}
	httpstatus.WriteForbidden(ctx)
	return false
}

// natsUserIDFromCtx returns the route userId after UUID validation.
// Non-UUID values (e.g. static path segments mis-routed as {userId}) get a 400.
func natsUserIDFromCtx(ctx *fasthttp.RequestCtx) (string, bool) {
	userID := httpctx.RouteParam(ctx, "userId")
	if err := apikit.ValidateUUID(userID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return "", false
	}
	return userID, true
}

// ListUsers godoc
//
// @Summary List Users
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.NATSUserListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users [get]
func (h *Handler) ListUsers(ctx *fasthttp.RequestCtx) {
	clusterID := apikit.ClusterID(ctx)
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	users, err := h.accounts.ListUsers(httpctx.FromRequest(ctx), clusterID, account)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		users = filterNATSUsersForGrants(user, clusterID, account, users)
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, apikit.NonNilSlice(users), apikit.TotalMeta(len(users)))
}

// SubjectPermissions godoc
//
// @Summary Subject Permissions
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.SubjectPermissionsEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/subject-permissions [get]
// @Router /api/v1/clusters/{clusterId}/nats-users/subject-permissions [get]
func (h *Handler) SubjectPermissions(ctx *fasthttp.RequestCtx) {
	subject := strings.TrimSpace(commonstrings.BytesToString(ctx.QueryArgs().Peek("subject")))
	if commonstrings.IsEmpty(subject) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("subject"))
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	result, err := h.accounts.SubjectPermissions(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, subject)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		if !canListAllAccountNATSUsers(user, apikit.ClusterID(ctx), account) {
			result = filterSubjectPermissionsForGrants(user, apikit.ClusterID(ctx), account, result)
		}
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, result)
}

// GetUser godoc
//
// @Summary Get User
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} api.NATSUserEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId} [get]
func (h *Handler) GetUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	cid := apikit.ClusterID(ctx)
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		if !canViewNATSUser(user, cid, account, userID) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	}
	user, err := h.accounts.GetUser(httpctx.FromRequest(ctx), cid, account, userID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, user)
}

// canListAllAccountNATSUsers is true for cluster-wide or account-level grants.
func canListAllAccountNATSUsers(user domain.User, clusterID, account string) bool {
	if policy.AuthorizeAccessCluster(user, clusterID) {
		return true
	}
	accountKey := domain.AccountResourceKey(clusterID, account)
	for _, g := range user.Grants {
		if g.ResourceType == domain.ResourceAccount && g.ResourceKey == accountKey {
			return true
		}
	}
	return false
}

func grantedNATSUserIDs(user domain.User, clusterID, account string) map[string]struct{} {
	prefix := domain.AccountResourceKey(clusterID, account) + ":"
	out := make(map[string]struct{})
	for _, g := range user.Grants {
		if g.ResourceType != domain.ResourceNATSUser {
			continue
		}
		if !strings.HasPrefix(g.ResourceKey, prefix) {
			continue
		}
		id := strings.TrimPrefix(g.ResourceKey, prefix)
		if commonstrings.IsEmpty(id) || strings.Contains(id, ":") {
			continue
		}
		out[id] = struct{}{}
	}
	return out
}

func canViewNATSUser(user domain.User, clusterID, account, natsUserID string) bool {
	if canListAllAccountNATSUsers(user, clusterID, account) {
		return true
	}
	_, ok := grantedNATSUserIDs(user, clusterID, account)[natsUserID]
	return ok
}

func filterNATSUsersForGrants(user domain.User, clusterID, account string, users []domain.NATSAccountUser) []domain.NATSAccountUser {
	if canListAllAccountNATSUsers(user, clusterID, account) {
		return users
	}
	allowed := grantedNATSUserIDs(user, clusterID, account)
	if len(allowed) == 0 {
		return nil
	}
	out := make([]domain.NATSAccountUser, 0, len(allowed))
	for _, u := range users {
		if _, ok := allowed[u.ID]; ok {
			out = append(out, u)
		}
	}
	return out
}

func filterSubjectPermissionsForGrants(user domain.User, clusterID, account string, result domain.SubjectPermissionsResult) domain.SubjectPermissionsResult {
	allowed := grantedNATSUserIDs(user, clusterID, account)
	if len(allowed) == 0 {
		result.Publish = nil
		result.Subscribe = nil
		result.QueueSubscribe = nil
		return result
	}
	result.Publish = filterSubjectPermissionEntries(result.Publish, allowed)
	result.Subscribe = filterSubjectPermissionEntries(result.Subscribe, allowed)
	result.QueueSubscribe = filterSubjectPermissionEntries(result.QueueSubscribe, allowed)
	return result
}

func filterSubjectPermissionEntries(entries []domain.SubjectPermissionEntry, allowed map[string]struct{}) []domain.SubjectPermissionEntry {
	if len(entries) == 0 {
		return entries
	}
	out := make([]domain.SubjectPermissionEntry, 0, len(entries))
	for _, e := range entries {
		if _, ok := allowed[e.UserID]; ok {
			out = append(out, e)
		}
	}
	return out
}

func requireListAllAccountNATSUsers(ctx *fasthttp.RequestCtx, account string) bool {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return false
	}
	if !canListAllAccountNATSUsers(user, apikit.ClusterID(ctx), account) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}

// goalign:ignore
type natsUserConfigRequest struct {
	Name                   string                     `json:"name"`
	AccountName            string                     `json:"accountName"`
	SigningGroup           string                     `json:"signingGroup"`
	Tags                   []string                   `json:"tags"`
	PubAllow               []string                   `json:"pubAllow"`
	PubDeny                []string                   `json:"pubDeny"`
	SubAllow               []string                   `json:"subAllow"`
	SubDeny                []string                   `json:"subDeny"`
	AllowedConnectionTypes []string                   `json:"allowedConnectionTypes"`
	SrcCIDRs               []string                   `json:"srcCidrs"`
	TimesLocale            string                     `json:"timesLocale"`
	TimeRanges             []domain.NATSUserTimeRange `json:"timeRanges"`
	MaxSubs                int64                      `json:"maxSubs"`
	MaxPayload             int64                      `json:"maxPayload"`
	MaxData                int64                      `json:"maxData"`
	JWTLifetimeNs          int64                      `json:"jwtLifetimeNs"`
	RespMaxMsgs            int                        `json:"respMaxMsgs"`
	RespTTLNs              int64                      `json:"respTTLNs"`
	BearerToken            bool                       `json:"bearerToken"`
	ProxyRequired          bool                       `json:"proxyRequired"`
}

// CreateUser godoc
//
// @Summary Create User
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 201 {object} api.NATSUserEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users [post]
func (h *Handler) CreateUser(ctx *fasthttp.RequestCtx) {
	var req natsUserConfigRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("name"))
		return
	}
	if commonstrings.IsEmpty(req.AccountName) {
		req.AccountName = "Default"
	}
	if err := domain.ValidateAccountName(req.AccountName); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if !requireMutateAccountAccess(ctx, req.AccountName) {
		return
	}
	user, err := h.accounts.CreateUser(httpctx.FromRequest(ctx), domain.NATSAccountUserCreate{
		ClusterID:              apikit.ClusterID(ctx),
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
	if !policy.AuthorizeDownloadCreds(user, apikit.ClusterID(ctx), account, natsUserID) {
		httpstatus.WriteForbidden(ctx)
		return false
	}
	return true
}

func redactNATSCredsUnlessAllowed(ctx *fasthttp.RequestCtx, account string, creds domain.NATSAccountUserCreds) domain.NATSAccountUserCreds {
	user, ok := auth.UserFromContext(httpctx.FromRequest(ctx))
	if !ok || !policy.AuthorizeDownloadCreds(user, apikit.ClusterID(ctx), account, creds.ID) {
		creds.Seed = ""
		creds.Cred = ""
		creds.JWT = ""
	}
	return creds
}

// UpdateUser godoc
//
// @Summary Update User
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} api.NATSUserEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId} [put]
func (h *Handler) UpdateUser(ctx *fasthttp.RequestCtx) {
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
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	user, err := h.accounts.UpdateUser(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, userID, domain.NATSAccountUserUpdate{
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

// DeleteUser godoc
//
// @Summary Delete User
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId} [delete]
func (h *Handler) DeleteUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	c := httpctx.FromRequest(ctx)
	affected, err := h.accounts.DeleteUser(c, apikit.ClusterID(ctx), account, userID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	for _, id := range affected {
		h.auth.InvalidateUser(c, id)
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// DownloadCreds godoc
//
// @Summary Download Creds
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} api.NATSCredsEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId}/creds [get]
func (h *Handler) DownloadCreds(ctx *fasthttp.RequestCtx) {
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
	creds, err := h.accounts.GetCreds(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, creds)
}

// RotateUser godoc
//
// @Summary Rotate User
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} api.NATSCredsEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId}/rotate [post]
func (h *Handler) RotateUser(ctx *fasthttp.RequestCtx) {
	userID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	creds, err := h.accounts.RotateUser(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, userID, h.cfg.NATS.AccountSeed)
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

// MintJWT godoc
//
// @Summary Mint JWT
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} api.NATSCredsEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId}/mint-jwt [post]
func (h *Handler) MintJWT(ctx *fasthttp.RequestCtx) {
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
	creds, err := h.accounts.MintJWT(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, userID, h.cfg.NATS.AccountSeed)
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

// AssignPerson godoc
//
// @Summary Assign Person
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param userId path string true "userId"
// @Produce json
// @Success 200 {object} api.NATSUserEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/nats-users/{userId}/assign [post]
func (h *Handler) AssignPerson(ctx *fasthttp.RequestCtx) {
	natsUserID, ok := natsUserIDFromCtx(ctx)
	if !ok {
		return
	}
	account := h.accountFromCtx(ctx)
	// Assign mints a credential_downloader grant — operators with write must not
	// self-escalate into CanDownloadCreds via assign alone.
	if !requireManageAccountAccess(ctx, account) {
		return
	}
	var req struct {
		UserID string `json:"userId"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	prev, err := h.accounts.GetUser(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, natsUserID)
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	user, err := h.accounts.AssignPerson(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, natsUserID, req.UserID)
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

// ListSigningGroups godoc
//
// @Summary List Signing Groups
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.SigningGroupListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/signing-groups [get]
func (h *Handler) ListSigningGroups(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		if !canListAllAccountNATSUsers(user, apikit.ClusterID(ctx), account) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	}
	groups, err := h.accounts.ListSigningGroups(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, apikit.NonNilSlice(groups), apikit.TotalMeta(len(groups)))
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

// CreateSigningGroup godoc
//
// @Summary Create Signing Group
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 201 {object} api.SigningGroupEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/signing-groups [post]
func (h *Handler) CreateSigningGroup(ctx *fasthttp.RequestCtx) {
	var req createSigningGroupRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.AccountName) {
		req.AccountName = h.accountFromCtx(ctx)
	}
	if err := domain.ValidateAccountName(req.AccountName); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if !requireMutateAccountAccess(ctx, req.AccountName) {
		return
	}
	group, err := h.accounts.CreateSigningGroup(httpctx.FromRequest(ctx), domain.SigningGroupCreate{
		ClusterID:   apikit.ClusterID(ctx),
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

// UpdateSigningGroup godoc
//
// @Summary Update Signing Group
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param groupId path string true "groupId"
// @Produce json
// @Success 200 {object} api.SigningGroupEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/signing-groups/{groupId} [put]
func (h *Handler) UpdateSigningGroup(ctx *fasthttp.RequestCtx) {
	var req createSigningGroupRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	group, err := h.accounts.UpdateSigningGroup(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, httpctx.RouteParam(ctx, "groupId"), domain.SigningGroupUpdate{
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
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, group)
}

// DeleteSigningGroup godoc
//
// @Summary Delete Signing Group
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param groupId path string true "groupId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/signing-groups/{groupId} [delete]
func (h *Handler) DeleteSigningGroup(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	err := h.accounts.DeleteSigningGroup(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, httpctx.RouteParam(ctx, "groupId"))
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

// ListExports godoc
//
// @Summary List Exports
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ExportListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/sharing/exports [get]
func (h *Handler) ListExports(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireAccountAccess(ctx, account) {
		return
	}
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		if !canListAllAccountNATSUsers(user, apikit.ClusterID(ctx), account) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	}
	kind := commonstrings.BytesToString(ctx.QueryArgs().Peek("kind"))
	items, err := h.accounts.ListExports(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, kind)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, apikit.NonNilSlice(items), apikit.TotalMeta(len(items)))
}

type createExportRequest struct {
	AccountName string `json:"accountName"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

// CreateExport godoc
//
// @Summary Create Export
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 201 {object} api.ExportEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/sharing/exports [post]
func (h *Handler) CreateExport(ctx *fasthttp.RequestCtx) {
	var req createExportRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) || commonstrings.IsEmpty(req.Kind) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("name/kind"))
		return
	}
	if commonstrings.IsEmpty(req.AccountName) {
		req.AccountName = "Default"
	}
	if err := domain.ValidateAccountName(req.AccountName); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if !requireMutateAccountAccess(ctx, req.AccountName) {
		return
	}
	item, err := h.accounts.CreateExport(httpctx.FromRequest(ctx), domain.NATSAccountExportCreate{
		ClusterID:   apikit.ClusterID(ctx),
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

// UpdateExport godoc
//
// @Summary Update Export
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param exportId path string true "exportId"
// @Produce json
// @Success 200 {object} api.ExportEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/sharing/exports/{exportId} [put]
func (h *Handler) UpdateExport(ctx *fasthttp.RequestCtx) {
	var req createExportRequest
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Name) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, apikit.ErrMissing("name"))
		return
	}
	account := h.accountFromCtx(ctx)
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	item, err := h.accounts.UpdateExport(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, httpctx.RouteParam(ctx, "exportId"), domain.NATSAccountExportUpdate{
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, item)
}

// DeleteExport godoc
//
// @Summary Delete Export
// @Tags NATSAccounts
// @Param clusterId path string true "clusterId"
// @Param exportId path string true "exportId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/sharing/exports/{exportId} [delete]
func (h *Handler) DeleteExport(ctx *fasthttp.RequestCtx) {
	account := h.accountFromCtx(ctx)
	if !requireMutateAccountAccess(ctx, account) {
		return
	}
	err := h.accounts.DeleteExport(httpctx.FromRequest(ctx), apikit.ClusterID(ctx), account, httpctx.RouteParam(ctx, "exportId"))
	if errors.Is(err, domain.ErrNotFound) {
		httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
