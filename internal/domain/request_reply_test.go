package domain

import (
	"encoding/base64"
	"testing"

	"github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestReplyPayloadFormatNormalize(t *testing.T) {
	t.Parallel()

	assert.Equal(t, RequestReplyFormatJSON, RequestReplyPayloadFormat("").Normalize())
	assert.Equal(t, RequestReplyFormatJSON, RequestReplyPayloadFormat("JSON").Normalize())
	assert.Equal(t, RequestReplyFormatMsgPack, RequestReplyPayloadFormat("msgpack").Normalize())
	assert.Equal(t, RequestReplyFormatProtobuf, RequestReplyPayloadFormat("protobuf").Normalize())
	assert.Equal(t, RequestReplyFormatRaw, RequestReplyPayloadFormat("raw").Normalize())
	assert.Equal(t, RequestReplyPayloadFormat(""), RequestReplyPayloadFormat("cbor").Normalize())
}

func TestRequestReplyProbeCreateValidateFormats(t *testing.T) {
	t.Parallel()

	jsonPayload := base64.StdEncoding.EncodeToString(strings.StringToBytes(`{"ping":true}`))
	ok := RequestReplyProbeCreate{
		Subject:       "svc.ping",
		PayloadB64:    jsonPayload,
		PayloadFormat: RequestReplyFormatJSON,
	}
	require.NoError(t, ok.Validate())

	badJSON := RequestReplyProbeCreate{
		Subject:       "svc.ping",
		PayloadB64:    base64.StdEncoding.EncodeToString(strings.StringToBytes(`{not-json`)),
		PayloadFormat: RequestReplyFormatJSON,
	}
	assert.Error(t, badJSON.Validate())

	msgpack := RequestReplyProbeCreate{
		Subject:       "svc.ping",
		PayloadB64:    base64.StdEncoding.EncodeToString([]byte{0x81, 0xa4, 0x70, 0x69, 0x6e, 0x67, 0xc3}),
		PayloadFormat: RequestReplyFormatMsgPack,
	}
	require.NoError(t, msgpack.Validate())

	badFormat := RequestReplyProbeCreate{
		Subject:       "svc.ping",
		PayloadFormat: "cbor",
	}
	assert.Error(t, badFormat.Validate())
}

func TestValidateProbeSubject(t *testing.T) {
	t.Parallel()
	ok, err := CanonicalProbeSubject("  svc.ping  ")
	require.NoError(t, err)
	assert.Equal(t, "svc.ping", ok)

	assert.Error(t, ValidateProbeSubject("svc.*"))
	assert.Error(t, ValidateProbeSubject("foo.>"))
	assert.Error(t, ValidateProbeSubject("$JS.API.INFO"))
	assert.Error(t, ValidateProbeSubject("$SYS.SERVER.1"))
	assert.Error(t, ValidateProbeSubject("_INBOX.abc"))
	assert.Error(t, ValidateProbeSubject("a..b"))
	assert.Error(t, ValidateProbeSubject("$OTHER.x"))
}

func TestNormalizeProbeTimeoutMs(t *testing.T) {
	t.Parallel()
	ms, err := NormalizeProbeTimeoutMs(0)
	require.NoError(t, err)
	assert.Equal(t, DefaultRequestReplyTimeoutMs, ms)
	_, err = NormalizeProbeTimeoutMs(MaxRequestReplyTimeoutMs + 1)
	assert.Error(t, err)
	_, err = NormalizeProbeTimeoutMs(-1)
	assert.Error(t, err)
}

func TestRequestReplyProbeUpdateValidateUpdateUsesCurrentFormat(t *testing.T) {
	t.Parallel()
	payload := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02})
	in := RequestReplyProbeUpdate{PayloadB64: &payload}
	// JSON validation would fail for arbitrary bytes; msgpack current format allows it.
	require.NoError(t, in.ValidateUpdate(RequestReplyFormatMsgPack))
	assert.Error(t, in.ValidateUpdate(RequestReplyFormatJSON))
}

func TestTruncateReplyPreview(t *testing.T) {
	t.Parallel()
	small := strings.StringToBytes("hi")
	assert.Equal(t, small, TruncateReplyPreview(small))
	big := make([]byte, MaxRequestReplyReplyPreview+10)
	out := TruncateReplyPreview(big)
	assert.Len(t, out, MaxRequestReplyReplyPreview)
}
