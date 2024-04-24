---
title: "The l2-load-balancer module"
---

{% alert level="warning" %} The module is **experimental**, don't use it in production environments.

The module implements an improved (relative to the standard [L2 mode in MetalLB](../../modules/380-metallb/#layer-2-mode)) balancing mechanism for services in bare metal clusters when there is no option to use cloud load balancers or [MetalLB](../../modules/380-metallb/#mode-bgp) in BGP mode with Equal-cost multi-path (ECMP) configured.

Based on [MetalLB](https://metallb.universe.tf/) solution.

## Principle of operation compared to L2 mode in MetalLB module

[MetalLB in L2 mode](../../modules/380-metallb/#layer-2-mode) allows to order _Service_ with `LoadBalancer` type, the operation of which is based on the fact that balancing nodes simulate ARP-responses from the "public" IP in a peering network. This mode has a significant limitation — only one balancing node handles all the incoming traffic of this service at a time. Therefore:

* The node selected as the leader for the "public" IP becomes a "bottleneck" with no possibility of horizontal scaling.
* If the balancer node fails, all current connections will be dropped for switching to a new balancing node that will be selected as the leader.

<div data-presentation="../../presentations/381-l2-load-balancer/basics_metallb_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/18vcVJ1cY2yn19vBM_dTNW3hF0w9SE4S81VZc2P6fVFM/ --->

This module helps bypass these limitations. It provides a new _L2LoadBalancer_ interface that:

* Allows to automatically order multiple _Service_ with type `LoadBalancer` according to the number of balancer nodes.
* Provides an even distribution of L2 leadership among the balancer nodes.

<div data-presentation="../../presentations/381-l2-load-balancer/basics_l2loadbalancer_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1rVQ2RZ645HKAcMFU2hI_MPJZu_zA70IZ2OYWI1wSW34/ --->

Thus:
* The application will receive not a single, but several (according to the number of balancer nodes) "public" IPs. These IPs will need to be configured as A-records for the application's public domain. For further horizontal scaling, additional balancer nodes will need to be added, the corresponding _Service_ will be created automatically, you just need to add them to the list of A-records for the application domain.
* If one of the balancer nodes fails, only a part of the connections will be prone to failover to the healthy node.
