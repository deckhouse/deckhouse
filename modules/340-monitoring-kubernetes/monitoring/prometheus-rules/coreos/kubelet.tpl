- name: coreos.kubelet
  rules:
  - alert: K8SNodeNotReady
    expr: min(kube_node_status_condition{condition="Ready",status="true"}) BY (node) == 0
    for: 10m
    labels:
      severity_level: "3"
    annotations:
      plk_protocol_version: "1"
      description: The Kubelet on {{ `{{ $labels.node }}` }} has not checked in with the API,
        or has set itself to NotReady, for more than 10 minutes
      summary: Node status is NotReady
  - alert: K8SManyNodesNotReady
    expr: count(kube_node_status_condition{condition="Ready",status="true"} == 0)
      > 1 and (count(kube_node_status_condition{condition="Ready",status="true"} ==
      0) / count(kube_node_status_condition{condition="Ready",status="true"})) > 0.2
    for: 1m
    labels:
      severity_level: "3"
    annotations:
      plk_protocol_version: "1"
      description: '{{ `{{ $value }}` }}% of Kubernetes nodes are not ready'
      summary: Too many nodes are not ready
  - alert: K8SKubeletDown
    expr: (count(up{job="kubelet"} == 0) or absent(up{job="kubelet"} == 1)) / count(up{job="kubelet"}) * 100 > 3
    for: 10m
    labels:
      severity_level: "4"
      tier: "cluster"
    annotations:
      plk_protocol_version: "1"
      plk_group_for__target_down: "TargetDown,prometheus=deckhouse,job=kubelet,kubernetes=~kubernetes"
      description: Prometheus failed to scrape {{ `{{ $value }}` }}% of kubelets.
      summary: A few kubelets cannot be scraped
  - alert: K8SKubeletDown
    expr: (count(up{job="kubelet"} == 0) or absent(up{job="kubelet"} == 1)) / count(up{job="kubelet"}) * 100 > 10
    for: 30m
    labels:
      severity_level: "3"
      tier: "cluster"
    annotations:
      plk_protocol_version: "1"
      plk_group_for__target_down: "TargetDown,prometheus=deckhouse,job=kubelet,kubernetes=~kubernetes"
      description: Prometheus failed to scrape {{ `{{ $value }}` }}% of kubelets.
      summary: Many kubelets cannot be scraped
  - alert: K8SKubeletTooManyPods
{{- if semverCompare "<1.19" .Values.global.discovery.kubernetesVersion }}
    expr: kubelet_running_pod_count > on(node) kube_node_status_capacity_pods * 0.9
{{- else }}
    expr: kubelet_running_pods > on(node) kube_node_status_capacity_pods * 0.9
{{- end }}
    for: 10m
    labels:
      severity_level: "7"
    annotations:
      plk_protocol_version: "1"
      description: Kubelet {{ `{{ $labels.node }}` }} is running {{ `{{ $value }}` }} pods, close
        to the limit of {{ `{{ print "kube_node_status_capacity_pods{node='" $labels.node "'}" | query | first | value }}` }}
      summary: Kubelet is close to pod limit
