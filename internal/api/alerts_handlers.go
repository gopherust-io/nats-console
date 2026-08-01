package api

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

type AlertsHandler struct {
	alerts *app.AlertService
	cfg    config.Config
}

func NewAlertsHandler(alerts *app.AlertService, cfg config.Config) *AlertsHandler {
	return &AlertsHandler{alerts: alerts, cfg: cfg}
}

func (h *AlertsHandler) List(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}

	filter := domain.AlertFilter{
		Status:    commonstrings.BytesToString(ctx.QueryArgs().Peek("status")),
		ClusterID: commonstrings.BytesToString(ctx.QueryArgs().Peek("clusterId")),
		Severity:  commonstrings.BytesToString(ctx.QueryArgs().Peek("severity")),
		Limit:     queryInt(ctx, "limit", h.cfg.AuditDefaultLimit),
		Offset:    queryInt(ctx, "offset", 0),
	}
	if !commonstrings.IsEmpty(filter.Status) && filter.Status != domain.AlertStatusOpen && filter.Status != domain.AlertStatusClosed {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("invalid status"))
		return
	}
	if !commonstrings.IsEmpty(filter.Severity) && !domain.ValidAlertSeverity(filter.Severity) {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("invalid severity"))
		return
	}
	if !commonstrings.IsEmpty(filter.ClusterID) {
		if err := validateUUID(filter.ClusterID); err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		if !auth.CanAccessClusterOrAccount(storeUser, filter.ClusterID) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	} else if clusterIDs := accessibleClusterIDs(actor, storeUser); clusterIDs != nil {
		filter.ClusterIDs = clusterIDs
		if len(clusterIDs) == 0 {
			httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, []domain.Alert{}, totalMeta(0))
			return
		}
	}

	alerts, total, err := h.alerts.List(httpctx.FromRequest(ctx), filter)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, nonNilSlice(alerts), totalMeta(total))
}

func (h *AlertsHandler) OpenSummary(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	var clusterIDs []string
	if ids := accessibleClusterIDs(actor, storeUser); ids != nil {
		clusterIDs = ids
		if len(clusterIDs) == 0 {
			httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.AlertOpenSummary{Count: 0, Alerts: []domain.Alert{}})
			return
		}
	}
	alerts, total, err := h.alerts.OpenUnacknowledged(httpctx.FromRequest(ctx), clusterIDs, 8)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.AlertOpenSummary{
		Count:  total,
		Alerts: nonNilSlice(alerts),
	})
}

func (h *AlertsHandler) Get(ctx *fasthttp.RequestCtx) {
	_, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	id := httpctx.RouteParam(ctx, "alertId")
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	alert, err := h.alerts.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	if !auth.CanAccessClusterOrAccount(storeUser, alert.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, alert)
}

func (h *AlertsHandler) Acknowledge(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	id := httpctx.RouteParam(ctx, "alertId")
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	existing, err := h.alerts.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	if !auth.CanAccessClusterOrAccount(storeUser, existing.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	alert, err := h.alerts.Acknowledge(httpctx.FromRequest(ctx), id, actor.Username)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, alert)
}

func (h *AlertsHandler) ListRules(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	clusterID := commonstrings.BytesToString(ctx.QueryArgs().Peek("clusterId"))
	if !commonstrings.IsEmpty(clusterID) {
		if err := validateUUID(clusterID); err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
	}
	rules, err := h.alerts.ListRules(httpctx.FromRequest(ctx), clusterID, false)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, nonNilSlice(rules), totalMeta(len(rules)))
}

func (h *AlertsHandler) Metrics(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, map[string]any{
		"metrics":     append([]string(nil), domain.DefaultDashboardMetrics...),
		"comparators": append([]string(nil), domain.AlertComparators...),
		"severities":  append([]string(nil), domain.AlertSeverities...),
	})
}

func (h *AlertsHandler) GetRule(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	id := httpctx.RouteParam(ctx, "ruleId")
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	rule, err := h.alerts.GetRule(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, rule)
}

