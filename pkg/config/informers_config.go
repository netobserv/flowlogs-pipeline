package config

import "github.com/netobserv/flowlogs-pipeline/pkg/api"

type Informers struct {
	KubeConfig      api.NetworkTransformKubeConfig `yaml:"kubeConfig"`
	KafkaConfig     api.EncodeKafka                `yaml:"kafkaConfig"`
	MetricsSettings MetricsSettings                `yaml:"metricsSettings"`
	PProfPort       int32                          `yaml:"pprofPort"`
	LogLevel        string                         `yaml:"logLevel"`
}
