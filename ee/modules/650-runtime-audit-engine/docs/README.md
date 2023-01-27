---
title: "The runtime-audit-engine module"
---

## Overview

The module implements runtime threats detection engine. 
It can collect Linux core system calls and Kubernetes API audit events, enrich them with metadata from Kubernetes Pods and generate security audit events by condition rules.

This module:
* Detects threats at runtime by observing the behavior of your applications and containers.
* Helps to detect CVEs exploits, cryptocurrency mining attacks. 
* Improves Kubernetes security by detecting:
  * A shell running inside a container or Pod in Kubernetes.
  * A container running in privileged mode, or mounting a sensitive path, such as `/proc`, from the host.
  * Unexpected read of a sensitive file, such as `/etc/shadow`.

## Architecture

The core of the module is [Falco](https://falco.org/). 
Deckhouse deploys Falco agents as a DaemonSet on every node, and they start consuming kernel / kube audit events.

![Falco DaemonSet](../../images/650-runtime-audit-engine/falco_daemonset.png)

> NOTE: To achieve the maximum level of security, Falco is recommended to be deployed as a systemd unit.
> However, a Kubernetes cluster with the autoscaling feature enabled makes it harder to operate. 
> Additional security measurements such as multitenancy and admission policy control mechanisms provide the required level of security to mitigate attacks on the Falco DaemonSet.

A single Pod consists of five containers:
![Falco Pod](../../images/650-runtime-audit-engine/falco_pod.png)

1. `falco-driver-loader` — init container that compiles eBPF program and stores it into empty dir to provide access to it for Falco.
2. `falco` — collects events, enriches them with metadata and stores them.
3. `rules-loader` — collects custom resources (`FalcoAuditRules`) from Kubernetes and store them in a shared directory (empty dir).
4. `falcosidekick` — only exports events as metrics to be able to alert on them.
5. `kube-rbac-proxy` — protects `falcosidekick` metrics endpoint.

## Audit Rules

An ability to collect events on its own means nothing, because the amount of data that can be exported from a Linux kernel is too big to analyze by a human.
Rules aimed to solve this problem and collect only events according to certain pattens implied to detect any suspicious activity.

The main part of a rule is a condition expression (for which the [conditions syntax](https://falco.org/docs/rules/conditions/) is used).

### Embedded rules

There is a pair of rule files included that cannot be switched off. 
These rules are aimed to highlight Deckhouse security problems and problems connected to the `runtime-audit-engine` module itself.

- `/etc/falco/falco_rules.yaml` — syscall rules
- `/etc/falco/k8s_audit_rules.yaml` — Kubernetes audit rules


### Custom audit rules

To add custom security audit rules, users can use a custom resource definition `FalcoAuditRules`. 
Each Falco agent Pod has a sidecar container with a [shell-operator](https://github.com/flant/shell-operator) instance.
This sidecar reads rules from custom resources and stores it on Pod's filesystem in `/etc/falco/rules.d/` folder.
Falco will automatically reload the configuration once a new rule appears.

![Falco shell-operator](../../images/650-runtime-audit-engine/falco_shop.png)

This schema allows to use the IaC approach to maintain Falco rules.

## Requirements

### OS

The module uses the eBPF Falco driver to ingest syscall data. It works better for environments where loading a kernel module is untrusted or not supported, e.g., GKE, EKS, and other Managed Kubernetes solutions.
Yet there are known limitations for the eBPF driver:
* The eBPF probe may not work for every system.
* At least Linux kernel version 4.14 is required, but the Falco project suggests an LTS kernel of 4.14/4.19 or above.

### CPU / Memory

Falco agents are deployed on every node. Resources consumption of each Pod depends on the number of rules or ingested events.

## Kubernetes Audit Webhook

[Webhook audit mode](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#webhook-backend) should be configured to collect audit events of `kube-apiserver`. 
If the [control-plane-manager](../040-control-plane-manager/) module is enabled, settings will be automatically applied when the `runtime-audit-engine` module is enabled.

For Kubernetes cluster with control plane out of Deckhouse control, it is possible to manually configure the webhook.
1. Create a webhook kubeconfig file with the address `https://127.0.0.1:9765/k8s-audit` and CA (ca.crt) from the `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls` secret.
    
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
2. Add the `--audit-webhook-config-file` flag to the `kube-apiserver` manifest that points to the previously created file.

> NOTE: Do not forget to configure audit policy, because, by default, Deckhouse only collect audit events from system namespaces.
> An example of configuration can be found in the [control-plane-manager](../040-control-plane-manager/) module documentation.
