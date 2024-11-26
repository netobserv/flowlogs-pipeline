package filters

import (
	"regexp"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

var flow = config.GenericMap{
	"namespace":       "foo",
	"name":            "bar",
	"bytes":           15,
	"other_namespace": "foo",
}

func TestFilterPresence(t *testing.T) {
	pred := Presence("namespace")
	assert.True(t, pred(flow))

	pred = Presence("namespacezzz")
	assert.False(t, pred(flow))
}

func TestFilterAbsence(t *testing.T) {
	pred := Absence("namespace")
	assert.False(t, pred(flow))

	pred = Absence("namespacezzz")
	assert.True(t, pred(flow))
}

func TestFilterEqual(t *testing.T) {
	pred := Equal("namespace", "foo", false)
	assert.True(t, pred(flow))

	pred = Equal("namespace", "foozzz", false)
	assert.False(t, pred(flow))

	pred = Equal("namespacezzz", "foo", false)
	assert.False(t, pred(flow))

	pred = Equal("bytes", 15, false)
	assert.True(t, pred(flow))

	pred = Equal("bytes", "15", false)
	assert.False(t, pred(flow))

	pred = Equal("bytes", "15", true)
	assert.True(t, pred(flow))

	pred = Equal("namespace", "$(other_namespace)", false)
	assert.True(t, pred(flow))

	pred = Equal("namespace", "$(name)", false)
	assert.False(t, pred(flow))
}

func TestFilterNotEqual(t *testing.T) {
	pred := NotEqual("namespace", "foo", false)
	assert.False(t, pred(flow))

	pred = NotEqual("namespace", "foozzz", false)
	assert.True(t, pred(flow))

	pred = NotEqual("namespacezzz", "foo", false)
	assert.True(t, pred(flow))

	pred = NotEqual("bytes", 15, false)
	assert.False(t, pred(flow))

	pred = NotEqual("bytes", 15, true)
	assert.True(t, pred(flow))

	pred = NotEqual("bytes", "15", false)
	assert.True(t, pred(flow))

	pred = NotEqual("bytes", "15", true)
	assert.False(t, pred(flow))

	pred = NotEqual("namespace", "$(other_namespace)", false)
	assert.False(t, pred(flow))

	pred = NotEqual("namespace", "$(name)", false)
	assert.True(t, pred(flow))
}

func TestFilterRegex(t *testing.T) {
	pred := Regex("namespace", regexp.MustCompile("f.+"))
	assert.True(t, pred(flow))

	pred = Regex("namespace", regexp.MustCompile("g.+"))
	assert.False(t, pred(flow))

	pred = Regex("namespacezzz", regexp.MustCompile("f.+"))
	assert.False(t, pred(flow))

	pred = Regex("bytes", regexp.MustCompile("1[0-9]"))
	assert.True(t, pred(flow))

	pred = Regex("bytes", regexp.MustCompile("2[0-9]"))
	assert.False(t, pred(flow))
}

func TestFilterNotRegex(t *testing.T) {
	pred := NotRegex("namespace", regexp.MustCompile("f.+"))
	assert.False(t, pred(flow))

	pred = NotRegex("namespace", regexp.MustCompile("g.+"))
	assert.True(t, pred(flow))

	pred = NotRegex("namespacezzz", regexp.MustCompile("f.+"))
	assert.True(t, pred(flow))

	pred = NotRegex("bytes", regexp.MustCompile("1[0-9]"))
	assert.False(t, pred(flow))

	pred = NotRegex("bytes", regexp.MustCompile("2[0-9]"))
	assert.True(t, pred(flow))
}

func Test_Filters_extractVarLookups(t *testing.T) {
	variables := extractVarLookups("$(abc)--$(def)")

	assert.Equal(t, [][]string{{"$(abc)", "abc"}, {"$(def)", "def"}}, variables)

	variables = extractVarLookups("")
	assert.Empty(t, variables)
}
