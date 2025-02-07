---
title: "Network policies"
permalink: en/admin/network/network-policies-overview.html
---

<!-- Transferred from https://deckhouse.io/products/kubernetes-platform/documentation/latest/network_security_setup.html -->

If the infrastructure where Deckhouse Kubernetes Platform is running has requirements to limit host-to-host network communications, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](modules/cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [configuration](modules/cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* Traffic between [podSubnetCIDR](installing/configuration.html#clusterconfiguration) encapsulated within a VXLAN is allowed (if inspection and filtering of traffic within a VXLAN tunnel is performed).
* If there is integration with external systems (e.g. LDAP, SMTP or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the tables on the current page. Note that most ports are in the 4200-4299 range. When new platform components are added, they will be assigned ports from this range (if it is possible).

{% include network_security_setup.liquid %}

<!-- example taken from tutorials -->

## Network policy configuration example

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
  namespace: default
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
          project: myproject
    - podSelector:
        matchLabels:
          role: frontend
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
