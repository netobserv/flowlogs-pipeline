package kubernetes

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/datasource"
	inf "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var ipInfo = map[string]*model.ResourceMetaData{
	"1.2.3.4": nil,
	"10.0.0.1": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "ns-1",
		},
		Kind:        "Pod",
		HostName:    "host-1",
		HostIP:      "100.0.0.1",
		NetworkName: "primary",
	},
	"10.0.0.2": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "ns-2",
		},
		Kind:        "Pod",
		HostName:    "host-2",
		HostIP:      "100.0.0.2",
		NetworkName: "primary",
	},
	"20.0.0.1": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "service-1",
			Namespace: "ns-1",
		},
		Kind:        "Service",
		NetworkName: "primary",
	},
}

var customKeysInfo = map[string]*model.ResourceMetaData{
	"~~aa:bb:cc:dd:ee:ff": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "ns-1",
		},
		Kind:        "Pod",
		HostName:    "host-1",
		HostIP:      "100.0.0.1",
		NetworkName: "custom-network",
	},
}

var nodes = map[string]*model.ResourceMetaData{
	"host-1": {
		ObjectMeta: v1.ObjectMeta{
			Name: "host-1",
			Labels: map[string]string{
				nodeZoneLabelName: "us-east-1a",
			},
		},
		Kind:        "Node",
		NetworkName: "primary",
	},
	"host-2": {
		ObjectMeta: v1.ObjectMeta{
			Name: "host-2",
			Labels: map[string]string{
				nodeZoneLabelName: "us-east-1b",
			},
		},
		Kind:        "Node",
		NetworkName: "primary",
	},
}

var nt = api.TransformNetwork{
	Rules: api.NetworkTransformRules{
		{
			Type: api.NetworkAddKubernetes,
			Kubernetes: &api.K8sRule{
				IPField:  "SrcAddr",
				MACField: "SrcMAC",
				Output:   "SrcK8s",
				AddZone:  true,
			},
		},
		{
			Type: api.NetworkAddKubernetes,
			Kubernetes: &api.K8sRule{
				IPField:  "DstAddr",
				MACField: "DstMAC",
				Output:   "DstK8s",
				AddZone:  true,
			},
		},
	},
}

func setupStubs(ipInfo, customKeysInfo, nodes map[string]*model.ResourceMetaData) {
	cfg, informers := inf.SetupStubs(ipInfo, customKeysInfo, nodes)
	ds = &datasource.Datasource{Informers: informers}
	infConfig = cfg
}

func TestEnrich(t *testing.T) {
	setupStubs(ipInfo, customKeysInfo, nodes)
	nt.Preprocess()

	// Pod to unknown
	entry := config.GenericMap{
		"SrcAddr": "10.0.0.1",    // pod-1
		"DstAddr": "42.42.42.42", // unknown
	}
	for _, r := range nt.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstAddr":            "42.42.42.42",
		"SrcAddr":            "10.0.0.1",
		"SrcK8s_HostIP":      "100.0.0.1",
		"SrcK8s_HostName":    "host-1",
		"SrcK8s_Name":        "pod-1",
		"SrcK8s_Namespace":   "ns-1",
		"SrcK8s_OwnerName":   "",
		"SrcK8s_OwnerType":   "",
		"SrcK8s_Type":        "Pod",
		"SrcK8s_Zone":        "us-east-1a",
		"SrcK8s_NetworkName": "primary",
	}, entry)

	// Pod to pod
	entry = config.GenericMap{
		"SrcAddr": "10.0.0.1", // pod-1
		"DstAddr": "10.0.0.2", // pod-2
	}
	for _, r := range nt.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstAddr":            "10.0.0.2",
		"DstK8s_HostIP":      "100.0.0.2",
		"DstK8s_HostName":    "host-2",
		"DstK8s_Name":        "pod-2",
		"DstK8s_Namespace":   "ns-2",
		"DstK8s_OwnerName":   "",
		"DstK8s_OwnerType":   "",
		"DstK8s_Type":        "Pod",
		"DstK8s_Zone":        "us-east-1b",
		"DstK8s_NetworkName": "primary",
		"SrcAddr":            "10.0.0.1",
		"SrcK8s_HostIP":      "100.0.0.1",
		"SrcK8s_HostName":    "host-1",
		"SrcK8s_Name":        "pod-1",
		"SrcK8s_Namespace":   "ns-1",
		"SrcK8s_OwnerName":   "",
		"SrcK8s_OwnerType":   "",
		"SrcK8s_Type":        "Pod",
		"SrcK8s_Zone":        "us-east-1a",
		"SrcK8s_NetworkName": "primary",
	}, entry)

	// Pod to service
	entry = config.GenericMap{
		"SrcAddr": "10.0.0.2", // pod-2
		"DstAddr": "20.0.0.1", // service-1
	}
	for _, r := range nt.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstAddr":            "20.0.0.1",
		"DstK8s_Name":        "service-1",
		"DstK8s_Namespace":   "ns-1",
		"DstK8s_OwnerName":   "",
		"DstK8s_OwnerType":   "",
		"DstK8s_Type":        "Service",
		"DstK8s_NetworkName": "primary",
		"SrcAddr":            "10.0.0.2",
		"SrcK8s_HostIP":      "100.0.0.2",
		"SrcK8s_HostName":    "host-2",
		"SrcK8s_Name":        "pod-2",
		"SrcK8s_Namespace":   "ns-2",
		"SrcK8s_OwnerName":   "",
		"SrcK8s_OwnerType":   "",
		"SrcK8s_Type":        "Pod",
		"SrcK8s_Zone":        "us-east-1b",
		"SrcK8s_NetworkName": "primary",
	}, entry)
}

