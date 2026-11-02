package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
)

// CollectorServer wraps a Flow Collector connection & session
type CollectorServer struct {
	grpcServer *grpc.Server
	collector  *collectorAPI
	closeOnce  sync.Once
}

type collectorOptions struct {
	grpcServerOptions []grpc.ServerOption
}

// CollectorOption allows overriding the default configuration of the CollectorServer instance.
// Use them in the StartCollector function.
type CollectorOption func(options *collectorOptions)

func WithGRPCServerOptions(options ...grpc.ServerOption) CollectorOption {
	return func(copt *collectorOptions) {
		copt.grpcServerOptions = options
	}
}

// StartCollector listens in background for gRPC+Protobuf flows in the given port, and forwards each
// set of *genericmap.Flow by the provided channel.
func StartCollector(
	port int, recordForwarder chan<- *genericmap.Flow, options ...CollectorOption,
) (*CollectorServer, error) {
	copts := collectorOptions{}
	for _, opt := range options {
		opt(&copts)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	grpcServer := grpc.NewServer(copts.grpcServerOptions...)
	collector := &collectorAPI{
		recordForwarder: recordForwarder,
		done:            make(chan struct{}),
	}
	genericmap.RegisterCollectorServer(grpcServer, collector)
	reflection.Register(grpcServer)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			panic("error connecting to server: " + err.Error())
		}
	}()
	return &CollectorServer{
		grpcServer: grpcServer,
		collector:  collector,
	}, nil
}

func (c *CollectorServer) Close() error {
	c.closeOnce.Do(func() {
		c.collector.close()
		c.grpcServer.Stop()
		c.collector.wg.Wait()
	})
	return nil
}

type collectorAPI struct {
	genericmap.UnimplementedCollectorServer
	recordForwarder chan<- *genericmap.Flow
	done            chan struct{}
	mu              sync.RWMutex
	closed          bool
	wg              sync.WaitGroup
}

var okReply = &genericmap.CollectorReply{}

func (c *collectorAPI) Send(_ context.Context, records *genericmap.Flow) (*genericmap.CollectorReply, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, context.Canceled
	}
	c.wg.Add(1)
	c.mu.RUnlock()
	defer c.wg.Done()

	select {
	case c.recordForwarder <- records:
	case <-c.done:
		return nil, context.Canceled
	}
	return okReply, nil
}

func (c *collectorAPI) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.done)
	}
}
