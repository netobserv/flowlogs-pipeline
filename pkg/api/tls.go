package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

type TLSConfig struct {
	Type               string `yaml:"type,omitempty" json:"type,omitempty" enum:"TLSTypeEnum" doc:"type of TLS: none, simple or mutual."`
	CertPath           string `yaml:"certPath,omitempty" json:"certPath,omitempty" doc:"path to the certificate. On client-side, this is only used for Mutual TLS."`
	KeyPath            string `yaml:"keyPath,omitempty" json:"keyPath,omitempty" doc:"path to the private key. On client-side, this is only used for Mutual TLS."`
	CACertPath         string `yaml:"caCertPath,omitempty" json:"caCertPath,omitempty" doc:"path to the CA certificate. On server-side, this is only required for Mutual TLS."`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify,omitempty" json:"insecureSkipVerify,omitempty" doc:"skip verifying the peer certificate chain and host name"`
}

type TLSTypeEnum struct {
	None   string `yaml:"none" json:"none" doc:"No TLS"`
	Simple string `yaml:"simple" json:"simple" doc:"One-way TLS"`
	Mutual string `yaml:"mutual" json:"mutual" doc:"Mutual TLS"`
}

func TLSTypeName(operation string) string {
	return GetEnumName(TLSTypeEnum{}, operation)
}

func (c *TLSConfig) IsEnabled() bool {
	return c != nil && c.Type != "" && c.Type != TLSTypeName("None")
}

func (c *TLSConfig) AsClient() (*tls.Config, error) {
	if !c.IsEnabled() {
		return nil, nil
	}
	if c.CACertPath == "" {
		return nil, errors.New("caCertPath must be provided for client TLS")
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
	if err := c.setCA(tlsConfig); err != nil {
		return nil, err
	}

	if c.Type == TLSTypeName("Mutual") {
		if c.CertPath == "" {
			return nil, errors.New("certPath must be provided for client mTLS")
		}
		if c.KeyPath == "" {
			return nil, errors.New("keyPath must be provided for client mTLS")
		}
		if err := c.setCerts(tlsConfig); err != nil {
			return nil, err
		}
	}
	return tlsConfig, nil
}

func (c *TLSConfig) AsServer() (*tls.Config, error) {
	if !c.IsEnabled() {
		return nil, nil
	}
	if c.CertPath == "" {
		return nil, errors.New("certPath must be provided for server TLS")
	}
	if c.KeyPath == "" {
		return nil, errors.New("keyPath must be provided for server TLS")
	}

	tlsConfig := &tls.Config{
		// TLS clients must use TLS 1.2 or higher
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
	if err := c.setCerts(tlsConfig); err != nil {
		return nil, err
	}

	if c.Type == TLSTypeName("Mutual") {
		if c.CACertPath == "" {
			return nil, errors.New("caCertPath must be provided for server mTLS")
		}
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		if err := c.setCA(tlsConfig); err != nil {
			return nil, err
		}
	}

	return tlsConfig, nil
}

func (c *TLSConfig) setCA(cfg *tls.Config) error {
	caCert, err := os.ReadFile(c.CACertPath)
	if err != nil {
		return fmt.Errorf("could not read CA file: %w", err)
	}
	cfg.RootCAs = x509.NewCertPool()
	cfg.RootCAs.AppendCertsFromPEM(caCert)
	return nil
}

func (c *TLSConfig) setCerts(cfg *tls.Config) error {
	cert, err := os.ReadFile(c.CertPath)
	if err != nil {
		return fmt.Errorf("could not read cert file: %w", err)
	}
	key, err := os.ReadFile(c.KeyPath)
	if err != nil {
		return fmt.Errorf("could not read key file: %w", err)
	}
	pair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return fmt.Errorf("could not create X509KeyPair: %w", err)
	}
	cfg.Certificates = []tls.Certificate{pair}
	return nil
}
