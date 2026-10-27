package write

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/require"
	"github.com/vmware/go-ipfix/pkg/entities"
	ipfixExporter "github.com/vmware/go-ipfix/pkg/exporter"
	"github.com/vmware/go-ipfix/pkg/registry"
)

func allCustomFields() []entities.InfoElement {
	var all []entities.InfoElement
	for _, list := range [][]entities.InfoElement{KubeFields, CustomNetworkFields, CustomNetworkFieldsV4, CustomNetworkFieldsV6, DeprecatedFields} {
		all = append(all, list...)
	}
	return all
}

func TestNoFieldConflict(t *testing.T) {
	fields := make(map[uint16]entities.InfoElement)
	for _, field := range allCustomFields() {
		if prev, exists := fields[field.ElementId]; exists {
			t.Errorf("Custom field %s conflicts with %s (id: %d)", prev.Name, field.Name, field.ElementId)
		}
		fields[field.ElementId] = field
	}
}

// Frozen signature of every released custom element. Append a line when adding a field; never edit
// or delete one, as collectors decode against these. To retire a field, move it to DeprecatedFields.
var frozenCustomFields = []string{
	"7733 sourcePodNamespace type=13 len=65535",
	"7734 sourcePodName type=13 len=65535",
	"7735 destinationPodNamespace type=13 len=65535",
	"7736 destinationPodName type=13 len=65535",
	"7737 sourceNodeName type=13 len=65535",
	"7738 destinationNodeName type=13 len=65535",
	"7739 latestDroppedCause type=13 len=65535",
	"7740 timeFlowRttNs type=4 len=8",
	"7741 interfaces type=13 len=65535",
	"7742 directions type=13 len=65535",
	"7743 sourceServiceName type=13 len=65535",
	"7744 destinationServiceName type=13 len=65535",
}

func TestCustomFieldsAreFrozen(t *testing.T) {
	var actual []string
	for _, f := range allCustomFields() {
		actual = append(actual, fmt.Sprintf("%d %s type=%d len=%d", f.ElementId, f.Name, f.DataType, f.Len))
	}
	sort.Strings(actual)
	require.Equal(t, frozenCustomFields, actual, "Custom fields don't match the frozen list. If you added new fields, update the frozen list. You must not remove or modify existing fields (unless they weren't released).")
}

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
