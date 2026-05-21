{{ range $alias := .Aliases }}
{{- printf "* %s - `%s`" $alias.Name (join "`, `" $alias.Verbs) }}
{{ end }}
{% if page.lang == "ru" %}
Каждая следующая роль наследует права предыдущих ролей. В блоке роли показаны только права, которые она добавляет.

Список включает правила текущей ролевой модели и стандартные правила модулей Deckhouse, доступных в репозитории при генерации документации. Права, которые добавляются модулями, появляются при включении соответствующего модуля и отзываются при его выключении. Правила внешних модулей и пользовательские ClusterRole с аннотацией `user-authz.deckhouse.io/access-level` в этот список не попадают; их можно посмотреть в кластере командой ниже.
{% else %}
Each next role inherits permissions from the previous roles. A role block shows only the permissions added by that role.

The list includes the current role-based model rules and default rules from Deckhouse modules available in the repository at documentation generation time. Permissions added by modules appear when the corresponding module is enabled and are revoked when it is disabled. External modules and user-defined ClusterRoles annotated with `user-authz.deckhouse.io/access-level` are not included; use the command below to inspect them in a cluster.
{% endif %}
{{ range $role := .Roles }}
{{`{{site.data.i18n.common.role[page.lang] | capitalize }}`}} `{{ $role.Name }}`
{{- if $role.AdditionalRoles }}
{{- printf " ({{site.data.i18n.common.includes_rules_from[page.lang]}} `%s`)" (join "`, `" $role.AdditionalRoles) }}
{{- end }}:

{{ if $role.Rules }}
```text
{{ $role.Rules | toYaml -}}
```
{{ end }}
{{ end }}
