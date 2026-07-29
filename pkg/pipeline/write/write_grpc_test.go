package write

import (
	cryptotls "crypto/tls"
	"testing"

	"github.com/mariomac/guara/pkg/test"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc"
	"github.com/netobserv/flowlogs-pipeline/pkg/tlsprofile"
	"github.com/stretchr/testify/require"
)

func Test_WriteGRPC(t *testing.T) {
	port, err := test.FreeTCPPort()
	require.NoError(t, err)
	cc, err := grpc.ConnectClient("127.0.0.1", port, nil)
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

func Test_ResolveTLSConfig_NoTLSConfiguration(t *testing.T) {
	// Explicitly clear all three vars: a TLS_* value inherited from the
	// parent environment must not make this test nondeterministic.
	t.Setenv(tlsprofile.EnvMinVersion, "")
	t.Setenv(tlsprofile.EnvCipherSuites, "")
	t.Setenv(tlsprofile.EnvCurvePreferences, "")

	tlsCfg, err := resolveTLSConfig(nil)
	require.NoError(t, err)
	require.Nil(t, tlsCfg, "no explicit TLS and no env override should keep the connection insecure")
}

func Test_ResolveTLSConfig_EnvironmentOnlyTLS(t *testing.T) {
	t.Setenv(tlsprofile.EnvMinVersion, "771")
	t.Setenv(tlsprofile.EnvCipherSuites, "49199,49200")

	tlsCfg, err := resolveTLSConfig(nil)
	require.NoError(t, err)
	require.NotNil(t, tlsCfg, "a valid env override alone should turn on TLS")
	require.Equal(t, uint16(cryptotls.VersionTLS12), tlsCfg.MinVersion)
	require.Equal(t, []uint16{49199, 49200}, tlsCfg.CipherSuites)
}

func Test_ResolveTLSConfig_EnvironmentOnlyAllTLS13CipherSuites(t *testing.T) {
	// No TLS_MIN_VERSION: the base env-only config already defaults to TLS 1.3,
	// and TLS_CIPHER_SUITES only contains TLS 1.3 suite IDs, which are never
	// applied to CipherSuites. With nothing actually applied, TLS is not
	// turned on, same as if no env override had been set at all.
	t.Setenv(tlsprofile.EnvCipherSuites, "4865,4866,4867")

	tlsCfg, err := resolveTLSConfig(nil)
	require.NoError(t, err)
	require.Nil(t, tlsCfg, "an all-TLS-1.3 cipher list has nothing to apply, so TLS should stay off")
}

func Test_ResolveTLSConfig_ExplicitTLSWithOverride(t *testing.T) {
	t.Setenv(tlsprofile.EnvMinVersion, "771")

	tlsCfg, err := resolveTLSConfig(&api.ClientTLS{InsecureSkipVerify: true})
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	require.True(t, tlsCfg.InsecureSkipVerify, "explicit TLS config should still be honored")
	require.Equal(t, uint16(cryptotls.VersionTLS12), tlsCfg.MinVersion, "env override should apply on top of the explicit TLS config")
}

func Test_ResolveTLSConfig_MalformedEnvironmentValue(t *testing.T) {
	t.Setenv(tlsprofile.EnvMinVersion, "not-a-number")

	// Malformed env value with no explicit TLS config.
	tlsCfg, err := resolveTLSConfig(nil)
	require.Error(t, err)
	require.Nil(t, tlsCfg)

	// Malformed env value with an explicit TLS config: the invalid override
	// must not be silently dropped in favor of the explicit config.
	tlsCfg, err = resolveTLSConfig(&api.ClientTLS{})
	require.Error(t, err)
	require.Nil(t, tlsCfg)
}
