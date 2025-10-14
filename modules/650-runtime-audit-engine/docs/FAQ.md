---
title: "FAQ"
---


## How to collect events

Pods of `runtime-audit-engine` output all events to stdout.
Those events can then be collected by [log-shipper-agents](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/log-shipper/) and sent to any supported destination.

Below is an example [ClusterLoggingConfig](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/log-shipper/cr.html#clusterloggingconfig) configuration for the `log-shipper` module:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingConfig
metadata:
  name: falco-events
spec:
  destinationRefs:
  - xxxx
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchExpressions:
        - key: "kubernetes.io/metadata.name"
          operator: In
          values: [d8-runtime-audit-engine]
  labelFilter:
  - operator: Regex
    values: ["\\{.*"] # to collect only JSON logs
    field: "message"
  type: KubernetesPods
```

## How to create an alert

All metrics are automatically collected by Prometheus. Add a [CustomPrometheusRule](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/prometheus/cr.html#customprometheusrules) to enable alerts.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: falco-critical-alerts
spec:
  groups:
  - name: falco-critical-alerts
    rules:
    - alert: FalcoCriticalAlertsAreFiring
      for: 1m
      annotations:
        description: |
          There is a suspicious activity on a node {{ $labels.node }}. 
          Check you events journal for details.
        summary: Falco detects a critical security incident
      expr: |
        sum by (node) (rate(falcosecurity_falcosidekick_falco_events_total{priority="Critical"}[5m]) > 0)
```

{{< alert >}}
Alerts work best in combination with event storage, such as Elasticsearch or Loki. Alerts warn the user about suspicious activity on a node.
Once an alert is received, we recommend that you check event storage and examine the events that triggered it.
{{< /alert >}}



## How to apply the Falco rules found on the Internet

The structure of native Falco rules is different from the CRD schema.
It is due to limitations of schema validation capabilities in Kubernetes.

The script for converting Falco rules to resources [FalcoAuditRules](cr.html#falcoauditrules) is built into the `d8` utility functionality.
Using it, you can apply Falco rules in Deckhouse::

```shell
d8 tools far-converter /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```
Example of a script output:

```yaml
# /path/to/falco/rule_example.yaml
- macro: spawned_process
  condition: (evt.type in (execve, execveat) and evt.dir=<)

- rule: Linux Cgroup Container Escape Vulnerability (CVE-2022-0492)
  desc: "This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel."
  condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
  output: "Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-0492) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)"
  priority: CRITICAL
  tags: [process, mitre_privilege_escalation]
```

```yaml
# ./my-rules-cr.yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: rule-example
spec:
  rules:
    - macro:
        name: spawned_process
        condition: (evt.type in (execve, execveat) and evt.dir=<)
    - rule:
        name: Linux Cgroup Container Escape Vulnerability (CVE-2022-0492)
        condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
        desc: This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel.
        output: Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-0492) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)
        priority: Critical
        tags:
          - process
          - mitre_privilege_escalation
```