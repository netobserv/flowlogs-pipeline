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
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

func (cg *ConfGen) generateFlowlogs2MetricsConfig(fileName string) error {
	config := map[string]interface{}{
		"log-level": "debug", // TODO: remove this temporal debug statement
		"pipeline": map[string]interface{}{
			"ingest": map[string]interface{}{
				"type": "collector",
				"collector": map[string]interface{}{
					"port":     cg.config.Ingest.Collector.Port,
					"hostname": cg.config.Ingest.Collector.HostName,
				},
			},
			"decode": map[string]interface{}{
				"type": "json",
			},
			"transform": []interface{}{
				map[string]interface{}{
					"type": "generic",
					"generic": map[string]interface{}{
						"rules": cg.config.Transform.Generic.Rules,
					},
				},
				map[string]interface{}{
					"type": "network",
					"network": map[string]interface{}{
						"rules": cg.transformRules,
					},
				},
			},
			"extract": map[string]interface{}{
				"type":       "aggregates",
				"aggregates": cg.aggregateDefinitions,
			},
			"encode": map[string]interface{}{
				"type": "prom",
				"prom": map[string]interface{}{
					"port":    cg.config.Encode.Prom.Port,
					"prefix":  cg.config.Encode.Prom.Prefix,
					"metrics": cg.promMetrics,
				},
			},
			"write": map[string]interface{}{
				"type": "none",
			},
		},
	}

	configData, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	header := "# This file was generated automatically by flowlogs2metrics confgenerator"
	data := fmt.Sprintf("%s\n%s\n", header, configData)
	err = ioutil.WriteFile(fileName, []byte(data), 0664)
	if err != nil {
		return err
	}

	return nil
}
