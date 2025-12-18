- name: cloud-data-discoverer.general
  rules:
  - alert: D8CloudDataDiscovererCloudRequestError
    for: 1h
    expr: max by(job, namespace)(cloud_data_discovery_cloud_request_error == 1)
    labels:
      severity_level: "6"
      d8_module: node-manager
      d8_component: cloud-data-discoverer
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Cloud data discoverer cannot get data from the cloud.
      plk_create_group_if_not_exists__malfunctioning: "D8CloudDataDiscovererMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__malfunctioning: "D8CloudDataDiscovererMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Refer to the cloud data discoverer's logs for details:

        ```shell
        d8 k -n {{`{{ $labels.namespace }}`}} logs deploy/cloud-data-discoverer
        ```

  - alert: D8CloudDataDiscovererSaveError
    for: 1h
    expr: max by(job, namespace)(cloud_data_discovery_update_resource_error == 1)
    labels:
      severity_level: "6"
      d8_module: node-manager
      d8_component: cloud-data-discoverer
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Cloud data discoverer cannot save data to a Kubernetes resource.
      plk_create_group_if_not_exists__malfunctioning: "D8CloudDataDiscovererMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__malfunctioning: "D8CloudDataDiscovererMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Refer to the cloud data discoverer's logs for details:

        ```shell
        d8 k -n {{`{{ $labels.namespace }}`}} logs deploy/cloud-data-discoverer
        ```

  - alert: ClusterHasOrphanedDisks
    for: 1h
    expr: max by(job, id, name, namespace)(cloud_data_discovery_orphaned_disk_info == 1)
    labels:
      severity_level: "6"
      d8_module: node-manager
      d8_component: cloud-data-discoverer
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Cloud data discoverer found orphaned disks in the cloud.
      plk_create_group_if_not_exists__main: "ClusterHasCloudDataDiscovererAlerts,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__main: "ClusterHasCloudDataDiscovererAlerts,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        The cloud data discoverer has found disks in the cloud that do not have a corresponding PersistentVolume in the cluster.

        You can safely delete these disks manually from your cloud provider:

        ID: {{`{{ $labels.id }}`}}, Name: {{`{{ $labels.name }}`}}

  - alert: UnmetCloudConditions
    for: 1h
    expr: max by(job, id, name, namespace)(cloud_data_discovery_cloud_conditions_error == 1)
    labels:
      severity_level: "6"
      d8_module: node-manager
      d8_component: cloud-data-discoverer
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Deckhouse update is unavailable due to unmet cloud provider conditions.
      plk_create_group_if_not_exists__main: "ClusterHasUnmetCloudConditionAlerts,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__main: "ClusterHasUnmetCloudConditionAlerts,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Deckhouse has detected that some cloud provider–specific conditions have not been met.
        Until these issues are resolved, updating to the new Deckhouse release is not possible.

        Troubleshooting details:

        - Name: {{`{{ $labels.name }}`}}
        - Message: {{`{{ $labels.message }}`}}
