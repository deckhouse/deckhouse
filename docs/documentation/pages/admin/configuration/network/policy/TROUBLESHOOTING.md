---
title: "Diagnostics and observability"
permalink: en/admin/configuration/network/policy/troubleshooting.html
description: |
  Tools for inspecting applied network policies in Deckhouse Kubernetes Platform: kubectl describe, Hubble UI and CLI, flow logs, and a "policy not applied" checklist.
relatedLinks:
  - title: "HubbleMonitoringConfig — cni-cilium module"
    url: /modules/cni-cilium/cr.html#hubblemonitoringconfig
  - title: "Troubleshooting Policy — Cilium documentation"
    url: https://docs.cilium.io/en/v1.17/security/policy/#troubleshooting
  - title: "Kubernetes NetworkPolicy"
    url: kubernetes_networkpolicy.html
  - title: "CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy"
    url: cilium_networkpolicy.html
  - title: "Host firewall on nodes"
    url: host_firewall.html
---

This page covers commands and techniques for inspecting applied network policies and investigating connectivity issues. Some tools require the [`cni-cilium`](/modules/cni-cilium/) module — this is noted explicitly.

## Inspecting an applied policy

Resource summary:

```bash
d8 k describe networkpolicy <name> -n <namespace>
d8 k describe ciliumnetworkpolicy <name> -n <namespace>
d8 k describe ciliumclusterwidenetworkpolicy <name>
```

The output shows the selected pods or nodes, the resulting ingress and egress rules, and any validation errors.

List policies that affect a namespace:

```bash
d8 k get networkpolicy,ciliumnetworkpolicy -n <namespace>
d8 k get ciliumclusterwidenetworkpolicy
```

See which pods are isolated and which policies apply to them (Cilium clusters only):

```bash
d8 k -n d8-cni-cilium exec ds/agent -- cilium-dbg endpoint list
d8 k -n d8-cni-cilium exec ds/agent -- cilium-dbg endpoint get <endpoint-id>
```

In `cilium-dbg endpoint list`, each pod endpoint shows `POLICY (ingress)` and `POLICY (egress)` status: `Enabled`, `Disabled`, or `Disabled (Audit)` when [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) is on. Example output:

```text
ENDPOINT   POLICY (ingress)   POLICY (egress)   IDENTITY   LABELS (source:key[=value])
1847       Enabled            Enabled            15234      k8s:app=client,k8s:io.kubernetes.pod.namespace=netpol-test
2156       Enabled            Disabled           28765      k8s:app=server,k8s:io.kubernetes.pod.namespace=netpol-test
3          Disabled           Disabled           1          reserved:host
```

`POLICY (ingress): Enabled` means at least one ingress policy applies to the pod and default-deny is active for inbound traffic. `POLICY (egress): Disabled` means there are no egress policies and outbound traffic is unrestricted.

## Observability via Hubble

Hubble shows policy verdicts in real time and is the primary diagnostic tool in Cilium clusters.

In Hubble UI, connections between pods and services are tagged as `forwarded`, `dropped`, or `audit`. Drop events show which policy blocked the traffic and which rule field matched.

`hubble observe` can filter events by type. In DKP, the `hubble` client ships with the agent, so it is convenient to run commands via `d8 k exec` against a cilium-agent pod:

```bash
d8 k -n d8-cni-cilium exec -it ds/agent -- hubble observe --type policy-verdict --verdict DROPPED
d8 k -n d8-cni-cilium exec -it ds/agent -- hubble observe --type policy-verdict --verdict AUDITED
d8 k -n d8-cni-cilium exec -it ds/agent -- hubble observe --from-pod my-app/client --to-pod my-app/api
```

{% alert level="info" %}
DKP does not provide a standalone `d8 hubble` binary. Hubble CLI access is provided by running `exec` into a `cilium-agent` pod, as shown above. The `-it` flag is required for streaming output when `--last` is not specified.
{% endalert %}

