package k8scache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var clog = log.WithField("component", "k8scache.Client")

const (
	// Reconnection configuration
	maxReconnectAttempts = 10
	initialBackoff       = 1 * time.Second
	maxBackoff           = 60 * time.Second
	backoffMultiplier    = 2.0
	// Default values for configurable parameters
	defaultUpdateBufferSize = 100
	defaultSendTimeout      = 10 * time.Second
	defaultBatchSize        = 100
)

// ClientConfig holds configuration for the gRPC client
type ClientConfig struct {
	// ProcessorID identifies this informer instance
	ProcessorID string
	// TLS configuration (optional)
	TLSEnabled  bool
	TLSCertPath string
	TLSKeyPath  string
	TLSCAPath   string
	// TLSServerName is the expected server name for TLS verification (optional).
	// If set, this name will be used to validate the server certificate regardless of the connection address.
	// Useful when connecting to pods by IP but validating against DNS names in the certificate.
	TLSServerName string
	// InsecureSkipVerify skips TLS certificate verification (not recommended for production)
	InsecureSkipVerify bool
	// Cache configuration (optional, defaults used if 0)
	UpdateBufferSize int           // Size of the update channel buffer (default: 100)
	SendTimeout      time.Duration // Timeout for sending updates to processors (default: 10s)
	BatchSize        int           // Maximum number of entries to send in a single update (default: 100)
}

// InformerDataSource defines the interface for getting all resources from informers.
type InformerDataSource interface {
	GetAllResources() []*model.ResourceMetaData
}

// Client manages gRPC connections to FLP processor servers and pushes cache updates.
type Client struct {
	// processorID identifies this informer instance
	processorID string
	// TLS configuration
	tlsConfig ClientConfig
	// connections tracks active processor connections
	connections map[string]*processorConnection
	mu          sync.RWMutex
	// version tracks the current cache version
	version atomic.Int64
	// updateChan receives cache updates from informers
	updateChan chan *CacheUpdate
	// ctx and cancel for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	// Configurable parameters
	sendTimeout time.Duration
	batchSize   int
	// Informer data source for snapshots
	informer InformerDataSource
	infMu    sync.RWMutex
}

// processorConnection represents a connection to a single FLP processor
type processorConnection struct {
	address string
	conn    *grpc.ClientConn
	stream  KubernetesCacheService_StreamUpdatesClient
	// Track if connection is healthy
	healthy atomic.Bool
	// Cancel function for this connection's context
	cancel context.CancelFunc
	// Reconnect tracking
	reconnectAttempts int
	mu                sync.Mutex // Protects conn, stream, cancel, and reconnectAttempts
}

// getStream returns a copy of the stream pointer under lock.
// The caller must not hold pc.mu when calling this method.
func (pc *processorConnection) getStream() KubernetesCacheService_StreamUpdatesClient {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.stream
}

// cancelStream cancels the stream context, which will cause any blocked Send/Recv to return an error.
// This is used when a timeout occurs to unblock operations and trigger reconnection.
// The caller must not hold pc.mu when calling this method.
func (pc *processorConnection) cancelStream() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if pc.cancel != nil {
		pc.cancel()
	}
}

// closeConnection closes the connection and cancels the context under lock.
// The caller must not hold pc.mu when calling this method.
func (pc *processorConnection) closeConnection() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if pc.cancel != nil {
		pc.cancel()
	}
	if pc.conn != nil {
		pc.conn.Close()
	}
}

