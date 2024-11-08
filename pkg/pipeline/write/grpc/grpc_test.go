package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
)

const timeout = 5 * time.Second

func TestGRPCCommunication(t *testing.T) {
	port, err := test.FreeTCPPort()
	require.NoError(t, err)
	serverOut := make(chan *genericmap.Flow)
	_, err = StartCollector(port, serverOut)
	require.NoError(t, err)
	cc, err := ConnectClient("127.0.0.1", port)
	require.NoError(t, err)
	client := cc.Client()

	gm := config.GenericMap{
		"test": "test",
	}
	value, err := json.Marshal(gm)
	require.NoError(t, err)

	go func() {
		_, err = client.Send(context.Background(),
			&genericmap.Flow{
				GenericMap: &anypb.Any{
					Value: value,
				},
			})
		require.NoError(t, err)
	}()

	var rs *genericmap.Flow
	select {
	case rs = <-serverOut:
	case <-time.After(timeout):
		require.Fail(t, "timeout waiting for flows")
	}
	assert.NotNil(t, rs.GenericMap)
	assert.EqualValues(t, value, rs.GenericMap.Value)
	select {
	case rs = <-serverOut:
		assert.Failf(t, "shouldn't have received any flow", "Got: %#v", rs)
	default:
		// ok!
	}
}

func TestConstructorOptions(t *testing.T) {
	port, err := test.FreeTCPPort()
	require.NoError(t, err)
	intercepted := make(chan struct{})
	// Override the default GRPC collector to verify that StartCollector is applying the
	// passed options
	_, err = StartCollector(port, make(chan *genericmap.Flow),
		WithGRPCServerOptions(grpc.UnaryInterceptor(func(
			ctx context.Context,
			req interface{},
			_ *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (resp interface{}, err error) {
			close(intercepted)
			return handler(ctx, req)
		})))
	require.NoError(t, err)
	cc, err := ConnectClient("127.0.0.1", port)
	require.NoError(t, err)
	client := cc.Client()

	go func() {
		_, err = client.Send(context.Background(),
			&genericmap.Flow{GenericMap: &anypb.Any{}})
		require.NoError(t, err)
	}()

	select {
	case <-intercepted:
	case <-time.After(timeout):
		require.Fail(t, "timeout waiting for unary interceptor to work")
	}
}
