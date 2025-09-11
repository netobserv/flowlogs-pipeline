package grpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/netobserv/loki-client-go/pkg/backoff"
	"github.com/netobserv/loki-client-go/pkg/logproto"
	"github.com/netobserv/loki-client-go/pkg/metric"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/promql/parser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// Label reserved to override the tenant ID while processing pipeline stages
	ReservedLabelTenantID = "__tenant_id__"

	LatencyLabel = "filename"
	HostLabel    = "host"
	MetricPrefix = "netobserv"
)

var (
	// GRPC-specific metrics
	grpcSentBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_sent_bytes_total",
		Help:      "Number of bytes sent via GRPC.",
	}, []string{HostLabel})
	grpcDroppedBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_dropped_bytes_total",
		Help:      "Number of bytes dropped because failed to be sent via GRPC after all retries.",
	}, []string{HostLabel})
	grpcSentEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_sent_entries_total",
		Help:      "Number of log entries sent via GRPC.",
	}, []string{HostLabel})
	grpcDroppedEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_dropped_entries_total",
		Help:      "Number of log entries dropped because failed to be sent via GRPC after all retries.",
	}, []string{HostLabel})
	grpcRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_request_duration_seconds",
		Help:      "Duration of GRPC requests.",
	}, []string{"status_code", HostLabel})
	grpcBatchRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_batch_retries_total",
		Help:      "Number of times GRPC batches has had to be retried.",
	}, []string{HostLabel})
	grpcConnectionStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: MetricPrefix,
		Name:      "loki_grpc_connection_status",
		Help:      "Status of GRPC connection (1 = connected, 0 = disconnected).",
	}, []string{HostLabel})
	grpcStreamLag *metric.Gauges

	grpcCountersWithHost = []*prometheus.CounterVec{
		grpcSentBytes, grpcDroppedBytes, grpcSentEntries, grpcDroppedEntries,
	}

	UserAgent = fmt.Sprintf("loki-grpc-client/%s", version.Version)
)

func init() {
	prometheus.MustRegister(grpcSentBytes)
	prometheus.MustRegister(grpcDroppedBytes)
	prometheus.MustRegister(grpcSentEntries)
	prometheus.MustRegister(grpcDroppedEntries)
	prometheus.MustRegister(grpcRequestDuration)
	prometheus.MustRegister(grpcBatchRetries)
	prometheus.MustRegister(grpcConnectionStatus)

	var err error
	grpcStreamLag, err = metric.NewGauges(MetricPrefix+"_loki_grpc_stream_lag_seconds",
		"Difference between current time and last batch timestamp for successful GRPC sends",
		metric.GaugeConfig{Action: "set"},
		int64(1*time.Minute.Seconds()),
	)
	if err != nil {
		panic(err)
	}
	prometheus.MustRegister(grpcStreamLag)
}

// Client for pushing logs via GRPC
type Client struct {
	logger  log.Logger
	cfg     Config
	conn    *grpc.ClientConn
	pusher  logproto.PusherClient
	quit    chan struct{}
	once    sync.Once
	entries chan entry
	wg      sync.WaitGroup

	externalLabels model.LabelSet

	// Connection management
	connMux   sync.RWMutex
	connected bool
}

// New creates a new GRPC client from config
func New(cfg Config) (*Client, error) {
	logger := level.NewFilter(log.NewLogfmtLogger(os.Stdout), level.AllowWarn())
	return NewWithLogger(cfg, logger)
}

// NewWithDefault creates a new client with default configuration
func NewWithDefault(serverAddress string) (*Client, error) {
	cfg, err := NewDefaultConfig(serverAddress)
	if err != nil {
		return nil, err
	}
	return New(cfg)
}

// NewWithLogger creates a new GRPC client with a logger and config
func NewWithLogger(cfg Config, logger log.Logger) (*Client, error) {
	if cfg.ServerAddress == "" {
		return nil, errors.New("grpc client needs server address")
	}

	c := &Client{
		logger:         log.With(logger, "component", "grpc-client", "host", cfg.ServerAddress),
		cfg:            cfg,
		quit:           make(chan struct{}),
		entries:        make(chan entry),
		externalLabels: cfg.ExternalLabels.LabelSet,
	}

	// Initialize connection
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to GRPC server: %w", err)
	}

	// Initialize counters to 0
	for _, counter := range grpcCountersWithHost {
		counter.WithLabelValues(c.cfg.ServerAddress).Add(0)
	}

	grpcConnectionStatus.WithLabelValues(c.cfg.ServerAddress).Set(1)

	c.wg.Add(1)
	go c.run()
	return c, nil
}

// connect establishes GRPC connection
func (c *Client) connect() error {
	opts, err := c.cfg.BuildDialOptions()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(c.cfg.ServerAddress, opts...)
	if err != nil {
		return err
	}

	c.connMux.Lock()
	c.conn = conn
	c.pusher = logproto.NewPusherClient(conn)
	c.connected = true
	c.connMux.Unlock()

	level.Info(c.logger).Log("msg", "connected to GRPC server", "address", c.cfg.ServerAddress)
	return nil
}

// isConnected returns connection status
func (c *Client) isConnected() bool {
	c.connMux.RLock()
	defer c.connMux.RUnlock()
	return c.connected
}

// reconnect attempts to reconnect to the server
func (c *Client) reconnect() error {
	c.connMux.Lock()
	defer c.connMux.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}

	c.connected = false
	grpcConnectionStatus.WithLabelValues(c.cfg.ServerAddress).Set(0)

	opts, err := c.cfg.BuildDialOptions()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(c.cfg.ServerAddress, opts...)
	if err != nil {
		return err
	}

	c.conn = conn
	c.pusher = logproto.NewPusherClient(conn)
	c.connected = true
	grpcConnectionStatus.WithLabelValues(c.cfg.ServerAddress).Set(1)

	level.Info(c.logger).Log("msg", "reconnected to GRPC server", "address", c.cfg.ServerAddress)
	return nil
}

