/*
 * Copyright (C) 2022 IBM, Inc.
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
 *
 */

package encode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/schema"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	parquet "github.com/parquet-go/parquet-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testS3ConfigJSON = `---
log-level: debug
pipeline:
  - name: encode1
parameters:
  - name: encode1
    encode:
      type: s3
      s3:
        endpoint: 1.2.3.4:9000
        bucket: bucket1
        account: account1
        accessKeyId: accessKey1
        secretAccessKey: secretAccessKey1
        format: json
        writeTimeout: 1s
        batchSize: 3
        objectHeaderParameters:
          key1: val1
          key2: val2
          key3: val3
          key4: val4
`

const testS3ConfigParquet = `---
log-level: debug
pipeline:
  - name: encode1
parameters:
  - name: encode1
    encode:
      type: s3
      s3:
        endpoint: 1.2.3.4:9000
        bucket: bucket1
        account: production-cluster
        prefix: netobserv
        accessKeyId: accessKey1
        secretAccessKey: secretAccessKey1
        writeTimeout: 1s
        batchSize: 3
`

var (
	syncChan chan bool
)

type fakeS3Writer struct {
	mutex        sync.Mutex
	objects      [][]byte
	objectNames  []string
	bucketNames  []string
	contentTypes []string
}

func (f *fakeS3Writer) putObject(bucket, objectName string, data []byte, contentType string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	f.objects = append(f.objects, cp)
	f.objectNames = append(f.objectNames, objectName)
	f.bucketNames = append(f.bucketNames, bucket)
	f.contentTypes = append(f.contentTypes, contentType)
	syncChan <- true
	return nil
}

func initNewEncodeS3(t *testing.T, configString string) *encodeS3 {
	v, cfg := test.InitConfig(t, configString)
	require.NotNil(t, v)

	syncChan = make(chan bool, 10)
	newEncode, err := NewEncodeS3(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)
	encodeS3 := newEncode.(*encodeS3)
	f := &fakeS3Writer{}
	encodeS3.s3Writer = f
	return encodeS3
}

func Test_RedactedKey(t *testing.T) {
	v, cfg := test.InitConfig(t, testS3ConfigJSON)
	require.NotNil(t, v)

	asString := fmt.Sprintf("%v", *cfg.Parameters[0].Encode.S3)
	assert.Contains(t, asString, "[REDACTED]")
	assert.Contains(t, asString, "account1")
	assert.NotContains(t, asString, "secretAccessKey1")
}

func Test_EncodeS3_JSON(t *testing.T) {
	utils.InitExitChannel()
	defer utils.CloseExitChannel()
	enc := initNewEncodeS3(t, testS3ConfigJSON)
	require.Equal(t, api.S3FormatJSON, enc.format)
	require.Equal(t, "1.2.3.4:9000", enc.s3Params.Endpoint)

	entries := test.GetExtractMockEntries2()
	for i := range entries {
		enc.Encode(entries[i])
	}

	<-syncChan

	fakeWriter := enc.s3Writer.(*fakeS3Writer)
	fakeWriter.mutex.Lock()
	defer fakeWriter.mutex.Unlock()

	require.Equal(t, contentTypeJSON, fakeWriter.contentTypes[0])
	var object0 map[string]interface{}
	require.NoError(t, json.Unmarshal(fakeWriter.objects[0], &object0))
	require.Equal(t, float64(3), object0["number_of_flow_logs"])
	require.Equal(t, "bucket1", fakeWriter.bucketNames[0])
	require.Contains(t, fakeWriter.objectNames[0], "account1")
	require.Contains(t, fakeWriter.objectNames[0], "year=")
	require.Contains(t, fakeWriter.objectNames[0], "stream-id=")
	require.Contains(t, fakeWriter.objectNames[0], "00000000")
}

