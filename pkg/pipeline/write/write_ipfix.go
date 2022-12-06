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

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/vmware/go-ipfix/pkg/entities"
	ipfixExporter "github.com/vmware/go-ipfix/pkg/exporter"
	"github.com/vmware/go-ipfix/pkg/registry"
)

type writeIpfix struct {
	hostPort           string
	transport          string
	templateIDv4       uint16
	templateIDv6       uint16
	enrichEnterpriseID uint32
	exporter           *ipfixExporter.ExportingProcess
}

// IPv6Type value as defined in IEEE 802: https://www.iana.org/assignments/ieee-802-numbers/ieee-802-numbers.xhtml
const IPv6Type = 0x86DD

var ilog = logrus.WithField("component", "write.Ipfix")

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

func addElementToTemplate(elementName string, value []byte, elements *[]entities.InfoElementWithValue, registryID uint32) error {
	element, err := registry.GetInfoElement(elementName, registryID)
	if err != nil {
		ilog.WithError(err).Errorf("Did not find the element with name %s", elementName)
		return err
	}
	ie, err := entities.DecodeAndCreateInfoElementWithValue(element, value)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to decode element %s", elementName)
		return err
	}
	*elements = append(*elements, ie)
	return nil
}

func addKubeContextToTemplate(elements *[]entities.InfoElementWithValue, registryID uint32) error {
	err := addElementToTemplate("sourcePodNamespace", nil, elements, registryID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("sourcePodName", nil, elements, registryID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationPodNamespace", nil, elements, registryID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationPodName", nil, elements, registryID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("sourceNodeName", nil, elements, registryID)
	if err != nil {
		return err
	}
	return nil
}

func addKubeContextToRecord(elements *[]entities.InfoElementWithValue, record config.GenericMap, registryID uint32) error {
	if record["SrcK8S_Namespace"] != nil {
		err := addElementToTemplate("sourcePodNamespace", []byte(record["SrcK8S_Namespace"].(string)), elements, registryID)
		if err != nil {
			return err
		}
	} else {
		err := addElementToTemplate("sourcePodNamespace", []byte("none"), elements, registryID)
		if err != nil {
			return err
		}
	}
	if record["SrcK8S_OwnerName"] != nil {
		err := addElementToTemplate("sourcePodName", []byte(record["SrcK8S_OwnerName"].(string)), elements, registryID)
		if err != nil {
			return err
		}
	} else {
		err := addElementToTemplate("sourcePodName", []byte("none"), elements, registryID)
		if err != nil {
			return err
		}
	}
	if record["DstK8S_Namespace"] != nil {
		err := addElementToTemplate("destinationPodNamespace", []byte(record["DstK8S_Namespace"].(string)), elements, registryID)
		if err != nil {
			return err
		}
	} else {
		err := addElementToTemplate("destinationPodNamespace", []byte("none"), elements, registryID)
		if err != nil {
			return err
		}
	}
	if record["DstK8S_OwnerName"] != nil {
		err := addElementToTemplate("destinationPodName", []byte(record["DstK8S_OwnerName"].(string)), elements, registryID)
		if err != nil {
			return err
		}
	} else {
		err := addElementToTemplate("destinationPodName", []byte("none"), elements, registryID)
		if err != nil {
			return err
		}
	}
	if record["SrcK8S_HostName"] != nil {
		err := addElementToTemplate("sourceNodeName", []byte(record["SrcK8S_HostName"].(string)), elements, registryID)
		if err != nil {
			return err
		}
	} else {
		err := addElementToTemplate("sourceNodeName", []byte("none"), elements, registryID)
		if err != nil {
			return err
		}
	}
	return nil
}

func loadCustomRegistry(EnterpriseID uint32) error {
	err := registry.InitNewRegistry(EnterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to initialize registry")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("sourcePodNamespace", 7733, 13, EnterpriseID, 65535)), EnterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("sourcePodName", 7734, 13, EnterpriseID, 65535)), EnterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("destinationPodNamespace", 7735, 13, EnterpriseID, 65535)), EnterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("destinationPodName", 7736, 13, EnterpriseID, 65535)), EnterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("sourceNodeName", 7737, 13, EnterpriseID, 65535)), EnterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	return nil
}

func SendTemplateRecordv4(exporter *ipfixExporter.ExportingProcess, enrichEnterpriseID uint32) (uint16, error) {
	templateID := exporter.NewTemplateID()
	templateSet := entities.NewSet(false)
	err := templateSet.PrepareSet(entities.Template, templateID)
	if err != nil {
		ilog.WithError(err).Error("Failed in PrepareSet")
		return 0, err
	}
	elements := make([]entities.InfoElementWithValue, 0)

	err = addElementToTemplate("ethernetType", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowDirection", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceMacAddress", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationMacAddress", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceIPv4Address", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationIPv4Address", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("protocolIdentifier", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceTransportPort", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationTransportPort", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("octetDeltaCount", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowStartMilliseconds", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowEndMilliseconds", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("packetDeltaCount", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("interfaceName", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	if enrichEnterpriseID != 0 {
		err = addKubeContextToTemplate(&elements, enrichEnterpriseID)
		if err != nil {
			return 0, err
		}
	}
	err = templateSet.AddRecord(elements, templateID)
	if err != nil {
		ilog.WithError(err).Error("Failed in Add Record")
		return 0, err
	}
	_, err = exporter.SendSet(templateSet)
	if err != nil {
		ilog.WithError(err).Error("Failed to send template record")
		return 0, err
	}

	return templateID, nil
}

func SendTemplateRecordv6(exporter *ipfixExporter.ExportingProcess, enrichEnterpriseID uint32) (uint16, error) {
	templateID := exporter.NewTemplateID()
	templateSet := entities.NewSet(false)
	err := templateSet.PrepareSet(entities.Template, templateID)
	if err != nil {
		ilog.WithError(err).Error("Failed in PrepareSet")
		return 0, err
	}
	elements := make([]entities.InfoElementWithValue, 0)

	err = addElementToTemplate("ethernetType", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowDirection", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceMacAddress", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationMacAddress", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceIPv6Address", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationIPv6Address", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("nextHeaderIPv6", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("sourceTransportPort", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("destinationTransportPort", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("octetDeltaCount", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowStartMilliseconds", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("flowEndMilliseconds", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("packetDeltaCount", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	err = addElementToTemplate("interfaceName", nil, &elements, registry.IANAEnterpriseID)
	if err != nil {
		return 0, err
	}
	if enrichEnterpriseID != 0 {
		err = addKubeContextToTemplate(&elements, enrichEnterpriseID)
		if err != nil {
			return 0, err
		}
	}

	err = templateSet.AddRecord(elements, templateID)
	if err != nil {
		ilog.WithError(err).Error("Failed in Add Record")
		return 0, err
	}
	_, err = exporter.SendSet(templateSet)
	if err != nil {
		ilog.WithError(err).Error("Failed to send template record")
		return 0, err
	}

	return templateID, nil
}

func (t *writeIpfix) sendDataRecord(record config.GenericMap) error {
	// Create data set with 1 data record

	elements := make([]entities.InfoElementWithValue, 0)

	err := addElementToTemplate("ethernetType", makeByteFromUint16(uint16(record["Etype"].(uint32))), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("flowDirection", makeByteFromUint8(uint8(record["FlowDirection"].(int))), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("sourceMacAddress", net.HardwareAddr(record["SrcMac"].(string)), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationMacAddress", net.HardwareAddr(record["DstMac"].(string)), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	if IPv6Type == record["Etype"].(uint32) {
		err = addElementToTemplate("sourceIPv6Address", net.ParseIP(record["SrcAddr"].(string)), &elements, registry.IANAEnterpriseID)
		if err != nil {
			return err
		}
		err = addElementToTemplate("destinationIPv6Address", net.ParseIP(record["DstAddr"].(string)), &elements, registry.IANAEnterpriseID)
		if err != nil {
			return err
		}
		err = addElementToTemplate("nextHeaderIPv6", makeByteFromUint8(uint8(record["Proto"].(uint32))), &elements, registry.IANAEnterpriseID)
		if err != nil {
			return err
		}
	} else {
		err = addElementToTemplate("sourceIPv4Address", net.ParseIP(record["SrcAddr"].(string)), &elements, registry.IANAEnterpriseID)
		if err != nil {
			return err
		}
		err = addElementToTemplate("destinationIPv4Address", net.ParseIP(record["DstAddr"].(string)), &elements, registry.IANAEnterpriseID)
		if err != nil {
			return err
		}
		err = addElementToTemplate("protocolIdentifier", makeByteFromUint8(uint8(record["Proto"].(uint32))), &elements, registry.IANAEnterpriseID)
		if err != nil {
			return err
		}
	}

	err = addElementToTemplate("sourceTransportPort", makeByteFromUint16(uint16(record["SrcPort"].(uint32))), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("destinationTransportPort", makeByteFromUint16(uint16(record["DstPort"].(uint32))), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("octetDeltaCount", makeByteFromUint64(record["Bytes"].(uint64)), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("flowStartMilliseconds", makeByteFromUint64(uint64(record["TimeFlowStartMs"].(int64))), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("flowEndMilliseconds", makeByteFromUint64(uint64(record["TimeFlowEndMs"].(int64))), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("packetDeltaCount", makeByteFromUint64(record["Packets"].(uint64)), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	err = addElementToTemplate("interfaceName", []byte(record["Interface"].(string)), &elements, registry.IANAEnterpriseID)
	if err != nil {
		return err
	}
	if t.enrichEnterpriseID != 0 {
		err = addKubeContextToRecord(&elements, record, t.enrichEnterpriseID)
		if err != nil {
			return err
		}
	}

	dataSet := entities.NewSet(false)
	if IPv6Type == record["Etype"].(uint32) {
		err = dataSet.PrepareSet(entities.Data, t.templateIDv6)
		if err != nil {
			ilog.Errorf("Failed in PrepareSet")
			return err
		}
	} else {
		err = dataSet.PrepareSet(entities.Data, t.templateIDv4)
		if err != nil {
			ilog.Errorf("Failed in PrepareSet")
			return err
		}
	}
	err = dataSet.AddRecord(elements, t.templateIDv4)
	if err != nil {
		ilog.WithError(err).Error("Failed in Add Record")
		return err
	}
	_, err = t.exporter.SendSet(dataSet)
	if err != nil {
		ilog.WithError(err).Error("Failed in Send Record")
		return err
	}
	return nil
}

// Write writes a flow before being stored
func (t *writeIpfix) Write(entry config.GenericMap) {
	ilog.Tracef("entering writeIpfix Write")

	err := t.sendDataRecord(entry)
	if err != nil {
		ilog.WithError(err).Error("Failed in send IPFIX record")
	}
}

// NewWriteStdout create a new write
func NewWriteIpfix(params config.StageParam) (Writer, error) {
	ilog.Debugf("entering NewWriteIpfix")
	writeIpfix := &writeIpfix{}
	if params.Write != nil && params.Write.Ipfix != nil {
		writeIpfix.transport = params.Write.Ipfix.Transport
		writeIpfix.hostPort = fmt.Sprintf("%s:%d", params.Write.Ipfix.TargetHost, params.Write.Ipfix.TargetPort)
		writeIpfix.enrichEnterpriseID = uint32(params.Write.Ipfix.EnterpriseID)
	}
	// Initialize IPFIX registry and send templates
	registry.LoadRegistry()
	var err error
	if params.Write.Ipfix.EnterpriseID != 0 {
		err = loadCustomRegistry(writeIpfix.enrichEnterpriseID)
		if err != nil {
			ilog.Fatalf("Failed to load Custom(%d) Registry", writeIpfix.enrichEnterpriseID)
		}
	}

	// Create exporter using local server info
	input := ipfixExporter.ExporterInput{
		CollectorAddress:    writeIpfix.hostPort,
		CollectorProtocol:   writeIpfix.transport,
		ObservationDomainID: 1,
		TempRefTimeout:      1,
	}
	writeIpfix.exporter, err = ipfixExporter.InitExportingProcess(input)
	if err != nil {
		ilog.Fatalf("Got error when connecting to server %s: %v", writeIpfix.hostPort, err)
		return nil, err
	}
	ilog.Infof("Created exporter connecting to server with address: %s", writeIpfix.hostPort)

	writeIpfix.templateIDv4, err = SendTemplateRecordv4(writeIpfix.exporter, writeIpfix.enrichEnterpriseID)
	if err != nil {
		ilog.WithError(err).Error("Failed in send IPFIX template v4 record")
		return nil, err
	}

	writeIpfix.templateIDv6, err = SendTemplateRecordv6(writeIpfix.exporter, writeIpfix.enrichEnterpriseID)
	if err != nil {
		ilog.WithError(err).Error("Failed in send IPFIX template v6 record")
		return nil, err
	}
	return writeIpfix, nil
}
