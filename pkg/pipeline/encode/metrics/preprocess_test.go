package metrics

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
)

func Test_Flatten(t *testing.T) {
	pp := Preprocess(&api.MetricsItem{Flatten: []string{"interfaces", "events"}})
	fl := pp.GenerateFlatParts(config.GenericMap{
		"namespace":  "A",
		"interfaces": []string{"eth0", "123456"},
		"events": []any{
			map[string]string{"type": "egress", "name": "my_egress"},
			map[string]string{"type": "acl", "name": "my_policy"},
		},
		"bytes": 7,
	})
	assert.Equal(t, []config.GenericMap{
		{
			"interfaces":  "eth0",
			"events>type": "egress",
			"events>name": "my_egress",
		},
		{
			"interfaces":  "123456",
			"events>type": "egress",
			"events>name": "my_egress",
		},
		{
			"interfaces":  "eth0",
			"events>type": "acl",
			"events>name": "my_policy",
		},
		{
			"interfaces":  "123456",
			"events>type": "acl",
			"events>name": "my_policy",
		},
	}, fl)
}
