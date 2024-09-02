package kubernetes

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	inf "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var ipInfo = map[string]*inf.Info{
	"1.2.3.4": nil,
	"10.0.0.1": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "ns-1",
		},
		Type:     "Pod",
		HostName: "host-1",
		HostIP:   "100.0.0.1",
	},
	"10.0.0.2": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "ns-2",
		},
		Type:     "Pod",
		HostName: "host-2",
		HostIP:   "100.0.0.2",
	},
	"20.0.0.1": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "service-1",
			Namespace: "ns-1",
		},
		Type: "Service",
	},
}

var macInfo = map[string]*inf.Info{
	"AA:BB:CC:DD:EE:FF": {
		ObjectMeta: v1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "ns-1",
		},
		Type:     "Pod",
		HostName: "host-1",
		HostIP:   "100.0.0.1",
	},
}

var nodes = map[string]*inf.Info{
	"host-1": {
		ObjectMeta: v1.ObjectMeta{
			Name: "host-1",
			Labels: map[string]string{
				nodeZoneLabelName: "us-east-1a",
			},
		},
		Type: "Node",
	},
	"host-2": {
		ObjectMeta: v1.ObjectMeta{
			Name: "host-2",
			Labels: map[string]string{
				nodeZoneLabelName: "us-east-1b",
			},
		},
		Type: "Node",
	},
}

var rules = api.NetworkTransformRules{
	{
		Type: api.NetworkAddKubernetes,
		Kubernetes: &api.K8sRule{
			Input:    "SrcAddr",
			MacInput: "SrcMAC",
			Output:   "SrcK8s",
			AddZone:  true,
		},
	},
	{
		Type: api.NetworkAddKubernetes,
		Kubernetes: &api.K8sRule{
			Input:    "DstAddr",
			MacInput: "DstMAC",
			Output:   "DstK8s",
			AddZone:  true,
		},
	},
}

func TestEnrich(t *testing.T) {
	informers = inf.SetupStubs(ipInfo, macInfo, nodes)

	// Pod to unknown
	entry := config.GenericMap{
		"SrcAddr": "10.0.0.1",    // pod-1
		"DstAddr": "42.42.42.42", // unknown
	}
	for _, r := range rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstAddr":          "42.42.42.42",
		"SrcAddr":          "10.0.0.1",
		"SrcK8s_HostIP":    "100.0.0.1",
		"SrcK8s_HostName":  "host-1",
		"SrcK8s_Name":      "pod-1",
		"SrcK8s_Namespace": "ns-1",
		"SrcK8s_OwnerName": "",
		"SrcK8s_OwnerType": "",
		"SrcK8s_Type":      "Pod",
		"SrcK8s_Zone":      "us-east-1a",
	}, entry)

	// Pod to pod
	entry = config.GenericMap{
		"SrcAddr": "10.0.0.1", // pod-1
		"DstAddr": "10.0.0.2", // pod-2
	}
	for _, r := range rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstAddr":          "10.0.0.2",
		"DstK8s_HostIP":    "100.0.0.2",
		"DstK8s_HostName":  "host-2",
		"DstK8s_Name":      "pod-2",
		"DstK8s_Namespace": "ns-2",
		"DstK8s_OwnerName": "",
		"DstK8s_OwnerType": "",
		"DstK8s_Type":      "Pod",
		"DstK8s_Zone":      "us-east-1b",
		"SrcAddr":          "10.0.0.1",
		"SrcK8s_HostIP":    "100.0.0.1",
		"SrcK8s_HostName":  "host-1",
		"SrcK8s_Name":      "pod-1",
		"SrcK8s_Namespace": "ns-1",
		"SrcK8s_OwnerName": "",
		"SrcK8s_OwnerType": "",
		"SrcK8s_Type":      "Pod",
		"SrcK8s_Zone":      "us-east-1a",
	}, entry)

	// Pod to service
	entry = config.GenericMap{
		"SrcAddr": "10.0.0.2", // pod-2
		"DstAddr": "20.0.0.1", // service-1
	}
	for _, r := range rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstAddr":          "20.0.0.1",
		"DstK8s_Name":      "service-1",
		"DstK8s_Namespace": "ns-1",
		"DstK8s_OwnerName": "",
		"DstK8s_OwnerType": "",
		"DstK8s_Type":      "Service",
		"SrcAddr":          "10.0.0.2",
		"SrcK8s_HostIP":    "100.0.0.2",
		"SrcK8s_HostName":  "host-2",
		"SrcK8s_Name":      "pod-2",
		"SrcK8s_Namespace": "ns-2",
		"SrcK8s_OwnerName": "",
		"SrcK8s_OwnerType": "",
		"SrcK8s_Type":      "Pod",
		"SrcK8s_Zone":      "us-east-1b",
	}, entry)
}

