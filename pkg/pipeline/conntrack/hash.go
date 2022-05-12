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

package conntrack

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"hash"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	log "github.com/sirupsen/logrus"
)

type hashType []byte

// TODO: what's a better name for this struct?
type totalHashType struct {
	hashA     hashType
	hashB     hashType
	hashTotal hashType
}

func copyTotalHash(h totalHashType) totalHashType {
	newHashA := make([]byte, len(h.hashA))
	newHashB := make([]byte, len(h.hashB))
	newHashTotal := make([]byte, len(h.hashTotal))
	copy(newHashA, h.hashA)
	copy(newHashB, h.hashB)
	copy(newHashTotal, h.hashTotal)
	return totalHashType{
		hashA:     newHashA,
		hashB:     newHashB,
		hashTotal: newHashTotal,
	}
}

// ComputeHash computes the hash of a flow log according to keyDefinition.
// Two flow logs will have the same hash if they belong to the same connection.
func ComputeHash(flowLog config.GenericMap, keyDefinition api.KeyDefinition, hasher hash.Hash) (*totalHashType, error) {
	fieldGroup2hash := make(map[string]hashType)

	// Compute the hash of each field group
	for _, fg := range keyDefinition.FieldGroups {
		h, err := computeHashFields(flowLog, fg.Fields, hasher)
		if err != nil {
			return nil, fmt.Errorf("compute hash: %w", err)
		}
		fieldGroup2hash[fg.Name] = h
	}

	// Compute the total hash
	th := &totalHashType{}
	hasher.Reset()
	for _, fgName := range keyDefinition.Hash.FieldGroupRefs {
		hasher.Write(fieldGroup2hash[fgName])
	}
	if keyDefinition.Hash.FieldGroupARef != "" {
		th.hashA = fieldGroup2hash[keyDefinition.Hash.FieldGroupARef]
		th.hashB = fieldGroup2hash[keyDefinition.Hash.FieldGroupBRef]
		// Determine order between A's and B's hash to get the same hash for both flow logs from A to B and from B to A.
		if bytes.Compare(th.hashA, th.hashB) < 0 {
			hasher.Write(th.hashA)
			hasher.Write(th.hashB)
		} else {
			hasher.Write(th.hashB)
			hasher.Write(th.hashA)
		}
	}
	th.hashTotal = hasher.Sum([]byte{})
	return th, nil
}

func computeHashFields(flowLog config.GenericMap, fieldNames []string, hasher hash.Hash) (hashType, error) {
	hasher.Reset()
	for _, fn := range fieldNames {
		f, ok := flowLog[fn]
		if !ok {
			log.Warningf("Missing field %v", fn)
			continue
		}
		bytes, err := toBytes(f)
		if err != nil {
			return nil, err
		}
		hasher.Write(bytes)
	}
	return hasher.Sum([]byte{}), nil
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
