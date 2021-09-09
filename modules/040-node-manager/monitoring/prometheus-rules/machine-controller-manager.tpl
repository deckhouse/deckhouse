{{- if include "machine_controller_manager_enabled" . }}
- name: d8.machine-controller-manager.availability
  rules:
  - alert: D8MachineControllerManagerPodIsNotReady
    expr: min by (pod) (kube_pod_status_ready{condition="false", namespace="d8-cloud-instance-manager", pod=~"machine-controller-manager-.*"}) > 0
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_pending_until_firing_for: "10m"
      plk_grouped_by__d8_machine_controller_manager_unavailable: "D8MachineControllerManagerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "pod"
      summary: The {{`{{$labels.pod}}`}} Pod is NOT Ready.

  - alert: D8MachineControllerManagerPodIsNotRunning
    expr: max by (namespace, pod, phase) (kube_pod_status_phase{namespace="d8-cloud-instance-manager",phase!="Running",pod=~"machine-controller-manager-.*"} > 0)
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_pending_until_firing_for: "10m"
      plk_grouped_by__d8_machine_controller_manager_unavailable: "D8MachineControllerManagerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "phase"
      summary: The machine-controller-manager Pod is NOT Running.
      description: |-
        The {{`{{$labels.pod}}`}} Pod is {{`{{$labels.phase}}`}}.

        Run the following command to check the status of the Pod: `kubectl -n {{`{{$labels.namespace}}`}} get pods {{`{{$labels.pod}}`}} -o json | jq .status`.

  - alert: D8MachineControllerManagerTargetDown
    expr: max by (job) (up{job="machine-controller-manager", namespace="d8-cloud-instance-manager"} == 0)
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: deckhouse
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_pending_until_firing_for: "5m"
      plk_grouped_by__d8_machine_controller_manager_unavailable: "D8MachineControllerManagerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "instance,pod"
      plk_ignore_labels: "job"
      summary: Prometheus is unable to scrape machine-controller-manager's metrics.

  - alert: D8MachineControllerManagerTargetAbsent
    expr: absent(up{job="machine-controller-manager", namespace="d8-cloud-instance-manager"} == 1)
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: prometheus
      d8_component: machine-controller-manager
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_pending_until_firing_for: "5m"
      plk_grouped_by__d8_machine_controller_manager_unavailable: "D8MachineControllerManagerUnavailable,tier=cluster,prometheus=deckhouse"
      summary: There is no machine-controller-manager target in Prometheus.
      description: |-
        Machine controller manager manages ephemeral Nodes in the cluster. Its unavailability will result in the inability to add/delete Nodes.

        The recommended course of action:
        1. Check the availability and status of `machine-controller-manager` Pods: `kubectl -n d8-cloud-instance-manager get pods -l app=machine-controller-manager`;
        2. Check the availability of the `machine-controller-manager` Deployment: `kubectl -n d8-cloud-instance-manager get deploy machine-controller-manager`;
        3. Check the status of the `machine-controller-manager` Deployment: `kubectl -n d8-cloud-instance-manager describe deploy machine-controller-manager`.

  - alert: D8MachineControllerManagerUnavailable
    expr: |
      count(ALERTS{alertname=~"D8MachineControllerManagerPodIsNotReady|D8MachineControllerManagerPodIsNotRunning|D8MachineControllerManagerTargetAbsent|D8MachineControllerManagerTargetDown", alertstate="firing"}) > 0
      OR
      count(ALERTS{alertname=~"KubernetesDeploymentReplicasUnavailable", namespace="d8-cloud-instance-manager", deployment="machine-controller-manager", alertstate="firing"}) > 0
      OR
      count(ALERTS{alertname=~"KubernetesDeploymentStuck", namespace="d8-cloud-instance-manager", deployment="machine-controller-manager", alertstate="firing"}) > 0
    labels:
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      plk_group_for__machine_controller_manager_replicas_unavailable: "KubernetesDeploymentReplicasUnavailable,namespace=d8-cloud-instance-manager,prometheus=deckhouse,deployment=machine-controller-manager"
      plk_group_for__machine_controller_manager_stuck: "KubernetesDeploymentStuck,namespace=d8-cloud-instance-manager,prometheus=deckhouse,deployment=machine-controller-manager"
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse"
      summary: The machine-controller-manager is down.
      description: |
        The `machine-controller-manager` is down. You can find out the exact problem in the relevant alerts.

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
      plk_grouped_by__d8_machine_controller_manager_malfunctioning: "D8MachineControllerManagerMalfunctioning,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "pod"
      summary: The machine-controller-manager module: too many restarts.
      description: |
        The number of restarts in the last hour: {{`{{ $value }}`}}.

        Excessive machine-controller-manager restarts indicate that something is wrong. Normally, it should be up and running all the time.

        Please, refer to the logs: `kubectl -n d8-cloud-instance-manager logs -f -l app=machine-controller-manager -c controller`.

  - alert: D8MachineControllerManagerMalfunctioning
    expr: |
      count(ALERTS{alertname=~"D8MachineControllerManagerPodIsRestartingTooOften", alertstate="firing"}) > 0
    labels:
      tier: cluster
      d8_module: node-manager
      d8_component: machine-controller-manager
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      summary: Machine-controller-manager does not work as expected.
      description: |
        Machine-controller-manager does not work as expected. The detailed information is available in one of the relevant alerts.
{{- else }}
[]
{{- end }}
