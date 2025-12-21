/*
 * Copyright (C) 2024 IBM, Inc.
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

package transform

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	defaultAnomalyWindow      = 30
	defaultAnomalySensitivity = 3.0
)

var anomalyLog = logrus.WithField("component", "transform.Anomaly")

type anomalyState struct {
	values      []float64
	sum         float64
	sumSq       float64
	baseline    float64
	initialized bool
}

func (s *anomalyState) addValue(v float64, window int) {
	s.values = append(s.values, v)
	s.sum += v
	s.sumSq += v * v
	if len(s.values) > window {
		oldest := s.values[0]
		s.values = s.values[1:]
		s.sum -= oldest
		s.sumSq -= oldest * oldest
	}
}

func (s *anomalyState) mean() float64 {
	if len(s.values) == 0 {
		return 0
	}
	return s.sum / float64(len(s.values))
}

func (s *anomalyState) stddev() float64 {
	if len(s.values) < 2 {
		return 0
	}
	mean := s.mean()
	variance := (s.sumSq / float64(len(s.values))) - (mean * mean)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

type Anomaly struct {
	mu             sync.Mutex
	states         map[string]*anomalyState
	config         api.TransformAnomaly
	windowSize     int
	baselineWindow int
	sensitivity    float64
	alpha          float64
	outputPrefix   string
	opMetrics      *operational.Metrics
	errorsCounter  *prometheus.CounterVec
}
var anomalyErrorsCounter = operational.DefineMetric(
	"transform_anomaly_errors",
	"Counter of errors during anomaly transformation",
	operational.TypeCounter,
	"type", "field",
)
// NewTransformAnomaly creates a new anomaly transformer.
func NewTransformAnomaly(params config.StageParam, opMetrics *operational.Metrics) (Transformer, error) {
	anomalyConfig := api.TransformAnomaly{}
	if params.Transform != nil && params.Transform.Anomaly != nil {
		anomalyConfig = *params.Transform.Anomaly
	}
	if anomalyConfig.ValueField == "" {
		return nil, fmt.Errorf("valueField must be provided for anomaly transform")
	}

	window := anomalyConfig.WindowSize
	if window <= 0 {
		window = defaultAnomalyWindow
	}
	baselineWindow := anomalyConfig.BaselineWindow
	if baselineWindow <= 0 {
		baselineWindow = window / 2
		if baselineWindow == 0 {
			baselineWindow = 1
		}
	}
	sensitivity := anomalyConfig.Sensitivity
	if sensitivity <= 0 {
		sensitivity = defaultAnomalySensitivity
	}
	alpha := anomalyConfig.EWMAAlpha
	if alpha <= 0 {
		alpha = 2.0 / (float64(window) + 1.0)
	}
	if len(anomalyConfig.KeyFields) == 0 {
		anomalyConfig.KeyFields = []string{"SrcAddr", "DstAddr", "Proto"}
	}
	if anomalyConfig.Algorithm == "" {
		anomalyConfig.Algorithm = api.AnomalyAlgorithmZScore
	}

	outputPrefix := anomalyConfig.Prefix
    	anomalyLog.Infof("NewTransformAnomaly algorithm=%s window=%d baselineWindow=%d prefix=%q", anomalyConfig.Algorithm, window, baselineWindow, outputPrefix)
	return &Anomaly{
		states:         make(map[string]*anomalyState),
		config:         anomalyConfig,
		windowSize:     window,
		baselineWindow: baselineWindow,
		sensitivity:    sensitivity,
		alpha:          alpha,
		outputPrefix:   outputPrefix,
		opMetrics:      opMetrics,
		errorsCounter:  opMetrics.NewCounterVec(&anomalyErrorsCounter),
	}, nil
}

// Transform calculates anomaly scores per key and appends anomaly fields.
func (a *Anomaly) Transform(entry config.GenericMap) (config.GenericMap, bool) {
	value, err := utils.ConvertToFloat64(entry[a.config.ValueField])
	if err != nil {
	    if a.errorsCounter != nil {
        			a.errorsCounter.WithLabelValues("ValueConversionError", a.config.ValueField).Inc()
        		}
		return entry, false
	}
	key := a.buildKey(entry)

	a.mu.Lock()
	state, ok := a.states[key]
	if !ok {
		state = &anomalyState{}
		a.states[key] = state
	}
	anomalyType, score := a.score(state, value)
	state.addValue(value, a.windowSize)
	stateSize := len(state.values)
	a.mu.Unlock()

	output := entry.Copy()
	output[a.outputField("anomaly_score")] = score
	output[a.outputField("anomaly_type")] = anomalyType
	output[a.outputField("baseline_window")] = stateSize

	return output, true
}

func (a *Anomaly) score(state *anomalyState, value float64) (string, float64) {
	if len(state.values) < a.baselineWindow {
		if !state.initialized {
			state.baseline = value
			state.initialized = true
		}
		return "warming_up", 0
	}

	switch a.config.Algorithm {
	case api.AnomalyAlgorithmEWMA:
		return a.scoreEWMA(state, value)
	case api.AnomalyAlgorithmZScore:
		fallthrough
	default:
		return a.scoreZScore(state, value)
	}
}

func (a *Anomaly) scoreEWMA(state *anomalyState, value float64) (string, float64) {
	if !state.initialized {
		state.baseline = value
		state.initialized = true
	}
	deviation := value - state.baseline
	stddev := state.stddev()
	if stddev == 0 {
		stddev = math.Max(math.Abs(state.baseline)*1e-6, 1e-9)
	}
	score := math.Abs(deviation) / stddev
	state.baseline = state.baseline + a.alpha*(value-state.baseline)
	anomalyType := "normal"
	if score >= a.sensitivity {
		if deviation > 0 {
			anomalyType = "ewma_high"
		} else {
			anomalyType = "ewma_low"
		}
	}
	return anomalyType, score
}

func (a *Anomaly) scoreZScore(state *anomalyState, value float64) (string, float64) {
	mean := state.mean()
	stddev := state.stddev()
	if stddev == 0 {
		stddev = math.Max(math.Abs(mean)*1e-6, 1e-9)
	}
	score := math.Abs(value-mean) / stddev
	anomalyType := "normal"
	if score >= a.sensitivity {
		if value > mean {
			anomalyType = "zscore_high"
		} else {
			anomalyType = "zscore_low"
		}
	}
	return anomalyType, score
}

func (a *Anomaly) buildKey(entry config.GenericMap) string {
	parts := make([]string, 0, len(a.config.KeyFields))
	for _, key := range a.config.KeyFields {
		if val, ok := entry[key]; ok {
			parts = append(parts, utils.ConvertToString(val))
		} else {
			parts = append(parts, "<missing>")
		}
	}
	return strings.Join(parts, "|")
}
func (a *Anomaly) outputField(name string) string {
	return a.outputPrefix + name
}

// Reset clears the internal state; useful for tests.
func (a *Anomaly) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.states = make(map[string]*anomalyState)
}
