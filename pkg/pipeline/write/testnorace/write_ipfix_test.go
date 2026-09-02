package testnorace

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/decode"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmware/go-ipfix/pkg/collector"
	"github.com/vmware/go-ipfix/pkg/entities"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	startTime  = time.Now()
	endTime    = startTime.Add(7 * time.Second)
	fullPBFlow = pbflow.Record{
		Sampling:  10,
		Direction: pbflow.Direction_EGRESS,
		Bytes:     1024,
		DataLink: &pbflow.DataLink{
			DstMac: 0x112233445566,
			SrcMac: 0x010203040506,
		},
		Network: &pbflow.Network{
			SrcAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
			},
			DstAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
			},
			Dscp: 1,
		},
		Duplicate:   false,
		EthProtocol: 2048,
		Packets:     3,
		Transport: &pbflow.Transport{
			Protocol: 6,
			SrcPort:  23000,
			DstPort:  443,
		},
		TimeFlowStart: timestamppb.New(startTime),
		TimeFlowEnd:   timestamppb.New(endTime),

		AgentIp: &pbflow.IP{
			IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x0a090807},
		},
		PktDropBytes:           15,
		PktDropPackets:         1,
		PktDropLatestFlags:     1,
		PktDropLatestState:     1,
		PktDropLatestDropCause: 5,
		Flags:                  0x110,
		DnsId:                  123,
		DnsFlags:               0x80,
		DnsErrno:               0,
		DnsLatency:             durationpb.New(150 * time.Millisecond),
		TimeFlowRtt:            durationpb.New(20 * time.Millisecond),
		IcmpCode:               0,
		IcmpType:               0,
		DupList: []*pbflow.DupMapEntry{
			{
				Interface: "eth0",
				Direction: pbflow.Direction_EGRESS,
			},
			{
				Interface: "a1234567",
				Direction: pbflow.Direction_INGRESS,
			},
		},
		Xlat: &pbflow.Xlat{
			SrcAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x02030405},
			},
			DstAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x06070809},
			},
			SrcPort: 888,
			DstPort: 889,
		},
	}

	icmpPBFlow = pbflow.Record{
		Direction: pbflow.Direction_INGRESS,
		Bytes:     1024,
		DataLink: &pbflow.DataLink{
			DstMac: 0x112233445566,
			SrcMac: 0x010203040506,
		},
		Network: &pbflow.Network{
			SrcAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
			},
			DstAddr: &pbflow.IP{
				IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
			},
		},
		EthProtocol: 2048,
		Packets:     3,
		Transport: &pbflow.Transport{
			Protocol: 1,
		},
		TimeFlowStart: timestamppb.New(startTime),
		TimeFlowEnd:   timestamppb.New(endTime),

		AgentIp: &pbflow.IP{
			IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x0a090807},
		},
		Flags:    0x110,
		IcmpCode: 10,
		IcmpType: 8,
		DupList: []*pbflow.DupMapEntry{
			{
				Interface: "eth0",
				Direction: pbflow.Direction_EGRESS,
			},
			{
				Interface: "a1234567",
				Direction: pbflow.Direction_INGRESS,
			},
		},
	}
)

