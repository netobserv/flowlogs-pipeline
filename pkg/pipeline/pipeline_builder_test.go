package pipeline

import (
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const baseConfig = `parameters:
- name: ingest1
  ingest:
    type: file
    file:
      filename: ../../hack/examples/ocp-ipfix-flowlogs.json
- decode:
    type: none
  name: decode1
- write:
    type: none
  name: write1
`

func TestConnectionVerification_Pass(t *testing.T) {
	test.InitConfig(t, baseConfig+`pipeline:
- { follows: ingest1, name: decode1 }
- { follows: decode1, name: write1 }
`)
	_, err := NewPipeline()
	assert.NoError(t, err)
}

func TestConnectionVerification(t *testing.T) {
	type testCase struct {
		description     string
		config          string
		failingNodeName string
	}
	for _, tc := range []testCase{{
		description: "ingest does not have outputs",
		config: baseConfig + `- transform:
    type: none
  name: transform1
- transform:
    type: none
  name: transform2
pipeline:
- name: ingest1
- { follows: decode1, name: transform1 }
- { follows: transform1, name: transform2 }
- { follows: transform2, name: transform1 }
- { follows: decode1, name: write1 }
`,
		failingNodeName: "ingest1",
	}, {
		description: "middle node decode1 does not have inputs",
		config: baseConfig + `- name: decode2
  decode:
    type: none
pipeline:
- { follows: ingest1, name: decode2 }
- { follows: decode1, name: write1 }
- { follows: decode2, name: write1 }
`,
		failingNodeName: "decode1",
	}, {
		description: "middle node decode1 does not have outputs",
		config: baseConfig + `- name: decode2
  decode:
    type: none
pipeline:
- { follows: ingest1, name: decode1 }
- { follows: ingest1, name: decode2 }
- { follows: decode2, name: write1 }
`,
		failingNodeName: "decode1",
	}, {
		description: "terminal node write1 does not have inputs",
		config: baseConfig + `- transform:
    type: none
  name: transform1
pipeline:
- { follows: ingest1, name: decode1 }
- { follows: decode1, name: transform1 }
`,
		failingNodeName: "write1",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			test.InitConfig(t, tc.config)
			_, err := NewPipeline()
			require.Error(t, err)
			require.IsType(t, &Error{}, err, err.Error())
			assert.Equal(t, tc.failingNodeName, err.(*Error).StageName, err.Error())
		})
	}
}
