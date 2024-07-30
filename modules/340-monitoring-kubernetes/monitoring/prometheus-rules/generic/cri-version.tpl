- name: cri.version
  rules:
  - alert: UnsupportedContainerRuntimeVersion
    expr: |
      sum by (container_runtime_version, job, kernel_version, kubelet_version, kubeproxy_version, node, os_image) (
        kube_node_info{kubelet_version=~"v1.(1[6-9]|2[0-9]).+", container_runtime_version!~"containerd://1\\.[4-7]\\..*"}
        * on (node) group_left(label_node_deckhouse_io_group) kube_node_labels
        * on (label_node_deckhouse_io_group) group_left(cri_type)
        label_replace(
          node_group_info{cri_type!="NotManaged"},
          "label_node_deckhouse_io_group", "$0", "name", ".*"
        )
      )
    for: 20m
    labels:
      impact: negligible
      likelihood: certain
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: markdown
      description: |-
        Unsupported version {{`{{$labels.container_runtime_version}}`}} of CRI installed on {{`{{$labels.node}}`}} node.
        Supported version of CRI for kubernetes {{`{{$labels.kubelet_version}}`}} version:
        * Containerd 1.4.*
        * Containerd 1.5.*
        * Containerd 1.6.*
        * Containerd 1.7.*
      summary: >
        Unsupported version of CRI {{`{{$labels.container_runtime_version}}`}} installed for Kubernetes version: {{`{{$labels.kubelet_version}}`}}
