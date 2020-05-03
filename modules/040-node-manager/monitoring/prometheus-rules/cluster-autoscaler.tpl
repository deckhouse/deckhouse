{{- if include "cluster_autoscaler_enabled" . }}
- name: d8.cluster-autoscaler.availability
  rules:
  - alert: D8ClusterAutoscalerManagerPodIsNotReady
    expr: min by (pod) (kube_pod_status_ready{condition="false", namespace="d8-cloud-instance-manager", pod=~"cluster-autoscaler-.*"}) > 0
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_pending_until_firing_for: "10m"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "pod"
      summary: Под {{`{{$labels.pod}}`}} находится в состоянии НЕ Ready

  - alert: D8ClusterAutoscalerPodIsNotRunning
    expr: max by (namespace, pod, phase) (kube_pod_status_phase{namespace="d8-cloud-instance-manager",phase!="Running",pod=~"cluster-autoscaler-.*"} > 0)
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_pending_until_firing_for: "10m"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "phase"
      summary: Под cluster-autoscaler находится в состоянии НЕ Running
      description: |-
        Под {{`{{$labels.pod}}`}} находится в состоянии {{`{{$labels.phase}}`}}. Для проверки статуса пода необходимо выполнить:
        1. `kubectl -n {{`{{$labels.namespace}}`}} get pods {{`{{$labels.pod}}`}} -o json | jq .status`

  - alert: D8ClusterAutoscalerTargetDown
    expr: max by (job) (up{job="cluster-autoscaler", namespace="d8-cloud-instance-manager"} == 0)
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: deckhouse
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_pending_until_firing_for: "5m"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "instance,pod"
      plk_ignore_labels: "job"
      summary: Prometheus не может получить метрики cluster autoscaler'a.

  - alert: D8ClusterAutoscalerTargetAbsent
    expr: absent(up{job="cluster-autoscaler", namespace="d8-cloud-instance-manager"} == 1)
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: prometheus
      d8_component: cluster-autoscaler
    annotations:
      plk_markup_format: "markdown"
      plk_protocol_version: "1"
      plk_pending_until_firing_for: "5m"
      plk_grouped_by__d8_cluster_autoscaler_unavailable: "D8ClusterAutoscalerUnavailable,tier=cluster,prometheus=deckhouse"
      summary: >
        В таргетах prometheus нет cluster-autoscaler
      description: |-
        Cluster autoscaler используется для автоматического скейлинга нод в кластере, его недоступность не позволит увеличить
        количество нод, если будет нехватать ресурсов для scheduling'a подов. Также недоступность cluster-autoscaler
        может привести к лишним затратам на инстансы в cloud'e, от которых можно отказаться, так как они не утилизируются.

        Необходимо выполнить следующие действия:
        1. Проверить наличие и состояние подов cluster-autoscaler `kubectl -n d8-cloud-instance-manager get pods -l app=cluster-autoscaler`
        2. Проверить наличие deployment'a cluster-autoscaler `kubectl -n d8-cloud-instance-manager get deploy cluster-autoscaler`
        3. Посмотреть состояние deployment'a cluster-autoscaler `kubectl -n d8-cloud-instance-manager describe deploy cluster-autoscaler`

  - alert: D8ClusterAutoscalerUnavailable
    expr: |
      (count(ALERTS{alertname=~"D8ClusterAutoscalerManagerPodIsNotReady|D8ClusterAutoscalerPodIsNotRunning|D8ClusterAutoscalerTargetAbsent|D8ClusterAutoscalerTargetDown", alertstate="firing"})
      +
      count(ALERTS{alertname=~"KubernetesDeploymentReplicasUnavailable", namespace="d8-cloud-instance-manager", deployment="cluster-autoscaler", alertstate="firing"})
      +
      count(ALERTS{alertname=~"KubernetesDeploymentStuck", namespace="d8-cloud-instance-manager", deployment="cluster-autoscaler", alertstate="firing"})) > 1
    labels:
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      plk_group_for__cluster_autoscaler_replicas_unavailable: "KubernetesDeploymentReplicasUnavailable,namespace=d8-cloud-instance-manager,prometheus=deckhouse,deployment=cluster-autoscaler"
      plk_group_for__cluster_autoscaler_stuck: "KubernetesDeploymentStuck,namespace=d8-cloud-instance-manager,prometheus=deckhouse,deployment=cluster-autoscaler"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse"
      summary: Cluster autoscaler не работает
      description: |
        Cluster autoscaler не работает. Что именно с ним не так можно узнать в одном из связанных алертов.

- name: d8.cluster-autoscaler.malfunctioning
  rules:
  - alert: D8ClusterAutoscalerPodIsRestartingTooOften
    expr: max by (pod) (increase(kube_pod_container_status_restarts_total{namespace="d8-cloud-instance-manager", pod=~"cluster-autoscaler-.*"}[1h]) and kube_pod_container_status_restarts_total{namespace="d8-cloud-instance-manager", pod=~"cluster-autoscaler-.*"}) > 5
    labels:
      severity_level: "9"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "pod"
      summary: Cluster autoscaler слишком часто перезагружается
      description: |
        Количество перезапусков за последний час: {{`{{ $value }}`}}.

        Частый перезапуск Cluster autoscaler не является нормальной ситуацией, он должен быть постоянно запущена и работать.
        Необходимо посмотреть логи:
        1. `kubectl -n d8-cloud-instance-manager logs -f -l app=cluster-autoscaler -c controller`

  - alert: D8ClusterAutoscalerTooManyErrors
    expr: sum by(instance) (increase(cluster_autoscaler_errors_total[20m]) > 5)
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_grouped_by__d8_cluster_autoscaler_malfunctioning: "D8ClusterAutoscalerMalfunctioning,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "instance"
      summary: Слишком много ошибок в работе сluster autoscaler
      description: |
        Cluster autoscaler получил ошибку от cloud provider при попытке скейлинга в кластере.

        Необходимо посмотреть логи:
        1. `kubectl -n d8-cloud-instance-manager logs -f -l app=cluster-autoscaler -c cluster-autoscaler`

  - alert: D8ClusterAutoscalerMalfunctioning
    expr: |
      count(ALERTS{alertname=~"D8ClusterAutoscalerPodIsRestartingTooOften|D8ClusterAutoscalerTooManyErrors", alertstate="firing"}) > 1
    labels:
      tier: cluster
      d8_module: node-manager
      d8_component: cluster-autoscaler
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_alert_type: "group"
      summary: Cluster autoscaler работает некорректно
      description: |
        Cluster autoscaler работает некорректно. Что именно с ним не так можно узнать в одном из связанных алертов.
{{- else }}
[]
{{- end }}
