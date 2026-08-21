/*
 * Copyright (C) 2026 NetObserv Authors.
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
 *
 */

// Package schema defines the shared NetObserv flow record schema used by
// flowBuffer (in-memory) and the Parquet S3 encoder.
package schema

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"
)

const (
	// ParquetVersion is stored as key/value metadata on Parquet objects.
	ParquetVersion = "1"
	// ParquetVersionKey is the metadata key for the schema version.
	ParquetVersionKey = "netobserv.parquet.version"
)

// FlowRecordV1 is the Parquet / buffer schema v1.
// Nullable feature fields use pointers so missing values stay null in Parquet.
type FlowRecordV1 struct {
	TimeFlowStartMs int64  `parquet:"TimeFlowStartMs"`
	TimeFlowEndMs   int64  `parquet:"TimeFlowEndMs"`
	TimeReceived    *int64 `parquet:"TimeReceived,optional"`

	SrcAddr string `parquet:"SrcAddr,optional"`
	DstAddr string `parquet:"DstAddr,optional"`
	SrcPort *int32 `parquet:"SrcPort,optional"`
	DstPort *int32 `parquet:"DstPort,optional"`
	Proto   *int32 `parquet:"Proto,optional"`
	Etype   *int32 `parquet:"Etype,optional"`
	Dscp    *int32 `parquet:"Dscp,optional"`
	Flags   *int32 `parquet:"Flags,optional"`

	Bytes    *int64 `parquet:"Bytes,optional"`
	Packets  *int64 `parquet:"Packets,optional"`
	Sampling *int64 `parquet:"Sampling,optional"`

	SrcK8S_Name        string `parquet:"SrcK8S_Name,optional"`
	SrcK8S_Namespace   string `parquet:"SrcK8S_Namespace,optional"`
	SrcK8S_Type        string `parquet:"SrcK8S_Type,optional"`
	SrcK8S_OwnerName   string `parquet:"SrcK8S_OwnerName,optional"`
	SrcK8S_OwnerType   string `parquet:"SrcK8S_OwnerType,optional"`
	SrcK8S_HostIP      string `parquet:"SrcK8S_HostIP,optional"`
	SrcK8S_HostName    string `parquet:"SrcK8S_HostName,optional"`
	SrcK8S_Zone        string `parquet:"SrcK8S_Zone,optional"`
	SrcK8S_NetworkName string `parquet:"SrcK8S_NetworkName,optional"`

	DstK8S_Name        string `parquet:"DstK8S_Name,optional"`
	DstK8S_Namespace   string `parquet:"DstK8S_Namespace,optional"`
	DstK8S_Type        string `parquet:"DstK8S_Type,optional"`
	DstK8S_OwnerName   string `parquet:"DstK8S_OwnerName,optional"`
	DstK8S_OwnerType   string `parquet:"DstK8S_OwnerType,optional"`
	DstK8S_HostIP      string `parquet:"DstK8S_HostIP,optional"`
	DstK8S_HostName    string `parquet:"DstK8S_HostName,optional"`
	DstK8S_Zone        string `parquet:"DstK8S_Zone,optional"`
	DstK8S_NetworkName string `parquet:"DstK8S_NetworkName,optional"`

	// Feature fields (nullable)
	TimeFlowRttNs          *int64 `parquet:"TimeFlowRttNs,optional"`
	DnsId                  *int32 `parquet:"DnsId,optional"`
	DnsLatencyMs           *int64 `parquet:"DnsLatencyMs,optional"`
	DnsFlagsResponseCode   string `parquet:"DnsFlagsResponseCode,optional"`
	PktDropBytes           *int64 `parquet:"PktDropBytes,optional"`
	PktDropPackets         *int64 `parquet:"PktDropPackets,optional"`
	PktDropLatestState     string `parquet:"PktDropLatestState,optional"`
	PktDropLatestDropCause string `parquet:"PktDropLatestDropCause,optional"`
	FlowDirection          *int32 `parquet:"FlowDirection,optional"`
	AgentIP                string `parquet:"AgentIP,optional"`
}

// FlowTimestampMs returns the best available flow timestamp in milliseconds.
// Prefers TimeFlowEndMs, then TimeFlowStartMs, then TimeReceived (seconds→ms).
func FlowTimestampMs(m config.GenericMap) (int64, bool) {
	if v, ok := int64Field(m, "TimeFlowEndMs"); ok && v > 0 {
		return v, true
	}
	if v, ok := int64Field(m, "TimeFlowStartMs"); ok && v > 0 {
		return v, true
	}
	if v, ok := int64Field(m, "TimeReceived"); ok && v > 0 {
		// TimeReceived is typically unix seconds
		if v < 1e12 {
			return v * 1000, true
		}
		return v, true
	}
	return 0, false
}