func TestEnrichedIPFIXFlow(t *testing.T) {
	cp := startCollector(t)

	flow := decode.PBFlowToMap(&fullPBFlow)

	// Convert TCP flags
	flow["Flags"] = utils.DecodeTCPFlags(uint(fullPBFlow.Flags))

	// Add enrichment
	flow["SrcK8S_Name"] = "pod A"
	flow["SrcK8S_Namespace"] = "ns1"
	flow["SrcK8S_HostName"] = "node1"
	flow["DstK8S_Name"] = "pod B"
	flow["DstK8S_Namespace"] = "ns2"
	flow["DstK8S_HostName"] = "node2"

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost:   cp.udpAddr().IP.String(),
				TargetPort:   cp.udpAddr().Port,
				Transport:    cp.udpAddr().Network(),
				EnterpriseID: 9999,
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	// 2nd = IPv6 template (ignore)
	_, err = cp.read(5 * time.Second)
	require.NoError(t, err)
	// 3rd = data record
	dataMsg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	cp.Stop()

	expectedFields := write.IPv4IANAFields
	for _, f := range write.KubeFields {
		expectedFields = append(expectedFields, f.Name)
	}
	for _, f := range write.CustomNetworkFields {
		expectedFields = append(expectedFields, f.Name)
	}
	for _, f := range write.CustomNetworkFieldsV4 {
		expectedFields = append(expectedFields, f.Name)
	}

	// Check template
	assert.Equal(t, uint16(10), tplv4Msg.GetVersion())
	templateSet := tplv4Msg.GetSet()
	templateElements := templateSet.GetRecords()[0].GetOrderedElementList()
	assert.Len(t, templateElements, len(expectedFields))
	assert.Equal(t, uint32(0), templateElements[0].GetInfoElement().EnterpriseId)

	// Check data
	assert.Equal(t, uint16(10), dataMsg.GetVersion())
	dataSet := dataMsg.GetSet()
	record := dataSet.GetRecords()[0]

	for _, name := range expectedFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Truef(t, exist, "element with name %s should exist in the record", name)
		assert.NotNil(t, element)
		matchElement(t, element, flow)
	}
}

func TestIPv6IPFIXFlow(t *testing.T) {
	cp := startCollector(t)

	flow := decode.PBFlowToMap(&fullPBFlow)
	// Set as IPv6
	flow["Etype"] = write.IPv6Type
	flow["SrcAddr"] = "2001:db8::1111"
	flow["DstAddr"] = "2001:db8::2222"
	flow["XlatSrcAddr"] = "2001:db8::3333"
	flow["XlatDstAddr"] = "2001:db8::4444"

	// Convert TCP flags
	flow["Flags"] = utils.DecodeTCPFlags(uint(fullPBFlow.Flags))

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost:   cp.udpAddr().IP.String(),
				TargetPort:   cp.udpAddr().Port,
				Transport:    cp.udpAddr().Network(),
				EnterpriseID: 9999,
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	_, err = cp.read(5 * time.Second)
	require.NoError(t, err)
	// 2nd = IPv6 template
	tplv6Msg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	// 3rd = data record
	dataMsg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	cp.Stop()

	expectedFields := write.IPv6IANAFields
	for _, f := range write.KubeFields {
		expectedFields = append(expectedFields, f.Name)
	}
	for _, f := range write.CustomNetworkFields {
		expectedFields = append(expectedFields, f.Name)
	}
	for _, f := range write.CustomNetworkFieldsV6 {
		expectedFields = append(expectedFields, f.Name)
	}

	// Check template
	assert.Equal(t, uint16(10), tplv6Msg.GetVersion())
	templateSet := tplv6Msg.GetSet()
	templateElements := templateSet.GetRecords()[0].GetOrderedElementList()
	assert.Len(t, templateElements, len(expectedFields))
	assert.Equal(t, uint32(0), templateElements[0].GetInfoElement().EnterpriseId)

	// Check data
	assert.Equal(t, uint16(10), dataMsg.GetVersion())
	dataSet := dataMsg.GetSet()
	record := dataSet.GetRecords()[0]

	for _, name := range expectedFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Truef(t, exist, "element with name %s should exist in the record", name)
		assert.NotNil(t, element)
		matchElement(t, element, flow)
	}
}

