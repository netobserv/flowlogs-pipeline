package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkGenericMap_Copy(b *testing.B) {
	m := GenericMap{}
	for i := 0; i < 20; i++ {
		m[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d", i)
	}

	for i := 0; i < b.N; i++ {
		_ = m.Copy()
	}
}

func TestGenericMap_IsDuplicate(t *testing.T) {
	table := []struct {
		name     string
		input    GenericMap
		expected bool
	}{
		{
			"Duplicate: true",
			GenericMap{duplicateFieldName: true},
			true,
		},
		{
			"Duplicate: false",
			GenericMap{duplicateFieldName: false},
			false,
		},
		{
			"Missing field",
			GenericMap{},
			false,
		},
		{
			"Convert 'true'",
			GenericMap{duplicateFieldName: "true"},
			true,
		},
		{
			"Convert 'false'",
			GenericMap{duplicateFieldName: "false"},
			false,
		},
		{
			"Conversion failure: 'maybe'",
			GenericMap{duplicateFieldName: "maybe"},
			false,
		},
	}

	for _, testCase := range table {
		t.Run(testCase.name, func(tt *testing.T) {
			actual := testCase.input.IsDuplicate()
			require.Equal(tt, testCase.expected, actual)
		})
	}
}
