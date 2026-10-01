package loki

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/netobserv/loki-client-go/pkg/backoff"
	"github.com/netobserv/loki-client-go/pkg/metrics"

	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"

	"github.com/grafana/loki/pkg/push"
)

const (
	protoContentType = "application/x-protobuf"
	maxErrMsgLen     = 1024

	// Label reserved to override the tenant ID while processing
	// pipeline stages
	ReservedLabelTenantID = "__tenant_id__"

	transportHTTP = "http"
)

var (
	// TODO: Check if changing agent is safe
	UserAgent = fmt.Sprintf("promtail/%s", version.Version)
	log       = slog.With("component", "http-client")
)

func init() {
	metrics.RegisterMetrics()
}

// Client for pushing logs in snappy-compressed protos over HTTP.
type Client struct {
	cfg     *Config
	client  *http.Client
	quit    chan struct{}
	once    sync.Once
	entries chan entry
	wg      sync.WaitGroup

	externalLabels model.LabelSet
}

type entry struct {
	tenantID string
	labels   model.LabelSet
	push.Entry
}

// New makes a new Client from config
func New(cfg *Config) (*Client, error) {
	if cfg.URL.URL == nil {
		return nil, errors.New("client needs target URL")
	}

	c := &Client{
		cfg:     cfg,
		quit:    make(chan struct{}),
		entries: make(chan entry),

		externalLabels: cfg.ExternalLabels.LabelSet,
	}

	err := cfg.Client.Validate()
	if err != nil {
		return nil, err
	}

	// Both the option and the client config have to agree before the transport
	// negotiates HTTP/2, so mirror Client.EnableHTTP2 here. Callers that never
	// set it keep the previous behaviour of HTTP/2 and keep-alives both off.
	var opts []config.HTTPClientOption
	if !cfg.EnableKeepAlives {
		opts = append(opts, config.WithKeepAlivesDisabled())
	}
	if !cfg.Client.EnableHTTP2 {
		opts = append(opts, config.WithHTTP2Disabled())
	}

	c.client, err = config.NewClientFromConfig(cfg.Client, "loki-client", opts...)
	if err != nil {
		return nil, err
	}

	c.client.Timeout = cfg.Timeout

	// Initialize counters to 0 so the metrics are exported before the first
	// occurrence of incrementing to avoid missing metrics.
	for _, counter := range metrics.CountersWithHost {
		counter.WithLabelValues(c.cfg.URL.Host, transportHTTP).Add(0)
	}

	c.wg.Add(1)
	go c.run()
	return c, nil
}

// NewWithDefault creates a new client with default configuration.
func NewWithDefault(url string) (*Client, error) {
	cfg, err := NewDefaultConfig(url)
	if err != nil {
		return nil, err
	}
	return New(&cfg)
}

func (c *Client) run() {
	batches := map[string]*batch{}

	// Given the client handles multiple batches (1 per tenant) and each batch
	// can be created at a different point in time, we look for batches whose
	// max wait time has been reached every 10 times per BatchWait, so that the
	// maximum delay we have sending batches is 10% of the max waiting time.
	// We apply a cap of 10ms to the ticker, to avoid too frequent checks in
	// case the BatchWait is very low.
	minWaitCheckFrequency := 10 * time.Millisecond
	maxWaitCheckFrequency := c.cfg.BatchWait / 10
	if maxWaitCheckFrequency < minWaitCheckFrequency {
		maxWaitCheckFrequency = minWaitCheckFrequency
	}

	maxWaitCheck := time.NewTicker(maxWaitCheckFrequency)

	defer func() {
		// Send all pending batches
		for tenantID, batch := range batches {
			c.sendBatch(tenantID, batch)
		}

		c.wg.Done()
	}()

	for {
		select {
		case <-c.quit:
			return

		case e := <-c.entries:
			batch, ok := batches[e.tenantID]

			// If the batch doesn't exist yet, we create a new one with the entry
			if !ok {
				batches[e.tenantID] = newBatch(e)
				break
			}

			// If adding the entry to the batch will increase the size over the max
			// size allowed, we do send the current batch and then create a new one
			if batch.sizeBytesAfter(e) > c.cfg.BatchSize {
				c.sendBatch(e.tenantID, batch)

				batches[e.tenantID] = newBatch(e)
				break
			}

			// The max size of the batch isn't reached, so we can add the entry
			batch.add(e)

		case <-maxWaitCheck.C:
			// Send all batches whose max wait time has been reached
			for tenantID, batch := range batches {
				if batch.age() < c.cfg.BatchWait {
					continue
				}

				c.sendBatch(tenantID, batch)
				delete(batches, tenantID)
			}
		}
	}
}

