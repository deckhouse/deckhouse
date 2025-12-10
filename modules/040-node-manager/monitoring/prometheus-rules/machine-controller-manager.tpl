{{- if include "machine_controller_manager_enabled" . }}
- name: d8.machine-controller-manager.availability
  rules:
  - alert: D8MachineControllerManagerPodIsNotReady
    expr: min by (pod) (kube_pod_status_ready{condition="false", namespace="d8-cloud-instance-manager", pod=~"machine-controller-manager-.*"}) > 0
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: The {{`{{$labels.pod}}`}} Pod is NOT Ready.

  - alert: D8MachineControllerManagerPodIsNotRunning
    expr: absent(kube_pod_status_phase{namespace="d8-cloud-instance-manager",phase="Running",pod=~"machine-controller-manager-.*"})
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "phase"
      summary: The machine-controller-manager Pod is NOT Running.
      description: |-
        The {{`{{$labels.pod}}`}} Pod is {{`{{$labels.phase}}`}}.

        To check the Pod's status, run the following command:

        ```shell
        d8 k -n {{`{{$labels.namespace}}`}} get pods {{`{{$labels.pod}}`}} -o json | jq .status
        ```

  - alert: D8MachineControllerManagerTargetDown
    expr: max by (job) (up{job="machine-controller-manager", namespace="d8-cloud-instance-manager"} == 0)
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: deckhouse
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "instance,pod"
      plk_ignore_labels: "job"
      summary: Prometheus is unable to scrape the machine-controller-manager's metrics.

  - alert: D8MachineControllerManagerTargetAbsent
    expr: absent(up{job="machine-controller-manager", namespace="d8-cloud-instance-manager"} == 1)
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: prometheus
      d8_component: machine-controller-manager
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_create_group_if_not_exists__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Machine-controller-manager target is missing in Prometheus.
      description: |-
        `Machine-controller-manager` controls ephemeral nodes in the cluster.
        If it becomes unavailable, it will be impossible to create or delete nodes in the cluster.

        To resolve the issue, follow these steps:

        1. Check availability and status of `machine-controller-manager` Pods:

           ```shell
           d8 k -n d8-cloud-instance-manager get pods -l app=machine-controller-manager
           ```

        2. Verify availability of the `machine-controller-manager` Deployment:

           ```shell
           d8 k -n d8-cloud-instance-manager get deployment machine-controller-manager
           ```

        3. Check the Deployment’s status:

           ```shell
           d8 k -n d8-cloud-instance-manager describe deployment machine-controller-manager
           ```

- name: d8.machine-controller-manager.malfunctioning
  rules:
  - alert: D8MachineControllerManagerPodIsRestartingTooOften
    expr: max by (pod) (increase(kube_pod_container_status_restarts_total{namespace="d8-cloud-instance-manager", pod=~"machine-controller-manager-.*"}[1h]) and kube_pod_container_status_restarts_total{namespace="d8-cloud-instance-manager", pod=~"machine-controller-manager-.*"}) > 5
    labels:
      severity_level: "9"
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: Too many machine-controller-manager restarts detected.
      description: |
        The `machine-controller-manager` has restarted {{`{{ $value }}`}} times in the past hour.

        Frequent restarts may indicate a problem.
        The `machine-controller-manager` is expected to run continuously without interruption.

        Check the logs for details:

        ```shell
        d8 k -n d8-cloud-instance-manager logs -f -l app=machine-controller-manager -c controller
        ```

{{- else }}
[]
{{- end }}
