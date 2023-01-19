---
title: "The runtime-audit-engine module: usage"
---

## How to collect events?

Pods of runtime-audit-engine outputs all events to stdout. 
These events then can be collected by the `log-shipper` module and be sent to any supported destination.

An example of `ClusterLoggingConfig` for `log-shipper`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: falco-events
spec:
  destinationRefs:
  - xxxx
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - d8-runtime-audit-events
  labelsFilter:
  - operator: Regex
    values: ["\{.*"] # to collect only JSON logs
    field: "message"
  type: KubernetesPods
```

## How to alert?

All metrics are automatically collected by Prometheus. Add a CustomPrometheusRule to enable alerts.

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
          Check you events journal for more details.
        summary: Falco detects a critical security incident
      expr: |
        sum by (node) (rate(falco_events{priority="Critical"}[5m]) > 0)
```
> NOTE: Alerts are best work with an events storage like Elasticsearch or Loki. With an alert users can be notified that something is happening.
> The next step is to find what using previously collected events.

## How to deploy Falco rules that I found on the Internet?

The structure of native Falco rules is different from the CRD schema.
It is due to limitations of schema validation capabilities in Kubernetes.

To make the process of migrating native Falco rules to Deckhouse more convenient,
there is a script that can help to convert a rule file for Falco to FalcoAuditRules custom resource.

```shell
git clone github.com/deckhouse/deckhouse
cd deckhouse/ee/modules/650-runtime-audit-engine/hack/fav-converter
go run main.go -input /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```

Example of a script result:

```yaml
# /path/to/falco/rule_example.yaml
- rule: Linux Cgroup Container Escape Vulnerability (CVE-2022-4092)
  desc: "This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel."
  condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
  output: "Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-4092) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)"
  priority: CRITICAL
  tags: [process, mitre_privilege_escalation]
```

```yaml
# ./my-rules-cr.yaml
apiversion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: rule-example
spec:
    rules:
    - rule:
        name: Linux Cgroup Container Escape Vulnerability (CVE-2022-4092)
        condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
        desc: This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel.
        output: Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-4092) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)
        priority: Critical
        tags:
        - process
        - mitre_privilege_escalation
```
