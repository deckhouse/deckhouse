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

// TODO(nabokihms): figure out why do we need this rule.
//   The assumption is that it is required to send logs to datastream indexes in which logs were previously send
//   by logstash. Elasticsearch can only show logs with either timestamp or @timestamp field.
//   Thus, without this rule appending to logstash datastream indexes is not possible.
//   Now it seems more like a weird kludge.

// StreamRule puts the vector timestamp to the label recognized by Elasticsearch.
const StreamRule Rule = `
."@timestamp" = del(.timestamp)
`

// DeDotRule replaces all dots in kubernetes labels to avoid ELasticsearch treating them as nested objects.
//
// Related issue https://github.com/timberio/vector/issues/3588
// P.S. pod_labels is always an object type if it is present, so we can panic on error here.
const DeDotRule Rule = `
if exists(.pod_labels) {
    .pod_labels = map_keys(object!(.pod_labels), recursive: true) -> |key| { replace(key, ".", "_") }
}
`

// ParsedDataCleanUpRule cleans up the temporary parsed data object.
const ParsedDataCleanUpRule Rule = `
if exists(.parsed_data) {
    del(.parsed_data)
}
`
