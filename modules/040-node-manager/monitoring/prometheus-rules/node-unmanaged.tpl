- name: d8.node-unmanaged
  rules:
    - alert: D8NodeIsUnmanaged
      expr: max by (node) (d8_unmanaged_nodes_on_cluster) > 0
      for: 10m
      labels:
        tier: cluster
        severity_level: "9"
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_incident_initial_status: "todo"
        plk_grouped_by__main: "D8ClusterHasUnmanagedNodes,tier=cluster,prometheus=deckhouse"
    {{- if .Values.global.modules.publicDomainTemplate }}
        summary: Нода {{`{{ $labels.node }}`}} находится не под управлением модуля [node-manager]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/).
        description: |
          Нода {{`{{ $labels.node }}`}} находистя не под управлением модуля [node-manager]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "deckhouse") }}/modules/040-node-manager/).
    {{- else }}
        summary: Нода {{`{{ $labels.node }}`}} находится не под управлением модуля `node-manager`.
        description: |
          Нода {{`{{ $labels.node }}`}} находится не под управлением модуля `node-manager`.
    {{- end }}

          Что необходимо сделать:
          - Создать `NodeGroup` в которой будет жить нода или выбрать из существующих;
          - Навесить лейбл `node.deckhouse.io/group: <nodeGroup_name>`: `kubectl label node {{`{{ $labels.node }}`}} node.deckhouse.io/group=<nodeGroup_name>`
          - Получить скрипт для адопта ноды: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-<nodeGroup_name> -o json | jq '.data."adopt.sh"' -r`
          - Выполнить данный `base64` на ноде {{`{{ $labels.node }}`}}: `echo <base64_string> | base64 -d | bash`
          - Посмотреть лог выполнения: `journalctl -fu bashible`
    - alert: D8ClusterHasUnmanagedNodes
      expr: count(ALERTS{alertname="D8NodeIsUnmanaged", alertstatie="firing"}) > 0
      for: 10m
      labels:
        tier: cluster
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_alert_type: "group"
        summary: В кластере есть ноды не под управлением `node-manager`. Подробную информацию можно увидеть в связанных алертах.
        description: В кластере есть ноды не под управлением `node-manager`. Подробную информацию можно увидеть в связанных алертах.

