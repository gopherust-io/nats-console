//go:build integration

package integration_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
)

func TestPublishStreamMessage(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"PUBLISH_TEST","subjects":["pub.test"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(respBody))

	payload := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(`{"hello":"world"}`))
	publishBody := fmt.Sprintf(`{"subject":"pub.test","data":%q}`, payload)
	resp, err = srv.Client.Post(base+"/streams/PUBLISH_TEST/messages", "application/json", strings.NewReader(publishBody))
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, commonstrings.BytesToString(respBody))

	var published struct {
		Data struct {
			Stream  string `json:"stream"`
			Subject string `json:"subject"`
			Seq     uint64 `json:"seq"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &published))
	assert.Equal(t, "PUBLISH_TEST", published.Data.Stream)
	assert.Equal(t, "pub.test", published.Data.Subject)
	assert.Greater(t, published.Data.Seq, uint64(0))

	resp, err = srv.Client.Get(fmt.Sprintf("%s/streams/PUBLISH_TEST/messages?seq=%d", base, published.Data.Seq))
	require.NoError(t, err)
	getBody := resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Data struct {
			Message struct {
				Data string `json:"data"`
			} `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(getBody, &got))
	decoded, err := base64.StdEncoding.DecodeString(got.Data.Message.Data)
	require.NoError(t, err)
	assert.JSONEq(t, `{"hello":"world"}`, commonstrings.BytesToString(decoded))
}
