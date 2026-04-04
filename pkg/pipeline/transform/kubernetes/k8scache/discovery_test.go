/*
 * Copyright (C) 2024 Red Hat, Inc.
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

package k8scache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartProcessorDiscovery_InvalidResyncInterval(t *testing.T) {
	ctx := context.Background()
	client := NewClient(&ClientConfig{
		ProcessorID: "test",
		TLSEnabled:  false,
	})

	testCases := []struct {
		name           string
		resyncInterval int
		shouldFail     bool
	}{
		{
			name:           "zero interval",
			resyncInterval: 0,
			shouldFail:     true,
		},
		{
			name:           "negative interval",
			resyncInterval: -1,
			shouldFail:     true,
		},
		{
			name:           "negative interval large",
			resyncInterval: -100,
			shouldFail:     true,
		},
		{
			name:           "positive interval",
			resyncInterval: 10,
			shouldFail:     false, // Will fail for other reasons (no k8s), but not validation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DiscoveryConfig{
				Kubeconfig:        "/nonexistent/path/to/kubeconfig", // Will fail on k8s config
				ProcessorSelector: "app=test",
				ProcessorPort:     9090,
				ResyncInterval:    tc.resyncInterval,
			}

			err := StartProcessorDiscovery(ctx, client, cfg)

			if tc.shouldFail {
				assert.Error(t, err, "should fail with invalid ResyncInterval")
				assert.Contains(t, err.Error(), "invalid ResyncInterval", "error should mention ResyncInterval")
			} else {
				// Will fail due to invalid kubeconfig, but not due to ResyncInterval
				assert.Error(t, err, "will fail due to kubeconfig")
				assert.NotContains(t, err.Error(), "invalid ResyncInterval", "error should not be about ResyncInterval")
			}
		})
	}
}
