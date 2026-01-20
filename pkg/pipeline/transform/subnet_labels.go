package transform

import (
	"fmt"
	"net/netip"

	"github.com/gaissmai/bart"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

type subnetLabels struct {
	table        *bart.Fast[string]
	privateLabel string
}

func newSubnetLabels(cfg []api.NetworkTransformSubnetLabel) (subnetLabels, error) {
	var private string
	snTable := new(bart.Fast[string])
	for _, category := range cfg {
		for _, cidr := range category.CIDRs {
			parsed, err := netip.ParsePrefix(cidr)
			if err != nil {
				return subnetLabels{}, fmt.Errorf("category %s: fail to parse CIDR, %w", category.Name, err)
			}
			snTable.Insert(parsed, category.Name)
		}
		if category.CheckPrivate {
			private = category.Name
		}
	}
	return subnetLabels{table: snTable, privateLabel: private}, nil
}

func (s *subnetLabels) apply(mutableFlow config.GenericMap, rule *api.NetworkAddSubnetLabelRule) {
	if anyIP, ok := mutableFlow[rule.Input]; ok {
		var ip netip.Addr
		if ip, ok = anyIP.(netip.Addr); !ok {
			if strIP, ok := anyIP.(string); ok {
				ip, _ = netip.ParseAddr(strIP)
			}
		}
		if ip.IsValid() {
			if lbl, ok := s.table.Lookup(ip); ok {
				mutableFlow[rule.Output] = lbl
			} else if ip.IsPrivate() && s.privateLabel != "" {
				mutableFlow[rule.Output] = s.privateLabel
			}
		}
	}
}