// NewClient creates a new cache client (informer side)
func NewClient(config *ClientConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	// Set defaults for configurable parameters if not provided
	bufferSize := config.UpdateBufferSize
	if bufferSize == 0 {
		bufferSize = defaultUpdateBufferSize
	}

	sendTimeout := config.SendTimeout
	if sendTimeout == 0 {
		sendTimeout = defaultSendTimeout
	}

	batchSize := config.BatchSize
	if batchSize == 0 {
		batchSize = defaultBatchSize
	}

	clog.WithFields(log.Fields{
		"update_buffer_size": bufferSize,
		"send_timeout":       sendTimeout,
		"batch_size":         batchSize,
	}).Info("Cache client configuration")

	return &Client{
		processorID: config.ProcessorID,
		tlsConfig:   *config,
		connections: make(map[string]*processorConnection),
		updateChan:  make(chan *CacheUpdate, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
		sendTimeout: sendTimeout,
		batchSize:   batchSize,
	}
}

// getTransportCredentials creates gRPC transport credentials based on TLS configuration
func (c *Client) getTransportCredentials() (credentials.TransportCredentials, error) {
	if !c.tlsConfig.TLSEnabled {
		clog.Debug("Using insecure credentials (TLS disabled)")
		return insecure.NewCredentials(), nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.tlsConfig.InsecureSkipVerify,
	}

	// Set ServerName if provided (allows connecting by IP while validating against DNS name in certificate)
	if c.tlsConfig.TLSServerName != "" {
		tlsConfig.ServerName = c.tlsConfig.TLSServerName
		clog.WithField("server_name", c.tlsConfig.TLSServerName).Debug("TLS ServerName override configured")
	}

	// Load client cert/key if provided
	if c.tlsConfig.TLSCertPath != "" && c.tlsConfig.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(c.tlsConfig.TLSCertPath, c.tlsConfig.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		clog.WithFields(log.Fields{
			"cert": c.tlsConfig.TLSCertPath,
			"key":  c.tlsConfig.TLSKeyPath,
		}).Debug("Loaded client certificate")
	}

	// Load CA cert if provided
	if c.tlsConfig.TLSCAPath != "" {
		caCert, err := os.ReadFile(c.tlsConfig.TLSCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.RootCAs = caCertPool
		clog.WithField("ca", c.tlsConfig.TLSCAPath).Debug("Loaded CA certificate")
	}

	return credentials.NewTLS(tlsConfig), nil
}

// AddProcessor connects to a new FLP processor server
func (c *Client) AddProcessor(address string) error {
	return c.AddProcessorWithTimeout(address, 30*time.Second)
}

// AddProcessorWithTimeout connects to a new FLP processor server with a timeout
func (c *Client) AddProcessorWithTimeout(address string, timeout time.Duration) error {
	// First check: quick lock to see if already connected
	c.mu.Lock()
	if _, exists := c.connections[address]; exists {
		c.mu.Unlock()
		clog.WithField("address", address).Debug("processor already connected")
		return nil
	}
	c.mu.Unlock()

	clog.WithField("address", address).Info("connecting to FLP processor")

	// Create a context with timeout for the connection attempt
	ctx, cancel := context.WithTimeout(c.ctx, timeout)
	defer cancel()

	// Perform slow network operations without holding the lock
	// Get transport credentials
	creds, err := c.getTransportCredentials()
	if err != nil {
		return fmt.Errorf("failed to create transport credentials: %w", err)
	}

	// Create connection with context
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)), // 50MB max message size
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Create stream with timeout context
	client := NewKubernetesCacheServiceClient(conn)
	streamCtx, streamCancel := context.WithCancel(c.ctx)

	// Use a channel to implement timeout for stream creation
	type streamResult struct {
		stream KubernetesCacheService_StreamUpdatesClient
		err    error
	}
	streamChan := make(chan streamResult, 1)

	go func() {
		stream, err := client.StreamUpdates(streamCtx)
		select {
		case streamChan <- streamResult{stream: stream, err: err}:
		case <-ctx.Done():
			// Timeout occurred during stream creation, attempt cleanup
			if stream != nil {
				if closeErr := stream.CloseSend(); closeErr != nil {
					clog.WithError(closeErr).WithField("address", address).
						Debug("failed to close stream during timeout cleanup")
				}
			}
		}
	}()

	var stream KubernetesCacheService_StreamUpdatesClient
	select {
	case result := <-streamChan:
		if result.err != nil {
			streamCancel()
			conn.Close()
			return fmt.Errorf("failed to create stream to %s: %w", address, result.err)
		}
		stream = result.stream
	case <-ctx.Done():
		streamCancel()
		conn.Close()
		return fmt.Errorf("timeout connecting to %s: %w", address, ctx.Err())
	}

	// Second check: re-acquire lock and verify no other goroutine added this connection
	c.mu.Lock()
	if _, exists := c.connections[address]; exists {
		c.mu.Unlock()
		// Another goroutine added this connection while we were connecting
		// Clean up our newly-created resources
		streamCancel()
		conn.Close()
		clog.WithField("address", address).Debug("processor was connected by another goroutine, discarding duplicate")
		return nil
	}

	// We won the race, store the connection
	pc := &processorConnection{
		address: address,
		conn:    conn,
		stream:  stream,
		cancel:  streamCancel,
	}
	pc.healthy.Store(true)

	c.connections[address] = pc
	c.mu.Unlock()

	// Start receiver goroutine for this connection
	go c.receiveFromProcessor(pc)

	clog.WithField("address", address).Info("connected to FLP processor")
	return nil
}

