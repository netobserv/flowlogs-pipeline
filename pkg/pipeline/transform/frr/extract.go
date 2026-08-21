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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// extractASNMappings parses an FRRConfiguration into CIDR → local ASN mappings.
//
// Source of truth for local ASN labeling:
//   - routers[].asn with routers[].prefixes
//   - routers[].asn with neighbors[].toAdvertise.allowed.prefixes
//
// Intentionally ignored (not ownership signals):
//   - neighbors[].toReceive (accept allowlist, not peer-owned prefixes)
//   - neighbors[].asn (peer ASN without reliable prefix ownership)
func extractASNMappings(obj *unstructured.Unstructured) (map[string]uint32, error) {
	if obj == nil {
		return nil, fmt.Errorf("nil FRRConfiguration")
	}

	out := make(map[string]uint32)
	routers, found, err := unstructured.NestedSlice(obj.Object, "spec", "bgp", "routers")
	if err != nil {
		return nil, fmt.Errorf("reading spec.bgp.routers: %w", err)
	}
	if !found {
		return out, nil
	}

	for _, router := range routers {
		routerMap, ok := router.(map[string]interface{})
		if !ok {
			continue
		}
		asn, err := nestedUint32(routerMap, "asn")
		if err != nil || asn == 0 {
			continue
		}

		for _, prefix := range nestedStringSlice(routerMap, "prefixes") {
			out[prefix] = asn
		}

		neighbors, _, err := unstructured.NestedSlice(routerMap, "neighbors")
		if err != nil {
			return nil, fmt.Errorf("reading neighbors: %w", err)
		}
		for _, neighbor := range neighbors {
			neighborMap, ok := neighbor.(map[string]interface{})
			if !ok {
				continue
			}
			for _, prefix := range extractAdvertisePrefixes(neighborMap) {
				out[prefix] = asn
			}
		}
	}
	return out, nil
}

func extractAdvertisePrefixes(neighbor map[string]interface{}) []string {
	prefixes, found, err := unstructured.NestedSlice(neighbor, "toAdvertise", "allowed", "prefixes")
	if err != nil || !found {
		return nil
	}
	return coercePrefixList(prefixes)
}

func coercePrefixList(prefixes []interface{}) []string {
	var result []string
	for _, p := range prefixes {
		switch v := p.(type) {
		case string:
			if v != "" {
				result = append(result, v)
			}
		case map[string]interface{}:
			if prefix, ok, _ := unstructured.NestedString(v, "prefix"); ok && prefix != "" {
				result = append(result, prefix)
			}
		}
	}
	return result
}

func nestedStringSlice(obj map[string]interface{}, field string) []string {
	raw, found, err := unstructured.NestedSlice(obj, field)
	if err != nil || !found {
		return nil
	}
	var out []string
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func nestedUint32(obj map[string]interface{}, field string) (uint32, error) {
	v, found, err := unstructured.NestedFieldNoCopy(obj, field)
	if err != nil {
		return 0, err
	}
	if !found || v == nil {
		return 0, fmt.Errorf("field %q not found", field)
	}
	switch n := v.(type) {
	case int64:
		return uint32(n), nil
	case int32:
		return uint32(n), nil
	case int:
		return uint32(n), nil
	case float64:
		return uint32(n), nil
	default:
		return 0, fmt.Errorf("field %q has unsupported type %T", field, v)
	}
}
