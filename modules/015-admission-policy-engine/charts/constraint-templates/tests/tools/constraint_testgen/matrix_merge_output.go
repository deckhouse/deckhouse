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

import "fmt"

func mergeDocFromMatrixParts(bases map[string]matrixBase, baseName string, merge map[string]interface{}, containerMerges, initContainerMerges []interface{}) (any, error) {
	b, ok := bases[baseName]
	if !ok {
		return nil, fmt.Errorf("unknown base %q", baseName)
	}
	var m any
	var err error
	if merge == nil {
		m, err = mergeBaseDocument(b.Document, nil)
	} else {
		m, err = mergeBaseDocument(b.Document, merge)
	}
	if err != nil {
		return nil, err
	}
	cma := ifaceSliceToAny(containerMerges)
	ima := ifaceSliceToAny(initContainerMerges)
	if len(cma) > 0 {
		m, err = applyNamedContainerListPatches(m, "containers", cma)
		if err != nil {
			return nil, err
		}
	}
	if len(ima) > 0 {
		m, err = applyNamedContainerListPatches(m, "initContainers", ima)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func ifaceSliceToAny(s []interface{}) []any {
	if s == nil {
		return nil
	}
	out := make([]any, len(s))
	for i := range s {
		out[i] = s[i]
	}
	return out
}
