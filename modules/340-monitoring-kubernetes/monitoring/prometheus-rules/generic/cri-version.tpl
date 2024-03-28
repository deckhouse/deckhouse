- name: cri.version
  rules:
  - alert: UnsupportedContainerRuntimeVersion
    expr: |
      sum by (container_runtime_version, job, kernel_version, kubelet_version, kubeproxy_version, node, os_image) (
        kube_node_info{kubelet_version=~"v1.(1[6-9]|2[0-9]).+", container_runtime_version!~"docker://1\\.13\\..*|docker://1[7-9]\\..*|docker://20\\..*|docker://3\\..*|containerd://1\\.[4-7]\\..*"}
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
        * Docker 1.13.x
        * Docker 17.x
        * Docker 18.x
        * Docker 19.x
        * Docker 20.x
        * Docker 3.x (for moby project in Azure)
        * Containerd 1.4.*
        * Containerd 1.5.*
        * Containerd 1.6.*
        * Containerd 1.7.*
      summary: >
        Unsupported version of CRI {{`{{$labels.container_runtime_version}}`}} installed for Kubernetes version: {{`{{$labels.kubelet_version}}`}}

  - alert: DeprecatedDockerContainerRuntime
    expr: |
      sum by (container_runtime_version, node) (
        kube_node_info{container_runtime_version=~"docker://.*"}
        * on (node) group_left(label_node_deckhouse_io_group) kube_node_labels
        * on (label_node_deckhouse_io_group) group_left(cri_type)
        label_replace(
          node_group_info{cri_type!="NotManaged"},
          "label_node_deckhouse_io_group", "$0", "name", ".*"
        )
      )
    labels:
      tier: cluster
      severity_level: "4"
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: markdown
      description: |-
        Found docker CRI installed on {{`{{$labels.node}}`}} node.
        Docker runtime is deprecated and will be removed in the nearest future.
        You should [migrate](https://deckhouse.io/documentation/v1/modules/040-node-manager/faq.html#how-to-change-cri-for-the-whole-cluster) to Containerd CRI.
      summary: >
        Deprecated version of CRI {{`{{$labels.container_runtime_version}}`}} installed on {{`{{$labels.node}}`}} node.
