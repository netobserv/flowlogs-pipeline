package infinispan

import (
	"encoding/json"
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"

	ispnclient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	v14 "github.com/infinispan/infinispan-operator/pkg/infinispan/client/v14"
	"github.com/infinispan/infinispan-operator/pkg/mime"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("component", "transform.Network.Cache")

const (
	// TODO: implements other cache scenarios
	// K8SCacheName          = "K8SCache"
	// ConntrackCacheName    = "Conntrack"
	// MultiClusterCacheName = "MultiCluster"
	DedupCacheName = "deduper"

	InterfacesField = "Interfaces"
	DirectionsField = "IfDirections"
)

type Cache struct {
	ispnclient.HttpClient
	ispn api.Infinispan
}

func NewCache(endpoint string) (*Cache, error) {
	log.Debug("Entering NewCache ...")

	client := New(Config{
		Endpoint: endpoint,
		// TODO: implement credentials
		/*Credentials: &Credentials{
			Username: "netobserv",
			Password: "netobserv",
		},*/
	})
	ispn := v14.New(client)

	return &Cache{
		HttpClient: client,
		ispn:       ispn,
	}, nil
}

func (c *Cache) getDedupKey(v config.GenericMap) string {
	return fmt.Sprintf("%d-%s-%d-%s-%d", v["Proto"], v["SrcAddr"], v["SrcPort"], v["DstAddr"], v["DstPort"])
}

func (c *Cache) PutDedupToCache(v config.GenericMap) error {
	key := c.getDedupKey(v)
	log.Debugf("PutDedupToCache %s ...", key)

	// check if cache already exists
	// An assumption is made that interfaces involved for a 5 tuples will keep being involved in the whole flows sequence
	_, found, err := c.ispn.Cache(DedupCacheName).Get(key)
	if err != nil {
		log.Errorf("PutDedupToCache check existing error %v", err)
		return err
	} else if found {
		log.Debugf("PutDedupToCache %s already exists in cache", key)
		return nil
	}

	// only append interfaces and directions to cached value
	value, err := json.Marshal(config.GenericMap{
		InterfacesField: v[InterfacesField],
		DirectionsField: v[DirectionsField],
	})
	if err != nil {
		log.Errorf("PutDedupToCache marshal error %v", err)
		return err
	}

	return c.ispn.Cache(DedupCacheName).Put(key, string(value), mime.ApplicationProtostream)
}

func (c *Cache) GetDedup(v config.GenericMap) (*config.GenericMap, error) {
	key := c.getDedupKey(v)
	log.Debugf("GetDedup %s ...", key)

	str, found, err := c.ispn.Cache(DedupCacheName).Get(key)
	if err != nil {
		return nil, err
	} else if found {
		gm := config.GenericMap{}
		err = json.Unmarshal([]byte(str), &gm)
		if err != nil {
			log.Errorf("GetDedup unmarshal error %v", err)
		} else {
			log.Debugf("GetDedup found %v in cache", &gm)
		}
		return &gm, err
	}
	return nil, nil
}
