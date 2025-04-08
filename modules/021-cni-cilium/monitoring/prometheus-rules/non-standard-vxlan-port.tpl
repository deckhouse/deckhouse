- name: kubernetes.cni-cilium.non-standard-vxlan-port
  rules:
  - alert: NonStandardVxlanPort
    expr: >-
      count(d8_cni_cilium_non_standard_vxlan_port) > 0
    labels:
      severity_level: "9"
    annotations:
      summary: Cillium is configured to use a non-standard port for tunneling.
      description: |-
        TODO: Provide some description below.
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_extended_monitoring_deprecated_annotation: "NonStandardVxlanPort,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_extended_monitoring_deprecated_annotation: "NonStandardVxlanPort,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