func (c *Client) run() {
	batches := map[string]*batch{}

	// Batch timer logic similar to HTTP client
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

			// Create new batch if doesn't exist
			if !ok {
				batches[e.tenantID] = newBatch(e.tenantID, e)
				break
			}

			// Send batch if adding entry would exceed max size
			if batch.sizeBytesAfter(e) > c.cfg.BatchSize {
				c.sendBatch(e.tenantID, batch)
				batches[e.tenantID] = newBatch(e.tenantID, e)
				break
			}

			// Add entry to existing batch
			batch.add(e)

		case <-maxWaitCheck.C:
			// Send batches that have reached max wait time
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
	req, entriesCount := batch.createPushRequest()

	if len(req.Streams) == 0 {
		return
	}

	ctx := context.Background()
	backoffConfig := backoff.New(ctx, c.cfg.BackoffConfig)

	var err error
	for backoffConfig.Ongoing() {
		start := time.Now()
		err = c.push(ctx, tenantID, req)

		// Convert error to status code for metrics
		statusCode := c.getStatusCode(err)
		grpcRequestDuration.WithLabelValues(statusCode, c.cfg.ServerAddress).Observe(time.Since(start).Seconds())

		if err == nil {
			// Success metrics
			grpcSentEntries.WithLabelValues(c.cfg.ServerAddress).Add(float64(entriesCount))
			for _, s := range req.Streams {
				lbls, parseErr := parser.ParseMetric(s.Labels)
				if parseErr != nil {
					level.Warn(c.logger).Log("msg", "error parsing stream labels for lag metric", "error", parseErr)
					continue
				}

				var lblSet model.LabelSet
				for i := range lbls {
					if lbls[i].Name == LatencyLabel {
						lblSet = model.LabelSet{
							model.LabelName(HostLabel):    model.LabelValue(c.cfg.ServerAddress),
							model.LabelName(LatencyLabel): model.LabelValue(lbls[i].Value),
						}
						break
					}
				}
				if lblSet != nil && len(s.Entries) > 0 {
					grpcStreamLag.With(lblSet).Set(time.Since(s.Entries[len(s.Entries)-1].Timestamp).Seconds())
				}
			}
			return
		}

		// Check if we should retry
		if !c.shouldRetry(err) {
			break
		}

		level.Warn(c.logger).Log("msg", "error sending batch via GRPC, will retry", "error", err)
		grpcBatchRetries.WithLabelValues(c.cfg.ServerAddress).Inc()
		backoffConfig.Wait()
	}

	// Failed after all retries
	level.Error(c.logger).Log("msg", "final error sending batch via GRPC", "error", err)
	grpcDroppedEntries.WithLabelValues(c.cfg.ServerAddress).Add(float64(entriesCount))
}

func (c *Client) push(ctx context.Context, tenantID string, req *logproto.PushRequest) error {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	// Add tenant ID to metadata if specified
	if tenantID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-scope-orgid", tenantID)
	}

	// Add user agent
	ctx = metadata.AppendToOutgoingContext(ctx, "user-agent", UserAgent)

	// Ensure we're connected
	if !c.isConnected() {
		if err := c.reconnect(); err != nil {
			return fmt.Errorf("failed to reconnect: %w", err)
		}
	}

	c.connMux.RLock()
	pusher := c.pusher
	c.connMux.RUnlock()

	_, err := pusher.Push(ctx, req)
	if err != nil {
		// Handle connection errors
		if isConnectionError(err) {
			c.connMux.Lock()
			c.connected = false
			grpcConnectionStatus.WithLabelValues(c.cfg.ServerAddress).Set(0)
			c.connMux.Unlock()
		}
		return err
	}

	return nil
}

func (c *Client) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		// Non-GRPC errors might be connection issues, retry
		return true
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Aborted, codes.Internal, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

func (c *Client) getStatusCode(err error) string {
	if err == nil {
		return "OK"
	}

	st, ok := status.FromError(err)
	if !ok {
		return "Unknown"
	}

	return st.Code().String()
}

func isConnectionError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return true // Non-GRPC errors are likely connection issues
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

func (c *Client) getTenantID(labels model.LabelSet) string {
	// Check if overridden in pipeline stages
	if value, ok := labels[ReservedLabelTenantID]; ok {
		return string(value)
	}

	// Check config
	if c.cfg.TenantID != "" {
		return c.cfg.TenantID
	}

	return ""
}

// Stop the client
func (c *Client) Stop() {
	c.once.Do(func() {
		close(c.quit)

		c.connMux.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.connected = false
			grpcConnectionStatus.WithLabelValues(c.cfg.ServerAddress).Set(0)
		}
		c.connMux.Unlock()
	})
	c.wg.Wait()
}

// Handle implements EntryHandler; adds a new line to the next batch; send is async
func (c *Client) Handle(ls model.LabelSet, t time.Time, s string) error {
	if len(c.externalLabels) > 0 {
		ls = c.externalLabels.Merge(ls)
	}

	// Get tenant ID and remove special label
	tenantID := c.getTenantID(ls)
	if _, ok := ls[ReservedLabelTenantID]; ok {
		ls = ls.Clone()
		delete(ls, ReservedLabelTenantID)
	}

	c.entries <- entry{tenantID, ls, logproto.Entry{
		Timestamp: t,
		Line:      s,
	}}
	return nil
}

func (c *Client) UnregisterLatencyMetric(labels model.LabelSet) {
	labels[HostLabel] = model.LabelValue(c.cfg.ServerAddress)
	grpcStreamLag.Delete(labels)
}
