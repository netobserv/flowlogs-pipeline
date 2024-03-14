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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
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
        account: account1
        accessKeyId: accessKey1
        secretAccessKey: secretAccessKey1
        writeTimeout: 1s
        batchSize: 3
        objectHeaderParameters:
          key1: val1
          key2: val2
          key3: val3
          key4: val4
`

var (
	syncChan chan bool
)

type fakeS3Writer struct {
	mutex       sync.Mutex
	objects     []map[string]interface{}
	objectNames []string
	bucketNames []string
}

func (f *fakeS3Writer) putObject(bucket string, objectName string, object map[string]interface{}) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.objects = append(f.objects, object)
	f.objectNames = append(f.objectNames, objectName)
	f.bucketNames = append(f.bucketNames, bucket)
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
	f.objects = nil
	f.objectNames = nil
	f.bucketNames = nil
	return encodeS3
}

func Test_EncodeS3(t *testing.T) {
	utils.InitExitChannel()
	encodeS3 := initNewEncodeS3(t, testS3Config1)
	require.Equal(t, "1.2.3.4:9000", encodeS3.s3Params.Endpoint)
	require.Equal(t, "bucket1", encodeS3.s3Params.Bucket)
	require.Equal(t, "account1", encodeS3.s3Params.Account)
	require.Equal(t, "accessKey1", encodeS3.s3Params.AccessKeyID)
	require.Equal(t, "secretAccessKey1", encodeS3.s3Params.SecretAccessKey)

	entries := test.GetExtractMockEntries2()
	for i := range entries {
		encodeS3.Encode(entries[i])
	}

	<-syncChan

	writer := encodeS3.s3Writer
	fakeWriter := writer.(*fakeS3Writer)
	fakeWriter.mutex.Lock()
	object0 := fakeWriter.objects[0]
	objectName0 := fakeWriter.objectNames[0]

	// confirm object header fields
	require.Contains(t, object0, "version")
	require.Contains(t, object0, "capture_start_time")
	require.Contains(t, object0, "capture_end_time")
	require.Contains(t, object0, "number_of_flow_logs")

	// confirm that object created has batchSize=3 entries
	require.Equal(t, 3, object0["number_of_flow_logs"])

	// confirm object names, bucket name
	require.Equal(t, "bucket1", fakeWriter.bucketNames[0])
	expectedSubstring := "account1"
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "year"
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "month="
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "day="
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "hour="
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "stream-id="
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "00000000"
	require.Contains(t, objectName0, expectedSubstring)
	expectedSubstring = "00000001"
	require.Contains(t, fakeWriter.objectNames[1], expectedSubstring)
	expectedSubstring = "00000002"
	require.Contains(t, fakeWriter.objectNames[2], expectedSubstring)
	fakeWriter.mutex.Unlock()
	utils.CloseExitChannel()
}

func Test_timeout(t *testing.T) {
	utils.InitExitChannel()
	encodeS3 := initNewEncodeS3(t, testS3Config1)
	entry := test.GetExtractMockEntry()
	// write a single entry, which will be written only after a timeout
	encodeS3.Encode(entry)
	writer := encodeS3.s3Writer
	fakeWriter := writer.(*fakeS3Writer)
	fakeWriter.mutex.Lock()
	require.Equal(t, 0, len(fakeWriter.objects))
	fakeWriter.mutex.Unlock()
	fmt.Printf("sleep for a second to reach timeout \n")
	time.Sleep(1 * time.Second)
	<-syncChan
	fakeWriter.mutex.Lock()
	require.Equal(t, 1, len(fakeWriter.objects))
	fakeWriter.mutex.Unlock()
	utils.CloseExitChannel()
}

const testS3Config2 = `---
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

func Test_defaults(t *testing.T) {
	utils.InitExitChannel()
	encodeS3 := initNewEncodeS3(t, testS3Config2)
	require.Equal(t, defaultTimeOut, encodeS3.s3Params.WriteTimeout)
	require.Equal(t, defaultBatchSize, encodeS3.s3Params.BatchSize)
	utils.CloseExitChannel()
}
