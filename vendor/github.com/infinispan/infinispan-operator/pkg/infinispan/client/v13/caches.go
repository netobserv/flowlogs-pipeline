package v13

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	httpClient "github.com/infinispan/infinispan-operator/pkg/http"
	"github.com/infinispan/infinispan-operator/pkg/infinispan/client/api"
	"github.com/infinispan/infinispan-operator/pkg/mime"
)

const CachesPath = BasePath + "/caches"

type cache struct {
	httpClient.HttpClient
	name string
}

type caches struct {
	httpClient.HttpClient
}

func (c *cache) url() string {
	return fmt.Sprintf("%s/%s", CachesPath, url.PathEscape(c.name))
}

func (c *cache) entryUrl(key string) string {
	return fmt.Sprintf("%s/%s", c.url(), url.PathEscape(key))
}

func (c *cache) Config(contentType mime.MimeType) (config string, err error) {
	path := c.url() + "?action=config"
	rsp, err := c.HttpClient.Get(path, nil)
	if err = httpClient.ValidateResponse(rsp, err, "getting cache config", http.StatusOK); err != nil {
		return
	}
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	var body json.RawMessage
	if err = json.NewDecoder(rsp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("unable to decode: %w", err)
	}
	return string(body), nil
}

func (c *cache) Create(config string, contentType mime.MimeType, flags ...string) (err error) {
	headers := map[string]string{
		"Content-Type": string(contentType),
	}

	if len(flags) > 0 {
		headers["Flags"] = strings.Join(flags, ",")
	}

	rsp, err := c.Post(c.url(), config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "creating cache", http.StatusOK)
	return
}

func (c *cache) CreateWithTemplate(templateName string) (err error) {
	path := fmt.Sprintf("%s?template=%s", c.url(), templateName)
	rsp, err := c.Post(path, "", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "creating cache with template", http.StatusOK)
	return
}

func (c *cache) Delete() (err error) {
	rsp, err := c.HttpClient.Delete(c.url(), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "deleting cache", http.StatusOK, http.StatusNotFound)
	return
}

func (c *cache) Exists() (exist bool, err error) {
	rsp, err := c.Head(c.url(), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	if err = httpClient.ValidateResponse(rsp, err, "validating cache exists", http.StatusOK, http.StatusNoContent, http.StatusNotFound); err != nil {
		return
	}

	switch rsp.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return
	}
	return
}

func (c *cache) Get(key string) (val string, exists bool, err error) {
	rsp, err := c.HttpClient.Get(c.entryUrl(key), nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	if err = httpClient.ValidateResponse(rsp, err, "getting cache entry", http.StatusOK, http.StatusNotFound); err != nil {
		return
	}
	if rsp.StatusCode == http.StatusNotFound {
		return
	}
	body, err := readResponseBody(rsp)
	if err != nil {
		return "", false, err
	}
	return body, true, err
}

func (c *cache) Put(key, value string, contentType mime.MimeType) (err error) {
	headers := map[string]string{
		"Content-Type": string(contentType),
	}
	rsp, err := c.HttpClient.Post(c.entryUrl(key), value, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	if err = httpClient.ValidateResponse(rsp, err, "putting cache entry", http.StatusNoContent); err != nil {
		return
	}
	return nil
}

func (c *cache) Size() (size int, err error) {
	rsp, err := c.HttpClient.Get(c.url()+"?action=size", nil)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	if err = httpClient.ValidateResponse(rsp, err, "get cache size", http.StatusOK); err != nil {
		return
	}
	body, err := readResponseBody(rsp)
	if err != nil {
		return
	}
	return strconv.Atoi(body)
}

func (c *cache) RollingUpgrade() api.RollingUpgrade {
	return &rollingUpgrade{
		cache:      c,
		HttpClient: c.HttpClient,
	}
}

func (c *cache) UpdateConfig(config string, contentType mime.MimeType) (err error) {
	headers := map[string]string{
		"Content-Type": string(contentType),
	}

	rsp, err := c.HttpClient.Put(c.url(), config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "updating cache", http.StatusOK)
	return
}

func (c *caches) ConvertConfiguration(config string, contentType mime.MimeType, reqType mime.MimeType) (transformed string, err error) {
	path := CachesPath + "?action=convert"
	headers := map[string]string{
		"Accept":       string(reqType),
		"Content-Type": string(contentType),
	}
	rsp, err := c.Post(path, config, headers)
	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()
	err = httpClient.ValidateResponse(rsp, err, "converting configuration", http.StatusOK)
	if err != nil {
		return
	}
	return readResponseBody(rsp)
}

// EqualConfiguration is not supported in Infinispan 13
func (c *caches) EqualConfiguration(_, _ string) (bool, error) {
	return false, &api.NotSupportedError{
		Version: MajorVersion,
	}
}

func (c *caches) Names() (names []string, err error) {
	rsp, err := c.Get(CachesPath, nil)
	if err = httpClient.ValidateResponse(rsp, err, "getting caches", http.StatusOK); err != nil {
		return
	}

	defer func() {
		err = httpClient.CloseBody(rsp, err)
	}()

	if err := json.NewDecoder(rsp.Body).Decode(&names); err != nil {
		return nil, fmt.Errorf("unable to decode: %w", err)
	}
	return
}

func readResponseBody(rsp *http.Response) (string, error) {
	responseBody, responseErr := io.ReadAll(rsp.Body)
	if responseErr != nil {
		return "", fmt.Errorf("unable to read response body: %w", responseErr)
	}
	return string(responseBody), nil
}
