package v13

import (
	"fmt"
	"io"
	"net/http"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/mime"
)

type rollingUpgrade struct {
	*cache
	httpClient.HttpClient
}

func (r *rollingUpgrade) url() string {
	return r.cache.url() + "/rolling-upgrade/source-connection"
}

func (r *rollingUpgrade) AddSource(config string, contentType mime.MimeType) (err error) {
	headers := map[string]string{
		"Content-Type": string(contentType),
	}
	rsp, err := r.Post(r.url(), config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	return httpClient.ValidateResponse(rsp, err, "add remote store", http.StatusNoContent)
}

func (r *rollingUpgrade) DisconnectSource() (err error) {
	rsp, err := r.HttpClient.Delete(r.url(), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	return httpClient.ValidateResponse(rsp, err, "disconnect source", http.StatusNoContent)
}

func (r *rollingUpgrade) SourceConnected() (bool, error) {
	rsp, err := r.Head(r.url(), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err := httpClient.ValidateResponse(rsp, err, "source-connected", http.StatusOK, http.StatusNotFound); err != nil {
		return false, err
	}
	return rsp.StatusCode == http.StatusOK, nil
}

func (r *rollingUpgrade) SyncData() (string, error) {
	path := r.cache.url() + "?action=sync-data"
	rsp, err := r.Post(path, "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err = httpClient.ValidateResponse(rsp, err, "syncing data", http.StatusOK); err != nil {
		return "", err
	}

	all, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to decode: %w", err)
	}
	return string(all), nil
}
