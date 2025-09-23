package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/netobserv/loki-client-go/pkg/backoff"
	"github.com/netobserv/loki-client-go/pkg/labelutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Default configuration values for GRPC client
const (
	DefaultBatchWait            = 1 * time.Second
	DefaultBatchSize        int = 1024 * 1024
	DefaultMinBackoff           = 500 * time.Millisecond
	DefaultMaxBackoff           = 5 * time.Minute
	DefaultMaxRetries       int = 10
	DefaultTimeout              = 10 * time.Second
	DefaultMaxRecvMsgSize       = 1024 * 1024 * 64 // 64MB
	DefaultMaxSendMsgSize       = 1024 * 1024 * 16 // 16MB
	DefaultKeepAlive            = 30 * time.Second
	DefaultKeepAliveTimeout     = 5 * time.Second
)

// Config describes configuration for a GRPC pusher client.
type Config struct {
	// Server address (host:port)
	ServerAddress string `yaml:"server_address"`

	// Batching configuration
	BatchWait time.Duration `yaml:"batch_wait"`
	BatchSize int           `yaml:"batch_size"`

	// Connection configuration
	MaxRecvMsgSize int           `yaml:"max_recv_msg_size"`
	MaxSendMsgSize int           `yaml:"max_send_msg_size"`
	Timeout        time.Duration `yaml:"timeout"`

	// TLS configuration
	TLS TLSConfig `yaml:"tls"`

	// Keep alive configuration
	KeepAlive        time.Duration `yaml:"keep_alive"`
	KeepAliveTimeout time.Duration `yaml:"keep_alive_timeout"`

	// Retry configuration
	BackoffConfig backoff.BackoffConfig `yaml:"backoff_config"`

	// Labels to add to any time series when communicating with loki
	ExternalLabels labelutil.LabelSet `yaml:"external_labels,omitempty"`

	// Tenant ID for multi-tenant mode (empty string means single tenant)
	TenantID string `yaml:"tenant_id"`
}

// TLSConfig contains TLS configuration for GRPC client
type TLSConfig struct {
	// Enable TLS
	Enabled bool `yaml:"enabled"`

	// Path to certificate file
	CertFile string `yaml:"cert_file"`

	// Path to key file
	KeyFile string `yaml:"key_file"`

	// Path to CA file
	CAFile string `yaml:"ca_file"`

	// Server name for certificate verification
	ServerName string `yaml:"server_name"`

	// Skip certificate verification (insecure)
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

// NewDefaultConfig creates a default configuration for a given GRPC server address.
func NewDefaultConfig(serverAddress string) (Config, error) {
	var cfg Config
	f := &flag.FlagSet{}
	cfg.RegisterFlags(f)
	if err := f.Parse(nil); err != nil {
		return cfg, err
	}
	cfg.ServerAddress = serverAddress
	return cfg, nil
}

// RegisterFlags registers configuration flags
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.RegisterFlagsWithPrefix("", f)
}

