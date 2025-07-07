{{- $orchestrator := (.Values.registry.internal).orchestrator -}}
{{- if ((($orchestrator).state).node_services).run -}}
- name: d8.registry-manager.state
  rules:
    # Manager && StaticPod Manager
    - alert: D8RegistryManagerPodIsNotReady
      expr: |-
        min by (pod, namespace, node, created_by_kind, created_by_name) (
            kube_pod_status_ready{condition="true",namespace="d8-system",pod=~"^registry.+"}
          * on (namespace, pod) group_left(node, created_by_kind, created_by_name)
            kube_pod_info{created_by_kind!="Node"}
        ) != 1
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: manager
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_labels_as_annotations: "pod,namespace,node"
        plk_create_group_if_not_exists__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} Pod is NOT Ready.
        description: |
          The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} Pod is NOT Ready.

          The recommended course of action:
          1. Retrieve details of the {{`{{ $labels.created_by_kind }}`}}: `kubectl -n {{`{{ $labels.namespace }}`}} describe {{`{{ $labels.created_by_kind }}`}}/{{`{{ $labels.created_by_name }}`}}`
          2. View the status of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} describe pod/{{`{{ $labels.pod }}`}}`
          3. View the logs of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} logs pod/{{`{{ $labels.pod }}`}}`

    # Manager && StaticPod Manager
    - alert: D8RegistryManagerPodIsNotRunning
      expr: |-
        min by (pod, namespace, node, created_by_kind, created_by_name) (
            kube_pod_status_phase{phase="Running",namespace="d8-system",pod=~"^registry-.+"}
          * on (namespace, pod) group_left(node, created_by_kind, created_by_name)
            kube_pod_info{created_by_kind!="Node"}
        ) != 1
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: manager
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_labels_as_annotations: "pod,namespace,node"
        plk_create_group_if_not_exists__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} Pod is NOT Running.
        description: |
          The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} Pod is NOT Running.

          The recommended course of action:
          1. Retrieve details of the {{`{{ $labels.created_by_kind }}`}}: `kubectl -n {{`{{ $labels.namespace }}`}} describe {{`{{ $labels.created_by_kind }}`}}/{{`{{ $labels.created_by_name }}`}}`
          2. View the status of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} describe pod/{{`{{ $labels.pod }}`}}`
          3. View the logs of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} logs pod/{{`{{ $labels.pod }}`}}`

    # Manager
    - alert: D8RegistryManagerPodIsNotRunning
      expr: |-
        absent(kube_pod_status_phase{namespace="d8-system",phase="Running",pod=~"^registry-manager.*"})
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: manager
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_create_group_if_not_exists__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The d8-system/registry-manager Pod is NOT Running.
        description: |
          The d8-system/registry-manager Pod is NOT Running.

          The recommended course of action:
          1. Retrieve details of the Daemonset: `kubectl -n d8-system describe Daemonset/registry-manager`
          2. View the status of the Pod: `kubectl -n d8-system describe pod/registry-manager-*`
          3. View the logs of the Pod: `kubectl -n d8-system logs pod/registry-manager-*`

    # StaticPod Manager
    - alert: D8RegistryManagerPodIsNotRunning
      expr: |-
        absent(kube_pod_status_phase{namespace="d8-system",phase="Running",pod=~"^registry-staticpod-manager.*"})
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: manager
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_create_group_if_not_exists__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_registry_manager_health: "D8RegistryManagerHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The d8-system/registry-staticpod-manager Pod is NOT Running.
        description: |
          The d8-system/registry-staticpod-manager Pod is NOT Running.

          The recommended course of action:
          1. Retrieve details of the Daemonset: `kubectl -n d8-system describe Daemonset/registry-staticpod-manager`
          2. View the status of the Pod: `kubectl -n d8-system describe pod/registry-staticpod-manager-*`
          3. View the logs of the Pod: `kubectl -n d8-system logs pod/registry-staticpod-manager-*`
{{- else }}
[]
{{- end }}
