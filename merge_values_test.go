package main

import (
	"reflect"
	"testing"
)

func TestMergeValues(t *testing.T) {
	a := map[interface{}]interface{}{
		"one": map[interface{}]interface{}{
			"from-a":    []interface{}{"one", "two", "three"},
			"from-both": []interface{}{"one", "three", "five", "seven", "nine", "eleven"},
		},
		"two":   2,
		"three": "3",
	}

	b := map[interface{}]interface{}{
		"one": map[interface{}]interface{}{
			"from-b":    []interface{}{"one", "two", "three"},
			"from-both": []interface{}{"two", "four", "six", "eight", "ten", "twelve"},
		},
		"three": 3,
		"four":  4,
	}

	expectedRes := map[interface{}]interface{}{
		"one": map[interface{}]interface{}{
			"from-a":    []interface{}{"one", "two", "three"},
			"from-b":    []interface{}{"one", "two", "three"},
			"from-both": []interface{}{"one", "three", "five", "seven", "nine", "eleven", "two", "four", "six", "eight", "ten", "twelve"},
		},
		"two":   2,
		"three": 3,
		"four":  4,
	}

	res := MergeValues(a, b)

	if !reflect.DeepEqual(res, expectedRes) {
		t.Errorf("MERGE FAILED\nMAP-1:\n%v\nMAP-2:\n%v\nEXPECTED:\n%v\nGOT:\n%v", a, b, expectedRes, res)
	}
}
