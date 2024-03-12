package pipeline

import (
	"errors"
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
      decoder:
        type: json
- write:
    type: none
  name: write1
`

func TestConnectionVerification_Pass(t *testing.T) {
	_, cfg := test.InitConfig(t, baseConfig+`pipeline:
- { follows: ingest1, name: write1 }
`)
	_, err := NewPipeline(cfg)
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
- { follows: transform1, name: transform2 }
- { follows: transform2, name: transform1 }
- { follows: transform1, name: write1 }
`,
		failingNodeName: "ingest1",
	}, {
		description: "middle node transform1 does not have inputs",
		config: baseConfig + `- transform:
    type: none
  name: transform2
- transform:
    type: none
  name: transform1
pipeline:
- { follows: ingest1, name: transform2 }
- { follows: transform1, name: write1 }
- { follows: transform2, name: write1 }
`,
		failingNodeName: "transform1",
	}, {
		description: "middle node transform1 does not have outputs",
		config: baseConfig + `- transform:
    type: none
  name: transform2
- transform:
    type: none
  name: transform1
pipeline:
- { follows: ingest1, name: transform1 }
- { follows: ingest1, name: transform2 }
- { follows: transform2, name: write1 }
`,
		failingNodeName: "transform1",
	}, {
		description: "terminal node write1 does not have inputs",
		config: baseConfig + `- transform:
    type: none
  name: transform1
pipeline:
- { follows: ingest1, name: transform1 }
`,
		failingNodeName: "write1",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			_, cfg := test.InitConfig(t, tc.config)
			_, err := NewPipeline(cfg)
			require.Error(t, err)
			var castErr *Error
			require.True(t, errors.As(err, &castErr), err.Error())
			assert.Equal(t, tc.failingNodeName, castErr.StageName, err.Error())
		})
	}
}
