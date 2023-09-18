package ingest

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
)

type createIngester = func(*operational.Metrics, config.StageParam) (Ingester, error)

func runIngester(builder createIngester, cfg *config.Ingest, out chan config.GenericMap) (Ingester, error) {
	ingester, err := builder(
		operational.NewMetrics(&config.MetricsSettings{}),
		config.StageParam{Ingest: cfg},
	)
	if err != nil {
		return nil, err
	}
	if out != nil {
		go ingester.Ingest(out)
	}
	return ingester, nil
}
