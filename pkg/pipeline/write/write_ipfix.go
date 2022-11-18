/*
 * Copyright (C) 2021 IBM, Inc.
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

package write

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/go-ipfix/pkg/entities"
	ipfixExporter "github.com/vmware/go-ipfix/pkg/exporter"
	"github.com/vmware/go-ipfix/pkg/registry"
)

type writeIpfix struct {
	hostPort     string
	transport    string
	templateIDv4 uint16
	templateIDv6 uint16
	exporter     *ipfixExporter.ExportingProcess
}

// IPv6Type value as defined in IEEE 802: https://www.iana.org/assignments/ieee-802-numbers/ieee-802-numbers.xhtml
const IPv6Type = 0x86DD

func makeByteFromUint8(value uint8) []byte {
	bs := make([]byte, 1)
	bs[0] = value
	return bs
}

func makeByteFromUint16(value uint16) []byte {
	bs := make([]byte, 2)
	binary.BigEndian.PutUint16(bs, value)
	return bs
}

func makeByteFromUint64(value uint64) []byte {
	bs := make([]byte, 8)
	binary.BigEndian.PutUint64(bs, value)
	return bs
}

func addElementToTemplate(elementName string, value []byte, elements *[]entities.InfoElementWithValue) error {
	element, err := registry.GetInfoElement(elementName, registry.IANAEnterpriseID)
	if err != nil {
		log.WithError(err).Errorf("Did not find the element with name %s", elementName)
		return err
	}
	ie, err := entities.DecodeAndCreateInfoElementWithValue(element, value)
	if err != nil {
		log.WithError(err).Errorf("Failed to decode element %s", elementName)
		return err
	}
	*elements = append(*elements, ie)
	return nil
}

// func loadSevoneRegistry() {
// 	registerInfoElement(*entities.NewInfoElement("sourcePodNamespace", 7733, 13, 56506, 65535), 56506)
// 	registerInfoElement(*entities.NewInfoElement("sourcePodName", 7734, 13, 56506, 65535), 56506)
// 	registerInfoElement(*entities.NewInfoElement("destinationPodNamespace", 7735, 13, 56506, 65535), 56506)
// 	registerInfoElement(*entities.NewInfoElement("destinationPodName", 7736, 13, 56506, 65535), 56506)
// 	registerInfoElement(*entities.NewInfoElement("sourceNodeName", 7737, 13, 56506, 65535), 56506)
// }

func SendTemplateRecordv4(exporter *ipfixExporter.ExportingProcess) (uint16, error) {
	templateID := exporter.NewTemplateID()
	templateSet := entities.NewSet(false)
	err := templateSet.PrepareSet(entities.Template, templateID)
	if err != nil {
		log.WithError(err).Error("Failed in PrepareSet")
		return 0, err
	}
	elements := make([]entities.InfoElementWithValue, 0)

	err = addElementToTemplate("ethernetType", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowDirection", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceMacAddress", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationMacAddress", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceIPv4Address", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationIPv4Address", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("protocolIdentifier", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceTransportPort", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationTransportPort", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("octetDeltaCount", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowStartMilliseconds", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowEndMilliseconds", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("packetDeltaCount", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("interfaceName", nil, &elements)
	if err != nil {
		return 0, err
	}

	fmt.Printf("%+v", elements)
	err = templateSet.AddRecord(elements, templateID)
	if err != nil {
		log.WithError(err).Error("Failed in Add Record")
		return 0, err
	}
	_, err = exporter.SendSet(templateSet)
	if err != nil {
		log.WithError(err).Error("Failed to send template record")
		return 0, err
	}

	return templateID, nil
}

func SendTemplateRecordv6(exporter *ipfixExporter.ExportingProcess) (uint16, error) {
	templateID := exporter.NewTemplateID()
	templateSet := entities.NewSet(false)
	err := templateSet.PrepareSet(entities.Template, templateID)
	if err != nil {
		log.WithError(err).Error("Failed in PrepareSet")
		return 0, err
	}
	elements := make([]entities.InfoElementWithValue, 0)

	err = addElementToTemplate("ethernetType", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowDirection", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceMacAddress", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationMacAddress", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceIPv6Address", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationIPv6Address", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("nextHeaderIPv6", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceTransportPort", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationTransportPort", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("octetDeltaCount", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowStartMilliseconds", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowEndMilliseconds", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("packetDeltaCount", nil, &elements)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("interfaceName", nil, &elements)
	if err != nil {
		return 0, err
	}

	err = templateSet.AddRecord(elements, templateID)
	if err != nil {
		log.WithError(err).Error("Failed in Add Record")
		return 0, err
	}
	_, err = exporter.SendSet(templateSet)
	if err != nil {
		log.WithError(err).Error("Failed to send template record")
		return 0, err
	}

	return templateID, nil
}

func (t *writeIpfix) sendDataRecord(record config.GenericMap) error {
	// Create data set with 1 data record

	elements := make([]entities.InfoElementWithValue, 0)

	err := addElementToTemplate("ethernetType", makeByteFromUint16(uint16(record["Etype"].(uint32))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("flowDirection", makeByteFromUint8(uint8(record["FlowDirection"].(int))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("sourceMacAddress", net.HardwareAddr(record["SrcMac"].(string)), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationMacAddress", net.HardwareAddr(record["DstMac"].(string)), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("sourceIPv4Address", net.ParseIP(record["SrcAddr"].(string)), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationIPv4Address", net.ParseIP(record["DstAddr"].(string)), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("protocolIdentifier", makeByteFromUint8(uint8(record["Proto"].(uint32))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("sourceTransportPort", makeByteFromUint16(uint16(record["SrcPort"].(uint32))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationTransportPort", makeByteFromUint16(uint16(record["DstPort"].(uint32))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("octetDeltaCount", makeByteFromUint64(record["Bytes"].(uint64)), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("flowStartMilliseconds", makeByteFromUint64(uint64(record["TimeFlowStartMs"].(int64))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("flowEndMilliseconds", makeByteFromUint64(uint64(record["TimeFlowEndMs"].(int64))), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("packetDeltaCount", makeByteFromUint64(record["Packets"].(uint64)), &elements)
	if err != nil {
		return err
	}
	err = addElementToTemplate("interfaceName", []byte(record["Interface"].(string)), &elements)
	if err != nil {
		return err
	}

	dataSet := entities.NewSet(false)
	err = dataSet.PrepareSet(entities.Data, t.templateIDv4)
	if err != nil {
		log.Errorf("Failed in PrepareSet")
		return err
	}
	err = dataSet.AddRecord(elements, t.templateIDv4)
	if err != nil {
		log.WithError(err).Error("Failed in Add Record")
		return err
	}
	_, err = t.exporter.SendSet(dataSet)
	if err != nil {
		log.WithError(err).Error("Failed in Send Record")
		return err
	}
	log.Printf("Sending IPFIX with %s -> %s", record["SrcAddr"], record["DstAddr"])
	return nil
}

// }

// Write writes a flow before being stored
func (t *writeIpfix) Write(entry config.GenericMap) {
	log.Tracef("entering writeStdout Write")

	var order sort.StringSlice
	for fieldName := range entry {
		order = append(order, fieldName)
	}
	order.Sort()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "\n\nFlow record at %s:\n", time.Now().Format(time.StampMilli))
	for _, field := range order {
		fmt.Fprintf(w, "%v\t=\t%v\n", field, entry[field])
	}
	w.Flush()
	err := t.sendDataRecord(entry)
	if err != nil {
		log.WithError(err).Error("Failed in send IPFIX record")
	}
}

// NewWriteStdout create a new write
func NewWriteIpfix(params config.StageParam) (Writer, error) {
	log.Debugf("entering NewWriteIpfix")
	writeIpfix := &writeIpfix{}
	if params.Write != nil && params.Write.Ipfix != nil {
		writeIpfix.transport = params.Write.Ipfix.Transport
		writeIpfix.hostPort = fmt.Sprintf("%s:%d", params.Write.Ipfix.TargetHost, params.Write.Ipfix.TargetPort)
	}
	// Initialize IPFIX registry and send templates
	registry.LoadRegistry()
	// Create exporter using local server info
	input := ipfixExporter.ExporterInput{
		CollectorAddress:    writeIpfix.hostPort,
		CollectorProtocol:   writeIpfix.transport,
		ObservationDomainID: 1,
		TempRefTimeout:      1,
	}
	var err error
	writeIpfix.exporter, err = ipfixExporter.InitExportingProcess(input)
	if err != nil {
		log.Fatalf("Got error when connecting to local server %s: %v", writeIpfix.hostPort, err)
		return nil, err
	}
	log.Infof("Created exporter connecting to local server with address: %s", writeIpfix.hostPort)

	writeIpfix.templateIDv4, err = SendTemplateRecordv4(writeIpfix.exporter)
	if err != nil {
		log.WithError(err).Error("Failed in send IPFIX template v4 record")
		return nil, err
	}

	writeIpfix.templateIDv6, err = SendTemplateRecordv6(writeIpfix.exporter)
	if err != nil {
		log.WithError(err).Error("Failed in send IPFIX template v6 record")
		return nil, err
	}
	return writeIpfix, nil
}
