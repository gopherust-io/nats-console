package api

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/internal/httpctx/httpstatus"
)

func (h *Handler) ListClusters(ctx *fasthttp.RequestCtx) {
	clusters, err := h.svc.Cluster.List(httpctx.FromRequest(ctx))
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	if actor, ok := actorFromContext(ctx); ok {
		clusters = filterClustersForActor(clusters, actor)
	}
	clusters = nonNilSlice(clusters)
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, clusters, totalMeta(len(clusters)))
}

func (h *Handler) GetCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cluster, err := h.svc.Cluster.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, cluster)
}

func (h *Handler) TestCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	result, err := h.svc.Cluster.Test(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, result)
}

func (h *Handler) GetClusterConnection(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	status, err := h.svc.Cluster.ConnectionStatus(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	httpstatus.WriteData(ctx, fasthttp.StatusOK, status)
}

func (h *Handler) ListClusterConnections(ctx *fasthttp.RequestCtx) {
	statuses := h.svc.Cluster.ListConnectionStatuses(httpctx.FromRequest(ctx))
	if actor, ok := actorFromContext(ctx); ok {
		statuses = filterConnectionStatusesForActor(statuses, actor)
	}
	statuses = nonNilSlice(statuses)
	httpstatus.WriteDataMeta(ctx, fasthttp.StatusOK, statuses, totalMeta(len(statuses)))
}

// errClusterCreateDisabled is returned for POST /api/v1/clusters — registrations
// are devops-managed (env bootstrap / ops tooling), not created via the console API.
var errClusterCreateDisabled = errors.New("cluster registration is devops-managed")

// errClusterUpdateDisabled is returned for PUT /api/v1/clusters/{id} — cluster
// configuration is devops-managed, not edited via the console API.
var errClusterUpdateDisabled = errors.New("cluster configuration is devops-managed")

func (h *Handler) CreateClusterDisabled(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteError(ctx, fasthttp.StatusMethodNotAllowed, errClusterCreateDisabled)
}

func (h *Handler) UpdateClusterDisabled(ctx *fasthttp.RequestCtx) {
	httpstatus.WriteError(ctx, fasthttp.StatusMethodNotAllowed, errClusterUpdateDisabled)
}

func (h *Handler) DeleteCluster(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		httpstatus.WriteUnauthorized(ctx)
		return
	}
	perms := domain.PermissionsFor(actor)
	if !perms.DeleteClusters {
		httpstatus.WriteError(ctx, fasthttp.StatusForbidden, domain.ErrForbidden)
		return
	}
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.svc.Cluster.Delete(httpctx.FromRequest(ctx), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpstatus.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		httpstatus.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func clusterID(ctx *fasthttp.RequestCtx) string {
	return httpctx.RouteParam(ctx, "clusterId")
}
