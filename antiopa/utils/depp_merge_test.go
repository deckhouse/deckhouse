package utils

import (
	"reflect"
	"testing"
)

func TestDeepMerge(t *testing.T) {
	data := make(map[string]map[interface{}]interface{})
	for _, key := range []string{"a", "original-a"} {
		data[key] = map[interface{}]interface{}{
			"one": map[interface{}]interface{}{
				"from-a":    []interface{}{"one", "two", "three"},
				"from-both": []interface{}{"one", "three", "five", "seven", "nine", "eleven"},
			},
			"two":   2,
			"three": "3",
			"array": []string{"a", "b", "c"},
		}
	}
	for _, key := range []string{"b", "original-b"} {
		data[key] = map[interface{}]interface{}{
			"one": map[interface{}]interface{}{
				"from-b":    []interface{}{"one", "two", "three"},
				"from-both": []interface{}{"two", "four", "six", "eight", "ten", "twelve"},
			},
			"three": 3,
			"four":  4,
			"array": []int{1, 2, 3},
		}
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
		"array": []int{1, 2, 3},
	}

	res := DeepMerge(data["a"], data["b"])

	if !reflect.DeepEqual(res, expectedRes) {
		t.Errorf("DeepMerge FAILED\nmap-1:\n%v\nmap-2:\n%v\nEXPECTED:\n%v\nGOT:\n%v", data["a"], data["b"], expectedRes, res)
	}

	// Проверяем, что функция DeepMerge не меняет входные данные-параметры
	if !reflect.DeepEqual(data["a"], data["original-a"]) {
		t.Errorf("DeepMerge FAILED: function changed input params\noriginal map a:\n%v\nmap a after call:\n%v", data["original-a"], data["a"])
	}
	if !reflect.DeepEqual(data["b"], data["original-b"]) {
		t.Errorf("DeepMerge FAILED: function changed input params\noriginal map b:\n%v\nmap b after call:\n%v", data["original-b"], data["b"])
	}
}

func TestMergeValuesWithNullValue(t *testing.T) {
	res := DeepMerge(
		map[interface{}]interface{}{
			"hello": map[interface{}]interface{}{
				"bbb": nil,
			},
		},
		map[interface{}]interface{}{
			"hello": map[interface{}]interface{}{
				"bbb": nil,
			},
		})

	if !reflect.DeepEqual(res, map[interface{}]interface{}{
		"hello": map[interface{}]interface{}{
			"bbb": nil,
		},
	}) {
		t.Errorf("MergeValues FAILED: got unexpected merge result: %v", res)
	}
}
