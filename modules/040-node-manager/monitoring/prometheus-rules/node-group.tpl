{{- define "todo_list" }}
        Probably, machine-controller-manager is unable to create a machine using the cloud provider module. Possible causes:
          1. Cloud provider limits on available resources;
          2. No access to the cloud provider API;
          3. Cloud provider or instance class misconfiguration;
          4. Problems with bootstrapping the Machine.

        The recommended course of action:
          1. Run `kubectl get ng {{`{{ $labels.node_group }}`}} -o yaml`. In the `.status.lastMachineFailures` field you can find all errors related to the creation of Machines;
          2. The absence of Machines in the list that have been in Pending status for more than a couple of minutes means that Machines are continuously being created and deleted because of some error:
          `kubectl -n d8-cloud-instance-manager get machine`;
          3. Refer to the Machine description if the logs do not include error messages and the Machine continues to be Pending:
          `kubectl -n d8-cloud-instance-manager get machine <machine_name> -o json | jq .status.bootstrapStatus`;
          4. The output similar to the one below means that you have to use nc to examine the bootstrap logs:
             ```json
             {
               "description": "Use 'nc 192.168.199.158 8000' to get bootstrap logs.",
               "tcpEndpoint": "192.168.199.158"
             }
             ```
          5. The absence of information about the endpoint for getting logs means that `cloudInit` is not working correctly. This may be due to the incorrect configuration of the instance class for the cloud provider.
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
      plk_create_group_if_not_exists__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,d8_module=node-manager,d8_component=node-group"
      plk_grouped_by__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "node_group"
      summary: There are unavailable instances in the {{`{{ $labels.node_group }}`}} node group.
      description: |
        The number of unavailable instances is {{`{{ $value }}`}}. See the relevant alerts for more information.
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
      plk_create_group_if_not_exists__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,d8_module=node-manager,d8_component=node-group"
      plk_grouped_by__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "node_group"
      summary: There are no available instances in the {{`{{ $labels.node_group }}`}} node group.
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
      plk_create_group_if_not_exists__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,d8_module=node-manager,d8_component=node-group"
      plk_grouped_by__d8_cluster_has_node_groups_with_unavailable_replicas: "ClusterHasNodeGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "node_group"
      summary: The number of simultaneously unavailable instances in the {{`{{ $labels.node_group }}`}} node group exceeds the allowed value.
      description: |
        Possibly, autoscaler has provisioned too many Nodes. Take a look at the state of the Machine in the cluster.
{{- template "todo_list" }}
