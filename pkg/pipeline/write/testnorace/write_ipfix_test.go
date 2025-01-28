package testnorace

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
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
	FullPBFlow = pbflow.Record{
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
			Protocol: 17,
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
	}
)

func TestEnrichedIPFIXFlow(t *testing.T) {
	cp := startCollector(t)
	addr := cp.GetAddress().(*net.UDPAddr)

	flow := decode.PBFlowToMap(&FullPBFlow)

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
				TargetHost:   addr.IP.String(),
				TargetPort:   addr.Port,
				Transport:    addr.Network(),
				EnterpriseID: 9999,
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg := <-cp.GetMsgChan()
	// 2nd = IPv6 template (ignore)
	<-cp.GetMsgChan()
	// 3rd = data record
	dataMsg := <-cp.GetMsgChan()
	cp.Stop()

	expectedFields := write.IPv4IANAFields
	expectedFields = append(expectedFields, write.KubeFields...)
	expectedFields = append(expectedFields, write.CustomNetworkFields...)

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

func TestEnrichedIPFIXPartialFlow(t *testing.T) {
	cp := startCollector(t)
	addr := cp.GetAddress().(*net.UDPAddr)

	flow := decode.PBFlowToMap(&FullPBFlow)

	// Add partial enrichment
	flow["SrcK8S_Name"] = "pod A"
	flow["SrcK8S_Namespace"] = "ns1"
	flow["SrcK8S_HostName"] = "node1"

	// Remove a field
	delete(flow, "TimeFlowRttNs")

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost:   addr.IP.String(),
				TargetPort:   addr.Port,
				Transport:    addr.Network(),
				EnterpriseID: 9999,
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg := <-cp.GetMsgChan()
	// 2nd = IPv6 template (ignore)
	<-cp.GetMsgChan()
	// 3rd = data record
	dataMsg := <-cp.GetMsgChan()
	cp.Stop()

	expectedFields := write.IPv4IANAFields
	expectedFields = append(expectedFields, write.KubeFields...)
	expectedFields = append(expectedFields, write.CustomNetworkFields...)

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
	addr := cp.GetAddress().(*net.UDPAddr)

	flow := decode.PBFlowToMap(&FullPBFlow)

	// Add partial enrichment (must be ignored)
	flow["SrcK8S_Name"] = "pod A"
	flow["SrcK8S_Namespace"] = "ns1"
	flow["SrcK8S_HostName"] = "node1"

	writer, err := write.NewWriteIpfix(config.StageParam{
		Write: &config.Write{
			Ipfix: &api.WriteIpfix{
				TargetHost: addr.IP.String(),
				TargetPort: addr.Port,
				Transport:  addr.Network(),
				// No enterprise ID here
			},
		},
	})
	require.NoError(t, err)

	writer.Write(flow)

	// Read collector
	// 1st = IPv4 template
	tplv4Msg := <-cp.GetMsgChan()
	// 2nd = IPv6 template (ignore)
	<-cp.GetMsgChan()
	// 3rd = data record
	dataMsg := <-cp.GetMsgChan()
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
	for _, name := range write.KubeFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Falsef(t, exist, "element with name %s should NOT exist in the record", name)
		assert.Nil(t, element)
	}
}

//nolint:cyclop
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

func startCollector(t *testing.T) *collector.CollectingProcess {
	address, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
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
		conn, err := net.Dial(cp.GetAddress().Network(), cp.GetAddress().String())
		if err != nil {
			return false, err
		}
		conn.Close()
		return true, nil
	}
	err = wait.PollUntilContextTimeout(context.TODO(), 100*time.Millisecond, 2*time.Second, false, checkConn)
	require.NoError(t, err, "Connection timeout in collector setup")

	return cp
}
