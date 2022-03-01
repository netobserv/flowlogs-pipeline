package ingest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const timeout = 5 * time.Second

func TestIngest(t *testing.T) {
	collectorPort, err := test.UDPPort()
	require.NoError(t, err)
	ic := &ingestCollector{
		hostname:       "0.0.0.0",
		port:           collectorPort,
		batchFlushTime: 10 * time.Millisecond,
		exitChan:       make(chan bool),
	}
	forwarded := make(chan []interface{})
	defer close(forwarded)

	// GIVEN an IPFIX collector Ingester
	go ic.Ingest(forwarded)

	client, err := test.NewIPFIXClient(collectorPort)
	require.NoError(t, err)

	// The IPFIX client might send information before the Ingester is actually listening,
	// so we might need to repeat the submission
	test.Eventually(t, timeout, func(t require.TestingT) {
		// WHEN the service receives an IPFIX message
		require.NoError(t, client.SendTemplate())
		require.NoError(t, client.SendFlow(12345678, "1.2.3.4"))

		select {
		case received := <-forwarded:
			// THEN the bytes are forwarded
			require.NotEmpty(t, received)
			require.IsType(t, "string", received[0])
			flow := map[string]interface{}{}
			require.NoError(t, json.Unmarshal([]byte(received[0].(string)), &flow))
			assert.EqualValues(t, 12345678, flow["TimeFlowStart"])
			assert.EqualValues(t, 12345678, flow["TimeFlowEnd"])
			assert.Equal(t, "1.2.3.4", flow["SrcAddr"])
		case <-time.After(50 * time.Millisecond):
			require.Fail(t, "error waiting for ingester to forward received data")
		}
	})
}
