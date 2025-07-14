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

// ParseJSONMEssage If it can parse the log into a object with parsing depth
// or leaves the log in its original state
const ParseJSONMessage Rule = `
if is_string(.message) {
  .message = parse_json(
    .message{{if ne .depth 0}}, max_depth: {{.depth}}{{end}}
  ) ?? .message
}`

// ParseKlogMessage If it can parse the log from the klog format to object
// or leaves the log in its original state
const ParseKlogMessage Rule = `
if is_string(.message) {
  .message = parse_klog(.message) ?? .message
}`

// ParseCLFMessage If it can parse the log from the CLF format to object
// or leaves the log in its original state
const ParseCLFMessage Rule = `
if is_string(.message) {
  .message = parse_common_log(.message) ?? .message
}`

// ParseSysLogMessage If it can parse the log from the syslog format to object
// or leaves the log in its original state
const ParseSysLogMessage Rule = `
if is_string(.message) {
  .message = parse_syslog(.message) ?? .message
}`

// ParseLogfmtMessage If it can parse the log from the logfmt format to object
// or leaves the log in its original state
const ParseLogfmtMessage Rule = `
if is_string(.message) {
  .message = parse_logfmt(.message) ?? .message
}`

// ParseStringMessage Packs the log as a string into a object with a key targetField
const ParseStringMessage Rule = `
if is_string(.message) {
  .message = {"{{.targetField}}": .message}
}`

// ReplaceKeys recursive replace keys in the labels.
const ReplaceKeys Rule = `
{{ range $label := $.spec.Labels }}
if exists({{$label}}) {
  {{$label}} = map_keys(
    object!({{$label}}), recursive: true
  ) -> |key| {
    replace(key, "{{$.spec.Source}}", "{{$.spec.Target}}")
  }
}
{{- end }}
`

// DropLabels delete labels.
const DropLabels Rule = `
{{ range $label := $.spec.Labels }}
if exists({{$label}}) {
  del({{$label}})
}
{{- end }}
`
