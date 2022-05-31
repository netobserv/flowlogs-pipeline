package ingest_decode

import (
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
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
		exitChan:       make(chan struct{}),
	}
	forwarded := make(chan []config.GenericMap)
	//defer close(forwarded)

	// GIVEN an IPFIX collector Ingester
	go ic.Ingest(forwarded)

	client, err := test.NewIPFIXClient(collectorPort)
	require.NoError(t, err)

	received := waitForFlows(t, client, forwarded)
	require.NotEmpty(t, received)
	flow := received[0]
	assert.EqualValues(t, 12345678, flow["TimeFlowStart"])
	assert.EqualValues(t, 12345678, flow["TimeFlowEnd"])
	assert.Equal(t, "1.2.3.4", flow["SrcAddr"])
}

// The IPFIX client might send information before the Ingester is actually listening,
// so we might need to repeat the submission until the ingest starts forwarding logs
func waitForFlows(t *testing.T, client *test.IPFIXClient, forwarded chan []config.GenericMap) []config.GenericMap {
	var start = time.Now()
	for {
		if client.SendTemplate() == nil &&
			client.SendFlow(12345678, "1.2.3.4") == nil {
			select {
			case received := <-forwarded:
				return received
			default:
				// nothing yet received
			}
		}
		if time.Since(start) > timeout {
			require.Fail(t, "error waiting for ingester to forward received data")
		}
		time.After(50 * time.Millisecond)
	}
}
