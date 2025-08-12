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
	Action       TransformationAction `json:"action"`
	ReplaceKeys  ReplaceKeysSpec      `josn:"replaceKeys,omitempty"`
	ParseMessage ParseMessageSpec     `json:"parseMessage,omitempty"`
	DropLabels   DropLabelsSpec       `json:"dropLabels,omitempty"`
}

type TransformationAction string

const (
	ReplaceKeys  TransformationAction = "ReplaceKeys"
	ParseMessage TransformationAction = "ParseMessage"
	DropLabels   TransformationAction = "DropLabels"
)

type ReplaceKeysSpec struct {
	Source string   `json:"source"`
	Target string   `json:"target"`
	Labels []string `json:"labels"`
}
type DropLabelsSpec struct {
	Labels []string `json:"labels"`
}
type ParseMessageSpec struct {
	SourceFormat SourceFormat           `json:"sourceFormat"`
	String       SourceFormatStringSpec `json:"string,omitempty"`
	JSON         SourceFormatJSONSpec   `json:"json,omitempty"`
}
type SourceFormat string

const (
	FormatString SourceFormat = "String"
	FormatJSON   SourceFormat = "JSON"
	FormatKlog   SourceFormat = "Klog"
	FormatSysLog SourceFormat = "SysLog"
	FormatCLF    SourceFormat = "CLF"
	FormatLogfmt SourceFormat = "Logfmt"
)

type SourceFormatStringSpec struct {
	TargetField string `json:"targetField"`
}
type SourceFormatJSONSpec struct {
	Depth int `json:"depth,omitempty"`
}
