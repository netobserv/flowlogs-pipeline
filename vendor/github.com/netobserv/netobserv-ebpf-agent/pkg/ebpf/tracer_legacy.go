package ebpf

import "github.com/netobserv/netobserv-ebpf-agent/pkg/metrics"

// This file contains legacy implementations kept for old kernels

func (m *FlowFetcher) legacyLookupAndDeleteMap(met *metrics.Metrics) map[BpfFlowId][]BpfFlowMetrics {
	flowMap := m.objects.AggregatedFlows

	iterator := flowMap.Iterate()
	var flows = make(map[BpfFlowId][]BpfFlowMetrics, m.cacheMaxSize)
	var id BpfFlowId
	var metrics []BpfFlowMetrics
	count := 0

	// Deleting while iterating is really bad for performance (like, really!) as it causes seeing multiple times the same key
	// This is solved in >=4.20 kernels with LookupAndDelete
	for iterator.Next(&id, &metrics) {
		count++
		if err := flowMap.Delete(id); err != nil {
			log.WithError(err).WithField("flowId", id).Warnf("couldn't delete flow entry")
			met.Errors.WithErrorName("flow-fetcher-legacy", "CannotDeleteFlows").Inc()
		}
		// We observed that eBFP PerCPU map might insert multiple times the same key in the map
		// (probably due to race conditions) so we need to re-join metrics again at userspace
		flows[id] = append(flows[id], metrics...)
	}
	met.BufferSizeGauge.WithBufferName("hashmap-legacy-total").Set(float64(count))
	met.BufferSizeGauge.WithBufferName("hashmap-legacy-unique").Set(float64(len(flows)))

	return flows
}

func (p *PacketFetcher) legacyLookupAndDeleteMap(met *metrics.Metrics) map[int][]*byte {
	packetMap := p.objects.PacketRecord
	iterator := packetMap.Iterate()
	packets := make(map[int][]*byte, p.cacheMaxSize)

	var id int
	var packet []*byte
	for iterator.Next(&id, &packet) {
		if err := packetMap.Delete(id); err != nil {
			log.WithError(err).WithField("packetID ", id).Warnf("couldn't delete  entry")
			met.Errors.WithErrorName("pkt-fetcher-legacy", "CannotDeleteEntry").Inc()
		}
		packets[id] = append(packets[id], packet...)
	}
	return packets
}
