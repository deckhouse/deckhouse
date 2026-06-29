---
title: Runtime security audit architecture
permalink: en/architecture/security/runtime-audit.html
search: security audit, runtime audit, audit rules, falco
description: Runtime security audit architecture in Deckhouse Kubernetes Platform.
---

The runtime security audit of Deckhouse Kubernetes Platform (DKP) is based on the [Falco](https://falco.org/) threat detection system.
This mechanism analyzes Linux kernel events and Kubernetes API audit events to detect suspicious activity
in running containers and across the cluster.

DKP deploys Falco agents on each node as part of a DaemonSet.
Once started, the agents begin collecting OS system calls and Kubernetes audit data.

{% alert level="info" %}
Falco developers recommend running it as a systemd service,
which can be challenging in Kubernetes clusters that support autoscaling.
DKP includes additional security mechanisms such as multitenancy and resource control policies.
Combined with the DaemonSet deployment, these mechanisms ensure a high level of protection.
{% endalert %}

![Falco agents on DKP cluster nodes](../../images/runtime-audit-engine/falco_daemonset.svg)

Each cluster node runs a Falco Pod with the following components:

- `falco`: Collects events, enriches them with metadata, and outputs them to stdout.
- `rules-loader`: Retrieves rule data from [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resources
  and stores them in a shared directory.
- [`falcosidekick`](https://github.com/falcosecurity/falcosidekick): Receives events from `falco`
  and exports them as metrics to external systems.
- `kube-rbac-proxy`: Protects the `falcosidekick` metrics endpoint from unauthorized access.

![Falco Pod components](../../images/runtime-audit-engine/falco_pod.svg)
<!--- Source: https://docs.google.com/drawings/d/1rxSuJFs0tumfZ56WbAJ36crtPoy_NiPBHE6Hq5lejuI --->

## Audit rules

Security event analysis is performed using rules that define suspicious behavior patterns.
Each rule consists of a condition expression written in accordance with [Falco's condition syntax](https://falco.org/docs/concepts/rules/conditions/).

### Built-in rules

DKP provides the following types of built-in rules:

- **Kubernetes audit rules**: Help detect security issues in DKP and in the audit mechanism itself.
  These rules are located in the `falco` container at `/etc/falco/k8s_audit_rules.yaml`.

### Custom rules

Custom rules can be defined using the [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resource.

Each Falco agent includes a sidecar container with a [`shell-operator`](https://github.com/flant/shell-operator) instance.
This instance reads rules from Kubernetes resources, converts them into Falco rule format,
and stores them in the `/etc/falco/rules.d/` directory inside the Pod.
When a new rule is added, Falco automatically reloads the configuration.

![Shell-operator handling Falco rules](../../images/runtime-audit-engine/falco_shop.svg)

## Relation to the security-events-manager module

The runtime security audit (`runtime-audit-engine`) is one of the security event sources
for the [`security-events-manager`](security-events.html) module.
Events detected by Falco agents can be collected, normalized, and delivered
to storage and analytics systems via the unified security events processing pipeline.