var otelRules = api.NetworkTransformRules{
	{
		Type: api.NetworkAddKubernetes,
		Kubernetes: &api.K8sRule{
			Input:    "source.ip",
			Output:   "source.",
			Assignee: "otel",
			AddZone:  true,
		},
	},
	{
		Type: api.NetworkAddKubernetes,
		Kubernetes: &api.K8sRule{
			Input:    "destination.ip",
			Output:   "destination.",
			Assignee: "otel",
			AddZone:  true,
		},
	},
}

func TestEnrich_Otel(t *testing.T) {
	informers = inf.SetupStubs(ipInfo, macInfo, nodes)

	// Pod to unknown
	entry := config.GenericMap{
		"source.ip":      "10.0.0.1",    // pod-1
		"destination.ip": "42.42.42.42", // unknown
	}
	for _, r := range otelRules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"destination.ip":            "42.42.42.42",
		"source.ip":                 "10.0.0.1",
		"source.k8s.host.ip":        "100.0.0.1",
		"source.k8s.host.name":      "host-1",
		"source.k8s.name":           "pod-1",
		"source.k8s.namespace.name": "ns-1",
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
	for _, r := range otelRules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"destination.ip":                 "10.0.0.2",
		"destination.k8s.host.ip":        "100.0.0.2",
		"destination.k8s.host.name":      "host-2",
		"destination.k8s.name":           "pod-2",
		"destination.k8s.namespace.name": "ns-2",
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
	for _, r := range otelRules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"destination.ip":                 "20.0.0.1",
		"destination.k8s.name":           "service-1",
		"destination.k8s.namespace.name": "ns-1",
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
		"source.k8s.pod.name":            "pod-2",
		"source.k8s.pod.uid":             types.UID(""),
		"source.k8s.owner.name":          "",
		"source.k8s.owner.type":          "",
		"source.k8s.type":                "Pod",
		"source.k8s.zone":                "us-east-1b",
	}, entry)
}

func TestEnrich_EmptyNamespace(t *testing.T) {
	informers = inf.SetupStubs(ipInfo, macInfo, nodes)

	// We need to check that, whether it returns NotFound or just an empty namespace,
	// there is no map entry for that namespace (an empty-valued map entry is not valid)
	entry := config.GenericMap{
		"SrcAddr": "1.2.3.4", // would return an empty namespace
		"DstAddr": "3.2.1.0", // would return NotFound
	}

	for _, r := range rules {
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
	informers = inf.SetupStubs(ipInfo, macInfo, nodes)

	// Pod to unknown using MAC
	entry := config.GenericMap{
		"SrcMAC": "AA:BB:CC:DD:EE:FF", // pod-1
		"DstMAC": "GG:HH:II:JJ:KK:LL", // unknown
	}
	for _, r := range rules {
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstMAC":           "GG:HH:II:JJ:KK:LL",
		"SrcMAC":           "AA:BB:CC:DD:EE:FF",
		"SrcK8s_HostIP":    "100.0.0.1",
		"SrcK8s_HostName":  "host-1",
		"SrcK8s_Name":      "pod-1",
		"SrcK8s_Namespace": "ns-1",
		"SrcK8s_OwnerName": "",
		"SrcK8s_OwnerType": "",
		"SrcK8s_Type":      "Pod",
		"SrcK8s_Zone":      "us-east-1a",
	}, entry)

	// remove the MAC rules and retry
	entry = config.GenericMap{
		"SrcMAC": "AA:BB:CC:DD:EE:FF", // pod-1
		"DstMAC": "GG:HH:II:JJ:KK:LL", // unknown
	}
	for _, r := range rules {
		r.Kubernetes.MacInput = ""
		Enrich(entry, r.Kubernetes)
	}
	assert.Equal(t, config.GenericMap{
		"DstMAC": "GG:HH:II:JJ:KK:LL",
		"SrcMAC": "AA:BB:CC:DD:EE:FF",
	}, entry)
}
