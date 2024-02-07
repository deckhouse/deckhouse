---
title: "The runtime-audit-engine module"
description: Overview of the runtime-audit-engine Deckhouse module, which implements a runtime threats detection engine in the Kubernetes cluster.
---

## Overview

The module implements a runtime threats detection engine.

The module collects Linux kernel system calls and Kubernetes API audit events (by `k8saudit` plugin enabled by default), enrich them with metadata from Kubernetes Pods and generate security audit events according to conditional rules.

This module:
* Detects threats at runtime by observing the behavior of your applications and containers.
* Helps to detect CVEs exploits and cryptocurrency mining attacks. 
* Improves Kubernetes security by detecting:
  * Shells running in containers or Pods in Kubernetes.
  * Containers running in privileged mode or attempting to mount sensitive paths, such as `/proc`, on the host.
  * Unauthorized attempts to read confidential files such as `/etc/shadow`.

## Architecture

The module is based on the [Falco](https://falco.org/) system. 
Deckhouse deploys Falco agents (which run as a DaemonSet) on every node. The agents then start consuming kernel / kube audit events.

![Falco DaemonSet](../../images/650-runtime-audit-engine/falco_daemonset.svg)
<!--- Source: https://docs.google.com/drawings/d/1NZ91z8NXNiuS50ybcMoMsZI3SbQASZXJGLANdaNNm_U --->

{% alert %}
Falco developers recommend deploying Falco as a systemd unit for maximum security.
However, a Kubernetes cluster with the autoscaling feature enabled makes it hard to operate. 
Additional security mechanisms of Deckhouse (implemented by other modules), such as multitenancy and admission policy control, provide the required level of security to mitigate attacks on the Falco DaemonSet.
{% endalert %}

There are four different containers in a single agent Pod:
![Falco Pod](../../images/650-runtime-audit-engine/falco_pod.svg)
<!--- Source: https://docs.google.com/drawings/d/1rxSuJFs0tumfZ56WbAJ36crtPoy_NiPBHE6Hq5lejuI --->

1. `falco` — collects events, enriches them with metadata and sends them to stdout.
2. `rules-loader` — collects ([FalcoAuditRules](cr.html#falcoauditrules)) CRs from Kubernetes and saves them in a shared directory (empty dir).
3. `falcosidekick` — it takes a `Falco` events and forward them to different outputs in a fan-out way. By default, it exports events as metrics on which alerts can be generated. [Falcosidekick source code](https://github.com/falcosecurity/falcosidekick).
4. `kube-rbac-proxy` — protects the `falcosidekick` metric's endpoint.

## Audit Rules

The event collection itself is a low-yielding activity because the amount of data coming from the Linux kernel is too large to be analyzed by a human.
Rules address this problem by collecting events according to certain pattens that can help in detecting suspicious activities.

The main part of a rule is a conditional expression (which uses the [conditions syntax](https://falco.org/docs/rules/conditions/)).

### Embedded rules

There is a built-in set of rules that cannot be disabled. It helps to identify problems with Deckhouse security as well as security problems affecting the `runtime-audit-engine` module.

- `/etc/falco/k8s_audit_rules.yaml` — Kubernetes audit rules.


### Custom audit rules

Users can use a `FalcoAuditRules` CRD to add custom security audit rules. 
Each Falco agent Pod has a sidecar container running [shell-operator](https://github.com/flant/shell-operator).
This sidecar reads rules from the custom resources, converts them to Falco rules and saves Falco rules in the Pod's `/etc/falco/rules.d/` directory.
Falco automatically reloads the configuration when a new rule becomes available.

![Falco shell-operator](../../images/650-runtime-audit-engine/falco_shop.svg)
<!--- Source: https://docs.google.com/drawings/d/13MFYtiwH4Y66SfEPZIcS7S2wAY6vnKcoaztxsmX1hug --->

Such a schema allows the IaC approach to be used to maintain Falco rules.

## Requirements

### OS

The module uses the eBPF Falco driver to ingest syscall data. It is better suited for environments where loading a kernel module is prohibited or unsupported, such as GKE, EKS, and other managed Kubernetes solutions.
The eBPF driver has the following requirements:
* The eBPF probe may not work for every system.
* Linux kernel version >= 5.8.
* Enabled [eBPF](https://www.kernel.org/doc/html/v5.8/bpf/btf.html). Check with the command `ls -lah /sys/kernel/btf/vmlinux`, or find `CONFIG_DEBUG_INFO_BTF=y` in the list of kernel build parameters.

> eBPF probes may not work on some systems.

### CPU / Memory

Falco agents are running on every node. Therefore, the resource consumption of each Pod depends on the number of rules or ingested events.

## Kubernetes Audit Webhook

[Webhook audit mode](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#webhook-backend) should be configured to collect audit events of `kube-apiserver`. 
If the [control-plane-manager](../040-control-plane-manager/) module is enabled, settings will be automatically applied when the `runtime-audit-engine` module is enabled.

You can manually configure the webhook for Kubernetes clusters with a control plane that is not controlled by Deckhouse:
1. Create a webhook kubeconfig file with the `https://127.0.0.1:9765/k8s-audit` address and the CA (ca.crt) from the `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls` secret.
    
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
2. Add the `--audit-webhook-config-file` flag to the `kube-apiserver` manifest. The flag must point to the previously created file.

{% alert level="warning" %}
Remember to configure the audit policy, because Deckhouse only collects Kubernetes audit events from the system namespaces by default. 
An example of configuration can be found in the [control-plane-manager](../040-control-plane-manager/) module documentation.
{% endalert %}

## Alerting

If a number of `runtime-audit-engine` pods are not scheduled, then the `D8RuntimeAuditEngineNotScheduledInCluster` alert will be generated.
