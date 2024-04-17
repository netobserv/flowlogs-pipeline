// Package api provides the version indedependent interfaces and types that form the basis of the Infinispan client.
package api

import (
	"bytes"
	"fmt"

	"github.com/infinispan/infinispan-operator/pkg/mime"
)

// Infinispan is the entry point for the client containing all sub-interfaces for interacting with Infinispan
type Infinispan interface {
	Cache(name string) Cache
	Caches() Caches
	Container() Container
	Logging() Logging
	Metrics() Metrics
	ProtobufMetadataCacheName() string
	ScriptCacheName() string
	Server() Server
}

// Container interface contains all operations and sub-interfaces related to interactions with the Infinispan cache-container
type Container interface {
	Info() (*ContainerInfo, error)
	Backups() Backups
	HealthStatus() (HealthStatus, error)
	Members() ([]string, error)
	RebalanceDisable() error
	RebalanceEnable() error
	Restores() Restores
	Shutdown() error
	Xsite() Xsite
}

// Backups contains all operations required for container Backup operations
type Backups interface {
	Create(name string, config *BackupConfig) error
	Status(name string) (Status, error)
}

// Restores contains all operations required for container Restore operations
type Restores interface {
	Create(name string, config *RestoreConfig) error
	Status(name string) (Status, error)
}

// Cache contains all operations and sub-interfaces for manipulating a specific cache
type Cache interface {
	Config(contentType mime.MimeType) (string, error)
	Create(config string, contentType mime.MimeType, flags ...string) error
	CreateWithTemplate(templateName string) error
	Delete() error
	Exists() (bool, error)
	Get(key string) (string, bool, error)
	Put(key, value string, contentType mime.MimeType) error
	RollingUpgrade() RollingUpgrade
	Size() (int, error)
	UpdateConfig(config string, contentType mime.MimeType) error
}

// RollingUpgrade contains all operations for coordinating rolling upgrades on a specific cache
type RollingUpgrade interface {
	AddSource(config string, contentType mime.MimeType) error
	DisconnectSource() error
	SourceConnected() (bool, error)
	SyncData() (string, error)
}

// Caches contains all generic cache operations that aren't specific to a single cache
type Caches interface {
	ConvertConfiguration(config string, contentType, reqType mime.MimeType) (string, error)
	EqualConfiguration(a, b string) (bool, error)
	Names() ([]string, error)
}

// Cluster contains all operations that are performed cluster-wide
type Cluster interface {
	GracefulShutdown() error
	GracefulShutdownTask() error
}

// Logging contains all operatirons related to logging
type Logging interface {
	GetLoggers() (map[string]string, error)
	SetLogger(name, level string) error
}

// Metrics contains all operations related to the /metrics endpoint
type Metrics interface {
	Get(postfix string) (buf *bytes.Buffer, err error)
}

// Server contains all operations related to the server process
type Server interface {
	Stop() error
}

// Xsite contains all Xsite replated operations
type Xsite interface {
	PushAllState() error
}

// HealthStatus indicated the possible statuses of the Infinispan server
type HealthStatus string

const (
	HealthStatusDegraded          HealthStatus = "DEGRADED"
	HealthStatusHealth            HealthStatus = "HEALTHY"
	HealthStatusHealthRebalancing HealthStatus = "HEALTHY_REBALANCING"
	HealthStatusFailed            HealthStatus = "FAILED"
)

type Status string

const (
	// StatusNotFound means that the operation could not be found.
	StatusNotFound Status = "NotFound"
	// StatusSucceeded means that the operation has completed.
	StatusSucceeded Status = "Succeeded"
	// StatusRunning means that the operation is in progress.
	StatusRunning Status = "Running"
	// StatusFailed means that the operation failed.
	StatusFailed Status = "Failed"
	// StatusUnknown means that the state of the operation could not be obtained, typically due to an error in communicating with the infinispan server.
	StatusUnknown Status = "Unknown"
)

type BackupConfig struct {
	Directory string `json:"directory" validate:"required"`
	// +optional
	Resources BackupRestoreResources `json:"resources,omitempty"`
}

type RestoreConfig struct {
	Location string `json:"location" validate:"required"`
	// +optional
	Resources BackupRestoreResources `json:"resources,omitempty"`
}

type BackupRestoreResources struct {
	// +optional
	Caches []string `json:"caches,omitempty"`
	// +optional
	Templates []string `json:"templates,omitempty"`
	// +optional
	Counters []string `json:"counters,omitempty"`
	// +optional
	ProtoSchemas []string `json:"proto-schemas,omitempty"`
	// +optional
	Tasks []string `json:"tasks,omitempty"`
}

type ContainerInfo struct {
	Coordinator bool           `json:"coordinator"`
	SitesView   *[]interface{} `json:"sites_view,omitempty"`
}

type NotSupportedError struct {
	Version string
}

func (n *NotSupportedError) Error() string {
	return fmt.Sprintf("Operation not supported with Operand Major Version'%s'", n.Version)
}
