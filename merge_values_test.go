package main

import (
	"reflect"
	"testing"
)

func TestMergeValues(t *testing.T) {
	a := map[string]interface{}{
		"one": map[string]interface{}{
			"from-a":    []string{"one", "two", "three"},
			"from-both": []string{"one", "three", "five", "seven", "nine", "eleven"},
		},
		"two":   2,
		"three": "3",
	}

	b := map[string]interface{}{
		"one": map[string]interface{}{
			"from-b":    []string{"one", "two", "three"},
			"from-both": []string{"two", "four", "six", "eight", "ten", "twelve"},
		},
		"three": 3,
		"four":  4,
	}

	expectedRes := map[string]interface{}{
		"one": map[string]interface{}{
			"from-a":    []string{"one", "two", "three"},
			"from-b":    []string{"one", "two", "three"},
			"from-both": []string{"one", "three", "five", "seven", "nine", "eleven", "two", "four", "six", "eight", "ten", "twelve"},
		},
		"two":   2,
		"three": 3,
		"four":  4,
	}

	res := MergeValues(a, b)

	if !reflect.DeepEqual(res, expectedRes) {
		t.Errorf("Merge %v with %v failed: expected %v, got %v", a, b, expectedRes, res)
	}
}
