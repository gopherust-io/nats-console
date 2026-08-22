package ops

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func TestRedactClusterURLsHidesFromAccountScoped(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	cluster := domain.Cluster{
		ID:            clusterID,
		NATSURL:       "nats://user:secret@nats:4222",
		MonitoringURL: "http://user:pass@nats:8222",
	}
	accountUser := domain.User{
		Roles: []string{domain.RoleViewer},
		Grants: []domain.AccessGrant{{
			ResourceType: domain.ResourceAccount,
			ResourceKey:  clusterID + ":Default",
			Role:         domain.GrantObserver,
		}},
	}

	got := redactClusterURLs(cluster, accountUser)
	assert.Empty(t, got.NATSURL)
	assert.Empty(t, got.MonitoringURL)
}

func TestRedactClusterURLsKeepsSystemAccessStripsUserinfo(t *testing.T) {
	t.Parallel()

	clusterID := "550e8400-e29b-41d4-a716-446655440000"
	cluster := domain.Cluster{
		ID:            clusterID,
		NATSURL:       "nats://user:secret@nats:4222",
		MonitoringURL: "http://user:pass@nats:8222",
	}
	admin := domain.User{
		Roles: []string{domain.RoleAdmin},
		AccessRules: &domain.AccessRules{
			ClusterIDs: []string{clusterID},
		},
	}

	got := redactClusterURLs(cluster, admin)
	assert.Equal(t, "nats://nats:4222", got.NATSURL)
	assert.Equal(t, "http://nats:8222", got.MonitoringURL)
}

func TestRedactURLUserinfoMultiURL(t *testing.T) {
	t.Parallel()

	raw := "nats://user:secret@a:4222,nats://user2:secret2@b:4222"
	got := redactURLUserinfo(raw)
	assert.Equal(t, "nats://a:4222,nats://b:4222", got)
	assert.NotContains(t, got, "secret")
}
