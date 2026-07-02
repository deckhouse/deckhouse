---
title: "Kubernetes NetworkPolicy"
permalink: en/admin/configuration/network/policy/kubernetes_networkpolicy.html
description: |
  Kubernetes NetworkPolicy model, selectors, default policies, and API limitations in Deckhouse Kubernetes Platform.
relatedLinks:
  - title: "Network Policies — Kubernetes documentation"
    url: https://kubernetes.io/docs/concepts/services-networking/network-policies/
  - title: "Kube-router: Enforcing Kubernetes network policies with iptables and ipset"
    url: https://cloudnativelabs.github.io/post/2017-05-1-kube-network-policies/
  - title: "Common policy examples"
    url: examples.html
  - title: "Diagnostics and observability"
    url: troubleshooting.html
---

The standard [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) resource (`networking.k8s.io/v1`) defines L3/L4 traffic rules for pods (TCP, UDP, optionally SCTP). In DKP, these policies are enforced by the [`cni-cilium`](/modules/cni-cilium/) module or the [`network-policy-engine`](/modules/network-policy-engine/) module, depending on the CNI in use; the mapping between CNI and engine is described in [Network policy implementation in DKP](configuration.html#network-policy-implementation-in-dkp).

## Isolation model

NetworkPolicy describes what is allowed — there are no deny rules. By default, a pod is not isolated: all ingress and egress traffic is allowed. A pod becomes isolated as soon as any policy selects it via `spec.podSelector` and lists the matching direction in `spec.policyTypes`:

- A pod is isolated for ingress when a policy with `policyTypes: [Ingress]` selects it. Only the traffic listed in `ingress` is then allowed.
- A pod is isolated for egress when a policy with `policyTypes: [Egress]` selects it. Only the traffic listed in `egress` is then allowed.
- Reply traffic for allowed connections is always implicit.

Policies are additive: when several policies select the same pod, the resulting allow set is the union of all rules. Order of evaluation does not matter.

For a connection between a source pod and a destination pod, both the source's `egress` and the destination's `ingress` must allow it. If either side denies, the connection does not happen.

## Selectors and fields

A policy uses the following selectors and fields:

- `spec.podSelector` — required. Defines which pods the policy applies to. The policy affects pods in the same namespace where it is created. An empty selector `{}` matches every pod in the namespace.
- `spec.policyTypes` — a list with `Ingress`, `Egress`, or both. If omitted, `Ingress` is always set; `Egress` is set only when the policy has egress rules.
- `ingress[].from` and `egress[].to` — sources and destinations (see below).
- `ingress[].ports` and `egress[].ports` — protocols and ports; a single port or a range via `endPort`.

Inside `from` and `to`, four selector types are available:

- `podSelector` — pods in the same namespace as the policy;
- `namespaceSelector` — all pods in the matching namespaces;
- `podSelector` and `namespaceSelector` in the same item — pods with the given labels in namespaces with the given labels;
- `ipBlock` — a CIDR range with an optional `except` list. Use it for cluster-external addresses, since pod IPs are ephemeral.

A complete example that uses every main construct:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: db-access
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: db
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - ipBlock:
            cidr: 172.17.0.0/16
            except:
              - 172.17.1.0/24
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
        - podSelector:
            matchLabels:
              role: backend
      ports:
        - protocol: TCP
          port: 6379
  egress:
    - to:
        - ipBlock:
            cidr: 10.0.0.0/24
      ports:
        - protocol: TCP
          port: 5978
```

This policy applies to pods labeled `app: db` in namespace `my-app`. Ingress on TCP/6379 is allowed from three sources: the CIDR `172.17.0.0/16` excluding `172.17.1.0/24`, any pod in namespace `frontend`, and pods labeled `role: backend` in the local namespace. Egress is allowed only on TCP/5978 to subnet `10.0.0.0/24`.

### AND vs OR in selectors

A single array item with two selectors means AND (intersection):

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            user: alice
        podSelector:
          matchLabels:
            role: client
```

This allows traffic from pods labeled `role=client` in namespaces labeled `user=alice`.

Two separate array items mean OR (union):

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            user: alice
      - podSelector:
          matchLabels:
            role: client
```

This allows traffic from pods labeled `role=client` in the local namespace, or from any pod in namespaces labeled `user=alice`.

### Selecting a namespace by name

The spec has no field that points to a namespace by name. Use the `kubernetes.io/metadata.name` label instead. Kubernetes automatically applies this label to every namespace, and its value equals the namespace name.

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: frontend
```

### Port ranges

The `endPort` field defines a port range:

```yaml
egress:
  - to:
      - ipBlock:
          cidr: 10.0.0.0/24
    ports:
      - protocol: TCP
        port: 32000
        endPort: 32768
```

Constraints: `endPort` must be greater than or equal to `port`; `endPort` requires `port`; both values must be numeric.

## Default policies for a namespace

Apply baseline policies to set the default behavior of a namespace:

- deny all ingress to the namespace's pods:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: default-deny-ingress
  spec:
    podSelector: {}
    policyTypes:
      - Ingress
  ```

- deny all egress from the namespace's pods:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: default-deny-egress
  spec:
    podSelector: {}
    policyTypes:
      - Egress
  ```

- deny both directions: list both values in `policyTypes` without rules.
- allow all ingress or egress: add an empty rule `ingress: [{}]` or `egress: [{}]`.

{% alert level="warning" %}
A default deny-egress policy blocks DNS too. If the pods use DNS, add a separate policy that allows egress to the kube-dns service in `kube-system` (UDP/53 and TCP/53).
{% endalert %}

## Edge cases

### `hostNetwork` pods

NetworkPolicy behavior for pods with `hostNetwork: true` is not defined by the API. Most engines, including Cilium and kube-router, treat such traffic as node traffic; `podSelector` and `namespaceSelector` do not match these pods. To filter such traffic, use `ipBlock` with the node IP or the [host firewall on nodes](host_firewall.html).

### Pod lifecycle

After a NetworkPolicy is created, the engine applies it asynchronously. A newly started pod selected by the policy may run for a short time without isolation rules or with partial rules. For critical dependencies, use init containers that wait for the required endpoints.

### Existing connections

Behavior on policy changes during an open connection is implementation-defined: some engines tear down the connection, others let it finish. Avoid changing policies, pod labels, or namespace labels while important traffic is in flight.

### L4 only

NetworkPolicy is defined for L4 (TCP, UDP, optionally SCTP). Behavior for other protocols (ICMP, ARP) depends on the engine and may differ.

## Enforcement without Cilium: the `network-policy-engine` module

In clusters without Cilium, the [`network-policy-engine`](/modules/network-policy-engine/) module enforces policies on top of [kube-router](https://github.com/cloudnativelabs/kube-router). It deploys a DaemonSet in the `d8-system` namespace; kube-router runs as a Network Policy Controller and [translates policies into `iptables` and `ipset` rules](https://cloudnativelabs.github.io/post/2017-05-1-kube-network-policies/) on every node:

- each policy becomes its own `KUBE-NWPLCY-*` chain;
- each isolated pod gets a `KUBE-POD-SPECIFIC-FW-*` chain;
- source and destination pod IPs are stored in ipsets, which keeps large rule sets compact and updates fast.

Only the standard Kubernetes formats are supported: `networking.k8s.io/NetworkPolicy API`, V1/GA, and beta semantics. Cilium extensions (CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, L7 rules, FQDN, deny rules) are not supported by this engine.

Ready-to-use standard policy examples that work with both `network-policy-engine` and `cni-cilium` are available on the [Common policy examples](examples.html) page.

## API limitations

The Kubernetes NetworkPolicy API does not support the following scenarios (these are API-level limitations, not engine-level):

- L7 rules (HTTP, gRPC, Kafka, DNS-name filtering);
- deny rules — the model is "default deny + explicit allow";
- selecting services by name;
- forcing all traffic through a shared gateway;
- targeting specific nodes by their Kubernetes identity (only via `ipBlock` with node IPs);
- logging — which connections were allowed or denied;
- blocking loopback or traffic from the pod's own node.

Some of these tasks are addressed by [CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html), available in clusters with `cni-cilium`.

