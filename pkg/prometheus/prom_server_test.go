package prometheus

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartPromServer(t *testing.T) {
	srv := InitializePrometheus(&config.MetricsSettings{})

	serverURL := "http://0.0.0.0:9090"
	t.Logf("Started test http server: %v", serverURL)

	httpClient := &http.Client{}

	// wait for our test http server to come up
	checkHTTPReady(httpClient, serverURL)

	r, err := http.NewRequest("GET", serverURL+"/metrics", nil)
	require.NoError(t, err)

	resp, err := httpClient.Do(r)
	require.NoError(t, err)
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyString := string(bodyBytes)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, bodyString, "go_gc_duration_seconds")

	_ = srv.Shutdown(context.Background())
}

func TestStartPromServer_HeadersLimit(t *testing.T) {
	srv := InitializePrometheus(&config.MetricsSettings{})

	serverURL := "http://0.0.0.0:9090"
	t.Logf("Started test http server: %v", serverURL)

	httpClient := &http.Client{}

	// wait for our test http server to come up
	checkHTTPReady(httpClient, serverURL)

	r, err := http.NewRequest("GET", serverURL+"/metrics", nil)
	require.NoError(t, err)

	// Set many headers
	oneKBString := strings.Repeat(".", 1024)
	for i := 0; i < 1025; i++ {
		r.Header.Set(fmt.Sprintf("test-header-%d", i), oneKBString)
	}

	resp, err := httpClient.Do(r)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusRequestHeaderFieldsTooLarge, resp.StatusCode)

	_ = srv.Shutdown(context.Background())
}

// TestStartPromServer_AuthEnabled checks that the metrics endpoint is actually guarded
// end-to-end, over a real listener, when authentication is configured.
func TestStartPromServer_AuthEnabled(t *testing.T) {
	apiServer := test.StubK8sAuthServer(t)

	srv, err := StartServerAsync(&api.PromConnectionInfo{
		Port: 9099,
		Auth: &api.PromAuth{Type: api.PromAuthKubernetes, KubeConfigPath: test.WriteKubeConfig(t, apiServer.URL)},
	}, "", prom.DefaultGatherer)
	require.NoError(t, err)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	serverURL := "http://0.0.0.0:9099"
	httpClient := &http.Client{}
	checkHTTPReady(httpClient, serverURL)

	t.Run("anonymous request is refused", func(t *testing.T) {
		resp, err := httpClient.Get(serverURL + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.NotContains(t, string(body), "go_gc_duration_seconds")
	})

	t.Run("authorized request is served", func(t *testing.T) {
		r, err := http.NewRequest("GET", serverURL+"/metrics", nil)
		require.NoError(t, err)
		r.Header.Set("Authorization", "Bearer "+test.AllowedToken)

		resp, err := httpClient.Do(r)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, string(body), "go_gc_duration_seconds")
	})
}

// TestStartPromServer_AuthMisconfigured checks the fail-closed property: a bad auth
// configuration must stop the server from starting rather than silently expose metrics.
func TestStartPromServer_AuthMisconfigured(t *testing.T) {
	srv, err := StartServerAsync(&api.PromConnectionInfo{
		Port: 9098,
		Auth: &api.PromAuth{Type: "not-a-real-type"},
	}, "", prom.DefaultGatherer)

	require.Error(t, err)
	assert.Nil(t, srv)

	// Nothing is listening on the port.
	_, dialErr := net.DialTimeout("tcp", "0.0.0.0:9098", time.Second)
	assert.Error(t, dialErr)
}

func checkHTTPReady(httpClient *http.Client, url string) {
	for i := 0; i < 60; i++ {
		if r, err := httpClient.Get(url); err == nil {
			r.Body.Close()
			break
		}
		time.Sleep(time.Second)
	}
}
