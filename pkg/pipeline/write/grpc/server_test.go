package grpc

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write/grpc/genericmap"
)

func TestCollectorAPICloseUnblocksSend(t *testing.T) {
	api := &collectorAPI{
		recordForwarder: make(chan *genericmap.Flow),
		done:            make(chan struct{}),
	}

	sendDone := make(chan struct{})
	go func() {
		defer close(sendDone)
		if _, err := api.Send(nil, &genericmap.Flow{}); err == nil {
			t.Error("Send succeeded after the collector was closed")
		}
	}()

	api.close()
	api.wg.Wait()
	<-sendDone
}
