package ingest

import (
	"fmt"

	"github.com/mariomac/pipes/pkg/node"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/netobserv-agent/pkg/grpc"
	"github.com/netobserv/netobserv-agent/pkg/pbflow"
)

const defaultBufferLen = 100

// GRPCProtobuf ingests data from the NetObserv eBPF Agent, using Protocol Buffers over gRPC
type GRPCProtobuf struct {
	cfg         api.IngestGRPCProto
	flowPackets chan *pbflow.Records
}

func GRPCProtobufProvider(cfg api.IngestGRPCProto) node.StartFunc[[]interface{}] {
	// TODO: one of the pending issues in the pipes library is to find a better way
	// to check this kind of errors and propagate them
	if cfg.Port == 0 {
		panic("ingest port not specified")
	}
	bufLen := cfg.BufferLen
	if bufLen == 0 {
		bufLen = defaultBufferLen
	}
	return GRPCProtobuf{
		cfg:         cfg,
		flowPackets: make(chan *pbflow.Records, bufLen),
	}.Ingest
}

func (no *GRPCProtobuf) Ingest(out chan<- []interface{}) {
	collector, err := grpc.StartCollector(no.cfg.Port, no.flowPackets)
	if err != nil {
		// TODO: one of the pending issues in the pipes library is to find a better way
		// to propagate errors during instantiation
		panic(fmt.Errorf("starting grpc collector: %w", err))
	}
	go func() {
		<-utils.ExitChannel()
		close(no.flowPackets)
		collector.Close()
	}()
	for fp := range no.flowPackets {
		out <- []interface{}{fp}
	}
}
