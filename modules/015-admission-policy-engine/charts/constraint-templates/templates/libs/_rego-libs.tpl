{{- define "constraint-templates.lib.common.container-review" -}}
{{ .Files.Get "files/libs/common.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.exception.spe" -}}
{{ .Files.Get "files/libs/exception.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.range" -}}
{{ .Files.Get "files/libs/range.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.set" -}}
{{ .Files.Get "files/libs/set.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.str" -}}
{{ .Files.Get "files/libs/str.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.match" -}}
{{ .Files.Get "files/libs/match.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.bool" -}}
{{ .Files.Get "files/libs/bool.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.path" -}}
{{ .Files.Get "files/libs/path.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.object" -}}
{{ .Files.Get "files/libs/object.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_bool" -}}
{{ .Files.Get "files/libs/check_bool.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_set" -}}
{{ .Files.Get "files/libs/check_set.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_range" -}}
{{ .Files.Get "files/libs/check_range.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_subset" -}}
{{ .Files.Get "files/libs/check_subset.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_object_match" -}}
{{ .Files.Get "files/libs/check_object_match.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_path" -}}
{{ .Files.Get "files/libs/check_path.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.resolve_value" -}}
{{ .Files.Get "files/libs/resolve_value.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.expand_synonyms" -}}
{{ .Files.Get "files/libs/expand_synonyms.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.message" -}}
{{ .Files.Get "files/libs/message.rego" }}
{{- end -}}
