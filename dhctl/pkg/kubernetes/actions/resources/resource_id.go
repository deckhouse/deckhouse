// Copyright 2024 Flant JSC
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

package resources

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceID struct {
	Name             string
	Namespace        string
	GroupVersionKind schema.GroupVersionKind
}

func (r *ResourceID) String() string {
	return r.GroupVersionKindNamespaceNameString()
}

func (r *ResourceID) GroupVersionKindNamespaceNameString() string {
	return strings.Join([]string{r.GroupVersionKindNamespaceString(), r.Name}, "/")
}

func (r *ResourceID) GroupVersionKindNamespaceString() string {
	var resultElems []string

	if r.Namespace != "" {
		resultElems = append(resultElems, fmt.Sprint("ns:", r.Namespace))
	}

	gvk := r.GroupVersionKindString()
	if gvk != "" {
		resultElems = append(resultElems, gvk)
	}

	return strings.Join(resultElems, "/")
}

func (r *ResourceID) GroupVersionKindString() string {
	var gvkElems []string

	if r.GroupVersionKind.Group != "" {
		gvkElems = append(gvkElems, r.GroupVersionKind.Group)
	}

	if r.GroupVersionKind.Version != "" {
		gvkElems = append(gvkElems, r.GroupVersionKind.Version)
	}

	if r.GroupVersionKind.Kind != "" {
		gvkElems = append(gvkElems, r.GroupVersionKind.Kind)
	}

	return strings.Join(gvkElems, "/")
}
