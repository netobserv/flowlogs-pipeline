package ingest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/grpc"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	flow "github.com/netsampler/goflow2/utils"
	"github.com/prometheus/client_golang/prometheus"
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	defaultBufferLen = 100
	decoderName      = "protobuf"
	decoderVersion   = "protobuf1.0"
)

// Prometheus metrics describing the performance of the eBPF ingest
var (
	flowDecoderCount = flow.DecoderStats.With(
		prometheus.Labels{"worker": "", "name": decoderName})
	processDelaySummary = flow.NetFlowTimeStatsSum.With(
		prometheus.Labels{"version": decoderVersion, "router": ""})
	flowTrafficBytesSum = flow.MetricPacketSizeSum
	flowErrors          = flow.NetFlowErrors
)

// GRPCProtobuf ingests data from the NetObserv eBPF Agent, using Protocol Buffers over gRPC
type GRPCProtobuf struct {
	collector   *grpc.CollectorServer
	flowPackets chan *pbflow.Records
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
	collector, err := grpc.StartCollector(netObserv.Port, flowPackets,
		grpc.WithGRPCServerOptions(grpc2.UnaryInterceptor(instrumentGRPC(netObserv.Port))))
	if err != nil {
		return nil, err
	}
	return &GRPCProtobuf{
		collector:   collector,
		flowPackets: flowPackets,
	}, nil
}

func (no *GRPCProtobuf) Ingest(out chan<- []interface{}) {
	go func() {
		<-utils.ExitChannel()
		close(no.flowPackets)
		no.collector.Close()
	}()
	for fp := range no.flowPackets {
		out <- []interface{}{fp}
	}
}

func (no *GRPCProtobuf) Close() error {
	err := no.collector.Close()
	close(no.flowPackets)
	return err
}

func instrumentGRPC(port int) grpc2.UnaryServerInterceptor {
	localPort := strconv.Itoa(port)
	return func(
		ctx context.Context,
		req interface{},
		info *grpc2.UnaryServerInfo,
		handler grpc2.UnaryHandler,
	) (resp interface{}, err error) {
		timeReceived := time.Now()
		if info.FullMethod != "/pbflow.Collector/Send" {
			return handler(ctx, req)
		}
		flowRecords := req.(*pbflow.Records)

		// instrument difference between flow time and ingest time
		for _, entry := range flowRecords.Entries {
			delay := timeReceived.Sub(entry.TimeFlowEnd.AsTime()).Seconds()
			processDelaySummary.Observe(delay)
		}

		// instruments number of decoded flow messages
		flowDecoderCount.Inc()

		// instruments number of processed individual flows
		linesProcessed.Add(float64(len(flowRecords.Entries)))

		// extract sender IP address
		remoteIP := "unknown"
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if auth := md.Get(":authority"); len(auth) > 0 {
				if portIdx := strings.IndexByte(auth[0], ':'); portIdx > 0 {
					remoteIP = auth[0][:portIdx]
				} else {
					remoteIP = auth[0]
				}
			}
		}
		trafficLabels := prometheus.Labels{
			"type":       "protobuf",
			"remote_ip":  remoteIP,
			"local_ip":   "0.0.0.0",
			"local_port": localPort,
		}

		// instrument message bytes
		flowTrafficBytesSum.With(trafficLabels).Observe(float64(proto.Size(flowRecords)))

		resp, err = handler(ctx, req)
		if err != nil {
			flowErrors.With(prometheus.Labels{"router": "", "error": err.Error()}).Inc()
		}
		return resp, err
	}
}
