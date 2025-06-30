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

//VRL for transformations destination logs.

const (
	// Parse message klog to json. If message string and parse without error.
	ParseKlogMessage Rule = `
if is_string(.message) {
  .message = parse_klog(.message) ?? .message
}`
	// Parse message CLF to json. If message string and parse without error.
	ParseCLFMessage Rule = `
if is_string(.message) {
  .message = parse_common_log(.message) ?? .message
}`
	// Parse message syslog to json. If message string and parse without error.
	ParseSysLogMessage Rule = `
if is_string(.message) {
  .message = parse_syslog(.message) ?? .message
}`
	// Parse message logfmt to json. If message string and parse without error.
	ParseLogfmtMessage Rule = `
if is_string(.message) {
  .message = parse_logfmt(.message) ?? .message
}`
)

// Recursive replace keys in the label
func ReplaceKeys(label, source, target string) string {
	return fmt.Sprintf("if exists(%s) {\n%s = map_keys(object!(%s), recursive: true) "+
		"-> |key| { replace(key, \"%s\", \"%s\")}\n}", label, label, label, source, target)
}

// Parse message from string to json object. If message string
func ParseStringMessage(targetField string) string {
	return fmt.Sprintf("if is_string(.message) {\n.message =  { \"%s\": .message }\n}", targetField)
}

// Parse message json with depth
func ParseJSONMessage(depth int) string {
	maxDepth := ""
	if depth > 0 {
		maxDepth = fmt.Sprintf(", max_depth: %d", depth)
	}
	return fmt.Sprintf("if is_string(.message) {\n.message = parse_json(.message%s) ?? .message\n}", maxDepth)
}

// Parse message nginx log with format to json object
func ParseNginxLogMessage(format string) string {
	return fmt.Sprintf("if is_string(.message) {\n.message = parse_nginx_log(.message, \"%s\") ?? .message\n}", format)
}

// Delete label
func DropLabels(label string) string {
	return fmt.Sprintf("if exists(%s) {\n del(%s)\n}", label, label)
}