Each agent sees events only for its own node. For cluster-wide event collection, use Hubble UI or export via [HubbleMonitoringConfig](/modules/cni-cilium/cr.html#hubblemonitoringconfig).

Example output of `hubble observe --type policy-verdict --verdict DROPPED`:

```text
Jun 10 12:00:01.234   netpol-test/outsider:52341   ->   netpol-test/server:8080   Policy verdict   INGRESS DENIED
Jun 10 12:00:01.236   netpol-test/outsider:52342   ->   netpol-test/server:8080   Policy verdict   INGRESS DENIED
```

Example output of `hubble observe --type policy-verdict --verdict AUDITED` (with `policyAuditMode` on):

```text
Jun 10 12:05:01.101   netpol-test/client:53124   ->   netpol-test/server:8080   Policy verdict   INGRESS AUDITED
```

The output includes policy and selector identifiers and the specific ingress/egress fields that matched, which makes it easy to find the rule that blocked or allowed the connection.

## Continuous flow logs collection

For continuous flow log collection, enable export through the [HubbleMonitoringConfig](/modules/cni-cilium/cr.html#hubblemonitoringconfig) resource. Configuration is described in the [cni-cilium examples](/modules/cni-cilium/examples.html#hubblemonitoringconfig).

Once export is on, `cilium-agent` writes events to `/var/log/cilium/hubble/flow.log` on every node. For central collection, use the [`log-shipper`](/modules/log-shipper/) module with a ClusterLoggingConfig of type `File` that reads this file.

{% alert level="warning" %}
Updating HubbleMonitoringConfig restarts every Cilium agent in the cluster.
{% endalert %}

## Diagnosing FQDN rules

If a `toFQDNs` rule does not allow traffic, inspect the DNS-name to IP cache maintained by `cilium-agent`:

```bash
d8 k -n d8-cni-cilium exec ds/agent -- cilium-dbg fqdn cache list
```

Example output when DNS requests have been intercepted:

```text
Endpoint   Source Namespace   Source Name   FQDN           TTL    ExpirationTime               IPs
1847       netpol-test        client        example.com.   299    2026-06-10T12:05:00.000Z     93.184.216.34
```

The output shows entries with the source, DNS name, resolved IPs, and TTL. If there is no entry for the expected name, the pod either did not make a DNS request, or the DNS request is not allowed by a policy with DNS inspection (`rules.dns`). The cache mechanics are described in [DNS Policy and IP Discovery](cilium_networkpolicy.html#dns-policy-and-ip-discovery).

Also check policy verdicts for DNS traffic:

```bash
d8 k -n d8-cni-cilium exec ds/agent -- hubble observe --type policy-verdict --port 53
```

## Common mistakes

### DNS broken after default-deny egress

A default-deny egress policy also blocks DNS. Add an egress rule to the kube-dns service in the `kube-system` namespace (UDP/53 and TCP/53). For details, refer to [Default policies for a namespace](kubernetes_networkpolicy.html#default-policies-for-a-namespace).

### AND vs OR mixed up

Two selectors in one `from`/`to` array item is AND; two separate items is OR. Refer to [AND vs OR in selectors](kubernetes_networkpolicy.html#and-vs-or-in-selectors) for the correct structure.

### Policy does not apply to `hostNetwork` pods

Most engines, including Cilium and kube-router, treat such pods as node traffic. Use the [host firewall on nodes](host_firewall.html) to filter such traffic.

### FQDN rule does not allow traffic

Cilium must observe DNS to keep the resolved IP set up to date. In any policy with `toFQDNs`, also allow egress to kube-dns and enable DNS inspection via `rules.dns`. For an example, refer to [FQDN rules](cilium_networkpolicy.html#fqdn-rules).

### Connection drops after a policy change

Behavior for in-flight connections is not defined by the standard — some engines tear them down. Change policies during a maintenance window.

## "Policy not applied" checklist

If a policy is created but traffic does not behave as expected, walk through these checks:

1. Which engine is enabled. The standard NetworkPolicy is supported by both engines; CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, L7, and FQDN require `cni-cilium`. The engine capabilities are listed in [What is available in each engine](configuration.html#what-is-available-in-each-engine).
1. The selector matches the pods: `d8 k get pods -n <namespace> -l <key>=<value>` should return the expected list.
1. `policyTypes` is correct. With `Ingress` only, egress stays unrestricted; with `Egress` only, ingress stays unrestricted.
1. AND vs OR in selectors. Re-check the array structure — a common cause of overly broad or overly narrow rules.
1. Audit mode. When [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) is on, policies do not block traffic. `cilium-dbg endpoint list` shows this as `Disabled (Audit)`.
1. Eventual consistency. Cilium and kube-router apply policies asynchronously. Wait a few seconds and re-test.
1. Policy status (CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy only). `d8 k get ciliumnetworkpolicy <name> -n <namespace> -o yaml` shows parse and apply errors in `status`.
1. Conflict with a deny rule. Cilium deny rules override any allow rules. Look for policies with `ingressDeny` or `egressDeny` selecting the same endpoint.