var ntOtel = api.TransformNetwork{
	Rules: api.NetworkTransformRules{
		{
			Type: api.NetworkAddKubernetes,
			Kubernetes: &api.K8sRule{
				IPField:  "source.ip",
				Output:   "source.",
				Assignee: "otel",
				AddZone:  true,
			},
		},
		{
			Type: api.NetworkAddKubernetes,
			Kubernetes: &api.K8sRule{
				IPField:  "destination.ip",
				Output:   "destination.",
				Assignee: "otel",
				AddZone:  true,
			},
		},
	},
}

func TestEnrich_Otel(t *testing.T) {
	setupStubs(ipInfo, customKeysInfo, nodes)
	ntOtel.Preprocess()

	// Pod to unknown
	entry := config.GenericMap{
		"source.ip":      "10.0.0.1",    // pod-1
		"destination.ip": "42.42.42.42", // unknown
	}
	for _, r := range ntOtel.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"destination.ip":            "42.42.42.42",
		"source.ip":                 "10.0.0.1",
		"source.k8s.host.ip":        "100.0.0.1",
		"source.k8s.host.name":      "host-1",
		"source.k8s.name":           "pod-1",
		"source.k8s.namespace.name": "ns-1",
		"source.k8s.net.name":       "primary",
		"source.k8s.pod.name":       "pod-1",
		"source.k8s.pod.uid":        types.UID(""),
		"source.k8s.owner.name":     "",
		"source.k8s.owner.type":     "",
		"source.k8s.type":           "Pod",
		"source.k8s.zone":           "us-east-1a",
	}, entry)

	// Pod to pod
	entry = config.GenericMap{
		"source.ip":      "10.0.0.1", // pod-1
		"destination.ip": "10.0.0.2", // pod-2
	}
	for _, r := range ntOtel.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"destination.ip":                 "10.0.0.2",
		"destination.k8s.host.ip":        "100.0.0.2",
		"destination.k8s.host.name":      "host-2",
		"destination.k8s.name":           "pod-2",
		"destination.k8s.namespace.name": "ns-2",
		"destination.k8s.net.name":       "primary",
		"destination.k8s.pod.name":       "pod-2",
		"destination.k8s.pod.uid":        types.UID(""),
		"destination.k8s.owner.name":     "",
		"destination.k8s.owner.type":     "",
		"destination.k8s.type":           "Pod",
		"destination.k8s.zone":           "us-east-1b",
		"source.ip":                      "10.0.0.1",
		"source.k8s.host.ip":             "100.0.0.1",
		"source.k8s.host.name":           "host-1",
		"source.k8s.name":                "pod-1",
		"source.k8s.namespace.name":      "ns-1",
		"source.k8s.net.name":            "primary",
		"source.k8s.pod.name":            "pod-1",
		"source.k8s.pod.uid":             types.UID(""),
		"source.k8s.owner.name":          "",
		"source.k8s.owner.type":          "",
		"source.k8s.type":                "Pod",
		"source.k8s.zone":                "us-east-1a",
	}, entry)

	// Pod to service
	entry = config.GenericMap{
		"source.ip":      "10.0.0.2", // pod-2
		"destination.ip": "20.0.0.1", // service-1
	}
	for _, r := range ntOtel.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"destination.ip":                 "20.0.0.1",
		"destination.k8s.name":           "service-1",
		"destination.k8s.namespace.name": "ns-1",
		"destination.k8s.net.name":       "primary",
		"destination.k8s.service.name":   "service-1",
		"destination.k8s.service.uid":    types.UID(""),
		"destination.k8s.owner.name":     "",
		"destination.k8s.owner.type":     "",
		"destination.k8s.type":           "Service",
		"source.ip":                      "10.0.0.2",
		"source.k8s.host.ip":             "100.0.0.2",
		"source.k8s.host.name":           "host-2",
		"source.k8s.name":                "pod-2",
		"source.k8s.namespace.name":      "ns-2",
		"source.k8s.net.name":            "primary",
		"source.k8s.pod.name":            "pod-2",
		"source.k8s.pod.uid":             types.UID(""),
		"source.k8s.owner.name":          "",
		"source.k8s.owner.type":          "",
		"source.k8s.type":                "Pod",
		"source.k8s.zone":                "us-east-1b",
	}, entry)
}

