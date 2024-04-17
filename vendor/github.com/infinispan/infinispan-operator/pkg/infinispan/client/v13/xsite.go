package v13

import (
	"encoding/json"
	"fmt"
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
)

const XSitePath = CacheManagerPath + "/x-site/backups"

type xsite struct {
	httpClient.HttpClient
}

func (x *xsite) PushAllState() (err error) {
	rsp, err := x.Get(XSitePath, nil)
	if err = httpClient.ValidateResponse(rsp, err, "Retrieving xsite status", http.StatusOK); err != nil {
		return
	}

	defer func() {
		cerr := rsp.Body.Close()
		if err == nil {
			err = cerr
		}
	}()

	type xsiteStatus struct {
		Status string `json:"status"`
	}
	var statuses map[string]xsiteStatus
	if err := json.NewDecoder(rsp.Body).Decode(&statuses); err != nil {
		return fmt.Errorf("unable to decode: %w", err)
	}

	// Statuses will be empty if no xsite caches are configured
	for k, v := range statuses {
		if v.Status == "online" {
			url := fmt.Sprintf("%s/%s?action=start-push-state", XSitePath, k)
			rsp, err = x.Post(url, "", nil)
			if err = httpClient.ValidateResponse(rsp, err, "Pushing xsite state", http.StatusOK); err != nil {
				return
			}
		}
	}
	return
}
