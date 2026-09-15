package api

// TransformAnomalyAlgorithm defines the supported anomaly detection strategies.
// For doc generation, enum definitions must match format `Constant Type = "value" // doc`
type TransformAnomalyAlgorithm string

const (
	AnomalyAlgorithmEWMA   TransformAnomalyAlgorithm = "ewma"   // exponentially weighted moving average baseline
	AnomalyAlgorithmZScore TransformAnomalyAlgorithm = "zscore" // rolling z-score over a sliding window
)

// TransformAnomaly describes configuration for anomaly detection stages.
type TransformAnomaly struct {
	Algorithm      TransformAnomalyAlgorithm `yaml:"algorithm,omitempty" json:"algorithm,omitempty" doc:"(enum) algorithm used to score anomalies: ewma or zscore"`
	ValueField     string                    `yaml:"valueField,omitempty" json:"valueField,omitempty" doc:"field containing the numeric value to evaluate"`
	KeyFields      []string                  `yaml:"keyFields,omitempty" json:"keyFields,omitempty" doc:"list of fields combined to build the per-entity baseline key"`
	Prefix         string                    `yaml:"prefix,omitempty" json:"prefix,omitempty" doc:"prefix added to output fields to disambiguate when multiple anomaly stages are used"`
	WindowSize     int                       `yaml:"windowSize,omitempty" json:"windowSize,omitempty" doc:"number of recent samples to keep for baseline statistics"`
	BaselineWindow int                       `yaml:"baselineWindow,omitempty" json:"baselineWindow,omitempty" doc:"minimum number of samples before anomaly scores are emitted"`
	Sensitivity    float64                   `yaml:"sensitivity,omitempty" json:"sensitivity,omitempty" doc:"threshold multiplier for flagging anomalies (e.g., z-score)"`
	EWMAAlpha      float64                   `yaml:"ewmaAlpha,omitempty" json:"ewmaAlpha,omitempty" doc:"smoothing factor for ewma algorithm; derived from windowSize if omitted"`
}