func TestEnrich_EmptyNamespace(t *testing.T) {
	setupStubs(ipInfo, customKeysInfo, nodes)
	nt.Preprocess()

	// We need to check that, whether it returns NotFound or just an empty namespace,
	// there is no map entry for that namespace (an empty-valued map entry is not valid)
	entry := config.GenericMap{
		"SrcAddr": "1.2.3.4", // would return an empty namespace
		"DstAddr": "3.2.1.0", // would return NotFound
	}

	for _, r := range nt.Rules {
		Enrich(entry, r.Kubernetes)
	}

	assert.NotContains(t, entry, "SrcK8s_Namespace")
	assert.NotContains(t, entry, "DstK8s_Namespace")
}

func TestEnrichLayer(t *testing.T) {
	rule := api.NetworkTransformRule{
		KubernetesInfra: &api.K8sInfraRule{
			NamespaceNameFields: []api.K8sReference{
				{Namespace: "SrcK8S_Namespace", Name: "SrcK8S_Name"},
				{Namespace: "DstK8S_Namespace", Name: "DstK8S_Name"},
			},
			Output:        "K8S_FlowLayer",
			InfraPrefixes: []string{"netobserv", "openshift-"},
			InfraRefs: []api.K8sReference{
				{
					Name:      "kubernetes",
					Namespace: "default",
				},
			},
		},
	}

	// infra to infra => infra
	flow := config.GenericMap{
		"SrcK8S_Name":      "prometheus-0",
		"SrcK8S_Namespace": "openshift-monitoring",
		"SrcAddr":          "10.0.0.1",
		"DstK8S_Name":      "flowlog-pipeline-12345",
		"DstK8S_Namespace": "netobserv",
		"DstAddr":          "10.0.0.3",
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "infra", flow["K8S_FlowLayer"])

	// infra to node => infra
	flow = config.GenericMap{
		"SrcK8S_Name":      "prometheus-0",
		"SrcK8S_Namespace": "openshift-monitoring",
		"SrcAddr":          "10.0.0.1",
		"DstK8S_Name":      "host-12345",
		"DstAddr":          "30.0.0.1",
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "infra", flow["K8S_FlowLayer"])

	// node to kubernetes => infra
	flow = config.GenericMap{
		"SrcK8S_Name":      "host-12345",
		"SrcAddr":          "30.0.0.1",
		"DstK8S_Name":      "kubernetes",
		"DstK8S_Namespace": "default",
		"DstAddr":          "20.0.0.1",
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "infra", flow["K8S_FlowLayer"])

	// node to external => infra
	flow = config.GenericMap{
		"SrcK8S_Name": "host-12345",
		"SrcAddr":     "30.0.0.1",
		"DstAddr":     "1.2.3.4", // external
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "infra", flow["K8S_FlowLayer"])

	// app to app => app
	flow = config.GenericMap{
		"SrcK8S_Name":      "my-app-12345",
		"SrcK8S_Namespace": "my-namespace",
		"SrcAddr":          "10.0.0.2",
		"DstK8S_Name":      "my-app",
		"DstK8S_Namespace": "my-namespace",
		"DstAddr":          "20.0.0.2",
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "app", flow["K8S_FlowLayer"])

	// node to app => app
	flow = config.GenericMap{
		"SrcK8S_Name":      "host-12345",
		"SrcAddr":          "30.0.0.1",
		"DstK8S_Name":      "my-app",
		"DstK8S_Namespace": "my-namespace",
		"DstAddr":          "20.0.0.2",
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "app", flow["K8S_FlowLayer"])

	// app to infra => app
	flow = config.GenericMap{
		"SrcK8S_Name":      "my-app-12345",
		"SrcK8S_Namespace": "my-namespace",
		"SrcAddr":          "10.0.0.2",
		"DstK8S_Name":      "kubernetes",
		"DstK8S_Namespace": "default",
		"DstAddr":          "20.0.0.1",
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "app", flow["K8S_FlowLayer"])

	// app to external => app
	flow = config.GenericMap{
		"SrcK8S_Name":      "my-app-12345",
		"SrcK8S_Namespace": "my-namespace",
		"SrcAddr":          "10.0.0.2",
		"DstAddr":          "1.2.3.4", // external
	}
	EnrichLayer(flow, rule.KubernetesInfra)

	assert.Equal(t, "app", flow["K8S_FlowLayer"])
}

