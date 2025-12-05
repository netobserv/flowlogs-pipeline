package ingest

import (
	"context"
	"testing"
	"time"

	test2 "github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	flowgrpc "github.com/netobserv/netobserv-ebpf-agent/pkg/grpc/flow"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tlsConfig struct {
	clientCertPath        string
	clientKeyPath         string
	serverCAPathForClient string
	serverCertPath        string
	serverKeyPath         string
	clientCAPathForServer string
}

func TestGRPC_NoTLS(t *testing.T) {
	runGRPCTestForTLS(t, tlsConfig{}, "")
}

func TestGRPC_SimpleTLS(t *testing.T) {
	ca, _, _, cert, key, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	runGRPCTestForTLS(
		t,
		tlsConfig{
			serverCAPathForClient: ca,
			serverCertPath:        cert,
			serverKeyPath:         key,
		},
		"",
	)
}

func TestGRPC_MutualTLS(t *testing.T) {
	ca, user, userKey, serverCert, serverKey, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	runGRPCTestForTLS(
		t,
		tlsConfig{
			clientCAPathForServer: ca,
			clientCertPath:        user,
			clientKeyPath:         userKey,
			serverCAPathForClient: ca,
			serverCertPath:        serverCert,
			serverKeyPath:         serverKey,
		},
		"",
	)
}

func TestGRPC_FailExpectingTLS(t *testing.T) {
	_, _, _, cert, key, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	runGRPCTestForTLS(
		t,
		tlsConfig{
			serverCAPathForClient: "", // by not providing server CA, the client is in NOTLS mode
			serverCertPath:        cert,
			serverKeyPath:         key,
		},
		"connection error",
	)
}

func TestGRPC_FailExpectingMTLS(t *testing.T) {
	ca, _, _, serverCert, serverKey, cleanup := test.CreateAllCerts(t)
	defer cleanup()
	runGRPCTestForTLS(
		t,
		tlsConfig{
			clientCAPathForServer: ca,
			clientCertPath:        "", // by not providing client cert, the client is in simple TLS mode
			clientKeyPath:         "",
			serverCAPathForClient: ca,
			serverCertPath:        serverCert,
			serverKeyPath:         serverKey,
		},
		"write tcp", // would prefer a more explicit error, but that's all that we get: "write tcp 127.0.0.1:54154->127.0.0.1:40763: write: broken pipe"
	)
}

// nolint:gocritic // hugeParam; don't care in tests
func runGRPCTestForTLS(t *testing.T, certs tlsConfig, expectErrorContains string) {
	port, err := test2.FreeTCPPort()
	require.NoError(t, err)

	out := make(chan config.GenericMap)
	cfg := &config.Ingest{
		GRPC: &api.IngestGRPCProto{
			Port:         port,
			CertPath:     certs.serverCertPath,
			KeyPath:      certs.serverKeyPath,
			ClientCAPath: certs.clientCAPathForServer,
		},
	}
	ingester, err := NewGRPCProtobuf(operational.NewMetrics(&config.MetricsSettings{}), config.StageParam{Ingest: cfg})
	require.NoError(t, err)

	go ingester.Ingest(out)

	flowSender, err := flowgrpc.ConnectClient("127.0.0.1", port, certs.serverCAPathForClient, certs.clientCertPath, certs.clientKeyPath)
	require.NoError(t, err)
	defer flowSender.Close()

	record := &pbflow.Records{
		Entries: []*pbflow.Record{{
			EthProtocol: 2048,
			Transport:   &pbflow.Transport{},
			Network: &pbflow.Network{
				SrcAddr: &pbflow.IP{
					IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x01020304},
				},
				DstAddr: &pbflow.IP{
					IpFamily: &pbflow.IP_Ipv4{Ipv4: 0x05060708},
				},
			},
		}},
	}

	_, err = flowSender.Client().Send(context.Background(), record)
	if expectErrorContains != "" {
		require.ErrorContains(t, err, expectErrorContains)
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
