{{ range $alias := .Aliases }}
{{- printf "* %s - `%s`" $alias.Name (join "`, `" $alias.Verbs) }}
{{ end }}
{{- range $role := .Roles }}
{{`{{site.data.i18n.common.role[page.lang] | capitalize }}`}} `{{ $role.Name }}`
{{- if $role.AdditionalRoles }}
{{- printf " ({{site.data.i18n.common.includes_rules_from[page.lang]}} `%s`)" (join "`, `" $role.AdditionalRoles) }}
{{- end }}:
{{ if $role.Rules }}
```text
{{ $role.Rules | toYaml -}}
```
{{- end }}
{{ end }}
