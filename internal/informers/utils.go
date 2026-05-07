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
