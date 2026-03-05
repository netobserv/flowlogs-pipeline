package cni

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	multusHandler      = MultusHandler{}
	pod                = v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
	secondaryNetConfig = []api.SecondaryNetwork{{Index: map[string]any{"mac": nil}}}
)

func TestExtractNetStatusKeys(t *testing.T) {
	// Annotation not found => no error, no key
	keys, namedKeys, err := multusHandler.GetPodUniqueKeys(&pod, secondaryNetConfig)
	require.NoError(t, err)
	require.Empty(t, keys)
	require.Empty(t, namedKeys)

	// Annotation malformed => error, no key
	pod.Annotations = map[string]string{statusAnnotation: "whatever"}
	keys, namedKeys, err = multusHandler.GetPodUniqueKeys(&pod, secondaryNetConfig)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot read annotation")
	require.Empty(t, keys)
	require.Empty(t, namedKeys)

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
	keys, namedKeys, err = multusHandler.GetPodUniqueKeys(&pod, secondaryNetConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"~~86:1d:96:ff:55:0d"}, keys)
	require.Equal(t, map[string]string{"~~86:1d:96:ff:55:0d": "macvlan-conf"}, namedKeys)

	// Composed key
	secondaryNetConfig[0].Index = map[string]any{"mac": nil, "ip": nil, "interface": nil}
	keys, namedKeys, err = multusHandler.GetPodUniqueKeys(&pod, secondaryNetConfig)
	require.NoError(t, err)
	require.Equal(t, []string{"net1~192.168.1.205~86:1d:96:ff:55:0d"}, keys)
	require.Equal(t, map[string]string{"net1~192.168.1.205~86:1d:96:ff:55:0d": "macvlan-conf"}, namedKeys)
}
