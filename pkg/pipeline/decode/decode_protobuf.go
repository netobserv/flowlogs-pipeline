package decode

import (
	"fmt"
	"net"
	"time"

	"github.com/mariomac/pipes/pkg/graph/stage"
	"github.com/mariomac/pipes/pkg/node"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/netobserv-agent/pkg/pbflow"
	"github.com/sirupsen/logrus"
)

var pflog = logrus.WithField("component", "Protobuf")

// Protobuf decodes protobuf flow records definitions, as forwarded by
// ingest.NetObservAgent, into a Generic Map that follows the same naming conventions
// as the IPFIX flows from ingest.IngestCollector
type Protobuf struct {
}

type ProtobufConfig struct {
	stage.Instance
}

func ProtobufProvider(_ ProtobufConfig) node.MiddleFunc[[]interface{}, []config.GenericMap] {
	pb := Protobuf{}
	return func(in <-chan []interface{}, out chan<- []config.GenericMap) {
		for i := range in {
			out <- pb.Decode(i)
		}
	}
}

// Decode decodes input strings to a list of flow entries
func (p *Protobuf) Decode(in []interface{}) []config.GenericMap {
	if len(in) == 0 {
		pflog.Warn("empty input. Skipping")
		return []config.GenericMap{}
	}
	pb, ok := in[0].(*pbflow.Records)
	if !ok {
		pflog.WithField("type", fmt.Sprintf("%T", pb)).
			Warn("expecting input to be *pbflow.Records. Skipping")
	}
	out := make([]config.GenericMap, 0, len(pb.Entries))
	for _, entry := range pb.Entries {
		out = append(out, pbFlowToMap(entry))
	}
	return out
}

func pbFlowToMap(flow *pbflow.Record) config.GenericMap {
	if flow == nil {
		return config.GenericMap{}
	}
	out := config.GenericMap{
		"FlowDirection": int(flow.Direction.Number()),
		"Bytes":         flow.Bytes,
		"SrcAddr":       ipToStr(flow.Network.GetSrcAddr()),
		"DstAddr":       ipToStr(flow.Network.GetDstAddr()),
		"SrcMac":        macToStr(flow.DataLink.GetSrcMac()),
		"DstMac":        macToStr(flow.DataLink.GetDstMac()),
		"SrcPort":       flow.Transport.GetSrcPort(),
		"DstPort":       flow.Transport.GetDstPort(),
		"Etype":         flow.EthProtocol,
		"Packets":       flow.Packets,
		"Proto":         flow.Transport.GetProtocol(),
		"TimeFlowStart": flow.TimeFlowStart.GetSeconds(),
		"TimeFlowEnd":   flow.TimeFlowEnd.GetSeconds(),
		"TimeReceived":  time.Now().Unix(),
		"Interface":     flow.Interface,
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
