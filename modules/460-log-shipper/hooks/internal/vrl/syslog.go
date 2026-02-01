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
  to_string!(.k8s_labels || ""),
  to_string!(.extra_labels || ""),
  "-", # procid
  to_string!(.syslog.message_id || "-"), # msgid
  "-", # structured-data
  decode_base16!("EFBBBF") + to_string!(.message || encode_json(.)) # msg
], separator: " ")

if err != null {
  log("Unable to construct syslog message for event:" + err + ". Dropping invalid event: " + encode_json(.), level: "error", rate_limit_secs: 10)
}
`

// SyslogLabelsRule generates VRL rule to create structured-data from source labels (k8s/file) and extra labels.
// sourceLabels are the label keys for the current pipeline source (from loglabels.GetSyslogLabels).
const SyslogLabelsRule Rule = `
sd_params = []
{{ range $label := $.sourceLabels }}
if exists(.{{$label}}) && !is_null(.{{$label}}) {
  sd_params = append(sd_params, [{{$label | printf "%q"}} + "=\"" + to_string!(.{{$label}}) + "\""])
}
{{- end }}
# Handle pod_labels_* expansion
if exists(.pod_labels) && !is_null(.pod_labels) && is_object(.pod_labels) {
  pod_labels_obj = object!(.pod_labels)
  pod_labels_keys = keys(pod_labels_obj)
  for pod_labels_keys -> |key| {
    if exists(pod_labels_obj[key]) && !is_null(pod_labels_obj[key]) {
      sd_params = append(sd_params, ["pod_labels_" + to_string!(key) + "=\"" + to_string!(pod_labels_obj[key]) + "\""])
    }
  }
}
.k8s_labels = if length(sd_params) > 0 {
  join!(sd_params, separator: " ")
}

sd_params = []
{{ range $key, $value := $.extraLabels }}
if exists({{$value}}) && !is_null({{$value}}) {
  sd_params = append(sd_params, [{{$key | printf "%q"}} + "=\"" + to_string!({{$value}}) + "\""])
}
{{- end }}
.extra_labels = if length(sd_params) > 0 {
  join!(sd_params, separator: " ")
}
`
