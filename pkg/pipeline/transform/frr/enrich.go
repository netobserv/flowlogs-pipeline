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
	"sync"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

var (
	storeMu sync.RWMutex
	store   Store
)

// InitStore starts the FRRConfiguration informer. Safe to call once; subsequent
// calls are no-ops while a store is already set.
func InitStore(kubeConfigPath string) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	if store != nil {
		return nil
	}
	s := NewInformerStore()
	if err := s.Start(kubeConfigPath); err != nil {
		return err
	}
	store = s
	return nil
}

// SetStore injects a Store implementation (for tests).
func SetStore(s Store) {
	storeMu.Lock()
	defer storeMu.Unlock()
	store = s
}

// ResetStore clears the package-level store (for tests).
func ResetStore() {
	storeMu.Lock()
	defer storeMu.Unlock()
	if is, ok := store.(*InformerStore); ok && is != nil {
		is.Stop()
	}
	store = nil
}

func getStore() Store {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return store
}

// Enrich looks up the IP in inputField via LPM against FRR advertised prefixes
// and writes the matching local ASN (stringified) to outputField.
func Enrich(outputEntry config.GenericMap, rule *api.NetworkAddASNLabelRule) {
	if rule == nil || rule.Input == "" || rule.Output == "" {
		log.Error("add_asn_label rule: missing input or output configuration")
		return
	}
	ip, ok := outputEntry.LookupString(rule.Input)
	if !ok || ip == "" {
		return
	}
	s := getStore()
	if s == nil {
		return
	}
	asn, ok := s.Lookup(ip)
	if !ok {
		return
	}
	outputEntry[rule.Output] = strconv.FormatUint(uint64(asn), 10)
}