// RegisterFlagsWithPrefix registers flags with a given prefix
func (c *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.StringVar(&c.ServerAddress, prefix+"grpc.server-address", "", "GRPC server address (host:port)")
	f.DurationVar(&c.BatchWait, prefix+"grpc.batch-wait", DefaultBatchWait, "Maximum wait period before sending batch.")
	f.IntVar(&c.BatchSize, prefix+"grpc.batch-size-bytes", DefaultBatchSize, "Maximum batch size to accrue before sending.")

	f.IntVar(&c.MaxRecvMsgSize, prefix+"grpc.max-recv-msg-size", DefaultMaxRecvMsgSize, "Maximum message size the client can receive.")
	f.IntVar(&c.MaxSendMsgSize, prefix+"grpc.max-send-msg-size", DefaultMaxSendMsgSize, "Maximum message size the client can send.")
	f.DurationVar(&c.Timeout, prefix+"grpc.timeout", DefaultTimeout, "Maximum time to wait for server to respond to a request")

	f.BoolVar(&c.TLS.Enabled, prefix+"grpc.tls.enabled", false, "Enable TLS")
	f.StringVar(&c.TLS.CertFile, prefix+"grpc.tls.cert-file", "", "Path to client certificate file")
	f.StringVar(&c.TLS.KeyFile, prefix+"grpc.tls.key-file", "", "Path to client key file")
	f.StringVar(&c.TLS.CAFile, prefix+"grpc.tls.ca-file", "", "Path to CA certificate file")
	f.StringVar(&c.TLS.ServerName, prefix+"grpc.tls.server-name", "", "Server name for certificate verification")
	f.BoolVar(&c.TLS.InsecureSkipVerify, prefix+"grpc.tls.insecure-skip-verify", false, "Skip certificate verification")

	f.DurationVar(&c.KeepAlive, prefix+"grpc.keep-alive", DefaultKeepAlive, "Keep alive interval")
	f.DurationVar(&c.KeepAliveTimeout, prefix+"grpc.keep-alive-timeout", DefaultKeepAliveTimeout, "Keep alive timeout")

	f.IntVar(&c.BackoffConfig.MaxRetries, prefix+"grpc.max-retries", DefaultMaxRetries, "Maximum number of retries when sending batches.")
	f.DurationVar(&c.BackoffConfig.MinBackoff, prefix+"grpc.min-backoff", DefaultMinBackoff, "Initial backoff time between retries.")
	f.DurationVar(&c.BackoffConfig.MaxBackoff, prefix+"grpc.max-backoff", DefaultMaxBackoff, "Maximum backoff time between retries.")

	f.Var(&c.ExternalLabels, prefix+"grpc.external-labels", "list of external labels to add to each log (e.g: --grpc.external-labels=lb1=v1,lb2=v2)")
	f.StringVar(&c.TenantID, prefix+"grpc.tenant-id", "", "Tenant ID to use when pushing logs to Loki.")
}

// BuildDialOptions creates GRPC dial options from the configuration
func (c *Config) BuildDialOptions() ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	// Message size limits
	opts = append(opts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(c.MaxSendMsgSize),
		),
	)

	// Keep alive settings
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                c.KeepAlive,
		Timeout:             c.KeepAliveTimeout,
		PermitWithoutStream: true,
	}))

	// TLS configuration
	if c.TLS.Enabled {
		tlsConfig := &tls.Config{
			ServerName:         c.TLS.ServerName,
			InsecureSkipVerify: c.TLS.InsecureSkipVerify,
		}

		// Load CA certificate if specified
		if c.TLS.CAFile != "" {
			caCert, err := os.ReadFile(c.TLS.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA certificate file %s: %w", c.TLS.CAFile, err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("failed to parse CA certificate from %s", c.TLS.CAFile)
			}
			tlsConfig.RootCAs = caCertPool
		}

		if c.TLS.CertFile != "" && c.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile)
			if err != nil {
				return nil, err
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts, nil
}

// UnmarshalYAML implements YAML unmarshaler
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type raw Config
	var cfg raw

	if c.ServerAddress != "" {
		// Use existing values as defaults
		cfg = raw(*c)
	} else {
		// Set sane defaults
		cfg = raw{
			BackoffConfig: backoff.BackoffConfig{
				MaxBackoff: DefaultMaxBackoff,
				MaxRetries: DefaultMaxRetries,
				MinBackoff: DefaultMinBackoff,
			},
			BatchSize:        DefaultBatchSize,
			BatchWait:        DefaultBatchWait,
			Timeout:          DefaultTimeout,
			MaxRecvMsgSize:   DefaultMaxRecvMsgSize,
			MaxSendMsgSize:   DefaultMaxSendMsgSize,
			KeepAlive:        DefaultKeepAlive,
			KeepAliveTimeout: DefaultKeepAliveTimeout,
		}
	}

	if err := unmarshal(&cfg); err != nil {
		return err
	}

	*c = Config(cfg)
	return nil
}
