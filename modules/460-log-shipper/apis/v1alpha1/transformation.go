/*
Copyright 2021 Flant JSC

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

package v1alpha1

// Modules labeles transformation that users can use
type TransformationSpec struct {
	Action                  string                      `json:"action"`
	ReplaceDotKeys          ReplaceDotKeysSpec          `josn:"replaceDotKeys,omitempty"`
	EnsureStructuredMessage EnsureStructuredMessageSpec `json:"ensureStructuredMessage,omitempty"`
	DropLabels              DropLabelsSpec              `json:"dropLabels,omitempty"`
}
type ReplaceDotKeysSpec struct {
	Labels []string `json:"labels,omitempty"`
}
type EnsureStructuredMessageSpec struct {
	SourceFormat string                 `json:"sourceFormat"`
	String       SourceFormatStringSpec `json:"string,omitempty"`
	JSON         SourceFormatJSONSpec   `json:"json,omitempty"`
	Klog         SourceFormatKlogSpec   `json:"klog,omitempty"`
}
type SourceFormatStringSpec struct {
	TargetField string `json:"targetField"`
	Depth       int    `json:"depth,omitempty"`
}
type SourceFormatJSONSpec struct {
	Depth int `json:"depth,omitempty"`
}
type SourceFormatKlogSpec struct {
	Depth int `json:"depth,omitempty"`
}

type DropLabelsSpec struct {
	Labels []string `json:"labels"`
}
