package ops

import (
	"errors"
	"net/url"
	"strings"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/api/apikit"
	"github.com/gopherust-io/nats-consol/internal/app/policy"
	"github.com/gopherust-io/nats-consol/internal/auth"
	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

// ListClusters godoc
//
// @Summary List Clusters
// @Tags Clusters
// @Produce json
// @Success 200 {object} api.ClusterListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters [get]
func (h *Handler) ListClusters(ctx *fasthttp.RequestCtx) {
	clusters, err := h.Svc.Cluster.List(httpctx.FromRequest(ctx))
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if actor, ok := apikit.ActorFromContext(ctx); ok {
		clusters = apikit.FilterClustersForActor(clusters, actor)
	}
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		for i := range clusters {
			clusters[i] = redactClusterURLs(clusters[i], user)
		}
	}
	clusters = apikit.NonNilSlice(clusters)
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, clusters, apikit.TotalMeta(len(clusters)))
}

// GetCluster godoc
//
// @Summary Get Cluster
// @Tags Clusters
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ClusterEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId} [get]
func (h *Handler) GetCluster(ctx *fasthttp.RequestCtx) {
	id := apikit.ClusterID(ctx)
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cluster, err := h.Svc.Cluster.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	if user, ok := auth.UserFromContext(httpctx.FromRequest(ctx)); ok {
		cluster = redactClusterURLs(cluster, user)
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, cluster)
}

// redactClusterURLs strips URL userinfo always, and hides nats/monitoring URLs
// from principals without system-level cluster access (account-scoped readers).
func redactClusterURLs(cluster domain.Cluster, user domain.User) domain.Cluster {
	cluster.NATSURL = redactURLUserinfo(cluster.NATSURL)
	cluster.MonitoringURL = redactURLUserinfo(cluster.MonitoringURL)
	if !policy.AuthorizeAccessCluster(user, cluster.ID) {
		cluster.NATSURL = ""
		cluster.MonitoringURL = ""
	}
	return cluster
}

func redactURLUserinfo(raw string) string {
	if commonstrings.IsEmpty(raw) {
		return raw
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if commonstrings.IsEmpty(part) {
			continue
		}
		u, err := url.Parse(part)
		if err != nil || u.User == nil {
			out = append(out, part)
			continue
		}
		u.User = nil
		out = append(out, u.String())
	}
	return strings.Join(out, ",")
}

// TestCluster godoc
//
// @Summary Test Cluster
// @Tags API
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ClusterTestEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/test [post]
func (h *Handler) TestCluster(ctx *fasthttp.RequestCtx) {
	id := apikit.ClusterID(ctx)
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	result, err := h.Svc.Cluster.Test(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, result)
}

// GetClusterConnection godoc
//
// @Summary Get Cluster Connection
// @Tags API
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ConnectionStatusEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId}/connection [get]
func (h *Handler) GetClusterConnection(ctx *fasthttp.RequestCtx) {
	id := apikit.ClusterID(ctx)
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	status, err := h.Svc.Cluster.ConnectionStatus(httpctx.FromRequest(ctx), id)
	if err != nil {
		apikit.WriteAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, status)
}

// ListClusterConnections godoc
//
// @Summary List Cluster Connections
// @Tags Clusters
// @Produce json
// @Success 200 {object} api.ConnectionStatusListEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/connections [get]
func (h *Handler) ListClusterConnections(ctx *fasthttp.RequestCtx) {
	statuses := h.Svc.Cluster.ListConnectionStatuses(httpctx.FromRequest(ctx))
	if actor, ok := apikit.ActorFromContext(ctx); ok {
		statuses = apikit.FilterConnectionStatusesForActor(statuses, actor)
	}
	statuses = apikit.NonNilSlice(statuses)
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, statuses, apikit.TotalMeta(len(statuses)))
}

// errClusterCreateDisabled is returned for POST /api/v1/clusters — registrations
// are devops-managed (env bootstrap / ops tooling), not created via the console API.
var errClusterCreateDisabled = errors.New("cluster registration is devops-managed")

// errClusterUpdateDisabled is returned for PUT /api/v1/clusters/{id} — cluster
// configuration is devops-managed, not edited via the console API.
var errClusterUpdateDisabled = errors.New("cluster configuration is devops-managed")

// CreateClusterDisabled godoc
//
// @Summary Create Cluster Disabled
// @Tags Clusters
// @Produce json
// @Success 200 {object} api.ErrorEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters [post]
func (h *Handler) CreateClusterDisabled(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteError(ctx, fasthttp.StatusMethodNotAllowed, errClusterCreateDisabled)
}

// UpdateClusterDisabled godoc
//
// @Summary Update Cluster Disabled
// @Tags Clusters
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} api.ErrorEnvelope
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId} [put]
func (h *Handler) UpdateClusterDisabled(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteError(ctx, fasthttp.StatusMethodNotAllowed, errClusterUpdateDisabled)
}

// DeleteCluster godoc
//
// @Summary Delete Cluster
// @Tags Clusters
// @Param clusterId path string true "clusterId"
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} api.ErrorEnvelope
// @Failure 403 {object} api.ErrorEnvelope
// @Failure 404 {object} api.ErrorEnvelope
// @Security BasicAuth
// @Security BearerAuth
// @Security SessionCookie
// @Router /api/v1/clusters/{clusterId} [delete]
func (h *Handler) DeleteCluster(ctx *fasthttp.RequestCtx) {
	actor, ok := apikit.ActorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	perms := domain.PermissionsFor(actor)
	if !perms.DeleteClusters {
		httpstatus.WriteError(ctx, fasthttp.StatusForbidden, domain.ErrForbidden)
		return
	}
	id := apikit.ClusterID(ctx)
	if err := apikit.ValidateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.Svc.Cluster.Delete(httpctx.FromRequest(ctx), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}
