- name: cri.version
  rules:
  - alert: UnsupportedContainerRuntimeVersion
{{- if and (semverCompare ">=1.16" .Values.global.discovery.kubernetesVersion) (semverCompare "<1.20" .Values.global.discovery.kubernetesVersion) }}
    expr: sum by (container_runtime_version, job, kernel_version, kubelet_version, kubeproxy_version, node, os_image) (kube_node_info{kubelet_version=~"v1.1[4-9].+", container_runtime_version!~"docker://1\\.13\\..*|docker://1[7-9]\\..*|docker://3\\..*|containerd://1\\.4\\..*"})
{{- else if and (semverCompare ">=1.20" .Values.global.discovery.kubernetesVersion) (semverCompare "<1.30" .Values.global.discovery.kubernetesVersion) }}
    expr: sum by (container_runtime_version, job, kernel_version, kubelet_version, kubeproxy_version, node, os_image) (kube_node_info{kubelet_version=~"v1.2[0-9].+", container_runtime_version!~"docker://1\\.13\\..*|docker://1[7-9]\\..*|docker://3\\..*|containerd://1\\.4\\..*"})
{{- end }}
    for: 20m
    labels:
      impact: negligible
      likelihood: certain
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: markdown
      plk_incident_initial_status: "todo"
      description: |-
        Unsupported version {{`{{$labels.container_runtime_version}}`}} of CRI installed on {{`{{$labels.node}}`}} node.
        Supported version of CRI for kubernetes {{`{{$labels.kubelet_version}}`}} version:
        * Docker 1.13.x
        * Docker 17.x
        * Docker 18.x
        * Docker 19.x
        * Containerd 1.4.*
        * 3.x (for moby project in Azure)
      summary: >
        Unsupported version of CRI {{`{{$labels.container_runtime_version}}`}} installed for Kubernetes version: {{`{{$labels.kubelet_version}}`}}
  - alert: DeprecatedDockerContainerRuntime
    expr: sum by (container_runtime_version, node) (kube_node_info{container_runtime_version=~"docker://.*"})
    labels:
      tier: cluster
      severity_level: "9"
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: markdown
      plk_incident_initial_status: "todo"
      description: |-
        Found docker CRI installed on {{`{{$labels.node}}`}} node.
        Docker runtime is deprecated and will be removed in the nearest future.
        You should migrate to Containerd CRI.
      summary: >
        Deprecated version of CRI {{`{{$labels.container_runtime_version}}`}} installed on {{`{{$labels.node}}`}} node.
