package infinispan

import (
	_ "embed"
	"encoding/json"
	"fmt"

	ispnclient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/mime"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/sirupsen/logrus"

	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	v14 "github.com/infinispan/infinispan-operator/pkg/infinispan/client/v14"
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

func NewCache(kubeConfigPath string) (*Cache, error) {
	log.Debug("Entering NewCache ...")

	client := New(Config{
		Endpoint: "http://cache.netobserv.svc.cluster.local:11222",
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

func (c *Cache) DedupFlows(is bool, isSrcReporter bool, flow config.GenericMap) (config.GenericMap, bool) {
	if !is {
		if isSrcReporter {
			// add flow to cache and stop pipeline for now
			err := c.PutDedupToCache(flow)
			if err != nil {
				log.Errorf("DedupFlows error putting cache %v for %v", err, flow)
			}
			return nil, false
		}

		found, err := c.GetDedup(flow)
		if err != nil {
			log.Errorf("DedupFlows error getting cache %v for %v", err, flow)
		} else if found != nil {
			// append interfaces and direction to flow
			cachedFlow := *found
			log.Debugf("DedupFlows merging cached flow %v with %v ...", cachedFlow, flow)
			if len(cachedFlow[InterfacesField].([]interface{})) == len(cachedFlow[DirectionsField].([]interface{})) {
				for i := 0; i < len(cachedFlow[InterfacesField].([]interface{})); i++ {
					flow[InterfacesField] = append(flow[InterfacesField].([]string), fmt.Sprintf("%s*", cachedFlow[InterfacesField].([]interface{})[i].(string)))
					flow[DirectionsField] = append(flow[DirectionsField].([]int), int(cachedFlow[DirectionsField].([]interface{})[i].(float64)))
				}
			}
			log.Debugf("DedupFlows merged flow: %v", flow)
		}
	}

	return flow, true
}
