---
title: "Network policies"
permalink: en/virtualization-platform/documentation/user/network/network-policies.html
---

## Key points

To manage incoming and outgoing traffic for virtual machines at OSI layer 3 or 4, standard Kubernetes network policies are used. More details can be found in the official documentation: [Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/).

There are two primary types of traffic management:

- Ingress – incoming traffic;
- Egress – outgoing traffic.

For controlling intra-cluster traffic, it is recommended to use `podSelector` and `namespaceSelector`. For network interactions outside the cluster, use `ipBlock`.  
Network policy rules are applied simultaneously, following an additive principle, to all virtual machines that match the specified labels.

The following examples will demonstrate usage based on a project named `test-project` with two virtual machines in the `test-project` namespace.

By default, incoming and outgoing traffic is unrestricted:

```shell
d8 k get vm -n test-project
```

Example output:

```console
NAME   PHASE     NODE           IPADDRESS     AGE
vm-a   Running   virtlab-2      10.66.20.70   5m
vm-b   Running   virtlab-1      10.66.20.71   5m
```

Virtual machines have corresponding labels:

```shell
d8 k get vm -n test-project -o yaml | less
```

Example output:

```yaml
- apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualMachine
  metadata:
    labels:
      vm: a
    name: vm-a
    namespace: test-project
- apiVersion: virtualization.deckhouse.io/v1alpha2
  kind: VirtualMachine
  metadata:
    labels:
      vm: b
    name: vm-b
    namespace: test-project
```

## Isolation of all incoming traffic for virtual machine

A network policy that restricts all incoming traffic to virtual machines with the label `vm-a` in the `test-project` namespace:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vm-a-deny-ingress
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  policyTypes:
    - Ingress
```

The policy type Ingress indicates that the rules for incoming traffic will be applied. Since no Ingress rules are specified in the configuration, all incoming traffic will be restricted.

Similarly, outgoing traffic can be restricted by adding Egress to the `spec.policyTypes` block.

```yaml
policyTypes:
  - Egress
  - Ingress
```

## Allowing incoming traffic between virtual machines

A network policy allowing incoming traffic from virtual machines with the label `vm-b` to virtual machines with the label `vm-a`:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-from-vm-b-to-vm-a
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  ingress:
    - from:
      - podSelector:
          matchLabels:
            vm: b
  policyTypes:
    - Ingress
```

With `spec.podSelector`, a network policy with type Ingress is applied to all virtual machines with the label `vm: a`. In the `spec.ingress` specification, a rule is defined that allows incoming traffic `from` virtual machines with the label `vm: b`.

## Allowing outgoing traffic from a virtual machine to external addresses

A network policy that allows outgoing traffic from virtual machines with the label `vm-a` to the external address 8.8.8.8:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-egress-from-vm-a-to-8-8-8-8
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  egress:
    - to:
      - ipBlock:
          cidr: 8.8.8.8/32
        ports:
          - protocol: TCP
            port: 53
  policyTypes:
    - Egress
```

The Egress policy type indicates that outgoing traffic rules will be applied in the `spec.egress` specification. The `TCP` protocol and port `53` are specified, allowing traffic to that port.

Ports can be specified as a range using the additional `endPort` field within the `ports` block.

```yaml
ports:
  - protocol: TCP
    port: 32000
    endPort: 32768
```

## Allowing incoming traffic between namespaces

The network policy allows incoming traffic to virtual machines with the label `vm: a` from the `another-project` namespace, which has the corresponding label `kubernetes.io/metadata.name: another-project`.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-from-namespace-another-project-to-vm-a
  namespace: test-project
spec:
  podSelector:
    matchLabels:
      vm: a
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: another-project
  policyTypes:
    - Ingress
```

## Useful Links

You can find the full specification of network policies in the following documentation:

- [Kubernetes Network Policies Documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies).
- [Kubernetes API Reference (v1.31)](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#networkpolicy-v1-networking-k8s-io).

  Where `1.31` refers to the Kubernetes release version. Please specify the supported version in your cluster if necessary.
