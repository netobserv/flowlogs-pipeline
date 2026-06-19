/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package pipeline

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/stretchr/testify/require"
)

func TestNewHealthServer(t *testing.T) {
	readyPath := "/ready"
	livePath := "/live"

	type args struct {
		pipeline Pipeline
		address  string
	}
	type want struct {
		statusCode int
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{name: "pipeline running", args: args{pipeline: Pipeline{IsRunning: true}, address: "0.0.0.0:7000"}, want: want{statusCode: 200}},
		{name: "pipeline not running", args: args{pipeline: Pipeline{IsRunning: false}, address: "0.0.0.0:7001"}, want: want{statusCode: 503}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			opts := config.Options{HealthAddr: tt.args.address}
			server := operational.NewHealthServer(&opts, tt.args.pipeline.IsAlive, tt.args.pipeline.IsReady)
			require.NotNil(t, server)

			client := &http.Client{}

			time.Sleep(time.Second)
			readyURL := url.URL{Scheme: "http", Host: tt.args.address, Path: readyPath}
			var resp, err = client.Get(readyURL.String())
			require.NoError(t, err)
			require.Equal(t, tt.want.statusCode, resp.StatusCode)

			liveURL := url.URL{Scheme: "http", Host: tt.args.address, Path: livePath}
			resp, err = client.Get(liveURL.String())
			require.NoError(t, err)
			require.Equal(t, tt.want.statusCode, resp.StatusCode)

		})
	}
}
