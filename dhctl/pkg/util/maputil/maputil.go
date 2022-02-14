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

func Values(m map[string]string) []string {
	keysList := make([]string, 0, len(m))
	for _, v := range m {
		keysList = append(keysList, v)
	}

	return keysList
}
