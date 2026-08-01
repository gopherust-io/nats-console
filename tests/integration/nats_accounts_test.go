//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/gopherust-io/nats-consol/tests/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNATSAccountUsersLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	createBody := `{
		"name":"app-worker",
		"accountName":"Default",
		"pubAllow":["orders.>"],
		"subAllow":["events.>"]
	}`
	resp, err := srv.Client.Post(base+"/nats-users", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create nats user: %s", string(respBody))

	var created map[string]any
	require.NoError(t, sonic.Unmarshal(respBody, &created))
	data, _ := created["data"].(map[string]any)
	require.NotNil(t, data)
	userID, _ := data["id"].(string)
	require.NotEmpty(t, userID)
	// Root admin (Basic auth in test client) may download credentials on create.
	assert.NotEmpty(t, data["seed"])
	assert.NotEmpty(t, data["creds"])

	resp, err = srv.Client.Get(base + "/nats-users?account=Default")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list nats users: %s", string(respBody))
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &list))
	require.GreaterOrEqual(t, list.Meta.Total, 1)

	resp, err = srv.Client.Get(base + "/nats-users/" + userID + "?account=Default")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "get nats user: %s", string(respBody))
	var got struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &got))
	assert.Equal(t, userID, got.Data.ID)
	assert.Equal(t, "app-worker", got.Data.Name)

	updateBody := `{
		"pubAllow":["orders.>","metrics.>"],
		"subAllow":["events.>"]
	}`
	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    base+"/nats-users/"+userID+"?account=Default",
		Body:   strings.NewReader(updateBody),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "update nats user: %s", string(respBody))

	resp, err = srv.Client.Get(base + "/nats-users/subject-permissions?account=Default&subject=orders.create")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "subject permissions: %s", string(respBody))
	var perms struct {
		Data struct {
			Subject string `json:"subject"`
			Publish []struct {
				UserID string `json:"userId"`
				Name   string `json:"name"`
			} `json:"publish"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &perms))
	assert.Equal(t, "orders.create", perms.Data.Subject)
	found := false
	for _, p := range perms.Data.Publish {
		if p.UserID == userID {
			found = true
			assert.Equal(t, "app-worker", p.Name)
			break
		}
	}
	require.True(t, found, "expected user in publish permissions: %s", string(respBody))

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base+"/nats-users/"+userID+"?account=Default",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete nats user")
}

func TestNATSAccountSigningGroupsLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	createBody := `{
		"name":"workers",
		"accountName":"Default",
		"scoped":true,
		"pubAllow":["jobs.>"],
		"subAllow":["jobs.>"]
	}`
	resp, err := srv.Client.Post(base+"/signing-groups", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create signing group: %s", string(respBody))
	var created struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &created))
	require.NotEmpty(t, created.Data.ID)
	assert.Equal(t, "workers", created.Data.Name)

	resp, err = srv.Client.Get(base + "/signing-groups?account=Default")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list signing groups: %s", string(respBody))
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &list))
	require.GreaterOrEqual(t, list.Meta.Total, 1)

	updateBody := `{
		"scoped":true,
		"pubAllow":["jobs.>","metrics.>"],
		"subAllow":["jobs.>"]
	}`
	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    base+"/signing-groups/"+created.Data.ID+"?account=Default",
		Body:   strings.NewReader(updateBody),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "update signing group: %s", string(respBody))

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base+"/signing-groups/"+created.Data.ID+"?account=Default",
	})
	require.NoError(t, err)
	body := resp.Body
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete signing group: %s", string(body))
}

func TestNATSAccountExportsLifecycle(t *testing.T) {
	stack := testutil.SetupStack(t)
	srv := stack.NewServer(t, nil)
	base := srv.BaseURL(stack.DefaultClusterID(t))

	createBody := `{
		"accountName":"Default",
		"kind":"stream",
		"name":"orders-export",
		"subject":"orders.>",
		"description":"shared orders"
	}`
	resp, err := srv.Client.Post(base+"/sharing/exports", "application/json", strings.NewReader(createBody))
	require.NoError(t, err)
	respBody := resp.Body
	require.Equal(t, http.StatusCreated, resp.StatusCode, "create export: %s", string(respBody))
	var created struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &created))
	require.NotEmpty(t, created.Data.ID)

	resp, err = srv.Client.Get(base + "/sharing/exports?account=Default")
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "list exports: %s", string(respBody))
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, sonic.Unmarshal(respBody, &list))
	require.GreaterOrEqual(t, list.Meta.Total, 1)

	updateBody := fmt.Sprintf(`{
		"name":"orders-export-v2",
		"subject":"orders.>",
		"description":"updated"
	}`)
	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodPut,
		URL:    base+"/sharing/exports/"+created.Data.ID+"?account=Default",
		Body:   strings.NewReader(updateBody),
		Header: http.Header{
			"Content-Type": {"application/json"},
		},
	})
	require.NoError(t, err)
	respBody = resp.Body
	require.Equal(t, http.StatusOK, resp.StatusCode, "update export: %s", string(respBody))

	resp, err = srv.Client.Do(&testutil.Request{
		Method: http.MethodDelete,
		URL:    base+"/sharing/exports/"+created.Data.ID+"?account=Default",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode, "delete export")
}