func TestEnrichUsingMac(t *testing.T) {
	setupStubs(ipInfo, customKeysInfo, nodes)
	nt.Preprocess()

	// Pod to unknown using MAC
	entry := config.GenericMap{
		"SrcAddr": "8.8.8.8",
		"SrcMAC":  "aa:bb:cc:dd:ee:ff", // pod-1
		"DstAddr": "9.9.9.9",
		"DstMAC":  "GG:HH:II:JJ:KK:LL", // unknown
	}
	for _, r := range nt.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"SrcAddr":            "8.8.8.8",
		"SrcMAC":             "aa:bb:cc:dd:ee:ff",
		"DstAddr":            "9.9.9.9",
		"DstMAC":             "GG:HH:II:JJ:KK:LL",
		"SrcK8s_HostIP":      "100.0.0.1",
		"SrcK8s_HostName":    "host-1",
		"SrcK8s_Name":        "pod-1",
		"SrcK8s_Namespace":   "ns-1",
		"SrcK8s_OwnerName":   "",
		"SrcK8s_OwnerType":   "",
		"SrcK8s_Type":        "Pod",
		"SrcK8s_Zone":        "us-east-1a",
		"SrcK8s_NetworkName": "custom-network",
	}, entry)

	// remove the MAC rules and retry
	entry = config.GenericMap{
		"SrcMAC": "aa:bb:cc:dd:ee:ff", // pod-1
		"DstMAC": "GG:HH:II:JJ:KK:LL", // unknown
	}
	for _, r := range nt.Rules {
		r.Kubernetes.MACField = ""
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstMAC": "GG:HH:II:JJ:KK:LL",
		"SrcMAC": "aa:bb:cc:dd:ee:ff",
	}, entry)
}

