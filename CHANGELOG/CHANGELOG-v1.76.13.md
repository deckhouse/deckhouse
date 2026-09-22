# Changelog v1.76.13

## Know before update


 - After the update, the controller adds the annotations and labels of every ServiceWithHealthchecks to the Service created for it.
    If a ServiceWithHealthchecks of the `LoadBalancer` type carries annotations that configure the load balancer
    (`network.deckhouse.io/load-balancer-ips`, `network.deckhouse.io/load-balancer-shared-ip-key` and similar),
    the load balancer controller applies them and may assign a different address to the service, which causes a short interruption of the connections.
 - Endpoints for pods in a terminal phase (Failed/Succeeded) are no longer published. In DVP clusters this prevents traffic from being routed to a VirtualMachine IP that has been reused by another pod. Pods being deleted are now published with the serving and terminating conditions, which enables the graceful shutdown flow for consumers. Pod readiness is derived from the PodReady condition, and stale probe results are reset when a pod becomes not ready, is recreated, or changes its IP.
 - The SSH key and sudo password in `SSHCredentials` (used by CAPS to reach static nodes) are now
    returned as `<omitted>` to callers without `get` on the `sshcredentials/sensitive` subresource,
    masked in the audit log, and encrypted in etcd when `apiserver.encryptionEnabled` is on.
    `d8:manage:infrastructure:viewer` and `:manager` no longer see these values. CAPS still does, and
    so does every holder of a `deckhouse.io` wildcard, since `*` matches subresources: `SuperAdmin`,
    the `kubeadm:cluster-admins` group, and the `deckhouse` and `webhook-handler` SAs of `d8-system`.
    
    For masked readers the `last-applied-configuration` annotation disappears from `get -o yaml`,
    and creating an `SSHCredentials` with `<omitted>` fails with `422 Invalid` — editing still works.
    Treat previously exposed keys as leaked and rotate them. With `apiserver.encryptionEnabled` on,
    rewrite existing objects to encrypt them: `d8 k get sshcredentials -o json | d8 k replace -f -`.
 - The `service-with-healthchecks` status logic was heavily refactored to reduce API and etcd load. If you rely on `lastProbeTime` observability on every probe, explicitly enable `verboseStatus` in the module configuration.

## Fixes


 - **[common]** Quote user-controlled values in the prometheus, upmeter, prometheus-pushgateway and extended-monitoring templates to prevent YAML injection into the module release. [#22774](https://github.com/deckhouse/deckhouse/pull/22774)
 - **[deckhouse-controller]** Honor a channel-level release suspend for clusters that reach the suspended version through a step-by-step update. [#22746](https://github.com/deckhouse/deckhouse/pull/22746)
    A Deckhouse release suspended on its release channel is no longer applied by clusters that are behind and reach it through a step-by-step update. The suspend flag lives only in the release-channel image; previously it was dropped when the target release was built from its per-version image, so lagging clusters updated to a suspended release anyway.
 - **[node-manager]** Fix NodeCapacity calculation. [#22677](https://github.com/deckhouse/deckhouse/pull/22677)
 - **[node-manager]** Hide the CAPS SSH key and sudo password in SSHCredentials from users without the `sshcredentials/sensitive` subresource. [#22631](https://github.com/deckhouse/deckhouse/pull/22631)
    The SSH key and sudo password in `SSHCredentials` (used by CAPS to reach static nodes) are now
    returned as `<omitted>` to callers without `get` on the `sshcredentials/sensitive` subresource,
    masked in the audit log, and encrypted in etcd when `apiserver.encryptionEnabled` is on.
    `d8:manage:infrastructure:viewer` and `:manager` no longer see these values. CAPS still does, and
    so does every holder of a `deckhouse.io` wildcard, since `*` matches subresources: `SuperAdmin`,
    the `kubeadm:cluster-admins` group, and the `deckhouse` and `webhook-handler` SAs of `d8-system`.
    
    For masked readers the `last-applied-configuration` annotation disappears from `get -o yaml`,
    and creating an `SSHCredentials` with `<omitted>` fails with `422 Invalid` — editing still works.
    Treat previously exposed keys as leaked and rotate them. With `apiserver.encryptionEnabled` on,
    rewrite existing objects to encrypt them: `d8 k get sshcredentials -o json | d8 k replace -f -`.
 - **[service-with-healthchecks]** Annotations and labels of a ServiceWithHealthchecks are now copied to the Service created for it. [#22878](https://github.com/deckhouse/deckhouse/pull/22878)
    After the update, the controller adds the annotations and labels of every ServiceWithHealthchecks to the Service created for it.
    If a ServiceWithHealthchecks of the `LoadBalancer` type carries annotations that configure the load balancer
    (`network.deckhouse.io/load-balancer-ips`, `network.deckhouse.io/load-balancer-shared-ip-key` and similar),
    the load balancer controller applies them and may assign a different address to the service, which causes a short interruption of the connections.
 - **[service-with-healthchecks]** Fixed an API server overload issue ("status storm"), resolved validation errors for ClusterIP services, corrected pod readiness evaluation logic, and improved code quality. [#19455](https://github.com/deckhouse/deckhouse/pull/19455)
    The `service-with-healthchecks` status logic was heavily refactored to reduce API and etcd load. If you rely on `lastProbeTime` observability on every probe, explicitly enable `verboseStatus` in the module configuration.
 - **[service-with-healthchecks]** Stopped publishing terminated pods in EndpointSlices and started publishing pods being deleted as terminating endpoints. [#22879](https://github.com/deckhouse/deckhouse/pull/22879)
    Endpoints for pods in a terminal phase (Failed/Succeeded) are no longer published. In DVP clusters this prevents traffic from being routed to a VirtualMachine IP that has been reused by another pod. Pods being deleted are now published with the serving and terminating conditions, which enables the graceful shutdown flow for consumers. Pod readiness is derived from the PodReady condition, and stale probe results are reset when a pod becomes not ready, is recreated, or changes its IP.
