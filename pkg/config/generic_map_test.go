package config

import (
	"fmt"
	"testing"
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
