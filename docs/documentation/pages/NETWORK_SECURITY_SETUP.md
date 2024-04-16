---
title: Configuring network policies for Deckhouse
permalink: en/network_security_setup.html
lang: en
---

If the infrastructure where Deckhouse Kubernetes Pllatform is running has requirements to limit network communication, the following conditions must be met:

* [Tunneling mode](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) for traffic between pods is enabled.
* Inter-node communication is allowed on the ports shown in the tables on the current page.

{% include network_security_setup.liquid %}
