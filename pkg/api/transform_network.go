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

import "github.com/mariomac/pipes/pkg/graph/stage"

type TransformNetwork struct {
	stage.Instance
	Rules          NetworkTransformRules `yaml:"rules" doc:"list of transform rules, each includes:"`
	KubeConfigPath string                `yaml:"kubeconfigpath" doc:"path to kubeconfig file (optional)"`
	ServicesFile   string                `yaml:"servicesfile" doc:"path to services file (optional, default: /etc/services)"`
	ProtocolsFile  string                `yaml:"protocolsfile" doc:"path to protocols file (optional, default: /etc/protocols)"`
}

type TransformNetworkOperationEnum struct {
	ConnTracking  string `yaml:"conn_tracking" doc:"set output field to value of parameters field only for new flows by matching template in input field"`
	AddRegExIf    string `yaml:"add_regex_if" doc:"add output field if input field satisfies regex pattern from parameters field"`
	AddIf         string `yaml:"add_if" doc:"add output field if input field satisfies criteria from parameters field"`
	AddSubnet     string `yaml:"add_subnet" doc:"add output subnet field from input field and prefix length from parameters field"`
	AddLocation   string `yaml:"add_location" doc:"add output location fields from input"`
	AddService    string `yaml:"add_service" doc:"add output network service field from input port and parameters protocol field"`
	AddKubernetes string `yaml:"add_kubernetes" doc:"add output kubernetes fields from input"`
}

func TransformNetworkOperationName(operation string) string {
	return GetEnumName(TransformNetworkOperationEnum{}, operation)
}

type NetworkTransformRule struct {
	Input      string `yaml:"input" doc:"entry input field"`
	Output     string `yaml:"output" doc:"entry output field"`
	Type       string `yaml:"type" enum:"TransformNetworkOperationEnum" doc:"one of the following:"`
	Parameters string `yaml:"parameters" doc:"parameters specific to type"`
}

type NetworkTransformRules []NetworkTransformRule
