package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeFlags(t *testing.T) {
	flags528 := DecodeTCPFlags(528)
	assert.Equal(t, []string{"ACK", "FIN_ACK"}, flags528)

	flags256 := DecodeTCPFlags(256)
	assert.Equal(t, []string{"SYN_ACK"}, flags256)

	flags666 := DecodeTCPFlags(666)
	assert.Equal(t, []string{"SYN", "PSH", "ACK", "CWR", "FIN_ACK"}, flags666)
}
