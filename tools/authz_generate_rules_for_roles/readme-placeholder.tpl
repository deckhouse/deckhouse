{{ range $alias := .aliases }}
{{- printf "* %s - `%s`" $alias.name (join "`, `" $alias.verbs) }}
{{ end }}
```yaml
{{- range $role := .roles }}
Role `{{ $role.name }}`
{{- if $role.additionalRoles }}
{{- printf " (and all rules from `%s`)" (join "`, `" $role.additionalRoles) }}
{{- end }}:
{{ $role.rules | toYaml | indent 4 }}
{{- end }}
```
