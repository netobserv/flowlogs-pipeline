package cni

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var multusHandler = MultusHandler{}

func TestExtractNetStatusKeys(t *testing.T) {
	pod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
	indexes := []api.SecondaryNetwork{{Index: map[string]any{"mac": nil}}}

	// Annotation not found => no error, no key
	keys, namedKeys, err := multusHandler.GetPodUniqueKeys(&pod, indexes)
	require.NoError(t, err)
	assert.Empty(t, keys)
	assert.Empty(t, namedKeys)

	// Annotation malformed => error, no key
	pod.Annotations = map[string]string{statusAnnotation: "whatever"}
	keys, namedKeys, err = multusHandler.GetPodUniqueKeys(&pod, indexes)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot read annotation")
	assert.Empty(t, keys)
	assert.Empty(t, namedKeys)

	// Valid annotation => no error, key
	pod.Annotations = map[string]string{
		statusAnnotation: `
		[{
			"name": "cbr0",
			"ips": [
					"10.244.1.73"
			],
			"default": true,
			"mac": "aa:aa:96:ff:aa:aa",
			"dns": {}
		},{
			"name": "macvlan-conf",
			"interface": "net1",
			"ips": [
					"192.168.1.205"
			],
			"mac": "86:1d:96:ff:55:0d",
			"dns": {}
		}]
		`,
	}
	keys, namedKeys, err = multusHandler.GetPodUniqueKeys(&pod, indexes)
	require.NoError(t, err)
	assert.Equal(t, []string{"~~86:1d:96:ff:55:0d"}, keys)
	assert.Equal(t, map[string]string{"~~86:1d:96:ff:55:0d": "macvlan-conf"}, namedKeys)

	// Composed key
	indexes[0].Index = map[string]any{"mac": nil, "ip": nil, "interface": nil}
	keys, namedKeys, err = multusHandler.GetPodUniqueKeys(&pod, indexes)
	require.NoError(t, err)
	assert.Equal(t, []string{"net1~192.168.1.205~86:1d:96:ff:55:0d"}, keys)
	assert.Equal(t, map[string]string{"net1~192.168.1.205~86:1d:96:ff:55:0d": "macvlan-conf"}, namedKeys)
}

func TestBuildFlowKeysKubeVirt(t *testing.T) {
	indexes := []api.SecondaryNetwork{{Index: map[string]any{"mac": nil, "interface": nil}}}
	ruleConfig := &api.K8sRule{InterfacesField: "Interfaces", MACField: "SrcMAC"}

	// Empty flow => no key
	flow := config.GenericMap{}
	flowKeys := multusHandler.BuildKeys(flow, ruleConfig, indexes)
	assert.Empty(t, flowKeys)

	// Primary network flow => ignore by multus handler
	flow = config.GenericMap{
		"SrcMAC": "aa:aa:96:ff:aa:aa",
	}
	flowKeys = multusHandler.BuildKeys(flow, ruleConfig, indexes)
	assert.Empty(t, flowKeys)

	// Secondary network flow => return keys, including one for kubevirt-style interfaces (tap*)
	flow = config.GenericMap{
		"SrcMAC":     "02:00:00:00:00:01",
		"Interfaces": []string{"eth0", "tapc0f69e19ba2"},
	}
	flowKeys = multusHandler.BuildKeys(flow, ruleConfig, indexes)
	assert.Equal(t, []string{"eth0~~02:00:00:00:00:01", "tapc0f69e19ba2~~02:00:00:00:00:01", "podc0f69e19ba2~~02:00:00:00:00:01"}, flowKeys)
}

func TestBuildFlowKeysKubeVirt_RejectInterfaceOnly(t *testing.T) {
	indexes := []api.SecondaryNetwork{{Index: map[string]any{"interface": nil}}}
	ruleConfig := &api.K8sRule{InterfacesField: "Interfaces", MACField: "SrcMAC"}

	// Secondary network flow => return keys, excluding one for kubevirt-style interfaces (tap*)
	flow := config.GenericMap{
		"SrcMAC":     "02:00:00:00:00:01",
		"Interfaces": []string{"eth0", "tapsomething"},
	}
	flowKeys := multusHandler.BuildKeys(flow, ruleConfig, indexes)
	assert.Equal(t, []string{"eth0~~", "tapsomething~~"}, flowKeys)
}
