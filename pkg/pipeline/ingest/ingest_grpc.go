package ingest

import (
	"context"
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/grpc"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/sirupsen/logrus"
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var glog = logrus.WithField("component", "ingest.GRPCProtobuf")

const (
	defaultBufferLen = 100
)

// GRPCProtobuf ingests data from the NetObserv eBPF Agent, using Protocol Buffers over gRPC
type GRPCProtobuf struct {
	collector   *grpc.CollectorServer
	flowPackets chan *pbflow.Records
	metrics     *metrics
}

func NewGRPCProtobuf(opMetrics *operational.Metrics, params config.StageParam) (*GRPCProtobuf, error) {
	cfg := api.IngestGRPCProto{}
	if params.Ingest != nil && params.Ingest.GRPC != nil {
		cfg = *params.Ingest.GRPC
	}
	if cfg.Port == 0 {
		return nil, fmt.Errorf("ingest port not specified")
	}
	bufLen := cfg.BufferLen
	if bufLen == 0 {
		bufLen = defaultBufferLen
	}
	tlsConfig, err := cfg.TLS.AsServer()
	if err != nil {
		return nil, fmt.Errorf("error configuring TLS for GRPC ingestion: %w", err)
	}
	flowPackets := make(chan *pbflow.Records, bufLen)
	metrics := newMetrics(opMetrics, params.Name, params.Ingest.Type, func() int { return len(flowPackets) })
	options := []grpc2.ServerOption{grpc2.UnaryInterceptor(instrumentGRPC(metrics))}
	if tlsConfig != nil {
		options = append(options, grpc2.Creds(credentials.NewTLS(tlsConfig)))
	}
	collector, err := grpc.StartCollector(
		cfg.Port,
		flowPackets,
		grpc.WithGRPCServerOptions(options...),
	)
	if err != nil {
		return nil, err
	}
	return &GRPCProtobuf{
		collector:   collector,
		flowPackets: flowPackets,
		metrics:     metrics,
	}, nil
}

func (no *GRPCProtobuf) Ingest(out chan<- config.GenericMap) {
	no.metrics.createOutQueueLen(out)
	go func() {
		<-utils.ExitChannel()
		close(no.flowPackets)
		no.collector.Close()
	}()
	for fp := range no.flowPackets {
		glog.Debugf("Ingested %v records", len(fp.Entries))
		for _, entry := range fp.Entries {
			out <- decode.PBFlowToMap(entry)
		}
	}
}

func (no *GRPCProtobuf) Close() error {
	err := no.collector.Close()
	close(no.flowPackets)
	return err
}

func instrumentGRPC(m *metrics) grpc2.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc2.UnaryServerInfo,
		handler grpc2.UnaryHandler,
	) (resp interface{}, err error) {
		timer := m.stageDurationTimer()
		timeReceived := timer.Start()
		if info.FullMethod != "/pbflow.Collector/Send" {
			return handler(ctx, req)
		}
		flowRecords := req.(*pbflow.Records)

		// instrument difference between flow time and ingest time
		for _, entry := range flowRecords.Entries {
			delay := timeReceived.Sub(entry.TimeFlowEnd.AsTime()).Seconds()
			m.latency.Observe(delay)
		}

		// instrument flows processed counter
		m.flowsProcessed.Add(float64(len(flowRecords.Entries)))

		// instrument message bytes
		m.batchSizeBytes.Observe(float64(proto.Size(flowRecords)))

		resp, err = handler(ctx, req)
		if err != nil {
			// "trace" level used to minimize performance impact
			glog.Tracef("Reporting metric error: %v", err)
			m.error(fmt.Sprint(status.Code(err)))
		}

		// Stage duration
		timer.ObserveMilliseconds()

		return resp, err
	}
}
