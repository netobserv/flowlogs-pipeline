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

type ConnTrack struct {
	KeyFields         KeyFields     `yaml:"keyFields" doc:"fields that are used to identify the connection"`
	OutputRecordTypes []string      `yaml:"outputRecordTypes" doc:"output record types to emit"`
	OutputFields      []OutputField `yaml:"outputFields" doc:"list of output fields"`
}

// TODO: add annotations

type KeyFields struct {
	FieldGroups []FieldGroup
	Hash        ConnTrackHash
}

type FieldGroup struct {
	Name   string
	Fields []string
}

type ConnTrackHash struct {
	FieldGroups []string
	FieldGroupA string
	FieldGroupB string
}

type OutputField struct {
	Name      string `yaml:"name" doc:"entry input field"`
	Operation string `yaml:"operation" doc:"entry output field"`
	SplitAB   bool   `yaml:"splitAB" doc:"one of the following:"`
	Input     string `yaml:"input" doc:"parameters specific to type"`
}
