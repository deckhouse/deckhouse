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

// DropLabelsKeepOnly keeps only listed keys in the object at pathArray; drops the rest (dropLabels with keepOnly).
const DropLabelsKeepOnly Rule = `
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
if err == null && value != null && is_string(value) {
{{- if .useNamedGroups }}
  parsed, perr = parse_regex(value, r'{{.sourceRegex}}')
  if perr == null {
    replaced, rerr = replace(value, r'{{.sourceRegex}}', {{.replacementExpr}})
    if rerr == null {
      . = set!(., {{.pathArray}}, replaced)
    }
  }
{{- else }}
  replaced, rep_err = replace(value, r'{{.sourceRegex}}', {{.targetQuoted}})
  if rep_err == null {
    . = set!(., {{.pathArray}}, replaced)
  }
{{- end }}
}
`

// AddLabelsWhenLeaf is one addLabels when clause: compare or regex on a path; arrays use any-hit / no-hit (arrayWantAny).
// kind: literal (quoted rhs), rightPath (compare pathArray to rightPathArray), regex (pattern literal only).
const AddLabelsWhenLeaf Rule = `
val_{{.i}}, err_{{.i}} = get(., {{.pathArray}})
{{- if eq .kind "rightPath" }}
ref_{{.i}}, err_ref_{{.i}} = get(., {{.rightPathArray}})
{{- end }}
b_{{.i}} = false
{{- if eq .kind "rightPath" }}
if err_{{.i}} == null && err_ref_{{.i}} == null {
{{- else }}
if err_{{.i}} == null {
{{- end }}
  if is_array(val_{{.i}}) {
    hit_{{.i}} = filter(array!(val_{{.i}})) -> |_idx_{{.i}}, el_{{.i}}| {
{{- if eq .kind "regex" }}
      s_el_{{.i}}, err_el_{{.i}} = to_string(el_{{.i}})
      if err_el_{{.i}} != null {
        false
      } else {
        _, perr_{{.i}} = parse_regex(s_el_{{.i}}, r'{{.regex}}')
        perr_{{.i}} == null
      }
{{- else if eq .kind "rightPath" }}
      s_el_{{.i}}, err_el_{{.i}} = to_string(el_{{.i}})
      s_ref_el_{{.i}}, err_ref_el_{{.i}} = to_string(ref_{{.i}})
      err_el_{{.i}} == null && err_ref_el_{{.i}} == null && s_el_{{.i}} == s_ref_el_{{.i}}
{{- else }}
      s_el_{{.i}}, err_el_{{.i}} = to_string(el_{{.i}})
      err_el_{{.i}} == null && s_el_{{.i}} == {{.quotedValue}}
{{- end }}
    }
    b_{{.i}} = {{ if .arrayWantAny }}length(hit_{{.i}}) > 0{{ else }}length(hit_{{.i}}) == 0{{ end }}
  } else {
{{- if eq .kind "regex" }}
    s_{{.i}}, err_s_{{.i}} = to_string(val_{{.i}})
    if err_s_{{.i}} == null {
      _, perr_{{.i}} = parse_regex(s_{{.i}}, r'{{.regex}}')
      b_{{.i}} = perr_{{.i}} {{.regexFindOp}} null
    }
{{- else if eq .kind "rightPath" }}
    s_{{.i}}, err_str_{{.i}} = to_string(val_{{.i}})
    s_ref_{{.i}}, err_ref_str_{{.i}} = to_string(ref_{{.i}})
    b_{{.i}} = err_str_{{.i}} == null && err_ref_str_{{.i}} == null && s_{{.i}} {{.cmpOp}} s_ref_{{.i}}
{{- else }}
    s_{{.i}}, err_str_{{.i}} = to_string(val_{{.i}})
    b_{{.i}} = err_str_{{.i}} == null && s_{{.i}} {{.cmpOp}} {{.quotedValue}}
{{- end }}
  }
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
      . = merge(., parsed, deep: true)
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
  . = merge(., wrapped, deep: true)
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
    . = merge(., out, deep: true)
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
