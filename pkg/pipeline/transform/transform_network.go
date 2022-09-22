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

package transform

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"

	"github.com/Knetic/govaluate"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	netdb "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/netdb"
	log "github.com/sirupsen/logrus"
)

type Network struct {
	api.TransformNetwork
	svcNames *netdb.ServiceNames
}

func (n *Network) Transform(input []config.GenericMap) []config.GenericMap {
	outputEntries := make([]config.GenericMap, 0)
	for _, entry := range input {
		outputEntry := n.TransformEntry(entry)
		outputEntries = append(outputEntries, outputEntry)
	}
	return outputEntries
}

func (n *Network) TransformEntry(inputEntry config.GenericMap) config.GenericMap {
	// copy input entry before transform to avoid alteration on parallel stages
	outputEntry := inputEntry.Copy()

	// TODO: for efficiency and maintainability, maybe each case in the switch below should be an individual implementation of Transformer
	for _, rule := range n.Rules {
		switch rule.Type {
		case api.TransformNetworkOperationName("AddRegExIf"):
			matched, err := regexp.MatchString(rule.Parameters, fmt.Sprintf("%s", outputEntry[rule.Input]))
			if err != nil {
				continue
			}
			if matched {
				outputEntry[rule.Output] = outputEntry[rule.Input]
				outputEntry[rule.Output+"_Matched"] = true
			}
		case api.TransformNetworkOperationName("AddIf"):
			expressionString := fmt.Sprintf("val %s", rule.Parameters)
			expression, err := govaluate.NewEvaluableExpression(expressionString)
			if err != nil {
				log.Errorf("Can't evaluate AddIf rule: %+v expression: %v. err %v", rule, expressionString, err)
				continue
			}
			result, evaluateErr := expression.Evaluate(map[string]interface{}{"val": outputEntry[rule.Input]})
			if evaluateErr == nil && result.(bool) {
				if rule.Assignee != "" {
					outputEntry[rule.Output] = rule.Assignee
				} else {
					outputEntry[rule.Output] = outputEntry[rule.Input]
				}
				outputEntry[rule.Output+"_Evaluate"] = true
			}
		case api.TransformNetworkOperationName("AddSubnet"):
			_, ipv4Net, err := net.ParseCIDR(fmt.Sprintf("%v%s", outputEntry[rule.Input], rule.Parameters))
			if err != nil {
				log.Errorf("Can't find subnet for IP %v and prefix length %s - err %v", outputEntry[rule.Input], rule.Parameters, err)
				continue
			}
			outputEntry[rule.Output] = ipv4Net.String()
		case api.TransformNetworkOperationName("AddLocation"):
			var locationInfo *location.Info
			err, locationInfo := location.GetLocation(fmt.Sprintf("%s", outputEntry[rule.Input]))
			if err != nil {
				log.Errorf("Can't find location for IP %v err %v", outputEntry[rule.Input], err)
				continue
			}
			outputEntry[rule.Output+"_CountryName"] = locationInfo.CountryName
			outputEntry[rule.Output+"_CountryLongName"] = locationInfo.CountryLongName
			outputEntry[rule.Output+"_RegionName"] = locationInfo.RegionName
			outputEntry[rule.Output+"_CityName"] = locationInfo.CityName
			outputEntry[rule.Output+"_Latitude"] = locationInfo.Latitude
			outputEntry[rule.Output+"_Longitude"] = locationInfo.Longitude
		case api.TransformNetworkOperationName("AddService"):
			protocol := fmt.Sprintf("%v", outputEntry[rule.Parameters])
			portNumber, err := strconv.Atoi(fmt.Sprintf("%v", outputEntry[rule.Input]))
			if err != nil {
				log.Errorf("Can't convert port to int: Port %v - err %v", outputEntry[rule.Input], err)
				continue
			}
			var serviceName string
			protocolAsNumber, err := strconv.Atoi(protocol)
			if err == nil {
				// protocol has been submitted as number
				serviceName = n.svcNames.ByPortAndProtocolNumber(portNumber, protocolAsNumber)
			} else {
				// protocol has been submitted as any string
				serviceName = n.svcNames.ByPortAndProtocolName(portNumber, protocol)
			}
			if serviceName == "" {
				if err != nil {
					log.Debugf("Can't find service name for Port %v and protocol %v - err %v", outputEntry[rule.Input], protocol, err)
					continue
				}
			}
			outputEntry[rule.Output] = serviceName
		case api.TransformNetworkOperationName("AddKubernetes"):
			kubeInfo, err := kubernetes.Data.GetInfo(fmt.Sprintf("%s", outputEntry[rule.Input]))
			if err != nil {
				log.Debugf("Can't find kubernetes info for IP %v err %v", outputEntry[rule.Input], err)
				continue
			}
			outputEntry[rule.Output+"_Namespace"] = kubeInfo.Namespace
			outputEntry[rule.Output+"_Name"] = kubeInfo.Name
			outputEntry[rule.Output+"_Type"] = kubeInfo.Type
			outputEntry[rule.Output+"_OwnerName"] = kubeInfo.Owner.Name
			outputEntry[rule.Output+"_OwnerType"] = kubeInfo.Owner.Type
			if rule.Parameters != "" {
				for labelKey, labelValue := range kubeInfo.Labels {
					outputEntry[rule.Parameters+"_"+labelKey] = labelValue
				}
			}
			if kubeInfo.HostIP != "" {
				outputEntry[rule.Output+"_HostIP"] = kubeInfo.HostIP
				if kubeInfo.HostName != "" {
					outputEntry[rule.Output+"_HostName"] = kubeInfo.HostName
				}
			}
		default:
			log.Panicf("unknown type %s for transform.Network rule: %v", rule.Type, rule)
		}
	}

	return outputEntry
}