func Test_EncodeS3_Parquet(t *testing.T) {
	utils.InitExitChannel()
	defer utils.CloseExitChannel()
	enc := initNewEncodeS3(t, testS3ConfigParquet)
	require.Equal(t, api.S3FormatParquet, enc.format)

	entries := []config.GenericMap{
		{"TimeFlowStartMs": int64(1000), "TimeFlowEndMs": int64(2000), "SrcAddr": "10.0.0.1", "DstAddr": "10.0.0.2", "Bytes": int64(100), "SrcK8S_Name": "pod-a"},
		{"TimeFlowStartMs": int64(1100), "TimeFlowEndMs": int64(2100), "SrcAddr": "10.0.0.3", "DstAddr": "10.0.0.4", "Bytes": int64(200)},
		{"TimeFlowStartMs": int64(1200), "TimeFlowEndMs": int64(2200), "SrcAddr": "10.0.0.5", "DstAddr": "10.0.0.6", "Bytes": int64(300)},
	}
	for _, e := range entries {
		enc.Encode(e)
	}
	<-syncChan

	fakeWriter := enc.s3Writer.(*fakeS3Writer)
	fakeWriter.mutex.Lock()
	defer fakeWriter.mutex.Unlock()

	require.Equal(t, contentTypeParquet, fakeWriter.contentTypes[0])
	key := fakeWriter.objectNames[0]
	require.Contains(t, key, "netobserv/cluster_id=production-cluster/")
	require.Contains(t, key, "year=")
	require.Contains(t, key, "month=")
	require.Contains(t, key, "day=")
	require.Contains(t, key, "hour=")
	require.Contains(t, key, "part-")
	require.True(t, strings.HasSuffix(key, ".parquet"))

	// Round-trip schema + metadata
	rows, err := parquet.Read[schema.FlowRecordV1](bytes.NewReader(fakeWriter.objects[0]), int64(len(fakeWriter.objects[0])))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "10.0.0.1", rows[0].SrcAddr)
	assert.Equal(t, "pod-a", rows[0].SrcK8S_Name)
}

func Test_objectKey_ParquetLayout(t *testing.T) {
	now := time.Date(2026, 7, 17, 14, 30, 0, 0, time.UTC)
	key := objectKey(api.EncodeS3{Account: "prod", Prefix: "flows"}, api.S3FormatParquet, "flp-abc", 7, now)
	assert.Equal(t, "flows/cluster_id=prod/year=2026/month=07/day=17/hour=14/part-flp-abc-00000007.parquet", key)
}

func Test_EncodeS3_FlushOnExit(t *testing.T) {
	utils.InitExitChannel()
	enc := initNewEncodeS3(t, testS3ConfigParquet)
	// Partial batch — below batchSize=3 so timeout loop would otherwise wait
	enc.Encode(config.GenericMap{"SrcAddr": "10.0.0.1", "Bytes": int64(1)})
	enc.Encode(config.GenericMap{"SrcAddr": "10.0.0.2", "Bytes": int64(2)})

	utils.CloseExitChannel()
	select {
	case <-syncChan:
	case <-time.After(2 * time.Second):
		t.Fatal("expected S3 flush on exit for partial batch")
	}

	fakeWriter := enc.s3Writer.(*fakeS3Writer)
	fakeWriter.mutex.Lock()
	defer fakeWriter.mutex.Unlock()
	require.Len(t, fakeWriter.objects, 1)
	require.Equal(t, contentTypeParquet, fakeWriter.contentTypes[0])
}

func Test_EncodeParquetBytes_Metadata(t *testing.T) {
	data, err := EncodeParquetBytes([]config.GenericMap{
		{"TimeFlowStartMs": int64(1), "TimeFlowEndMs": int64(2), "SrcAddr": "1.1.1.1"},
	})
	require.NoError(t, err)
	file, err := parquet.OpenFile(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	found := false
	for _, kv := range file.Metadata().KeyValueMetadata {
		if kv.Key == schema.ParquetVersionKey && kv.Value == schema.ParquetVersion {
			found = true
		}
	}
	assert.True(t, found, "expected netobserv.parquet.version metadata")
}

func Test_timeout_JSON(t *testing.T) {
	utils.InitExitChannel()
	defer utils.CloseExitChannel()
	enc := initNewEncodeS3(t, testS3ConfigJSON)
	enc.Encode(test.GetExtractMockEntry())
	fakeWriter := enc.s3Writer.(*fakeS3Writer)
	fakeWriter.mutex.Lock()
	require.Equal(t, 0, len(fakeWriter.objects))
	fakeWriter.mutex.Unlock()
	time.Sleep(1 * time.Second)
	<-syncChan
	fakeWriter.mutex.Lock()
	require.Equal(t, 1, len(fakeWriter.objects))
	fakeWriter.mutex.Unlock()
}

const testS3ConfigDefaults = `---
log-level: debug
pipeline:
  - name: encode2
parameters:
  - name: encode2
    encode:
      type: s3
      s3:
        endpoint: 1.2.3.4:9000
        bucket: bucket1
        account: account1
        accessKeyId: accessKey1
        secretAccessKey: secretAccessKey1
`

func Test_defaults_Parquet(t *testing.T) {
	utils.InitExitChannel()
	defer utils.CloseExitChannel()
	enc := initNewEncodeS3(t, testS3ConfigDefaults)
	require.Equal(t, defaultTimeOut, enc.s3Params.WriteTimeout)
	require.Equal(t, defaultBatchSizeParquet, enc.s3Params.BatchSize)
	require.Equal(t, api.S3FormatParquet, enc.format)
}