func TestEnrichedIPFIXPartialFlow(t *testing.T) {
	cp := startCollector(t)

	flow := decode.PBFlowToMap(&fullPBFlow)

	// Add partial enrichment
	flow["SrcK8S_Name"] = "pod A"
	flow["SrcK8S_Namespace"] = "ns1"
	flow["SrcK8S_HostName"] = "node1"

	// Remove a field
	delete(flow, "TimeFlowRttNs")

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost:   cp.udpAddr().IP.String(),
				TargetPort:   cp.udpAddr().Port,
				Transport:    cp.udpAddr().Network(),
				EnterpriseID: 9999,
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	// 2nd = IPv6 template (ignore)
	_, err = cp.read(5 * time.Second)
	require.NoError(t, err)
	// 3rd = data record
	dataMsg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	cp.Stop()

	expectedFields := write.IPv4IANAFields
	for _, f := range write.KubeFields {
		expectedFields = append(expectedFields, f.Name)
	}
	for _, f := range write.CustomNetworkFields {
		expectedFields = append(expectedFields, f.Name)
	}
	for _, f := range write.CustomNetworkFieldsV4 {
		expectedFields = append(expectedFields, f.Name)
	}

	// Check template
	assert.Equal(t, uint16(10), tplv4Msg.GetVersion())
	templateSet := tplv4Msg.GetSet()
	templateElements := templateSet.GetRecords()[0].GetOrderedElementList()
	assert.Len(t, templateElements, len(expectedFields))
	assert.Equal(t, uint32(0), templateElements[0].GetInfoElement().EnterpriseId)

	// Check data
	assert.Equal(t, uint16(10), dataMsg.GetVersion())
	dataSet := dataMsg.GetSet()
	record := dataSet.GetRecords()[0]

	for _, name := range expectedFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Truef(t, exist, "element with name %s should exist in the record", name)
		assert.NotNil(t, element)
		matchElement(t, element, flow)
	}
}

func TestBasicIPFIXFlow(t *testing.T) {
	cp := startCollector(t)

	flow := decode.PBFlowToMap(&fullPBFlow)

	// Add partial enrichment (must be ignored)
	flow["SrcK8S_Name"] = "pod A"
	flow["SrcK8S_Namespace"] = "ns1"
	flow["SrcK8S_HostName"] = "node1"

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost: cp.udpAddr().IP.String(),
				TargetPort: cp.udpAddr().Port,
				Transport:  cp.udpAddr().Network(),
				// No enterprise ID here
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	// 2nd = IPv6 template (ignore)
	_, err = cp.read(5 * time.Second)
	require.NoError(t, err)
	// 3rd = data record
	dataMsg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	cp.Stop()

	// Check template
	assert.Equal(t, uint16(10), tplv4Msg.GetVersion())
	templateSet := tplv4Msg.GetSet()
	templateElements := templateSet.GetRecords()[0].GetOrderedElementList()
	assert.Len(t, templateElements, len(write.IPv4IANAFields))
	assert.Equal(t, uint32(0), templateElements[0].GetInfoElement().EnterpriseId)

	// Check data
	assert.Equal(t, uint16(10), dataMsg.GetVersion())
	dataSet := dataMsg.GetSet()
	record := dataSet.GetRecords()[0]

	for _, name := range write.IPv4IANAFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Truef(t, exist, "element with name %s should exist in the record", name)
		assert.NotNil(t, element)
		matchElement(t, element, flow)
	}

	// Make sure enriched fields are absent
	for _, f := range write.KubeFields {
		element, _, exist := record.GetInfoElementWithValue(f.Name)
		assert.Falsef(t, exist, "element with name %s should NOT exist in the record", f.Name)
		assert.Nil(t, element)
	}
}

func TestICMPIPFIXFlow(t *testing.T) {
	cp := startCollector(t)

	flow := decode.PBFlowToMap(&icmpPBFlow)

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost: cp.udpAddr().IP.String(),
				TargetPort: cp.udpAddr().Port,
				Transport:  cp.udpAddr().Network(),
				// No enterprise ID here
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	// 2nd = IPv6 template (ignore)
	_, err = cp.read(5 * time.Second)
	require.NoError(t, err)
	// 3rd = data record
	dataMsg, err := cp.read(5 * time.Second)
	require.NoError(t, err)
	cp.Stop()

	// Check template
	assert.Equal(t, uint16(10), tplv4Msg.GetVersion())
	templateSet := tplv4Msg.GetSet()
	templateElements := templateSet.GetRecords()[0].GetOrderedElementList()
	assert.Len(t, templateElements, len(write.IPv4IANAFields))
	assert.Equal(t, uint32(0), templateElements[0].GetInfoElement().EnterpriseId)

	// Check data
	assert.Equal(t, uint16(10), dataMsg.GetVersion())
	dataSet := dataMsg.GetSet()
	record := dataSet.GetRecords()[0]

	for _, name := range write.IPv4IANAFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Truef(t, exist, "element with name %s should exist in the record", name)
		assert.NotNil(t, element)
		matchElement(t, element, flow)
	}

	// Make sure enriched fields are absent
	for _, f := range write.KubeFields {
		element, _, exist := record.GetInfoElementWithValue(f.Name)
		assert.Falsef(t, exist, "element with name %s should NOT exist in the record", f.Name)
		assert.Nil(t, element)
	}
}

