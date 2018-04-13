package utils

import (
	"reflect"
)

type mergeValuesPair struct {
	A map[interface{}]interface{}
	B map[interface{}]interface{}
}

func CastArbitraryMap(mapArg interface{}) map[interface{}]interface{} {
	res := make(map[interface{}]interface{})

	reflMap := reflect.ValueOf(mapArg)
	for _, key := range reflMap.MapKeys() {
		value := reflMap.MapIndex(key)
		res[key.Interface()] = value.Interface()
	}

	return res
}

func CastArbitrarySlice(sliceArg interface{}) []interface{} {
	res := make([]interface{}, 0)

	reflSlice := reflect.ValueOf(sliceArg)
	for i := 0; i < reflSlice.Len(); i++ {
		value := reflSlice.Index(i)
		res = append(res, value.Interface())
	}

	return res
}

func DeepMerge(maps ...map[interface{}]interface{}) map[interface{}]interface{} {
	res := make(map[interface{}]interface{})
	for _, m := range maps {
		res = deepMergeTwo(res, m)
	}
	return res
}

func deepMergeTwo(A map[interface{}]interface{}, B map[interface{}]interface{}) map[interface{}]interface{} {
	res := make(map[interface{}]interface{})
	for key, value := range A {
		res[key] = value
	}

	queue := []mergeValuesPair{{A: res, B: B}}

	for len(queue) > 0 {
		pair := queue[0]
		queue = queue[1:]

		for k, v2 := range pair.B {
			v1, isExist := pair.A[k]

			if isExist {
				v1Type := reflect.TypeOf(v1)
				v2Type := reflect.TypeOf(v2)

				if (v1Type == v2Type) && (v1Type != nil) {
					switch v1Type.Kind() {
					case reflect.Map:
						resMap := make(map[interface{}]interface{})

						for key, value := range CastArbitraryMap(v1) {
							resMap[key] = value
						}
						pair.A[k] = resMap

						queue = append(queue, mergeValuesPair{
							A: resMap,
							B: CastArbitraryMap(v2),
						})
					case reflect.Array, reflect.Slice:
						resArr := make([]interface{}, 0)
						for _, elem := range CastArbitrarySlice(v1) {
							resArr = append(resArr, elem)
						}
						for _, elem := range CastArbitrarySlice(v2) {
							resArr = append(resArr, elem)
						}
						pair.A[k] = resArr
					default:
						pair.A[k] = v2
					}
				} else {
					pair.A[k] = v2
				}
			} else {
				pair.A[k] = v2
			}
		}
	}

	return res
}
