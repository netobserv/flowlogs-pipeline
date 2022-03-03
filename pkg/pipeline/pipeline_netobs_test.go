package pipeline

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/netobserv"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/prometheus/common/model"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Tests in this file are aimed at validating the Flowlogs Pipeline configuration for its
// usage within the network observability kube enrichment

const timeout = 5000 * time.Second

type testPipeline struct {
	pipe          *Pipeline
	client        *test.IPFIXClient
	lokiFlows     chan map[string]interface{}
	fakeloki      *httptest.Server
	stopInformers chan struct{}
}

func (p *testPipeline) Close() {
	close(p.lokiFlows)
	close(p.stopInformers)
	p.fakeloki.Close()
}

// NetobsTestPipeline reproduces a basic network observability pipeline and returns it.
// It is not created from the global YAML text configuration because
// we need a programmatic way to instantiate a pipeline and inject there the informers
func NetobsTestPipeline(t *testing.T) *testPipeline {
	// STAGE 1: IPFIX ingester
	// Find a free port for the IPFIX collector
	ipfixPort, err := test.UDPPort()
	require.NoError(t, err)
	collector, err := ingest.NewIngestCollector(api.IngestCollector{
		HostName:    "0.0.0.0",
		Port:        ipfixPort,
		BatchMaxLen: 1,
	})
	require.NoError(t, err)

	// STAGE 2: decoding from flow's JSON data to GenericMAP
	decoder, err := decode.NewDecodeJson()
	require.NoError(t, err)

	// STAGE 3: Kube Enricher getting its data from a mocked informer
	informers := netobserv.NewInformers(fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "influxdb-v2",
			Namespace: "default",
			Annotations: map[string]string{
				"anonotation": "true",
			},
		},
		Status: corev1.PodStatus{
			Phase:  corev1.PodRunning,
			PodIP:  "1.2.3.4",
			PodIPs: []corev1.PodIP{{IP: "1.2.3.4"}},
		},
	}))
	stopInformers := make(chan struct{})
	require.NoError(t, informers.Start(stopInformers))
	enricher := &netobserv.Enricher{
		Config: config.KubeEnrich{
			IPFields: map[string]string{"SrcAddr": ""},
		},
		Informers: informers,
	}

	// STAGE 4: Loki exporter writing to a fake loki server that captures all the forwarded data
	lokiFlows := make(chan map[string]interface{}, 256)
	fakeLoki := httptest.NewServer(test.FakeLokiHandler(lokiFlows))
	log.Debugf("fake loki URL: %s", fakeLoki.URL)
	lokiConfig := config.StageParam{
		Name: "loki",
		Write: config.Write{Loki: api.WriteLoki{
			URL:       fakeLoki.URL,
			TenantID:  "foo",
			BatchWait: "1s",
			BatchSize: 1,
			Timeout:   "1s",
			StaticLabels: map[model.LabelName]model.LabelValue{
				"testApp": "network-observability-test",
			},
		}},
	}
	// NewWriteLoki requires this to not fail during initialization.
	// TODO: simplify configuration and make not depending it on global variables
	config.Opt.Parameters = fmt.Sprintf(
		`[{"name":"write1","write":{"type":"loki","loki":{"url":"%s","batchSize":1,`+
			`"staticLabels":{"testApp":"network-observability-test"}}}}]`, fakeLoki.URL)
	config.Parameters = []config.StageParam{lokiConfig}
	lokiExporter, err := write.NewWriteLoki(lokiConfig)
	require.NoError(t, err)

	// Injecting all the components into a builder object, this way
	// we can provide test components
	pipe, err := (&builder{
		createdStages: map[string]interface{}{},
		pipelineEntryMap: map[string]*pipelineEntry{
			"ingest": {stageType: StageIngest, Ingester: collector},
			"decode": {stageType: StageDecode, Decoder: decoder},
			"enrich": {stageType: StageTransform, Transformer: enricher},
			"loki":   {stageType: StageWrite, Writer: lokiExporter},
		},
		configStages: []config.Stage{
			{Name: "loki", Follows: "enrich"},
			{Name: "enrich", Follows: "decode"},
			{Name: "decode", Follows: "ingest"},
		},
	}).build()
	require.NoError(t, err)

	go pipe.Run()

	client, err := test.NewIPFIXClient(ipfixPort)
	require.NoError(t, err)
	return &testPipeline{
		pipe:          pipe,
		client:        client,
		lokiFlows:     lokiFlows,
		fakeloki:      fakeLoki,
		stopInformers: stopInformers,
	}
}

func TestNetobservEndToEnd(t *testing.T) {
	// GIVEN a network observability pipeline
	pipe := NetobsTestPipeline(t)
	defer pipe.Close()

	var flow map[string]interface{}
	// Since the collector starts in background, it might take some time to accept a
	// flow, so we repeat the first submission until it succeeds
	test.Eventually(t, timeout, func(t require.TestingT) {
		require.NoError(t, pipe.client.SendTemplate())

		// WHEN a Pod flow is captured
		require.NoError(t, pipe.client.SendFlow(12345678, "1.2.3.4"))

		// THEN the flow data is forwarded to Loki
		select {
		case flow = <-pipe.lokiFlows:
			return
		case <-time.After(100 * time.Millisecond):
			require.Fail(t, "timeout while waiting for flows")
		}
	})

	assert.Equal(t, "1.2.3.4", flow["SrcAddr"])
	assert.EqualValues(t, 12345678, flow["TimeFlowStart"])
	assert.EqualValues(t, 12345678, flow["TimeFlowEnd"])

	// AND it is decorated with the proper Pod metadata
	assert.Equal(t, "influxdb-v2", flow["Pod"])
	assert.Equal(t, "influxdb-v2", flow["Workload"])
	assert.Equal(t, "Pod", flow["WorkloadKind"])
	assert.Equal(t, "default", flow["Namespace"])

	// AND WHEN a Flow is captured from an unknown Kubernetes entity
	require.NoError(t, pipe.client.SendFlow(12345699, "4.3.2.1"))

	// Some flows from the previous resubmissions might still be in queue. We wait until
	// the last flow is received
	test.Eventually(t, timeout, func(t require.TestingT) {
		select {
		case flow = <-pipe.lokiFlows:
			// THEN the flow data is forwarded anyway
			assert.Equal(t, "4.3.2.1", flow["SrcAddr"])
		case <-time.After(timeout):
			require.Fail(t, "timeout while waiting for flows")
		}
	})

	// THEN it is forwarded anyway
	assert.EqualValues(t, 12345699, flow["TimeFlowStart"])
	assert.EqualValues(t, 12345699, flow["TimeFlowEnd"])

	// BUT without any metadata decoration
	assert.NotContains(t, flow, "Pod")
	assert.NotContains(t, flow, "Workload")
	assert.NotContains(t, flow, "WorkloadKind")
	assert.NotContains(t, flow, "Namespace")
}
