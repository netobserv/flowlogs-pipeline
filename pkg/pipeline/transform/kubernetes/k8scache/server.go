package k8scache

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/datasource"
	log "github.com/sirupsen/logrus"
)

var slog = log.WithField("component", "k8scache.Server")

// KubernetesCacheServer implements the gRPC KubernetesCacheService server.
// It receives cache updates from the centralized informers client.
type KubernetesCacheServer struct {
	UnimplementedKubernetesCacheServiceServer
	datasource *datasource.Datasource
	version    atomic.Int64 // Last version received
}

// NewKubernetesCacheServer creates a new cache synchronization server.
func NewKubernetesCacheServer(ds *datasource.Datasource) *KubernetesCacheServer {
	server := &KubernetesCacheServer{
		datasource: ds,
	}
	server.version.Store(0)
	return server
}

// StreamUpdates implements the bidirectional streaming RPC
// The server:
//  1. Sends SyncRequest to ask for data
//  2. Receives CacheUpdate from client
//  3. Sends SyncAck to confirm receipt
//  4. Repeat steps 2-3
func (s *KubernetesCacheServer) StreamUpdates(stream KubernetesCacheService_StreamUpdatesServer) error {
	ctx := stream.Context()

	// Generate a unique ID for this connection (for logging)
	connectionID := fmt.Sprintf("flp-%d", time.Now().UnixNano())

	// Send SyncRequest with our current version so the client knows what to send
	// (full snapshot if 0, or incrementals from that version)
	lastVersion := s.version.Load()
	err := stream.Send(&SyncMessage{
		Message: &SyncMessage_Request{
			Request: &SyncRequest{
				ProcessorId: connectionID,
				LastVersion: lastVersion,
			},
		},
	})
	if err != nil {
		slog.WithError(err).Error("Failed to send initial SyncRequest")
		return err
	}

	slog.WithFields(log.Fields{
		"connection_id": connectionID,
		"last_version":  lastVersion,
	}).Info("Sent SyncRequest to client")

	// Receive updates from client
	for {
		select {
		case <-ctx.Done():
			slog.WithField("connection_id", connectionID).Info("Connection context cancelled")
			return ctx.Err()
		default:
			update, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				slog.WithField("connection_id", connectionID).Info("Client disconnected gracefully")
				return nil
			}
			if err != nil {
				slog.WithError(err).WithField("connection_id", connectionID).Warn("Error receiving from client")
				return err
			}

			// Process the update
			if err := s.processUpdate(update); err != nil {
				slog.WithError(err).WithField("version", update.Version).Error("Failed to process update")
				// Send NACK
				_ = stream.Send(&SyncMessage{
					Message: &SyncMessage_Ack{
						Ack: &SyncAck{
							ProcessorId: connectionID,
							Version:     update.Version,
							Success:     false,
							Error:       err.Error(),
						},
					},
				})
				continue
			}

			// Update was processed successfully
			s.version.Store(update.Version)

			// Send ACK
			err = stream.Send(&SyncMessage{
				Message: &SyncMessage_Ack{
					Ack: &SyncAck{
						ProcessorId: connectionID,
						Version:     update.Version,
						Success:     true,
					},
				},
			})
			if err != nil {
				slog.WithError(err).Error("Failed to send ACK")
				return err
			}

			slog.WithFields(log.Fields{
				"connection_id": connectionID,
				"version":       update.Version,
				"is_snapshot":   update.IsSnapshot,
				"num_entries":   len(update.Entries),
			}).Debug("Processed and acknowledged update")
		}
	}
}

// processUpdate applies a cache update to the local datasource (when KubernetesStore is set).
func (s *KubernetesCacheServer) processUpdate(update *CacheUpdate) error {
	entries := resourceEntriesToMeta(update.Entries)

	// Note: is_snapshot field is ignored - we only support incremental updates
	// If a processor restarts, it starts with empty cache and builds up from incoming ADD/UPDATE events

	switch update.Operation {
	case OperationType_OPERATION_ADD, OperationType_OPERATION_UPDATE:
		slog.WithField("num_entries", len(entries)).Debug("Received ADD/UPDATE")
		s.datasource.ApplyCacheAddOrUpdate(entries)
	case OperationType_OPERATION_DELETE:
		slog.WithField("num_entries", len(entries)).Debug("Received DELETE")
		s.datasource.ApplyCacheDelete(entries)
	case OperationType_OPERATION_UNSPECIFIED:
		slog.Warn("Received update with unspecified operation")
	default:
		slog.WithField("operation", update.Operation).Warn("Unknown operation type")
	}

	return nil
}

// GetCurrentVersion returns the last version received
func (s *KubernetesCacheServer) GetCurrentVersion() int64 {
	return s.version.Load()
}
