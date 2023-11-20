package pipeline

import (
	"context"
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/prometheus"
	"github.com/netobserv/netobserv-ebpf-agent/pkg/pbflow"
)

// StartFLPInProcess is an entry point to start the whole FLP / pipeline processing from imported code
func StartFLPInProcess(cfg *config.ConfigFileStruct) (*ingest.InProcess, error) {
	promServer := prometheus.InitializePrometheus(&cfg.MetricsSettings)

	// Create new flows pipeline
	ingester := ingest.NewInProcess(make(chan *pbflow.Records, 100))
	flp, err := newPipelineFromIngester(cfg, ingester)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize pipeline %w", err)
	}

	// Starts the flows pipeline; blocking call
	go func() {
		flp.Run()
		if promServer != nil {
			_ = promServer.Shutdown(context.Background())
		}
	}()

	return ingester, nil
}
