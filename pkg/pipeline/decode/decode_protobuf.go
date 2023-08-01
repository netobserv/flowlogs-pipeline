package decode

import (
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"

	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// Protobuf decodes protobuf flow records definitions, as forwarded by
// ingest.NetObservAgent, into a Generic Map that follows the same naming conventions
// as the IPFIX flows from ingest.IngestCollector
type Protobuf struct {
}

func NewProtobuf() (*Protobuf, error) {
	log.Debugf("entering NewProtobuf")
	return &Protobuf{}, nil
}

// Decode decodes the protobuf raw flows and returns a list of GenericMaps representing all
// the flows there
func (p *Protobuf) Decode(rawFlow []byte) (config.GenericMap, error) {
	record := pbflow.Record{}
	if err := proto.Unmarshal(rawFlow, &record); err != nil {
		return nil, fmt.Errorf("unmarshaling ProtoBuf record: %w", err)
	}
	return PBFlowToMap(&record), nil
}

func PBFlowToMap(flow *pbflow.Record) config.GenericMap {
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
		"Etype":           flow.EthProtocol,
		"Packets":         flow.Packets,
		"Duplicate":       flow.Duplicate,
		"Proto":           flow.Transport.GetProtocol(),
		"TimeFlowStartMs": flow.TimeFlowStart.AsTime().UnixMilli(),
		"TimeFlowEndMs":   flow.TimeFlowEnd.AsTime().UnixMilli(),
		"TimeReceived":    time.Now().Unix(),
		"Interface":       flow.Interface,
		"AgentIP":         ipToStr(flow.AgentIp),
	}

	proto := flow.Transport.GetProtocol()
	if proto == syscall.IPPROTO_ICMP || proto == syscall.IPPROTO_ICMPV6 {
		out["IcmpType"] = flow.GetIcmpType()
		out["IcmpCode"] = flow.GetIcmpCode()
	}

	if proto == syscall.IPPROTO_TCP || proto == syscall.IPPROTO_UDP || proto == syscall.IPPROTO_SCTP {
		if proto == syscall.IPPROTO_TCP {
			out["SrcPort"] = flow.Transport.GetSrcPort()
			out["DstPort"] = flow.Transport.GetDstPort()
			out["Flags"] = flow.Flags
		} else {
			out["SrcPort"] = flow.Transport.GetSrcPort()
			out["DstPort"] = flow.Transport.GetDstPort()
		}
	}

	if flow.GetDnsId() != 0 {
		out["DnsLatencyMs"] = flow.DnsLatency.AsDuration().Milliseconds()
		out["DnsId"] = flow.GetDnsId()
		out["DnsFlags"] = flow.GetDnsFlags()
		out["DnsFlagsResponseCode"] = dnsRcodeToStr(flow.GetDnsFlags() & 0xF)
	}

	if flow.GetTcpDropLatestDropCause() != 0 {
		out["TcpDropBytes"] = flow.TcpDropBytes
		out["TcpDropPackets"] = flow.TcpDropPackets
		out["TcpDropLatestFlags"] = flow.GetTcpDropLatestFlags()
		out["TcpDropLatestState"] = tcpStateToStr(flow.GetTcpDropLatestState())
		out["TcpDropLatestDropCause"] = tcpDropCauseToStr(flow.GetTcpDropLatestDropCause())
	}

	if flow.TimeFlowRtt.AsDuration().Nanoseconds() != 0 {
		out["TimeFlowRttNs"] = flow.TimeFlowRtt.AsDuration().Nanoseconds()
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

// tcpStateToStr is based on kernel TCP state definition
// https://elixir.bootlin.com/linux/v6.3/source/include/net/tcp_states.h#L12
func tcpStateToStr(state uint32) string {
	switch state {
	case 1:
		return "TCP_ESTABLISHED"
	case 2:
		return "TCP_SYN_SENT"
	case 3:
		return "TCP_SYN_RECV"
	case 4:
		return "TCP_FIN_WAIT1"
	case 5:
		return "TCP_FIN_WAIT2"
	case 6:
		return "TCP_CLOSE"
	case 7:
		return "TCP_CLOSE_WAIT"
	case 8:
		return "TCP_LAST_ACK"
	case 9:
		return "TCP_LISTEN"
	case 10:
		return "TCP_CLOSING"
	case 11:
		return "TCP_NEW_SYN_RECV"
	}
	return "TCP_INVALID_STATE"
}

// tcpDropCauseToStr is based on kernel drop cause definition
// https://elixir.bootlin.com/linux/latest/source/include/net/dropreason.h#L88
func tcpDropCauseToStr(dropCause uint32) string {
	switch dropCause {
	case 2:
		return "SKB_DROP_REASON_NOT_SPECIFIED"
	case 3:
		return "SKB_DROP_REASON_NO_SOCKET"
	case 4:
		return "SKB_DROP_REASON_PKT_TOO_SMALL"
	case 5:
		return "SKB_DROP_REASON_TCP_CSUM"
	case 6:
		return "SKB_DROP_REASON_SOCKET_FILTER"
	case 7:
		return "SKB_DROP_REASON_UDP_CSUM"
	case 8:
		return "SKB_DROP_REASON_NETFILTER_DROP"
	case 9:
		return "SKB_DROP_REASON_OTHERHOST"
	case 10:
		return "SKB_DROP_REASON_IP_CSUM"
	case 11:
		return "SKB_DROP_REASON_IP_INHDR"
	case 12:
		return "SKB_DROP_REASON_IP_RPFILTER"
	case 13:
		return "SKB_DROP_REASON_UNICAST_IN_L2_MULTICAST"
	case 14:
		return "SKB_DROP_REASON_XFRM_POLICY"
	case 15:
		return "SKB_DROP_REASON_IP_NOPROTO"
	case 16:
		return "SKB_DROP_REASON_SOCKET_RCVBUFF"
	case 17:
		return "SKB_DROP_REASON_PROTO_MEM"
	case 18:
		return "SKB_DROP_REASON_TCP_MD5NOTFOUND"
	case 19:
		return "SKB_DROP_REASON_TCP_MD5UNEXPECTED"
	case 20:
		return "SKB_DROP_REASON_TCP_MD5FAILURE"
	case 21:
		return "SKB_DROP_REASON_SOCKET_BACKLOG"
	case 22:
		return "SKB_DROP_REASON_TCP_FLAGS"
	case 23:
		return "SKB_DROP_REASON_TCP_ZEROWINDOW"
	case 24:
		return "SKB_DROP_REASON_TCP_OLD_DATA"
	case 25:
		return "SKB_DROP_REASON_TCP_OVERWINDOW"
	case 26:
		return "SKB_DROP_REASON_TCP_OFOMERGE"
	case 27:
		return "SKB_DROP_REASON_TCP_RFC7323_PAWS"
	case 28:
		return "SKB_DROP_REASON_TCP_INVALID_SEQUENCE"
	case 29:
		return "SKB_DROP_REASON_TCP_RESET"
	case 30:
		return "SKB_DROP_REASON_TCP_INVALID_SYN"
	case 31:
		return "SKB_DROP_REASON_TCP_CLOSE"
	case 32:
		return "SKB_DROP_REASON_TCP_FASTOPEN"
	case 33:
		return "SKB_DROP_REASON_TCP_OLD_ACK"
	case 34:
		return "SKB_DROP_REASON_TCP_TOO_OLD_ACK"
	case 35:
		return "SKB_DROP_REASON_TCP_ACK_UNSENT_DATA"
	case 36:
		return "SKB_DROP_REASON_TCP_OFO_QUEUE_PRUNE"
	case 37:
		return "SKB_DROP_REASON_TCP_OFO_DROP"
	case 38:
		return "SKB_DROP_REASON_IP_OUTNOROUTES"
	case 39:
		return "SKB_DROP_REASON_BPF_CGROUP_EGRESS"
	case 40:
		return "SKB_DROP_REASON_IPV6DISABLED"
	case 41:
		return "SKB_DROP_REASON_NEIGH_CREATEFAIL"
	case 42:
		return "SKB_DROP_REASON_NEIGH_FAILED"
	case 43:
		return "SKB_DROP_REASON_NEIGH_QUEUEFULL"
	case 44:
		return "SKB_DROP_REASON_NEIGH_DEAD"
	case 45:
		return "SKB_DROP_REASON_TC_EGRESS"
	case 46:
		return "SKB_DROP_REASON_QDISC_DROP"
	case 47:
		return "SKB_DROP_REASON_CPU_BACKLOG"
	case 48:
		return "SKB_DROP_REASON_XDP"
	case 49:
		return "SKB_DROP_REASON_TC_INGRESS"
	case 50:
		return "SKB_DROP_REASON_UNHANDLED_PROTO"
	case 51:
		return "SKB_DROP_REASON_SKB_CSUM"
	case 52:
		return "SKB_DROP_REASON_SKB_GSO_SEG"
	case 53:
		return "SKB_DROP_REASON_SKB_UCOPY_FAULT"
	case 54:
		return "SKB_DROP_REASON_DEV_HDR"
	case 55:
		return "SKB_DROP_REASON_DEV_READY"
	case 56:
		return "SKB_DROP_REASON_FULL_RING"
	case 57:
		return "SKB_DROP_REASON_NOMEM"
	case 58:
		return "SKB_DROP_REASON_HDR_TRUNC"
	case 59:
		return "SKB_DROP_REASON_TAP_FILTER"
	case 60:
		return "SKB_DROP_REASON_TAP_TXFILTER"
	case 61:
		return "SKB_DROP_REASON_ICMP_CSUM"
	case 62:
		return "SKB_DROP_REASON_INVALID_PROTO"
	case 63:
		return "SKB_DROP_REASON_IP_INADDRERRORS"
	case 64:
		return "SKB_DROP_REASON_IP_INNOROUTES"
	case 65:
		return "SKB_DROP_REASON_PKT_TOO_BIG"
	case 66:
		return "SKB_DROP_REASON_DUP_FRAG"
	case 67:
		return "SKB_DROP_REASON_FRAG_REASM_TIMEOUT"
	case 68:
		return "SKB_DROP_REASON_FRAG_TOO_FAR"
	case 69:
		return "SKB_DROP_REASON_TCP_MINTTL"
	case 70:
		return "SKB_DROP_REASON_IPV6_BAD_EXTHDR"
	case 71:
		return "SKB_DROP_REASON_IPV6_NDISC_FRAG"
	case 72:
		return "SKB_DROP_REASON_IPV6_NDISC_HOP_LIMIT"
	case 73:
		return "SKB_DROP_REASON_IPV6_NDISC_BAD_CODE"
	case 74:
		return "SKB_DROP_REASON_IPV6_NDISC_BAD_OPTIONS"
	case 75:
		return "SKB_DROP_REASON_IPV6_NDISC_NS_OTHERHOST"
	}
	return "SKB_DROP_UNKNOWN_CAUSE"
}

// dnsRcodeToStr decode DNS flags response code bits and return a string
// https://datatracker.ietf.org/doc/html/rfc2929#section-2.3
func dnsRcodeToStr(rcode uint32) string {
	switch rcode {
	case 0:
		return "NoError"
	case 1:
		return "FormErr"
	case 2:
		return "ServFail"
	case 3:
		return "NXDomain"
	case 4:
		return "NotImp"
	case 5:
		return "Refused"
	case 6:
		return "YXDomain"
	case 7:
		return "YXRRSet"
	case 8:
		return "NXRRSet"
	case 9:
		return "NotAuth"
	case 10:
		return "NotZone"
	case 16:
		return "BADVERS"
	case 17:
		return "BADKEY"
	case 18:
		return "BADTIME"
	case 19:
		return "BADMODE"
	case 20:
		return "BADNAME"
	case 21:
		return "BADALG"
	}
	return "UnDefined"
}
