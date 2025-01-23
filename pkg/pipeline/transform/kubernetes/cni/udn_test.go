package cni

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	udnHandler = UDNHandler{}
	udnPod     = v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
	udnConfig  = `{"default":{"ip_addresses":["10.128.2.20/23"],"mac_address":"0a:58:0a:80:02:14","routes":[{"dest":"10.128.0.0/14","nextHop":"10.128.2.1"},{"dest":"100.64.0.0/16","nextHop":"10.128.2.1"}],"ip_address":"10.128.2.20/23","role":"infrastructure-locked"},
	"mesh-arena/primary-udn":{"ip_addresses":["10.200.200.12/24"],"mac_address":"0a:58:0a:c8:c8:0c","gateway_ips":["10.200.200.1"],"routes":[{"dest":"172.30.0.0/16","nextHop":"10.200.200.1"},{"dest":"100.65.0.0/16","nextHop":"10.200.200.1"}],"ip_address":"10.200.200.12/24","gateway_ip":"10.200.200.1","tunnel_id":16,"role":"primary"}}`
)

func TestExtractUDNStatusKeys(t *testing.T) {
	// Annotation not found => no error, no key
	keys, err := udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Empty(t, keys)

	// Valid annotation => no error, key
	udnPod.Annotations = map[string]string{
		ovnAnnotation: udnConfig,
	}
	keys, err = udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Equal(t, []string{"mesh-arena/primary-udn~10.200.200.12"}, keys)
}
