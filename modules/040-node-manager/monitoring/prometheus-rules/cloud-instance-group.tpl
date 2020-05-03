{{- define "todo_list" }}
        Скорее всего, machine controller manager не может создать machine через cloud provider. Возможные причины:
          1. Уперлись в лимиты cloud provider по доступным ресурсам
          2. Недоступно api cloud provider
          3. Неправильно сконфигурирован cloud provider или instance class
          4. Проблемы с bootstrap'ом machine'ы

        Необходимо выполнить следующие действия:
          1. `kubectl -n d8-cloud-instance-manager logs -f -l app=machine-controller-manager -c controller`
          2. Если в логах видно, что machine постоянно создаются и удаляются из-за какой-то ошибки, то при получении списка
          machine вы увидете, что нет ни одной machine, которая находится в Pending больше пары минут
          `kubectl -n d8-cloud-instance-manager get machine`
          3. Если ошибок в логах нет и machine висят в pending, то надо посмотреть описание machine
          `kubectl -n d8-cloud-instance-manager get machine <machine_name> -o json | jq .status.bootstrapStatus`
          4. Если вы увидели вот такой вывод, то используйте nc, чтобы проветь логи bootstrap
          ```
          {
            "description": "Use 'nc 192.168.199.158 8000' to get bootstrap logs.",
            "tcpEndpoint": "192.168.199.158"
          }
          ```
          5. Если в выводе нет информации об endpoint для получнеия логов, то это значит, что cloudInit работает некорректно.
          Bозможные проблемы: неправильная конфигурация instance class для cloud provider.
{{- end }}

- name: d8.cloud-instance-group
  rules:
  - alert: CloudInstanceGroupReplicasUnavailable
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
      plk_grouped_by__cluster_has_cloud_node_groups_with_unavailable_replicas: "ClusterHasCloudInstanceGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "node_group"
      summary: В cloud instance group {{`{{ $labels.node_group }}`}} есть недоступные инстансы
      description: |
        Количество недоступных инстансов: {{`{{ $value }}`}}. Более подробная информация в связанных алертах.
{{- template "todo_list" }}

  - alert: CloudInstanceGroupReplicasUnavailable
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
      plk_grouped_by__cluster_has_cloud_node_groups_with_unavailable_replicas: "ClusterHasCloudInstanceGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "node_group"
      summary: В cloud instance group {{`{{ $labels.node_group }}`}} нет ни одного доступного инстанса
      description: |
{{- template "todo_list" }}

  - alert: CloudInstanceGroupReplicasUnavailable
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
      plk_grouped_by__cluster_has_cloud_node_groups_with_unavailable_replicas: "ClusterHasCloudInstanceGroupsWithUnavailableReplicas,tier=cluster,prometheus=deckhouse"
      plk_labels_as_annotations: "node_group"
      summary: В cloud instance group {{`{{ $labels.node_group }}`}} количество одновременно недоступных инстансов превышает допустимое значение.
      description: |
        Возможно, autoscaler заказал большое количество нод. Обратите внимание на состояние machine в кластере.
{{- template "todo_list" }}

  - alert: ClusterHasCloudInstanceGroupsWithUnavailableReplicas
    expr: count(max by (node_group) (ALERTS{alertname="CloudInstanceGroupReplicasUnavailable", alertstate="firing"})) > 0
    labels:
      tier: cluster
    annotations:
      plk_markup_format: markdown
      plk_protocol_version: "1"
      plk_alert_type: "group"
      summary: В кластере есть несколько cloud instance group c недоступными инстансами.
      description: |
        Подробную информацию можно получить в одном из связанных алертов.