func TestEnrichUsingUDN(t *testing.T) {
	var ntUDN = api.TransformNetwork{
		Rules: api.NetworkTransformRules{
			{
				Type: api.NetworkAddKubernetes,
				Kubernetes: &api.K8sRule{
					IPField:   "SrcAddr",
					MACField:  "SrcMAC",
					UDNsField: "Udns",
					Output:    "SrcK8s",
					AddZone:   true,
				},
			},
			{
				Type: api.NetworkAddKubernetes,
				Kubernetes: &api.K8sRule{
					IPField:   "DstAddr",
					MACField:  "DstMAC",
					UDNsField: "Udns",
					Output:    "DstK8s",
					AddZone:   true,
				},
			},
		},
	}
	customIndexes := map[string]*model.ResourceMetaData{
		"~~aa:bb:cc:dd:ee:ff": {
			ObjectMeta: v1.ObjectMeta{
				Name:      "pod-1",
				Namespace: "ns-1",
			},
			Kind:        "Pod",
			HostName:    "host-1",
			HostIP:      "100.0.0.1",
			NetworkName: "custom-network",
		},
		"ns-2/primary-udn~10.200.200.12": {
			ObjectMeta: v1.ObjectMeta{
				Name:      "pod-2",
				Namespace: "ns-2",
			},
			Kind:        "Pod",
			HostName:    "host-2",
			HostIP:      "100.0.0.2",
			NetworkName: "ns-2/primary-udn",
		},
	}
	setupStubs(ipInfo, customIndexes, nodes)
	ntUDN.Preprocess()

	// MAC-indexed Pod 1 to UDN-indexed Pod 2
	entry := config.GenericMap{
		"SrcAddr": "8.8.8.8",
		"SrcMAC":  "AA:BB:CC:DD:EE:FF", // pod-1
		"DstAddr": "10.200.200.12",     // pod-2 (UDN)
		"DstMAC":  "GG:HH:II:JJ:KK:LL", // unknown
		"Udns":    []string{"", "default", "ns-2/primary-udn"},
	}
	for _, r := range ntUDN.Rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"SrcAddr":            "8.8.8.8",
		"SrcMAC":             "AA:BB:CC:DD:EE:FF",
		"DstAddr":            "10.200.200.12",
		"DstMAC":             "GG:HH:II:JJ:KK:LL",
		"Udns":               []string{"", "default", "ns-2/primary-udn"},
		"SrcK8s_HostIP":      "100.0.0.1",
		"SrcK8s_HostName":    "host-1",
		"SrcK8s_Name":        "pod-1",
		"SrcK8s_Namespace":   "ns-1",
		"SrcK8s_OwnerName":   "",
		"SrcK8s_OwnerType":   "",
		"SrcK8s_Type":        "Pod",
		"SrcK8s_Zone":        "us-east-1a",
		"SrcK8s_NetworkName": "custom-network",
		"DstK8s_HostIP":      "100.0.0.2",
		"DstK8s_HostName":    "host-2",
		"DstK8s_Name":        "pod-2",
		"DstK8s_Namespace":   "ns-2",
		"DstK8s_OwnerName":   "",
		"DstK8s_OwnerType":   "",
		"DstK8s_Type":        "Pod",
		"DstK8s_Zone":        "us-east-1b",
		"DstK8s_NetworkName": "ns-2/primary-udn",
	}, entry)
}

func TestEnrich_LabelsAndAnnotationsPrefixes(t *testing.T) {
	testData := map[string]*model.ResourceMetaData{
		"10.0.0.10": {
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				Labels:    map[string]string{"app": "web", "tier": "backend"},
				Annotations: map[string]string{
					"owner":                "team-a",
					"prometheus.io/scrape": "true",
				},
			},
			Kind: "Pod",
		},
	}
	setupStubs(testData, nil, nodes)

	tests := []struct {
		name              string
		labelsPrefix      string
		annotationsPrefix string
		expectLabels      map[string]string
		expectAnnotations map[string]string
		notExpect         []string
	}{
		{
			name:              "both prefixes",
			labelsPrefix:      "K8s_Labels",
			annotationsPrefix: "K8s_Annotations",
			expectLabels:      map[string]string{"K8s_Labels_app": "web", "K8s_Labels_tier": "backend"},
			expectAnnotations: map[string]string{"K8s_Annotations_owner": "team-a", "K8s_Annotations_prometheus.io/scrape": "true"},
		},
		{
			name:         "labels only",
			labelsPrefix: "K8s_Labels",
			expectLabels: map[string]string{"K8s_Labels_app": "web"},
			notExpect:    []string{"K8s_Annotations_owner"},
		},
		{
			name:              "annotations only",
			annotationsPrefix: "K8s_Annotations",
			expectAnnotations: map[string]string{"K8s_Annotations_owner": "team-a"},
			notExpect:         []string{"K8s_Labels_app"},
		},
		{
			name:      "no prefixes",
			notExpect: []string{"K8s_Labels_app", "K8s_Annotations_owner"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := api.TransformNetwork{
				Rules: api.NetworkTransformRules{{
					Type: api.NetworkAddKubernetes,
					Kubernetes: &api.K8sRule{
						IPField:           "SrcAddr",
						Output:            "K8s",
						LabelsPrefix:      tt.labelsPrefix,
						AnnotationsPrefix: tt.annotationsPrefix,
					},
				}},
			}
			rule.Preprocess()

			entry := config.GenericMap{"SrcAddr": "10.0.0.10"}
			Enrich(entry, rule.Rules[0].Kubernetes)

			assert.Equal(t, "test-pod", entry["K8s_Name"])
			for k, v := range tt.expectLabels {
				assert.Equal(t, v, entry[k])
			}
			for k, v := range tt.expectAnnotations {
				assert.Equal(t, v, entry[k])
			}
			for _, k := range tt.notExpect {
				assert.NotContains(t, entry, k)
			}
		})
	}
}

