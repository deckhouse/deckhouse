- name: d8.monitoring-custom.reserved-domain-group
  rules:
  - alert: D8ReservedNodeLabelOrTaintFoundInCluster
    expr: |
      count(ALERTS{alertname=~"D8ReservedNodeLabelOrTaintFound", alertstate="firing"}) > 0
    labels:
      tier: cluster
      d8_component: monitoring-custom
      d8_module: monitoring-custom
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      summary: Resources requiring fixing up has been found in cluster.
      description: |
        What exactly needs fixing up can be found in linked alerts.

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
      plk_grouped_by__d8_monitoring_custom_reserved_domain_group: "D8ReservedNodeLabelOrTaintFoundInCluster,tier=cluster,prometheus=deckhouse"
      summary: "Node {{`{{ $labels.name }}`}} needs fixing up"
      description: |-
        Node {{`{{ $labels.name }}`}} uses:
        - reserved `metadata.labels` *node-role.deckhouse.io/* with ending not in `(system|frontend|monitoring|_deckhouse_module_name_)`
        - or reserved `spec.taints` *dedicated.deckhouse.io* with values not in `(system|frontend|monitoring|_deckhouse_module_name_)`

{{- if .Values.global.modules.publicDomainTemplate }}
        [Get instructions on how to fix it here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/faq.html#%D0%BA%D0%B0%D0%BA-%D0%B2%D1%8B%D0%B4%D0%B5%D0%BB%D0%B8%D1%82%D1%8C-%D1%83%D0%B7%D0%BB%D1%8B-%D0%BF%D0%BE%D0%B4-%D1%81%D0%BF%D0%B5%D1%86%D0%B8%D1%84%D0%B8%D1%87%D0%B5%D1%81%D0%BA%D0%B8%D0%B5-%D0%BD%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B8).
{{- end }}
