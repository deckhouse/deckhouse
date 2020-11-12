- name: d8.monitoring-custom.migrate-domain-group
  rules:
  - alert: D8DeprecatedNodeSelectorOrTolerationFoundInCluster
    expr: |
      count(ALERTS{alertname=~"D8DeprecatedNodeSelectorOrTolerationFound", alertstate="firing"}) > 0
    labels:
      tier: cluster
      d8_component: monitoring-custom
      d8_module: monitoring-custom
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      summary: Resources requiring migration has been found in cluster.
      description: |
        What exactly needs migration can be found in linked alerts.

  - alert: D8DeprecatedNodeGroupLabelOrTaintFoundInCluster
    expr: |
      count(ALERTS{alertname=~"D8DeprecatedNodeGroupLabelOrTaintFound", alertstate="firing"}) > 0
    labels:
      tier: cluster
      d8_component: monitoring-custom
      d8_module: monitoring-custom
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      summary: NodeGroups requiring migration has been found in cluster.
      description: |
        What exactly needs migration can be found in linked alerts.

- name: d8.monitoring-custom.migrate-domain
  rules:
  - alert: D8DeprecatedNodeSelectorOrTolerationFound
    expr: max(migrate_domain_controllers == 1) by (controller, namespace, name) == 1
    labels:
      d8_component: monitoring-custom
      d8_module: monitoring-custom
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_grouped_by__d8_monitoring_custom_migrate_domain_group: "D8DeprecatedNodeSelectorOrTolerationFoundInCluster,tier=cluster,prometheus=deckhouse"
      summary: "{{`{{ $labels.controller }}`}} {{`{{ $labels.name }}`}} needs migration"
      description: |-
        {{`{{ $labels.controller }}`}} {{`{{ $labels.name }}`}} in Namespace {{`{{ $labels.namespace }}`}} uses:
        - old `nodeSelector` *node-role.flant.com/`(system|frontend|monitoring|_deckhouse_module_name_)`*
        - or old `tolerations` *dedicated.flant.com*  with values `(system|frontend|monitoring|_deckhouse_module_name_|_wildcard_)`

        Domain `flant.com` in this keys is going to be changed to `deckhouse.io`.

{{- if .Values.global.modules.publicDomainTemplate }}
        [Get instructions on how to fix it here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/migration.html).
{{- end }}

  - alert: D8DeprecatedNodeSelectorOrTolerationFound
    expr: max(migrate_domain_resources == 1) by (kind, namespace, name) == 1
    labels:
      d8_component: monitoring-custom
      d8_module: monitoring-custom
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_grouped_by__d8_monitoring_custom_migrate_domain_group: "D8DeprecatedNodeSelectorOrTolerationFoundInCluster,tier=cluster,prometheus=deckhouse"
      summary: "{{`{{ $labels.kind }}`}} {{`{{ $labels.name }}`}} needs migration"
      description: |-
        {{`{{ $labels.kind }}`}} {{`{{ $labels.name }}`}} in Namespace {{`{{ $labels.namespace }}`}} (`none` means global resource) uses:
        - old `nodeSelector` *node-role.flant.com/`(system|frontend|monitoring|_deckhouse_module_name_)`*
        - or old `tolerations` *dedicated.flant.com*  with values `(system|frontend|monitoring|_deckhouse_module_name_|_wildcard_)`

        Domain `flant.com` in this keys is going to be changed to `deckhouse.io`.

{{- if .Values.global.modules.publicDomainTemplate }}
        [Get instructions on how to fix it here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/migration.html).
{{- end }}

  - alert: D8DeprecatedNodeGroupLabelOrTaintFound
    expr: max(migrate_domain_nodegroups == 1) by (name) == 1
    labels:
      d8_component: monitoring-custom
      d8_module: monitoring-custom
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_grouped_by__d8_monitoring_custom_migrate_domain_group: "D8DeprecatedNodeGroupLabelOrTaintFoundInCluster,tier=cluster,prometheus=deckhouse"
      summary: "NodeGroup {{`{{ $labels.name }}`}} needs migration"
      description: |-
        NodeGroup {{`{{ $labels.name }}`}} uses:
        - old `nodeTemplate.labels` *node-role.flant.com/`(system|frontend|monitoring|_deckhouse_module_name_)`*
        - or old `nodeTemplate.taints` *dedicated.flant.com*  with values `(system|frontend|monitoring|_deckhouse_module_name_)`

        Domain `flant.com` in this keys is going to be changed to `deckhouse.io`.

{{- if .Values.global.modules.publicDomainTemplate }}
        [Get instructions on how to fix it here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/migration.html).
{{- end }}

  - alert: D8DeprecatedNodeSelectorOrTolerationFound
    expr: max(migrate_domain_configmap == 1) by (namespace, name) == 1
    labels:
      d8_component: monitoring-custom
      d8_module: monitoring-custom
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_grouped_by__d8_monitoring_custom_migrate_domain_group: "D8DeprecatedNodeSelectorOrTolerationFoundInCluster,tier=cluster,prometheus=deckhouse"
      summary: "ConfigMap {{`{{ $labels.name }}`}} needs migration"
      description: |-
        ConfigMap {{`{{ $labels.name }}`}} in Namespace {{`{{ $labels.namespace }}`}} has module with parameters:
        - old `nodeSelector` *node-role.flant.com/`(system|frontend|monitoring|_deckhouse_module_name_)`*
        - or old `tolerations` *dedicated.flant.com*  with values `(system|frontend|monitoring|_deckhouse_module_name_|_wildcard_)`

        Domain `flant.com` in this keys is going to be changed to `deckhouse.io`.

{{- if .Values.global.modules.publicDomainTemplate }}
        [Get instructions on how to fix it here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/migration.html).
{{- end }}