// NewTransformNetwork create a new transform
func NewTransformNetwork(params config.StageParam) (Transformer, error) {
	var needToInitLocationDB = false
	var needToInitKubeData = false
	var needToInitNetworkServices = false

	jsonNetworkTransform := api.TransformNetwork{}
	if params.Transform != nil && params.Transform.Network != nil {
		jsonNetworkTransform = *params.Transform.Network
	}
	for _, rule := range jsonNetworkTransform.Rules {
		switch rule.Type {
		case api.TransformNetworkOperationName("AddLocation"):
			needToInitLocationDB = true
		case api.TransformNetworkOperationName("AddKubernetes"):
			needToInitKubeData = true
		case api.TransformNetworkOperationName("AddService"):
			needToInitNetworkServices = true
		}
	}

	if needToInitLocationDB {
		err := location.InitLocationDB()
		if err != nil {
			log.Debugf("location.InitLocationDB error: %v", err)
		}
	}

	if needToInitKubeData {
		err := kubernetes.Data.InitFromConfig(jsonNetworkTransform.KubeConfigPath)
		if err != nil {
			return nil, err
		}
	}

	var servicesDB *netdb.ServiceNames
	if needToInitNetworkServices {
		pFilename, sFilename := jsonNetworkTransform.GetServiceFiles()
		var err error
		protos, err := os.Open(pFilename)
		if err != nil {
			return nil, fmt.Errorf("opening protocols file %q: %w", pFilename, err)
		}
		defer protos.Close()
		services, err := os.Open(sFilename)
		if err != nil {
			return nil, fmt.Errorf("opening services file %q: %w", sFilename, err)
		}
		defer services.Close()
		servicesDB, err = netdb.LoadServicesDB(protos, services)
		if err != nil {
			return nil, err
		}
	}

	return &Network{
		TransformNetwork: api.TransformNetwork{
			Rules: jsonNetworkTransform.Rules,
		},
		svcNames: servicesDB,
	}, nil
}
