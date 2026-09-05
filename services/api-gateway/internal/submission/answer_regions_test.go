package submission

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDecodeAnswerRegionBBoxSupportsLegacyArrayAndPixelObject(t *testing.T) {
	tests := []struct {
		raw  string
		want []float64
	}{
		{`[10,20,100,40]`, []float64{10, 20, 100, 40}},
		{`{"x":11,"y":21,"width":101,"height":41}`, []float64{11, 21, 101, 41}},
		{`{"x":12,"y":22,"w":102,"h":42}`, []float64{12, 22, 102, 42}},
		{`{"not":"a bbox"}`, []float64{}},
	}
	for _, test := range tests {
		got := decodeAnswerRegionBBox(json.RawMessage(test.raw))
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("decodeAnswerRegionBBox(%s)=%v want %v", test.raw, got, test.want)
		}
	}
}
