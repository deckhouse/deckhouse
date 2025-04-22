- name: d8.cni-cilium.configmap
  rules:
  - alert: CniCiliumNonStandardVXLANPortFound
    expr: max by (port) (d8_cni_cilium_non_standard_vxlan_port == 1)
    for: 5m
    labels:
      severity_level: "4"
      tier: application
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__main: "ClusterHasCniCiliumNonStandardVXLANPort,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__main: "ClusterHasCniCiliumNonStandardVXLANPort,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Cilium configuration uses a non-standard VXLAN port.
      description: |
        The Cilium configuration specifies a non-standard VXLAN port {{`{{ $labels.port }}`}}. This port falls outside the recommended range (considering the cluster might be deployed on virtual machines):

        - `{{ add 4298 (.Values.global.discovery.dvpNestingLevel | default 0) }}`: When the virtualization module is enabled.
        - `{{ add 4299 (.Values.global.discovery.dvpNestingLevel | default 0) }}`: For a standard Deckhouse setup.

        To resolve this issue, update the `tunnel-port` parameter in the `cilium-configmap` ConfigMap located in the `d8-cni-cilium` namespace to match the recommended range.
        
        If you configured the non-standard port on purpose, ignore this alert.
