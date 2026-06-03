---
title: "Diagnostics and observability"
permalink: en/admin/configuration/network/policy/troubleshooting.html
description: |
  Tools for inspecting applied network policies in Deckhouse Kubernetes Platform: kubectl describe, Hubble UI and CLI, flow logs, and a "policy not applied" checklist.
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

In `cilium-dbg endpoint list`, each pod endpoint shows `POLICY (ingress)` and `POLICY (egress)` status: `Enabled`, `Disabled`, or `Disabled (Audit)` when [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) is on.

## Observability via Hubble

Hubble shows policy verdicts in real time and is the primary diagnostic tool in Cilium clusters.

In Hubble UI, connections between pods and services are tagged as `forwarded`, `dropped`, or `audit`. Drop events show which policy blocked the traffic and which rule field matched.

`hubble observe` can filter events by type:

```bash
hubble observe --type policy-verdict --verdict DROPPED
hubble observe --type policy-verdict --verdict AUDITED
hubble observe --from-pod my-app/client --to-pod my-app/api
```

The output includes policy and selector identifiers and the specific ingress/egress fields that matched, which makes it easy to find the rule that blocked or allowed the connection.

## Continuous flow logs collection

For continuous flow log collection, enable export through the [`HubbleMonitoringConfig`](/modules/cni-cilium/cr.html#hubblemonitoringconfig) resource. Configuration is described in the [cni-cilium examples](/modules/cni-cilium/examples.html#hubblemonitoringconfig).

Once export is on, `cilium-agent` writes events to `/var/log/cilium/hubble/flow.log` on every node. For central collection, use the [`log-shipper`](/modules/log-shipper/) module with a `ClusterLoggingConfig` of type `File` that reads this file.

{% alert level="warning" %}
Updating `HubbleMonitoringConfig` restarts every Cilium agent in the cluster.
{% endalert %}

## Common mistakes

- **DNS broken after default-deny egress.** A default-deny egress policy also blocks DNS. Add an egress rule to the kube-dns service in the `kube-system` namespace (UDP/53 and TCP/53). See [Kubernetes NetworkPolicy](kubernetes_networkpolicy.html#default-policies-for-a-namespace).
- **AND vs OR mixed up.** Two selectors in one `from`/`to` array item is AND; two separate items is OR. See [Kubernetes NetworkPolicy](kubernetes_networkpolicy.html#and-vs-or-in-selectors).
- **Policy does not apply to `hostNetwork` pods.** Most engines, including Cilium and kube-router, treat such pods as node traffic. Use a host firewall — see [Host firewall on nodes](host_firewall.html).
- **FQDN rule does not allow traffic.** Cilium must observe DNS to keep the resolved IP set up to date. In any policy with `toFQDNs`, also allow egress to kube-dns and enable DNS inspection via `rules.dns`. See the example in [CiliumNetworkPolicy](cilium_networkpolicy.html#fqdn-rules).
- **Connection drops after a policy change.** Behavior for in-flight connections is not defined by the standard — some engines tear them down. Change policies during a maintenance window.

## "Policy not applied" checklist

If a policy is created but traffic does not behave as expected, walk through these checks:

1. **Which engine is enabled.** The standard `NetworkPolicy` is supported by both engines; CNP, CCNP, L7, and FQDN require `cni-cilium`. See [Network policies](configuration.html#what-is-available-in-each-engine).
2. **The selector matches the pods.** `d8 k get pods -n <namespace> -l <key>=<value>` should return the expected list.
3. **`policyTypes` is correct.** With `Ingress` only, egress stays unrestricted; with `Egress` only, ingress stays unrestricted.
4. **AND vs OR in selectors.** Re-check the array structure — a common cause of overly broad or overly narrow rules.
5. **Audit mode.** When [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) is on, policies do not block traffic. `cilium-dbg endpoint list` shows this as `Disabled (Audit)`.
6. **Eventual consistency.** Cilium and kube-router apply policies asynchronously. Wait a few seconds and re-test.
7. **Policy status (CNP and CCNP only).** `d8 k get ciliumnetworkpolicy <name> -n <namespace> -o yaml` shows parse and apply errors in `status`.
8. **Conflict with a deny rule.** Cilium deny rules override any allow rules. Look for policies with `ingressDeny` or `egressDeny` selecting the same endpoint.

## See also

- [HubbleMonitoringConfig — cni-cilium module](/modules/cni-cilium/cr.html#hubblemonitoringconfig)
- [Troubleshooting Policy — Cilium documentation](https://docs.cilium.io/en/v1.17/security/policy/#troubleshooting)
- [Kubernetes NetworkPolicy](kubernetes_networkpolicy.html)
- [CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html)
- [Host firewall on nodes](host_firewall.html)
