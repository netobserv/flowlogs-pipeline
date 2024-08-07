package cni

import (
	"encoding/json"
	"fmt"
	"strings"

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

func (m *MultusPlugin) GetPodIPsAndMACs(pod *v1.Pod) ([]string, []string) {
	// Cf https://k8snetworkplumbingwg.github.io/multus-cni/docs/quickstart.html#network-status-annotations
	ips, macs, err := extractNetStatusIPsAndMACs(pod.Annotations)
	if err != nil {
		// Log the error as Info, do not block other ips indexing
		log.Infof("failed to index IPs from network-status annotation: %v", err)
	}
	log.Tracef("GetPodIPsAndMACs found ips: %v macs: %v for pod %s", ips, macs, pod.Name)
	return ips, macs
}

type netStatItem struct {
	IPs []string `json:"ips"`
	MAC string   `json:"mac"`
}

func extractNetStatusIPsAndMACs(annotations map[string]string) ([]string, []string, error) {
	if statusAnnotationJSON, ok := annotations[statusAnnotation]; ok {
		var ips, macs []string
		var networks []netStatItem
		err := json.Unmarshal([]byte(statusAnnotationJSON), &networks)
		if err == nil {
			for _, network := range networks {
				if len(network.IPs) > 0 {
					ips = append(ips, network.IPs...)
				}

				if len(network.MAC) > 0 {
					macs = append(macs, strings.ToUpper(network.MAC))
				}
			}
			return ips, macs, nil
		}

		return nil, nil, fmt.Errorf("cannot read annotation %s: %w", statusAnnotation, err)
	}
	// Annotation not present => just ignore, no error
	return nil, nil, nil
}