func matchElement(t *testing.T, element entities.InfoElementWithValue, flow config.GenericMap) {
	name := element.GetName()
	mapping, ok := write.MapIPFIXKeys[name]
	if !ok {
		assert.Fail(t, "missing check on element", name)
		return
	}
	expected := flow[mapping.Key]
	if mapping.Matcher != nil {
		assert.True(t, mapping.Matcher(element, expected), "unexpected "+name)
	} else {
		value := mapping.Getter(element)
		if expected == nil {
			assert.Empty(t, value, "unexpected "+name)
		} else {
			assert.Equal(t, expected, value, "unexpected "+name)
		}
	}
}

type collect struct {
	*collector.CollectingProcess
}

func startCollector(t *testing.T) collect {
	return startCollectorOnAddr(t, "127.0.0.1", 0)
}

func startCollectorOnAddr(t *testing.T, host string, port uint) collect {
	address, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, port))
	require.NoError(t, err)

	cp, err := collector.InitCollectingProcess(collector.CollectorInput{
		Address:       address.String(),
		Protocol:      address.Network(),
		MaxBufferSize: 2048,
		TemplateTTL:   0,
		ServerCert:    nil,
		ServerKey:     nil,
	})
	require.NoError(t, err)

	go cp.Start()

	// Wait for collector to be ready
	checkConn := func(_ context.Context) (bool, error) {
		addr := cp.GetAddress()
		if addr == nil || strings.HasSuffix(addr.String(), ":0") {
			return false, fmt.Errorf("random port is not resolved")
		}
		conn, err := net.Dial(addr.Network(), addr.String())
		if err != nil {
			return false, err
		}
		conn.Close()
		return true, nil
	}
	err = wait.PollUntilContextTimeout(context.TODO(), 100*time.Millisecond, 2*time.Second, false, checkConn)
	require.NoError(t, err, "Connection timeout in collector setup")

	return collect{CollectingProcess: cp}
}

func (c *collect) udpAddr() *net.UDPAddr {
	return c.GetAddress().(*net.UDPAddr)
}

// drain continuously reads from the collector's message channel, returning a
// flag that is set to true once at least one message has been received. The
// goroutine exits by itself when Stop() closes the channel.
func (c *collect) drain() *atomic.Bool {
	received := &atomic.Bool{}
	go func() {
		for range c.GetMsgChan() {
			received.Store(true)
		}
	}()
	return received
}

