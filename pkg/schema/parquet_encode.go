/*
 * Copyright (C) 2026 NetObserv Authors.
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
 */

package schema

import (
	"bytes"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	parquet "github.com/parquet-go/parquet-go"
)

// EncodeParquetBytes encodes flows as Parquet schema v1 with netobserv.parquet.version metadata.
func EncodeParquetBytes(flows []config.GenericMap) ([]byte, error) {
	records := make([]FlowRecordV1, 0, len(flows))
	for _, f := range flows {
		records = append(records, FromGenericMap(f))
	}
	buf := new(bytes.Buffer)
	w := parquet.NewGenericWriter[FlowRecordV1](buf,
		parquet.KeyValueMetadata(ParquetVersionKey, ParquetVersion),
	)
	if _, err := w.Write(records); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
