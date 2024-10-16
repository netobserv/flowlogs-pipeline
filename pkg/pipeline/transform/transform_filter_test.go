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

package transform

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfigTransformFilterRemoveField = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - type: remove_field
          removeField:
            input: dstPort
        - type: remove_field
          removeField:
            input: srcPort
`

const testConfigTransformFilterRemoveEntryIfExists = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - type: remove_entry_if_exists
          removeEntry:
            input: srcPort
          
`

const testConfigTransformFilterRemoveEntryIfDoesntExists = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - type: remove_entry_if_doesnt_exist
          removeEntry:
            input: doesntSrcPort
          
`
const testConfigTransformFilterRemoveEntryIfEqual = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - type: remove_entry_if_equal
          removeEntry:
            input: message
            value: "test message"
        - type: remove_entry_if_equal
          removeEntry:
            input: value          
            value: 8.0
`

const testConfigTransformFilterRemoveEntryIfNotEqual = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - type: remove_entry_if_not_equal
          removeEntry:
            input: message
            value: "test message"
`

const testConfigTransformFilterAddField = `---
log-level: debug
pipeline:
  - name: filter1
parameters:
  - name: filter1
    transform:
      type: filter
      filter:
        rules:
        - type: add_field_if_doesnt_exist
          addFieldIfDoesntExist:
            input: dstPort        
            value: dummy_value
        - type: add_field_if_doesnt_exist
          addFieldIfDoesntExist:
            input: dummy_field
            value: dummy_value
`

func getFilterExpectedOutput() config.GenericMap {
	return config.GenericMap{
		"srcIP":        "10.0.0.1",
		"8888IP":       "8.8.8.8",
		"emptyIP":      "",
		"level":        "error",
		"protocol":     "tcp",
		"protocol_num": 6,
		"value":        7.0,
		"message":      "test message",
		"dstIP":        "20.0.0.2",
	}
}

func TestNewTransformFilterRemoveField(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveField)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 2)

	input := test.GetIngestMockEntry(false)
	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	expectedOutput := getFilterExpectedOutput()
	require.Equal(t, expectedOutput, output)
}

func TestNewTransformFilterRemoveEntryIfExists(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfExists)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)
	_, ok := transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterRemoveEntryIfDoesntExists(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfDoesntExists)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)
	_, ok := transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterRemoveEntryIfEqual(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfEqual)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 2)

	input := test.GetIngestMockEntry(false)

	_, ok := transformFilter.Transform(input)
	require.False(t, ok)

	input["message"] = "dummy message"
	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, output["message"], "dummy message")

	input["value"] = 8.0
	_, ok = transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterRemoveEntryIfNotEqual(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterRemoveEntryIfNotEqual)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 1)

	input := test.GetIngestMockEntry(false)

	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, output["message"], "test message")

	input["message"] = "dummy message"
	_, ok = transformFilter.Transform(input)
	require.False(t, ok)
}

func TestNewTransformFilterAddField(t *testing.T) {
	newTransform := InitNewTransformFilter(t, testConfigTransformFilterAddField)
	transformFilter := newTransform.(*Filter)
	require.Len(t, transformFilter.Rules, 2)

	input := test.GetIngestMockEntry(false)
	output, ok := transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, 22, output["dstPort"])
	require.Equal(t, "dummy_value", output["dummy_field"])

	input = test.GetIngestMockEntry(false)
	input["dstPort"] = 3490
	input["dummy_field"] = 1
	output, ok = transformFilter.Transform(input)
	require.True(t, ok)
	require.Equal(t, 3490, output["dstPort"])
	require.Equal(t, 1, output["dummy_field"])
}

func InitNewTransformFilter(t *testing.T, configFile string) Transformer {
	v, cfg := test.InitConfig(t, configFile)
	require.NotNil(t, v)

	config := cfg.Parameters[0]
	newTransform, err := NewTransformFilter(config)
	require.NoError(t, err)
	return newTransform
}

func Test_Transform_AddIfScientificNotation(t *testing.T) {
	newFilter := Filter{
		Rules: []api.TransformFilterRule{
			{
				Type: "add_field_if",
				AddFieldIf: &api.TransformFilterRuleWithAssignee{
					Input:      "value",
					Output:     "bigger_than_10",
					Parameters: ">10",
				},
			},
			{
				Type: "add_field_if",
				AddFieldIf: &api.TransformFilterRuleWithAssignee{
					Input:      "value",
					Output:     "smaller_than_10",
					Parameters: "<10",
				},
			},
			{
				Type: "add_field_if",
				AddFieldIf: &api.TransformFilterRuleWithAssignee{
					Input:      "value",
					Output:     "dir",
					Assignee:   "in",
					Parameters: "==1",
				},
			},
			{
				Type: "add_field_if",
				AddFieldIf: &api.TransformFilterRuleWithAssignee{
					Input:      "value",
					Output:     "dir",
					Assignee:   "out",
					Parameters: "==0",
				},
			},
			{
				Type: "add_field_if",
				AddFieldIf: &api.TransformFilterRuleWithAssignee{
					Input:      "value",
					Output:     "not_one",
					Assignee:   "true",
					Parameters: "!=1",
				},
			},
			{
				Type: "add_field_if",
				AddFieldIf: &api.TransformFilterRuleWithAssignee{
					Input:      "value",
					Output:     "not_one",
					Assignee:   "false",
					Parameters: "==1",
				},
			},
		},
	}

	var entry config.GenericMap
	entry = config.GenericMap{
		"value": 1.2345e67,
	}
	output, ok := newFilter.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["bigger_than_10_Evaluate"])
	require.Equal(t, 1.2345e67, output["bigger_than_10"])
	require.Equal(t, "true", output["not_one"])

	entry = config.GenericMap{
		"value": 1.2345e-67,
	}
	output, ok = newFilter.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["smaller_than_10_Evaluate"])
	require.Equal(t, 1.2345e-67, output["smaller_than_10"])
	require.Equal(t, "true", output["not_one"])

	entry = config.GenericMap{
		"value": 1,
	}
	output, ok = newFilter.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["dir_Evaluate"])
	require.Equal(t, "in", output["dir"])
	require.Equal(t, "false", output["not_one"])

	entry = config.GenericMap{
		"value": 0,
	}
	output, ok = newFilter.Transform(entry)
	require.True(t, ok)
	require.Equal(t, true, output["dir_Evaluate"])
	require.Equal(t, "out", output["dir"])
	require.Equal(t, "true", output["not_one"])
}

func Test_TransformFilterDependentRulesAddRegExIf(t *testing.T) {
	var yamlConfig = []byte(`