// RemoveProcessor disconnects from a FLP processor
func (c *Client) RemoveProcessor(address string) {
	c.mu.Lock()
	pc, exists := c.connections[address]
	if !exists {
		c.mu.Unlock()
		return
	}

	clog.WithField("address", address).Info("disconnecting from FLP processor")
	delete(c.connections, address)
	c.mu.Unlock()

	// Close connection after releasing client lock to avoid holding both locks
	pc.closeConnection()
}

// RemoveStaleProcessors removes connections to processors that are no longer in the discovered set.
// This is called after discovery to clean up connections to pods that have been deleted or restarted
// with new IPs. Prevents memory leaks and zombie connections.
func (c *Client) RemoveStaleProcessors(discoveredAddresses map[string]bool) {
	c.mu.RLock()
	var staleAddresses []string
	for address := range c.connections {
		if !discoveredAddresses[address] {
			staleAddresses = append(staleAddresses, address)
		}
	}
	c.mu.RUnlock()

	if len(staleAddresses) > 0 {
		clog.WithFields(log.Fields{
			"num_stale":       len(staleAddresses),
			"stale_addresses": staleAddresses,
		}).Info("removing stale processor connections")

		for _, address := range staleAddresses {
			c.RemoveProcessor(address)
		}
	}
}

// Start begins processing cache updates and sending them to all connected processors
func (c *Client) Start() {
	go c.processorLoop()
}

// Stop shuts down the client
func (c *Client) Stop() {
	c.cancel()
	c.mu.Lock()
	connections := make([]*processorConnection, 0, len(c.connections))
	for _, pc := range c.connections {
		connections = append(connections, pc)
	}
	c.connections = make(map[string]*processorConnection)
	c.mu.Unlock()

	// Close all connections after releasing client lock
	for _, pc := range connections {
		pc.closeConnection()
	}
}

// SendAdd sends an ADD operation to all connected processors
// Entries are sent in batches according to the configured batch size
func (c *Client) SendAdd(entries []*model.ResourceMetaData) error {
	return c.sendBatched(entries, OperationType_OPERATION_ADD, false)
}

// SendUpdate sends an UPDATE operation to all connected processors
// Entries are sent in batches according to the configured batch size
func (c *Client) SendUpdate(entries []*model.ResourceMetaData) error {
	return c.sendBatched(entries, OperationType_OPERATION_UPDATE, false)
}

// SendDelete sends a DELETE operation to all connected processors
// Entries are sent in batches according to the configured batch size
func (c *Client) SendDelete(entries []*model.ResourceMetaData) error {
	return c.sendBatched(entries, OperationType_OPERATION_DELETE, false)
}

