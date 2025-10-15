---
title: Network interaction of the platform components
permalink: en/reference/network_interaction.html
description: |
  Detailed information on configuring network policies for the Deckhouse Kubernetes Platform, particularly in environments with constraints on host-to-host network communications. Outlines the necessary conditions to enable tunneling modes for pod traffic using CNI Cilium and Flannel.
lang: en
---

If the infrastructure where Deckhouse Kubernetes Platform (DKP) is running has requirements to limit host-to-host network communications, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](/modules/cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [configuration](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* Traffic between [podSubnetCIDR](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) encapsulated within a VXLAN is allowed (if inspection and filtering of traffic within a VXLAN tunnel is performed).
* If there is integration with external systems (e.g. LDAP, SMTP or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the tables on the current page. Note that most ports are in the 4200-4299 range. When new platform components are added, they will be assigned ports from this range (if it is possible).

{% offtopic title="How to check the current VXLAN port..." %}

```bash
d8 k -n d8-cni-cilium get cm cilium-config -o yaml | grep tunnel
```

Example output:

```console
routing-mode: tunnel
tunnel-port: "4298"
tunnel-protocol: vxlan
```

{%- endofftopic %}

{% alert level="info" %}
Changes related to the addition, removal, or reassignment of ports in the tables
are listed in the "Network" section of a respective DKP version on the [Release notes](../release-notes.html) page.
{% endalert %}

{% include network_security_setup.liquid %}
