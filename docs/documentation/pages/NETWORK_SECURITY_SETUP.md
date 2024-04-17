---
title: Configuring network policies for Deckhouse
permalink: en/network_security_setup.html
lang: en
---

If the infrastructure where Deckhouse Kubernetes Platform is running has requirements to limit network communication, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [configuration](modules/035-cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* If there is integration with external systems (e.g. LDAP, SMTP or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the tables on the current page.

{% include network_security_setup.liquid %}
