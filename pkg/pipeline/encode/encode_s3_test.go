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

package encode

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/require"
)

const testS3Config1 = `---
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
        account: tenant1
        accessKeyId: accessKey1
        secretAccessKey: secretAccessKey1
        writeTimeout: 10
        batchSize: 3
        objectHeaderParameters:
          key1: val1
          key2: val2
          key3: val3
          key4: val4
`

type fakeS3Writer struct {
	objects     []map[string]interface{}
	objectNames []string
	bucketNames []string
}

func (e *fakeS3Writer) putObject(bucket string, objectName string, object map[string]interface{}) error {
	e.objects = append(e.objects, object)
	e.objectNames = append(e.objectNames, objectName)
	e.bucketNames = append(e.bucketNames, bucket)
	return nil
}

func initNewEncodeS3(t *testing.T, configString string) *encodeS3 {
	v, cfg := test.InitConfig(t, configString)
	require.NotNil(t, v)

	newEncode, err := NewEncodeS3(operational.NewMetrics(&config.MetricsSettings{}), cfg.Parameters[0])
	require.NoError(t, err)
	encodeS3 := newEncode.(*encodeS3)
	encodeS3.s3Writer = &fakeS3Writer{}
	return encodeS3
}

func Test_EncodeS3(t *testing.T) {
	encodeS3 := initNewEncodeS3(t, testS3Config1)
	require.Equal(t, "1.2.3.4:9000", encodeS3.s3Params.Endpoint)
	require.Equal(t, "bucket1", encodeS3.s3Params.Bucket)
	require.Equal(t, "tenant1", encodeS3.s3Params.Account)
	require.Equal(t, "accessKey1", encodeS3.s3Params.AccessKeyId)
	require.Equal(t, "secretAccessKey1", encodeS3.s3Params.SecretAccessKey)

	entries := test.GetExtractMockEntries2()
	for i := range entries {
		encodeS3.Encode(entries[i])
	}

	// confirm object names, bucket name
	// confirm that object created has batchSize=3 entries
	writer := encodeS3.s3Writer
	fakeWriter := writer.(*fakeS3Writer)
	object0 := fakeWriter.objects[0]
	require.Contains(t, object0, "version")
	require.Contains(t, object0, "capture_start_time")
	require.Contains(t, object0, "capture_end_time")
	require.Contains(t, object0, "number_of_flow_logs")
	require.Equal(t, 3, object0["number_of_flow_logs"])
	require.Equal(t, "bucket1", fakeWriter.bucketNames[0])
}

// TBD: more tests; additional parameters, bad credentials, missing/default config parameters, timeout
