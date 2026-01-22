{{- $orchestrator := (.Values.registry.internal).orchestrator -}}
{{- if ((($orchestrator).state).node_services).nodes -}}
- name: d8.registry-static-pods.state
  rules:
    - alert: D8RegistryPodIsNotReady
      expr: |-
        min by (pod, namespace, node) (
            kube_pod_status_ready{condition="true",namespace="d8-system",pod=~"^(system|embedded)-registry.+"}
          * on (namespace, pod) group_left(node)
            kube_pod_info{created_by_kind="Node"}
        ) != 1
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: static-pod
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_labels_as_annotations: "pod,namespace,node"
        plk_create_group_if_not_exists__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} static Pod is NOT Ready.
        description: |
          The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} static Pod is NOT Ready.

          The recommended course of action:
          1. View the status of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} describe pod/{{`{{ $labels.pod }}`}}`
          2. View the logs of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} logs pod/{{`{{ $labels.pod }}`}}`
          3. View the logs of the registry manager
            - `kubectl -n d8-system logs pod/registry-manager-*`
            - `kubectl -n d8-system logs $(kubectl -n d8-system get lease/manager-leader -o jsonpath='{.spec.holderIdentity}' | awk -F_ '{print $1}') -f`
          4. View the logs of the registry static pod manager `kubectl -n d8-system logs pod/registry-nodeservices-manager-*`

    - alert: D8RegistryPodIsNotRunning
      expr: |-
        min by (pod, namespace, node) (
            kube_pod_status_phase{phase="Running",namespace="d8-system",pod=~"^registry-.+"}
          * on (namespace, pod) group_left(node)
            kube_pod_info{created_by_kind="Node"}
        ) != 1
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: static-pod
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_labels_as_annotations: "pod,namespace,node"
        plk_create_group_if_not_exists__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} static Pod is NOT Running.
        description: |
          The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} static Pod is NOT Running.

          The recommended course of action:
          1. View the status of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} describe pod/{{`{{ $labels.pod }}`}}`
          2. View the logs of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} logs pod/{{`{{ $labels.pod }}`}}`
          3. View the logs of the registry manager
            - `kubectl -n d8-system logs pod/registry-manager-*`
            - `kubectl -n d8-system logs $(kubectl -n d8-system get lease/manager-leader -o jsonpath='{.spec.holderIdentity}' | awk -F_ '{print $1}') -f`
          4. View the logs of the registry static pod manager `kubectl -n d8-system logs pod/registry-nodeservices-manager-*`

    - alert: D8RegistryPodIsNotRunning
      expr: |-
        absent(kube_pod_status_phase{phase="Running",namespace="d8-system", pod=~"^registry-.*", pod!~"^registry-.*manager.*"})
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: static-pod
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_create_group_if_not_exists__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The d8-system/registry-nodeservices-(node-name) static Pod is NOT Running.
        description: |
          The d8-system/registry-nodeservices-(node-name) static Pod is NOT Running.

          The recommended course of action:
          1. View the status of the Pod: `kubectl -n d8-system describe pod/registry-nodeservices-(node-name)`
          2. View the logs of the Pod: `kubectl -n d8-system logs pod/registry-nodeservices-(node-name)`
          3. View the logs of the registry manager
            - `kubectl -n d8-system logs pod/registry-manager-*`
            - `kubectl -n d8-system logs $(kubectl -n d8-system get lease/registry-manager-leader -o jsonpath='{.spec.holderIdentity}' | awk -F_ '{print $1}') -f`
          4. View the logs of the registry static pod manager `kubectl -n d8-system logs pod/registry-nodeservices-manager-*`

    - alert: D8RegistryPodIsTargetDown
      expr: |
        label_replace(
          up{job=~"^registry-(distribution|auth)$"},
          "host_ip",
          "$1",
          "instance",
          "([^:]+):.*"
        )
        * on (host_ip) group_left (pod, node, namespace)
          kube_pod_info{created_by_kind="Node",namespace="d8-system",pod=~"^registry-.+"}
        == 0
      for: 3m
      labels:
        severity_level: "5"
        d8_module: registry
        d8_component: static-pod
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: "markdown"
        plk_labels_as_annotations: "pod,namespace,node"
        plk_create_group_if_not_exists__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_embedded_registry_health: "D8RegistryHealth,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} static Pod target is down.
        description: |
          The {{`{{ $labels.namespace }}`}}/{{`{{ $labels.pod }}`}} static Pod target is down.

          The recommended course of action:
          1. View the status of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} describe pod/{{`{{ $labels.pod }}`}}`
          2. View the logs of the Pod: `kubectl -n {{`{{ $labels.namespace }}`}} logs pod/{{`{{ $labels.pod }}`}}`
          3. View the logs of the registry manager
            - `kubectl -n d8-system logs pod/registry-manager-*`
            - `kubectl -n d8-system logs $(kubectl -n d8-system get lease/registry-manager-leader -o jsonpath='{.spec.holderIdentity}' | awk -F_ '{print $1}') -f`
          4. View the logs of the registry static pod manager `kubectl -n d8-system logs pod/registry-nodeservices-manager-*`
{{- else }}
[]
{{- end }}
