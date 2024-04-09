package v13

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
)

const (
	CacheManagerPath = BasePath + "/cache-managers/default"
	ContainerPath    = BasePath + "/container"
	HealthPath       = CacheManagerPath + "/health"
	HealthStatusPath = HealthPath + "/status"
)

type container struct {
	httpClient.HttpClient
}

func (c *container) Info() (info *api.ContainerInfo, err error) {
	rsp, err := c.Get(CacheManagerPath, nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, err, "getting cache manager info", http.StatusOK); err != nil {
		return
	}

	if err = json.NewDecoder(rsp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	return
}

func (c *container) HealthStatus() (status api.HealthStatus, err error) {
	rsp, err := c.Get(HealthStatusPath, nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, err, "getting cache manager health status", http.StatusOK); err != nil {
		return
	}
	all, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to decode: %w", err)
	}
	return api.HealthStatus(string(all)), nil
}

func (c *container) Members() (members []string, err error) {
	rsp, err := c.Get(HealthPath, nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, err, "getting cluster members", http.StatusOK); err != nil {
		return
	}

	type Health struct {
		ClusterHealth struct {
			Nodes []string `json:"node_names"`
		} `json:"cluster_health"`
	}

	var health Health
	if err := json.NewDecoder(rsp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	return health.ClusterHealth.Nodes, nil
}

func (c *container) Backups() api.Backups {
	return &backups{c.HttpClient}
}

func (c *container) RebalanceDisable() error {
	return c.rebalance("disable-rebalancing")
}

func (c *container) RebalanceEnable() error {
	return c.rebalance("enable-rebalancing")
}

func (c *container) rebalance(action string) error {
	rsp, err := c.Post(CacheManagerPath+"?action="+action, "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, fmt.Sprintf("during %s", action), http.StatusNoContent)
	return err
}

func (c *container) Restores() api.Restores {
	return &restores{c.HttpClient}
}

func (c *container) Shutdown() (err error) {
	rsp, err := c.Post(ContainerPath+"?action=shutdown", "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "during graceful shutdown", http.StatusNoContent)
	return err
}

func (c *container) Xsite() api.Xsite {
	return &xsite{c.HttpClient}
}
