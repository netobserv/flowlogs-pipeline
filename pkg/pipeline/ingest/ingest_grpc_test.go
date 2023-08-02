package ingest

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	test2 "github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/grpc"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGRPC_NoTLS(t *testing.T) {
	runGRPCTestForTLS(t, nil, nil, false)
}

func TestGRPC_SimpleTLS(t *testing.T) {
	ca, user, userKey, cleanup := test.CreateClientCerts(t)
	defer cleanup()
	tlsConf := &api.TLSConfig{
		Type:       "simple",
		CACertPath: ca,
		CertPath:   user,
		KeyPath:    userKey,
	}
	tlsCl, err := tlsConf.AsClient()
	require.NoError(t, err)
	runGRPCTestForTLS(t, tlsCl, tlsConf, false)
}

func TestGRPC_MutualTLS(t *testing.T) {
	ca, user, userKey, serverCert, serverKey, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	tlsClConf := &api.TLSConfig{
		Type:       "mutual",
		CACertPath: ca,
		CertPath:   user,
		KeyPath:    userKey,
	}
	tlsCl, err := tlsClConf.AsClient()
	tlsServConf := &api.TLSConfig{
		Type:       "mutual",
		CACertPath: ca,
		CertPath:   serverCert,
		KeyPath:    serverKey,
	}
	require.NoError(t, err)
	runGRPCTestForTLS(t, tlsCl, tlsServConf, false)
}

func TestGRPC_FailExpectingTLS(t *testing.T) {
	ca, user, userKey, cleanup := test.CreateClientCerts(t)
	defer cleanup()
	tlsConf := &api.TLSConfig{
		Type:       "simple",
		CACertPath: ca,
		CertPath:   user,
		KeyPath:    userKey,
	}
	runGRPCTestForTLS(t, nil, tlsConf, true)
}

func TestGRPC_FailExpectingMTLS(t *testing.T) {
	ca, _, _, serverCert, serverKey, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	tlsClConf := &api.TLSConfig{
		Type:       "simple",
		CACertPath: ca,
	}
	tlsCl, err := tlsClConf.AsClient()
	tlsServConf := &api.TLSConfig{
		Type:       "mutual",
		CACertPath: ca,
		CertPath:   serverCert,
		KeyPath:    serverKey,
	}
	require.NoError(t, err)
	runGRPCTestForTLS(t, tlsCl, tlsServConf, true)
}

func runGRPCTestForTLS(t *testing.T, clientTLS *tls.Config, servertTLS *api.TLSConfig, expectSendError bool) {
	port, err := test2.FreeTCPPort()
	require.NoError(t, err)

	out := make(chan config.GenericMap)
	_, err = runIngester(NewGRPCProtobuf, &config.Ingest{
		GRPC: &api.IngestGRPCProto{
			Port: port,
			TLS:  servertTLS,
		},
	}, out)
	require.NoError(t, err)

	flowSender, err := grpc.ConnectClient("127.0.0.1", port, clientTLS)
	require.NoError(t, err)
	defer flowSender.Close()

	_, err = flowSender.Client().Send(context.Background(), &pbflow.Records{
		Entries: []*pbflow.Record{{
			Network: &pbflow.Network{
				SrcAddr: &pbflow.IP{
					IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
				},
				DstAddr: &pbflow.IP{
					IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
				},
			},
		}},
	})
	if expectSendError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	var received config.GenericMap
	select {
	case r := <-out:
		received = r
	case <-time.After(timeout):
		require.Fail(t, "timeout while waiting for Ingester to receive and process data")
	}

	assert.Equal(t, "1.2.3.4", received["SrcAddr"])
	assert.Equal(t, "5.6.7.8", received["DstAddr"])
}
