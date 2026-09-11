# Changelog v1.77.1

## Know before update


 - After the update, the controller adds the annotations and labels of every ServiceWithHealthchecks to the Service created for it.
    If a ServiceWithHealthchecks of the `LoadBalancer` type carries annotations that configure the load balancer
    (`network.deckhouse.io/load-balancer-ips`, `network.deckhouse.io/load-balancer-shared-ip-key` and similar),
    the load balancer controller applies them and may assign a different address to the service, which causes a short interruption of the connections.
 - CAPI NodeGroups whose MachineDeployment was stuck on an old MachineTemplate are rolled to the current InstanceClass after the update, within maxSurgePerZone/maxUnavailablePerZone. Groups already on the current template are not touched.
 - During the first `user-authz` release after the update the per-role bindings are protected from the release engine, the aggregated ones are created, and the per-role bindings are deleted right after the release. Permissions granted through annotated ClusterRoles stay in place throughout.
    The `user-authz.deckhouse.io/access-level` label is now set automatically on annotated ClusterRoles.
    In audit logs, access granted through annotated ClusterRoles is attributed to `user-authz:<level>:custom` instead of the individual ClusterRole.
 - Endpoints for pods in a terminal phase (Failed/Succeeded) are no longer published. In DVP clusters this prevents traffic from being routed to a VirtualMachine IP that has been reused by another pod. Pods being deleted are now published with the serving and terminating conditions, which enables the graceful shutdown flow for consumers. Pod readiness is derived from the PodReady condition, and stale probe results are reset when a pod becomes not ready, is recreated, or changes its IP.

## Fixes


 - **[candi]** Fix hanging sysctl tuner on nodes with many loop and dm devices [#22756](https://github.com/deckhouse/deckhouse/pull/22756)
 - **[candi]** containerd v2 no longer starts without the `erofs` snapshotter, and loads the `erofs` module before start. [#22885](https://github.com/deckhouse/deckhouse/pull/22885)
 - **[cloud-provider-aws]** fix GetCapacity not implemented error spam in logs [#22903](https://github.com/deckhouse/deckhouse/pull/22903)
 - **[cloud-provider-azure]** fix GetCapacity not implemented error spam in logs [#22903](https://github.com/deckhouse/deckhouse/pull/22903)
 - **[cloud-provider-dvp]** fix GetCapacity not implemented error spam in logs [#22903](https://github.com/deckhouse/deckhouse/pull/22903)
 - **[cloud-provider-gcp]** fix GetCapacity not implemented error spam in logs [#22903](https://github.com/deckhouse/deckhouse/pull/22903)
 - **[deckhouse-controller]** Honor a channel-level release suspend for clusters that reach the suspended version through a step-by-step update. [#22745](https://github.com/deckhouse/deckhouse/pull/22745)
    A Deckhouse release suspended on its release channel is no longer applied by clusters that are behind and reach it through a step-by-step update. The suspend flag lives only in the release-channel image; previously it was dropped when the target release was built from its per-version image, so lagging clusters updated to a suspended release anyway.
 - **[multitenancy-manager]** Refresh grant webhook CA bundles after the admission certificate is rotated. [#22822](https://github.com/deckhouse/deckhouse/pull/22822)
 - **[node-manager]** node-controller no longer removes taints set by other components on the first reconcile of Static and CloudPermanent nodes. [#22988](https://github.com/deckhouse/deckhouse/pull/22988)
 - **[node-manager]** node-controller reads the CAPI instance-class checksum from a helm-rendered ConfigMap instead of a possibly stale MachineTemplate, so MachineDeployments follow InstanceClass changes and their bootstrap Secrets keep a valid token. [#22909](https://github.com/deckhouse/deckhouse/pull/22909)
    CAPI NodeGroups whose MachineDeployment was stuck on an old MachineTemplate are rolled to the current InstanceClass after the update, within maxSurgePerZone/maxUnavailablePerZone. Groups already on the current template are not touched.
 - **[registry]** Fix the order in which the Unmanaged params are stored for the fallback to the Legacy mode. [#22976](https://github.com/deckhouse/deckhouse/pull/22976)
 - **[registrypackages]** Fix dm-verity devices that could never be closed for containerd 2.2.7 (CSE only). [#22807](https://github.com/deckhouse/deckhouse/pull/22807)
 - **[service-with-healthchecks]** Annotations and labels of a ServiceWithHealthchecks are now copied to the Service created for it. [#22835](https://github.com/deckhouse/deckhouse/pull/22835)
    After the update, the controller adds the annotations and labels of every ServiceWithHealthchecks to the Service created for it.
    If a ServiceWithHealthchecks of the `LoadBalancer` type carries annotations that configure the load balancer
    (`network.deckhouse.io/load-balancer-ips`, `network.deckhouse.io/load-balancer-shared-ip-key` and similar),
    the load balancer controller applies them and may assign a different address to the service, which causes a short interruption of the connections.
 - **[service-with-healthchecks]** Fixed a load balancer losing all its endpoints for good after a target pod was replaced. [#22979](https://github.com/deckhouse/deckhouse/pull/22979)
    The `service-with-healthchecks` controller and agent are restarted.
    A `ServiceWithHealthchecks` that was left in `NotAllEndpointsAreReady` with no EndpointSlice
    recovers automatically once the updated agent starts, no manual action is needed.
 - **[service-with-healthchecks]** Stopped publishing terminated pods in EndpointSlices and started publishing pods being deleted as terminating endpoints. [#22836](https://github.com/deckhouse/deckhouse/pull/22836)
    Endpoints for pods in a terminal phase (Failed/Succeeded) are no longer published. In DVP clusters this prevents traffic from being routed to a VirtualMachine IP that has been reused by another pod. Pods being deleted are now published with the serving and terminating conditions, which enables the graceful shutdown flow for consumers. Pod readiness is derived from the PodReady condition, and stale probe results are reset when a pod becomes not ready, is recreated, or changes its IP.
 - **[user-authz]** Grant custom ClusterRoles (annotated with `user-authz.deckhouse.io/access-level`) through one aggregated ClusterRole per access level, so the number of bindings per ClusterAuthorizationRule/AuthorizationRule no longer depends on the number of such roles. [#22967](https://github.com/deckhouse/deckhouse/pull/22967)
    During the first `user-authz` release after the update the per-role bindings are protected from the release engine, the aggregated ones are created, and the per-role bindings are deleted right after the release. Permissions granted through annotated ClusterRoles stay in place throughout.
    The `user-authz.deckhouse.io/access-level` label is now set automatically on annotated ClusterRoles.
    In audit logs, access granted through annotated ClusterRoles is attributed to `user-authz:<level>:custom` instead of the individual ClusterRole.
 - **[user-authz]** Group the module alerts under a group named apart from the alerts themselves, so the incident shelf accepts them. [#22994](https://github.com/deckhouse/deckhouse/pull/22994)

## Chore


 - **[candi]** bump cosign to 3.1.3/2.6.5 [#22761](https://github.com/deckhouse/deckhouse/pull/22761)
 - **[candi]** minget removed from alt_base_images [#22723](https://github.com/deckhouse/deckhouse/pull/22723)
 - **[deckhouse]** Update nelm version to v1.30.3. [#23005](https://github.com/deckhouse/deckhouse/pull/23005)
