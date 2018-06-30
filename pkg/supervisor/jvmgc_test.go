package supervisor

import (
	"bytes"
	"math"
	"testing"
)

func TestParseJstatStdout(t *testing.T) {
	stdout := `S0C    S1C    S0U    S1U      EC       EU        OC         OU       MC     MU    CCSC   CCSU   YGC     YGCT    FGC    FGCT     GCT
               34048.0 34048.0  0.0   17315.1 272640.0 216307.7  707840.0   194227.8  316324.0 296554.3 42592.0 38302.4    801   12.814  31      2.825   15.639
              `
	ret, err := parseJstatStdout(*bytes.NewBufferString(stdout))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]float64{
		"survivor0.capacity":              34048.0,
		"survivor1.capacity":              34048.0,
		"survivor0.usage":                 0.0,
		"survivor1.usage":                 17315.1,
		"eden.capacity":                   272640.0,
		"eden.usage":                      216307.7,
		"old.space.capacity":              707840.0,
		"old.space.usage":                 194227.8,
		"metaspace.capacity":              316324.0,
		"metaspace.usage":                 296554.3,
		"compressed.class.space.capacity": 42592.0,
		"compressed.class.space.usage":    38302.4,
		"young.gc.count":                  801,
		"young.gc.time":                   12.814,
		"full.gc.count":                   31,
		"full.gc.time":                    2.825,
		"total.gc.time":                   15.639,
	}
	if len(ret) != len(expected) {
		t.Fatalf("len of two jstat data map mimatch, %d != %d", len(ret), len(expected))
	}
	for key, val := range expected {
		if retVal, ok := ret[key]; !ok {
			t.Errorf("No found key %s in ret", key)
		} else if math.Abs(val-retVal) > 1 {
			t.Errorf("value for key %s mismatch, %f != %f", key, val, retVal)
		}
	}
}
