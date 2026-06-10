---
title: "Host firewall on nodes"
permalink: en/admin/configuration/network/policy/host_firewall.html
description: |
  Host firewall in Deckhouse Kubernetes Platform via CiliumClusterwideNetworkPolicy with nodeSelector. Safe rollout, mandatory rules, and control plane protection.
---

A host firewall is a Cilium mode where network policies apply to cluster nodes themselves rather than to pods. In DKP, you configure it through [`CiliumClusterwideNetworkPolicy`](cilium_networkpolicy.html) with the `nodeSelector` field. Available only in clusters with the [`cni-cilium`](/modules/cni-cilium/) module.

{% alert level="danger" %}
A bug in host policies can break SSH access, control plane operation, kubelet, or etcd. Always roll out host firewall through [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) and verify verdicts in Hubble before enforcing.
{% endalert %}

## How a host firewall differs from regular policies

A `CiliumClusterwideNetworkPolicy` with `nodeSelector` applies to a special host endpoint with the label `reserved:host` and filters traffic entering and leaving the node, including `hostNetwork` pods. Pod policies that use `endpointSelector` do not apply to the host endpoint — they target a different entity.

Host policies do not replace infrastructure-level protection (physical firewalls, cloud security groups). They are an additional filter layer inside the cluster.

## Safe rollout

Roll out a host firewall in stages, using audit mode:

1. Set [`policyAuditMode: true`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) in the `cni-cilium` module configuration. In audit mode, policies do not block traffic; they only log verdicts.
1. Apply the host policy set. At a minimum: a control plane policy (an example is provided below) and worker-node policies that allow SSH and required service ports.
1. Inspect verdicts in Hubble UI and via `hubble observe --type policy-verdict`. Expected traffic must be `verdict=ALLOWED`; anything in `verdict=AUDITED` would be blocked once audit mode is off.
1. Tune the policies until no unexpected `AUDITED` entries remain. Pay close attention to kubelet, etcd, kube-apiserver, ingress controllers, monitoring, and DNS.
1. Turn audit mode off (`policyAuditMode: false`).

If something breaks after enforcement, the fastest recovery path is to re-enable audit mode or delete the `CiliumClusterwideNetworkPolicy`. Detailed recovery steps are in [Cilium Emergency Recovery](https://docs.cilium.io/en/v1.17/security/host-firewall/#emergency-recovery).

## Mandatory rules

The host firewall policy set you maintain must explicitly configure the following permissions; otherwise, after audit mode is turned off, parts of the cluster will stop working:

- access from kube-apiserver to kubelet and webhook endpoints (ports 10250, 10255, and component webhook ports);
- inter-node access on etcd ports (2379, 2380), only between control plane nodes;
- access from worker nodes to the API server;
- BGP and service ports if MetalLB or a third-party load balancer is in use;
- platform component ports in the 4200–4299 range, listed in the [platform component network interaction list](../../../../reference/network_interaction.html);
- SSH from trusted administrative addresses;
- ICMP echo (optional, useful for diagnostics);
- DNS egress to kube-dns or an external resolver;
- access from monitoring to node-exporter and cilium-agent.

Tested policies for the most common cases are shown below: [control plane connectivity](#example-api-server-access-for-the-control-plane), [administrative SSH](#example-ssh-from-administrative-networks), and [worker nodes](#example-baseline-for-worker-nodes). Use them as a starting set.

Use entities to describe peers:

- `host` — the node itself;
- `remote-node` — other cluster nodes;
- `kube-apiserver` — the Kubernetes API server;
- `cluster` — every pod and node in the cluster;
- `world` — anything external (combine with `toCIDR`/`fromCIDR` for narrowing).

## Example: API server access for the control plane

A `CiliumClusterwideNetworkPolicy` that binds control plane nodes to the `kube-apiserver` entity. Without this binding, control plane operations may stutter for up to a minute when `cilium-agent` pods restart, due to a [Cilium Conntrack table reset](https://github.com/cilium/cilium/issues/19367):

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-control-plane-connectivity
spec:
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/control-plane: ""
  ingress:
    - fromEntities:
        - kube-apiserver
```

## Example: SSH from administrative networks

Allows inbound SSH (TCP/22) on every node from the listed subnets only:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-ssh-admin
spec:
  nodeSelector: {}
  ingress:
    - fromCIDR:
        - 192.0.2.0/24
        - 198.51.100.10/32
      toPorts:
        - ports:
            - port: "22"
              protocol: TCP
```

Replace the subnets with the ones used for administrative access. Do not leave `fromEntities: [world]` without a narrowing CIDR — that is the same as an open SSH.

## Example: baseline for worker nodes

Allows worker nodes to exchange traffic inside the cluster, reach the API server, and resolve DNS:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: allow-worker-baseline
spec:
  nodeSelector:
    matchExpressions:
      - key: node-role.kubernetes.io/control-plane
        operator: DoesNotExist
  ingress:
    - fromEntities:
        - cluster
        - remote-node
  egress:
    - toEntities:
        - kube-apiserver
        - cluster
        - remote-node
    - toEndpoints:
        - matchLabels:
            io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
```

This is a starting point. Extend the policy with rules for monitoring, ingress controllers, load balancers, and SSH that match your setup.

## Additional documentation

- [Host Firewall — Cilium documentation](https://docs.cilium.io/en/v1.17/security/host-firewall/)
- [CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html)
- [Platform component network interaction list](../../../../reference/network_interaction.html)
- [Diagnostics and observability](troubleshooting.html)
