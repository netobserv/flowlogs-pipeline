package dsl

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestAndOrEqual(t *testing.T) {
	predicate, err := Parse(`(srcnamespace="netobserv" OR (srcnamespace="ingress" AND dstnamespace="netobserv")) AND srckind!="service"`)
	assert.NoError(t, err)
	assert.NotNil(t, predicate)

	result := predicate(config.GenericMap{
		"srcnamespace": "plop",
		"dstnamespace": "netobserv",
		"srckind":      "pod",
	})
	assert.False(t, result)

	result = predicate(config.GenericMap{
		"srcnamespace": "ingress",
		"dstnamespace": "netobserv",
		"srckind":      "pod",
	})
	assert.True(t, result)

	result = predicate(config.GenericMap{
		"srcnamespace": "ingress",
		"dstnamespace": "netobserv",
		"srckind":      "service",
	})
	assert.False(t, result)
}

func TestRegexp(t *testing.T) {
	predicate, err := Parse(`srcnamespace=~"openshift.*" and dstnamespace!~"openshift.*"`)
	assert.NoError(t, err)
	assert.NotNil(t, predicate)

	result := predicate(config.GenericMap{
		"srcnamespace": "openshift-ingress",
		"dstnamespace": "my-app",
		"srckind":      "pod",
	})
	assert.True(t, result, "Should accept flows from OpenShift to App")

	result = predicate(config.GenericMap{
		"srcnamespace": "my-app",
		"dstnamespace": "openshift-ingress",
		"srckind":      "pod",
	})
	assert.False(t, result, "Should reject flows from App to OpenShift")

	result = predicate(config.GenericMap{
		"srcnamespace": "my-app",
		"dstnamespace": "my-app",
		"srckind":      "pod",
	})
	assert.False(t, result, "Should reject flows from App to App")

	result = predicate(config.GenericMap{
		"srcnamespace": "openshift-operators",
		"dstnamespace": "openshift-ingress",
		"srckind":      "pod",
	})
	assert.False(t, result, "Should reject flows from OpenShift to OpenShift")
}

func TestWith(t *testing.T) {
	predicate, err := Parse(`srcnamespace="foo" and with(rtt)`)
	assert.NoError(t, err)
	assert.NotNil(t, predicate)

	result := predicate(config.GenericMap{
		"srcnamespace": "foo",
		"rtt":          4.5,
	})
	assert.True(t, result, "Should accept flows from foo with rtt")

	result = predicate(config.GenericMap{
		"srcnamespace": "foo",
	})
	assert.False(t, result, "Should reject flows from foo without rtt")
}

func TestWithout(t *testing.T) {
	predicate, err := Parse(`srcnamespace="foo" or without(srcnamespace)`)
	assert.NoError(t, err)
	assert.NotNil(t, predicate)

	result := predicate(config.GenericMap{
		"srcnamespace": "foo",
	})
	assert.True(t, result, "Should accept flows from foo")

	result = predicate(config.GenericMap{})
	assert.True(t, result, "Should accept flows without srcnamespace")

	result = predicate(config.GenericMap{
		"srcnamespace": "bar",
	})
	assert.False(t, result, "Should reject flows from bar")
}

func TestNumeric(t *testing.T) {
	predicate, err := Parse(`flowdirection=0 and bytes > 15 and packets <= 2`)
	assert.NoError(t, err)
	assert.NotNil(t, predicate)

	result := predicate(config.GenericMap{
		"srcnamespace":  "plop",
		"flowdirection": 0,
		"bytes":         20,
		"packets":       1,
	})
	assert.True(t, result)

	result = predicate(config.GenericMap{
		"srcnamespace":  "plop",
		"flowdirection": int16(0),
		"bytes":         int16(20),
		"packets":       int16(1),
	})
	assert.True(t, result)

	result = predicate(config.GenericMap{
		"srcnamespace":  "plop",
		"flowdirection": 1,
		"bytes":         20,
		"packets":       1,
	})
	assert.False(t, result)

	result = predicate(config.GenericMap{
		"srcnamespace":  "plop",
		"flowdirection": 0,
		"bytes":         10,
		"packets":       1,
	})
	assert.False(t, result)

	result = predicate(config.GenericMap{
		"srcnamespace": "plop",
		"bytes":        20,
		"packets":      1,
	})
	assert.False(t, result)

	result = predicate(config.GenericMap{
		"flowdirection": 0,
		"bytes":         20,
		"packets":       2,
	})
	assert.True(t, result)

	result = predicate(config.GenericMap{
		"flowdirection": 0,
		"bytes":         15,
		"packets":       2,
	})
	assert.False(t, result)

	result = predicate(config.GenericMap{
		"flowdirection": 0,
		"bytes":         20,
		"packets":       3,
	})
	assert.False(t, result)
}
