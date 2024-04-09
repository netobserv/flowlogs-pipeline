package constants

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	// InitContainerImageName allows a custom initContainer image to be used
	InitContainerImageName = GetEnvWithDefault("INITCONTAINER_IMAGE", "registry.access.redhat.com/ubi8-micro")

	// ConfigListenerImageName is the image used by the ConfigListener Deployment
	ConfigListenerImageName = os.Getenv(ConfigListenerEnvName)
	ConfigListenerEnvName   = "CONFIG_LISTENER_IMAGE"

	// JGroupsDiagnosticsFlag is used to enable traces for JGroups
	JGroupsDiagnosticsFlag = GetEnvBool("JGROUPS_DIAGNOSTICS")
	// ThreadDumpPreStopFlag is used to print a server thread dump on container stop
	ThreadDumpPreStopFlag = GetEnvBool("THREAD_DUMP_PRE_STOP")

	// DefaultMemorySize string with default size for memory
	DefaultMemorySize = resource.MustParse("1Gi")

	// DefaultPVSize default size for persistent volume
	DefaultPVSize = resource.MustParse("1Gi")

	DeploymentAnnotations = map[string]string{
		"openshift.io/display-name":      "Infinispan Cluster",
		"openshift.io/documentation-url": "http://infinispan.org/documentation/",
	}

	SystemPodLabels = map[string]bool{
		appsv1.StatefulSetPodNameLabel:  true,
		appsv1.StatefulSetRevisionLabel: true,
		StatefulSetPodLabel:             true,
	}

	JGroupsFastMerge = strings.ToUpper(GetEnvWithDefault("TEST_ENVIRONMENT", "false")) == "TRUE"

	// 14.0.24.Final required to enable expiry in Gossip Router
	GossipRouterHeartBeatMinVersion = semver.Version{
		Major: 14,
		Minor: 0,
		Patch: 24,
	}
)

const (
	// DefaultDeveloperUser users to access the cluster rest API
	DefaultDeveloperUser = "developer"
	// DefaultCacheName default cache name for the CacheService
	DefaultCacheName                        = "default"
	AdminUsernameKey                        = "username"
	AdminPasswordKey                        = "password"
	InfinispanAdminPort                     = 11223
	InfinispanAdminPortName                 = "infinispan-adm"
	InfinispanJmxPort                       = 9999
	InfinispanJmxPortName                   = "jfr-jmx" // Jmx constant required for automatic lookup of JMX endpoint
	InfinispanPingPort                      = 8888
	InfinispanPingPortName                  = "ping"
	InfinispanUserPort                      = 11222
	InfinispanUserPortName                  = "infinispan"
	CrossSitePort                           = 7900
	CrossSitePortName                       = "xsite"
	GossipRouterDiagPort                    = 7500
	StatefulSetPodLabel                     = "app.kubernetes.io/created-by"
	StaticCrossSiteUriSchema                = "infinispan+xsite"
	CacheServiceFixedMemoryXmxMb            = 200
	CacheServiceJvmNativeMb                 = 220
	CacheServiceMinHeapFreeRatio            = 5
	CacheServiceMaxHeapFreeRatio            = 10
	CacheServiceJvmNativePercentageOverhead = 1
	CacheServiceMaxRamMb                    = CacheServiceFixedMemoryXmxMb + CacheServiceJvmNativeMb
	CacheServiceJavaOptions                 = "-Xmx%dM -Xms%dM -XX:MaxRAM=%dM -Dsun.zip.disableMemoryMapping=true -XX:+UseSerialGC -XX:MinHeapFreeRatio=%d -XX:MaxHeapFreeRatio=%d %s"
	CacheServiceNativeJavaOptions           = "-Xmx%dM -Xms%dM -Dsun.zip.disableMemoryMapping=true %s"

	NativeImageMarker             = "native"
	GeneratedSecretSuffix         = "generated-secret"
	InfinispanFinalizer           = "finalizer.infinispan.org"
	ServerEncryptRoot             = "/etc/encrypt"
	ServerEncryptTruststoreRoot   = ServerEncryptRoot + "/truststore"
	ServerEncryptKeystoreRoot     = ServerEncryptRoot + "/keystore"
	SiteTransportKeyStoreRoot     = ServerEncryptRoot + "/transport-site-tls"
	SiteRouterKeyStoreRoot        = ServerEncryptRoot + "/router-site-tls"
	SiteTrustStoreRoot            = ServerEncryptRoot + "/truststore-site-tls"
	ServerSecurityRoot            = "/etc/security"
	ServerIdentitiesFilename      = "identities.yaml"
	CliPropertiesFilename         = "cli.properties"
	ServerIdentitiesBatchFilename = "identities.cli"
	ServerAdminIdentitiesRoot     = ServerSecurityRoot + "/admin"
	ServerUserIdentitiesRoot      = ServerSecurityRoot + "/user"
	ServerOperatorSecurity        = ServerSecurityRoot + "/conf/operator-security"
	ServerRoot                    = "/opt/infinispan/server"

	EncryptTruststoreKey         = "truststore.p12"
	EncryptTruststorePasswordKey = "truststore-password"

	DefaultCacheTemplate = `<infinispan>
		<cache-container>
			<distributed-cache name="%v" mode="SYNC" owners="%d" statistics="true">
				<memory storage="OFF_HEAP" max-count="%d" when-full="REMOVE"/>
				<partition-handling when-split="ALLOW_READ_WRITES" merge-policy="REMOVE_ALL" />
			</distributed-cache>
		</cache-container>
	</infinispan>`
)

