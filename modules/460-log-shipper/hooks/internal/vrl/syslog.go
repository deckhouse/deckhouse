/*
Copyright 2024 Flant JSC

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

/*
let's make RFC 5424 compatible messages for rsyslog
read more about the format:
https://blog.datalust.co/seq-input-syslog/#rfc5424
*/

const SyslogEncodingRule Rule = `
if !exists(.syslog.severity) {
  .syslog.severity = 6;
} else if is_string(.syslog.severity) {
  .syslog.severity = to_syslog_severity!(.syslog.severity);
} else {
  .syslog.severity = 6;
};

pri = 1 * 8 + .syslog.severity;

., err = join([
  "<" + to_string(pri) + ">" + "1",     # <pri>version
  to_string!(.timestamp),
  to_string!(.kubernetes.pod_name || .hostname || "${VECTOR_SELF_NODE_NAME}"),
  to_string!(.app || .kubernetes.labels.app || .syslog.app || "-"),
  "-", # procid
  to_string!(.syslog.message_id || "-"), # msgid
  "-", # structured-data
  decode_base16!("EFBBBF") + to_string!(.message || encode_json(.)) # msg
], separator: " ")

if err != null {
  log("Unable to construct syslog message for event:" + err + ". Dropping invalid event: " + encode_json(.), level: "error", rate_limit_secs: 10)
}
`
