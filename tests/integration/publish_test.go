//go:build integration

package integration_test

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishStreamMessage(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	clusterID := stack.DefaultClusterID(t)
	base := srv.BaseURL(clusterID)

	createBody := `{"name":"PUBLISH_TEST","subjects":["pub.test"]}`
	resp, err := srv.Client.Post(base+"/streams", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(respBody))

	payload := base64.StdEncoding.EncodeToString([]byte(`{"hello":"world"}`))
	publishBody := fmt.Sprintf(`{"subject":"pub.test","data":%q}`, payload)
	resp, err = srv.Client.Post(base+"/streams/PUBLISH_TEST/messages", "application/json", strings.NewReader(publishBody))
	require.NoError(t, err)
	respBody, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(respBody))

	var published struct {
		Stream  string `json:"stream"`
		Subject string `json:"subject"`
		Seq     uint64 `json:"seq"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &published))
	assert.Equal(t, "PUBLISH_TEST", published.Stream)
	assert.Equal(t, "pub.test", published.Subject)
	assert.Greater(t, published.Seq, uint64(0))

	resp, err = srv.Client.Get(fmt.Sprintf("%s/streams/PUBLISH_TEST/messages?seq=%d", base, published.Seq))
	require.NoError(t, err)
	getBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got struct {
		Message struct {
			Data string `json:"data"`
		} `json:"message"`
	}
	require.NoError(t, sonic.Unmarshal(getBody, &got))
	decoded, err := base64.StdEncoding.DecodeString(got.Message.Data)
	require.NoError(t, err)
	assert.JSONEq(t, `{"hello":"world"}`, string(decoded))
}