func (h *AlertsHandler) CreateRule(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	var req domain.AlertRuleCreate
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if commonstrings.IsEmpty(req.Severity) {
		req.Severity = domain.AlertSeverityWarning
	}
	if commonstrings.IsEmpty(req.Comparator) {
		req.Comparator = domain.AlertComparatorGTE
	}
	if err := validateAlertRuleCreate(req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if !commonstrings.IsEmpty(req.ClusterID) && !auth.CanAccessCluster(storeUser, req.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	rule, err := h.alerts.CreateRule(httpctx.FromRequest(ctx), req, actor.Username)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, rule)
}

func (h *AlertsHandler) UpdateRule(ctx *fasthttp.RequestCtx) {
	_, storeUser, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	id := httpctx.RouteParam(ctx, "ruleId")
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	var req domain.AlertRuleUpdate
	if err := serializer.Unmarshal(ctx.PostBody(), &req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateAlertRuleUpdate(req); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.ClusterID != nil && !commonstrings.IsEmpty(*req.ClusterID) && !auth.CanAccessCluster(storeUser, *req.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	rule, err := h.alerts.UpdateRule(httpctx.FromRequest(ctx), id, req)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, rule)
}

func (h *AlertsHandler) DeleteRule(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	id := httpctx.RouteParam(ctx, "ruleId")
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.alerts.DeleteRule(httpctx.FromRequest(ctx), id); err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func requireManageAlertRules(ctx *fasthttp.RequestCtx) (domain.User, store.User, bool) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return domain.User{}, store.User{}, false
	}
	if !auth.CanManageAlertRules(storeUser) {
		httpstatus.WriteForbidden(ctx)
		return domain.User{}, store.User{}, false
	}
	return actor, storeUser, true
}

func actorStoreFromContext(ctx *fasthttp.RequestCtx) (domain.User, store.User, bool) {
	c := httpctx.FromRequest(ctx)
	user, ok := auth.UserFromContext(c)
	if !ok {
		return domain.User{}, store.User{}, false
	}
	return auth.StoreUserToDomain(user), user, true
}

// accessibleClusterIDs returns nil when the actor can see all clusters; otherwise the scoped list.
func accessibleClusterIDs(actor domain.User, storeUser store.User) []string {
	perms := domain.PermissionsFor(actor)
	if perms.IsRoot || perms.AllClusters {
		return nil
	}
	ids := append([]string(nil), perms.ClusterIDs...)
	for _, g := range storeUser.Grants {
		id := g.ResourceKey
		if i := strings.IndexByte(id, ':'); i >= 0 {
			id = id[:i]
		}
		if commonstrings.IsEmpty(id) {
			continue
		}
		found := slices.Contains(ids, id)
		if !found {
			ids = append(ids, id)
		}
	}
	return ids
}

func validateAlertRuleCreate(req domain.AlertRuleCreate) error {
	if commonstrings.IsEmpty(strings.TrimSpace(req.Name)) {
		return errors.New("name is required")
	}
	if !domain.ValidMetricName(req.Metric) {
		return errors.New("invalid metric")
	}
	if !domain.ValidAlertComparator(req.Comparator) {
		return errors.New("invalid comparator")
	}
	if !domain.ValidAlertSeverity(req.Severity) {
		return errors.New("invalid severity")
	}
	if !commonstrings.IsEmpty(req.ClusterID) {
		if err := validateUUID(req.ClusterID); err != nil {
			return err
		}
	}
	return nil
}

func validateAlertRuleUpdate(req domain.AlertRuleUpdate) error {
	if req.Metric != nil && !domain.ValidMetricName(*req.Metric) {
		return errors.New("invalid metric")
	}
	if req.Comparator != nil && !domain.ValidAlertComparator(*req.Comparator) {
		return errors.New("invalid comparator")
	}
	if req.Severity != nil && !domain.ValidAlertSeverity(*req.Severity) {
		return errors.New("invalid severity")
	}
	if req.ClusterID != nil && !commonstrings.IsEmpty(*req.ClusterID) {
		if err := validateUUID(*req.ClusterID); err != nil {
			return err
		}
	}
	if req.Name != nil && commonstrings.IsEmpty(strings.TrimSpace(*req.Name)) {
		return errors.New("name is required")
	}
	return nil
}

func queryInt(ctx *fasthttp.RequestCtx, key string, fallback int) int {
	raw := commonstrings.BytesToString(ctx.QueryArgs().Peek(key))
	if commonstrings.IsEmpty(raw) {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
