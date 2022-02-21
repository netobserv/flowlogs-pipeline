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

package pipeline

import (
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs2metrics/pkg/pipeline/write"
	"github.com/netobserv/flowlogs2metrics/pkg/test"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

const configTemplate = `---
log-level: debug
pipeline:
  ingest:
    type: file
    file:
      filename: /tmp/simple_end_to_end_test_data.txt
  decode:
    type: json
  transform:
    - type: generic
      generic:
        - input: Bytes
          output: fl2m_bytes
        - input: DstAddr
          output: fl2m_dstAddr
        - input: DstPort
          output: fl2m_dstPort
        - input: Packets
          output: fl2m_packets
        - input: SrcAddr
          output: fl2m_srcAddr
        - input: SrcPort
          output: fl2m_srcPort
  extract:
    type: none
  encode:
    type: none
  write:
    type: none
`
const inputData = `{"BiFlowDirection":0,"Bytes":20800,"DstAS":0,"DstAddr":"10.130.2.1","DstMac":"ce:ea:43:28:6d:88","DstNet":0,"DstPort":36936,"DstVlan":0,"EgressVrfID":0,"Etype":2048,"FlowDirection":0,"ForwardingStatus":0,"FragmentId":0,"FragmentOffset":0,"HasMPLS":false,"IPTTL":0,"IPTos":0,"IPv6FlowLabel":0,"IcmpCode":0,"IcmpType":0,"InIf":18,"IngressVrfID":0,"MPLS1Label":0,"MPLS1TTL":0,"MPLS2Label":0,"MPLS2TTL":0,"MPLS3Label":0,"MPLS3TTL":0,"MPLSCount":0,"MPLSLastLabel":0,"MPLSLastTTL":0,"OutIf":0,"Packets":400,"Proto":6,"SamplerAddress":"ZEAACA==","SamplingRate":0,"SequenceNum":1919,"SrcAS":0,"SrcAddr":"10.130.2.13","SrcHostIP":"10.0.197.206","SrcMac":"0a:58:0a:82:02:0d","SrcNet":0,"SrcPod":"observatorium-loki-ingester-0","SrcPort":3100,"SrcVlan":0,"SrcWorkloadKind":"StatefulSet","TCPFlags":0,"TimeFlowEnd":0,"TimeFlowStart":0,"TimeReceived":1637501832,"Type":4,"VlanId":0}
{"BiFlowDirection":0,"Bytes":20800,"DstAS":0,"DstAddr":"10.130.2.2","DstMac":"ce:ea:43:28:6d:88","DstNet":0,"DstPort":36936,"DstVlan":0,"EgressVrfID":0,"Etype":2048,"FlowDirection":0,"ForwardingStatus":0,"FragmentId":0,"FragmentOffset":0,"HasMPLS":false,"IPTTL":0,"IPTos":0,"IPv6FlowLabel":0,"IcmpCode":0,"IcmpType":0,"InIf":18,"IngressVrfID":0,"MPLS1Label":0,"MPLS1TTL":0,"MPLS2Label":0,"MPLS2TTL":0,"MPLS3Label":0,"MPLS3TTL":0,"MPLSCount":0,"MPLSLastLabel":0,"MPLSLastTTL":0,"OutIf":0,"Packets":400,"Proto":6,"SamplerAddress":"ZEAACA==","SamplingRate":0,"SequenceNum":1919,"SrcAS":0,"SrcAddr":"10.130.2.13","SrcHostIP":"10.0.197.206","SrcMac":"0a:58:0a:82:02:0d","SrcNet":0,"SrcPod":"observatorium-loki-ingester-0","SrcPort":3100,"SrcVlan":0,"SrcWorkloadKind":"StatefulSet","TCPFlags":0,"TimeFlowEnd":0,"TimeFlowStart":0,"TimeReceived":1637501832,"Type":4,"VlanId":0}
{"BiFlowDirection":0,"Bytes":20800,"DstAS":0,"DstAddr":"10.130.2.3","DstMac":"ce:ea:43:28:6d:88","DstNet":0,"DstPort":36936,"DstVlan":0,"EgressVrfID":0,"Etype":2048,"FlowDirection":0,"ForwardingStatus":0,"FragmentId":0,"FragmentOffset":0,"HasMPLS":false,"IPTTL":0,"IPTos":0,"IPv6FlowLabel":0,"IcmpCode":0,"IcmpType":0,"InIf":18,"IngressVrfID":0,"MPLS1Label":0,"MPLS1TTL":0,"MPLS2Label":0,"MPLS2TTL":0,"MPLS3Label":0,"MPLS3TTL":0,"MPLSCount":0,"MPLSLastLabel":0,"MPLSLastTTL":0,"OutIf":0,"Packets":400,"Proto":6,"SamplerAddress":"ZEAACA==","SamplingRate":0,"SequenceNum":1919,"SrcAS":0,"SrcAddr":"10.130.2.13","SrcHostIP":"10.0.197.206","SrcMac":"0a:58:0a:82:02:0d","SrcNet":0,"SrcPod":"observatorium-loki-ingester-0","SrcPort":3100,"SrcVlan":0,"SrcWorkloadKind":"StatefulSet","TCPFlags":0,"TimeFlowEnd":0,"TimeFlowStart":0,"TimeReceived":1637501832,"Type":4,"VlanId":0}
`

func Test_SimpleEndToEnd(t *testing.T) {
	fmt.Printf("entering Test_SimpleEndToEnd")
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var mainPipeline *Pipeline
	var err error
	var b []byte
	// perform initializations that would usually be done in main
	fmt.Printf("before os.WriteFile\n")
	err = os.WriteFile("/tmp/simple_end_to_end_test_data.txt", []byte(inputData), 0644)
	require.NoError(t, err)
	defer os.Remove("/tmp/simple_end_to_end_test_data.txt")

	fmt.Printf("before test.InitConfig\n")
	v := test.InitConfig(t, configTemplate)
	config.Opt.PipeLine.Ingest.Type = "file"
	config.Opt.PipeLine.Decode.Type = "json"
	config.Opt.PipeLine.Extract.Type = "none"
	config.Opt.PipeLine.Encode.Type = "none"
	config.Opt.PipeLine.Write.Type = "none"
	config.Opt.PipeLine.Ingest.File.Filename = "/tmp/simple_end_to_end_test_data.txt"
	fmt.Printf("filename = %s\n", config.Opt.PipeLine.Ingest.File.Filename)

	val := v.Get("pipeline.transform\n")
	b, err = json.Marshal(&val)
	require.NoError(t, err)
	config.Opt.PipeLine.Transform = string(b)

	mainPipeline, err = NewPipeline()
	require.NoError(t, err)

	// The file ingester reads the entire file, pushes it down the pipeline, and then exits
	// So we don't need to run it in a separate go-routine
	mainPipeline.Run()
	// What is there left to check? Check length of saved data of each stage in private structure.
	ingester := mainPipeline.Ingester.(*ingest.IngestFile)
	decoder := mainPipeline.Decoder.(*decode.DecodeJson)
	writer := mainPipeline.Writer.(*write.WriteNone)
	require.Equal(t, len(ingester.PrevRecords), len(decoder.PrevRecords))
	require.Equal(t, len(ingester.PrevRecords), len(writer.PrevRecords))
}