func (c *Client) sendBatch(tenantID string, batch *batch) {
	buf, entriesCount, err := batch.encode()
	if err != nil {
		log.Error("error encoding batch", "error", err)
		return
	}
	bufBytes := float64(len(buf))
	metrics.EncodedBytes.WithLabelValues(c.cfg.URL.Host, transportHTTP).Add(bufBytes)

	ctx := context.Background()
	backoff := backoff.New(ctx, c.cfg.BackoffConfig)
	var status int
	for backoff.Ongoing() {
		start := time.Now()
		status, err = c.send(ctx, tenantID, buf)
		metrics.RequestDuration.WithLabelValues(strconv.Itoa(status), c.cfg.URL.Host, transportHTTP).Observe(time.Since(start).Seconds())

		if err == nil {
			metrics.SentBytes.WithLabelValues(c.cfg.URL.Host, transportHTTP).Add(bufBytes)
			metrics.SentEntries.WithLabelValues(c.cfg.URL.Host, transportHTTP).Add(float64(entriesCount))
			return
		}

		// Only retry 429s, 500s and connection-level errors.
		if status > 0 && status != 429 && status/100 != 5 {
			break
		}

		log.Warn("error sending batch, will retry", "status", status, "error", err)
		metrics.BatchRetries.WithLabelValues(c.cfg.URL.Host, transportHTTP).Inc()
		backoff.Wait()
	}

	if err != nil {
		log.Error("final error sending batch", "status", status, "error", err)
		metrics.DroppedBytes.WithLabelValues(c.cfg.URL.Host, transportHTTP).Add(bufBytes)
		metrics.DroppedEntries.WithLabelValues(c.cfg.URL.Host, transportHTTP).Add(float64(entriesCount))
	}
}

func (c *Client) send(ctx context.Context, tenantID string, buf []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequest("POST", c.cfg.URL.String(), bytes.NewReader(buf))
	if err != nil {
		return -1, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", protoContentType)
	req.Header.Set("User-Agent", UserAgent)

	// If the tenant ID is not empty promtail is running in multi-tenant mode, so
	// we should send it to Loki
	if tenantID != "" {
		req.Header.Set("X-Scope-OrgID", tenantID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return -1, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error("error closing response body", "error", err)
		}
	}()

	if resp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(resp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
	} else {
		// A success body is normally empty, but an unread one would stop the
		// transport from reusing the connection when keep-alives are enabled.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrMsgLen))
	}
	return resp.StatusCode, err
}

func (c *Client) getTenantID(labels model.LabelSet) string {
	// Check if it has been overridden while processing the pipeline stages
	if value, ok := labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	// Check if has been specified in the config
	if c.cfg.TenantID != "" {
		return c.cfg.TenantID
	}

	// Defaults to an empty string, which means the X-Scope-OrgID header
	// will not be sent
	return ""
}

// Stop the client.
func (c *Client) Stop() {
	c.once.Do(func() { close(c.quit) })
	c.wg.Wait()
}

// Handle implement EntryHandler; adds a new line to the next batch; send is async.
func (c *Client) Handle(ls model.LabelSet, t time.Time, s string) error {
	if len(c.externalLabels) > 0 {
		ls = c.externalLabels.Merge(ls)
	}

	// Get the tenant  ID in case it has been overridden while processing
	// the pipeline stages, then remove the special label
	tenantID := c.getTenantID(ls)
	if _, ok := ls[ReservedLabelTenantID]; ok {
		// Clone the label set to not manipulate the input one
		ls = ls.Clone()
		delete(ls, ReservedLabelTenantID)
	}

	c.entries <- entry{tenantID, ls, push.Entry{
		Timestamp: t,
		Line:      s,
	}}
	return nil
}
