package api

import (
	"strconv"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	"github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type UsersHandler struct {
	svc *app.Services
	cfg config.Config
}

func NewUsersHandler(svc *app.Services, cfg config.Config) *UsersHandler {
	return &UsersHandler{svc: svc, cfg: cfg}
}

func (h *UsersHandler) List(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	users, err := h.svc.Users.List(httpctx.FromRequest(ctx), actor)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	items := toUserResponses(users)
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, items, totalMeta(len(items)))
}

func (h *UsersHandler) Create(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	var req struct {
		AccessRules *domain.AccessRules `json:"accessRules"`
		Username    string              `json:"username"`
		Email       string              `json:"email"`
		Password    string              `json:"password"`
		Roles       []string            `json:"roles"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if strings.IsEmpty(req.Username) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("username"))
		return
	}
	if err := validatePassword(req.Password); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if len(req.Roles) == 0 {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
		return
	}
	user, err := h.svc.Users.Create(httpctx.FromRequest(ctx), actor, domain.UserCreate{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		Roles:       req.Roles,
		AccessRules: req.AccessRules,
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, userResponseFromDomain(user))
}

func (h *UsersHandler) Update(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	userID := httpctx.RouteParam(ctx, "userId")
	if err := validateUUID(userID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	var req struct {
		Email       *string  `json:"email"`
		Password    *string  `json:"password"`
		Roles       []string `json:"roles"`
		AccessRules []byte   `json:"accessRules"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	update := domain.UserUpdate{}
	if req.Email != nil {
		update.Email = req.Email
	}
	if req.Password != nil {
		if err := validatePassword(*req.Password); err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		update.Password = req.Password
	}
	if req.Roles != nil {
		if len(req.Roles) == 0 {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
			return
		}
		update.Roles = req.Roles
		update.SetRoles = true
	}
	if len(req.AccessRules) > 0 {
		update.SetRules = true
		if strings.BytesToString(req.AccessRules) == "null" {
			update.AccessRules = nil
		} else {
			var rules domain.AccessRules
			if err := serializer.Unmarshal(req.AccessRules, &rules); err != nil {
				httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
				return
			}
			update.AccessRules = &rules
		}
	}
	user, err := h.svc.Users.Update(httpctx.FromRequest(ctx), actor, userID, update)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	h.svc.Auth.InvalidateUser(httpctx.FromRequest(ctx), userID)
	httpstatus.WriteData(ctx, fasthttp.StatusOK, userResponseFromDomain(user))
}

func (h *UsersHandler) Delete(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	userID := httpctx.RouteParam(ctx, "userId")
	if err := validateUUID(userID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.svc.Users.Delete(httpctx.FromRequest(ctx), actor, userID); err != nil {
		writeAPIError(ctx, err)
		return
	}
	h.svc.Auth.InvalidateUser(httpctx.FromRequest(ctx), userID)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *UsersHandler) SetRoles(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	userID := httpctx.RouteParam(ctx, "userId")
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if len(req.Roles) == 0 {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
		return
	}
	if err := validateUUID(userID); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	user, err := h.svc.Users.SetRoles(httpctx.FromRequest(ctx), actor, userID, req.Roles)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	h.svc.Auth.InvalidateUser(httpctx.FromRequest(ctx), userID)
	httpstatus.WriteData(ctx, fasthttp.StatusOK, userResponseFromDomain(user))
}

func actorFromContext(ctx *fasthttp.RequestCtx) (domain.User, bool) {
	c := httpctx.FromRequest(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok {
		return domain.User{}, false
	}
	return auth.StoreUserToDomain(user), true
}

type AuditHandler struct {
	svc *app.Services
	cfg config.Config
}

func NewAuditHandler(svc *app.Services, cfg config.Config) *AuditHandler {
	return &AuditHandler{svc: svc, cfg: cfg}
}

func (h *AuditHandler) List(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}

	offset, _ := strconv.Atoi(strings.BytesToString(ctx.QueryArgs().Peek("offset")))
	limit, _ := strconv.Atoi(strings.BytesToString(ctx.QueryArgs().Peek("limit")))
	limit = h.cfg.NormalizeAuditLimit(limit)
	if offset < 0 {
		offset = 0
	}
	clusterID := strings.BytesToString(ctx.QueryArgs().Peek("clusterId"))

	scope, err := auditFilterForActor(actor, clusterID)
	if err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusForbidden, err)
		return
	}
	scope.Limit = limit
	scope.Offset = offset

	entries, total, err := h.svc.Audit.List(httpctx.FromRequest(ctx), scope)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, nonNilSlice(entries), pageMeta(total, offset, limit))
}