// FromGenericMap maps an enriched GenericMap onto schema v1.
func FromGenericMap(m config.GenericMap) FlowRecordV1 {
	rec := FlowRecordV1{
		SrcAddr:                stringField(m, "SrcAddr"),
		DstAddr:                stringField(m, "DstAddr"),
		SrcK8S_Name:            stringField(m, "SrcK8S_Name"),
		SrcK8S_Namespace:       stringField(m, "SrcK8S_Namespace"),
		SrcK8S_Type:            stringField(m, "SrcK8S_Type"),
		SrcK8S_OwnerName:       stringField(m, "SrcK8S_OwnerName"),
		SrcK8S_OwnerType:       stringField(m, "SrcK8S_OwnerType"),
		SrcK8S_HostIP:          stringField(m, "SrcK8S_HostIP"),
		SrcK8S_HostName:        stringField(m, "SrcK8S_HostName"),
		SrcK8S_Zone:            stringField(m, "SrcK8S_Zone"),
		SrcK8S_NetworkName:     stringField(m, "SrcK8S_NetworkName"),
		DstK8S_Name:            stringField(m, "DstK8S_Name"),
		DstK8S_Namespace:       stringField(m, "DstK8S_Namespace"),
		DstK8S_Type:            stringField(m, "DstK8S_Type"),
		DstK8S_OwnerName:       stringField(m, "DstK8S_OwnerName"),
		DstK8S_OwnerType:       stringField(m, "DstK8S_OwnerType"),
		DstK8S_HostIP:          stringField(m, "DstK8S_HostIP"),
		DstK8S_HostName:        stringField(m, "DstK8S_HostName"),
		DstK8S_Zone:            stringField(m, "DstK8S_Zone"),
		DstK8S_NetworkName:     stringField(m, "DstK8S_NetworkName"),
		DnsFlagsResponseCode:   stringField(m, "DnsFlagsResponseCode"),
		PktDropLatestState:     stringField(m, "PktDropLatestState"),
		PktDropLatestDropCause: stringField(m, "PktDropLatestDropCause"),
		AgentIP:                stringField(m, "AgentIP"),
	}
	if v, ok := int64Field(m, "TimeFlowStartMs"); ok {
		rec.TimeFlowStartMs = v
	}
	if v, ok := int64Field(m, "TimeFlowEndMs"); ok {
		rec.TimeFlowEndMs = v
	}
	rec.TimeReceived = optionalInt64(m, "TimeReceived")
	rec.SrcPort = optionalInt32(m, "SrcPort")
	rec.DstPort = optionalInt32(m, "DstPort")
	rec.Proto = optionalInt32(m, "Proto")
	rec.Etype = optionalInt32(m, "Etype")
	rec.Dscp = optionalInt32(m, "Dscp")
	rec.Flags = optionalInt32(m, "Flags")
	rec.Bytes = optionalInt64(m, "Bytes")
	rec.Packets = optionalInt64(m, "Packets")
	rec.Sampling = optionalInt64(m, "Sampling")
	rec.TimeFlowRttNs = optionalInt64(m, "TimeFlowRttNs")
	rec.DnsId = optionalInt32(m, "DnsId")
	rec.DnsLatencyMs = optionalInt64(m, "DnsLatencyMs")
	rec.PktDropBytes = optionalInt64(m, "PktDropBytes")
	rec.PktDropPackets = optionalInt64(m, "PktDropPackets")
	rec.FlowDirection = optionalInt32(m, "FlowDirection")
	return rec
}

func stringField(m config.GenericMap, key string) string {
	if v, ok := m[key]; ok {
		return utils.ConvertToString(v)
	}
	return ""
}

func int64Field(m config.GenericMap, key string) (int64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	n, err := utils.ConvertToInt64(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func optionalInt64(m config.GenericMap, key string) *int64 {
	n, ok := int64Field(m, key)
	if !ok {
		return nil
	}
	return &n
}

func optionalInt32(m config.GenericMap, key string) *int32 {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	n, err := utils.ConvertToInt64(v)
	if err != nil {
		return nil
	}
	i := int32(n)
	return &i
}
