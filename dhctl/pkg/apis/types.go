/*
Copyright 2025 Flant JSC

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

package apis

import (
	"maps"
	"slices"
	"sort"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type (
	ListKindToGVR = map[string]schema.GroupVersionResource
)

func CopyListKindToGVR(listKinds ListKindToGVR) ListKindToGVR {
	res := make(ListKindToGVR, len(listKinds))
	maps.Copy(res, listKinds)
	return res
}

func GVRList(listKinds ListKindToGVR) []schema.GroupVersionResource {
	keys := slices.Collect(maps.Keys(listKinds))
	sort.Strings(keys)

	res := make([]schema.GroupVersionResource, 0, len(keys))
	for _, k := range keys {
		res = append(res, listKinds[k])
	}

	return res
}
