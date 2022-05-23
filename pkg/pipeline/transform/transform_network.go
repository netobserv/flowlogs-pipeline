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
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"text/template"

	"github.com/Knetic/govaluate"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/connection_tracking"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	netdb "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/network_services"
	log "github.com/sirupsen/logrus"
)

type Network struct {
	enumCache    *api.EnumNamesCache
	networkRules api.TransformNetwork
}

func (n *Network) Transform(inputEntries []config.GenericMap) []config.GenericMap {
	outputEntries := make([]config.GenericMap, 0)
	for _, entry := range inputEntries {
		outputEntry := n.TransformEntry(entry)
		outputEntries = append(outputEntries, outputEntry)
	}
	return outputEntries
}

func (n *Network) TransformEntry(inputEntry config.GenericMap) config.GenericMap {
	outputEntries := inputEntry

	for _, rule := range n.networkRules.Rules {
		switch rule.Type {
		case api.TransformNetworkOperationName(n.enumCache, "ConnTracking"):
			template, err := template.New("").Parse(rule.Input)
			if err != nil {
				panic(err)
			}
			buf := &bytes.Buffer{}
			err = template.Execute(buf, outputEntries)
			if err != nil {
				panic(err)
			}
			FlowIDFieldsAsString := buf.String()
			isNew := connection_tracking.CT.AddFlow(FlowIDFieldsAsString)
			if isNew {
				if rule.Parameters != "" {
					outputEntries[rule.Output] = rule.Parameters
				} else {
					outputEntries[rule.Output] = true
				}
			}

		case api.TransformNetworkOperationName(n.enumCache, "AddRegExIf"):
			matched, err := regexp.MatchString(rule.Parameters, fmt.Sprintf("%s", outputEntries[rule.Input]))
			if err != nil {
				continue
			}
			if matched {
				outputEntries[rule.Output] = outputEntries[rule.Input]
				outputEntries[rule.Output+"_Matched"] = true
			}
		case api.TransformNetworkOperationName(n.enumCache, "AddIf"):
			expressionString := fmt.Sprintf("val %s", rule.Parameters)
			expression, err := govaluate.NewEvaluableExpression(expressionString)
			if err != nil {
				log.Errorf("Can't evaluate AddIf rule: %+v expression: %v. err %v", rule, expressionString, err)
				continue
			}
			result, evaluateErr := expression.Evaluate(map[string]interface{}{"val": outputEntries[rule.Input]})
			if evaluateErr == nil && result.(bool) {
				outputEntries[rule.Output] = outputEntries[rule.Input]
				outputEntries[rule.Output+"_Evaluate"] = true
			}
		case api.TransformNetworkOperationName(n.enumCache, "AddSubnet"):
			_, ipv4Net, err := net.ParseCIDR(fmt.Sprintf("%v%s", outputEntries[rule.Input], rule.Parameters))
			if err != nil {
				log.Errorf("Can't find subnet for IP %v and prefix length %s - err %v", outputEntries[rule.Input], rule.Parameters, err)
				continue
			}
			outputEntries[rule.Output] = ipv4Net.String()
		case api.TransformNetworkOperationName(n.enumCache, "AddLocation"):
			var locationInfo *location.Info
			err, locationInfo := location.GetLocation(fmt.Sprintf("%s", outputEntries[rule.Input]))
			if err != nil {
				log.Errorf("Can't find location for IP %v err %v", outputEntries[rule.Input], err)
				continue
			}
			outputEntries[rule.Output+"_CountryName"] = locationInfo.CountryName
			outputEntries[rule.Output+"_CountryLongName"] = locationInfo.CountryLongName
			outputEntries[rule.Output+"_RegionName"] = locationInfo.RegionName
			outputEntries[rule.Output+"_CityName"] = locationInfo.CityName
			outputEntries[rule.Output+"_Latitude"] = locationInfo.Latitude
			outputEntries[rule.Output+"_Longitude"] = locationInfo.Longitude
		case api.TransformNetworkOperationName(n.enumCache, "AddService"):
			protocol := fmt.Sprintf("%v", outputEntries[rule.Parameters])
			portNumber, err := strconv.Atoi(fmt.Sprintf("%v", outputEntries[rule.Input]))
			if err != nil {
				log.Errorf("Can't convert port to int: Port %v - err %v", outputEntries[rule.Input], err)
				continue
			}
			service := netdb.GetServByPort(portNumber, netdb.GetProtoByName(protocol))
			if service == nil {
				protocolAsNumber, err := strconv.Atoi(fmt.Sprintf("%v", protocol))
				if err != nil {
					log.Infof("Can't find service name for Port %v and protocol %v - err %v", outputEntries[rule.Input], protocol, err)
					continue
				}
				service = netdb.GetServByPort(portNumber, netdb.GetProtoByNumber(protocolAsNumber))
				if service == nil {
					log.Infof("Can't find service name for Port %v and protocol %v - err %v", outputEntries[rule.Input], protocol, err)
					continue
				}
			}
			outputEntries[rule.Output] = service.Name
		case api.TransformNetworkOperationName(n.enumCache, "AddKubernetes"):
			var kubeInfo *kubernetes.Info
			kubeInfo, err := kubernetes.Data.GetInfo(fmt.Sprintf("%s", outputEntries[rule.Input]))
			if err != nil {
				log.Infof("Can't find kubernetes info for IP %v err %v", outputEntries[rule.Input], err)
				continue
			}
			outputEntries[rule.Output+"_Namespace"] = kubeInfo.Namespace
			outputEntries[rule.Output+"_Name"] = kubeInfo.Name
			outputEntries[rule.Output+"_Type"] = kubeInfo.Type
			outputEntries[rule.Output+"_OwnerName"] = kubeInfo.Owner.Name
			outputEntries[rule.Output+"_OwnerType"] = kubeInfo.Owner.Type
			if rule.Parameters != "" {
				for labelKey, labelValue := range kubeInfo.Labels {
					outputEntries[rule.Parameters+"_"+labelKey] = labelValue
				}
			}
			if kubeInfo.HostIP != "" {
				outputEntries[rule.Output+"_HostIP"] = kubeInfo.HostIP
				if kubeInfo.HostName != "" {
					outputEntries[rule.Output+"_HostName"] = kubeInfo.HostName
				}
			}
		default:
			log.Panicf("unknown type %s for transform.Network rule: %v", rule.Type, rule)
		}
	}

	return outputEntries
}

