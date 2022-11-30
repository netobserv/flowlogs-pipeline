/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *	 http://www.apache.org/licenses/LICENSE-2.0
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
	"sync"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	flpS3Version     = "1.0"
	defaultTimeOut   = 60
	defaultBatchSize = 10
)

type encodeS3 struct {
	s3Params          api.EncodeS3
	s3Client          *minio.Client
	recordsWritten    prometheus.Counter
	pendingEntries    []config.GenericMap
	mutex             *sync.Mutex
	expiryTime        int64
	streamId          string
	intervalStartTime time.Time
	sequenceNumber    int64
}

func (s *encodeS3) writeObject() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	nLogs := len(s.pendingEntries)
	if nLogs > s.s3Params.BatchSize {
		nLogs = s.s3Params.BatchSize
	}
	now := time.Now()
	object := s.GenerateStoreHeader(s.pendingEntries[0:nLogs], s.intervalStartTime, now)
	year := fmt.Sprintf("%04d", now.Year())
	month := fmt.Sprintf("%02d", now.Month())
	day := fmt.Sprintf("%02d", now.Day())
	hour := fmt.Sprintf("%02d", now.Hour())
	seq := fmt.Sprintf("%08d", s.sequenceNumber)
	objectName := s.s3Params.Account + "/year=" + year + "/month=" + month + "/day=" + day + "/hour=" + hour + "/stream-id=" + s.streamId + "/" + seq
	log.Debugf("S3 writeObject: objectName = %s", objectName)
	log.Debugf("S3 writeObject: object = %v", object)
	s.pendingEntries = s.pendingEntries[nLogs:]
	s.intervalStartTime = now
	s.sequenceNumber++

	// send to object store
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(object)
	if err != nil {
		log.Errorf("error encoding object: %v", err)
		return
	}
	log.Debugf("encoded object = %v", b)
	uploadInfo, err := s.s3Client.PutObject(context.Background(), s.s3Params.Bucket, objectName, b, int64(b.Len()), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	log.Debugf("uploadInfo = %v", uploadInfo)
	if err != nil {
		log.Errorf("error writing to object store: %v", err)
	}
}

func (s *encodeS3) GenerateStoreHeader(flows []config.GenericMap, startTime time.Time, endTime time.Time) map[string]interface{} {
	augmentedObject := make(map[string]interface{})
	// copy user defined keys from config to object header
	for key, value := range s.s3Params.ObjectHeaderParameters {
		augmentedObject[key] = value
	}
	augmentedObject["version"] = flpS3Version
	augmentedObject["capture_start_time"] = startTime.Format(time.RFC3339)
	augmentedObject["capture_end_time"] = endTime.Format(time.RFC3339)
	augmentedObject["number_of_flow_logs"] = len(flows)
	augmentedObject["flow_logs"] = flows
	augmentedObject["state"] = "ok"

	return augmentedObject
}

// Encode writes entries to object store
func (s *encodeS3) Encode(entry config.GenericMap) {
	log.Debugf("Encode S3, entry = %v", entry)
	s.mutex.Lock()
	s.pendingEntries = append(s.pendingEntries, entry)
	s.mutex.Unlock()
	s.recordsWritten.Inc()
	if len(s.pendingEntries) >= s.s3Params.BatchSize {
		s.writeObject()
	}
}

// NewEncodeS3 create a new writer to S3
func NewEncodeS3(opMetrics *operational.Metrics, params config.StageParam) (Encoder, error) {
	configParams := api.EncodeS3{}
	if params.Encode != nil && params.Encode.S3 != nil {
		configParams = *params.Encode.S3
	}
	log.Debugf("NewEncodeS3, config = %v", configParams)
	s3Client := connectS3(configParams)
	if configParams.WriteTimeout == 0 {
		configParams.WriteTimeout = defaultTimeOut
	}
	if configParams.BatchSize == 0 {
		configParams.BatchSize = defaultBatchSize
	}

	return &encodeS3{
		s3Params:          configParams,
		s3Client:          s3Client,
		recordsWritten:    opMetrics.CreateRecordsWrittenCounter(params.Name),
		pendingEntries:    make([]config.GenericMap, 0),
		expiryTime:        time.Now().Unix() + configParams.WriteTimeout,
		streamId:          time.Now().Format(time.RFC3339),
		intervalStartTime: time.Now(),
		mutex:             &sync.Mutex{},
	}, nil
}

func connectS3(config api.EncodeS3) *minio.Client {
	// Initialize s3 client object.
	s3Client, err := minio.New(config.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(config.AccessKeyId, config.SecretAccessKey, ""),
		// TBD: security parameters
		Secure: false,
	})
	if err != nil {
		log.Errorf("Error when creating S3 client: %v", err)
		return nil
	}

	found, err := s3Client.BucketExists(context.Background(), config.Bucket)
	if err != nil {
		log.Errorf("Error accessing S3 bucket: %v", err)
		return nil
	}
	if found {
		log.Infof("Bucket %s found", config.Bucket)
	}
	log.Infof("s3Client = %#v\n", s3Client) // s3Client is now setup
	return s3Client
}
