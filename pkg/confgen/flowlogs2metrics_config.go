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
			{"name": "ingest1"},
			{"name": "decode1",
				"follows": "ingest1",
			},
			{"name": "transform1",
				"follows": "decode1",
			},
			{"name": "transform2",
				"follows": "transform1",
			},
			{"name": "extract1",
				"follows": "transform2",
			},
			{"name": "encode1",
				"follows": "extract1",
			},
			{"name": "write1",
				"follows": "transform2",
			},
		},
		"parameters": []map[string]interface{}{
			{"name": "ingest1",
				"ingest": map[string]interface{}{
					"type": "collector",
					"collector": map[string]interface{}{
						"port":     cg.config.Ingest.Collector.Port,
						"hostname": cg.config.Ingest.Collector.HostName,
					},
				},
			},
			{"name": "decode1",
				"decode": map[string]interface{}{
					"type": "json",
				},
			},
			{"name": "transform1",
				"transform": map[string]interface{}{
					"type": "generic",
					"generic": map[string]interface{}{
						"rules": cg.config.Transform.Generic.Rules,
					},
				},
			},
			{"name": "transform2",
				"transform": map[string]interface{}{
					"type": "network",
					"network": map[string]interface{}{
						"rules": cg.transformRules,
					},
				},
			},
			{"name": "extract1",
				"extract": map[string]interface{}{
					"type":       "aggregates",
					"aggregates": cg.aggregateDefinitions,
				},
			},
			{"name": "encode1",
				"encode": map[string]interface{}{
					"type": "prom",
					"prom": map[string]interface{}{
						"port":    cg.config.Encode.Prom.Port,
						"prefix":  cg.config.Encode.Prom.Prefix,
						"metrics": cg.promMetrics,
					},
				},
			},
			{"name": "write1",
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
