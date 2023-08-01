package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
)

type ServerTLS struct {
	Type       string
	CertPath   string `yaml:"certPath,omitempty" json:"certPath,omitempty" doc:"path to the server certificate"`
	KeyPath    string `yaml:"keyPath,omitempty" json:"keyPath,omitempty" doc:"path to the server private key"`
	CACertPath string `yaml:"caCertPath,omitempty" json:"caCertPath,omitempty" doc:"path to the CA certificate, required for mTLS"`
}

type TLSTypeEnum struct {
	None   string `yaml:"none" json:"none" doc:"No TLS"`
	Simple string `yaml:"simple" json:"simple" doc:"One-way TLS"`
	Mutual string `yaml:"mutual" json:"mutual" doc:"Mutual TLS"`
}

func TLSTypeName(operation string) string {
	return GetEnumName(TLSTypeEnum{}, operation)
}

func (c *ServerTLS) Build() (*tls.Config, error) {
	if c == nil || c.Type == "" || c.Type == TLSTypeName("None") {
		return nil, nil
	}
	if c.CertPath == "" {
		return nil, errors.New("serverCertPath must be provided for TLS")
	}
	if c.KeyPath == "" {
		return nil, errors.New("serverKeyPath must be provided for TLS")
	}

	userCert, err := os.ReadFile(c.CertPath)
	if err != nil {
		return nil, err
	}
	userKey, err := os.ReadFile(c.KeyPath)
	if err != nil {
		return nil, err
	}
	pair, err := tls.X509KeyPair(userCert, userKey)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		// TLS clients must use TLS 1.2 or higher
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{pair},
	}

	if c.Type == TLSTypeName("Mutual") {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		caCert, err := os.ReadFile(c.CACertPath)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(caCert)
	}

	return tlsConfig, nil
}
