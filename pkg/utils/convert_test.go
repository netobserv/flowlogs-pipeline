package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConvert(t *testing.T) {
	type tt struct {
		input   interface{}
		wantf64 float64
		wantu32 uint32
		wantu64 uint64
		wanti64 int64
		wanti   int
	}
	cases := []tt{{
		input:   float64(1.1),
		wantf64: 1.1,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   float32(1.1),
		wantf64: 1.1,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   "1",
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   int32(1),
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   int64(1),
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   int(1),
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   uint32(1),
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   uint64(1),
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   uint(1),
		wantf64: 1.0,
		wantu32: 1,
		wantu64: 1,
		wanti64: 1,
		wanti:   1,
	}, {
		input:   time.Duration(42),
		wantf64: 42.0,
		wantu32: 42,
		wantu64: 42,
		wanti64: 42,
		wanti:   42,
	}}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%T", tc.input), func(t *testing.T) {
			f, err := ConvertToFloat64(tc.input)
			assert.NoError(t, err)
			assert.InDelta(t, tc.wantf64, f, 0.001, fmt.Sprintf("%T -> float64 failed", tc.input))

			u64, err := ConvertToUint64(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantu64, u64, fmt.Sprintf("%T -> uint64 failed", tc.input))

			u32, err := ConvertToUint32(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantu32, u32, fmt.Sprintf("%T -> uint32 failed", tc.input))

			i64, err := ConvertToInt64(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.wanti64, i64, fmt.Sprintf("%T -> int64 failed", tc.input))

			i, err := ConvertToInt(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.wanti, i, fmt.Sprintf("%T -> int failed", tc.input))
		})
	}
}
