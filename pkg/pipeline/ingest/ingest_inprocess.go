package ingest

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"

	"github.com/netobserv/netobserv-ebpf-agent/pkg/decode"
	"github.com/sirupsen/logrus"
)

var ilog = logrus.WithField("component", "ingest.InProcess")

// InProcess ingester, meant to be imported and used from another program via
type InProcess struct {
	flowPackets chan *pbflow.Records
}

func NewInProcess(flowPackets chan *pbflow.Records) *InProcess {
	return &InProcess{flowPackets: flowPackets}
}

func (d *InProcess) Ingest(out chan<- config.GenericMap) {
	go func() {
		<-utils.ExitChannel()
		d.Close()
	}()
	for fp := range d.flowPackets {
		ilog.Debugf("Ingested %v records", len(fp.Entries))
		for _, entry := range fp.Entries {
			out <- decode.PBFlowToMap(entry)
		}
	}
}

func (d *InProcess) Write(record *pbflow.Records) {
	d.flowPackets <- record
}

func (d *InProcess) Close() {
	close(d.flowPackets)
}
