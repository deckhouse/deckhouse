---
title: "CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy"
permalink: en/admin/configuration/network/policy/cilium_networkpolicy.html
description: |
  Cilium extensions for network policies in Deckhouse Kubernetes Platform: entities, L7 rules, FQDN rules, deny rules, and policyAuditMode.
---

In clusters with the [`cni-cilium`](/modules/cni-cilium/) module, two Cilium-specific formats are available in addition to the standard `NetworkPolicy`:

- `CiliumNetworkPolicy` (CNP) — a namespaced resource with L3–L7 rules;
- `CiliumClusterwideNetworkPolicy` (CCNP) — a cluster-scoped resource with the same rule language and `nodeSelector` support.

Cilium can enforce all three formats at the same time.

{% alert level="warning" %}
When `NetworkPolicy`, CNP, and CCNP are all in use, the resulting allow set is harder to reason about. Roll out new policies in audit mode and verify the behavior in Hubble.
{% endalert %}

## What CNP and CCNP add

Compared to the standard `NetworkPolicy`:

- L7 rules — HTTP, gRPC, Kafka, and DNS;
- FQDN rules in egress — filtering by DNS names;
- deny rules — explicit denial of traffic;
- entities — built-in groups of sources and destinations such as `kube-apiserver`, `host`, `remote-node`, `world`;
- references to Kubernetes services by name or labels (`toServices`) — egress rules without specifying a CIDR;
- ICMP and ICMPv6 filtering by packet type;
- TLS filtering by Server Name Indication (SNI);
- `nodeSelector` (CCNP only) — applies a rule to nodes themselves and is the basis for the [host firewall on nodes](host_firewall.html);
- audit mode via [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) — log policy verdicts without enforcing them.

## Resource structure

CNP and CCNP share the same `spec` shape. A minimal example:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: example
  namespace: default
spec:
  endpointSelector:
    matchLabels:
      app: db
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: frontend
      toPorts:
        - ports:
            - port: "5432"
              protocol: TCP
```

Key fields:

- `endpointSelector` — selects pods the policy applies to. Counterpart of `podSelector` in the standard `NetworkPolicy`.
- `nodeSelector` — selects nodes (CCNP only). Not used together with `endpointSelector` in the same policy.
- `ingress` and `egress` — rule arrays. Each rule has a peer field (`fromEndpoints`, `fromEntities`, `fromCIDR`, `fromCIDRSet`, `toEndpoints`, `toEntities`, `toCIDR`, `toCIDRSet`, `toFQDNs`, `toServices`) and an optional `toPorts` filter for protocols and ports.
- `ingressDeny` and `egressDeny` — deny rules. They are evaluated before allow rules.

Two special labels are useful in selectors. Cilium attaches them to every pod endpoint automatically:

- `io.kubernetes.pod.namespace` — the namespace where the pod runs. Use it in `fromEndpoints` and `toEndpoints` to reference pods in a specific namespace.
- `k8s-app`, `app`, and other regular pod labels — available without a prefix.

### Egress to a Kubernetes service

The `toServices` field describes egress to a Kubernetes service without knowing its CIDR. Match the service by name and namespace (`k8sService`) or by labels (`k8sServiceSelector`):

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-egress-to-redis
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toServices:
        - k8sService:
            serviceName: redis
            namespace: data
```

Unlike `toEndpoints`, the policy automatically tracks changes to the service's endpoint list.

## Entities

Entities are built-in groups of sources and destinations that make it easier to describe traffic to and from cluster components and infrastructure:

- `host` — the pod's own node, including the host's own traffic;
- `remote-node` — other cluster nodes;
- `kube-apiserver` — the Kubernetes API server (used by host firewall);
- `cluster` — every pod and node in the cluster;
- `world` — anything outside the cluster;
- `health` — Cilium health endpoints;
- `init` — containers that have not yet received an identity;
- `unmanaged` — pods not managed by Cilium;
- `all` — any entity.

An ingress rule that allows the API server to reach pods labeled `app: webhook`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-apiserver-to-webhook
  namespace: default
spec:
  endpointSelector:
    matchLabels:
      app: webhook
  ingress:
    - fromEntities:
        - kube-apiserver
      toPorts:
        - ports:
            - port: "9443"
              protocol: TCP
```

## L4 rules: ICMP and SNI

In addition to ports and protocols, `toPorts` supports more L4 filters.

### ICMP and ICMPv6

The `icmps` field allows or denies ICMP messages by packet type:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-icmp-echo
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: probe
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: monitoring
      icmps:
        - fields:
            - type: EchoRequest
              family: IPv4
```

