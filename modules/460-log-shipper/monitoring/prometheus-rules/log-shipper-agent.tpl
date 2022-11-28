- name: log-shipper-agent
  rules:
  - alert: D8LogShipperAgentNotScheduledInCluster
    for: 15m
    expr: |
      kube_daemonset_status_desired_number_scheduled{daemonset="log-shipper-agent", namespace="d8-log-shipper", job="kube-state-metrics"}
      -
      kube_daemonset_status_current_number_scheduled{daemonset="log-shipper-agent", namespace="d8-log-shipper", job="kube-state-metrics"}
      > 0
    labels:
      severity_level: "7"
      d8_module: log-shipper
      d8_component: agent
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Pods of log-shipper-agent cannot be scheduled in the cluster.
      description: |
        A number of log-shipper-agents are not scheduled.

        Consider checking state of the d8-log-shipper/log-shipper-agent DaemonSet.
        `kubectl -n d8-log-shipper get daemonset,pod --selector=app=log-shipper-agent`

  - alert: D8LogShipperAgentDoesNotSendLogs
    for: 15m
    expr: |
      sum by (node, component_id) (
        rate(vector_events_out_total{component_kind="sink", component_id=~"destination/.*"}[__SCRAPE_INTERVAL_X_4__])
      ) * on (node) (abs(kube_node_spec_unschedulable - 1))
      == 0
    labels:
      severity_level: "4"
      d8_module: log-shipper
      d8_component: agent
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Pods of log-shipper-agent cannot send logs to the `{{ `{{ $labels.component_id }}` }}` on the `{{ `{{ $labels. }}` }}` node.
      plk_create_group_if_not_exists__malfunctioning: "D8LogShipperMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__malfunctioning: "D8LogShipperMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Logs do not reach their destination, log-shipper agent on the `{{ `{{ $labels.node }}` }}` node cannot send logs for more than 15 minutes.

        Consider checking logs of the pod or follow advanced debug instructions.
        `kubectl -n d8-log-shipper get pods -o wide | grep {{ `{{ $labels.node }}` }}`

  - alert: D8LogShipperLogsDroppedByRateLimit
    for: 15m
    expr: |
      sum by (node, component_id) (
        rate(vector_events_discarded_total[__SCRAPE_INTERVAL_X_4__])
      ) * on (node) (abs(kube_node_spec_unschedulable - 1))
      > 0
    labels:
      severity_level: "4"
      d8_module: log-shipper
      d8_component: agent
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Pods of log-shipper-agent drop logs to the `{{ `{{ $labels.component_id }}` }}` on the `{{ `{{ $labels. }}` }}` node.
      plk_create_group_if_not_exists__malfunctioning: "D8LogShipperMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__malfunctioning: "D8LogShipperMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Rate limit rules are applied, log-shipper agent on the `{{ `{{ $labels.node }}` }}` node is dropping logs for more than 15 minutes.

        Consider checking logs of the pod or follow advanced debug instructions.
        `kubectl -n d8-log-shipper get pods -o wide | grep {{ `{{ $labels.node }}` }}`
