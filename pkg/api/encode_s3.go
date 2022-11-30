/*
 * Copyright (C) 2022 IBM, Inc.
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

package api

type EncodeS3 struct {
	Account                string                 `yaml:"account" json:"account" doc:"tenant id for this flow collector"`
	Endpoint               string                 `yaml:"endpoint" json:"endpoint" doc:"address of s3 server"`
	AccessKeyId            string                 `yaml:"accessKeyId" json:"accessKeyId" doc:"username to connect to server"`
	SecretAccessKey        string                 `yaml:"secretAccessKey" json:"secretAccessKey" doc:"password to connect to server"`
	Bucket                 string                 `yaml:"bucket" json:"bucket" doc:"bucket into which to store objects"`
	WriteTimeout           int64                  `yaml:"writeTimeout,omitempty" json:"writeTimeout,omitempty" doc:"timeout (in seconds) for write operation"`
	BatchSize              int                    `yaml:"batchSize,omitempty" json:"batchSize,omitempty" doc:"limit on how many flows will be buffered before being sent to an object"`
	ObjectHeaderParameters map[string]interface{} `yaml:"objectHeaderParameters,omitempty" json:"objectHeaderParameters,omitempty" doc:"parameters to include in object header (key/value pairs)"`
	// TBD: (TLS?) security parameters
}

// Structure of flow-log object as specified in https://cloud.ibm.com/docs/vpc?topic=vpc-fl-analyze#flow-log-data-format
// TBD - perhaps we don't need this structure, except for documentation.
/*
type FlowLogObject struct {
	StartTime                      string `yaml:"start_time,omitempty" json:"start_time,omitempty"`
	EndTime                        string `yaml:"end_time,omitempty" json:"end_time,omitempty"`
	ConnectionStartTime            string `yaml:"connection_start_time,omitempty" json:"connection_start_time,omitempty"`
	Direction                      string `yaml:"direction,omitempty" json:"direction,omitempty"`
	Action                         string `yaml:"action,omitempty" json:"action,omitempty"`
	InitiatorIp                    string `yaml:"initiator_ip,omitempty" json:"initiator_ip,omitempty"`
	TargetIp                       string `yaml:"target_ip,omitempty" json:"target_ip,omitempty"`
	InitiatorPort                  int16  `yaml:"initiator_port,omitempty" json:"initiator_port,omitempty"`
	TargetPort                     int16  `yaml:"target_port,omitempty" json:"target_port,omitempty"`
	TransportProtocol              uint8  `yaml:"transport_protocol,omitempty" json:"transport_protocol,omitempty"`
	EtherType                      string `yaml:"ether_type,omitempty" json:"ether_type,omitempty"`
	WasInitiated                   bool   `yaml:"was_initiated,omitempty" json:"was_initiated,omitempty"`
	WasTerminated                  bool   `yaml:"was_terminated,omitempty" json:"was_terminated,omitempty"`
	BytesFromInitiator             int64  `yaml:"bytes_from_initiator,omitempty" json:"bytes_from_initiator,omitempty"`
	PacketsFromInitiator           int64  `yaml:"packets_from_initiator,omitempty" json:"packets_from_initiator,omitempty"`
	BytesFromTarget                int64  `yaml:"bytes_from_target,omitempty" json:"bytes_from_target,omitempty"`
	PacketsFromTarget              int64  `yaml:"packets_from_target,omitempty" json:"packets_from_target,omitempty"`
	CumulativeBytesFromInitiator   int64  `yaml:"cumulative_bytes_from_initiator,omitempty" json:"cumulative_bytes_from_initiator,omitempty"`
	CumulativePacketsFromInitiator int64  `yaml:"cumulative_packets_from_initiator,omitempty" json:"cumulative_packets_from_initiator,omitempty"`
	CumulativeBytesFromTarget      int64  `yaml:"cumulative_bytes_from_target,omitempty" json:"cumulative_bytes_from_target,omitempty"`
	CumulativePacketsFromTarget    int64  `yaml:"cumulative_packets_from_target,omitempty" json:"cumulative_packets_from_target,omitempty"`
}

type FlowLogObjectHeader struct {
	Version              string          `json:"version"`
	CollectorCrn         string          `json:"collector_crn"`
	AttachedEndpointType string          `json:"attached_endpoint_type"`
	NetworkInterfaceId   string          `json:"network_interface_id"`
	InstanceCrn          string          `json:"instance_crn"`
	VpcCrn               string          `json:"vpc_crn"`
	CaptureStartTime     string          `json:"capture_start_time"`
	CaptureEndTime       string          `json:"capture_end_time"`
	NumberOfFlowLogs     uint32          `json:"number_of_flow_logs"`
	FlowLogs             []FlowLogObject `json:"flow_logs"`
	State                string          `json:"state"`
}
*/
