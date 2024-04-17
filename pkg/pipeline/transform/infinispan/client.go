package infinispan

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
)

type Credentials struct {
	Username string
	Password string
}

type Config struct {
	Endpoint    string
	Credentials *Credentials
}

type Client struct {
	client http.Client
	Config Config
}

func New(c Config) *Client {
	return &Client{
		client: http.Client{},
		Config: c,
	}
}

func (c *Client) getURL(path string) string {
	return fmt.Sprintf("%s/%s", c.Config.Endpoint, path)
}

func (c *Client) Get(path string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.getURL(path), nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

func (c *Client) Head(path string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, c.getURL(path), nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

func (c *Client) Post(path, payload string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, c.getURL(path), strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

func (c *Client) PostMultipart(path string, parts map[string]string, headers map[string]string) (*http.Response, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range parts {
		_ = writer.WriteField(k, v)
	}

	req, err := http.NewRequest(http.MethodPost, c.getURL(path), body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

func (c *Client) Put(path, payload string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPut, c.getURL(path), strings.NewReader(payload))
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}

func (c *Client) Delete(path string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, c.getURL(path), nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.client.Do(req)
}