const (
	// DefaultMinimumAutoscalePollPeriod minimum period for autoscaler polling loop
	DefaultMinimumAutoscalePollPeriod = 5 * time.Second
	//DefaultWaitOnCluster delay for the Infinispan cluster wait if it not created while Cache creation
	DefaultWaitOnCluster = 10 * time.Second
	// DefaultWaitOnCreateResource delay for wait until resource (Secret, ConfigMap, Service) is created
	DefaultWaitOnCreateResource = 2 * time.Second
	// DefaultLongWaitOnCreateResource delay for wait until non core resource is create (only Grafana CRD atm)
	DefaultLongWaitOnCreateResource = 60 * time.Second
	//DefaultWaitClusterNotWellFormed wait delay until cluster is not well formed
	DefaultWaitClusterNotWellFormed = 15 * time.Second
	// DefaultWaitPodsNotReady wait delay until cluster pods are ready
	DefaultWaitClusterPodsNotReady = 2 * time.Second
)

const DefaultKubeConfig = "~/.kube/config"

const (
	DefaultSiteKeyStoreFileName       = "keystore.p12"
	DefaultSiteTransportKeyStoreAlias = "transport"
	DefaultSiteRouterKeyStoreAlias    = "router"
	DefaultSiteTrustStoreFileName     = "truststore.p12"
)

const (
	AnnotationDomain             = "infinispan.org/"
	ListenerAnnotationGeneration = AnnotationDomain + "listener-generation"
	ListenerAnnotationDelete     = AnnotationDomain + "listener-delete"
	ListenerControllerDelete     = AnnotationDomain + "controller-delete"
)

// GetWithDefault return value if not empty else return defValue
func GetWithDefault(value, defValue string) string {
	if value == "" {
		return defValue
	}
	return value
}

// GetEnvWithDefault return os.Getenv(name) if exists else return defValue
func GetEnvWithDefault(name, defValue string) string {
	return GetWithDefault(os.Getenv(name), defValue)
}

func GetEnvBool(name string) bool {
	env := os.Getenv(name)
	if env == "" {
		return false
	}
	v, err := strconv.ParseBool(env)
	if err != nil {
		panic(fmt.Errorf("unable to parse bool env '%s': %w", name, err))
	}
	return v
}
