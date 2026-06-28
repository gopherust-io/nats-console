package api

import (
	"strings"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

var staticClusterPathSegments = map[string]struct{}{
	"connections": {},
}

func clusterIDFromPath(path string) string {
	const prefix = "/api/v1/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return ""
	}
	clusterID, _, _ := strings.Cut(rest, "/")
	if clusterID == "" {
		return ""
	}
	if _, ok := staticClusterPathSegments[clusterID]; ok {
		return ""
	}
	if !uuidPattern.MatchString(clusterID) {
		return ""
	}
	return clusterID
}

func filterClustersForActor(clusters []domain.Cluster, actor domain.User) []domain.Cluster {
	perms := domain.PermissionsFor(actor)
	if !hasClusterScope(perms) {
		return clusters
	}
	out := make([]domain.Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		if perms.AllowsCluster(cluster.ID) {
			out = append(out, cluster)
		}
	}
	return out
}

func filterConnectionStatusesForActor(statuses []domain.NATSConnectionStatus, actor domain.User) []domain.NATSConnectionStatus {
	perms := domain.PermissionsFor(actor)
	if !hasClusterScope(perms) {
		return statuses
	}
	out := make([]domain.NATSConnectionStatus, 0, len(statuses))
	for _, status := range statuses {
		if perms.AllowsCluster(status.ClusterID) {
			out = append(out, status)
		}
	}
	return out
}

func auditFilterForActor(actor domain.User, clusterID string) (domain.AuditFilter, error) {
	perms := domain.PermissionsFor(actor)
	filter := domain.AuditFilter{}
	if clusterID != "" {
		if !perms.AllowsCluster(clusterID) {
			return filter, domain.ErrForbidden
		}
		filter.ClusterID = clusterID
		return filter, nil
	}
	if hasClusterScope(perms) {
		filter.ClusterIDs = append([]string(nil), perms.ClusterIDs...)
	}
	return filter, nil
}

func hasClusterScope(perms domain.Permissions) bool {
	return !perms.IsRoot && len(perms.ClusterIDs) > 0
}
