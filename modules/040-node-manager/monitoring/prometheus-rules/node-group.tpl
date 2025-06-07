{{- define "todo_list" }}
        This probably means that `machine-controller-manager` is unable to create Machines using the cloud provider module.
        
        Possible causes:

        1. Cloud provider resource limits.
        2. No access to the cloud provider API.
        3. Misconfiguration of the cloud provider or instance class.
        4. Problems with bootstrapping the Machine.

        Recommended actions:
        
        1. Check the status of the NodeGroup:
        
           ```shell
           kubectl get ng {{`{{ $labels.node_group }}`}} -o yaml
           ```

           Look for errors in the `.status.lastMachineFailures` field.
        
        2. If no Machines stay in the Pending state for more than a couple of minutes, it likely means that Machines are being continuously created and deleted due to an error:

           ```shell
           kubectl -n d8-cloud-instance-manager get machine
           ```

        3. If logs don’t show errors, and a Machine continues to be Pending, check its bootstrap status:

           ```shell
           kubectl -n d8-cloud-instance-manager get machine <MACHINE_NAME> -o json | jq .status.bootstrapStatus
           ```

        4. If the output looks like the example below, connect via `nc` to examine bootstrap logs:

           ```json
           {
             "description": "Use 'nc 192.168.199.158 8000' to get bootstrap logs.",
             "tcpEndpoint": "192.168.199.158"
           }
           ```

          5. If there's no bootstrap log endpoint, `cloudInit` may not be working correctly. This could indicate a misconfigured instance class in the cloud provider.
{{- end }}

- name: d8.node-group
  rules:
  - alert: NodeGroupReplicasUnavailable
    expr: |
      max by (name) (mcm_machine_deployment_status_unavailable_replicas > 0)
      * on(name) group_left(node_group) machine_deployment_node_group_info
    for: 1h
    labels:
      severity_level: "8"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "node_group"
      summary: NodeGroup {{`{{ $labels.node_group }}`}} has unavailable instances.
      description: |
        There are {{`{{ $value }}`}} unavailable instances in the NodeGroup {{`{{ $labels.node_group }}`}}.
{{- template "todo_list" }}

  - alert: NodeGroupReplicasUnavailable
    expr: |
      max by (name) (mcm_machine_deployment_status_unavailable_replicas > 0 and mcm_machine_deployment_status_ready_replicas == 0)
      * on(name) group_left(node_group) machine_deployment_node_group_info
    for: 20m
    labels:
      severity_level: "7"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "node_group"
      summary: NodeGroup {{`{{ $labels.node_group }}`}} has no available instances.
      description: |
{{- template "todo_list" }}

  - alert: NodeGroupReplicasUnavailable
    expr: |
      max by (name) (mcm_machine_deployment_status_unavailable_replicas > mcm_machine_deployment_info_spec_rolling_update_max_surge)
      * on(name) group_left(node_group) machine_deployment_node_group_info
    for: 20m
    labels:
      severity_level: "8"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "node_group"
      summary: Too many unavailable instances in the {{`{{ $labels.node_group }}`}} NodeGroup.
      description: |
        The number of simultaneously unavailable instances in the {{`{{ $labels.node_group }}`}} NodeGroup exceeds the allowed threshold.
        This may indicate that the autoscaler has provisioned too many nodes at once. Check the state of the Machine in the cluster.
{{- template "todo_list" }}

  - alert: NodeGroupMasterTaintIsAbsent
    expr: |
      max (d8_nodegroup_taint_missing{name="master"}) > 0
    for: 20m
    labels:
      severity_level: "4"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: The master NodeGroup is missing the required control-plane taint.
      description: |
        The `master` NodeGroup doesn't have the `node-role.kubernetes.io/control-plane: NoSchedule` taint.  
        This may indicate a misconfiguration where control-plane nodes can run non-control-plane Pods.

        To resolve the issue, add the following to the `master` NodeGroup spec:

        ```yaml
          nodeTemplate:
            taints:
            - effect: NoSchedule
              key: node-role.kubernetes.io/control-plane
        ```
        
        Note that the taint `key: node-role.kubernetes.io/master` is deprecated and has no effect starting from Kubernetes 1.24.
