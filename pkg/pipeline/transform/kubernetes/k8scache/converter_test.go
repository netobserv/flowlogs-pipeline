package k8scache

import (
	"reflect"
	"testing"

	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestConverter_RoundtripPreservesAllFields is a guard against forgetting to wire
// a new model.ResourceMetaData field into converter.go (as happened with the
// Terminated field). It fills every direct field of ResourceMetaData with a
// non-zero value via reflection, converts to a gRPC ResourceEntry and back, then
// checks that no field was lost. Adding a new field to ResourceMetaData that is
// not mapped by the converter makes this test fail with no change to the test
// itself: the unmapped field comes back as its zero value and the reflection loop
// catches the mismatch.
//
// The embedded metav1.ObjectMeta is only partially mapped by the converter on
// purpose (to save memory we keep just a few fields), so its mapped subset is
// filled and asserted explicitly rather than by reflection.
func TestConverter_RoundtripPreservesAllFields(t *testing.T) {
	meta := &model.ResourceMetaData{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-pod",
			Namespace:         "test-ns",
			UID:               "pod-uid",
			ResourceVersion:   "42",
			CreationTimestamp: metav1.Unix(1000, 0),
			Labels:            map[string]string{"app": "web"},
			Annotations:       map[string]string{"desc": "test"},
		},
	}
	fillDirectFields(t, reflect.ValueOf(meta).Elem())

	got := resourceEntryToMeta(metaToResourceEntry(meta))
	require.NotNil(t, got)

	// Mapped ObjectMeta subset.
	assert.Equal(t, meta.Name, got.Name)
	assert.Equal(t, meta.Namespace, got.Namespace)
	assert.Equal(t, meta.UID, got.UID)
	assert.Equal(t, meta.ResourceVersion, got.ResourceVersion)
	assert.Equal(t, meta.CreationTimestamp.Unix(), got.CreationTimestamp.Unix())
	assert.Equal(t, meta.Labels, got.Labels)
	assert.Equal(t, meta.Annotations, got.Annotations)

	// Every direct (non-embedded) field must survive the roundtrip.
	origVal := reflect.ValueOf(*meta)
	gotVal := reflect.ValueOf(*got)
	tp := origVal.Type()
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.Anonymous {
			continue // ObjectMeta, asserted above
		}
		assert.Equal(t, origVal.Field(i).Interface(), gotVal.Field(i).Interface(),
			"field %q was not preserved by the ResourceMetaData<->ResourceEntry conversion; "+
				"did you forget to wire it into converter.go?", f.Name)
	}
}

// fillDirectFields sets every direct (non-embedded, exported) field of the struct
// to a non-zero value using reflection.
func fillDirectFields(t *testing.T, v reflect.Value) {
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		f := tp.Field(i)
		if f.Anonymous || f.PkgPath != "" { // skip embedded structs and unexported fields
			continue
		}
		setNonZero(t, v.Field(i), f.Name)
	}
}

// setNonZero assigns a deterministic non-zero value to v. It fails the test if it
// encounters a field kind it does not know how to fill, so that adding a field of
// a new kind forces this helper (and the converter) to be revisited.
func setNonZero(t *testing.T, v reflect.Value, name string) {
	//nolint:exhaustive
	switch v.Kind() {
	case reflect.String:
		v.SetString("val-" + name)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Slice:
		elem := reflect.MakeSlice(v.Type(), 1, 1)
		setNonZero(t, elem.Index(0), name)
		v.Set(elem)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		setNonZero(t, key, name+"-key")
		val := reflect.New(v.Type().Elem()).Elem()
		setNonZero(t, val, name+"-val")
		m.SetMapIndex(key, val)
		v.Set(m)
	default:
		t.Fatalf("field %q has unhandled kind %s; extend setNonZero (and check converter.go covers it)", name, v.Kind())
	}
}
