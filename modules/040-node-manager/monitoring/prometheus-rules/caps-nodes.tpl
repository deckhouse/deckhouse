- name: d8.caps-nodes
  rules:
  - alert: CapsInstanceUnavailable
    expr: max by (machine_deployment_name) (d8_caps_md_unavailable) > 0
    for: 30m
    labels:
      severity_level: "8"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_has_caps_machinedeployment_with_unavailable_replicas: "ClusterHasCapsMachineDeploymentWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_has_caps_machinedeployment_with_unavailable_replicas: "ClusterHasCapsMachineDeploymentWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "caps_md"
      summary: There are unavailable instances in the `{{`{{ $labels.machine_deployment_name }}`}}` MachineDeployment.
      description: |
        The MachineDeployment `{{`{{ $labels.machine_deployment_name }}`}}` has **{{`{{ $value }}`}}** unavailable instances.

        Check the status of the instances in the cluster with the following command:

        ```shell
        d8 k get instance -l node.deckhouse.io/group={{`{{ $labels.machine_deployment_name }}`}}
        ```