// sendBatched sends entries in batches to all connected processors
func (c *Client) sendBatched(entries []*model.ResourceMetaData, operation OperationType, isSnapshot bool) error {
	if len(entries) == 0 {
		return nil
	}

	batchSize := c.batchSize
	numBatches := (len(entries) + batchSize - 1) / batchSize

	if numBatches > 1 {
		clog.WithFields(log.Fields{
			"total_entries": len(entries),
			"batch_size":    batchSize,
			"num_batches":   numBatches,
			"operation":     operation,
		}).Debug("sending entries in batches")
	}

	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		batch := entries[i:end]
		version := c.version.Add(1)
		update := &CacheUpdate{
			Version:    version,
			IsSnapshot: isSnapshot,
			Operation:  operation,
			Entries:    metaToResourceEntries(batch),
		}

		if err := c.sendUpdate(update); err != nil {
			return fmt.Errorf("failed to send batch %d/%d: %w", (i/batchSize)+1, numBatches, err)
		}
	}

	return nil
}

// SendSnapshot sends a full snapshot to a specific processor.
// This is used when a processor connects/restarts to get the current state.
func (c *Client) SendSnapshot(entries []*model.ResourceMetaData, targetAddress string) error {
	batchSize := c.batchSize
	numBatches := (len(entries) + batchSize - 1) / batchSize

	clog.WithFields(log.Fields{
		"num_entries": len(entries),
		"batch_size":  batchSize,
		"num_batches": numBatches,
		"target":      targetAddress,
	}).Info("sending snapshot to processor")

	// Split entries into batches if needed
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		batch := entries[i:end]
		version := c.version.Add(1)
		update := &CacheUpdate{
			Version:    version,
			IsSnapshot: true,
			Operation:  OperationType_OPERATION_ADD,
			Entries:    metaToResourceEntries(batch),
		}

		// Send directly to the target processor
		c.mu.RLock()
		pc, exists := c.connections[targetAddress]
		c.mu.RUnlock()

		if !exists {
			return fmt.Errorf("processor %s not found", targetAddress)
		}

		if !pc.healthy.Load() {
			return fmt.Errorf("processor %s is not healthy", targetAddress)
		}

		stream := pc.getStream()
		if stream == nil {
			return fmt.Errorf("stream is nil for processor %s", targetAddress)
		}

		sendCtx, sendCancel := context.WithTimeout(context.Background(), c.sendTimeout)
		done := make(chan error, 1)

		go func() {
			select {
			case <-sendCtx.Done():
				return
			case done <- stream.Send(update):
			}
		}()

		select {
		case err := <-done:
			sendCancel()
			if err != nil {
				return fmt.Errorf("failed to send snapshot batch to %s: %w", targetAddress, err)
			}
		case <-sendCtx.Done():
			sendCancel()
			pc.cancelStream()
			return fmt.Errorf("timeout sending snapshot batch to %s", targetAddress)
		}

		clog.WithFields(log.Fields{
			"batch":       (i / batchSize) + 1,
			"num_batches": numBatches,
			"batch_size":  len(batch),
			"target":      targetAddress,
		}).Debug("sent snapshot batch")
	}

	clog.WithField("target", targetAddress).Info("snapshot sent successfully")
	return nil
}

// sendUpdate sends an update to the update channel (non-blocking with timeout)
func (c *Client) sendUpdate(update *CacheUpdate) error {
	select {
	case c.updateChan <- update:
		return nil
	case <-time.After(c.sendTimeout):
		return fmt.Errorf("timeout sending update to channel")
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// processorLoop reads from updateChan and sends to all connected processors
func (c *Client) processorLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case update := <-c.updateChan:
			c.broadcastUpdate(update)
		}
	}
}

