package datasource

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/informers"
)

func NewInformerDatasource(kubeconfig string, infConfig informers.Config, kafkaConfig *api.EncodeKafka, opMetrics *operational.Metrics) (Datasource, error) {
	inf := &informers.Informers{}
	if err := inf.InitFromConfig(kubeconfig, infConfig, kafkaConfig, opMetrics); err != nil {
		return nil, err
	}
	return inf, nil
}
