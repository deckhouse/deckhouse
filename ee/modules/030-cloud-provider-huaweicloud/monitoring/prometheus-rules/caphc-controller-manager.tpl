- name: d8.cloud-provider-huaweicloud.capi
  rules:
  - alert: D8HuaweiCloudMachineStuckInDeleting
    expr: caphc_machine_deleting_stuck > 0
    for: 0m
    labels:
      severity_level: "6"
      tier: cluster
      d8_module: cloud-provider-huaweicloud
      d8_component: caphc-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_labels_as_annotations: "name,namespace"
      summary: HuaweiCloudMachine {{`{{ $labels.name }}`}} is stuck in Deleting for more than 24h.
      description: |
        The HuaweiCloudMachine `{{`{{ $labels.name }}`}}` in namespace `{{`{{ $labels.namespace }}`}}` has been stuck in Deleting state for more than 24 hours.

        This usually means the deletion job in HuaweiCloud is failing or the API is unavailable.

        To inspect the machine status, run:

        ```shell
        d8 k get huaweicloudmachine -n {{`{{ $labels.namespace }}`}} {{`{{ $labels.name }}`}} -o yaml
        ```

        Check the controller logs for errors:

        ```shell
        d8 k logs -n d8-cloud-provider-huaweicloud -l app=caphc-controller-manager
        ```