// broadcastUpdate sends an update to all connected processors
func (c *Client) broadcastUpdate(update *CacheUpdate) {
	c.mu.RLock()
	connections := make([]*processorConnection, 0, len(c.connections))
	for _, pc := range c.connections {
		if pc.healthy.Load() {
			connections = append(connections, pc)
		}
	}
	c.mu.RUnlock()

	if len(connections) == 0 {
		clog.Warn("no healthy processor connections to send update")
		return
	}

	clog.WithFields(log.Fields{
		"version":     update.Version,
		"is_snapshot": update.IsSnapshot,
		"operation":   update.Operation,
		"num_entries": len(update.Entries),
		"num_targets": len(connections),
	}).Debug("broadcasting cache update")

	// Send to all processors concurrently
	var wg sync.WaitGroup
	for _, pc := range connections {
		wg.Add(1)
		go func(pc *processorConnection) {
			defer wg.Done()

			// Get stream reference under lock
			stream := pc.getStream()
			if stream == nil {
				clog.WithField("address", pc.address).Warn("stream is nil, skipping send")
				pc.healthy.Store(false)
				return
			}

			// Create a context with timeout for this send operation
			sendCtx, sendCancel := context.WithTimeout(context.Background(), c.sendTimeout)
			defer sendCancel()

			// Use a channel to signal completion or timeout
			done := make(chan error, 1)
			go func() {
				select {
				case <-sendCtx.Done():
					// Context cancelled, exit goroutine to prevent leak
					return
				case done <- stream.Send(update):
					// Send completed (success or error)
				}
			}()

			select {
			case err := <-done:
				if err != nil {
					clog.WithError(err).WithField("address", pc.address).Error("failed to send update")
					pc.healthy.Store(false)
				}
			case <-sendCtx.Done():
				clog.WithField("address", pc.address).Error("send operation timed out")
				pc.healthy.Store(false)

				// Cancel the stream context to ensure any blocked operations are unblocked
				// This will cause receiveFromProcessor to see an error and trigger reconnection
				pc.cancelStream()
			}
		}(pc)
	}
	wg.Wait()
}

// reconnect attempts to reconnect to a failed processor with exponential backoff
func (c *Client) reconnect(pc *processorConnection) bool {
	backoff := initialBackoff
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		// Lock pc.mu for updating reconnect attempts
		pc.mu.Lock()
		pc.reconnectAttempts = attempt
		address := pc.address
		pc.mu.Unlock()

		clog.WithFields(log.Fields{
			"address": address,
			"attempt": attempt,
			"backoff": backoff,
		}).Info("attempting to reconnect")

		// Wait before retry (with context cancellation support)
		select {
		case <-time.After(backoff):
		case <-c.ctx.Done():
			clog.WithField("address", address).Info("reconnection cancelled")
			return false
		}

		// Close old connection under lock
		pc.mu.Lock()
		if pc.conn != nil {
			pc.conn.Close()
		}
		if pc.cancel != nil {
			pc.cancel()
		}
		pc.mu.Unlock()

		// Get transport credentials (without holding any locks)
		creds, err := c.getTransportCredentials()
		if err != nil {
			clog.WithError(err).WithField("address", address).Warn("reconnect: failed to create transport credentials")
			backoff = min(time.Duration(float64(backoff)*backoffMultiplier), maxBackoff)
			continue
		}

		// Create new connection (without holding any locks)
		conn, err := grpc.NewClient(address,
			grpc.WithTransportCredentials(creds),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
		)
		if err != nil {
			clog.WithError(err).WithField("address", address).Warn("reconnect: failed to connect")
			backoff = min(time.Duration(float64(backoff)*backoffMultiplier), maxBackoff)
			continue
		}

		// Create new stream (without holding any locks)
		client := NewKubernetesCacheServiceClient(conn)
		ctx, cancel := context.WithCancel(c.ctx)
		stream, err := client.StreamUpdates(ctx)
		if err != nil {
			clog.WithError(err).WithField("address", address).Warn("reconnect: failed to create stream")
			cancel()
			conn.Close()
			backoff = min(time.Duration(float64(backoff)*backoffMultiplier), maxBackoff)
			continue
		}

		// Update connection under lock
		pc.mu.Lock()
		pc.conn = conn
		pc.stream = stream
		pc.cancel = cancel
		pc.healthy.Store(true)
		pc.reconnectAttempts = 0
		pc.mu.Unlock()

		clog.WithFields(log.Fields{
			"address": address,
			"attempt": attempt,
		}).Info("reconnection successful")

		return true
	}

	clog.WithFields(log.Fields{
		"address":  pc.address,
		"attempts": maxReconnectAttempts,
	}).Error("reconnection failed after max attempts, removing processor")

	// Remove from connections map after exhausting retries
	// This is safe now as we don't hold pc.mu when acquiring c.mu
	c.mu.Lock()
	delete(c.connections, pc.address)
	c.mu.Unlock()

	return false
}

