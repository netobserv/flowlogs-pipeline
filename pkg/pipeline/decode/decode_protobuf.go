package decode

import (
	"fmt"
	"net"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

var pflog = logrus.WithField("component", "Protobuf")

// Protobuf decodes protobuf flow records definitions, as forwarded by
// ingest.NetObservAgent, into a Generic Map that follows the same naming conventions
// as the IPFIX flows from ingest.IngestCollector
type Protobuf struct {
}

func NewProtobuf() (*Protobuf, error) {
	return &Protobuf{}, nil
}

// Decode decodes the protobuf raw flows and returns a list of GenericMaps representing all
// the flows there
func (p *Protobuf) Decode(rawFlows [][]byte) []config.GenericMap {
	flows := make([]config.GenericMap, 0, len(rawFlows))
	for _, pbRaw := range rawFlows {
		record := pbflow.Record{}
		if err := proto.Unmarshal(pbRaw, &record); err != nil {
			pflog.WithError(err).Debug("can't unmarshall received protobuf flow. Ignoring")
			continue
		}
		flows = append(flows, pbFlowToMap(&record))
	}
	return flows
}

// PBRecordsAsMaps transform all the flows in a pbflow.Records entry into a slice
// of GenericMaps
func PBRecordsAsMaps(flow *pbflow.Records) []config.GenericMap {
	out := make([]config.GenericMap, 0, len(flow.Entries))
	for _, entry := range flow.Entries {
		out = append(out, pbFlowToMap(entry))
	}
	return out
}

func pbFlowToMap(flow *pbflow.Record) config.GenericMap {
	if flow == nil {
		return config.GenericMap{}
	}
	out := config.GenericMap{
		"FlowDirection":   int(flow.Direction.Number()),
		"Bytes":           flow.Bytes,
		"SrcAddr":         ipToStr(flow.Network.GetSrcAddr()),
		"DstAddr":         ipToStr(flow.Network.GetDstAddr()),
		"SrcMac":          macToStr(flow.DataLink.GetSrcMac()),
		"DstMac":          macToStr(flow.DataLink.GetDstMac()),
		"SrcPort":         flow.Transport.GetSrcPort(),
		"DstPort":         flow.Transport.GetDstPort(),
		"Etype":           flow.EthProtocol,
		"Packets":         flow.Packets,
		"Proto":           flow.Transport.GetProtocol(),
		"TimeFlowStartMs": flow.TimeFlowStart.AsTime().UnixMilli(),
		"TimeFlowEndMs":   flow.TimeFlowEnd.AsTime().UnixMilli(),
		"TimeReceived":    time.Now().Unix(),
		"Interface":       flow.Interface,
	}
	return out
}

func ipToStr(ip *pbflow.IP) string {
	if ip.GetIpv6() != nil {
		return net.IP(ip.GetIpv6()).String()
	} else {
		n := ip.GetIpv4()
		return fmt.Sprintf("%d.%d.%d.%d",
			byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
}

func macToStr(mac uint64) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		uint8(mac>>40),
		uint8(mac>>32),
		uint8(mac>>24),
		uint8(mac>>16),
		uint8(mac>>8),
		uint8(mac))
}
