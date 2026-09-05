# Changelog v1.77.1

## Know before update


 - After the update, the controller adds the annotations and labels of every ServiceWithHealthchecks to the Service created for it.
    If a ServiceWithHealthchecks of the `LoadBalancer` type carries annotations that configure the load balancer
    (`network.deckhouse.io/load-balancer-ips`, `network.deckhouse.io/load-balancer-shared-ip-key` and similar),
    the load balancer controller applies them and may assign a different address to the service, which causes a short interruption of the connections.
 - Endpoints for pods in a terminal phase (Failed/Succeeded) are no longer published. In DVP clusters this prevents traffic from being routed to a VirtualMachine IP that has been reused by another pod. Pods being deleted are now published with the serving and terminating conditions, which enables the graceful shutdown flow for consumers. Pod readiness is derived from the PodReady condition, and stale probe results are reset when a pod becomes not ready, is recreated, or changes its IP.

## Fixes


 - **[candi]** Fix hanging sysctl tuner on nodes with many loop and dm devices [#22756](https://github.com/deckhouse/deckhouse/pull/22756)
 - **[deckhouse-controller]** Honor a channel-level release suspend for clusters that reach the suspended version through a step-by-step update. [#22745](https://github.com/deckhouse/deckhouse/pull/22745)
    A Deckhouse release suspended on its release channel is no longer applied by clusters that are behind and reach it through a step-by-step update. The suspend flag lives only in the release-channel image; previously it was dropped when the target release was built from its per-version image, so lagging clusters updated to a suspended release anyway.
 - **[multitenancy-manager]** Refresh grant webhook CA bundles after the admission certificate is rotated. [#22822](https://github.com/deckhouse/deckhouse/pull/22822)
 - **[registrypackages]** Fix dm-verity devices that could never be closed for containerd 2.2.7 (CSE only). [#22807](https://github.com/deckhouse/deckhouse/pull/22807)
 - **[service-with-healthchecks]** Annotations and labels of a ServiceWithHealthchecks are now copied to the Service created for it. [#22835](https://github.com/deckhouse/deckhouse/pull/22835)
    After the update, the controller adds the annotations and labels of every ServiceWithHealthchecks to the Service created for it.
    If a ServiceWithHealthchecks of the `LoadBalancer` type carries annotations that configure the load balancer
    (`network.deckhouse.io/load-balancer-ips`, `network.deckhouse.io/load-balancer-shared-ip-key` and similar),
    the load balancer controller applies them and may assign a different address to the service, which causes a short interruption of the connections.
 - **[service-with-healthchecks]** Stopped publishing terminated pods in EndpointSlices and started publishing pods being deleted as terminating endpoints. [#22836](https://github.com/deckhouse/deckhouse/pull/22836)
    Endpoints for pods in a terminal phase (Failed/Succeeded) are no longer published. In DVP clusters this prevents traffic from being routed to a VirtualMachine IP that has been reused by another pod. Pods being deleted are now published with the serving and terminating conditions, which enables the graceful shutdown flow for consumers. Pod readiness is derived from the PodReady condition, and stale probe results are reset when a pod becomes not ready, is recreated, or changes its IP.

## Chore


 - **[candi]** bump cosign to 3.1.3/2.6.5 [#22761](https://github.com/deckhouse/deckhouse/pull/22761)
 - **[candi]** minget removed from alt_base_images [#22723](https://github.com/deckhouse/deckhouse/pull/22723)
