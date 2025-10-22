# Changelog v1.36

## Know before update


 - All Ingress Nginx controllers with a not-specified version will upgrade to version `1.1`.
 - Deckhouse system controllers can be restarted due to new VPA settings.
 - Ingress Nginx controllers will restart.
 - Removed support of Kubernetes 1.19. You need to migrate to Kubernetes 1.20+ to upgrade Deckhouse to release 1.36.
 - Updating the patch version of istio will cause the `D8IstioDataPlaneVersionMismatch` alert to appear. To make the alert disappear, you must recreate all workloads so that the sidecar runs with the current version.
 - etcd Pods will restart and trigger leader elections. It will affect Kubernetes API performance until all the etcd Pods use the new configuration.

## Features


 - **[candi]** Add the `additionalRolePolicies` parameter to the `AWSClusterConfiguration` resource. [#2256](https://github.com/deckhouse/deckhouse/pull/2256)
 - **[candi]** Added support for Rocky Linux `9.0`. [#2232](https://github.com/deckhouse/deckhouse/pull/2232)
 - **[candi]** Added support for Kubernetes `1.24`. [#2210](https://github.com/deckhouse/deckhouse/pull/2210)
 - **[candi]** Set `maxAllowed` and `minAllowed` to all VPA objects. Set resource requests for all controllers if VPA is off. Added `global.modules.resourcesRequests.controlPlane` values. `global.modules.resourcesRequests.EveryNode` and `global.modules.resourcesRequests.masterNode` values are deprecated. [#1918](https://github.com/deckhouse/deckhouse/pull/1918)
    Deckhouse system controllers can be restarted due to new VPA settings.
 - **[deckhouse]** Add the ability to notify about the upcoming Deckhouse release via a webhook. [#2131](https://github.com/deckhouse/deckhouse/pull/2131)
 - **[docs]** Added the `How to install?` section on the site. [#2367](https://github.com/deckhouse/deckhouse/pull/2367)
 - **[ingress-nginx]** Changed default Ingress Nginx controller version to version `1.1`. [#2267](https://github.com/deckhouse/deckhouse/pull/2267)
    All Ingress Nginx controllers with a not-specified version will upgrade to version `1.1`.
 - **[linstor]** Automate kernel headers installation. [#2287](https://github.com/deckhouse/deckhouse/pull/2287)
 - **[log-shipper]** Refactored transforms composition, improved efficiency, and fixed destination transforms. [#2050](https://github.com/deckhouse/deckhouse/pull/2050)
 - **[monitoring-kubernetes]** New Capacity Planning dashboard. [#2365](https://github.com/deckhouse/deckhouse/pull/2365)
 - **[monitoring-kubernetes]** Add GRPC request handling time and etcd peer RTT graphs to etcd dashboard. [#2360](https://github.com/deckhouse/deckhouse/pull/2360)
    etcd Pods will restart and trigger leader elections. It will affect Kubernetes API performance until all the etcd Pods use the new configuration.
 - **[monitoring-kubernetes]** Add nodes count panel to the Nodes dashboard. [#2196](https://github.com/deckhouse/deckhouse/pull/2196)
 - **[node-manager]** Switched early-oom to PSI metrics [#2358](https://github.com/deckhouse/deckhouse/pull/2358)
 - **[prometheus]** Create mTLS secret to scrape metrics from workloads with PeerAuthentication mtls.mode = STRICT. [#2332](https://github.com/deckhouse/deckhouse/pull/2332)

## Fixes


 - **[cloud-provider-aws]** Bump cloud provider version to fix LB target group creation. [#2560](https://github.com/deckhouse/deckhouse/pull/2560)
 - **[cni-cilium]** Fix cilium mode for static clusters. [#2452](https://github.com/deckhouse/deckhouse/pull/2452)
    `cilium-agents` will be restarted.
 - **[cni-cilium]** Fix Cilium Terminating Endpoints with `externalTrafficPolicy: Local`. Backported https://github.com/cilium/cilium/pull/21062 [#2324](https://github.com/deckhouse/deckhouse/pull/2324)
 - **[control-plane-manager]** Fixed panic when a node with minimal RAM cannot be found. [#2635](https://github.com/deckhouse/deckhouse/pull/2635)
 - **[deckhouse]** Bump shell-operator image to avoid panic. [#2547](https://github.com/deckhouse/deckhouse/pull/2547)
 - **[deckhouse]** Fix stucked `DeckhouseUpdating` alert during the deckhouse update process. [#2472](https://github.com/deckhouse/deckhouse/pull/2472)
 - **[deckhouse]** Fix panic in a release tracking during the deckhouse update process. [#2465](https://github.com/deckhouse/deckhouse/pull/2465)
 - **[dhctl]** Fail if there is an empty host for SSH connection. [#2346](https://github.com/deckhouse/deckhouse/pull/2346)
 - **[flant-integration]** Fixed telemetry reporting control-plane nodes as nodes for charge. [#2617](https://github.com/deckhouse/deckhouse/pull/2617)
 - **[ingress-nginx]** Improve metrics collection script. [#2350](https://github.com/deckhouse/deckhouse/pull/2350)
    Ingress Nginx controllers will restart.
 - **[ingress-nginx]** The ability to change the `defaultControllerVersion` parameter without rebooting Deckhouse. [#2338](https://github.com/deckhouse/deckhouse/pull/2338)
 - **[istio]** Fix `D8IstioDataPlaneWithoutIstioInjectionConfigured` alert description. [#2599](https://github.com/deckhouse/deckhouse/pull/2599)
 - **[istio]** Fixed `D8IstioActualDataPlaneRevisionNeDesired` alert. [#2558](https://github.com/deckhouse/deckhouse/pull/2558)
 - **[istio]** Fix default `tlsMode` behavior. [#2479](https://github.com/deckhouse/deckhouse/pull/2479)
 - **[istio]** Fix `tlsMode` param behavior. [#2385](https://github.com/deckhouse/deckhouse/pull/2385)
 - **[istio]** Use `maxUnavailable` strategy for istiod Deployment instead of the default one (25%). [#2202](https://github.com/deckhouse/deckhouse/pull/2202)
 - **[linstor]** Fix linstor-affinity-controller leader election. [#2489](https://github.com/deckhouse/deckhouse/pull/2489)
 - **[linstor]** fix image tag regression [#2437](https://github.com/deckhouse/deckhouse/pull/2437)
    default
 - **[linstor]** Bump DRBD version to `9.1.9`. [#2359](https://github.com/deckhouse/deckhouse/pull/2359)
 - **[linstor]** Change module order. [#2323](https://github.com/deckhouse/deckhouse/pull/2323)
 - **[log-shipper]** Stop generating pointless 'parse_json' transform, which improves performance. [#2619](https://github.com/deckhouse/deckhouse/pull/2619)
 - **[log-shipper]** Fix the bug when the many sources point to the same input and only the last is working. [#2619](https://github.com/deckhouse/deckhouse/pull/2619)
 - **[log-shipper]** Ignore pipelines without destinations. [#2480](https://github.com/deckhouse/deckhouse/pull/2480)
 - **[log-shipper]** Rewrite Elasticsearch dedot rule in VRL to improve performance. [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[log-shipper]** Prevent Vector from stopping logs processing if Kubernetes API server was restarted. [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[log-shipper]** Fix memory leak for internal metrics. [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[monitoring-kubernetes]** Add deployments to kube-state-metrics's allowlist. [#2642](https://github.com/deckhouse/deckhouse/pull/2642)
 - **[monitoring-kubernetes]** Check current `node_memory_SUnreclaim_bytes` in the `NodeSUnreclaimBytesUsageHigh` alert. [#2510](https://github.com/deckhouse/deckhouse/pull/2510)
 - **[monitoring-kubernetes]** Better way to ignore kubelet mounts for node-exporter. [#2427](https://github.com/deckhouse/deckhouse/pull/2427)
 - **[monitoring-kubernetes]** Change steppedLine to false for CPU panels and add sorting. [#2371](https://github.com/deckhouse/deckhouse/pull/2371)
 - **[node-manager]** Fixed failing on not existing control-plane node labels. [#2635](https://github.com/deckhouse/deckhouse/pull/2635)
 - **[node-manager]** Updated govmomi to the latest version so that VMs with Networks under Distributed Virtual Switch can be created. [#2444](https://github.com/deckhouse/deckhouse/pull/2444)
 - **[node-manager]** Fail early-oom gracefully with a helpful log message and a low-severity alert if PSI subsystem is unavailable. [#2451](https://github.com/deckhouse/deckhouse/pull/2451)
 - **[node-manager]** Fixed a bug with spot Machine deletion in Yandex.Cloud. It now correctly deletes machines in 15 minute intervals. [#2394](https://github.com/deckhouse/deckhouse/pull/2394)
 - **[node-manager]** Do not drain single-master and single standalone nodes where Deckhouse works with automatic approve mode for disruption. [#2386](https://github.com/deckhouse/deckhouse/pull/2386)
    A node in the `master` nodeGroup with a single node and `Automatic` disruption approval mode will not be drained before approval.
    If Deckhouse works not on a master node and this node is single (or one node in Ready status) in the nodeGroup, and for this nodeGroup the `Automatic` disruption approval mode is set, then disruption operations will be approved for this node without draining.
 - **[node-manager]** Changed cluster autoscaler timeouts to avoid node flapping. [#2279](https://github.com/deckhouse/deckhouse/pull/2279)
 - **[prometheus]** Fixed input lag by bumping Grafana version to `8.5.13`. [#2603](https://github.com/deckhouse/deckhouse/pull/2603)
 - **[snapshot-controller]** Fix `maxSurge` for `snapshot-validation-webhook`. [#2450](https://github.com/deckhouse/deckhouse/pull/2450)
 - **[upmeter]** Reduces QPS and burst in `upmeter-agent` to reduce `kube-apiserver` latency in multi-control-plane setups. [#2666](https://github.com/deckhouse/deckhouse/pull/2666)
 - **[upmeter]** Fix non-working statuspage by removing hardcoded localhost in backend URL [#2499](https://github.com/deckhouse/deckhouse/pull/2499)
 - **[upmeter]** Fix deckhouse probe by placing EnableKubeEventCb call properly. [#2422](https://github.com/deckhouse/deckhouse/pull/2422)
 - **[upmeter]** Bundled CSS into the status page for the desired look in restricted environments. [#2349](https://github.com/deckhouse/deckhouse/pull/2349)

## Chore


 - **[candi]** Removed support of Kubernetes 1.19. [#2255](https://github.com/deckhouse/deckhouse/pull/2255)
    Removed support of Kubernetes 1.19. You need to migrate to Kubernetes 1.20+ to upgrade Deckhouse to release 1.36.
 - **[cni-cilium]** Reverted bpf masquerading mode for all cilium installations except in OpenStack.
    Set `rp_filter` to 0 for all interfaces in the `sysctl_tuner` script. [#2481](https://github.com/deckhouse/deckhouse/pull/2481)
 - **[cni-cilium]** Cilium various fixes. [#2252](https://github.com/deckhouse/deckhouse/pull/2252)
    `cilium-agent` will restart.
 - **[deckhouse]** Changed the module order from 020 to 002. [#2412](https://github.com/deckhouse/deckhouse/pull/2412)
 - **[flant-integration]** Added new distros supported by Deckhouse. [#2284](https://github.com/deckhouse/deckhouse/pull/2284)
 - **[istio]** Bump istio version from `1.13.3` to `1.13.7`. [#2400](https://github.com/deckhouse/deckhouse/pull/2400)
    Updating the patch version of istio will cause the `D8IstioDataPlaneVersionMismatch` alert to appear. To make the alert disappear, you must recreate all workloads so that the sidecar runs with the current version.
 - **[istio]** Added `D8IstioDeprecatedIstioVersionInstalled` alert for depricated istio versions. [#2389](https://github.com/deckhouse/deckhouse/pull/2389)
 - **[istio]** Refactored istio revision monitoring. [#2273](https://github.com/deckhouse/deckhouse/pull/2273)
 - **[log-shipper]** Update Vector to 0.23 [#2192](https://github.com/deckhouse/deckhouse/pull/2192)
 - **[monitoring-deckhouse]** Separate monitoring rules from the `deckhouse` module to handle CRDs creation order. [#2412](https://github.com/deckhouse/deckhouse/pull/2412)
 - **[monitoring-kubernetes]** Bump `kube-state-metrics` to version `2.6.0`. [#2291](https://github.com/deckhouse/deckhouse/pull/2291)
 - **[node-manager]** Added automatic migration to the `control-plane` node role. [#2635](https://github.com/deckhouse/deckhouse/pull/2635)
 - **[node-manager]** Clarify master NodeGroup cri change. [#2525](https://github.com/deckhouse/deckhouse/pull/2525)
 - **[priority-class]** Changed the module order from 010 to 001. [#2412](https://github.com/deckhouse/deckhouse/pull/2412)