func (c *collect) read(timeout time.Duration) (*entities.Message, error) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(timeout)
		cancel()
	}()
	select {
	case msg := <-c.GetMsgChan():
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestIngestEnriched(t *testing.T) {
	var pen uint32 = 2
	collectorPort, err := test.UDPPort()
	require.NoError(t, err)
	stage := config.NewIPFIXPipeline("ingest-ipfix", api.IngestIpfix{
		Port:    collectorPort,
		Mapping: generateWriteMapping(pen),
	})
	ic, err := ingest.NewIngestIPFIX(operational.NewMetrics(&config.MetricsSettings{}), stage.GetStageParams()[0])
	require.NoError(t, err)
	forwarded := make(chan config.GenericMap)

	go ic.Ingest(forwarded)

	flow := decode.PBFlowToMap(&fullPBFlow)

	// Convert TCP flags
	flow["Flags"] = utils.DecodeTCPFlags(uint(fullPBFlow.Flags))

	// Add enrichment
	flow["SrcK8S_Name"] = "pod A"
	flow["SrcK8S_Namespace"] = "ns1"
	flow["DstK8S_Name"] = "pod B"
	flow["DstK8S_Namespace"] = "ns2"

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost:   "0.0.0.0",
				TargetPort:   int(collectorPort),
				Transport:    "udp",
				EnterpriseID: int(pen),
			},
		},
	})
	require.NoError(t, err)
	writer.Write(flow)

	// Wait for flow
	for {
		select {
		case received := <-forwarded:
			assert.Equal(t, "1.2.3.4", received["SrcAddr"])
			assert.Equal(t, "127.0.0.1", received["SamplerAddress"])
			assert.Equal(t, []byte("ns1"), received["CustomBytes_1"])
			assert.Equal(t, []byte("pod A"), received["CustomBytes_2"])
			assert.Equal(t, []byte("ns2"), received["CustomBytes_3"])
			assert.Equal(t, []byte("pod B"), received["CustomBytes_4"])
			return
		default:
			// nothing yet received
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func generateWriteMapping(pen uint32) []api.NetFlowMapField {
	var mapping []api.NetFlowMapField
	allCustom := []entities.InfoElement{}
	allCustom = append(allCustom, write.KubeFields...)
	allCustom = append(allCustom, write.CustomNetworkFields...)
	countString := 0
	countOther := 0
	for _, in := range allCustom {
		out := api.NetFlowMapField{
			PenProvided: true,
			Pen:         pen,
			Type:        in.ElementId,
		}
		if in.DataType == entities.String {
			countString++
			out.Destination = fmt.Sprintf("CustomBytes_%d", countString)
		} else {
			countOther++
			out.Destination = fmt.Sprintf("CustomInteger_%d", countOther)
		}
		mapping = append(mapping, out)
	}
	return mapping
}

// TestIPFIXReconnect verifies that, when the collector is addressed by a DNS
// name and it restarts behind a *different* IP (e.g. a Service/Pod that comes
// back with a new address), the exporter re-resolves that name and reconnects.
// See https://github.com/orgs/netobserv/discussions/1649
func TestIPFIXReconnect(t *testing.T) {
	const collectorName = "collector.test."
	const ip1 = "127.0.0.1"
	const ip2 = "127.0.0.2"

	// In-process mocked DNS server allows to simulate an IP change
	srv, err := mockdns.NewServerWithLogger(map[string]mockdns.Zone{
		collectorName: {A: []string{ip1}},
	}, discardLogger{}, false)
	require.NoError(t, err)
	defer srv.Close()
	srv.PatchNet(net.DefaultResolver)
	defer mockdns.UnpatchNet(net.DefaultResolver)

	port, err := test.UDPPort()
	require.NoError(t, err)

	cp := startCollectorOnAddr(t, ip1, port)
	received := cp.drain()

	flow := decode.PBFlowToMap(&fullPBFlow)

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TplSendInterval: api.Duration{Duration: 1 * time.Second},
				TargetHost:      collectorName,
				TargetPort:      int(port),
				Transport:       "udp",
			},
		},
	})
	require.NoError(t, err)

	// Sanity check: the first collector receives data.
	require.Eventually(t, func() bool {
		writer.Write(flow)
		return received.Load()
	}, 5*time.Second, 200*time.Millisecond, "first collector never received data")
	cp.Stop()

	// Restart the collector on a different loopback IP, same port, and repoint the DNS name to it
	cp = startCollectorOnAddr(t, ip2, port)
	received = cp.drain()
	srv.Resolver().Zones = map[string]mockdns.Zone{
		collectorName: {A: []string{ip2}},
	}

	// Make sure the exporter re-Dials on the refresh interval, resolving to 127.0.0.2, and the new collector starts receiving data
	require.Eventually(t, func() bool {
		writer.Write(flow)
		return received.Load()
	}, 8*time.Second, 500*time.Millisecond,
		"collector on the new IP never received data: reconnect after DNS change failed")
	cp.Stop()
}

// discardLogger silences the mockdns server's per-query trace logging.
type discardLogger struct{}

func (discardLogger) Printf(string, ...interface{}) {}
