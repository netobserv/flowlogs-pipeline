/*
 * Copyright (C) 2024 IBM, Inc.
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
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/sirupsen/logrus"
	"github.com/vmware/go-ipfix/pkg/entities"
	ipfixExporter "github.com/vmware/go-ipfix/pkg/exporter"
	"github.com/vmware/go-ipfix/pkg/registry"
)

type writeIpfix struct {
	templateIDv4 uint16
	templateIDv6 uint16
	exporter     *ipfixExporter.ExportingProcess
	tplV4        entities.Set
	tplV6        entities.Set
	entitiesV4   []entities.InfoElementWithValue
	entitiesV6   []entities.InfoElementWithValue
}

type FieldMap struct {
	Key     string
	Getter  func(entities.InfoElementWithValue) any
	Setter  func(entities.InfoElementWithValue, any)
	Matcher func(entities.InfoElementWithValue, any) bool
}

// IPv6Type value as defined in IEEE 802: https://www.iana.org/assignments/ieee-802-numbers/ieee-802-numbers.xhtml
const IPv6Type uint16 = 0x86DD

var (
	ilog       = logrus.WithField("component", "write.Ipfix")
	IANAFields = []string{
		"ethernetType",
		"flowDirection",
		"sourceMacAddress",
		"destinationMacAddress",
		"protocolIdentifier",
		"sourceTransportPort",
		"destinationTransportPort",
		"octetDeltaCount",
		"flowStartMilliseconds",
		"flowEndMilliseconds",
		"packetDeltaCount",
		"interfaceName",
		"tcpControlBits",
	}
	IPv4IANAFields = append([]string{
		"sourceIPv4Address",
		"destinationIPv4Address",
		"icmpTypeIPv4",
		"icmpCodeIPv4",
	}, IANAFields...)
	IPv6IANAFields = append([]string{
		"sourceIPv6Address",
		"destinationIPv6Address",
		"nextHeaderIPv6",
		"icmpTypeIPv6",
		"icmpCodeIPv6",
	}, IANAFields...)
	KubeFields = []string{
		"sourcePodNamespace",
		"sourcePodName",
		"destinationPodNamespace",
		"destinationPodName",
		"sourceNodeName",
		"destinationNodeName",
	}
	CustomNetworkFields = []string{
		"timeFlowRttNs",
		"interfaces",
		"directions",
	}

	MapIPFIXKeys = map[string]FieldMap{
		"sourceIPv4Address": {
			Key:    "SrcAddr",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetIPAddressValue().String() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetIPAddressValue(net.ParseIP(rec.(string))) },
		},
		"destinationIPv4Address": {
			Key:    "DstAddr",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetIPAddressValue().String() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetIPAddressValue(net.ParseIP(rec.(string))) },
		},
		"sourceIPv6Address": {
			Key:    "SrcAddr",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetIPAddressValue().String() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetIPAddressValue(net.ParseIP(rec.(string))) },
		},
		"destinationIPv6Address": {
			Key:    "DstAddr",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetIPAddressValue().String() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetIPAddressValue(net.ParseIP(rec.(string))) },
		},
		"nextHeaderIPv6": {
			Key:    "Proto",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned8Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned8Value(rec.(uint8)) },
		},
		"sourceMacAddress": {
			Key:    "SrcMac",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetMacAddressValue().String() },
			Setter: func(elt entities.InfoElementWithValue, rec any) {
				mac, _ := net.ParseMAC(rec.(string))
				elt.SetMacAddressValue(mac)
			},
		},
		"destinationMacAddress": {
			Key:    "DstMac",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetMacAddressValue().String() },
			Setter: func(elt entities.InfoElementWithValue, rec any) {
				mac, _ := net.ParseMAC(rec.(string))
				elt.SetMacAddressValue(mac)
			},
		},
		"ethernetType": {
			Key:    "Etype",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned16Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned16Value(rec.(uint16)) },
		},
		"flowDirection": {
			Key: "IfDirections",
			Setter: func(elt entities.InfoElementWithValue, rec any) {
				if dirs, ok := rec.([]int); ok && len(dirs) > 0 {
					elt.SetUnsigned8Value(uint8(dirs[0]))
				}
			},
			Matcher: func(elt entities.InfoElementWithValue, expected any) bool {
				ifdirs := expected.([]int)
				return int(elt.GetUnsigned8Value()) == ifdirs[0]
			},
		},
		"directions": {
			Key: "IfDirections",
			Getter: func(elt entities.InfoElementWithValue) any {
				var dirs []int
				for _, dir := range strings.Split(elt.GetStringValue(), ",") {
					d, _ := strconv.Atoi(dir)
					dirs = append(dirs, d)
				}
				return dirs
			},
			Setter: func(elt entities.InfoElementWithValue, rec any) {
				if dirs, ok := rec.([]int); ok && len(dirs) > 0 {
					var asStr []string
					for _, dir := range dirs {
						asStr = append(asStr, strconv.Itoa(dir))
					}
					elt.SetStringValue(strings.Join(asStr, ","))
				}
			},
		},
		"protocolIdentifier": {
			Key:    "Proto",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned8Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned8Value(rec.(uint8)) },
		},
		"sourceTransportPort": {
			Key:    "SrcPort",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned16Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned16Value(rec.(uint16)) },
		},
		"destinationTransportPort": {
			Key:    "DstPort",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned16Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned16Value(rec.(uint16)) },
		},
		"octetDeltaCount": {
			Key:    "Bytes",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned64Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned64Value(rec.(uint64)) },
		},
		"flowStartMilliseconds": {
			Key:    "TimeFlowStartMs",
			Getter: func(elt entities.InfoElementWithValue) any { return int64(elt.GetUnsigned64Value()) },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned64Value(uint64(rec.(int64))) },
		},
		"flowEndMilliseconds": {
			Key:    "TimeFlowEndMs",
			Getter: func(elt entities.InfoElementWithValue) any { return int64(elt.GetUnsigned64Value()) },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned64Value(uint64(rec.(int64))) },
		},
		"packetDeltaCount": {
			Key:    "Packets",
			Getter: func(elt entities.InfoElementWithValue) any { return uint32(elt.GetUnsigned64Value()) },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned64Value(uint64(rec.(uint32))) },
		},
		"interfaceName": {
			Key: "Interfaces",
			Setter: func(elt entities.InfoElementWithValue, rec any) {
				if ifs, ok := rec.([]string); ok && len(ifs) > 0 {
					elt.SetStringValue(ifs[0])
				}
			},
			Matcher: func(elt entities.InfoElementWithValue, expected any) bool {
				ifs := expected.([]string)
				return elt.GetStringValue() == ifs[0]
			},
		},
		"tcpControlBits": {
			Key:    "Flags",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned16Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned16Value(rec.(uint16)) },
		},
		"icmpTypeIPv4": {
			Key:    "IcmpType",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned8Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned8Value(rec.(uint8)) },
		},
		"icmpCodeIPv4": {
			Key:    "IcmpCode",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned8Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned8Value(rec.(uint8)) },
		},
		"icmpTypeIPv6": {
			Key:    "IcmpType",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned8Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned8Value(rec.(uint8)) },
		},
		"icmpCodeIPv6": {
			Key:    "IcmpCode",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetUnsigned8Value() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned8Value(rec.(uint8)) },
		},
		"interfaces": {
			Key:    "Interfaces",
			Getter: func(elt entities.InfoElementWithValue) any { return strings.Split(elt.GetStringValue(), ",") },
			Setter: func(elt entities.InfoElementWithValue, rec any) {
				if ifs, ok := rec.([]string); ok {
					elt.SetStringValue(strings.Join(ifs, ","))
				}
			},
		},
		"sourcePodNamespace": {
			Key:    "SrcK8S_Namespace",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetStringValue() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetStringValue(rec.(string)) },
		},
		"sourcePodName": {
			Key:    "SrcK8S_Name",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetStringValue() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetStringValue(rec.(string)) },
		},
		"destinationPodNamespace": {
			Key:    "DstK8S_Namespace",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetStringValue() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetStringValue(rec.(string)) },
		},
		"destinationPodName": {
			Key:    "DstK8S_Name",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetStringValue() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetStringValue(rec.(string)) },
		},
		"sourceNodeName": {
			Key:    "SrcK8S_HostName",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetStringValue() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetStringValue(rec.(string)) },
		},
		"destinationNodeName": {
			Key:    "DstK8S_HostName",
			Getter: func(elt entities.InfoElementWithValue) any { return elt.GetStringValue() },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetStringValue(rec.(string)) },
		},
		"timeFlowRttNs": {
			Key:    "TimeFlowRttNs",
			Getter: func(elt entities.InfoElementWithValue) any { return int64(elt.GetUnsigned64Value()) },
			Setter: func(elt entities.InfoElementWithValue, rec any) { elt.SetUnsigned64Value(uint64(rec.(int64))) },
		},
	}
)

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

func addNetworkEnrichmentToTemplate(elements *[]entities.InfoElementWithValue, registryID uint32) error {
	for _, field := range CustomNetworkFields {
		if err := addElementToTemplate(field, nil, elements, registryID); err != nil {
			return err
		}
	}
	return nil
}

func addKubeContextToTemplate(elements *[]entities.InfoElementWithValue, registryID uint32) error {
	for _, field := range KubeFields {
		if err := addElementToTemplate(field, nil, elements, registryID); err != nil {
			return err
		}
	}
	return nil
}

func loadCustomRegistry(enterpriseID uint32) error {
	err := registry.InitNewRegistry(enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to initialize registry")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("sourcePodNamespace", 7733, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("sourcePodName", 7734, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("destinationPodNamespace", 7735, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("destinationPodName", 7736, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("sourceNodeName", 7737, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("destinationNodeName", 7738, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("timeFlowRttNs", 7740, entities.Unsigned64, enterpriseID, 8)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("interfaces", 7741, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	err = registry.PutInfoElement((*entities.NewInfoElement("directions", 7742, entities.String, enterpriseID, 65535)), enterpriseID)
	if err != nil {
		ilog.WithError(err).Errorf("Failed to register element")
		return err
	}
	return nil
}

func prepareTemplate(templateID uint16, enrichEnterpriseID uint32, fields []string) (entities.Set, []entities.InfoElementWithValue, error) {
	templateSet := entities.NewSet(false)
	err := templateSet.PrepareSet(entities.Template, templateID)
	if err != nil {
		ilog.WithError(err).Error("prepareTemplate: failed to prepare set")
		return nil, nil, err
	}
	elements := make([]entities.InfoElementWithValue, 0)

	for _, field := range fields {
		err = addElementToTemplate(field, nil, &elements, registry.IANAEnterpriseID)
		if err != nil {
			return nil, nil, err
		}
	}
	if enrichEnterpriseID != 0 {
		err = addKubeContextToTemplate(&elements, enrichEnterpriseID)
		if err != nil {
			return nil, nil, err
		}
		err = addNetworkEnrichmentToTemplate(&elements, enrichEnterpriseID)
		if err != nil {
			return nil, nil, err
		}
	}
	err = templateSet.AddRecord(elements, templateID)
	if err != nil {
		ilog.WithError(err).Error("prepareTemplate: failed to add record")
		return nil, nil, err
	}

	return templateSet, elements, nil
}

func setElementValue(record config.GenericMap, ieValPtr *entities.InfoElementWithValue) error {
	ieVal := *ieValPtr
	name := ieVal.GetName()
	mapping, ok := MapIPFIXKeys[name]
	if !ok {
		return nil
	}
	if value := record[mapping.Key]; value != nil {
		mapping.Setter(ieVal, value)
	}
	return nil
}

func setEntities(record config.GenericMap, elements *[]entities.InfoElementWithValue) error {
	for _, ieVal := range *elements {
		err := setElementValue(record, &ieVal)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *writeIpfix) sendDataRecord(record config.GenericMap, v6 bool) error {
	dataSet := entities.NewSet(false)
	var templateID uint16
	if v6 {
		templateID = t.templateIDv6
		err := setEntities(record, &t.entitiesV6)
		if err != nil {
			return err
		}
	} else {
		templateID = t.templateIDv4
		err := setEntities(record, &t.entitiesV4)
		if err != nil {
			return err
		}
	}
	err := dataSet.PrepareSet(entities.Data, templateID)
	if err != nil {
		return err
	}
	if v6 {
		err = dataSet.AddRecord(t.entitiesV6, templateID)
		if err != nil {
			return err
		}
	} else {
		err = dataSet.AddRecord(t.entitiesV4, templateID)
		if err != nil {
			return err
		}
	}
	_, err = t.exporter.SendSet(dataSet)
	if err != nil {
		return err
	}
	return nil
}

// Write writes a flow before being stored
func (t *writeIpfix) Write(entry config.GenericMap) {
	ilog.Tracef("entering writeIpfix Write")
	if IPv6Type == entry["Etype"].(uint16) {
		err := t.sendDataRecord(entry, true)
		if err != nil {
			ilog.WithError(err).Error("Failed in send v6 IPFIX record")
		}
	} else {
		err := t.sendDataRecord(entry, false)
		if err != nil {
			ilog.WithError(err).Error("Failed in send v4 IPFIX record")
		}
	}
}

func (t *writeIpfix) startTemplateSenderLoop(interval time.Duration, exitChan <-chan struct{}) {
	// First send sync
	if _, err := t.exporter.SendSet(t.tplV4); err != nil {
		ilog.WithError(err).Error("Failed to send template V4")
	}
	if _, err := t.exporter.SendSet(t.tplV6); err != nil {
		ilog.WithError(err).Error("Failed to send template V6")
	}
	// Periodic sending async
	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-exitChan:
				log.Debugf("exiting sendTemplates because of signal")
				return
			case <-ticker.C:
				if _, err := t.exporter.SendSet(t.tplV4); err != nil {
					ilog.WithError(err).Error("Failed to send template V4")
				}
				if _, err := t.exporter.SendSet(t.tplV6); err != nil {
					ilog.WithError(err).Error("Failed to send template V6")
				}
			}
		}
	}()
}

// NewWriteIpfix creates a new write
func NewWriteIpfix(params config.StageParam) (Writer, error) {
	ilog.Debugf("entering NewWriteIpfix")

	ipfixConfigIn := api.WriteIpfix{}
	if params.Write != nil && params.Write.Ipfix != nil {
		ipfixConfigIn = *params.Write.Ipfix
	}
	// need to combine defaults with parameters that are provided in the config yaml file
	ipfixConfigIn.SetDefaults()

	if err := ipfixConfigIn.Validate(); err != nil {
		return nil, fmt.Errorf("the provided config is not valid: %w", err)
	}

	// Create exporter using local server info
	input := ipfixExporter.ExporterInput{
		CollectorAddress:    fmt.Sprintf("%s:%d", ipfixConfigIn.TargetHost, ipfixConfigIn.TargetPort),
		CollectorProtocol:   ipfixConfigIn.Transport,
		ObservationDomainID: 1,
		TempRefTimeout:      1,
	}

	exporter, err := ipfixExporter.InitExportingProcess(input)
	if err != nil {
		return nil, fmt.Errorf("error when connecting to IPFIX collector %s: %w", input.CollectorAddress, err)
	}
	ilog.Infof("Created IPFIX exporter connecting to server with address: %s", input.CollectorAddress)

	eeid := uint32(ipfixConfigIn.EnterpriseID)

	registry.LoadRegistry()
	if eeid != 0 {
		if err := loadCustomRegistry(eeid); err != nil {
			return nil, fmt.Errorf("failed to load custom registry with EnterpriseID=%d: %w", eeid, err)
		}
	}

	idV4 := exporter.NewTemplateID()
	setV4, entitiesV4, err := prepareTemplate(idV4, eeid, IPv4IANAFields)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare IPv4 template: %w", err)
	}

	idV6 := exporter.NewTemplateID()
	setV6, entitiesV6, err := prepareTemplate(idV6, eeid, IPv6IANAFields)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare IPv6 template: %w", err)
	}

	writeIpfix := &writeIpfix{
		exporter:     exporter,
		templateIDv4: idV4,
		tplV4:        setV4,
		entitiesV4:   entitiesV4,
		templateIDv6: idV6,
		tplV6:        setV6,
		entitiesV6:   entitiesV6,
	}

	writeIpfix.startTemplateSenderLoop(ipfixConfigIn.TplSendInterval.Duration, utils.ExitChannel())

	return writeIpfix, nil
}
