package api

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"
)

type AlertsHandler struct {
	store *store.Store
	cfg   config.Config
}

func NewAlertsHandler(st *store.Store, cfg config.Config) *AlertsHandler {
	return &AlertsHandler{store: st, cfg: cfg}
}

func (h *AlertsHandler) List(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}

	filter := domain.AlertFilter{
		Status:    string(ctx.QueryArgs().Peek("status")),
		ClusterID: string(ctx.QueryArgs().Peek("clusterId")),
		Severity:  string(ctx.QueryArgs().Peek("severity")),
		Limit:     queryInt(ctx, "limit", h.cfg.AuditDefaultLimit),
		Offset:    queryInt(ctx, "offset", 0),
	}
	if filter.Status != "" && filter.Status != domain.AlertStatusOpen && filter.Status != domain.AlertStatusClosed {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("invalid status"))
		return
	}
	if filter.Severity != "" && !domain.ValidAlertSeverity(filter.Severity) {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errors.New("invalid severity"))
		return
	}
	if filter.ClusterID != "" {
		if err := validateUUID(filter.ClusterID); err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		if !auth.CanAccessCluster(storeUser, filter.ClusterID) {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			return
		}
	} else if clusterIDs := accessibleClusterIDs(actor, storeUser); clusterIDs != nil {
		filter.ClusterIDs = clusterIDs
		if len(clusterIDs) == 0 {
			serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{"alerts": []domain.Alert{}, "total": 0})
			return
		}
	}

	alerts, total, err := h.store.ListAlerts(requestContext(ctx), filter)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"alerts": nonNilSlice(alerts),
		"total":  total,
	})
}

func (h *AlertsHandler) OpenSummary(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	var clusterIDs []string
	if ids := accessibleClusterIDs(actor, storeUser); ids != nil {
		clusterIDs = ids
		if len(clusterIDs) == 0 {
			serializer.WriteJSON(ctx, fasthttp.StatusOK, domain.AlertOpenSummary{Count: 0, Alerts: []domain.Alert{}})
			return
		}
	}
	alerts, total, err := h.store.ListOpenUnacknowledged(requestContext(ctx), clusterIDs, 8)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, domain.AlertOpenSummary{
		Count:  total,
		Alerts: nonNilSlice(alerts),
	})
}

func (h *AlertsHandler) Get(ctx *fasthttp.RequestCtx) {
	_, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	id := routeParam(ctx, "alertId")
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	alert, err := h.store.GetAlert(requestContext(ctx), id)
	if err != nil {
		writeAlertStoreError(ctx, err)
		return
	}
	if !auth.CanAccessCluster(storeUser, alert.ClusterID) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, alert)
}

func (h *AlertsHandler) Acknowledge(ctx *fasthttp.RequestCtx) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	id := routeParam(ctx, "alertId")
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	existing, err := h.store.GetAlert(requestContext(ctx), id)
	if err != nil {
		writeAlertStoreError(ctx, err)
		return
	}
	if !auth.CanAccessCluster(storeUser, existing.ClusterID) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return
	}
	alert, err := h.store.AcknowledgeAlert(requestContext(ctx), id, actor.Username)
	if err != nil {
		writeAlertStoreError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, alert)
}

func (h *AlertsHandler) ListRules(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	clusterID := string(ctx.QueryArgs().Peek("clusterId"))
	if clusterID != "" {
		if err := validateUUID(clusterID); err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
	}
	rules, err := h.store.ListAlertRules(requestContext(ctx), clusterID, false)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"rules": nonNilSlice(rules),
		"total": len(rules),
	})
}

func (h *AlertsHandler) Metrics(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, map[string]any{
		"metrics":     append([]string(nil), domain.DefaultDashboardMetrics...),
		"comparators": append([]string(nil), domain.AlertComparators...),
		"severities":  append([]string(nil), domain.AlertSeverities...),
	})
}

func (h *AlertsHandler) GetRule(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	id := routeParam(ctx, "ruleId")
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	rule, err := h.store.GetAlertRule(requestContext(ctx), id)
	if err != nil {
		writeAlertStoreError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, rule)
}

func (h *AlertsHandler) CreateRule(ctx *fasthttp.RequestCtx) {
	actor, _, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	var req domain.AlertRuleCreate
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Severity == "" {
		req.Severity = domain.AlertSeverityWarning
	}
	if req.Comparator == "" {
		req.Comparator = domain.AlertComparatorGTE
	}
	if err := validateAlertRuleCreate(req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	rule, err := h.store.CreateAlertRule(requestContext(ctx), req, actor.Username)
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, rule)
}

func (h *AlertsHandler) UpdateRule(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	id := routeParam(ctx, "ruleId")
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	var req domain.AlertRuleUpdate
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateAlertRuleUpdate(req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	rule, err := h.store.UpdateAlertRule(requestContext(ctx), id, req)
	if err != nil {
		writeAlertStoreError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, rule)
}

func (h *AlertsHandler) DeleteRule(ctx *fasthttp.RequestCtx) {
	if _, _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	id := routeParam(ctx, "ruleId")
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.store.DeleteAlertRule(requestContext(ctx), id); err != nil {
		writeAlertStoreError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func requireManageAlertRules(ctx *fasthttp.RequestCtx) (domain.User, store.User, bool) {
	actor, storeUser, ok := actorStoreFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return domain.User{}, store.User{}, false
	}
	if !auth.CanManageAlertRules(storeUser) {
		ctx.SetStatusCode(fasthttp.StatusForbidden)
		return domain.User{}, store.User{}, false
	}
	return actor, storeUser, true
}

func actorStoreFromContext(ctx *fasthttp.RequestCtx) (domain.User, store.User, bool) {
	c := requestContext(ctx)
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
		if id == "" {
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
	if strings.TrimSpace(req.Name) == "" {
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
	if req.ClusterID != "" {
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
	if req.ClusterID != nil && *req.ClusterID != "" {
		if err := validateUUID(*req.ClusterID); err != nil {
			return err
		}
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func writeAlertStoreError(ctx *fasthttp.RequestCtx, err error) {
	switch {
	case errors.Is(err, store.ErrAlertNotFound), errors.Is(err, store.ErrAlertRuleNotFound):
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
	default:
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
	}
}

func queryInt(ctx *fasthttp.RequestCtx, key string, fallback int) int {
	raw := string(ctx.QueryArgs().Peek(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
