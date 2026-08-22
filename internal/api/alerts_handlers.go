package api

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app"
	"github.com/gopherust-io/nats-consol/internal/app/policy"
	"github.com/gopherust-io/nats-consol/internal/config"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
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

// List godoc
//
// @Summary List
// @Tags Alerts
// @Produce json
// @Success 200 {object} AlertListEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alerts [get]
func (h *AlertsHandler) List(ctx *fasthttp.RequestCtx) {
	actor, ok := apikit.ActorFromContext(ctx)
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
		if err := apikit.ValidateUUID(filter.ClusterID); err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		if !policy.AuthorizeAccessClusterOrAccount(actor, filter.ClusterID) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	} else if clusterIDs := accessibleClusterIDs(actor); clusterIDs != nil {
		filter.ClusterIDs = clusterIDs
		if len(clusterIDs) == 0 {
			httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, []domain.Alert{}, apikit.TotalMeta(0))
			return
		}
	}

	alerts, total, err := h.alerts.List(httpctx.FromRequest(ctx), filter)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, apikit.NonNilSlice(alerts), apikit.TotalMeta(total))
}

// OpenSummary godoc
//
// @Summary Open Summary
// @Tags Alerts
// @Produce json
// @Success 200 {object} AlertOpenSummaryEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alerts/open-summary [get]
func (h *AlertsHandler) OpenSummary(ctx *fasthttp.RequestCtx) {
	actor, ok := apikit.ActorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	var clusterIDs []string
	if ids := accessibleClusterIDs(actor); ids != nil {
		clusterIDs = ids
		if len(clusterIDs) == 0 {
			httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.AlertOpenSummary{Count: 0, Alerts: []domain.Alert{}})
			return
		}
	}
	alerts, total, err := h.alerts.OpenUnacknowledged(httpctx.FromRequest(ctx), clusterIDs, 8)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, domain.AlertOpenSummary{
		Count:  total,
		Alerts: apikit.NonNilSlice(alerts),
	})
}

// Get godoc
//
// @Summary Get
// @Tags Alerts
// @Param alertId path string true "alertId"
// @Produce json
// @Success 200 {object} AlertEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alerts/{alertId} [get]
func (h *AlertsHandler) Get(ctx *fasthttp.RequestCtx) {
	actor, ok := apikit.ActorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	id := httpctx.RouteParam(ctx, "alertId")
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	alert, err := h.alerts.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if !policy.AuthorizeAccessClusterOrAccount(actor, alert.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, alert)
}

// Acknowledge godoc
//
// @Summary Acknowledge
// @Tags Alerts
// @Param alertId path string true "alertId"
// @Produce json
// @Success 200 {object} AlertEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alerts/{alertId}/acknowledge [post]
func (h *AlertsHandler) Acknowledge(ctx *fasthttp.RequestCtx) {
	actor, ok := apikit.ActorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	id := httpctx.RouteParam(ctx, "alertId")
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	existing, err := h.alerts.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if !policy.AuthorizeAccessClusterOrAccount(actor, existing.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	alert, err := h.alerts.Acknowledge(httpctx.FromRequest(ctx), id, actor.Username)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, alert)
}

// ListRules godoc
//
// @Summary List Rules
// @Tags Alerts
// @Produce json
// @Success 200 {object} AlertRuleListEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alert-rules [get]
func (h *AlertsHandler) ListRules(ctx *fasthttp.RequestCtx) {
	actor, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	clusterID := commonstrings.BytesToString(ctx.QueryArgs().Peek("clusterId"))
	if !commonstrings.IsEmpty(clusterID) {
		if err := apikit.ValidateUUID(clusterID); err != nil {
			httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
		if !policy.AuthorizeAccessCluster(actor, clusterID) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	}
	rules, err := h.alerts.ListRules(httpctx.FromRequest(ctx), clusterID, false)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	rules = filterAlertRulesForActor(rules, actor)
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, apikit.NonNilSlice(rules), apikit.TotalMeta(len(rules)))
}

// Metrics godoc
//
// @Summary Metrics
// @Tags Alerts
// @Produce json
// @Success 200 {object} DataMetaEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alert-rules/metrics [get]
func (h *AlertsHandler) Metrics(ctx *fasthttp.RequestCtx) {
	if _, ok := requireManageAlertRules(ctx); !ok {
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, map[string]any{
		"metrics":     append([]string(nil), domain.DefaultDashboardMetrics...),
		"comparators": append([]string(nil), domain.AlertComparators...),
		"severities":  append([]string(nil), domain.AlertSeverities...),
	})
}

