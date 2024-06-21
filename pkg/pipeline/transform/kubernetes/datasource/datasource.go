package datasource

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/cni"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
)

type Datasource interface {
	IndexLookup([]cni.SecondaryNetKey, string) *model.ResourceMetaData
	GetNodeByName(string) (*model.ResourceMetaData, error)
}