Without an explicit `icmps` rule, ICMP traffic is blocked together with TCP and UDP once L4 filtering is active.

### TLS Server Name Indication (SNI)

Egress can be restricted by SNI — the name a client sends in the TLS ClientHello. This filters access to external HTTPS services without a MITM:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-egress-tls-sni
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toFQDNs:
        - matchPattern: "*.example.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
          serverNames:
            - "api.example.com"
            - "static.example.com"
```

When `serverNames` is set, only TLS connections with the listed names are allowed; connections with any other SNI are blocked at the TLS handshake.

## L7 rules

CNP and CCNP can describe allowed application-level operations. L7 rules go inside `toPorts[].rules`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-readonly-api
  namespace: default
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: client
      toPorts:
        - ports:
            - port: "8080"
              protocol: TCP
          rules:
            http:
              - method: GET
                path: "/api/v1/.*"
```

Clients labeled `app: client` may only call `GET /api/v1/...` on pods labeled `app: api`, port 8080.

Supported protocols: HTTP, gRPC, Kafka, DNS. Details and limitations are described in [Layer 7 Examples in the Cilium docs](https://docs.cilium.io/en/v1.17/security/policy/#layer-7-examples).

### Kafka

For Kafka, L7 rules allow specific operations (`apiKey`) and topics:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-kafka-produce
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: kafka
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: producer
      toPorts:
        - ports:
            - port: "9092"
              protocol: TCP
          rules:
            kafka:
              - role: produce
                topic: orders
```

Pods labeled `app: producer` may only publish (`role: produce`) to the `orders` topic. Any other operations, including `consume` and `metadata`, are denied at the Kafka protocol level.

### DNS Policy and IP Discovery

When `toFQDNs` is in use, Cilium intercepts DNS responses allowed by `rules.dns` and updates an internal cache that maps DNS names to IP addresses. This cache is what `toFQDNs` rules check when allowing traffic. Therefore:

- DNS egress must be in the same policy as the FQDN rule, or in any other policy selecting the same pods;
- if a pod is not allowed to make DNS requests, its FQDN rules will not work;
- the TTL and lifetime of cache entries are determined by the Cilium agent based on DNS responses.

## FQDN rules

Egress traffic can be restricted by DNS names via `toFQDNs`. To let Cilium keep the resolved IP set up to date, allow DNS in the same policy and enable DNS inspection through `rules.dns`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-egress-to-example
  namespace: default
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toEndpoints:
        - matchLabels:
            io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
          rules:
            dns:
              - matchPattern: "*"
    - toFQDNs:
        - matchName: "api.example.com"
        - matchPattern: "*.cdn.example.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

## Deny rules

Unlike the standard `NetworkPolicy`, CNP and CCNP can explicitly deny traffic without removing broader allow rules:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: deny-egress-to-metadata
spec:
  endpointSelector: {}
  egressDeny:
    - toCIDR:
        - 169.254.169.254/32
```

Deny rules are evaluated before allow rules and override permissions from any other policy.

## Default policies via CNP

To put a namespace into default-deny mode, create a CNP with empty rule lists:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: default-deny
  namespace: secure
spec:
  endpointSelector: {}
  ingress: []
  egress: []
```

If the pods use DNS, also allow egress to kube-dns over UDP/53 and TCP/53.

## Audit mode (`policyAuditMode`)

The module parameter [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) puts Cilium into a mode where policies do not block traffic; they only log verdicts. This makes it safe to roll out large policy sets and verify them in Hubble UI before enforcement.

{% alert level="warning" %}
In audit mode, **no** network policy blocks traffic. Do not keep audit mode on permanently — turn it off after the rollout is done.
{% endalert %}

Recommended order:

1. Set `policyAuditMode: true` in the [`cni-cilium` module configuration](/modules/cni-cilium/configuration.html#parameters-policyauditmode).
1. Apply the policy set. Roll out host policies separately, following the procedure in [Host firewall on nodes](host_firewall.html).
1. Inspect verdicts in Hubble UI and via `hubble observe --type policy-verdict`. Look for `verdict=AUDITED` entries — these connections would be blocked outside audit mode.
1. Adjust policies until the log only contains expected `verdict=ALLOWED` and `verdict=AUDITED` entries.
1. Turn audit mode off (`policyAuditMode: false`).

Once audit mode is off, policies start blocking traffic that is not allowed by any rule.

## See also

- [Network Policy — Cilium documentation](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/)
- [Overview of Network Policy — Cilium documentation](https://docs.cilium.io/en/v1.17/security/policy/)
- [Host firewall on nodes](host_firewall.html)
- [Common policy examples](examples.html)
- [Diagnostics and observability](troubleshooting.html)
