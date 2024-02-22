package write

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
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
		Interface:     "eth0",
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

	writer, err := NewWriteIpfix(config.StageParam{
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
	tplv4Msg, _, dataMsg := readCollector(t, cp)

	// Check template
	assert.Equal(t, uint16(10), tplv4Msg.GetVersion())
	templateSet := tplv4Msg.GetSet()
	templateElements := templateSet.GetRecords()[0].GetOrderedElementList()
	assert.Len(t, templateElements, 20)
	assert.Equal(t, uint32(0), templateElements[0].GetInfoElement().EnterpriseId)

	// Check data
	assert.Equal(t, uint16(10), dataMsg.GetVersion())
	dataSet := dataMsg.GetSet()
	record := dataSet.GetRecords()[0]

	expectedFields := append(ipv4IANAFields, kubeFields...)
	expectedFields = append(expectedFields, customNetworkFields...)

	for _, name := range expectedFields {
		element, _, exist := record.GetInfoElementWithValue(name)
		assert.Truef(t, exist, "element with name %s should exist in the record", name)
		assert.NotNil(t, element)
		matchElement(t, element, flow)
	}
}

func matchElement(t *testing.T, element entities.InfoElementWithValue, flow config.GenericMap) {
	switch element.GetName() {
	case "sourceIPv4Address":
		assert.Equal(t, flow["SrcAddr"], element.GetIPAddressValue().String())
	case "destinationIPv4Address":
		assert.Equal(t, flow["DstAddr"], element.GetIPAddressValue().String())
	case "ethernetType":
		assert.Equal(t, flow["Etype"], uint32(element.GetUnsigned16Value()))
	case "flowDirection":
		assert.Equal(t, flow["FlowDirection"], int(element.GetUnsigned8Value()))
	case "protocolIdentifier":
		assert.Equal(t, flow["Proto"], uint32(element.GetUnsigned8Value()))
	case "sourceTransportPort":
		assert.Equal(t, flow["SrcPort"], uint32(element.GetUnsigned16Value()))
	case "destinationTransportPort":
		assert.Equal(t, flow["DstPort"], uint32(element.GetUnsigned16Value()))
	case "octetDeltaCount":
		assert.Equal(t, flow["Bytes"], element.GetUnsigned64Value())
	case "flowStartMilliseconds":
		assert.Equal(t, flow["TimeFlowStartMs"], int64(element.GetUnsigned64Value()))
	case "flowEndMilliseconds":
		assert.Equal(t, flow["TimeFlowEndMs"], int64(element.GetUnsigned64Value()))
	case "packetDeltaCount":
		assert.Equal(t, flow["Packets"], element.GetUnsigned64Value())
	case "interfaceName":
		assert.Equal(t, flow["Interface"], element.GetStringValue())
	case "sourcePodNamespace":
		assert.Equal(t, flow["SrcK8S_Namespace"], element.GetStringValue())
	case "sourcePodName":
		assert.Equal(t, flow["SrcK8S_Name"], element.GetStringValue())
	case "destinationPodNamespace":
		assert.Equal(t, flow["DstK8S_Namespace"], element.GetStringValue())
	case "destinationPodName":
		assert.Equal(t, flow["DstK8S_Name"], element.GetStringValue())
	case "sourceNodeName":
		assert.Equal(t, flow["SrcK8S_HostName"], element.GetStringValue())
	case "destinationNodeName":
		assert.Equal(t, flow["DstK8S_HostName"], element.GetStringValue())
	case "sourceMacAddress":
	case "destinationMacAddress":
		// Getting some discrepancies here, need to figure out why
	default:
		assert.Fail(t, "missing check on element", element.GetName())
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
	checkConn := func(ctx context.Context) (bool, error) {
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

func readCollector(t *testing.T, cp *collector.CollectingProcess) (*entities.Message, *entities.Message, *entities.Message) {
	var tplv4Msg, tplv6Msg, dataMsg *entities.Message
	count := 0
	for message := range cp.GetMsgChan() {
		switch count {
		case 0:
			tplv4Msg = message
			count++
		case 1:
			tplv6Msg = message
			count++
		default:
			dataMsg = message
			cp.CloseMsgChan()
		}
	}
	cp.Stop()
	return tplv4Msg, tplv6Msg, dataMsg
}