func TestEnrich_LabelsAnnotationsFiltering(t *testing.T) {
	testData := map[string]*model.ResourceMetaData{
		"10.0.0.10": {
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app":         "myapp",
					"version":     "v1",
					"environment": "prod",
					"team":        "backend",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
					"annotation3": "value3",
					"annotation4": "value4",
				},
			},
			Kind: "Pod",
		},
	}

	setupStubs(testData, nil, nodes)

	tests := []struct {
		name                 string
		labelsPrefix         string
		labelInclusions      []string
		labelExclusions      []string
		annotationsPrefix    string
		annotationInclusions []string
		annotationExclusions []string
		expectedLabels       []string
		expectedAnnotations  []string
		notExpect            []string
	}{
		{
			name:                "No filtering - all labels and annotations included",
			labelsPrefix:        "k8s_labels",
			annotationsPrefix:   "k8s_annotations",
			expectedLabels:      []string{"k8s_labels_app", "k8s_labels_version", "k8s_labels_environment", "k8s_labels_team"},
			expectedAnnotations: []string{"k8s_annotations_annotation1", "k8s_annotations_annotation2", "k8s_annotations_annotation3", "k8s_annotations_annotation4"},
		},
		{
			name:                "Only label inclusions specified",
			labelsPrefix:        "k8s_labels",
			labelInclusions:     []string{"app", "version"},
			annotationsPrefix:   "k8s_annotations",
			expectedLabels:      []string{"k8s_labels_app", "k8s_labels_version"},
			expectedAnnotations: []string{"k8s_annotations_annotation1", "k8s_annotations_annotation2", "k8s_annotations_annotation3", "k8s_annotations_annotation4"},
			notExpect:           []string{"k8s_labels_environment", "k8s_labels_team"},
		},
		{
			name:                "Only label exclusions specified",
			labelsPrefix:        "k8s_labels",
			labelExclusions:     []string{"environment", "team"},
			annotationsPrefix:   "k8s_annotations",
			expectedLabels:      []string{"k8s_labels_app", "k8s_labels_version"},
			expectedAnnotations: []string{"k8s_annotations_annotation1", "k8s_annotations_annotation2", "k8s_annotations_annotation3", "k8s_annotations_annotation4"},
			notExpect:           []string{"k8s_labels_environment", "k8s_labels_team"},
		},
		{
			name:                "Both label inclusions and exclusions - exclusions take precedence",
			labelsPrefix:        "k8s_labels",
			labelInclusions:     []string{"app", "version", "environment"},
			labelExclusions:     []string{"environment"},
			annotationsPrefix:   "k8s_annotations",
			expectedLabels:      []string{"k8s_labels_app", "k8s_labels_version"},
			expectedAnnotations: []string{"k8s_annotations_annotation1", "k8s_annotations_annotation2", "k8s_annotations_annotation3", "k8s_annotations_annotation4"},
			notExpect:           []string{"k8s_labels_environment", "k8s_labels_team"},
		},
		{
			name:                 "Only annotation inclusions specified",
			labelsPrefix:         "k8s_labels",
			annotationsPrefix:    "k8s_annotations",
			annotationInclusions: []string{"annotation1", "annotation3"},
			expectedLabels:       []string{"k8s_labels_app", "k8s_labels_version", "k8s_labels_environment", "k8s_labels_team"},
			expectedAnnotations:  []string{"k8s_annotations_annotation1", "k8s_annotations_annotation3"},
			notExpect:            []string{"k8s_annotations_annotation2", "k8s_annotations_annotation4"},
		},
		{
			name:                 "Only annotation exclusions specified",
			labelsPrefix:         "k8s_labels",
			annotationsPrefix:    "k8s_annotations",
			annotationExclusions: []string{"annotation2", "annotation4"},
			expectedLabels:       []string{"k8s_labels_app", "k8s_labels_version", "k8s_labels_environment", "k8s_labels_team"},
			expectedAnnotations:  []string{"k8s_annotations_annotation1", "k8s_annotations_annotation3"},
			notExpect:            []string{"k8s_annotations_annotation2", "k8s_annotations_annotation4"},
		},
		{
			name:                 "Both annotation inclusions and exclusions - exclusions take precedence",
			labelsPrefix:         "k8s_labels",
			annotationsPrefix:    "k8s_annotations",
			annotationInclusions: []string{"annotation1", "annotation2", "annotation3"},
			annotationExclusions: []string{"annotation2"},
			expectedLabels:       []string{"k8s_labels_app", "k8s_labels_version", "k8s_labels_environment", "k8s_labels_team"},
			expectedAnnotations:  []string{"k8s_annotations_annotation1", "k8s_annotations_annotation3"},
			notExpect:            []string{"k8s_annotations_annotation2", "k8s_annotations_annotation4"},
		},
		{
			name:                 "Combined filtering for both labels and annotations",
			labelsPrefix:         "k8s_labels",
			labelInclusions:      []string{"app", "version", "team"},
			labelExclusions:      []string{"team"},
			annotationsPrefix:    "k8s_annotations",
			annotationInclusions: []string{"annotation1", "annotation2"},
			annotationExclusions: []string{"annotation1"},
			expectedLabels:       []string{"k8s_labels_app", "k8s_labels_version"},
			expectedAnnotations:  []string{"k8s_annotations_annotation2"},
			notExpect:            []string{"k8s_labels_environment", "k8s_labels_team", "k8s_annotations_annotation1", "k8s_annotations_annotation3", "k8s_annotations_annotation4"},
		},
		{
			name:                 "Empty prefix - no labels or annotations added",
			labelsPrefix:         "",
			labelInclusions:      []string{"app"},
			annotationsPrefix:    "",
			annotationInclusions: []string{"annotation1"},
			notExpect:            []string{"k8s_labels_app", "k8s_labels_version", "k8s_labels_environment", "k8s_labels_team", "k8s_annotations_annotation1", "k8s_annotations_annotation2", "k8s_annotations_annotation3", "k8s_annotations_annotation4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := api.TransformNetwork{
				Rules: api.NetworkTransformRules{{
					Type: api.NetworkAddKubernetes,
					Kubernetes: &api.K8sRule{
						IPField:              "SrcAddr",
						Output:               "SrcK8s",
						LabelsPrefix:         tt.labelsPrefix,
						LabelInclusions:      tt.labelInclusions,
						LabelExclusions:      tt.labelExclusions,
						AnnotationsPrefix:    tt.annotationsPrefix,
						AnnotationInclusions: tt.annotationInclusions,
						AnnotationExclusions: tt.annotationExclusions,
					},
				}},
			}
			rule.Preprocess()

			entry := config.GenericMap{
				"SrcAddr": "10.0.0.10",
			}

			Enrich(entry, rule.Rules[0].Kubernetes)

			for _, label := range tt.expectedLabels {
				assert.Contains(t, entry, label, "Expected label %s to be present", label)
			}
			for _, annotation := range tt.expectedAnnotations {
				assert.Contains(t, entry, annotation, "Expected annotation %s to be present", annotation)
			}
			for _, k := range tt.notExpect {
				assert.NotContains(t, entry, k)
			}
		})
	}
}

