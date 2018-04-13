package utils

import (
	"fmt"
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

func castMapShouldEqual(input interface{}, expectedResult map[interface{}]interface{}) error {
	res := CastArbitraryMap(input)

	if !reflect.DeepEqual(res, expectedResult) {
		return fmt.Errorf("expected %+v map after cast, got %+v", expectedResult, res)
	}

	return nil
}

func castSliceShouldEqual(input interface{}, expectedResult []interface{}) error {
	res := CastArbitrarySlice(input)

	if !reflect.DeepEqual(res, expectedResult) {
		return fmt.Errorf("expected %+v slice after cast, got %+v", expectedResult, res)
	}

	return nil
}

func TestCast(t *testing.T) {
	var err error

	err = castMapShouldEqual(
		map[string]interface{}{"hello": 3, "world": "hello", "goodbye": []byte("aloha")},
		map[interface{}]interface{}{"hello": 3, "world": "hello", "goodbye": []byte("aloha")})
	if err != nil {
		t.Error(err)
	}

	err = castSliceShouldEqual(
		[]string{"one", "two", "three"},
		[]interface{}{"one", "two", "three"},
	)
	if err != nil {
		t.Error(err)
	}
}

func TestDeepMergeWithArbitraryMap(t *testing.T) {
	m1 := map[interface{}]interface{}{
		"key1": map[string]int{
			"one":   1,
			"two":   2,
			"three": 3,
		},
		"key2": map[string][]byte{
			"four": []byte{0x4},
			"five": []byte{0x5},
			"six":  []byte{0x6},
		},
		"key3": map[string]interface{}{
			"myarr":  []string{"alpha", "bravo", "charlie"},
			"myarr2": []string{"some", "strings"},
		},
	}

	m2 := map[interface{}]interface{}{
		"key1": map[string]int{
			"one":   10,
			"three": 30,
		},
		"key2": map[string][]byte{
			"four": []byte{0x4},
		},
		"key3": map[string]interface{}{
			"myarr":  []string{"delta"},
			"myarr2": []int{123, 456},
		},
	}

	res := DeepMerge(m1, m2)
	expectedResult := map[interface{}]interface{}{
		"key1": map[interface{}]interface{}{
			"one":   10,
			"two":   2,
			"three": 30,
		},
		"key2": map[interface{}]interface{}{
			"four": []interface{}{uint8(0x4), uint8(0x4)},
			"five": []uint8{uint8(0x5)}, // will not be casted to []interface{}!
			"six":  []uint8{uint8(0x6)}, // will not be casted to []interface{}!
		},
		"key3": map[interface{}]interface{}{
			"myarr":  []interface{}{"alpha", "bravo", "charlie", "delta"},
			"myarr2": []int{123, 456}, // will not be casted to []interface{}!
		},
	}

	if !reflect.DeepEqual(res, expectedResult) {
		t.Errorf("Expected %+v, got %+v", expectedResult, res)
	}
}
