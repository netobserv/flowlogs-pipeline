package v14

import (
	"github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	v13 "github.com/infinispan/infinispan-operator/pkg/infinispan/client/v13"
)

type infinispan struct {
	http.HttpClient
	ispn13 api.Infinispan
}

func New(client http.HttpClient) api.Infinispan {
	return &infinispan{
		HttpClient: client,
		ispn13:     v13.New(client),
	}
}

func (i *infinispan) Cache(name string) api.Cache {
	return i.ispn13.Cache(name)
}

func (i *infinispan) Caches() api.Caches {
	return &caches{
		HttpClient: i.HttpClient,
		Caches:     i.ispn13.Caches(),
	}
}

func (i *infinispan) Container() api.Container {
	return i.ispn13.Container()
}

func (i *infinispan) Logging() api.Logging {
	return i.ispn13.Logging()
}

func (i *infinispan) Metrics() api.Metrics {
	return i.ispn13.Metrics()
}

func (i *infinispan) ProtobufMetadataCacheName() string {
	return i.ispn13.ProtobufMetadataCacheName()
}

func (i *infinispan) ScriptCacheName() string {
	return i.ispn13.ScriptCacheName()
}

func (i *infinispan) Server() api.Server {
	return i.ispn13.Server()
}
