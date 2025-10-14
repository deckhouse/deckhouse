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
      summary: "Node {{`{{ $labels.name }}`}} is using a reserved label or taint."
      description: |-
        Deckhouse has detected that node {{`{{ $labels.name }}`}} is using one of the following:
        - A reserved `metadata.labels` object `node-role.deckhouse.io/`, which doesn't end with `(system|frontend|monitoring|_deckhouse_module_name_)`.
        - A reserved `spec.taints` object `dedicated.deckhouse.io`, with a value other than `(system|frontend|monitoring|_deckhouse_module_name_)`.

        For instructions on how to resolve this issue, refer to the [node allocation guide](/modulesnode-manager/faq.html#how-do-i-allocate-nodes-to-specific-loads).
