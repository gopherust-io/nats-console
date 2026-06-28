package api

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
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
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	users, err := h.svc.Users.List(requestContext(ctx), actor)
	if err != nil {
		writeUserMgmtError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, UsersListResponse{
		Users: nonNilSlice(users),
		Total: len(users),
	})
}

func (h *UsersHandler) Create(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	var req struct {
		AccessRules *domain.AccessRules `json:"accessRules"`
		Username    string              `json:"username"`
		Email       string              `json:"email"`
		Password    string              `json:"password"`
		Roles       []string            `json:"roles"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Username == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("username"))
		return
	}
	if req.Password == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("password"))
		return
	}
	if len(req.Roles) == 0 {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
		return
	}
	user, err := h.svc.Users.Create(requestContext(ctx), actor, domain.UserCreate{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		Roles:       req.Roles,
		AccessRules: req.AccessRules,
	})
	if err != nil {
		writeUserMgmtError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, user)
}

func (h *UsersHandler) Update(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	userID := routeParam(ctx, "userId")
	if err := validateUUID(userID); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	var req struct {
		Email       *string         `json:"email"`
		Password    *string         `json:"password"`
		Roles       []string        `json:"roles"`
		AccessRules json.RawMessage `json:"accessRules"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	update := domain.UserUpdate{}
	if req.Email != nil {
		update.Email = req.Email
	}
	if req.Password != nil {
		update.Password = req.Password
	}
	if req.Roles != nil {
		if len(req.Roles) == 0 {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
			return
		}
		update.Roles = req.Roles
		update.SetRoles = true
	}
	if len(req.AccessRules) > 0 {
		update.SetRules = true
		if string(req.AccessRules) == "null" {
			update.AccessRules = nil
		} else {
			var rules domain.AccessRules
			if err := json.Unmarshal(req.AccessRules, &rules); err != nil {
				serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
				return
			}
			update.AccessRules = &rules
		}
	}
	user, err := h.svc.Users.Update(requestContext(ctx), actor, userID, update)
	if err != nil {
		writeUserMgmtError(ctx, err)
		return
	}
	h.svc.Auth.InvalidateUser(userID)
	serializer.WriteJSON(ctx, fasthttp.StatusOK, user)
}

func (h *UsersHandler) Delete(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	userID := routeParam(ctx, "userId")
	if err := validateUUID(userID); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.svc.Users.Delete(requestContext(ctx), actor, userID); err != nil {
		writeUserMgmtError(ctx, err)
		return
	}
	h.svc.Auth.InvalidateUser(userID)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *UsersHandler) SetRoles(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	userID := routeParam(ctx, "userId")
	var req struct {
		Roles []string `json:"roles"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if len(req.Roles) == 0 {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("roles"))
		return
	}
	if err := validateUUID(userID); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	user, err := h.svc.Users.SetRoles(requestContext(ctx), actor, userID, req.Roles)
	if err != nil {
		writeUserMgmtError(ctx, err)
		return
	}
	h.svc.Auth.InvalidateUser(userID)
	serializer.WriteJSON(ctx, fasthttp.StatusOK, user)
}

func actorFromContext(ctx *fasthttp.RequestCtx) (domain.User, bool) {
	c := requestContext(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok {
		return domain.User{}, false
	}
	return auth.StoreUserToDomain(user), true
}

func writeUserMgmtError(ctx *fasthttp.RequestCtx, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrRootProtected), errors.Is(err, domain.ErrCannotEscalate):
		serializer.WriteError(ctx, fasthttp.StatusForbidden, err)
	default:
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
	}
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
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}

	offset, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("offset")))
	limit, _ := strconv.Atoi(string(ctx.QueryArgs().Peek("limit")))
	limit = h.cfg.NormalizeAuditLimit(limit)
	if offset < 0 {
		offset = 0
	}
	clusterID := string(ctx.QueryArgs().Peek("clusterId"))

	scope, err := auditFilterForActor(actor, clusterID)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusForbidden, err)
		return
	}
	scope.Limit = limit
	scope.Offset = offset

	entries, total, err := h.svc.Audit.List(requestContext(ctx), scope)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, AuditListResponse{
		Entries: nonNilSlice(entries),
		Total:   total,
	})
}
