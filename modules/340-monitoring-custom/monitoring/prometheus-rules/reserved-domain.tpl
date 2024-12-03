- name: d8.monitoring-custom.reserved-domain
  rules:
  - alert: D8ReservedNodeLabelOrTaintFound
    expr: max(reserved_domain_nodes == 1) by (name) == 1
    labels:
      d8_component: monitoring-custom
      d8_module: monitoring-custom
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_monitoring_custom_reserved_domain_group: "D8ReservedNodeLabelOrTaintFoundInCluster,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_monitoring_custom_reserved_domain_group: "D8ReservedNodeLabelOrTaintFoundInCluster,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: "Node {{`{{ $labels.name }}`}} needs fixing up"
      description: |-
        Node {{`{{ $labels.name }}`}} uses:
        - reserved `metadata.labels` *node-role.deckhouse.io/* with ending not in `(system|frontend|monitoring|_deckhouse_module_name_)`
        - or reserved `spec.taints` *dedicated.deckhouse.io* with values not in `(system|frontend|monitoring|_deckhouse_module_name_)`

        [Get instructions on how to fix it here]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/products/kubernetes-platform/documentation/v1/modules/040-node-manager/faq.html#how-do-i-allocate-nodes-to-specific-loads).
