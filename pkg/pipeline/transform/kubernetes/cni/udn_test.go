package cni

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	udnHandler = UDNHandler{}
	udnPod     = v1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
)

func udnConfigAnnotation(ip string) string {
	return fmt.Sprintf(`{"default":{"ip_addresses":["10.128.2.20/23"],"mac_address":"0a:58:0a:80:02:14","routes":[{"dest":"10.128.0.0/14","nextHop":"10.128.2.1"},{"dest":"100.64.0.0/16","nextHop":"10.128.2.1"}],"ip_address":"10.128.2.20/23","role":"infrastructure-locked"},
	"mesh-arena/primary-udn":
		{"ip_addresses":["%s"],
		"mac_address":"0a:58:0a:c8:c8:0c",
		"gateway_ips":["10.200.200.1"],
		"routes":[{"dest":"172.30.0.0/16","nextHop":"10.200.200.1"},{"dest":"100.65.0.0/16","nextHop":"10.200.200.1"}],
		"ip_address":"%s",
		"gateway_ip":"10.200.200.1",
		"tunnel_id":16,
		"role":"primary"}}`, ip, ip)
}

func TestExtractUDNStatusKeys(t *testing.T) {
	// Annotation not found => no error, no key
	keys, err := udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Empty(t, keys)

	// Valid annotation => no error, valid key
	udnPod.Annotations = map[string]string{
		ovnAnnotation: udnConfigAnnotation("10.200.200.12"),
	}
	keys, err = udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Equal(t, []string{"mesh-arena/primary-udn~10.200.200.12"}, keys)

	// Same check with a somewhat surprising CIDR found here as an IP, but it's really the IP part that should be used
	udnPod.Annotations = map[string]string{
		ovnAnnotation: udnConfigAnnotation("10.200.200.12/24"),
	}
	keys, err = udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Equal(t, []string{"mesh-arena/primary-udn~10.200.200.12"}, keys)

	// Same with IPv6
	udnPod.Annotations = map[string]string{
		ovnAnnotation: udnConfigAnnotation("2001:0db8::1111"),
	}
	keys, err = udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Equal(t, []string{"mesh-arena/primary-udn~2001:0db8::1111"}, keys)

	// Same with IPv6 as a CIDR
	udnPod.Annotations = map[string]string{
		ovnAnnotation: udnConfigAnnotation("2001:0db8::1111/24"),
	}
	keys, err = udnHandler.GetPodUniqueKeys(&udnPod)
	require.NoError(t, err)
	require.Equal(t, []string{"mesh-arena/primary-udn~2001:0db8::1111"}, keys)
}
