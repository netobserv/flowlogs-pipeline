package prometheus

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
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

func checkHTTPReady(httpClient *http.Client, url string) {
	for i := 0; i < 60; i++ {
		if r, err := httpClient.Get(url); err == nil {
			r.Body.Close()
			break
		}
		time.Sleep(time.Second)
	}
}
