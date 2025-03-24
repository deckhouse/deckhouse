- name: coreos.kubelet
  rules:
  - alert: K8SNodeNotReady
    expr: min(kube_node_status_condition{condition="Ready",status="true"}) BY (node) == 0 and
          min(kube_node_spec_unschedulable == 0) by (node)
    for: 10m
    labels:
      severity_level: "3"
    annotations:
      plk_protocol_version: "1"
      summary: The status of node {{ `{{ $labels.node }}` }} is `NotReady`.
      description: |
        The kubelet on node {{ `{{ $labels.node }}` }} has either failed to check in with the API server or has set itself to `NotReady` for more than 10 minutes.
      
  - alert: K8SManyNodesNotReady
    expr: count(kube_node_status_condition{condition="Ready",status="true"} == 0 and on (node) kube_node_spec_unschedulable == 0) > 1
      and (count(kube_node_status_condition{condition="Ready",status="true"} == 0 and on (node) kube_node_spec_unschedulable == 0) /
      count(kube_node_status_condition{condition="Ready",status="true"} and on (node) kube_node_spec_unschedulable == 0)) > 0.2
    for: 1m
    labels:
      severity_level: "3"
    annotations:
      plk_protocol_version: "1"
      summary: Too many nodes are not ready.
      description: '{{ `{{ $value }}` }}% of Kubernetes nodes are not ready.'
      
  - alert: K8SKubeletDown
    expr: (count(up{job="kubelet"} == 0) or absent(up{job="kubelet"} == 1)) / count(up{job="kubelet"}) * 100 > 3
    for: 10m
    labels:
      severity_level: "4"
      tier: "cluster"
    annotations:
      plk_protocol_version: "1"
      plk_group_for__target_down: "TargetDown,prometheus=deckhouse,job=kubelet,kubernetes=~kubernetes"
      summary: Several kubelets couldn't be scraped.
      description: Prometheus failed to scrape {{ `{{ $value }}` }}% of kubelets.
      
  - alert: K8SKubeletDown
    expr: (count(up{job="kubelet"} == 0) or absent(up{job="kubelet"} == 1)) / count(up{job="kubelet"}) * 100 > 10
    for: 30m
    labels:
      severity_level: "3"
      tier: "cluster"
    annotations:
      plk_protocol_version: "1"
      plk_group_for__target_down: "TargetDown,prometheus=deckhouse,job=kubelet,kubernetes=~kubernetes"
      summary: Multiple kubelets couldn't be scraped.
      description: Prometheus failed to scrape {{ `{{ $value }}` }}% of kubelets.
      
  - alert: K8SKubeletTooManyPods
    expr: kubelet_running_pods > on(node) (kube_node_status_capacity{job="kube-state-metrics",resource="pods",unit="integer"}) * 0.9
    for: 10m
    labels:
      severity_level: "7"
    annotations:
      plk_protocol_version: "1"
      summary: The kubelet on node {{ `{{ $labels.node }}` }} is approaching the pod limit.
      description: The kubelet on node {{ `{{ $labels.node }}` }} is running {{ `{{ $value }}` }} pods, which is close
        to the limit of {{ `{{ printf "kube_node_status_capacity{job=\"kube-state-metrics\",resource=\"pods\",unit=\"integer\",node=\"%s\"}" $labels.node | query | first | value }}` }}.
