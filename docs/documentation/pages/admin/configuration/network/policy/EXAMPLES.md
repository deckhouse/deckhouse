---
title: "Common policy examples"
permalink: en/admin/configuration/network/policy/examples.html
description: |
  Ready-to-use network policy recipes for Deckhouse Kubernetes Platform: deny external ingress, namespace and pod selectors, DNS egress, API server access, L7 and FQDN rules.
relatedLinks:
  - title: "Kubernetes NetworkPolicy"
    url: kubernetes_networkpolicy.html
  - title: "CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy"
    url: cilium_networkpolicy.html
  - title: "Host firewall on nodes"
    url: host_firewall.html
  - title: "Diagnostics and observability"
    url: troubleshooting.html
---

This page collects common scenarios for network policies. Standard NetworkPolicy examples work in any cluster that supports network policies; CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy examples require the [`cni-cilium`](/modules/cni-cilium/) module.

The resource shape itself is documented in [Kubernetes NetworkPolicy](kubernetes_networkpolicy.html) and [CiliumNetworkPolicy and CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html).

## Deny all external ingress to a namespace, allow internal traffic

A baseline policy for a namespace where pods talk to each other but are not reachable from outside:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-external-ingress
  namespace: my-app
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector: {}
  egress:
    - {}
```

The empty `egress: [{}]` keeps all egress allowed; ingress is allowed only from pods in the same namespace.

## Allow ingress from a specific namespace

Allows pods in the namespace labeled `kubernetes.io/metadata.name: frontend` to call pods labeled `app: api`:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend-to-api
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: api
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
      ports:
        - protocol: TCP
          port: 8080
```

Kubernetes sets the `kubernetes.io/metadata.name` label on every namespace automatically.

## Allow egress to DNS and one CIDR only

Default-deny egress plus explicit DNS and one external service:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: restrict-egress
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: client
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    - to:
        - ipBlock:
            cidr: 10.0.0.0/24
      ports:
        - protocol: TCP
          port: 5432
```

## Allow specific pods to reach the API server

Through the `kube-apiserver` entity, Cilium tracks the API server IPs automatically — the rule does not need to be updated when addresses change:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-controller-to-apiserver
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: controller
  egress:
    - toEntities:
        - kube-apiserver
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

## Allow only GET requests to an API at L7

Clients labeled `app: client` may only call `GET /api/v1/...` on pods labeled `app: api`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: readonly-api
  namespace: my-app
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

## Allow egress to specific DNS names (FQDN)

For FQDN rules, Cilium must observe DNS queries, so the same policy must also allow egress to the cluster DNS with DNS inspection:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: egress-to-fqdns
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toEntities:
        - cluster
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
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

{% alert level="info" %}
The DNS egress rule uses `toEntities: cluster` rather than a label selector targeting `kube-dns` pods. DKP deploys a `node-local-dns` DaemonSet alongside the main DNS service, so the actual DNS path from a pod may go through a `node-local-dns` instance. Using `toEntities: cluster` matches any cluster-internal DNS endpoint reliably.
{% endalert %}

## Deny access to the cloud metadata service

A cluster-scope deny rule blocks the cloud metadata service for every pod, even when other policies allow this egress:

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

