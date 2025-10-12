---
title: Security event audit
permalink: en/virtualization-platform/documentation/admin/platform-management/security/events/runtime-audit.html
---

Deckhouse Virtualization Platform (DVP) provides built-in tools for detecting security threats
by analyzing Linux kernel events and auditing Kubernetes API events.
With DVP, you can:

- Detect threats in environments by analyzing applications and containers.
- Identify attempts to exploit vulnerabilities from the CVE database and signs of cryptocurrency miner activity.
- Detect Kubernetes-specific threats, including:
  - Command-line shells launched in containers or pods.
  - Containers running in privileged mode.
  - Mounting insecure paths into containers (for example, `/proc`).
  - Attempts to read sensitive data (for example, from `/etc/shadow`).

## Data sources for security auditing

DVP uses two main sources of events:

- Linux kernel events — via the eBPF driver for the [Falco](https://falco.org/) threat detection system.
- [Kubernetes API audit](./kubernetes-api-audit.html) events — via integration with [Kubernetes auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) and a webhook interface.

For details about the auditing architecture, refer to [Architecture](/products/virtualization-platform/documentation/architecture/security/runtime-audit.html).

## Minimum requirements

To capture kernel events, you need:

- Linux kernel version 5.8 or higher.
- Support for [eBPF](https://www.kernel.org/doc/html/v5.8/bpf/btf.html).
  You can check for support in one of the following ways:
  - Make sure the `/sys/kernel/btf/vmlinux` file exists:

    ```shell
    ls -lah /sys/kernel/btf/vmlinux
    ```

  - Make sure the `CONFIG_DEBUG_INFO_BTF` option is enabled:

    ```shell
    grep -E "CONFIG_DEBUG_INFO_BTF=(y|m)" /boot/config-*
    ```

Falco agents run on each cluster node and consume resources depending on the number of rules applied and events collected.

{% alert level="info" %}
On some systems, eBPF probes may not work.
{% endalert %}

## Enabling security event auditing

1. Make sure the nodes meet the [minimum requirements](#minimum-requirements).
1. Enable auditing in Deckhouse using the following configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: runtime-audit-engine
   spec:
     enabled: true
   ```

1. (**Optional**) If the cluster control plane is not managed by DVP with `control-plane-manager`,
   configure the [Kubernetes API audit webhook](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#webhook-backend) manually.

All available security audit parameters are listed in the [`runtime-audit-engine`](/modules/runtime-audit-engine/configuration.html) module documentation.

### Manually configuring the Kubernetes API audit webhook

{% alert level="info" %}
Webhook configuration is not required if the `control-plane-manager` module is enabled.
In this case, when the `runtime-audit-engine` module is enabled,
the settings for collecting Kubernetes API audit events will be applied automatically.
{% endalert %}

To configure the webhook for receiving audit events from `kube-apiserver`:

1. Create a `kubeconfig` file for the webhook with the address `https://127.0.0.1:9765/k8s-audit`
   and certificate data (`ca.crt`) from the `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls` Secret.

   Example:

   ```yaml
   apiVersion: v1
   kind: Config
   clusters:
   - name: webhook
     cluster:
       certificate-authority-data: BASE64_CA
       server: "https://127.0.0.1:9765/k8s-audit"
   users:
   - name: webhook
   contexts:
   - context:
      cluster: webhook
      user: webhook
     name: webhook
   current-context: webhook
   ```

1. Specify the path to the configuration file with the `--audit-webhook-config-file` flag in the `kube-apiserver` manifest.
1. (**Optional**) To collect Kubernetes API audit events from not only system but also user namespaces, configure [audit policies](./kubernetes-api-audit.html#configuring-a-custom-audit-policy).

## Working with audit rules

Security event analysis is based on rules that define suspicious behavior criteria.
DVP includes:

- **Built-in rules**, including:
  - Kubernetes audit rules (located in the `falco` container at `/etc/falco/k8s_audit_rules.yaml`).
  
  To configure the list of built-in rules,
  use the [`settings.builtInRulesList`](/modules/runtime-audit-engine/configuration.html#parameters-builtinruleslist) parameter
  of the `runtime-audit-engine` module.

- **Custom rules**, defined via the [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resource.

For more information on how security audit rules work, refer to [Architecture](/products/virtualization-platform/documentation/architecture/security/runtime-audit.html).

### Adding a custom rule

To add a rule, create a [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) resource with the required conditions.
Use the [Falco condition syntax](https://falco.org/docs/concepts/rules/conditions/).
Falco agents will automatically apply the created rule.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: ownership-permissions
spec:
  rules:
  - macro:
      name: spawned_process
      condition: (evt.type in (execve, execveat) and evt.dir=<)
  - rule:
      name: Detect Ownership Change
      desc: detect file permission/ownership change
      condition: >
        spawned_process and proc.name in (chmod, chown) and proc.args contains "/tmp/"
      output: >
        The file or directory below has had its permissions or ownership changed (user=%user.name
        command=%proc.cmdline file=%fd.name parent=%proc.pname pcmdline=%proc.pcmdline gparent=%proc.aname[2])
      priority: Warning
      tags: [filesystem]
```

For more rule examples, see:

- [Falco rules official repository](https://github.com/falcosecurity/rules/blob/32b635394c40a56f8bdeb334c60a46e2edd9908c/rules/application_rules.yaml)
- [Falco rules on Artifact Hub](https://artifacthub.io/packages/search?kind=1&sort=relevance&page=1)

### Applying a third-party rule

Since Falco rule structure differs from DVP custom resource format,
third-party rules from the internet must be converted to a [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) resource
before applying them.

Use the following script to convert:

```shell
git clone github.com/deckhouse/deckhouse
cd deckhouse/ee/modules/runtime-audit-engine/hack/far-converter
go run main.go -input /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```

Example conversion result:

- Original rule:

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

- Converted resource:

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

## Log collection and alerts

DVP exports security audit events as Prometheus metrics,
allowing you to set up alerts via the [CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules) resource.
This makes it possible to:

- Connect an external log storage (for example, Loki or Elasticsearch).
- Configure alerts for critical events.

### Configuring log and event collection

All security audit events are sent to stdout.
To collect and forward events to a log storage, create a [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) resource
following the example:

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
      labelSelector:
        matchExpressions:
        - key: "kubernetes.io/metadata.name"
          operator: In
          values: [d8-runtime-audit-engine]
  labelFilter:
  - operator: Regex
    values: ["\\{.*"] # Collect only JSON-formatted logs.
    field: "message"
  type: KubernetesPods
```

### Configuring critical event alerts

To create alerts for critical events,
create a [CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules) object following the example:
{% raw %}

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

{% endraw %}

### Viewing metrics

To retrieve Prometheus metrics, use the PromQL query `falcosecurity_falcosidekick_falco_events_total{}`:

```shell
d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
  curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" | jq
```

## Debugging and simulating events

For debugging and simulating security events in DVP, you can use:

- The `event-generator` utility.
- The `/test` HTTP endpoint of the `falcosidekick` service.

### Enabling debug logging

Falco uses the `debug` logging level by default.

In Falcosidekick, debug logging is disabled by default.
To enable it,
set the [`spec.settings.debugLogging`](/modules/runtime-audit-engine/configuration.html#parameters-debuglogging) parameter to `true`,
for example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: runtime-audit-engine
spec:
  enabled: true
  settings:
    debugLogging: true
```

### Simulating events

#### Falco

The [`event-generator`](https://github.com/falcosecurity/event-generator) utility allows you to generate
various suspicious actions (for example, system calls or Kubernetes API audit events).

Run the following command to generate a test set of events in a Kubernetes cluster:

```shell
d8 k run falco-event-generator --image=falcosecurity/event-generator run
```

If you need to simulate a specific action, refer to the [utility's documentation](https://github.com/falcosecurity/event-generator/blob/main/events/README.md).

#### Falcosidekick

To simulate sending test events to the `falcosidekick` service, use its `/test` HTTP endpoint:

1. Create a test event by running:

   ```shell
   nsenter -t $(pidof falcosidekick) curl -X POST -H "Content-Type: application/json" -H "Accept: application/json" http://localhost:2801/test
   ```

1. Check the event metric:

   ```shell
   d8 k -n d8-monitoring exec -it prometheus-main-0 prometheus -- \
     curl -s "http://127.0.0.1:9090/api/v1/query?query=falcosecurity_falcosidekick_falco_events_total" \
     | jq '.data.result.[] | select (.metric.priority_raw == "debug")'
   ```

   Example output:

   ```console
   {
     "metric": {
       "__name__": "falcosecurity_falcosidekick_falco_events_total",
       "container": "kube-rbac-proxy",
       "hostname": "falcosidekick",
       "instance": "192.168.208.7:4212",
       "job": "runtime-audit-engine",
       "node": "dev-master-0",
       "priority": "1",
       "priority_raw": "debug",
       "rule": "Test rule",
       "source": "internal",
       "tier": "cluster"
     },
     "value": [
       1744234729.799,
       "1"
     ]
   }
   ```
