package pbflow

import (
	"encoding/binary"
	"net"

	"github.com/netobserv/netobserv-ebpf-agent/pkg/ebpf"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/flow"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FlowsToPB is an auxiliary function to convert flow records, as returned by the eBPF agent,
// into protobuf-encoded messages ready to be sent to the collector via GRPC
func FlowsToPB(inputRecords []*flow.Record, maxLen int) []*Records {
	entries := make([]*Record, 0, len(inputRecords))
	for _, record := range inputRecords {
		entries = append(entries, FlowToPB(record))
	}
	var records []*Records
	for len(entries) > 0 {
		end := len(entries)
		if end > maxLen {
			end = maxLen
		}
		records = append(records, &Records{Entries: entries[:end]})
		entries = entries[end:]
	}
	return records
}

// FlowToPB is an auxiliary function to convert a single flow record, as returned by the eBPF agent,
// into a protobuf-encoded message ready to be sent to the collector via kafka
func FlowToPB(fr *flow.Record) *Record {
	var pbflowRecord = Record{
		EthProtocol: uint32(fr.Id.EthProtocol),
		Direction:   Direction(fr.Id.Direction),
		DataLink: &DataLink{
			SrcMac: macToUint64(&fr.Id.SrcMac),
			DstMac: macToUint64(&fr.Id.DstMac),
		},
		Network: &Network{
			Dscp: uint32(fr.Metrics.Dscp),
		},
		Transport: &Transport{
			Protocol: uint32(fr.Id.TransportProtocol),
			SrcPort:  uint32(fr.Id.SrcPort),
			DstPort:  uint32(fr.Id.DstPort),
		},
		IcmpType: uint32(fr.Id.IcmpType),
		IcmpCode: uint32(fr.Id.IcmpCode),
		Bytes:    fr.Metrics.Bytes,
		TimeFlowStart: &timestamppb.Timestamp{
			Seconds: fr.TimeFlowStart.Unix(),
			Nanos:   int32(fr.TimeFlowStart.Nanosecond()),
		},
		TimeFlowEnd: &timestamppb.Timestamp{
			Seconds: fr.TimeFlowEnd.Unix(),
			Nanos:   int32(fr.TimeFlowEnd.Nanosecond()),
		},
		Packets:                uint64(fr.Metrics.Packets),
		Duplicate:              fr.Duplicate,
		AgentIp:                agentIP(fr.AgentIP),
		Flags:                  uint32(fr.Metrics.Flags),
		Interface:              fr.Interface,
		PktDropBytes:           fr.Metrics.PktDrops.Bytes,
		PktDropPackets:         uint64(fr.Metrics.PktDrops.Packets),
		PktDropLatestFlags:     uint32(fr.Metrics.PktDrops.LatestFlags),
		PktDropLatestState:     uint32(fr.Metrics.PktDrops.LatestState),
		PktDropLatestDropCause: fr.Metrics.PktDrops.LatestDropCause,
		DnsId:                  uint32(fr.Metrics.DnsRecord.Id),
		DnsFlags:               uint32(fr.Metrics.DnsRecord.Flags),
		DnsErrno:               uint32(fr.Metrics.DnsRecord.Errno),
		TimeFlowRtt:            durationpb.New(fr.TimeFlowRtt),
	}
	if fr.Metrics.DnsRecord.Latency != 0 {
		pbflowRecord.DnsLatency = durationpb.New(fr.DNSLatency)
	}
	if len(fr.DupList) != 0 {
		pbflowRecord.DupList = make([]*DupMapEntry, 0)
		for _, m := range fr.DupList {
			for key, value := range m {
				pbflowRecord.DupList = append(pbflowRecord.DupList, &DupMapEntry{
					Interface: key,
					Direction: Direction(value),
				})
			}
		}
	}
	if fr.Id.EthProtocol == flow.IPv6Type {
		pbflowRecord.Network.SrcAddr = &IP{IpFamily: &IP_Ipv6{Ipv6: fr.Id.SrcIp[:]}}
		pbflowRecord.Network.DstAddr = &IP{IpFamily: &IP_Ipv6{Ipv6: fr.Id.DstIp[:]}}
	} else {
		pbflowRecord.Network.SrcAddr = &IP{IpFamily: &IP_Ipv4{Ipv4: flow.IntEncodeV4(fr.Id.SrcIp)}}
		pbflowRecord.Network.DstAddr = &IP{IpFamily: &IP_Ipv4{Ipv4: flow.IntEncodeV4(fr.Id.DstIp)}}
	}
	return &pbflowRecord
}

func PBToFlow(pb *Record) *flow.Record {
	if pb == nil {
		return nil
	}
	out := flow.Record{
		RawRecord: flow.RawRecord{
			Id: ebpf.BpfFlowId{
				Direction:         uint8(pb.Direction),
				EthProtocol:       uint16(pb.EthProtocol),
				TransportProtocol: uint8(pb.Transport.Protocol),
				SrcMac:            macToUint8(pb.DataLink.GetSrcMac()),
				DstMac:            macToUint8(pb.DataLink.GetDstMac()),
				SrcIp:             ipToIPAddr(pb.Network.GetSrcAddr()),
				DstIp:             ipToIPAddr(pb.Network.GetDstAddr()),
				SrcPort:           uint16(pb.Transport.SrcPort),
				DstPort:           uint16(pb.Transport.DstPort),
				IcmpType:          uint8(pb.IcmpType),
				IcmpCode:          uint8(pb.IcmpCode),
			},
			Metrics: ebpf.BpfFlowMetrics{
				Bytes:   pb.Bytes,
				Packets: uint32(pb.Packets),
				Flags:   uint16(pb.Flags),
				Dscp:    uint8(pb.Network.Dscp),
				PktDrops: ebpf.BpfPktDropsT{
					Bytes:           pb.PktDropBytes,
					Packets:         uint32(pb.PktDropPackets),
					LatestFlags:     uint16(pb.PktDropLatestFlags),
					LatestState:     uint8(pb.PktDropLatestState),
					LatestDropCause: pb.PktDropLatestDropCause,
				},
				DnsRecord: ebpf.BpfDnsRecordT{
					Id:      uint16(pb.DnsId),
					Flags:   uint16(pb.DnsFlags),
					Errno:   uint8(pb.DnsErrno),
					Latency: uint64(pb.DnsLatency.AsDuration()),
				},
			},
		},
		TimeFlowStart: pb.TimeFlowStart.AsTime(),
		TimeFlowEnd:   pb.TimeFlowEnd.AsTime(),
		AgentIP:       pbIPToNetIP(pb.AgentIp),
		Duplicate:     pb.Duplicate,
		Interface:     pb.Interface,
		TimeFlowRtt:   pb.TimeFlowRtt.AsDuration(),
		DNSLatency:    pb.DnsLatency.AsDuration(),
	}

	if len(pb.GetDupList()) != 0 {
		for _, entry := range pb.GetDupList() {
			intf := entry.Interface
			dir := uint8(entry.Direction)
			out.DupList = append(out.DupList, map[string]uint8{intf: dir})
		}
	}
	return &out
}

// Mac bytes are encoded in the same order as in the array. This is, a Mac
// like 11:22:33:44:55:66 will be encoded as 0x112233445566
func macToUint64(m *[flow.MacLen]uint8) uint64 {
	return uint64(m[5]) |
		(uint64(m[4]) << 8) |
		(uint64(m[3]) << 16) |
		(uint64(m[2]) << 24) |
		(uint64(m[1]) << 32) |
		(uint64(m[0]) << 40)
}

func agentIP(nip net.IP) *IP {
	if ip := nip.To4(); ip != nil {
		return &IP{IpFamily: &IP_Ipv4{Ipv4: binary.BigEndian.Uint32(ip)}}
	}
	// IPv6 address
	return &IP{IpFamily: &IP_Ipv6{Ipv6: nip}}
}

func pbIPToNetIP(ip *IP) net.IP {
	if ip.GetIpv6() != nil {
		return net.IP(ip.GetIpv6())
	}
	n := ip.GetIpv4()
	return net.IPv4(
		byte((n>>24)&0xFF),
		byte((n>>16)&0xFF),
		byte((n>>8)&0xFF),
		byte(n&0xFF))
}

func ipToIPAddr(ip *IP) flow.IPAddr {
	return flow.IPAddrFromNetIP(pbIPToNetIP(ip))
}

func macToUint8(mac uint64) [6]uint8 {
	return [6]uint8{
		uint8(mac >> 40),
		uint8(mac >> 32),
		uint8(mac >> 24),
		uint8(mac >> 16),
		uint8(mac >> 8),
		uint8(mac),
	}
}
