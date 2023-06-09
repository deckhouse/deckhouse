{{ range $alias := .aliases }}
{{- printf "* %s - `%s`" $alias.name (join "`, `" $alias.verbs) }}
{{ end }}
{{- range $role := .roles }}
{{`{{site.data.i18n.common.role[page.lang] | capitalize }}`}} `{{ $role.name }}`
{{- if $role.additionalRoles }}
{{- printf " ({{site.data.i18n.common.includes_rules_from[page.lang]}} `%s`)" (join "`, `" $role.additionalRoles) }}
{{- end }}:

```text
{{ $role.rules | toYaml -}}
```
{{ end }}
