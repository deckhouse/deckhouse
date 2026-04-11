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

// ReplaceKeys renames object keys under configured paths (recursive map_keys), replaceKeys transform.
const ReplaceKeys Rule = `
{{ range $path := $.spec.Paths }}
if exists({{$path}}) {
  {{$path}} = map_keys(
    object!({{$path}}), recursive: true
  ) -> |key| {
    replace(key, "{{$.spec.Source}}", "{{$.spec.Target}}")
  }
}
{{- end }}
`

// DropLabels deletes label subtrees at the given paths, dropLabels transform.
const DropLabels Rule = `
{{ range $path := $.spec.Paths }}
if exists({{$path}}) {
  del({{$path}})
}
{{- end }}
`

// DropLabelsKeepChildKeys keeps only listed keys in the object at pathArray; drops the rest (dropLabels with keepChildKeys).
const DropLabelsKeepChildKeys Rule = `
obj, err = get(., {{.pathArray}})
if err == null && is_object(obj) {
  filtered = {}
{{- range .keepKeys }}
  v, err2 = get(obj, [{{ . | quote }}])
  if err2 == null {
    filtered = set!(filtered, [{{ . | quote }}], v)
  }
{{- end }}
  . = set!(., {{.pathArray}}, filtered)
}
`

// ReplaceValueRule replaces in a string field by regex: literal replacement or named groups via mustache in target.
// Named captures read from the `parsed` value produced by parse_regex (same as parseMessage string-regex).
const ReplaceValueRule Rule = `
value, err = get(., {{.pathArray}})
if err == null && value != null {
  value_str, err_str = to_string(value)
  if err_str == null {
{{- if .useNamedGroups }}
    parsed, perr = parse_regex(value_str, r'{{.sourceRegex}}')
    if perr == null {
      replaced = replace(value_str, r'{{.sourceRegex}}', {{.replacementExpr}})
      . = set!(., {{.pathArray}}, replaced)
    }
{{- else }}
    replaced, rep_err = replace(value_str, r'{{.sourceRegex}}', {{.targetQuoted}})
    if rep_err == null {
      . = set!(., {{.pathArray}}, replaced)
    }
{{- end }}
  }
}
`

// AddLabelsWhenPresenceLeaf is one addLabels when clause: path exists (opCmp ==) or missing (opCmp !=).
const AddLabelsWhenPresenceLeaf Rule = `
_, err = get(., {{.pathArray}})
b_{{.i}} = err {{.opCmp}} null
`

// AddLabelsWhenLeaf is one addLabels when clause: compare or regex on a scalar path value.
// kind: literal (quoted rhs), regex (pattern literal only).
// Reuses val, err, s, err_s, perr across leaves; only b_{{.i}} is unique per condition.
const AddLabelsWhenLeaf Rule = `
val, err = get(., {{.pathArray}})
b_{{.i}} = false
if err == null {
  s, err_s = to_string(val)
{{- if eq .kind "regex" }}
  if err_s == null {
    _, perr = parse_regex(s, r'{{.regex}}')
    b_{{.i}} = perr {{.regexFindOp}} null
  }
{{- else }}
  b_{{.i}} = err_s == null && s {{.cmpOp}} {{.quotedValue}}
{{- end }}
}
`

// AddLabelsWhenMultiIf wraps label assignments so they run only when all when booleans (cond) are true.
const AddLabelsWhenMultiIf Rule = `if {{.cond}} {
{{.body}}
}
`

// AddLabelsAssign assigns a static string to a label field.
const AddLabelsAssign Rule = `{{.lhs}} = {{.rhs}}`

// AddLabelsFromPath copies a value from an event path into a label field (mustache path rhs).
const AddLabelsFromPath Rule = `
v, err = get(., {{.pathArray}})
if err == null {
  {{.lhs}} = v
}
`

// ParseMessageDest parses .message with a caller-built expression (JSON, klog, syslog, etc.); merge or set by pathArray.
const ParseMessageDest Rule = `
if is_string(.message) {
  parsed = {{.parseExpr}} ?? null
  if parsed != null {
{{if .mergeRoot}}
    if is_object(parsed) {
      . = merge!(., parsed, deep: true)
    }
{{else}}
    . = set!(., {{.pathArray}}, parsed)
{{end}}
  }
}
`

// ParseMessageString wraps raw .message in an object under targetField without parsing (string format, legacy).
const ParseMessageString Rule = `
if is_string(.message) {
  wrapped = {"{{.targetField}}": .message}
{{if .mergeRoot}}
  . = merge!(., wrapped, deep: true)
{{else}}
  . = set!(., {{.pathArray}}, wrapped)
{{end}}
}
`

// ParseMessageRegexString extracts fields from .message via parse_regex and builds out from outLines (string format with regex).
const ParseMessageRegexString Rule = `
if is_string(.message) {
  parsed, perr = parse_regex(string!(.message), r'{{.regex}}')
  if perr == null {
    out = {}
{{.outLines}}
{{if .mergeRoot}}
    . = merge!(., out, deep: true)
{{else}}
    . = set!(., {{.pathArray}}, out)
{{end}}
  }
}
`

// ParseMessageRegexStringOut is one set! line into out for ParseMessageRegexString.
const ParseMessageRegexStringOut Rule = `    out = set!(out, [{{ .key | quote }}], {{.value}})`

// RegexCaptureString is string!(parsed.<name>) for a named regex capture (replaceValue / parseMessage string-regex).
const RegexCaptureString Rule = `string!(parsed.{{.name}})`
