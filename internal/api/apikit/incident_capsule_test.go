package apikit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gopherust-io/nats-consol/internal/domain"
)

func TestIsJetStreamResourcePathIncludesIncidentCapsules(t *testing.T) {
	t.Parallel()
	id := "c1"
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/incident-capsules/abc"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/streams/ORDERS/incident-capsules"))
	assert.True(t, IsJetStreamResourcePath("/api/v1/clusters/"+id+"/streams/ORDERS_DLQ/dlq/messages/1/capsule"))
}

func TestIncidentCapsuleCaptureRequestValidateWired(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, domain.IncidentCapsuleCaptureRequest{}.Validate(), domain.ErrCapsuleConsumerRequired)
}
