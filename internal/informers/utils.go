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

package informers

import (
	"fmt"
	"os"
)

// formatAddress formats address for HTTP server
func formatAddress(port int) string {
	return fmt.Sprintf("0.0.0.0:%d", port)
}

// GetPodName returns the pod name from environment variable
func GetPodName() string {
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return "unknown"
		}
		return hostname
	}
	return podName
}

// GetNamespace returns the namespace from environment variable or default
func GetNamespace() string {
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		return "netobserv"
	}
	return namespace
}
