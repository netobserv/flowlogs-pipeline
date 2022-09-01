package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

func ReadExposedMetrics(t *testing.T) string {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:9090", nil)
	require.NotNil(t, req)
	w := httptest.NewRecorder()
	require.NotNil(t, w)
	promhttp.Handler().ServeHTTP(w, req)
	require.NotNil(t, w.Body)
	return w.Body.String()
}
