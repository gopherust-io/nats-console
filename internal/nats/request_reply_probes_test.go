package natsclient_test

import (
	"testing"

	libnats "github.com/gopherust-io/nats"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gopherust-io/nats-consol/internal/domain"
	natsclient "github.com/gopherust-io/nats-consol/internal/nats"
)

func TestProbeRequestHeaders(t *testing.T) {
	t.Parallel()

	jsonHeaders := natsclient.ProbeRequestHeaders(domain.RequestReplyFormatJSON, true)
	require.NotNil(t, jsonHeaders)
	assert.Equal(t, libnats.ContentTypeJSON, jsonHeaders.Get(libnats.HeaderContentType))

	assert.Nil(t, natsclient.ProbeRequestHeaders(domain.RequestReplyFormatJSON, false))
	assert.Nil(t, natsclient.ProbeRequestHeaders(domain.RequestReplyFormatRaw, true))

	mp := natsclient.ProbeRequestHeaders(domain.RequestReplyFormatMsgPack, true)
	require.NotNil(t, mp)
	assert.Equal(t, libnats.ContentTypeMsgPack, mp.Get(libnats.HeaderContentType))

	pb := natsclient.ProbeRequestHeaders(domain.RequestReplyFormatProtobuf, true)
	require.NotNil(t, pb)
	assert.Equal(t, libnats.ContentTypeProto, pb.Get(libnats.HeaderContentType))
}

func TestDetectReplyFormat(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "json", natsclient.DetectReplyFormat(nil))
	assert.Equal(t, "msgpack", natsclient.DetectReplyFormat(nats.Header{
		libnats.HeaderContentType: []string{libnats.ContentTypeMsgPack},
	}))
	assert.Equal(t, "protobuf", natsclient.DetectReplyFormat(nats.Header{
		libnats.HeaderContentType: []string{libnats.ContentTypeProto},
	}))
	assert.Equal(t, "json", natsclient.DetectReplyFormat(nats.Header{
		libnats.HeaderContentType: []string{libnats.ContentTypeJSON},
	}))
}
