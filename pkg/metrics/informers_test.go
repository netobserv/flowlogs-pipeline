package metrics

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/tlsprofile"
	"github.com/stretchr/testify/require"
)

func TestInformerMetricsServerTLS(t *testing.T) {
	t.Setenv(tlsprofile.EnvMinVersion, "")
	t.Setenv(tlsprofile.EnvCipherSuites, "")
	t.Setenv(tlsprofile.EnvCurvePreferences, "")
	certificateServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer certificateServer.Close()
	certificate := certificateServer.TLS.Certificates[0]
	keyDER, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}

	portListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portListener.Addr().(*net.TCPAddr).Port
	_ = portListener.Close()

	metricsServer := NewServer(port, certPath, keyPath)
	if err := metricsServer.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = metricsServer.Stop() }()

	caPool := x509.NewCertPool()
	caPool.AddCert(certificateServer.Certificate())
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS13,
	}}, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get("https://127.0.0.1:" + strconv.Itoa(port) + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.TLS == nil || response.TLS.Version != tls.VersionTLS13 {
		t.Fatalf("metrics endpoint did not serve TLS 1.3: status=%d TLS=%v", response.StatusCode, response.TLS)
	}
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "go_goroutines")

	// A plaintext metrics scrape must not succeed when TLS is configured.
	plainClient := &http.Client{Timeout: 5 * time.Second}
	defer plainClient.CloseIdleConnections()
	plainResponse, err := plainClient.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/metrics")
	if err == nil {
		defer plainResponse.Body.Close()
		require.NotEqual(t, http.StatusOK, plainResponse.StatusCode)
	}
}

func TestInformerMetricsServerRequiresCertificatePair(t *testing.T) {
	for _, paths := range [][2]string{{"cert.pem", ""}, {"", "key.pem"}, {"missing.crt", "missing.key"}} {
		t.Run(paths[0]+paths[1], func(t *testing.T) {
			err := NewServer(0, paths[0], paths[1]).Start()
			require.Error(t, err)
		})
	}
}

func TestInformerMetricsServerHTTP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	metricsServer := NewServer(port, "", "")
	require.NoError(t, metricsServer.Start())
	defer func() { require.NoError(t, metricsServer.Stop()) }()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/metrics")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Nil(t, response.TLS)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "go_goroutines")
}

func TestInformerMetricsServerPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener.Close()
	err = NewServer(listener.Addr().(*net.TCPAddr).Port, "", "").Start()
	require.ErrorContains(t, err, "listen for informer metrics")
}