// GetRule godoc
//
// @Summary Get Rule
// @Tags Alerts
// @Param ruleId path string true "ruleId"
// @Produce json
// @Success 200 {object} AlertRuleEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alert-rules/{ruleId} [get]
func (h *AlertsHandler) GetRule(ctx *fasthttp.RequestCtx) {
	actor, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	id := httpctx.RouteParam(ctx, "ruleId")
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	rule, err := h.alerts.GetRule(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if !canAccessAlertRuleCluster(actor, rule.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, rule)
}

// CreateRule godoc
//
// @Summary Create Rule
// @Tags Alerts
// @Produce json
// @Success 201 {object} AlertRuleEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alert-rules [post]
func (h *AlertsHandler) CreateRule(ctx *fasthttp.RequestCtx) {
	actor, ok := requireManageAlertRules(ctx)
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
	if !canAccessAlertRuleCluster(actor, req.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	rule, err := h.alerts.CreateRule(httpctx.FromRequest(ctx), req, actor.Username)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusCreated, rule)
}

// UpdateRule godoc
//
// @Summary Update Rule
// @Tags Alerts
// @Param ruleId path string true "ruleId"
// @Produce json
// @Success 200 {object} AlertRuleEnvelope
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alert-rules/{ruleId} [patch]
func (h *AlertsHandler) UpdateRule(ctx *fasthttp.RequestCtx) {
	actor, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	id := httpctx.RouteParam(ctx, "ruleId")
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	existing, err := h.alerts.GetRule(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if !canAccessAlertRuleCluster(actor, existing.ClusterID) {
		httpstatus.WriteForbidden(ctx)
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
	// Scoped actors cannot promote a rule to global (clearCluster / empty clusterId).
	if req.ClearCluster || (req.ClusterID != nil && commonstrings.IsEmpty(*req.ClusterID)) {
		if !canManageGlobalAlertRules(actor) {
			httpstatus.WriteForbidden(ctx)
			return
		}
	} else if req.ClusterID != nil && !policy.AuthorizeAccessCluster(actor, *req.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	rule, err := h.alerts.UpdateRule(httpctx.FromRequest(ctx), id, req)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, rule)
}

// DeleteRule godoc
//
// @Summary Delete Rule
// @Tags Alerts
// @Param ruleId path string true "ruleId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} ErrorEnvelope
// @Failure 403 {object} ErrorEnvelope
// @Failure 404 {object} ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/alert-rules/{ruleId} [delete]
func (h *AlertsHandler) DeleteRule(ctx *fasthttp.RequestCtx) {
	actor, ok := requireManageAlertRules(ctx)
	if !ok {
		return
	}
	id := httpctx.RouteParam(ctx, "ruleId")
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	existing, err := h.alerts.GetRule(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if !canAccessAlertRuleCluster(actor, existing.ClusterID) {
		httpstatus.WriteForbidden(ctx)
		return
	}
	if err := h.alerts.DeleteRule(httpctx.FromRequest(ctx), id); err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func requireManageAlertRules(ctx *fasthttp.RequestCtx) (domain.User, bool) {
	actor, ok := apikit.ActorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return domain.User{}, false
	}
	if !policy.AuthorizeManageAlertRules(actor) {
		httpstatus.WriteForbidden(ctx)
		return domain.User{}, false
	}
	return actor, true
}

func filterAlertRulesForActor(rules []domain.AlertRule, actor domain.User) []domain.AlertRule {
	if canManageGlobalAlertRules(actor) {
		return rules
	}
	out := make([]domain.AlertRule, 0, len(rules))
	for _, rule := range rules {
		if canAccessAlertRuleCluster(actor, rule.ClusterID) {
			out = append(out, rule)
		}
	}
	return out
}

// canManageGlobalAlertRules is true for root / unscoped admins (all clusters).
func canManageGlobalAlertRules(actor domain.User) bool {
	return accessibleClusterIDs(actor) == nil
}

// canAccessAlertRuleCluster allows empty clusterId (global) only for unscoped actors.
func canAccessAlertRuleCluster(actor domain.User, clusterID string) bool {
	if commonstrings.IsEmpty(clusterID) {
		return canManageGlobalAlertRules(actor)
	}
	return policy.AuthorizeAccessCluster(actor, clusterID)
}

// accessibleClusterIDs returns nil when the actor can see all clusters; otherwise the scoped list.
func accessibleClusterIDs(actor domain.User) []string {
	perms := domain.PermissionsFor(actor)
	if perms.IsRoot || perms.AllClusters {
		return nil
	}
	ids := append([]string(nil), perms.ClusterIDs...)
	for _, g := range actor.Grants {
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
		if err := apikit.ValidateUUID(req.ClusterID); err != nil {
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
		if err := apikit.ValidateUUID(*req.ClusterID); err != nil {
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