func TestShouldInclude(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		inclusions map[string]struct{}
		exclusions map[string]struct{}
		expected   bool
	}{
		{
			name:       "No inclusions or exclusions - should include",
			key:        "test",
			inclusions: map[string]struct{}{},
			exclusions: map[string]struct{}{},
			expected:   true,
		},
		{
			name:       "Key in inclusions - should include",
			key:        "test",
			inclusions: map[string]struct{}{"test": {}},
			exclusions: map[string]struct{}{},
			expected:   true,
		},
		{
			name:       "Key not in inclusions - should not include",
			key:        "test",
			inclusions: map[string]struct{}{"other": {}},
			exclusions: map[string]struct{}{},
			expected:   false,
		},
		{
			name:       "Key in exclusions - should not include",
			key:        "test",
			inclusions: map[string]struct{}{},
			exclusions: map[string]struct{}{"test": {}},
			expected:   false,
		},
		{
			name:       "Key in both inclusions and exclusions - exclusions win",
			key:        "test",
			inclusions: map[string]struct{}{"test": {}},
			exclusions: map[string]struct{}{"test": {}},
			expected:   false,
		},
		{
			name:       "Key not in exclusions, no inclusions - should include",
			key:        "test",
			inclusions: map[string]struct{}{},
			exclusions: map[string]struct{}{"other": {}},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldInclude(tt.key, tt.inclusions, tt.exclusions)
			assert.Equal(t, tt.expected, result)
		})
	}
}
