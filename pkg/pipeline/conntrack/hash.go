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

package contrack

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"hash/fnv"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

// ComputeHash computes the hash of a flow log according to keyFields.
// Two flow logs will have the same hash if they belong to the same connection.
func ComputeHash(flowLog config.GenericMap, keyFields api.KeyFields) ([]byte, error) {
	type hashType []byte
	fieldGroup2hash := make(map[string]hashType)

	// Compute the hash of each field group
	for _, fg := range keyFields.FieldGroups {
		h, err := computeHashFields(flowLog, fg.Fields)
		if err != nil {
			return nil, fmt.Errorf("compute hash: %w", err)
		}
		fieldGroup2hash[fg.Name] = h
	}

	// Compute the total hash
	hash := fnv.New32a()
	for _, fgName := range keyFields.Hash.FieldGroups {
		hash.Write(fieldGroup2hash[fgName])
	}
	if keyFields.Hash.FieldGroupA != "" {
		hashA := fieldGroup2hash[keyFields.Hash.FieldGroupA]
		hashB := fieldGroup2hash[keyFields.Hash.FieldGroupB]
		// Determine order between A's and B's hash to get the same hash for both flow logs from A to B and from B to A.
		if bytes.Compare(hashA, hashB) < 0 {
			hash.Write(hashA)
			hash.Write(hashB)
		} else {
			hash.Write(hashB)
			hash.Write(hashA)
		}
	}
	return hash.Sum([]byte{}), nil
}

func computeHashFields(flowLog config.GenericMap, fieldNames []string) ([]byte, error) {
	h := fnv.New32a()
	for _, fn := range fieldNames {
		f := flowLog[fn]
		bytes, err := toBytes(f)
		if err != nil {
			return nil, err
		}
		h.Write(bytes)
	}
	return h.Sum([]byte{}), nil
}

func toBytes(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	bytes := buf.Bytes()
	return bytes, nil
}
