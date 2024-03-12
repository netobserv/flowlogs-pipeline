package write

import (
	"testing"

	"github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/stretchr/testify/require"
)

func Test_WriteGRPC(t *testing.T) {
	port, err := test.FreeTCPPort()
	require.NoError(t, err)
	cc, err := grpc.ConnectClient("127.0.0.1", port)
	require.NoError(t, err)
	ws := writeGRPC{
		hostIP:     "127.0.0.1",
		hostPort:   port,
		clientConn: cc,
	}
	ws.Write(config.GenericMap{"key": "test"})
}

func Test_NewWriteGRPC(t *testing.T) {
	writer, err := NewWriteGRPC(config.StageParam{})
	require.Nil(t, writer)
	require.Error(t, err)

	writeParams := api.WriteGRPC{
		TargetHost: "target",
		TargetPort: 1234,
	}
	writer, err = NewWriteGRPC(config.StageParam{
		Write: &config.Write{
			GRPC: &writeParams,
		},
	})
	require.Nil(t, err)
	require.NotNil(t, writer)
}
