package health

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline"
	"github.com/stretchr/testify/require"
)

func TestNewHealthServer(t *testing.T) {
	readyPath := "/ready"
	livePath := "/live"

	type args struct {
		pipeline pipeline.Pipeline
		port     string
	}
	type want struct {
		statusCode int
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{name: "pipeline running", args: args{pipeline: pipeline.Pipeline{IsRunning: true}, port: "7000"}, want: want{statusCode: 200}},
		{name: "pipeline not running", args: args{pipeline: pipeline.Pipeline{IsRunning: false}, port: "7001"}, want: want{statusCode: 503}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			opts := config.Options{Health: config.Health{Port: tt.args.port}}
			expectedAddr := fmt.Sprintf("0.0.0.0:%s", opts.Health.Port)
			server := NewHealthServer(&opts, &tt.args.pipeline)
			require.NotNil(t, server)
			require.Equal(t, expectedAddr, server.address)

			client := &http.Client{}

			time.Sleep(time.Second)
			readyURL := url.URL{Scheme: "http", Host: expectedAddr, Path: readyPath}
			var resp, err = client.Get(readyURL.String())
			require.NoError(t, err)
			require.Equal(t, tt.want.statusCode, resp.StatusCode)

			liveURL := url.URL{Scheme: "http", Host: expectedAddr, Path: livePath}
			resp, err = client.Get(liveURL.String())
			require.NoError(t, err)
			require.Equal(t, tt.want.statusCode, resp.StatusCode)

		})
	}
}