log-level: debug
pipeline:
  - name: transform1
  - name: write1
    follows: transform1
parameters:
  - name: transform1
    transform:
      type: filter
      filter:
        rules:
        - type: add_field_if_doesnt_exist
          addFieldIfDoesntExist:
            input: subnetSrcIP
            value: 10.0.0.0/24
        - type: add_regex_if
          addRegexIf:
            input: subnetSrcIP
            output: match-10.0.*
            parameters: 10.0.*
        - type: add_regex_if
          addRegexIf:
            input: subnetSrcIP
            output: match-11.0.*
            parameters: 11.0.*
  - name: write1
    write:
      type: stdout
`)
	newFilter := InitNewTransformFilter(t, string(yamlConfig)).(*Filter)
	require.NotNil(t, newFilter)

	entry := test.GetIngestMockEntry(false)
	output, ok := newFilter.Transform(entry)
	require.True(t, ok)

	require.Equal(t, "10.0.0.1", output["srcIP"])
	require.Equal(t, "10.0.0.0/24", output["subnetSrcIP"])
	require.Equal(t, "10.0.0.0/24", output["match-10.0.*"])
	require.NotEqual(t, "10.0.0.0/24", output["match-11.0.*"])
}

func Test_AddLabelIf(t *testing.T) {
	entry := config.GenericMap{
		"param1": 5,
		"param2": 7,
		"param3": -1,
	}
	cfg := config.StageParam{
		Transform: &config.Transform{
			Filter: &api.TransformFilter{
				Rules: []api.TransformFilterRule{
					{
						Type: "add_label_if",
						AddLabelIf: &api.TransformFilterRuleWithAssignee{
							Input:      "param1",
							Parameters: "<10",
							Output:     "group1",
							Assignee:   "LT10",
						},
					},
					{
						Type: "add_label_if",
						AddLabelIf: &api.TransformFilterRuleWithAssignee{
							Input:      "param1",
							Parameters: ">=10",
							Output:     "group1",
							Assignee:   "GE10",
						},
					},
					{
						Type: "add_label_if",
						AddLabelIf: &api.TransformFilterRuleWithAssignee{
							Input:      "param2",
							Parameters: "<5",
							Output:     "group2",
							Assignee:   "LT5",
						},
					},
					{
						Type: "add_label_if",
						AddLabelIf: &api.TransformFilterRuleWithAssignee{
							Input:      "param3",
							Parameters: "<0",
							Output:     "group3",
							Assignee:   "LT0",
						},
					},
				},
			},
		},
	}

	tr, err := NewTransformFilter(cfg)
	require.NoError(t, err)

	output, ok := tr.Transform(entry)
	require.True(t, ok)
	require.Contains(t, output, "labels")
	labelsString := output["labels"]
	require.Contains(t, labelsString, "group1=LT10")
	require.Contains(t, labelsString, "group3=LT0")
	require.NotContains(t, labelsString, "group2")
	require.NotContains(t, labelsString, "group1=GE10")
}

func Test_AddLabel(t *testing.T) {
	entry := config.GenericMap{
		"param1": 5,
		"param2": 7,
		"param3": -1,
	}
	cfg := config.StageParam{
		Transform: &config.Transform{
			Filter: &api.TransformFilter{
				Rules: []api.TransformFilterRule{
					{
						Type: "add_label",
						AddLabel: &api.TransformFilterGenericRule{
							Input: "key1",
							Value: "value1",
						},
					},
					{
						Type: "add_label",
						AddLabel: &api.TransformFilterGenericRule{
							Input: "key2",
							Value: "value2",
						},
					},
					{
						Type: "add_label",
						AddLabel: &api.TransformFilterGenericRule{
							Input: "key3",
							Value: "value3",
						},
					},
				},
			},
		},
	}

	tr, err := NewTransformFilter(cfg)
	require.NoError(t, err)

	output, ok := tr.Transform(entry)
	require.True(t, ok)
	require.Contains(t, output, "labels")
	labelsString := output["labels"]
	require.Contains(t, labelsString, "key1=value1")
	require.Contains(t, labelsString, "key2=value2")

	cfg2 := config.StageParam{
		Transform: &config.Transform{
			Filter: &api.TransformFilter{},
		},
	}

	tr, err = NewTransformFilter(cfg2)
	require.NoError(t, err)

	output, ok = tr.Transform(entry)
	require.True(t, ok)
	require.NotContains(t, output, "labels")
}

func Test_AddField(t *testing.T) {
	entry := config.GenericMap{
		"param1": 5,
		"param2": 7,
		"param3": -1,
	}
	cfg := config.StageParam{
		Transform: &config.Transform{
			Filter: &api.TransformFilter{
				Rules: []api.TransformFilterRule{
					{
						Type: "add_field",
						AddField: &api.TransformFilterGenericRule{
							Input: "field1",
							Value: "value1",
						},
					},
					{
						Type: "add_field",
						AddField: &api.TransformFilterGenericRule{
							Input: "param1",
							Value: "new_value",
						},
					},
				},
			},
		},
	}

	tr, err := NewTransformFilter(cfg)
	require.NoError(t, err)

	output, ok := tr.Transform(entry)
	require.True(t, ok)
	require.Contains(t, output, "field1")
	require.Equal(t, "value1", output["field1"])
	require.Equal(t, "new_value", output["param1"])
}

func Test_Transform_RemoveEntryAllSatisfied(t *testing.T) {
	newFilter := api.TransformFilter{
		Rules: []api.TransformFilterRule{
			{
				Type: api.RemoveEntryAllSatisfied,
				RemoveEntryAllSatisfied: []*api.RemoveEntryRule{
					{
						Type: api.RemoveEntryIfEqualD,
						RemoveEntry: &api.TransformFilterGenericRule{
							Input: "FlowDirection",
							Value: 1,
						},
					},
					{
						Type: api.RemoveEntryIfExistsD,
						RemoveEntry: &api.TransformFilterGenericRule{
							Input: "DstK8S_OwnerName",
						},
					},
				},
			},
		},
	}

	tf, err := NewTransformFilter(config.StageParam{Transform: &config.Transform{Filter: &newFilter}})
	require.NoError(t, err)

	_, keep := tf.Transform(config.GenericMap{
		"FlowDirection":    0,
		"SrcK8S_OwnerName": "my-deployment",
		"DstK8S_OwnerName": "my-service",
		"Bytes":            10,
	}) // Ingress and internal => keep
	require.True(t, keep)

	_, keep = tf.Transform(config.GenericMap{
		"FlowDirection":    1,
		"SrcK8S_OwnerName": "my-deployment",
		"DstK8S_OwnerName": "my-service",
		"Bytes":            10,
	}) // Egress and internal => remove
	require.False(t, keep)

	_, keep = tf.Transform(config.GenericMap{
		"FlowDirection":    1,
		"SrcK8S_OwnerName": "my-deployment",
		"Bytes":            10,
	}) // Egress and external => keep
	require.True(t, keep)

	_, keep = tf.Transform(config.GenericMap{
		"FlowDirection":    0,
		"DstK8S_OwnerName": "my-deployment",
		"Bytes":            10,
	}) // Ingress and external => keep
	require.True(t, keep)
}

func Test_Transform_Sampling(t *testing.T) {
	newFilter := api.TransformFilter{
		Rules: []api.TransformFilterRule{
			{
				Type: api.ConditionalSampling,
				ConditionalSampling: []*api.SamplingCondition{
					{
						Rules: []*api.RemoveEntryRule{
							{
								Type: api.RemoveEntryIfEqualD,
								RemoveEntry: &api.TransformFilterGenericRule{
									Input: "FlowDirection",
									Value: 1,
								},
							},
							{
								Type: api.RemoveEntryIfExistsD,
								RemoveEntry: &api.TransformFilterGenericRule{
									Input: "DstK8S_OwnerName",
								},
							},
						},
						Value: 10,
					},
				},
			},
		},
	}

	tf, err := NewTransformFilter(config.StageParam{Transform: &config.Transform{Filter: &newFilter}})
	require.NoError(t, err)

	input := []config.GenericMap{}
	for i := 0; i < 1000; i++ {
		input = append(input, config.GenericMap{
			"FlowDirection":    0,
			"SrcK8S_OwnerName": "my-deployment",
			"DstK8S_OwnerName": "my-service",
			"Bytes":            10,
		}) // Ingress and internal => always keep
		input = append(input, config.GenericMap{
			"FlowDirection":    1,
			"SrcK8S_OwnerName": "my-deployment",
			"DstK8S_OwnerName": "my-service",
			"Bytes":            10,
		}) // Egress and internal => sample 1:10
	}

	countIngress := 0
	countEgress := 0

	for _, flow := range input {
		if out, ok := tf.Transform(flow); ok {
			switch out["FlowDirection"] {
			case 0:
				countIngress++
			case 1:
				countEgress++
			}
		}
	}

	assert.Equal(t, countIngress, 1000)
	assert.Less(t, countEgress, 300)
	assert.Greater(t, countEgress, 30)
}

func Test_Transform_KeepEntry(t *testing.T) {
	newFilter := api.TransformFilter{
		Rules: []api.TransformFilterRule{
			{
				Type: api.KeepEntry,
				KeepEntryAllSatisfied: []*api.KeepEntryRule{
					{
						Type: api.KeepEntryIfEqual,
						KeepEntry: &api.TransformFilterGenericRule{
							Input: "namespace",
							Value: "A",
						},
					},
					{
						Type: api.KeepEntryIfExists,
						KeepEntry: &api.TransformFilterGenericRule{
							Input: "workload",
						},
					},
				},
			},
			{
				Type: api.KeepEntry,
				KeepEntryAllSatisfied: []*api.KeepEntryRule{
					{
						Type: api.KeepEntryIfRegexMatch,
						KeepEntry: &api.TransformFilterGenericRule{
							Input: "service",
							Value: "abc.+",
						},
					},
				},
			},
		},
	}

	tf, err := NewTransformFilter(config.StageParam{Transform: &config.Transform{Filter: &newFilter}})
	require.NoError(t, err)

	_, keep := tf.Transform(config.GenericMap{
		"namespace": "A",
		"workload":  "my-deployment",
		"service":   "xyz",
	})
	require.True(t, keep)

	_, keep = tf.Transform(config.GenericMap{
		"namespace": "B",
		"workload":  "my-deployment",
		"service":   "xyz",
	})
	require.False(t, keep)

	_, keep = tf.Transform(config.GenericMap{
		"namespace": "A",
		"service":   "xyz",
	})
	require.False(t, keep)

	_, keep = tf.Transform(config.GenericMap{
		"namespace": "A",
		"service":   "abcd",
	})
	require.True(t, keep)
}

func Test_Transform_KeepEntrySampling(t *testing.T) {
	newFilter := api.TransformFilter{
		Rules: []api.TransformFilterRule{
			{
				Type: api.KeepEntry,
				KeepEntryAllSatisfied: []*api.KeepEntryRule{
					{
						Type: api.KeepEntryIfEqual,
						KeepEntry: &api.TransformFilterGenericRule{
							Input: "namespace",
							Value: "A",
						},
					},
				},
				KeepEntrySampling: 10,
			},
			{
				Type: api.KeepEntry,
				KeepEntryAllSatisfied: []*api.KeepEntryRule{
					{
						Type: api.KeepEntryIfEqual,
						KeepEntry: &api.TransformFilterGenericRule{
							Input: "namespace",
							Value: "B",
						},
					},
				},
			},
		},
	}

	tf, err := NewTransformFilter(config.StageParam{Transform: &config.Transform{Filter: &newFilter}})
	require.NoError(t, err)

	input := []config.GenericMap{}
	for i := 0; i < 1000; i++ {
		input = append(input, config.GenericMap{
			"namespace": "A",
		})
		input = append(input, config.GenericMap{
			"namespace": "B",
		})
	}

	countA := 0
	countB := 0

	for _, flow := range input {
		if out, ok := tf.Transform(flow); ok {
			switch out["namespace"] {
			case "A":
				countA++
			case "B":
				countB++
			}
		}
	}

	assert.Less(t, countA, 300)
	assert.Greater(t, countA, 30)
	assert.Equal(t, countB, 1000)
}
