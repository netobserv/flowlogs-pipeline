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
	"os"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/location"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/netdb"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/network"
	"github.com/sirupsen/logrus"
)

var tlog = logrus.WithField("component", "network.Transformer")

type Network struct {
	api.TransformNetwork
	transformers []network.Transformer
}

func (n *Network) Transform(inputEntry config.GenericMap) (config.GenericMap, bool) {
	// copy input entry before transform to avoid alteration on parallel stages
	outputEntry := inputEntry.Copy()

	for _, t := range n.transformers {
		if err := t.Transform(outputEntry); err != nil {
			tlog.WithError(err).Debug("error applying transformation. Ignoring")
		}
	}
	return outputEntry, true
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
			tlog.Debugf("location.InitLocationDB error: %v", err)
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

	transformers, err := transformerFromRules(jsonNetworkTransform.Rules, servicesDB)
	if err != nil {
		return nil, err
	}

	return &Network{
		TransformNetwork: api.TransformNetwork{
			Rules: jsonNetworkTransform.Rules,
		},
		transformers: transformers,
	}, nil
}

func transformerFromRules(
	rules api.NetworkTransformRules, servicesDB *netdb.ServiceNames,
) ([]network.Transformer, error) {
	var trans []network.Transformer
	for _, rule := range rules {
		switch rule.Type {
		case api.OpAddRegexIf:
			ari := network.AddRegexpIf(rule)
			trans = append(trans, &ari)
		case api.OpAddIf:
			ai := network.AddIf(rule)
			trans = append(trans, &ai)
		case api.OpAddSubnet:
			as := network.AddSubnet(rule)
			trans = append(trans, &as)
		case api.OpAddLocation:
			al := network.AddLocation(rule)
			trans = append(trans, &al)
		case api.OpAddService:
			as := network.AddService{
				NetworkTransformRule: rule,
				SvcNames:             servicesDB,
			}
			trans = append(trans, &as)
		case api.OpAddKubernetes:
			as := network.AddKubernetes(rule)
			trans = append(trans, &as)
		default:
			return nil, fmt.Errorf("unknown type %s for transform.Network rule: %v", rule.Type, rule)
		}
	}
	return trans, nil
}
