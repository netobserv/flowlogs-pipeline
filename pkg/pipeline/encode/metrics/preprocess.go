package metrics

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/utils"
)

type Predicate func(config.GenericMap) bool

var variableExtractor = regexp.MustCompile(`\$\(([^\)]+)\)`)

type Preprocessed struct {
	*api.MetricsItem
	filters         []preprocessedFilter
	MappedLabels    []MappedLabel
	FlattenedLabels []MappedLabel
}

type MappedLabel struct {
	Source string
	Target string
}

type preprocessedFilter struct {
	predicate Predicate
	useFlat   bool
}

func (p *Preprocessed) TargetLabels() []string {
	var targetLabels []string
	for _, l := range p.FlattenedLabels {
		targetLabels = append(targetLabels, l.Target)
	}
	for _, l := range p.MappedLabels {
		targetLabels = append(targetLabels, l.Target)
	}
	return targetLabels
}

func Presence(filter api.MetricsFilter) Predicate {
	return func(flow config.GenericMap) bool {
		_, found := flow[filter.Key]
		return found
	}
}

func Absence(filter api.MetricsFilter) Predicate {
	pred := Presence(filter)
	return func(flow config.GenericMap) bool { return !pred(flow) }
}

func Equal(filter api.MetricsFilter) Predicate {
	varLookups := extractVarLookups(filter.Value)
	return func(flow config.GenericMap) bool {
		if val, found := flow[filter.Key]; found {
			sVal, ok := val.(string)
			if !ok {
				sVal = fmt.Sprint(val)
			}
			value := filter.Value
			if len(varLookups) > 0 {
				value = injectVars(flow, value, varLookups)
			}
			return sVal == value
		}
		return false
	}
}

func NotEqual(filter api.MetricsFilter) Predicate {
	pred := Equal(filter)
	return func(flow config.GenericMap) bool { return !pred(flow) }
}

func Regex(filter api.MetricsFilter) Predicate {
	r, _ := regexp.Compile(filter.Value)
	return func(flow config.GenericMap) bool {
		if val, found := flow[filter.Key]; found {
			sVal, ok := val.(string)
			if !ok {
				sVal = fmt.Sprint(val)
			}
			return r.MatchString(sVal)
		}
		return false
	}
}

func NotRegex(filter api.MetricsFilter) Predicate {
	pred := Regex(filter)
	return func(flow config.GenericMap) bool { return !pred(flow) }
}

func filterToPredicate(filter api.MetricsFilter) Predicate {
	switch filter.Type {
	case api.MetricFilterEqual:
		return Equal(filter)
	case api.MetricFilterNotEqual:
		return NotEqual(filter)
	case api.MetricFilterPresence:
		return Presence(filter)
	case api.MetricFilterAbsence:
		return Absence(filter)
	case api.MetricFilterRegex:
		return Regex(filter)
	case api.MetricFilterNotRegex:
		return NotRegex(filter)
	}
	// Default = Exact
	return Equal(filter)
}

func extractVarLookups(value string) [][]string {
	// Extract list of variables to lookup
	// E.g: filter "$(SrcAddr):$(SrcPort)" would return [SrcAddr,SrcPort]
	if len(value) > 0 {
		return variableExtractor.FindAllStringSubmatch(value, -1)
	}
	return nil
}

func injectVars(flow config.GenericMap, filterValue string, varLookups [][]string) string {
	injected := filterValue
	for _, matchGroup := range varLookups {
		var value string
		if rawVal, found := flow[matchGroup[1]]; found {
			if sVal, ok := rawVal.(string); ok {
				value = sVal
			} else {
				value = utils.ConvertToString(rawVal)
			}
		}
		injected = strings.ReplaceAll(injected, matchGroup[0], value)
	}
	return injected
}

func Preprocess(def *api.MetricsItem) *Preprocessed {
	mi := Preprocessed{
		MetricsItem: def,
	}
	for _, l := range def.Labels {
		ml := MappedLabel{Source: l, Target: l}
		if as := def.Remap[l]; as != "" {
			ml.Target = as
		}
		if mi.isFlattened(l) {
			mi.FlattenedLabels = append(mi.FlattenedLabels, ml)
		} else {
			mi.MappedLabels = append(mi.MappedLabels, ml)
		}
	}
	for _, f := range def.Filters {
		mi.filters = append(mi.filters, preprocessedFilter{
			predicate: filterToPredicate(f),
			useFlat:   mi.isFlattened(f.Key),
		})
	}
	return &mi
}

func (p *Preprocessed) isFlattened(fieldPath string) bool {
	for _, flat := range p.Flatten {
		if fieldPath == flat || strings.HasPrefix(fieldPath, flat+">") {
			return true
		}
	}
	return false
}
