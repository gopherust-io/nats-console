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

func TestKVBucketAndKeyLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	createBody := `{"bucket":"CONFIG","history":5}`
	resp, err := srv.Client.Post(base+"/kv/buckets", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create kv bucket: %s", commonstrings.BytesToString(respBody))

	resp, err = srv.Client.Get(base + "/kv/buckets")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list kv buckets: %s", commonstrings.BytesToString(respBody))
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &list))
	require.GreaterOrEqual(t, list.Meta.Total, 1)

	resp, err = srv.Client.Get(base + "/kv/buckets/CONFIG")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "get kv bucket: %s", commonstrings.BytesToString(respBody))
	var bucket struct {
		Data struct {
			Bucket string `json:"bucket"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &bucket))
	assert.Equal(t, "CONFIG", bucket.Data.Bucket)

	valueB64 := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes(`{"enabled":true}`))
	putBody := fmt.Sprintf(`{"value":%q}`, valueB64)
	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    base+"/kv/buckets/CONFIG/keys/feature.flag",
		Body:   strings.NewReader(putBody),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "put kv entry: %s", commonstrings.BytesToString(respBody))

	resp, err = srv.Client.Get(base + "/kv/buckets/CONFIG/keys/feature.flag")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "get kv entry: %s", commonstrings.BytesToString(respBody))
	var entry struct {
		Data struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &entry))
	assert.Equal(t, "feature.flag", entry.Data.Key)
	assert.Equal(t, valueB64, entry.Data.Value)

	resp, err = srv.Client.Get(base + "/kv/buckets/CONFIG/keys")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list kv keys: %s", commonstrings.BytesToString(respBody))
	var keys struct {
		Data []string `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &keys))
	require.GreaterOrEqual(t, keys.Meta.Total, 1)
	assert.Contains(t, keys.Data, "feature.flag")

	resp, err = srv.Client.Get(base + "/kv/buckets/CONFIG/keys/feature.flag/history")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "kv history: %s", commonstrings.BytesToString(respBody))
	var history struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &history))
	require.GreaterOrEqual(t, history.Meta.Total, 1)

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base+"/kv/buckets/CONFIG/keys/feature.flag",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete kv entry")

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base+"/kv/buckets/CONFIG",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete kv bucket")
}

func TestKVBucketNotFound(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	resp, err := srv.Client.Get(base + "/kv/buckets/MISSING")
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusNotFound, resp.StatusCode, commonstrings.BytesToString(body))
}
