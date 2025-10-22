/*
Copyright 2024 Flant JSC

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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"maps"
	"sort"
)

func hashMD5(templates map[string][]byte, values map[string]interface{}) string {
	tmp := make(map[string]interface{}, len(templates))
	for k, v := range templates {
		tmp[k] = v
	}
	sum := make(map[string]interface{})
	maps.Copy(sum, tmp)
	for k, v := range values {
		sum[k] = v
	}
	hash := md5.New()
	hashObject(sum, &hash)
	return hex.EncodeToString(hash.Sum(nil))
}

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
	(*writer).Write([]byte(fmt.Sprintf(`%T`, value)))
	(*writer).Write([]byte(fmt.Sprintf("%v", value)))
}

func sortedKeys(sum map[string]interface{}) []string {
	var keys []string
	for k := range sum {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
