---
title: "Network policies"
permalink: en/admin/network/network-policies-overview.html
---

<!-- Transferred from https://deckhouse.io/products/kubernetes-platform/documentation/latest/network_security_setup.html -->

If the infrastructure where Deckhouse Kubernetes Platform is running has requirements to limit host-to-host network communications, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](../../reference/mc/cni-cilium/#parameters-tunnelmode) for CNI Cilium, [configuration](modules/cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* Traffic between [podSubnetCIDR](../../reference/mc/cni-flannel/#parameters-podnetworkmode) encapsulated within a VXLAN is allowed (if inspection and filtering of traffic within a VXLAN tunnel is performed).
* If there is integration with external systems (e.g. LDAP, SMTP or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the tables on the current page. Note that most ports are in the 4200-4299 range. When new platform components are added, they will be assigned ports from this range (if it is possible).

{% include network_security_setup.liquid %}
