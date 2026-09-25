---
title: Runtime audit architecture
permalink: en/architecture/security/runtime-audit.html
search: runtime audit, audit rules, falco
description: Runtime audit architecture in Deckhouse Platform.
---

The runtime audit of Deckhouse Platform (DP) is based on the [Falco](https://falco.org/) threat detection system.
This mechanism analyzes Linux kernel events and Kubernetes API audit events to detect suspicious activity
in running containers and across the cluster.

DP deploys Falco agents on each node as part of a DaemonSet.
Once started, the agents begin collecting OS system calls and Kubernetes audit data.

{% alert level="info" %}
Falco developers recommend running it as a systemd service,
which can be challenging in Kubernetes clusters that support autoscaling.
DP includes additional security mechanisms such as multitenancy and resource control policies.
Combined with the DaemonSet deployment, these mechanisms ensure a high level of protection.
{% endalert %}

![Falco agents on DP cluster nodes](../../images/runtime-audit-engine/falco_daemonset.svg)

Each cluster node runs a Falco Pod with the following components:

- `falco`: Collects events, enriches them with metadata, and outputs them to stdout.
- `rules-loader`: Retrieves rule data from [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resources
  and stores them in a shared directory. It also validates those resources through an admission webhook,
  so a rule that Falco cannot parse is rejected when you apply it rather than after it reaches the agents.
- [`falcosidekick`](https://github.com/falcosecurity/falcosidekick): Receives events from `falco`
  and exports them as metrics to external systems.
- `kube-rbac-proxy`: Protects the `falcosidekick` metrics endpoint from unauthorized access.

The Kubernetes metadata that `falco` adds to an event comes from `k8s-metacollector`,
a single Deployment in the module namespace.
The agents connect to it over gRPC and receive the metadata of the Pods running on their own node,
together with the namespaces and workloads those Pods belong to.
Without that connection the agents keep detecting events, but the events carry no Kubernetes context.

For a detailed architecture of the [`runtime-audit-engine`](/modules/runtime-audit-engine/) module, which implements DP security event audit, refer to the [corresponding documentation section](./runtime-audit-engine.html).

## Audit rules

Runtime audit event analysis is performed using rules that define suspicious behavior patterns.
Each rule consists of a condition expression written in accordance with [Falco's condition syntax](https://falco.org/docs/concepts/rules/conditions/).

### Built-in rules

DP provides the following types of built-in rules:

- **Kubernetes audit rules**: Help detect security issues in DP and in the audit mechanism itself.
  These rules are located in the `falco` container at `/etc/falco/k8s_audit_rules.yaml`.
- **Regulatory rules**: Meet the requirements of Order No. 118 of the Federal Service for Technical
  and Export Control of Russia of 4 July 2022, "Information security requirements for containerization tools".
  These `fstec` rules are described as a [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resource.

### Custom rules

Custom rules can be defined using the [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resource.

Each Falco agent includes a `rules-loader` sidecar container.
It watches the FalcoAuditRules resources, converts them into Falco rule format,
and stores them in the `/etc/falco/rules.d/` directory inside the Pod.
Falco watches that directory and reloads its configuration when the rules change,
so a new rule takes effect without restarting the agents.

The built-in rules travel the same path: DP renders them as FalcoAuditRules resources,
which is why `kubectl get falcoauditrules` lists them next to your own.

![rules-loader handling Falco rules](../../images/runtime-audit-engine/falco_shop.svg)
