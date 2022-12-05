# Changelog v1.34

## Know before update


 - **All Deckhouse components will be restarted** including control-plane, ingress-nginx.
 - After upgrading Deckhouse all nodes with Docker 18.* will request `disruptive update`. You will receive `NodeRequiresDisruptionApprovalForUpdate` if you have manual `approvalMode` in NodeGroup.
 - After upgrading Deckhouse, all nodes with the installed `docker.io` package will request `disruptive update`. You will receive `NodeRequiresDisruptionApprovalForUpdate` if you have manual `approvalMode` in the NodeGroup.
 - Modified existing alerts:
    * Removed predefined groups in Polk.
    * Added group auto-creation in Polk.
    * Added the `for` parameter for all alerts.
    * Removed the `plk_pending_until_firing_for` annotation from all alerts. LGTM as far as can evaluate alerts.
 - The `ru-central1-c` **Yandex.cloud** zone was [deprecated](https://cloud.yandex.com/en/docs/overview/concepts/ru-central1-c-deprecation).
    For new clusters NAT-instance will be created in `ru-central1-a` zone. For old instances you should add to `withNATInstance.natInstanceInternalAddress` (you can get address from Yandex.Cloud console) 
    and `withNATInstance.internalSubnetID` (you can get address using command `kubectl -n d8-system exec -it deploy/deckhouse -- deckhouse-controller module values cloud-provider-yandex -o json | jq -r '.cloudProviderYandex.internal.providerDiscoveryData.zoneToSubnetIdMap["ru-central1-c"]'`) to prevent NAT-instance recreation during a converge process.

## Features


 - **[candi]** Forbid use docker.io package. [#2175](https://github.com/deckhouse/deckhouse/pull/2175)
    After upgrading Deckhouse, all nodes with the installed `docker.io` package will request `disruptive update`. You will receive `NodeRequiresDisruptionApprovalForUpdate` if you have manual `approvalMode` in the NodeGroup.
 - **[candi]** Forbid using Docker 18.* [#2134](https://github.com/deckhouse/deckhouse/pull/2134)
    After upgrading Deckhouse all nodes with Docker 18.* will request `disruptive update`. You will receive `NodeRequiresDisruptionApprovalForUpdate` if you have manual `approvalMode` in NodeGroup.
 - **[candi]** New Kuberenetes patch versions. [#1724](https://github.com/deckhouse/deckhouse/pull/1724)
    Restart of Kubernetes control plane components.
 - **[cert-manager]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[cloud-provider-azure]** Provide new default StorageClasses for disks large than 4 TiB. [#1652](https://github.com/deckhouse/deckhouse/pull/1652)
 - **[cloud-provider-yandex]** Validate `serviceAccountJSON`. [#1904](https://github.com/deckhouse/deckhouse/pull/1904)
 - **[cloud-provider-yandex]** Move NAT-instance to `ru-central1-a` for new instances. [#1592](https://github.com/deckhouse/deckhouse/pull/1592)
    The `ru-central1-c` **Yandex.cloud** zone was [deprecated](https://cloud.yandex.com/en/docs/overview/concepts/ru-central1-c-deprecation).
    For new clusters NAT-instance will be created in `ru-central1-a` zone. For old instances you should add to `withNATInstance.natInstanceInternalAddress` (you can get address from Yandex.Cloud console) 
    and `withNATInstance.internalSubnetID` (you can get address using command `kubectl -n d8-system exec -it deploy/deckhouse -- deckhouse-controller module values cloud-provider-yandex -o json | jq -r '.cloudProviderYandex.internal.providerDiscoveryData.zoneToSubnetIdMap["ru-central1-c"]'`) to prevent NAT-instance recreation during a converge process.
 - **[control-plane-manager]** Added feature gate `EndpointSliceTerminatingCondition` for Kubernetes 1.20. [#2112](https://github.com/deckhouse/deckhouse/pull/2112)
    all control-plane components should be restarted.
 - **[control-plane-manager]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[deckhouse]** Add collect debug info command [#1787](https://github.com/deckhouse/deckhouse/pull/1787)
 - **[deckhouse]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[dhctl]** Add confirmation for waiting Deckhouse controller readiness and control-plane node readiness. [#1629](https://github.com/deckhouse/deckhouse/pull/1629)
 - **[extended-monitoring]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[ingress-nginx]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[linstor]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[log-shipper]** Added new logs destination - Vector [#1730](https://github.com/deckhouse/deckhouse/pull/1730)
 - **[log-shipper]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[monitoring-applications]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[monitoring-kubernetes]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[monitoring-kubernetes-control-plane]** Added dashboard showing deprecated APIs. [#1867](https://github.com/deckhouse/deckhouse/pull/1867)
 - **[monitoring-kubernetes-control-plane]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[node-local-dns]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[node-manager]** Yandex.Cloud's Preemptible instances will start being gracefully deleted when crossing 20 hours since creation. [#1744](https://github.com/deckhouse/deckhouse/pull/1744)
 - **[node-manager]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[okmeter]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[operator-prometheus]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[prometheus]** Do not collect some deprecated and unneeded metrics [#1925](https://github.com/deckhouse/deckhouse/pull/1925)
 - **[prometheus]** Validate `GrafanaDashboardDefinition` definition field, and add a readiness probe for the Grafana dashboard provisioner. [#1904](https://github.com/deckhouse/deckhouse/pull/1904)
 - **[prometheus]** Added ability to deploy in-cluster Alertmanager. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
    Modified existing alerts:
    * Removed predefined groups in Polk.
    * Added group auto-creation in Polk.
    * Added the `for` parameter for all alerts.
    * Removed the `plk_pending_until_firing_for` annotation from all alerts. LGTM as far as can evaluate alerts.
 - **[snapshot-controller]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[terraform-manager]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[upmeter]** Added basic probe for cert-manager in `control-plane` availability group. [#1760](https://github.com/deckhouse/deckhouse/pull/1760)
 - **[upmeter]** Added dynamic probes for Nginx Ingress Controller Pods. [#1701](https://github.com/deckhouse/deckhouse/pull/1701)
 - **[upmeter]** Added dynamic probes for the violation of the minimal expected count of nodes in `CloudEphemeral` node groups. [#1701](https://github.com/deckhouse/deckhouse/pull/1701)
 - **[upmeter]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[user-authn]** Make the kubelogin tab active by default. [#1827](https://github.com/deckhouse/deckhouse/pull/1827)

## Fixes


 - **[candi]** Fix bash for start kubelet step. [#2174](https://github.com/deckhouse/deckhouse/pull/2174)
 - **[candi]** Use Debian buster containerd package by default. [#2135](https://github.com/deckhouse/deckhouse/pull/2135)
 - **[candi]** Start kubelet manually if it is not running. [#2132](https://github.com/deckhouse/deckhouse/pull/2132)
 - **[candi]** Fix docker config creation. [#2044](https://github.com/deckhouse/deckhouse/pull/2044)
 - **[candi]** Fixed the applying of disk size for CloudPermanent nodes in `YandexClusterConfiguration`. [#1900](https://github.com/deckhouse/deckhouse/pull/1900)
 - **[cert-manager]** Fix patch for cert-manager certificate owner ref field [#1985](https://github.com/deckhouse/deckhouse/pull/1985)
 - **[cert-manager]** Respect the global IngressClass in the `letsencrypt` ClusterIssuer. [#1750](https://github.com/deckhouse/deckhouse/pull/1750)
 - **[cni-cilium]** Bandwidth controller metrics are not erroring out now. Also added logging to three controllers so that we can diagnose possible issues better. [#2155](https://github.com/deckhouse/deckhouse/pull/2155)
 - **[deckhouse]** Change DeckhouseUpdating Prometheus rule severity_level to avoid alert deferring [#1929](https://github.com/deckhouse/deckhouse/pull/1929)
 - **[dhctl]** Do not try to remove the `dhctl.deckhouse.io/node-for-converge` label if the node object was deleted during converge. [#1930](https://github.com/deckhouse/deckhouse/pull/1930)
 - **[dhctl]** Exclude password authentication check while connecting to host. [#1629](https://github.com/deckhouse/deckhouse/pull/1629)
 - **[extended-monitoring]** Fixed PVC usage alerts. [#1868](https://github.com/deckhouse/deckhouse/pull/1868)
 - **[helm_lib]** Tolerate evictions for cluster components on node scaling. [#1912](https://github.com/deckhouse/deckhouse/pull/1912)
    All controllers with the all-node toleration strategy (master node components, system daemonsets) will be restarted.
 - **[ingress-nginx]** Update the `D8IngressNginxControllerVersionDeprecated` alert description. [#2088](https://github.com/deckhouse/deckhouse/pull/2088)
 - **[ingress-nginx]** Upgrade 0.49 ingress controller to fix out-of-bounds temporary error [#1945](https://github.com/deckhouse/deckhouse/pull/1945)
    IngressNginxController of the version 0.49 will be restarted
 - **[ingress-nginx]** Fixed wildcard `vhost` label in `ingress-controller` metrics. [#1630](https://github.com/deckhouse/deckhouse/pull/1630)
    Ingress controller Pods will be restarted.
 - **[istio]** Fixed `D8IstioDataPlanePatchVersionMismatch` alert description. [#2048](https://github.com/deckhouse/deckhouse/pull/2048)
 - **[kube-dns]** Updated CoreDNS to v1.9.3. With patches to persuade coredns to respect deprecated Service annotation `service.alpha.kubernetes.io/tolerate-unready-endpoints`. Alerts the user to the need for migrating from deprecated annotation. [#1952](https://github.com/deckhouse/deckhouse/pull/1952)
 - **[linstor]** fix timestamp on linstor dashboard [#2250](https://github.com/deckhouse/deckhouse/pull/2250)
 - **[log-shipper]** Fix DaemonSet alerts. [#1912](https://github.com/deckhouse/deckhouse/pull/1912)
 - **[monitoring-kubernetes]** Added alert `NodeSUnreclaimBytesUsageHigh`. [#2154](https://github.com/deckhouse/deckhouse/pull/2154)
 - **[monitoring-kubernetes]** Ignore containers rootfs mount point for node-exporter in GKE. [#2100](https://github.com/deckhouse/deckhouse/pull/2100)
 - **[monitoring-kubernetes]** Fix eviction inodes imagefs and node fs if containerd and kubelet directory is a symlink. [#2061](https://github.com/deckhouse/deckhouse/pull/2061)
 - **[monitoring-kubernetes]** Fixed PVC usage Grafana dashboards. [#1868](https://github.com/deckhouse/deckhouse/pull/1868)
 - **[node-local-dns]** Correct check on coredns startup. [#2120](https://github.com/deckhouse/deckhouse/pull/2120)
    The `node-local-dns` will restart.
 - **[node-local-dns]** Revert service account to prevent Pod from getting stuck in a Terminating state. [#2111](https://github.com/deckhouse/deckhouse/pull/2111)
 - **[node-local-dns]** node-local-dns now works properly with cni-cilium. [#2037](https://github.com/deckhouse/deckhouse/pull/2037)
    node-local-dns Pods should restart.
 - **[node-local-dns]** Updated CoreDNS to v1.9.3. [#1952](https://github.com/deckhouse/deckhouse/pull/1952)
 - **[node-manager]** Remove bashible-apiserver deployment to avoid race condition [#2199](https://github.com/deckhouse/deckhouse/pull/2199)
 - **[node-manager]** Remove bashible-apiserver deployment to avoid race condition [#2191](https://github.com/deckhouse/deckhouse/pull/2191)
 - **[node-manager]** Remove race condition while updating `bashible-apiserver`. [#2185](https://github.com/deckhouse/deckhouse/pull/2185)
    `bashible-apiserver` will restart.
 - **[node-manager]** Fix unbound variable bootstrap_job_log_pid when bootstrap static-node [#1917](https://github.com/deckhouse/deckhouse/pull/1917)
 - **[node-manager]** Increased the `cluster-autoscaler` node cooldown after scaling-up to prevent flapping (10m instead of 2m) [#1746](https://github.com/deckhouse/deckhouse/pull/1746)
 - **[operator-prometheus]** Adjust scrape timeout using helm helper. [#2083](https://github.com/deckhouse/deckhouse/pull/2083)
 - **[prometheus]** Use `X-Auth-Token` header for remote write credentials according to the documentation. [#2205](https://github.com/deckhouse/deckhouse/pull/2205)
 - **[prometheus]** Rollback prometheus-module ServiceMonitor label selector to `app=prometheus`. [#2107](https://github.com/deckhouse/deckhouse/pull/2107)
 - **[prometheus]** Update Grafana to 8.5.9 to fix various CVE [#2039](https://github.com/deckhouse/deckhouse/pull/2039)
 - **[upmeter]** Fixed bug when cleaning old upmeter probe garbage resulting in errors stucks Deckhouse main queue [#2221](https://github.com/deckhouse/deckhouse/pull/2221)
 - **[upmeter]** Added the auto-clean of garbage `UpmeterHookProbe` object produced by a bug. [#2080](https://github.com/deckhouse/deckhouse/pull/2080)
 - **[upmeter]** Fix certificate name for DexAuthenticator. [#2060](https://github.com/deckhouse/deckhouse/pull/2060)
 - **[upmeter]** Add RBAC to watch nodes [#2051](https://github.com/deckhouse/deckhouse/pull/2051)
 - **[upmeter]** Fixed garbage collection for control-plane probes [#1943](https://github.com/deckhouse/deckhouse/pull/1943)
 - **[user-authn]** Refactor Dex probes, and collect metrics from Dex. [#1935](https://github.com/deckhouse/deckhouse/pull/1935)

## Chore


 - **[candi]** Update base images:
    - alpine: alpine:3.16.0
    - debian buster: debian:buster-20220527
    - debian bullseye: debian:bullseye-20220527
    - nginx: nginx:1.23.0-alpine
    - python: python:3.7.13-alpine3.16
    - shell-operator: flant/shell-operator:v1.0.10
    - ubuntu: ubuntu:bionic-20220531 [#1858](https://github.com/deckhouse/deckhouse/pull/1858)
    **All Deckhouse components will be restarted** including control-plane, ingress-nginx.
 - **[candi]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[cert-manager]** Bump version to 1.8.2 [#1946](https://github.com/deckhouse/deckhouse/pull/1946)
    The field spec.privateKey.rotationPolicy on Certificate resources is now validated. Valid options are Never and Always. If you are using a GitOps flow and one of your YAML manifests contains a Certificate with an invalid value, you will need to update it with a valid value to prevent your GitOps tool from failing on the new validation. 
    You can find certificates with an invalid rotationPolicy value with the next command: `kubectl get certificate -A -ojson | jq -r '.items[] | select(.spec.privateKey.rotationPolicy | strings | . != "Always" and . != "Never") | "\(.metadata.name) in namespace \(.metadata.namespace) has rotationPolicy=\(.spec.privateKey.rotationPolicy)"'`
 - **[control-plane-manager]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    All control plane Pods will be restarted.
 - **[deckhouse]** image-copier one-liner to detect if there are Pods with irrelevant registry. [#1959](https://github.com/deckhouse/deckhouse/pull/1959)
 - **[dhctl]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[docs]** Clarified docs for `rootDiskSize` in OpenStackInstanceClass [#2041](https://github.com/deckhouse/deckhouse/pull/2041)
 - **[ingress-nginx]** Mark Ingress controllers below 1.1 as deprecated. [#2004](https://github.com/deckhouse/deckhouse/pull/2004)
    Fire alerts about deprecated ingress controllers.
 - **[ingress-nginx]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    All Nginx Ingress Pods of controllers >=0.46 will be restarted.
 - **[istio]** Create an alert about irrelevant services with `type: ExternalName` and specified `.spec.ports field`. [#1954](https://github.com/deckhouse/deckhouse/pull/1954)
 - **[kube-dns]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    `kube-dns` Pods will be restarted.
 - **[kube-proxy]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    `kube-proxy` Pods will be restarted.
 - **[monitoring-kubernetes-control-plane]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[node-manager]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[prometheus]** Added names of malformed dashboard definitions to the log of `dashboard_provisioner` container. [#2085](https://github.com/deckhouse/deckhouse/pull/2085)
 - **[prometheus-metrics-adapter]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[registrypackages]** Added labels to registry package images. [#1847](https://github.com/deckhouse/deckhouse/pull/1847)
    Some of the registry packages will be reinstalled.
 - **[snapshot-controller]** fix typo in documentation [#1969](https://github.com/deckhouse/deckhouse/pull/1969)
    low
 - **[snapshot-controller]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[upmeter]** Fixed up garbage cleaning [#2090](https://github.com/deckhouse/deckhouse/pull/2090)
 - **[upmeter]** Added envs for a proxy to StatefulSet. [#2081](https://github.com/deckhouse/deckhouse/pull/2081)
    upmeter Pods will be restarted.
 - **[upmeter]** Added descriptions for avaiability groups "nodegroups" and "nginx". [#2013](https://github.com/deckhouse/deckhouse/pull/2013)
 - **[upmeter]** Tracking nodes switched to informer instead of direct listing requests. [#2007](https://github.com/deckhouse/deckhouse/pull/2007)
 - **[upmeter]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[upmeter]** Added alerts for garbage objects left by probes. [#1648](https://github.com/deckhouse/deckhouse/pull/1648)
 - **[vertical-pod-autoscaler]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)

