package pipeline

import (
	"context"
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/prometheus"
)

// StartFLPInProcess is an entry point to start the whole FLP / pipeline processing from imported code
func StartFLPInProcess(cfg *config.Root, in chan config.GenericMap) error {
	// Ensure encode/write stages (e.g. S3) observe SIGTERM/SIGINT and can flush.
	// Safe when the host process already registered the same signals.
	if utils.ExitChannel() == nil {
		utils.SetupElegantExit()
	}

	promServer := prometheus.InitializePrometheus(&cfg.MetricsSettings)

	// Create new flows pipeline
	ingester := ingest.NewInProcess(in)
	flp, err := newPipelineFromIngester(cfg, ingester)
	if err != nil {
		return fmt.Errorf("failed to initialize pipeline %w", err)
	}

	// Starts the flows pipeline; blocking call
	go func() {
		flp.Run()
		if promServer != nil {
			_ = promServer.Shutdown(context.Background())
		}
	}()

	return nil
}
