// Copyright 2026 Flant JSC
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

package main

// deepMerge recursively merges patch into base for maps.
// Slices and scalars in patch replace the value at that path entirely.
func deepMerge(base, patch any) any {
	if patch == nil {
		return cloneYAMLValue(base)
	}
	switch p := patch.(type) {
	case []any:
		return cloneYAMLSlice(p)
	case map[string]any:
		bm, ok := base.(map[string]any)
		if !ok || bm == nil {
			return cloneYAMLMap(p)
		}
		out := cloneYAMLMap(bm)
		for k, v := range p {
			if v == nil {
				delete(out, k)
				continue
			}
			if cur, ok := out[k]; ok {
				out[k] = deepMerge(cur, v)
			} else {
				out[k] = cloneYAMLValue(v)
			}
		}
		return out
	default:
		return patch
	}
}

func cloneYAMLValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return cloneYAMLMap(x)
	case []any:
		return cloneYAMLSlice(x)
	default:
		return x
	}
}

func cloneYAMLMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneYAMLValue(v)
	}
	return out
}

func cloneYAMLSlice(s []any) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = cloneYAMLValue(v)
	}
	return out
}

func mergeBaseDocument(baseDoc any, merge any) (any, error) {
	if merge == nil {
		return cloneYAMLValue(baseDoc), nil
	}
	return deepMerge(baseDoc, merge), nil
}
