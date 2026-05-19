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

import (
	"fmt"
)

// applyNamedContainerListPatches deep-merges each patch["merge"] into spec[field][i]
// where container name equals patch["name"].
func applyNamedContainerListPatches(doc any, field string, patches []any) (any, error) {
	if len(patches) == 0 {
		return doc, nil
	}
	root, err := asMap(doc)
	if err != nil {
		return nil, fmt.Errorf("apply container patches: %w", err)
	}
	specVal, ok := root["spec"]
	if !ok || specVal == nil {
		return nil, fmt.Errorf("apply container patches: spec missing")
	}
	spec, err := asMap(specVal)
	if err != nil {
		return nil, fmt.Errorf("apply container patches: spec must be a map")
	}
	listVal, hasList := spec[field]
	var list []interface{}
	if !hasList || listVal == nil {
		if field == "initContainers" {
			list = []interface{}{}
		} else {
			return nil, fmt.Errorf("apply container patches: spec.%s missing", field)
		}
	} else {
		var err error
		list, err = asSlice(listVal)
		if err != nil {
			return nil, fmt.Errorf("apply container patches: spec.%s: %w", field, err)
		}
	}
	for _, pRaw := range patches {
		p, err := asMap(pRaw)
		if err != nil {
			return nil, fmt.Errorf("apply container patches: patch must be a map")
		}
		cname, _ := p["name"].(string)
		if cname == "" {
			return nil, fmt.Errorf("apply container patches: patch.name is required")
		}
		mergePart, ok := p["merge"]
		if !ok || mergePart == nil {
			continue
		}
		found := false
		for i, elem := range list {
			cm, err := asMap(elem)
			if err != nil {
				continue
			}
			en, _ := stringField(cm["name"])
			if en != cname {
				continue
			}
			merged := deepMerge(cm, mergePart)
			mmap, err := asMap(merged)
			if err != nil {
				return nil, fmt.Errorf("apply container patches: merge result not a map")
			}
			list[i] = mmap
			found = true
			break
		}
		if !found {
			if field != "initContainers" {
				return nil, fmt.Errorf("apply container patches: no %s item named %q", field, cname)
			}
			newC := map[string]interface{}{"name": cname}
			merged := deepMerge(newC, mergePart)
			if mm, err := asMap(merged); err == nil {
				list = append(list, mm)
			} else {
				list = append(list, merged)
			}
		}
	}
	spec[field] = list
	root["spec"] = spec
	return root, nil
}

func stringField(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return fmt.Sprint(v), true
}

func asMap(v any) (map[string]interface{}, error) {
	m, ok := v.(map[string]interface{})
	if ok {
		return m, nil
	}
	return nil, fmt.Errorf("expected map, got %T", v)
}

func asSlice(v any) ([]interface{}, error) {
	s, ok := v.([]interface{})
	if ok {
		return s, nil
	}
	return nil, fmt.Errorf("expected slice, got %T", v)
}
