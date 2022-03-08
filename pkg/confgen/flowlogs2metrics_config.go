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

	"gopkg.in/yaml.v2"
)

func (cg *ConfGen) generateFlowlogs2PipelineConfig(fileName string) error {
	config := map[string]interface{}{
		"log-level": "error",
		"pipeline": []map[string]string{
			{"name": "ingest_collector"},
			{"name": "decode_json",
				"follows": "ingest_collector",
			},
			{"name": "transform_generic",
				"follows": "decode_json",
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
			{"name": "write_none",
				"follows": "encode_prom",
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
						"port":     cg.config.Ingest.Collector.Port,
						"hostname": cg.config.Ingest.Collector.HostName,
					},
				},
			},
			{"name": "decode_json",
				"decode": map[string]interface{}{
					"type": "json",
				},
			},
			{"name": "transform_generic",
				"transform": map[string]interface{}{
					"type": "generic",
					"generic": map[string]interface{}{
						"rules": cg.config.Transform.Generic.Rules,
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
			{"name": "write_none",
				"write": map[string]interface{}{
					"type": "none",
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
