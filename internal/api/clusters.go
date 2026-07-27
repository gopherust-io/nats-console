package api

import (
	"errors"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/internal/httpctx"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"

	"github.com/valyala/fasthttp"
)

func (h *Handler) ListClusters(ctx *fasthttp.RequestCtx) {
	clusters, err := h.svc.Cluster.List(httpctx.FromRequest(ctx))
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	if actor, ok := actorFromContext(ctx); ok {
		clusters = filterClustersForActor(clusters, actor)
	}
	clusters = nonNilSlice(clusters)
	serializer.WriteJSON(ctx, fasthttp.StatusOK, ClustersListResponse{
		Clusters: clusters,
		Total:    len(clusters),
	})
}

func (h *Handler) GetCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cluster, err := h.svc.Cluster.Get(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeDomainError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, cluster)
}

func (h *Handler) TestCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	result, err := h.svc.Cluster.Test(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeDomainError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, result)
}

func (h *Handler) GetClusterConnection(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	status, err := h.svc.Cluster.ConnectionStatus(httpctx.FromRequest(ctx), id)
	if err != nil {
		writeDomainError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, status)
}

func (h *Handler) ListClusterConnections(ctx *fasthttp.RequestCtx) {
	statuses := h.svc.Cluster.ListConnectionStatuses(httpctx.FromRequest(ctx))
	if actor, ok := actorFromContext(ctx); ok {
		statuses = filterConnectionStatusesForActor(statuses, actor)
	}
	statuses = nonNilSlice(statuses)
	serializer.WriteJSON(ctx, fasthttp.StatusOK, ConnectionsListResponse{
		Connections: statuses,
		Total:       len(statuses),
	})
}

func (h *Handler) CreateCluster(ctx *fasthttp.RequestCtx) {
	var req struct {
		Name          string `json:"name"`
		NATSURL       string `json:"natsUrl"`
		MonitoringURL string `json:"monitoringUrl"`
		CredsFilePath string `json:"credsFilePath"`
		Token         string `json:"token"`
		IsDefault     bool   `json:"isDefault"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cluster, err := h.svc.Cluster.Create(httpctx.FromRequest(ctx), domain.ClusterCreate{
		Name:          req.Name,
		NATSURL:       req.NATSURL,
		MonitoringURL: req.MonitoringURL,
		CredsFilePath: req.CredsFilePath,
		Token:         req.Token,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusCreated, cluster)
}

func (h *Handler) UpdateCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	var req struct {
		Name          *string `json:"name"`
		NATSURL       *string `json:"natsUrl"`
		MonitoringURL *string `json:"monitoringUrl"`
		CredsFilePath *string `json:"credsFilePath"`
		Token         *string `json:"token"`
		IsDefault     *bool   `json:"isDefault"`
	}
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	cluster, err := h.svc.Cluster.Update(httpctx.FromRequest(ctx), id, domain.ClusterUpdate{
		Name:          req.Name,
		NATSURL:       req.NATSURL,
		MonitoringURL: req.MonitoringURL,
		CredsFilePath: req.CredsFilePath,
		Token:         req.Token,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		writeDomainError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, cluster)
}

func (h *Handler) DeleteCluster(ctx *fasthttp.RequestCtx) {
	actor, ok := actorFromContext(ctx)
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}
	perms := domain.PermissionsFor(actor)
	if !perms.DeleteClusters {
		serializer.WriteError(ctx, fasthttp.StatusForbidden, domain.ErrForbidden)
		return
	}
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.svc.Cluster.Delete(httpctx.FromRequest(ctx), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
			return
		}
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func writeDomainError(ctx *fasthttp.RequestCtx, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
}

func clusterID(ctx *fasthttp.RequestCtx) string {
	return httpctx.RouteParam(ctx, "clusterId")
}