// receiveFromProcessor handles incoming messages from a processor (SyncRequest/SyncAck)
func (c *Client) receiveFromProcessor(pc *processorConnection) {
	defer func() {
		pc.healthy.Store(false)
		clog.WithField("address", pc.address).Info("receiver goroutine stopped")
	}()

	for {
		// Get stream reference under lock
		stream := pc.getStream()
		if stream == nil {
			clog.WithField("address", pc.address).Warn("stream is nil in receiver, exiting")
			return
		}

		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				clog.WithField("address", pc.address).Info("processor disconnected (EOF)")
			} else {
				clog.WithError(err).WithField("address", pc.address).Warn("error receiving from processor")
			}

			// Mark as unhealthy and attempt reconnection
			pc.healthy.Store(false)

			// Attempt to reconnect
			if c.reconnect(pc) {
				// Reconnection successful, restart receiver loop
				clog.WithField("address", pc.address).Info("restarting receiver after successful reconnection")
				continue
			}

			// Reconnection failed, exit
			return
		}

		switch m := msg.Message.(type) {
		case *SyncMessage_Request:
			c.handleSyncRequest(pc, m.Request)
		case *SyncMessage_Ack:
			c.handleSyncAck(pc, m.Ack)
		default:
			clog.WithField("address", pc.address).Warn("received unknown message type")
		}
	}
}

// handleSyncRequest handles a SyncRequest from a processor
func (c *Client) handleSyncRequest(pc *processorConnection, req *SyncRequest) {
	clog.WithFields(log.Fields{
		"address":      pc.address,
		"processor_id": req.ProcessorId,
		"last_version": req.LastVersion,
	}).Info("received SyncRequest from processor")

	// Only send snapshot if LastVersion is 0 (processor is new or restarted)
	// If LastVersion > 0, the processor is reconnecting and will continue receiving
	// incremental updates (ADD/UPDATE/DELETE) from where it left off
	if req.LastVersion == 0 {
		// Get informer data source
		c.infMu.RLock()
		informer := c.informer
		c.infMu.RUnlock()

		if informer == nil {
			clog.Warn("informer not set, cannot send snapshot")
			return
		}

		// Get all resources from informer cache (local, no K8s API query)
		allResources := informer.GetAllResources()

		clog.WithFields(log.Fields{
			"address":       pc.address,
			"processor_id":  req.ProcessorId,
			"num_resources": len(allResources),
		}).Info("sending snapshot to new/restarted processor")

		// Send snapshot to this specific processor
		if err := c.SendSnapshot(allResources, pc.address); err != nil {
			clog.WithError(err).WithField("address", pc.address).Error("failed to send snapshot")
		} else {
			clog.WithField("address", pc.address).Info("snapshot sent successfully to processor")
		}
	} else {
		clog.WithFields(log.Fields{
			"address":      pc.address,
			"processor_id": req.ProcessorId,
			"last_version": req.LastVersion,
		}).Info("processor reconnecting with existing state, continuing incremental updates")
	}
}

// handleSyncAck handles a SyncAck from a processor
func (c *Client) handleSyncAck(pc *processorConnection, ack *SyncAck) {
	if ack.Success {
		clog.WithFields(log.Fields{
			"address":      pc.address,
			"processor_id": ack.ProcessorId,
			"version":      ack.Version,
		}).Debug("received ACK from processor")
	} else {
		clog.WithFields(log.Fields{
			"address":      pc.address,
			"processor_id": ack.ProcessorId,
			"version":      ack.Version,
			"error":        ack.Error,
		}).Error("received NACK from processor")
	}
}

// GetVersion returns the current cache version
func (c *Client) GetVersion() int64 {
	return c.version.Load()
}

// SetInformer sets the informer data source for obtaining snapshots
func (c *Client) SetInformer(informer InformerDataSource) {
	c.infMu.Lock()
	defer c.infMu.Unlock()
	c.informer = informer
	clog.Info("Informer data source set for snapshot generation")
}
