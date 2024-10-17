---
title: Configuring network policies for Deckhouse
permalink: en/network_security_setup.html
description: |
  Detailed information on configuring network policies for the Deckhouse Kubernetes Platform, particularly in environments with constraints on host-to-host network communications. Outlines the necessary conditions to enable tunneling modes for pod traffic using CNI Cilium and Flannel.
lang: en
---

If the infrastructure where Deckhouse Kubernetes Platform is running has requirements to limit host-to-host network communications, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [configuration](modules/035-cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* If there is integration with external systems (e.g. LDAP, SMTP or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the tables on the current page. Note that most ports are in the 4200-4299 range. When new platform components are added, they will be assigned ports from this range (if it is possible).

{% include network_security_setup.liquid %}
