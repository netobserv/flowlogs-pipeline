/*
 * Copyright (C) 2026 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package frr

import (
	"net"
	"sort"
)

// prefixEntry maps a CIDR to an ASN. Entries are matched with longest-prefix match.
type prefixEntry struct {
	network *net.IPNet
	bits    int
	asn     uint32
}

// ASNTable is an immutable LPM table of CIDR → ASN. Safe for concurrent reads.
type ASNTable struct {
	entries []prefixEntry
}

// BuildASNTable builds a longest-prefix-match table from CIDR → ASN mappings.
// Invalid CIDRs are skipped. ASN 0 entries are skipped (reserved / invalid for labeling).
// On duplicate equal-length prefixes, the last write wins.
func BuildASNTable(cidrs map[string]uint32) *ASNTable {
	byCIDR := make(map[string]prefixEntry, len(cidrs))
	for cidr, asn := range cidrs {
		if asn == 0 {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil || ipNet == nil {
			continue
		}
		ones, _ := ipNet.Mask.Size()
		byCIDR[ipNet.String()] = prefixEntry{
			network: ipNet,
			bits:    ones,
			asn:     asn,
		}
	}

	entries := make([]prefixEntry, 0, len(byCIDR))
	for _, e := range byCIDR {
		entries = append(entries, e)
	}
	// Longest prefix first; stable order for equal lengths by CIDR string.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].bits != entries[j].bits {
			return entries[i].bits > entries[j].bits
		}
		return entries[i].network.String() < entries[j].network.String()
	})
	return &ASNTable{entries: entries}
}

// Lookup returns the ASN for the longest matching prefix, or (0, false) if none.
func (t *ASNTable) Lookup(ip net.IP) (uint32, bool) {
	if t == nil || ip == nil {
		return 0, false
	}
	for _, e := range t.entries {
		if e.network.Contains(ip) {
			return e.asn, true
		}
	}
	return 0, false
}

// LookupString parses ip and looks up its ASN.
func (t *ASNTable) LookupString(ipStr string) (uint32, bool) {
	return t.Lookup(net.ParseIP(ipStr))
}

// Len returns the number of prefixes in the table.
func (t *ASNTable) Len() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}
