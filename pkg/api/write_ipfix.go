package api

import (
	"errors"
)

type WriteIpfix struct {
	TargetHost   string `yaml:"targetHost,omitempty" json:"targetHost,omitempty" doc:"IPFIX Collector host target IP"`
	TargetPort   int    `yaml:"targetPort,omitempty" json:"targetPort,omitempty" doc:"IPFIX Collector host target port"`
	Transport    string `yaml:"transport,omitempty" json:"transport,omitempty" doc:"Transport protocol (TCP/UDP) to be used for the IPFIX connection"`
	EnterpriseID int    `yaml:"enterpriseId,omitempty" json:"EnterpriseId,omitempty" doc:"Enterprise ID for exporting transformations"`
}

func (w *WriteIpfix) SetDefaults() {
	if w.Transport == "" {
		w.Transport = "TCP"
	}
}

func (wl *WriteIpfix) Validate() error {
	if wl == nil {
		return errors.New("you must provide a configuration")
	}
	if wl.TargetHost == "" {
		return errors.New("targethost can't be empty")
	}
	if wl.TargetPort == 0 {
		return errors.New("targetport can't be empty")
	}
	return nil
}
