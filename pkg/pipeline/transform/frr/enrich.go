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
	"strconv"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

// InitStore starts the FRRConfiguration informer and returns the resulting Store.
func InitStore(kubeConfigPath string) (Store, error) {
	s := NewInformerStore()
	if err := s.Start(kubeConfigPath); err != nil {
		return nil, err
	}
	return s, nil
}

// Enrich looks up the IP in inputField via LPM against FRR advertised prefixes
// and writes the matching local ASN (stringified) to outputField. It no-ops when
// store is nil (e.g. FRR enrichment is disabled).
func Enrich(store Store, outputEntry config.GenericMap, rule *api.NetworkAddASNLabelRule) {
	if store == nil {
		return
	}
	if rule == nil || rule.Input == "" || rule.Output == "" {
		log.Error("add_asn_label rule: missing input or output configuration")
		return
	}
	ip, ok := outputEntry.LookupString(rule.Input)
	if !ok || ip == "" {
		return
	}
	asn, ok := store.Lookup(ip)
	if !ok {
		return
	}
	outputEntry[rule.Output] = strconv.FormatUint(uint64(asn), 10)
}
