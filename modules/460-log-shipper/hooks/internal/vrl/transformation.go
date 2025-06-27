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

package vrl

import "fmt"

func ReplaceDotKeys(label string) string {
	return fmt.Sprintf("if exists(%s) {\n%s = map_keys(object!(%s), recursive: true) "+
		"-> |key| { replace(key, \".\", \"_\")}\n}", label, label, label)
}

func EnsureStructuredMessageString(targetField string) string {
	return fmt.Sprintf("if is_string(.message) {\n.message =  { \"%s\": .message }\n}", targetField)
}
func EnsureStructuredMessageJSON(depth int) string {
	maxDepth := ""
	if depth > 0 {
		maxDepth = fmt.Sprintf(", max_depth: %d", depth)
	}
	return fmt.Sprintf(".message = parse_json(.message%s) ?? .message", maxDepth)
}
func EnsureStructuredMessageKlog() string {
	return ".message = parse_klog(.message) ?? .message"
}
func DropLabels(label string) string {
	return fmt.Sprintf("if exists(%s) {\n del(%s)\n}", label, label)
}
