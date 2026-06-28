package api

import (
	"errors"

	"github.com/gopherust-io/nats-consol/internal/domain"
	"github.com/gopherust-io/nats-consol/pkg/common/serializer"

	"github.com/valyala/fasthttp"
)

type clusterCreateRequest struct {
	Name          string `json:"name"`
	NATSURL       string `json:"natsUrl"`
	MonitoringURL string `json:"monitoringUrl"`
	CredsFilePath string `json:"credsFilePath"`
	Token         string `json:"token"`
	IsDefault     bool   `json:"isDefault"`
}

type clusterUpdateRequest struct {
	Name          *string `json:"name"`
	NATSURL       *string `json:"natsUrl"`
	MonitoringURL *string `json:"monitoringUrl"`
	CredsFilePath *string `json:"credsFilePath"`
	Token         *string `json:"token"`
	IsDefault     *bool   `json:"isDefault"`
}

func (h *Handler) ListClusters(ctx *fasthttp.RequestCtx) {
	clusters, err := h.svc.Cluster.List(requestContext(ctx))
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
	cluster, err := h.svc.Cluster.Get(requestContext(ctx), id)
	if err != nil {
		writeDomainError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, cluster)
}

func (h *Handler) CreateCluster(ctx *fasthttp.RequestCtx) {
	var req clusterCreateRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	if err := validateClusterName(req.Name); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateNATSURL(req.NATSURL); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := validateHTTPURL(req.MonitoringURL); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	cluster, err := h.svc.Cluster.Create(requestContext(ctx), domain.ClusterCreate{
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
	var req clusterUpdateRequest
	if err := parseJSONBody(ctx, &req); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name != nil {
		if err := validateClusterName(*req.Name); err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
	}
	if req.NATSURL != nil {
		if err := validateNATSURL(*req.NATSURL); err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
	}
	if req.MonitoringURL != nil {
		if err := validateHTTPURL(*req.MonitoringURL); err != nil {
			serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
			return
		}
	}

	cluster, err := h.svc.Cluster.Update(requestContext(ctx), id, domain.ClusterUpdate{
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
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if err := h.svc.Cluster.Delete(requestContext(ctx), id); err != nil {
		writeDomainError(ctx, err)
		return
	}
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *Handler) TestCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := validateUUID(id); err != nil {
		serializer.WriteError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	result, err := h.svc.Cluster.Test(requestContext(ctx), id)
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
	status, err := h.svc.Cluster.ConnectionStatus(requestContext(ctx), id)
	if err != nil {
		writeDomainError(ctx, err)
		return
	}
	serializer.WriteJSON(ctx, fasthttp.StatusOK, status)
}

func (h *Handler) ListClusterConnections(ctx *fasthttp.RequestCtx) {
	statuses := h.svc.Cluster.ListConnectionStatuses(requestContext(ctx))
	if actor, ok := actorFromContext(ctx); ok {
		statuses = filterConnectionStatusesForActor(statuses, actor)
	}
	statuses = nonNilSlice(statuses)
	serializer.WriteJSON(ctx, fasthttp.StatusOK, ConnectionsListResponse{
		Connections: statuses,
		Total:       len(statuses),
	})
}

func writeDomainError(ctx *fasthttp.RequestCtx, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		serializer.WriteError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	serializer.WriteError(ctx, fasthttp.StatusInternalServerError, err)
}

func clusterID(ctx *fasthttp.RequestCtx) string {
	return routeParam(ctx, "clusterId")
}
