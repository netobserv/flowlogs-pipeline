package infinispan

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"

	ispnclient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/mime"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/sirupsen/logrus"

	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	v14 "github.com/infinispan/infinispan-operator/pkg/infinispan/client/v14"
)

var log = logrus.WithField("component", "transform.Network.Cache")
var regex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

const (
	// TODO: implements other cache scenarios
	// K8SCacheName          = "K8SCache"
	// ConntrackCacheName    = "Conntrack"
	// MultiClusterCacheName = "MultiCluster"
	DedupCacheName = "deduper"
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
	return regex.ReplaceAllString(
		fmt.Sprintf("%d-%d-%s-%d-%s-%d", v["TimeFlowEndMs"], v["Proto"], v["SrcAddr"], v["SrcPort"], v["DstAddr"], v["DstPort"]),
		"-",
	)
}

func (c *Cache) PutDedupInnerToCache(v config.GenericMap) error {
	key := c.getDedupKey(v)
	log.Debugf("PutDedupInnerToCache %s ...", key)

	value, err := json.Marshal(v)
	if err != nil {
		log.Errorf("PutDedupInnerToCache marshal error %v", err)
		return err
	}

	return c.ispn.Cache(DedupCacheName).Put(key, string(value), mime.ApplicationProtostream)
}

func (c *Cache) GetDedupInner(v config.GenericMap) (*config.GenericMap, error) {
	key := c.getDedupKey(v)
	log.Debugf("GetDedupInner %s ...", key)

	str, found, err := c.ispn.Cache(DedupCacheName).Get(key)
	if err != nil {
		return nil, err
	} else if found {
		gm := config.GenericMap{}
		err = json.Unmarshal([]byte(str), &gm)
		if err != nil {
			log.Errorf("GetDedupInner unmarshal error %v", err)
		} else {
			log.Debugf("found %v in cache", &gm)
		}
		return &gm, err
	}
	return nil, nil
}
