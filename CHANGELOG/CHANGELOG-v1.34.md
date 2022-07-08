# Changelog v1.34

## Know before update


 - **All Deckhouse components will be restarted** including control-plane, ingress-nginx.
 - Modified existing alerts:
    * Removed predefined groups in Polk.
    * Added group auto-creation in Polk.
    * Added the `for` parameter for all alerts.
    * Removed the `plk_pending_until_firing_for` annotation from all alerts. LGTM as far as can evaluate alerts.
 - The `ru-central1-c` **Yandex.cloud** zone was [deprecated](https://cloud.yandex.com/en/docs/overview/concepts/ru-central1-c-deprecation).
    For new clusters NAT-instance will be created in `ru-central1-a` zone. For old instances you should add to `withNATInstance.natInstanceInternalAddress` (you can get address from Yandex.Cloud console) 
    and `withNATInstance.internalSubnetID` (you can get address using command `kubectl -n d8-system exec -it deploy/deckhouse -- deckhouse-controller module values cloud-provider-yandex -o json | jq -r '.cloudProviderYandex.internal.providerDiscoveryData.zoneToSubnetIdMap["ru-central1-c"]'`) to prevent NAT-instance recreation during a converge process.

## Features


 - **[candi]** New Kuberenetes patch versions. [#1724](https://github.com/deckhouse/deckhouse/pull/1724)
    Restart of Kubernetes control plane components.
 - **[cert-manager]** Removed the `plk_pending_until_firing_for` annotation from all alerts. [#1446](https://github.com/deckhouse/deckhouse/pull/1446)
 - **[cloud-provider-azure]** Provide new default StorageClasses for disks large than 4 TiB. [#1652](https://github.com/deckhouse/deckhouse/pull/1652)
 - **[cloud-provider-yandex]** Validate `serviceAccountJSON`. [#1904](https://github.com/deckhouse/deckhouse/pull/1904)
 - **[cloud-provider-yandex]** Move NAT-instance to `ru-central1-a` for new instances. [#1592](https://github.com/deckhouse/deckhouse/pull/1592)
    The `ru-central1-c` **Yandex.cloud** zone was [deprecated](https://cloud.yandex.com/en/docs/overview/concepts/ru-central1-c-deprecation).
    For new clusters NAT-instance will be created in `ru-central1-a` zone. For old instances you should add to `withNATInstance.natInstanceInternalAddress` (you can get address from Yandex.Cloud console) 
    and `withNATInstance.internalSubnetID` (you can get address using command `kubectl -n d8-system exec -it deploy/deckhouse -- deckhouse-controller module values cloud-provider-yandex -o json | jq -r '.cloudProviderYandex.internal.providerDiscoveryData.zoneToSubnetIdMap["ru-central1-c"]'`) to prevent NAT-instance recreation during a converge process.
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


 - **[candi]** Fixed the applying of disk size for CloudPermanent nodes in `YandexClusterConfiguration`. [#1900](https://github.com/deckhouse/deckhouse/pull/1900)
 - **[cert-manager]** Fix patch for cert-manager certificate owner ref field [#1985](https://github.com/deckhouse/deckhouse/pull/1985)
 - **[cert-manager]** Respect the global IngressClass in the `letsencrypt` ClusterIssuer. [#1750](https://github.com/deckhouse/deckhouse/pull/1750)
 - **[deckhouse]** Change DeckhouseUpdating Prometheus rule severity_level to avoid alert deferring [#1929](https://github.com/deckhouse/deckhouse/pull/1929)
 - **[dhctl]** Do not try to remove the `dhctl.deckhouse.io/node-for-converge` label if the node object was deleted during converge. [#1930](https://github.com/deckhouse/deckhouse/pull/1930)
 - **[dhctl]** Exclude password authentication check while connecting to host. [#1629](https://github.com/deckhouse/deckhouse/pull/1629)
 - **[extended-monitoring]** Fixed PVC usage alerts. [#1868](https://github.com/deckhouse/deckhouse/pull/1868)
 - **[helm_lib]** Tolerate evictions for cluster components on node scaling. [#1912](https://github.com/deckhouse/deckhouse/pull/1912)
    All controllers with the all-node toleration strategy (master node components, system daemonsets) will be restarted.
 - **[ingress-nginx]** Upgrade 0.49 ingress controller to fix out-of-bounds temporary error [#1945](https://github.com/deckhouse/deckhouse/pull/1945)
    IngressNginxController of the version 0.49 will be restarted
 - **[ingress-nginx]** Fixed wildcard `vhost` label in `ingress-controller` metrics. [#1630](https://github.com/deckhouse/deckhouse/pull/1630)
    Ingress controller Pods will be restarted.
 - **[kube-dns]** Updated CoreDNS to v1.9.3. With patches to persuade coredns to respect deprecated Service annotation `service.alpha.kubernetes.io/tolerate-unready-endpoints`. Alerts the user to the need for migrating from deprecated annotation. [#1952](https://github.com/deckhouse/deckhouse/pull/1952)
 - **[log-shipper]** Fix DaemonSet alerts. [#1912](https://github.com/deckhouse/deckhouse/pull/1912)
 - **[monitoring-kubernetes]** Fixed PVC usage Grafana dashboards. [#1868](https://github.com/deckhouse/deckhouse/pull/1868)
 - **[node-local-dns]** Updated CoreDNS to v1.9.3. [#1952](https://github.com/deckhouse/deckhouse/pull/1952)
 - **[node-manager]** Fix unbound variable bootstrap_job_log_pid when bootstrap static-node [#1917](https://github.com/deckhouse/deckhouse/pull/1917)
 - **[node-manager]** Increased the `cluster-autoscaler` node cooldown after scaling-up to prevent flapping (10m instead of 2m) [#1746](https://github.com/deckhouse/deckhouse/pull/1746)
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
 - **[ingress-nginx]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    All Nginx Ingress Pods of controllers >=0.46 will be restarted.
 - **[istio]** Create an alert about irrelevant services with `type: ExternalName` and specified `.spec.ports field`. [#1954](https://github.com/deckhouse/deckhouse/pull/1954)
 - **[kube-dns]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    `kube-dns` Pods will be restarted.
 - **[kube-proxy]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
    `kube-proxy` Pods will be restarted.
 - **[monitoring-kubernetes-control-plane]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[node-manager]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[prometheus-metrics-adapter]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[registrypackages]** Added labels to registry package images. [#1847](https://github.com/deckhouse/deckhouse/pull/1847)
    Some of the registry packages will be reinstalled.
 - **[snapshot-controller]** fix typo in documentation [#1969](https://github.com/deckhouse/deckhouse/pull/1969)
    low
 - **[snapshot-controller]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[upmeter]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)
 - **[upmeter]** Added alerts for garbage objects left by probes. [#1648](https://github.com/deckhouse/deckhouse/pull/1648)
 - **[vertical-pod-autoscaler]** Switched to the new official Kubernetes registry. [#1717](https://github.com/deckhouse/deckhouse/pull/1717)

