- name: d8.node-unmanaged
  rules:
    - alert: D8NodeIsUnmanaged
      expr: max by (node) (d8_unmanaged_nodes_on_cluster) > 0
      for: 10m
      labels:
        tier: cluster
        severity_level: "9"
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_cluster_has_unmanaged_nodes: "D8ClusterHasUnmanagedNodes,tier=cluster,d8_module=node-manager,d8_component=node-group"
        plk_grouped_by__d8_cluster_has_unmanaged_nodes: "D8ClusterHasUnmanagedNodes,tier=cluster,prometheus=deckhouse"
    {{- if .Values.global.modules.publicDomainTemplate }}
        summary: The {{`{{ $labels.node }}`}} Node is not managed by the [node-manager]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/) module.
        description: |
          The {{`{{ $labels.node }}`}} Node is not managed by the [node-manager]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/) module.
    {{- else }}
        summary: The {{`{{ $labels.node }}`}} Node is not managed by the `node-manager`.
        description: |
          The {{`{{ $labels.node }}`}} Node is not managed by the `node-manager`.
    {{- end }}

          The recommended actions are as follows:
          - Create a `NodeGroup` for the Node or select the existing one;
          - Add a `node.deckhouse.io/group: <nodeGroup_name>`: `kubectl label node {{`{{ $labels.node }}`}} node.deckhouse.io/group=<nodeGroup_name>` label to it;
          - Get the script for adopting the Node: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-<nodeGroup_name> -o json | jq '.data."adopt.sh"' -r`;
          - Perform `base64` decoding on the {{`{{ $labels.node }}`}} Node: `echo <base64_string> | base64 -d | bash`;
          - Analyze the execution log: `journalctl -fu bashible`.
