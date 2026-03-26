package k8scache

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var clog = log.WithField("component", "k8scache.Client")

// Client manages gRPC connections to FLP processor servers and pushes cache updates.
type Client struct {
	// processorID identifies this informer instance
	processorID string
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
}

// NewClient creates a new cache client (informer side)
func NewClient(processorID string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		processorID: processorID,
		connections: make(map[string]*processorConnection),
		updateChan:  make(chan *CacheUpdate, 100), // Buffer for updates
		ctx:         ctx,
		cancel:      cancel,
	}
}

// AddProcessor connects to a new FLP processor server
func (c *Client) AddProcessor(address string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.connections[address]; exists {
		clog.WithField("address", address).Debug("processor already connected")
		return nil
	}

	clog.WithField("address", address).Info("connecting to FLP processor")

	// Create connection
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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

	pc := &processorConnection{
		address: address,
		conn:    conn,
		stream:  stream,
		cancel:  cancel,
	}
	pc.healthy.Store(true)

	c.connections[address] = pc

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
	pc.cancel()
	pc.conn.Close()
	delete(c.connections, address)
}

// Start begins processing cache updates and sending them to all connected processors
func (c *Client) Start() {
	go c.processorLoop()
}

// Stop shuts down the client
func (c *Client) Stop() {
	c.cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, pc := range c.connections {
		pc.cancel()
		pc.conn.Close()
	}
	c.connections = make(map[string]*processorConnection)
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
		"version":      update.Version,
		"is_snapshot":  update.IsSnapshot,
		"operation":    update.Operation,
		"num_entries":  len(update.Entries),
		"num_targets":  len(connections),
	}).Debug("broadcasting cache update")

	// Send to all processors concurrently
	var wg sync.WaitGroup
	for _, pc := range connections {
		wg.Add(1)
		go func(pc *processorConnection) {
			defer wg.Done()
			if err := pc.stream.Send(update); err != nil {
				clog.WithError(err).WithField("address", pc.address).Error("failed to send update")
				pc.healthy.Store(false)
			}
		}(pc)
	}
	wg.Wait()
}

// receiveFromProcessor handles incoming messages from a processor (SyncRequest/SyncAck)
func (c *Client) receiveFromProcessor(pc *processorConnection) {
	defer func() {
		pc.healthy.Store(false)
		clog.WithField("address", pc.address).Info("receiver goroutine stopped")
	}()

	for {
		msg, err := pc.stream.Recv()
		if err != nil {
			if err == io.EOF {
				clog.WithField("address", pc.address).Info("processor disconnected (EOF)")
			} else {
				clog.WithError(err).WithField("address", pc.address).Warn("error receiving from processor")
			}
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
