package ingest_decode

import (
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/netobserv-agent/pkg/grpc"
	"github.com/netobserv/netobserv-agent/pkg/pbflow"
)

const defaultBufferLen = 100

// GRPCProtobuf ingests data from the NetObserv eBPF Agent, using Protocol Buffers over gRPC
type GRPCProtobuf struct {
	collector   *grpc.CollectorServer
	flowPackets chan *pbflow.Records
	decoder     *decode.Protobuf
}

func NewGRPCProtobuf(params config.StageParam) (*GRPCProtobuf, error) {
	netObserv := api.IngestGRPCProto{}
	if params.Ingest != nil && params.Ingest.GRPC != nil {
		netObserv = *params.Ingest.GRPC
	}
	if netObserv.Port == 0 {
		return nil, fmt.Errorf("ingest port not specified")
	}
	bufLen := netObserv.BufferLen
	if bufLen == 0 {
		bufLen = defaultBufferLen
	}
	flowPackets := make(chan *pbflow.Records, bufLen)
	collector, err := grpc.StartCollector(netObserv.Port, flowPackets)
	if err != nil {
		return nil, err
	}
	decoder, err := decode.NewProtobuf()
	if err != nil {
		return nil, err
	}
	return &GRPCProtobuf{
		collector:   collector,
		flowPackets: flowPackets,
		decoder:     decoder,
	}, nil
}

func (no *GRPCProtobuf) Ingest(out chan<- []config.GenericMap) {
	go func() {
		<-utils.ExitChannel()
		close(no.flowPackets)
		no.collector.Close()
	}()
	for fp := range no.flowPackets {
		out <- no.decoder.Decode([]interface{}{fp})
	}
}

func (no *GRPCProtobuf) Close() error {
	err := no.collector.Close()
	close(no.flowPackets)
	return err
}
