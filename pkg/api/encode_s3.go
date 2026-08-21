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

package api

const (
	// S3FormatParquet is the default object format (Hive-partitioned Parquet schema v1).
	S3FormatParquet = "parquet"
	// S3FormatJSON is the legacy JSON object format (explicit opt-in).
	S3FormatJSON = "json"
)

type EncodeS3 struct {
	Account                string                 `yaml:"account" json:"account" doc:"cluster / tenant id used as Hive cluster_id partition"`
	Endpoint               string                 `yaml:"endpoint" json:"endpoint" doc:"address of s3 server"`
	AccessKeyID            string                 `yaml:"accessKeyId" json:"accessKeyId" doc:"username to connect to server"`
	SecretAccessKey        RedactedText           `yaml:"secretAccessKey" json:"secretAccessKey" doc:"password to connect to server"`
	Bucket                 string                 `yaml:"bucket" json:"bucket" doc:"bucket into which to store objects"`
	Prefix                 string                 `yaml:"prefix,omitempty" json:"prefix,omitempty" doc:"optional key prefix before cluster_id partition"`
	Format                 string                 `yaml:"format,omitempty" json:"format,omitempty" doc:"object format: parquet (default) or json"`
	WriteTimeout           Duration               `yaml:"writeTimeout,omitempty" json:"writeTimeout,omitempty" doc:"timeout for flush when batch is not full (default: 60s)"`
	BatchSize              int                    `yaml:"batchSize,omitempty" json:"batchSize,omitempty" doc:"max flows buffered before writing an object (default: 5000 parquet / 10 json)"`
	Secure                 bool                   `yaml:"secure,omitempty" json:"secure,omitempty" doc:"true for https, false for http (default: false)"`
	ObjectHeaderParameters map[string]interface{} `yaml:"objectHeaderParameters,omitempty" json:"objectHeaderParameters,omitempty" doc:"parameters to include in JSON object header (json format only)"`
}
