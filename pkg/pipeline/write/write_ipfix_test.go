package write

import (
	"bytes"
	"encoding/binary"
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

// Test both field values and encoded record lengths when an exporter reuses its
// template elements for complete and incomplete flows.
func TestDataRecordLengthMatchesEncoding(t *testing.T) {
	registry.LoadRegistry()

	tests := []struct {
		name          string
		ipv6          bool
		first         config.GenericMap
		partial       config.GenericMap
		last          config.GenericMap
		firstExpected map[string]string
		partialExpect map[string]string
		lastExpected  map[string]string
	}{
		{
			name: "IPv4",
			first: config.GenericMap{
				"SrcAddr": "192.0.2.10", "DstAddr": "192.0.2.20",
				"SrcMac": "02:00:00:00:00:10", "DstMac": "02:00:00:00:00:20",
			},
			partial: config.GenericMap{
				"SrcAddr": "198.51.100.5", "SrcMac": "02:00:00:00:00:05",
			},
			last: config.GenericMap{
				"SrcAddr": "203.0.113.30", "DstAddr": "203.0.113.40",
				"SrcMac": "02:00:00:00:00:30", "DstMac": "02:00:00:00:00:40",
			},
			firstExpected: map[string]string{
				"sourceIPv4Address": "192.0.2.10", "destinationIPv4Address": "192.0.2.20",
				"sourceMacAddress": "02:00:00:00:00:10", "destinationMacAddress": "02:00:00:00:00:20",
			},
			partialExpect: map[string]string{
				"sourceIPv4Address": "198.51.100.5", "destinationIPv4Address": "0.0.0.0",
				"sourceMacAddress": "02:00:00:00:00:05", "destinationMacAddress": "00:00:00:00:00:00",
			},
			lastExpected: map[string]string{
				"sourceIPv4Address": "203.0.113.30", "destinationIPv4Address": "203.0.113.40",
				"sourceMacAddress": "02:00:00:00:00:30", "destinationMacAddress": "02:00:00:00:00:40",
			},
		},
		{
			name: "IPv6",
			ipv6: true,
			first: config.GenericMap{
				"SrcAddr": "2001:db8::10", "DstAddr": "2001:db8::20",
				"SrcMac": "02:00:00:00:00:10", "DstMac": "02:00:00:00:00:20",
			},
			partial: config.GenericMap{
				"SrcAddr": "2001:db8::5", "SrcMac": "02:00:00:00:00:05",
			},
			last: config.GenericMap{
				"SrcAddr": "2001:db8::30", "DstAddr": "2001:db8::40",
				"SrcMac": "02:00:00:00:00:30", "DstMac": "02:00:00:00:00:40",
			},
			firstExpected: map[string]string{
				"sourceIPv6Address": "2001:db8::10", "destinationIPv6Address": "2001:db8::20",
				"sourceMacAddress": "02:00:00:00:00:10", "destinationMacAddress": "02:00:00:00:00:20",
			},
			partialExpect: map[string]string{
				"sourceIPv6Address": "2001:db8::5", "destinationIPv6Address": "::",
				"sourceMacAddress": "02:00:00:00:00:05", "destinationMacAddress": "00:00:00:00:00:00",
			},
			lastExpected: map[string]string{
				"sourceIPv6Address": "2001:db8::30", "destinationIPv6Address": "2001:db8::40",
				"sourceMacAddress": "02:00:00:00:00:30", "destinationMacAddress": "02:00:00:00:00:40",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const templateID = 256
			_, elements, err := prepareTemplate(templateID, 0, tt.ipv6)
			require.NoError(t, err)
			wrongFamilyAddress := "192.0.2.1"
			validAddress := "192.0.2.10"
			validAddressField := "sourceIPv4Address"
			if !tt.ipv6 {
				wrongFamilyAddress = "2001:db8::1"
			} else {
				validAddress = "2001:db8::10"
				validAddressField = "sourceIPv6Address"
			}

			// Reuse the template elements across full, empty, partial, and full records.
			// Missing fields must be zeroed, and defaults must not leak into the next flow.
			records := []struct {
				name     string
				flow     config.GenericMap
				expected map[string]string
			}{
				{name: "full values", flow: tt.first, expected: tt.firstExpected},
				{name: "all values missing", flow: config.GenericMap{}, expected: zeroAddressAndMACValues(tt.ipv6)},
				{name: "partial values", flow: tt.partial, expected: tt.partialExpect},
				{
					name: "unparseable IP and MAC values",
					flow: config.GenericMap{
						"SrcAddr": "not-an-ip", "DstAddr": "also-not-an-ip",
						"SrcMac": "not-a-mac", "DstMac": "also-not-a-mac",
					},
					expected: zeroAddressAndMACValues(tt.ipv6),
				},
				{
					name: "invalid destination preserves valid source",
					flow: config.GenericMap{
						"SrcAddr": validAddress, "DstAddr": "not-an-ip",
						"SrcMac": "02:00:00:00:00:10", "DstMac": "not-a-mac",
					},
					expected: func() map[string]string {
						want := zeroAddressAndMACValues(tt.ipv6)
						want[validAddressField] = validAddress
						want["sourceMacAddress"] = "02:00:00:00:00:10"
						return want
					}(),
				},
				{
					// IPv4 text parses to a 16-byte IPv4-mapped value, so To16 alone
					// does not detect it as the wrong family for an IPv6 field.
					name: "wrong-family addresses",
					flow: config.GenericMap{
						"SrcAddr": wrongFamilyAddress, "DstAddr": wrongFamilyAddress,
						"SrcMac": "02:00:00:00:00:10", "DstMac": "02:00:00:00:00:20",
					},
					expected: func() map[string]string {
						want := zeroAddressAndMACValues(tt.ipv6)
						want["sourceMacAddress"] = "02:00:00:00:00:10"
						want["destinationMacAddress"] = "02:00:00:00:00:20"
						return want
					}(),
				},
				{name: "full values after defaults", flow: tt.last, expected: tt.lastExpected},
			}

			for _, record := range records {
				t.Run(record.name, func(t *testing.T) {
					createDataRecord(record.flow, elements)
					assertIPFIXValues(t, elements, record.expected)
					assertIPFIXRecordLengthMatchesEncoding(t, templateID, elements)
				})
			}
		})
	}
}

func zeroAddressAndMACValues(ipv6 bool) map[string]string {
	addressField, zeroAddress := "sourceIPv4Address", "0.0.0.0"
	destinationField := "destinationIPv4Address"
	if ipv6 {
		addressField, zeroAddress = "sourceIPv6Address", "::"
		destinationField = "destinationIPv6Address"
	}
	return map[string]string{
		addressField:            zeroAddress,
		destinationField:        zeroAddress,
		"sourceMacAddress":      "00:00:00:00:00:00",
		"destinationMacAddress": "00:00:00:00:00:00",
	}
}

func assertIPFIXValues(t *testing.T, elements []entities.InfoElementWithValue, expected map[string]string) {
	t.Helper()
	for name, want := range expected {
		mapping, ok := MapIPFIXKeys[name]
		require.True(t, ok, "missing IPFIX mapping for %q", name)

		found := false
		for _, element := range elements {
			if element.GetName() == name {
				require.Equal(t, want, mapping.Getter(element), "unexpected value for %s", name)
				found = true
				break
			}
		}
		require.True(t, found, "IPFIX element %q is missing from the template", name)
	}
}

func assertIPFIXRecordLengthMatchesEncoding(t *testing.T, templateID uint16, elements []entities.InfoElementWithValue) {
	t.Helper()
	set := entities.NewSet(false)
	require.NoError(t, set.PrepareSet(entities.Data, templateID))
	require.NoError(t, set.AddRecord(elements, templateID))
	set.UpdateLenInHeader()

	buf := &bytes.Buffer{}
	announced, err := ipfixExporter.WriteIPFIXMsgToBuffer(set, 1, 0, time.Now(), buf)
	require.NoError(t, err)
	require.Equal(t, announced, buf.Len(), "IPFIX message length must match its encoded bytes")
	setHeaderStart := entities.MsgHeaderLength
	encodedSetLength := binary.BigEndian.Uint16(buf.Bytes()[setHeaderStart+2 : setHeaderStart+entities.SetHeaderLen])
	require.Equal(t, uint16(set.GetSetLength()), encodedSetLength, "IPFIX set header length must match its encoded bytes")
}
