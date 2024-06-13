package cni

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	statusAnnotation = "k8s.v1.cni.cncf.io/network-status"
)

type MultusPlugin struct {
	Plugin
}

func (m *MultusPlugin) GetNodeIPs(_ *v1.Node) []string {
	// No CNI-specific logic needed for pods
	return nil
}

func (m *MultusPlugin) GetPodIPs(pod *v1.Pod) []string {
	// Cf https://k8snetworkplumbingwg.github.io/multus-cni/docs/quickstart.html#network-status-annotations
	ips, err := extractNetStatusIPs(pod.Annotations)
	if err != nil {
		// Log the error as Info, do not block other ips indexing
		log.Infof("failed to index IPs from network-status annotation: %v", err)
	}
	return ips
}

type netStatItem struct {
	IPs []string `json:"ips"`
}

func extractNetStatusIPs(annotations map[string]string) ([]string, error) {
	if statusAnnotationJSON, ok := annotations[statusAnnotation]; ok {
		var allIPs []string
		var networks []netStatItem
		err := json.Unmarshal([]byte(statusAnnotationJSON), &networks)
		if err == nil {
			for _, network := range networks {
				allIPs = append(allIPs, network.IPs...)
			}
			return allIPs, nil
		}

		return nil, fmt.Errorf("cannot read annotation %s: %w", statusAnnotation, err)
	}
	// Annotation not present => just ignore, no error
	return nil, nil
}
