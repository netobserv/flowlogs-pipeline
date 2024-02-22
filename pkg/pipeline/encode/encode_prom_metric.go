package encode

import (
	"fmt"
	"regexp"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

type Predicate func(flow config.GenericMap) bool

type MetricInfo struct {
	api.MetricsItem
	FilterPredicates []Predicate
}

func presence(filter api.MetricsFilter) Predicate {
	return func(flow config.GenericMap) bool {
		_, found := flow[filter.Key]
		return found
	}
}

func absence(filter api.MetricsFilter) Predicate {
	return func(flow config.GenericMap) bool {
		_, found := flow[filter.Key]
		return !found
	}
}

func exact(filter api.MetricsFilter) Predicate {
	return func(flow config.GenericMap) bool {
		return matchExactly(flow, filter)
	}
}

func different(filter api.MetricsFilter) Predicate {
	return func(flow config.GenericMap) bool {
		return !matchExactly(flow, filter)
	}
}

func regex(filter api.MetricsFilter) Predicate {
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

func filterToPredicate(filter api.MetricsFilter) Predicate {
	switch filter.Type {
	case api.PromFilterExact:
		return exact(filter)
	case api.PromFilterDifferent:
		return different(filter)
	case api.PromFilterPresence:
		return presence(filter)
	case api.PromFilterAbsence:
		return absence(filter)
	case api.PromFilterRegex:
		return regex(filter)
	}
	// Default = Exact
	return exact(filter)
}

func CreateMetricInfo(def api.MetricsItem) *MetricInfo {
	mi := MetricInfo{
		MetricsItem: def,
	}
	for _, f := range def.GetFilters() {
		mi.FilterPredicates = append(mi.FilterPredicates, filterToPredicate(f))
	}
	return &mi
}

func matchExactly(flow config.GenericMap, filter api.MetricsFilter) bool {
	if val, found := flow[filter.Key]; found {
		sVal, ok := val.(string)
		if !ok {
			sVal = fmt.Sprint(val)
		}
		return sVal == filter.Value
	}
	return false
}
