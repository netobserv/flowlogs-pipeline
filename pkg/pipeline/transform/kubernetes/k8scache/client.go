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
	// Send timeout
	sendTimeout = 10 * time.Second
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
	// InsecureSkipVerify skips TLS certificate verification (not recommended for production)
	InsecureSkipVerify bool
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
	return &Client{
		processorID: config.ProcessorID,
		tlsConfig:   *config,
		connections: make(map[string]*processorConnection),
		updateChan:  make(chan *CacheUpdate, 100), // Buffer for updates
		ctx:         ctx,
		cancel:      cancel,
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
	// First check: quick lock to see if already connected
	c.mu.Lock()
	if _, exists := c.connections[address]; exists {
		c.mu.Unlock()
		clog.WithField("address", address).Debug("processor already connected")
		return nil
	}
	c.mu.Unlock()

	clog.WithField("address", address).Info("connecting to FLP processor")

	// Perform slow network operations without holding the lock
	// Get transport credentials
	creds, err := c.getTransportCredentials()
	if err != nil {
		return fmt.Errorf("failed to create transport credentials: %w", err)
	}

	// Create connection
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)), // 50MB max message size
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Create stream
	client := NewKubernetesCacheServiceClient(conn)
	ctx, cancel := context.WithCancel(c.ctx)
	stream, err := client.StreamUpdates(ctx)
	if err != nil {
		cancel()
		conn.Close()
		return fmt.Errorf("failed to create stream to %s: %w", address, err)
	}

	// Second check: re-acquire lock and verify no other goroutine added this connection
	c.mu.Lock()
	if _, exists := c.connections[address]; exists {
		c.mu.Unlock()
		// Another goroutine added this connection while we were connecting
		// Clean up our newly-created resources
		cancel()
		conn.Close()
		clog.WithField("address", address).Debug("processor was connected by another goroutine, discarding duplicate")
		return nil
	}

	// We won the race, store the connection
	pc := &processorConnection{
		address: address,
		conn:    conn,
		stream:  stream,
		cancel:  cancel,
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
	defer c.mu.Unlock()

	pc, exists := c.connections[address]
	if !exists {
		return
	}

	clog.WithField("address", address).Info("disconnecting from FLP processor")
	delete(c.connections, address)

	// Close connection after releasing client lock to avoid holding both locks
	c.mu.Unlock()
	pc.closeConnection()
	c.mu.Lock()
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
func (c *Client) SendAdd(entries []*model.ResourceMetaData) error {
	version := c.version.Add(1)
	update := &CacheUpdate{
		Version:    version,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_ADD,
		Entries:    metaToResourceEntries(entries),
	}

	return c.sendUpdate(update)
}

// SendUpdate sends an UPDATE operation to all connected processors
func (c *Client) SendUpdate(entries []*model.ResourceMetaData) error {
	version := c.version.Add(1)
	update := &CacheUpdate{
		Version:    version,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_UPDATE,
		Entries:    metaToResourceEntries(entries),
	}

	return c.sendUpdate(update)
}

// SendDelete sends a DELETE operation to all connected processors
func (c *Client) SendDelete(entries []*model.ResourceMetaData) error {
	version := c.version.Add(1)
	update := &CacheUpdate{
		Version:    version,
		IsSnapshot: false,
		Operation:  OperationType_OPERATION_DELETE,
		Entries:    metaToResourceEntries(entries),
	}

	return c.sendUpdate(update)
}

// sendUpdate sends an update to the update channel (non-blocking with timeout)
func (c *Client) sendUpdate(update *CacheUpdate) error {
	select {
	case c.updateChan <- update:
		return nil
	case <-time.After(5 * time.Second):
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

			// Use a channel to signal completion or timeout
			done := make(chan error, 1)
			go func() {
				done <- stream.Send(update)
			}()

			select {
			case err := <-done:
				if err != nil {
					clog.WithError(err).WithField("address", pc.address).Error("failed to send update")
					pc.healthy.Store(false)
				}
			case <-time.After(sendTimeout):
				clog.WithField("address", pc.address).Error("send operation timed out")
				pc.healthy.Store(false)

				// Tear down the stream to unblock the blocked Send() and trigger reconnection
				// First try to close the send side gracefully
				if err := stream.CloseSend(); err != nil {
					clog.WithError(err).WithField("address", pc.address).Warn("failed to close send side of stream")
				}

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
	pc.mu.Lock()
	defer pc.mu.Unlock()

	backoff := initialBackoff
	for attempt := 1; attempt <= maxReconnectAttempts; attempt++ {
		pc.reconnectAttempts = attempt

		clog.WithFields(log.Fields{
			"address": pc.address,
			"attempt": attempt,
			"backoff": backoff,
		}).Info("attempting to reconnect")

		// Wait before retry (with context cancellation support)
		select {
		case <-time.After(backoff):
		case <-c.ctx.Done():
			clog.WithField("address", pc.address).Info("reconnection cancelled")
			return false
		}

		// Close old connection
		if pc.conn != nil {
			pc.conn.Close()
		}
		if pc.cancel != nil {
			pc.cancel()
		}

		// Get transport credentials
		creds, err := c.getTransportCredentials()
		if err != nil {
			clog.WithError(err).WithField("address", pc.address).Warn("reconnect: failed to create transport credentials")
			backoff = min(time.Duration(float64(backoff)*backoffMultiplier), maxBackoff)
			continue
		}

		// Create new connection
		conn, err := grpc.NewClient(pc.address,
			grpc.WithTransportCredentials(creds),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(50*1024*1024)),
		)
		if err != nil {
			clog.WithError(err).WithField("address", pc.address).Warn("reconnect: failed to connect")
			backoff = min(time.Duration(float64(backoff)*backoffMultiplier), maxBackoff)
			continue
		}

		// Create new stream
		client := NewKubernetesCacheServiceClient(conn)
		ctx, cancel := context.WithCancel(c.ctx)
		stream, err := client.StreamUpdates(ctx)
		if err != nil {
			clog.WithError(err).WithField("address", pc.address).Warn("reconnect: failed to create stream")
			cancel()
			conn.Close()
			backoff = min(time.Duration(float64(backoff)*backoffMultiplier), maxBackoff)
			continue
		}

		// Update connection
		pc.conn = conn
		pc.stream = stream
		pc.cancel = cancel
		pc.healthy.Store(true)
		pc.reconnectAttempts = 0

		clog.WithFields(log.Fields{
			"address": pc.address,
			"attempt": attempt,
		}).Info("reconnection successful")

		return true
	}

	clog.WithFields(log.Fields{
		"address":  pc.address,
		"attempts": maxReconnectAttempts,
	}).Error("reconnection failed after max attempts, removing processor")

	// Remove from connections map after exhausting retries
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

	// Note: We only send incremental updates (ADD/UPDATE/DELETE).
	// No initial snapshot is sent. Processors start with empty cache and build up
	// from incoming events. If a processor restarts, it loses enrichment until
	// resources are updated again (acceptable trade-off per design).
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
