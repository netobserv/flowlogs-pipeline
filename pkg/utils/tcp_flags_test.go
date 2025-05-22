package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeFlags(t *testing.T) {
	flags528 := DecodeTCPFlagsU16(528)
	assert.Equal(t, []string{"ACK", "FIN_ACK"}, flags528)

	flags256 := DecodeTCPFlagsU32(256)
	assert.Equal(t, []string{"SYN_ACK"}, flags256)

	flags666 := DecodeTCPFlagsU16(666)
	assert.Equal(t, []string{"SYN", "PSH", "ACK", "CWR", "FIN_ACK"}, flags666)
}
