package helm

import (
	"fmt"
	"hash"
	"sort"
)

func hashObject(sum map[string]interface{}, hash *hash.Hash) {

	barrier(hash)
	var keys = sortedKeys(sum)
	for _, key := range keys {
		v := sum[key]

		escapeKey(key, hash)
		switch o := v.(type) {
		case map[string]interface{}:
			hashObject(o, hash)
		case []interface{}:
			hashArray(o, hash)
		default:
			escapeValue(o, hash)
		}
	}
	barrier(hash)
}

func hashArray(array []interface{}, hash *hash.Hash) {

	barrier(hash)
	for _, v := range array {

		switch o := v.(type) {
		case map[string]interface{}:
			hashObject(o, hash)
		case []interface{}:
			hashArray(o, hash)
		default:
			escapeValue(o, hash)
		}
	}
	barrier(hash)
}

func escapeKey(key string, writer *hash.Hash) {
	(*writer).Write([]byte(key))
	barrier(writer)
}

func escapeValue(value interface{}, writer *hash.Hash) {
	(*writer).Write([]byte(fmt.Sprintf(`%T`, value)))
	barrier(writer)
	(*writer).Write([]byte(fmt.Sprintf("%v", value)))
	barrier(writer)
}

// an invalide utf8 byte is used as a barrier
var barrierValue = []byte{byte('\255')}

func barrier(writer *hash.Hash) {
	(*writer).Write(barrierValue)
}

func sortedKeys(json map[string]interface{}) []string {
	var keys []string
	for k := range json {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}
