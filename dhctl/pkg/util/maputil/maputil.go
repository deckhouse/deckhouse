// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package maputil

func ExcludeKeys(m map[string]string, excludeKeys ...string) map[string]string {
	excludeKeysSet := make(map[string]struct{})
	for _, k := range excludeKeys {
		excludeKeysSet[k] = struct{}{}
	}

	res := make(map[string]string)

	for k, v := range m {
		if _, ok := excludeKeysSet[k]; ok {
			continue
		}

		res[k] = v
	}

	return res
}

func Join[K comparable, V any](dst map[K]V, sources ...map[K]V) {
	for _, src := range sources {
		for k, v := range src {
			dst[k] = v
		}
	}
}

func Clone[K comparable, V any](in map[K]V) (out map[K]V) {
	out = make(map[K]V)
	for k, v := range in {
		out[k] = v
	}
	return
}

func Keys[K comparable, V any](m map[K]V) []K {
	keysList := make([]K, 0, len(m))
	for k := range m {
		keysList = append(keysList, k)
	}

	return keysList
}

func Values[K comparable, V any](m map[K]V) []V {
	valuesList := make([]V, 0, len(m))
	for _, v := range m {
		valuesList = append(valuesList, v)
	}

	return valuesList
}
