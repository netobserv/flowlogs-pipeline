package pipeline

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInProcessFLP(t *testing.T) {
	pipeline := config.NewPresetIngesterPipeline()
	pipeline = pipeline.WriteStdout("writer", api.WriteStdout{Format: "json"})
	in := make(chan config.GenericMap, 100)
	defer close(in)
	err := StartFLPInProcess(pipeline.ToConfigFileStruct(), in)
	require.NoError(t, err)

	capturedOut, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	// yield thread to allow pipe services correctly start
	time.Sleep(10 * time.Millisecond)

	in <- config.GenericMap{
		"SrcAddr": "1.2.3.4",
		"DstAddr": "5.6.7.8",
		"Dscp":    float64(1),
		"DstMac":  "11:22:33:44:55:66",
		"SrcMac":  "01:02:03:04:05:06",
	}

	scanner := bufio.NewScanner(capturedOut)
	require.True(t, scanner.Scan())
	capturedRecord := map[string]interface{}{}
	bytes := scanner.Bytes()
	require.NoError(t, json.Unmarshal(bytes, &capturedRecord), string(bytes))

	assert.EqualValues(t, map[string]interface{}{
		"SrcAddr": "1.2.3.4",
		"DstAddr": "5.6.7.8",
		"Dscp":    float64(1),
		"DstMac":  "11:22:33:44:55:66",
		"SrcMac":  "01:02:03:04:05:06",
	}, capturedRecord)
}
