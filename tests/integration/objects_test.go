//go:build integration

package integration_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectBucketAndObjectLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	createBody := `{"bucket":"ARTIFACTS"}`
	resp, err := srv.Client.Post(base+"/objects/buckets", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create object bucket: %s", commonstrings.BytesToString(respBody))

	resp, err = srv.Client.Get(base + "/objects/buckets")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list object buckets: %s", commonstrings.BytesToString(respBody))
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &list))
	require.GreaterOrEqual(t, list.Meta.Total, 1)

	resp, err = srv.Client.Get(base + "/objects/buckets/ARTIFACTS")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "get object bucket: %s", commonstrings.BytesToString(respBody))
	var bucket struct {
		Data struct {
			Bucket string `json:"bucket"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &bucket))
	assert.Equal(t, "ARTIFACTS", bucket.Data.Bucket)

	dataB64 := base64.StdEncoding.EncodeToString(commonstrings.StringToBytes("hello-object"))
	putBody := fmt.Sprintf(`{"data":%q}`, dataB64)
	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    base + "/objects/buckets/ARTIFACTS/objects/readme.txt",
		Body:   strings.NewReader(putBody),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "put object: %s", commonstrings.BytesToString(respBody))

	resp, err = srv.Client.Get(base + "/objects/buckets/ARTIFACTS/objects/readme.txt")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "get object: %s", commonstrings.BytesToString(respBody))
	var obj struct {
		Data struct {
			Name string `json:"name"`
			Data string `json:"data"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &obj))
	assert.Equal(t, "readme.txt", obj.Data.Name)
	assert.Equal(t, dataB64, obj.Data.Data)

	resp, err = srv.Client.Get(base + "/objects/buckets/ARTIFACTS/objects")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list objects: %s", commonstrings.BytesToString(respBody))
	var objects struct {
		Data []string `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &objects))
	require.GreaterOrEqual(t, objects.Meta.Total, 1)
	assert.Contains(t, objects.Data, "readme.txt")

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base + "/objects/buckets/ARTIFACTS/objects/readme.txt",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete object")

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base + "/objects/buckets/ARTIFACTS",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete object bucket")
}
