{{- define "constraint-templates.lib.common.container-review" -}}
{{ .Files.Get "templates/libs/common.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.exception.spe" -}}
{{ .Files.Get "templates/libs/exception.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.range" -}}
{{ .Files.Get "templates/libs/range.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.set" -}}
{{ .Files.Get "templates/libs/set.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.str" -}}
{{ .Files.Get "templates/libs/str.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.match" -}}
{{ .Files.Get "templates/libs/match.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.bool" -}}
{{ .Files.Get "templates/libs/bool.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.path" -}}
{{ .Files.Get "templates/libs/path.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.object" -}}
{{ .Files.Get "templates/libs/object.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_bool" -}}
{{ .Files.Get "templates/libs/check_bool.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_set" -}}
{{ .Files.Get "templates/libs/check_set.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_range" -}}
{{ .Files.Get "templates/libs/check_range.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_subset" -}}
{{ .Files.Get "templates/libs/check_subset.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_object_match" -}}
{{ .Files.Get "templates/libs/check_object_match.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.check_path" -}}
{{ .Files.Get "templates/libs/check_path.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.resolve_value" -}}
{{ .Files.Get "templates/libs/resolve_value.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.expand_synonyms" -}}
{{ .Files.Get "templates/libs/expand_synonyms.rego" }}
{{- end -}}

{{- define "constraint-templates.lib.message" -}}
{{ .Files.Get "templates/libs/message.rego" }}
{{- end -}}
