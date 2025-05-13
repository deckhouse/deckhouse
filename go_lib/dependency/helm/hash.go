/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helm

import (
	"fmt"
	"hash"
	"sort"
)

func hashObject(sum map[string]interface{}, hash *hash.Hash) {
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
}

func hashArray(array []interface{}, hash *hash.Hash) {
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
}

func escapeKey(key string, writer *hash.Hash) {
	(*writer).Write([]byte(key))
}

func escapeValue(value interface{}, writer *hash.Hash) {
	fmt.Fprintf(*writer, `%T`, value)
	fmt.Fprintf(*writer, "%v", value)
}

func sortedKeys(sum map[string]interface{}) []string {
	keys := make([]string, 0, len(sum))
	for k := range sum {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}
