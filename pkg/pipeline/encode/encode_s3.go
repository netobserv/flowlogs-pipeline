/*
 * Copyright (C) 2022 IBM, Inc.
 * Copyright (C) 2026 NetObserv Authors.
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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/schema"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	flpS3Version            = "v0.1"
	defaultBatchSizeJSON    = 10
	defaultBatchSizeParquet = 5000
	contentTypeParquet      = "application/vnd.apache.parquet"
	contentTypeJSON         = "application/json"
)

var (
	defaultTimeOut = api.Duration{Duration: 60 * time.Second}
	s3log          = logrus.WithField("component", "encode.S3")
)

type encodeS3 struct {
	s3Params       api.EncodeS3
	format         string
	s3Writer       s3WriteEntries
	recordsWritten prometheus.Counter
	objectsWritten prometheus.Counter
	bytesWritten   prometheus.Counter
	writeErrors    prometheus.Counter
	pendingEntries []config.GenericMap
	mutex          *sync.Mutex
	expiryTime     time.Time
	exitChan       <-chan struct{}
	streamID       string
	intervalStart  time.Time
	sequenceNumber int64
}

type s3WriteEntries interface {
	putObject(bucket, objectName string, data []byte, contentType string) error
}

type encodeS3Writer struct {
	s3Client *minio.Client
	s3Params *api.EncodeS3
}

// objectKey builds the S3 object key for the configured format.
func objectKey(params api.EncodeS3, format, streamID string, seq int64, now time.Time) string {
	year := fmt.Sprintf("%04d", now.Year())
	month := fmt.Sprintf("%02d", now.Month())
	day := fmt.Sprintf("%02d", now.Day())
	hour := fmt.Sprintf("%02d", now.Hour())
	clusterID := params.Account
	if clusterID == "" {
		clusterID = "unknown"
	}

	if format == api.S3FormatJSON {
		// Legacy JSON layout (backward compatible when format=json)
		seqStr := fmt.Sprintf("%08d", seq)
		base := params.Account + "/year=" + year + "/month=" + month + "/day=" + day + "/hour=" + hour + "/stream-id=" + streamID + "/" + seqStr
		if params.Prefix != "" {
			return path.Join(strings.Trim(params.Prefix, "/"), base)
		}
		return base
	}

	// Parquet Hive partitions:
	// [<prefix>/]cluster_id=<id>/year=YYYY/month=MM/day=DD/hour=HH/part-<stream>-<seq>.parquet
	part := fmt.Sprintf("part-%s-%08d.parquet", streamID, seq)
	parts := []string{
		fmt.Sprintf("cluster_id=%s", clusterID),
		"year=" + year,
		"month=" + month,
		"day=" + day,
		"hour=" + hour,
		part,
	}
	key := strings.Join(parts, "/")
	if params.Prefix != "" {
		return path.Join(strings.Trim(params.Prefix, "/"), key)
	}
	return key
}

// EncodeParquetBytes encodes flows as Parquet schema v1 (exported for tests).
// Prefer schema.EncodeParquetBytes for new callers outside this package.
func EncodeParquetBytes(flows []config.GenericMap) ([]byte, error) {
	return schema.EncodeParquetBytes(flows)
}

// The mutex must be held when calling writeObject
func (s *encodeS3) writeObject() error {
	nLogs := len(s.pendingEntries)
	if nLogs == 0 {
		return nil
	}
	if nLogs > s.s3Params.BatchSize {
		nLogs = s.s3Params.BatchSize
	}
	now := time.Now()
	batch := s.pendingEntries[0:nLogs]
	objectName := objectKey(s.s3Params, s.format, s.streamID, s.sequenceNumber, now)

	var (
		data        []byte
		contentType string
		err         error
	)
	if s.format == api.S3FormatJSON {
		object := s.GenerateStoreHeader(batch, s.intervalStart, now)
		b := new(bytes.Buffer)
		if err = json.NewEncoder(b).Encode(object); err != nil {
			s.writeErrors.Inc()
			s3log.Errorf("error encoding JSON object: %v", err)
			return err
		}
		data = b.Bytes()
		contentType = contentTypeJSON
	} else {
		data, err = EncodeParquetBytes(batch)
		if err != nil {
			s.writeErrors.Inc()
			s3log.Errorf("error encoding Parquet object: %v", err)
			return err
		}
		contentType = contentTypeParquet
	}

	s.pendingEntries = s.pendingEntries[nLogs:]
	s.intervalStart = now
	s.expiryTime = now.Add(s.s3Params.WriteTimeout.Duration)
	s.sequenceNumber++

	s3log.Tracef("S3 writeObject: objectName = %s len=%d", objectName, len(data))
	err = s.s3Writer.putObject(s.s3Params.Bucket, objectName, data, contentType)
	if err != nil {
		s.writeErrors.Inc()
		s3log.Errorf("error in writing object: %v", err)
		return err
	}
	s.objectsWritten.Inc()
	s.bytesWritten.Add(float64(len(data)))
	return nil
}

func (s *encodeS3) GenerateStoreHeader(flows []config.GenericMap, startTime time.Time, endTime time.Time) map[string]interface{} {
	object := make(map[string]interface{})
	for key, value := range s.s3Params.ObjectHeaderParameters {
		object[key] = value
	}
	object["version"] = flpS3Version
	object["capture_start_time"] = startTime.Format(time.RFC3339)
	object["capture_end_time"] = endTime.Format(time.RFC3339)
	object["number_of_flow_logs"] = len(flows)
	object["flow_logs"] = flows
	return object
}

func (s *encodeS3) Update(_ config.StageParam) {
	s3log.Warn("Encode S3 Writer, update not supported")
}

func (s *encodeS3) createObjectTimeoutLoop() {
	ticker := time.NewTicker(s.s3Params.WriteTimeout.Duration)
	for {
		select {
		case <-s.exitChan:
			// Flush remaining flows before exit so short captures / pod
			// termination do not drop a partial Parquet batch.
			s.mutex.Lock()
			n := len(s.pendingEntries)
			err := s.writeObject()
			s.mutex.Unlock()
			if err != nil {
				s3log.Errorf("flush on exit failed (%d pending): %v", n, err)
			} else if n > 0 {
				s3log.Infof("flushed %d flow(s) to S3 on exit", n)
			} else {
				s3log.Debugf("exiting createObjectTimeoutLoop (nothing pending)")
			}
			return
		case <-ticker.C:
			s.mutex.Lock()
			_ = s.writeObject()
			s.mutex.Unlock()
		}
	}
}

// Encode queues entries to be sent to object store
func (s *encodeS3) Encode(entry config.GenericMap) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.pendingEntries = append(s.pendingEntries, entry)
	s.recordsWritten.Inc()
	if len(s.pendingEntries) >= s.s3Params.BatchSize {
		_ = s.writeObject()
	}
}

func resolveS3Format(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", api.S3FormatParquet:
		return api.S3FormatParquet
	case api.S3FormatJSON:
		return api.S3FormatJSON
	default:
		s3log.Warnf("unknown S3 format %q, using parquet", format)
		return api.S3FormatParquet
	}
}

func streamIDForInstance() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		// Keep key-friendly
		h = strings.ReplaceAll(h, ".", "-")
		if len(h) > 32 {
			h = h[:32]
		}
		return h
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// NewEncodeS3 creates a new writer to S3
func NewEncodeS3(opMetrics *operational.Metrics, params config.StageParam) (Encoder, error) {
	configParams := api.EncodeS3{}
	if params.Encode != nil && params.Encode.S3 != nil {
		configParams = *params.Encode.S3
	}
	format := resolveS3Format(configParams.Format)
	s3log.Debugf("NewEncodeS3, format=%s config=%v", format, configParams)
	s3Writer := &encodeS3Writer{
		s3Params: &configParams,
	}
	if configParams.WriteTimeout.Duration == time.Duration(0) {
		configParams.WriteTimeout = defaultTimeOut
	}
	if configParams.BatchSize == 0 {
		if format == api.S3FormatJSON {
			configParams.BatchSize = defaultBatchSizeJSON
		} else {
			configParams.BatchSize = defaultBatchSizeParquet
		}
	}

	s := &encodeS3{
		s3Params:       configParams,
		format:         format,
		s3Writer:       s3Writer,
		recordsWritten: opMetrics.CreateRecordsWrittenCounter(params.Name),
		objectsWritten: opMetrics.CreateS3ObjectsWrittenCounter(params.Name, format),
		bytesWritten:   opMetrics.CreateS3BytesWrittenCounter(params.Name, format),
		writeErrors:    opMetrics.CreateS3WriteErrorsCounter(params.Name, format),
		pendingEntries: make([]config.GenericMap, 0),
		expiryTime:     time.Now().Add(configParams.WriteTimeout.Duration),
		exitChan:       utils.ExitChannel(),
		streamID:       streamIDForInstance(),
		intervalStart:  time.Now(),
		mutex:          &sync.Mutex{},
	}
	go s.createObjectTimeoutLoop()
	return s, nil
}

func normalizeS3Endpoint(endpoint string, secure bool) (string, bool) {
	ep := strings.TrimSpace(endpoint)
	lower := strings.ToLower(ep)
	switch {
	case strings.HasPrefix(lower, "https://"):
		ep = ep[len("https://"):]
		secure = true
	case strings.HasPrefix(lower, "http://"):
		ep = ep[len("http://"):]
		secure = false
	}
	return strings.TrimSuffix(ep, "/"), secure
}

func (e *encodeS3Writer) connectS3(config *api.EncodeS3) (*minio.Client, error) {
	endpoint, secure := normalizeS3Endpoint(config.Endpoint, config.Secure)
	minioOptions := minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, string(config.SecretAccessKey), ""),
		Secure: secure,
	}
	s3Client, err := minio.New(endpoint, &minioOptions)
	if err != nil {
		s3log.Errorf("Error when creating S3 client: %v", err)
		return nil, err
	}

	found, err := s3Client.BucketExists(context.Background(), config.Bucket)
	if err != nil {
		s3log.Errorf("Error accessing S3 bucket: %v", err)
		return nil, err
	}
	if found {
		s3log.Infof("S3 Bucket %s found", config.Bucket)
	}
	return s3Client, nil
}

func (e *encodeS3Writer) putObject(bucket, objectName string, data []byte, contentType string) error {
	if e.s3Client == nil {
		s3Client, err := e.connectS3(e.s3Params)
		if s3Client == nil {
			return err
		}
		e.s3Client = s3Client
	}
	_, err := e.s3Client.PutObject(context.Background(), bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (e *encodeS3Writer) Update(_ config.StageParam) {
	s3log.Warn("Encode S3 Writer, update not supported")
}
