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

package confgen

import (
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func (cg *ConfGen) GenerateFlowlogs2PipelineConfig() map[string]interface{} {
	config := map[string]interface{}{
		"log-level": "error",
		"pipeline": []map[string]string{
			{"name": "ingest_collector"},
			{"name": "transform_generic",
				"follows": "ingest_collector",
			},
			{"name": "transform_network",
				"follows": "transform_generic",
			},
			{"name": "extract_aggregate",
				"follows": "transform_network",
			},
			{"name": "encode_prom",
				"follows": "extract_aggregate",
			},
			{"name": "write_loki",
				"follows": "transform_network",
			},
		},
		"parameters": []map[string]interface{}{
			{"name": "ingest_collector",
				"ingest": map[string]interface{}{
					"type": "collector",
					"collector": map[string]interface{}{
						"port":       cg.config.Ingest.Collector.Port,
						"portLegacy": cg.config.Ingest.Collector.PortLegacy,
						"hostname":   cg.config.Ingest.Collector.HostName,
					},
				},
			},
			{"name": "transform_generic",
				"transform": map[string]interface{}{
					"type": "generic",
					"generic": map[string]interface{}{
						"policy": "replace_keys",
						"rules":  cg.config.Transform.Generic.Rules,
					},
				},
			},
			{"name": "transform_network",
				"transform": map[string]interface{}{
					"type": "network",
					"network": map[string]interface{}{
						"rules": cg.transformRules,
					},
				},
			},
			{"name": "extract_aggregate",
				"extract": map[string]interface{}{
					"type":       "aggregates",
					"aggregates": cg.aggregateDefinitions,
				},
			},
			{"name": "encode_prom",
				"encode": map[string]interface{}{
					"type": "prom",
					"prom": map[string]interface{}{
						"port":    cg.config.Encode.Prom.Port,
						"prefix":  cg.config.Encode.Prom.Prefix,
						"metrics": cg.promMetrics,
					},
				},
			},
			{"name": "write_loki",
				"write": map[string]interface{}{
					"type": cg.config.Write.Type,
					"loki": cg.config.Write.Loki,
				},
			},
		},
	}
	return config
}

func (cg *ConfGen) GenerateTruncatedConfig(stages []string) map[string]interface{} {
	parameters := make([]map[string]interface{}, len(stages))
	for i, stage := range stages {
		switch stage {
		case "ingest":
			parameters[i] = map[string]interface{}{
				"name": "ingest_collector",
				"ingest": map[string]interface{}{
					"type": "collector",
					"collector": map[string]interface{}{
						"port":       cg.config.Ingest.Collector.Port,
						"portLegacy": cg.config.Ingest.Collector.PortLegacy,
						"hostname":   cg.config.Ingest.Collector.HostName,
					},
				},
			}
		case "transform_generic":
			parameters[i] = map[string]interface{}{
				"name": "transform_generic",
				"transform": map[string]interface{}{
					"type": "generic",
					"generic": map[string]interface{}{
						"policy": "replace_keys",
						"rules":  cg.config.Transform.Generic.Rules,
					},
				},
			}
		case "transform_network":
			parameters[i] = map[string]interface{}{
				"name": "transform_network",
				"transform": map[string]interface{}{
					"type": "network",
					"network": map[string]interface{}{
						"rules": cg.transformRules,
					},
				},
			}
		case "extract_aggregate":
			parameters[i] = map[string]interface{}{
				"name": "extract_aggregate",
				"extract": map[string]interface{}{
					"type":       "aggregates",
					"aggregates": cg.aggregateDefinitions,
				},
			}
		case "encode_prom":
			parameters[i] = map[string]interface{}{
				"name": "encode_prom",
				"encode": map[string]interface{}{
					"type": "prom",
					"prom": map[string]interface{}{
						"port":    cg.config.Encode.Prom.Port,
						"prefix":  cg.config.Encode.Prom.Prefix,
						"metrics": cg.promMetrics,
					},
				},
			}
		case "write_loki":
			parameters[i] = map[string]interface{}{
				"name": "write_loki",
				"write": map[string]interface{}{
					"type": cg.config.Write.Type,
					"loki": cg.config.Write.Loki,
				},
			}
		}
	}
	log.Debugf("parameters = %v \n", parameters)
	config := map[string]interface{}{
		"parameters": parameters,
	}
	return config
}

func (cg *ConfGen) writeConfigFile(fileName string, config map[string]interface{}) error {
	configData, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	header := "# This file was generated automatically by flowlogs-pipeline confgenerator"
	data := fmt.Sprintf("%s\n%s\n", header, configData)
	err = ioutil.WriteFile(fileName, []byte(data), 0664)
	if err != nil {
		return err
	}

	return nil
}
