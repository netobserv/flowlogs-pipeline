package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteIpfixSetDefaultsPathMTU(t *testing.T) {
	tests := []struct {
		name    string
		initial int
		want    int
	}{
		{name: "default", want: defaultPathMTU},
		{name: "configured value", initial: 1200, want: 1200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := WriteIpfix{PathMTU: tt.initial}
			config.SetDefaults()
			require.Equal(t, tt.want, config.PathMTU)
		})
	}
}

func TestWriteIpfixValidatePathMTU(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		pathMTU   int
		wantError bool
	}{
		{name: "UDP below minimum", transport: "udp", pathMTU: minPathMTU - 1, wantError: true},
		{name: "UDP at minimum", transport: "udp", pathMTU: minPathMTU},
		{name: "UDP above minimum", transport: "udp", pathMTU: 1400},
		{name: "TCP ignores path MTU", transport: "tcp", pathMTU: minPathMTU - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := WriteIpfix{
				TargetHost: "collector.example.test",
				TargetPort: 2055,
				Transport:  tt.transport,
				PathMTU:    tt.pathMTU,
			}
			config.SetDefaults()
			err := config.Validate()
			if tt.wantError {
				require.Error(t, err)
				require.ErrorContains(t, err, "pathMtu should be at least")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
