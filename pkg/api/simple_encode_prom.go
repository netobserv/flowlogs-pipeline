package api

type SimplePromEncode struct {
	Metrics []SimplePromMetricsItem `yaml:"metrics" json:"metrics,omitempty" doc:"list of prometheus metric definitions, each includes:"`
	Port    int                     `yaml:"port" json:"port,omitempty" doc:"port number to expose \"/metrics\" endpoint"`
	Prefix  string                  `yaml:"prefix" json:"prefix,omitempty" doc:"prefix added to each metric name"`
}

type SimplePromMetricsItem struct {
	Name      string    `yaml:"name" json:"name" doc:"the metric name"`
	RecordKey string    `yaml:"recordKey,omitempty" json:"recordKey,omitempty" doc:"internal field on which to perform the operation"`
	Type      string    `yaml:"type" json:"type" enum:"PromEncodeOperationEnum" doc:"metric type, one of the following:"`
	Labels    []string  `yaml:"labels" json:"labels" doc:"labels to be associated with the metric"`
	Buckets   []float64 `yaml:"buckets" json:"buckets" doc:"histogram buckets"`
}
