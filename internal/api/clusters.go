package api

import (
	"errors"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/internal/store"
	"github.com/valyala/fasthttp"
)

type clusterCreateRequest struct {
	Name          string `json:"name"`
	NATSURL       string `json:"nats_url"`
	MonitoringURL string `json:"monitoring_url"`
	CredsFilePath string `json:"creds_file_path"`
	Token         string `json:"token"`
	IsDefault     bool   `json:"is_default"`
}

type clusterUpdateRequest struct {
	Name          *string `json:"name"`
	NATSURL       *string `json:"nats_url"`
	MonitoringURL *string `json:"monitoring_url"`
	CredsFilePath *string `json:"creds_file_path"`
	Token         *string `json:"token"`
	IsDefault     *bool   `json:"is_default"`
}

func (h *Handler) ListClusters(ctx *fasthttp.RequestCtx) {
	clusters, err := h.store.ListClusters(requestContext(ctx))
	if err != nil {
		writeError(ctx, fasthttp.StatusInternalServerError, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"clusters": clusters,
		"total":    len(clusters),
	})
}

func (h *Handler) GetCluster(ctx *fasthttp.RequestCtx) {
	cluster, err := h.store.GetCluster(requestContext(ctx), clusterID(ctx))
	if err != nil {
		writeStoreError(ctx, err)
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, cluster)
}

func (h *Handler) CreateCluster(ctx *fasthttp.RequestCtx) {
	var req clusterCreateRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("name"))
		return
	}
	if req.NATSURL == "" {
		writeError(ctx, fasthttp.StatusBadRequest, errMissing("nats_url"))
		return
	}

	cluster, err := h.store.CreateCluster(requestContext(ctx), store.ClusterCreate{
		Name:          req.Name,
		NATSURL:       req.NATSURL,
		MonitoringURL: req.MonitoringURL,
		CredsFilePath: req.CredsFilePath,
		Token:         req.Token,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}
	h.nats.Evict(cluster.ID)
	writeJSON(ctx, fasthttp.StatusCreated, cluster)
}

func (h *Handler) UpdateCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	var req clusterUpdateRequest
	if err := sonic.Unmarshal(ctx.PostBody(), &req); err != nil {
		writeError(ctx, fasthttp.StatusBadRequest, err)
		return
	}

	cluster, err := h.store.UpdateCluster(requestContext(ctx), id, store.ClusterUpdate{
		Name:          req.Name,
		NATSURL:       req.NATSURL,
		MonitoringURL: req.MonitoringURL,
		CredsFilePath: req.CredsFilePath,
		Token:         req.Token,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		writeStoreError(ctx, err)
		return
	}
	h.nats.Evict(id)
	writeJSON(ctx, fasthttp.StatusOK, cluster)
}

func (h *Handler) DeleteCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	if err := h.store.DeleteCluster(requestContext(ctx), id); err != nil {
		writeStoreError(ctx, err)
		return
	}
	h.nats.Evict(id)
	ctx.SetStatusCode(fasthttp.StatusNoContent)
}

func (h *Handler) TestCluster(ctx *fasthttp.RequestCtx) {
	id := clusterID(ctx)
	serverName, jetstream, err := h.nats.Test(requestContext(ctx), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeStoreError(ctx, err)
			return
		}
		writeJSON(ctx, fasthttp.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"ok":          true,
		"message":     "connected",
		"server_name": serverName,
		"jetstream":   jetstream,
	})
}

func writeStoreError(ctx *fasthttp.RequestCtx, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(ctx, fasthttp.StatusNotFound, err)
		return
	}
	writeError(ctx, fasthttp.StatusInternalServerError, err)
}

func clusterID(ctx *fasthttp.RequestCtx) string {
	return param(ctx, "clusterId")
}
