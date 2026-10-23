package write

import (
	"bytes"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
	"github.com/vmware/go-ipfix/pkg/entities"
	ipfixExporter "github.com/vmware/go-ipfix/pkg/exporter"
	"github.com/vmware/go-ipfix/pkg/registry"
)

// This test ensures missing fields don't lead to inconsistant records
// Some element records must be zeroe'd with "Default" mapping implemented.
func TestDataRecordLengthMatchesEncoding(t *testing.T) {
	registry.LoadRegistry()

	// Test IPv4 and IPv6 templates
	for _, isV6 := range []bool{false, true} {
		_, elements, err := prepareTemplate(256, 0, isV6)
		require.NoError(t, err)

		// Minimal flow: no Mac / IP etc.
		createDataRecord(config.GenericMap{}, elements)

		set := entities.NewSet(false)
		require.NoError(t, set.PrepareSet(entities.Data, 256))
		require.NoError(t, set.AddRecord(elements, 256))

		buf := &bytes.Buffer{}
		announced, err := ipfixExporter.WriteIPFIXMsgToBuffer(set, 1, 0, time.Now(), buf)
		require.NoError(t, err)
		require.Equal(t, announced, buf.Len())
	}
}