// NewTransformNetwork create a new transform
func NewTransformNetwork(params config.StageParam) (Transformer, error) {
	var needToInitLocationDB = false
	var needToInitKubeData = false
	var needToInitConnectionTracking = false
	var needToInitNetworkServices = false

	enumCache := api.InitEnumCache(7)

	jsonNetworkTransform := params.Transform.Network
	for _, rule := range jsonNetworkTransform.Rules {
		switch rule.Type {
		case api.TransformNetworkOperationName(enumCache, "AddLocation"):
			needToInitLocationDB = true
		case api.TransformNetworkOperationName(enumCache, "AddKubernetes"):
			needToInitKubeData = true
		case api.TransformNetworkOperationName(enumCache, "ConnTracking"):
			needToInitConnectionTracking = true
		case api.TransformNetworkOperationName(enumCache, "AddService"):
			needToInitNetworkServices = true
		}
	}

	if needToInitConnectionTracking {
		connection_tracking.InitConnectionTracking()
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

	if needToInitNetworkServices {
		err := netdb.Init(jsonNetworkTransform.ProtocolsFile, jsonNetworkTransform.ServicesFile)
		if err != nil {
			return nil, err
		}
	}

	return &Network{
		enumCache: enumCache,
		networkRules: api.TransformNetwork{
			Rules: jsonNetworkTransform.Rules,
		},
	}, nil
}
