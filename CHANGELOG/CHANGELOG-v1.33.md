# Changelog v1.33

## Know before update


 - Components of the `kube-dns` module will be restarted.
 - Control plane components will be restarted.
 - Etcd will be restarted. The `quota-backend-bytes` parameter added to etcd is calculated depending on control-plane memory capacity.
 - Ingress controllers version 0.33+ will be restarted.
 - IngressNginx controllers 0.25 and 0.26 are removed. Ingress controller version 1.1 will restart.
 - Openvpn and admin panel will be restarted.
 - Prometheus will be restarted.

## Features


 - **[candi]** Upgrade Yandex Cloud terraform provider to 0.74.0 [#1649](https://github.com/deckhouse/deckhouse/pull/1649)
 - **[candi]** Added support for Ubuntu 22.04 LTS. [#1505](https://github.com/deckhouse/deckhouse/pull/1505)
 - **[candi]** Bumped containerd to v1.5.11. [#1386](https://github.com/deckhouse/deckhouse/pull/1386)
 - **[candi]** Added support for Kubernetes 1.23. [#1290](https://github.com/deckhouse/deckhouse/pull/1290)
 - **[candi]** Improved `candi` bundle detection to detect CentOS-based distros. [#1173](https://github.com/deckhouse/deckhouse/pull/1173)
 - **[cert-manager]** Added support of certificate owner ref on certificate level [#1601](https://github.com/deckhouse/deckhouse/pull/1601)
 - **[cert-manager]** Added Cloudflare's APIToken support for ClusterIssuer. [#1528](https://github.com/deckhouse/deckhouse/pull/1528)
 - **[cloud-provider-aws]** Added the ability to configure peering connections to the without-nat and standard layouts. [#514](https://github.com/deckhouse/deckhouse/pull/514)
 - **[cloud-provider-azure]** Enabled accelerated networking for new `machine-controller-manager` instances. [#1266](https://github.com/deckhouse/deckhouse/pull/1266)
 - **[cloud-provider-yandex]** Changed default platform to `standard-v3` for new instances created by `machine-controller-manager`. [#1361](https://github.com/deckhouse/deckhouse/pull/1361)
 - **[cni-cilium]** 1. Updated Cilium to v1.11.5
    2. Cilium will no longer terminate host network connections abruptly when Host Policies are in effect: https://github.com/cilium/cilium/issues/19367 [#1620](https://github.com/deckhouse/deckhouse/pull/1620)
 - **[cni-cilium]** The new module responsible for providing a network between multiple nodes in a cluster using the [cilium](https://cilium.io/). [#592](https://github.com/deckhouse/deckhouse/pull/592)
    Without a way to migrate from existing CNIs at this moment.
 - **[cni-flannel]** Bumped flannel to 0.15.1. [#1173](https://github.com/deckhouse/deckhouse/pull/1173)
 - **[control-plane-manager]** Bolt-on `healthCheckNodePort` of the control-plane Service and `trafficPolicy: Local`. [#1839](https://github.com/deckhouse/deckhouse/pull/1839)
 - **[control-plane-manager]** Added `authentication-token-webhook-cache-ttl` parameter to apiserver. [#1791](https://github.com/deckhouse/deckhouse/pull/1791)
 - **[control-plane-manager]** Calculate and add the `quota-backend-bytes` parameter to etcd. [#1389](https://github.com/deckhouse/deckhouse/pull/1389)
    Etcd will be restarted. The `quota-backend-bytes` parameter added to etcd is calculated depending on control-plane memory capacity.
 - **[deckhouse]** Automatically apply the first release on Deckhouse bootstrap. [#1851](https://github.com/deckhouse/deckhouse/pull/1851)
 - **[deckhouse-controller]** Added the `edit` command for the `deckhouse-controller` to be able to modify cluster configuration files. [#1558](https://github.com/deckhouse/deckhouse/pull/1558)
 - **[dhctl]** Prevent to break already bootstrapped cluster when bootstrap new cluster [#1811](https://github.com/deckhouse/deckhouse/pull/1811)
 - **[dhctl]** For new Deckhouse installations images for control-plane (image for pause container, for example) will be used from the Deckhouse registry. [#1517](https://github.com/deckhouse/deckhouse/pull/1517)
 - **[extended-monitoring]** List objects from the kube-apiserver cache, avoid hitting etcd on each list. It should decrease control plane resource consumption. [#1535](https://github.com/deckhouse/deckhouse/pull/1535)
 - **[extended-monitoring]** The module is available in the Deckhouse Community Edition and enabled by default. [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[helm]** Added deprecated APIs alerts for k8s 1.22 and 1.25 [#1461](https://github.com/deckhouse/deckhouse/pull/1461)
 - **[istio]** Upgraded to 1.12 or 1.13 and new version control method. [#1431](https://github.com/deckhouse/deckhouse/pull/1431)
 - **[keepalived]** The module is available in the Deckhouse Enterprise Edition [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[log-shipper]** Label filter support for log-shipper. Users will be able to filter log messages based on their metadata labels. [#1424](https://github.com/deckhouse/deckhouse/pull/1424)
 - **[namespace-configurator]** The module is available in the Deckhouse Community Edition and enabled by default. [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[network-gateway]** The module is available in the Deckhouse Enterprise Edition [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[okmeter]** The module is available in the Deckhouse Community Edition but requires Okmeter license. [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[openvpn]** Hooks are rewritten in Go. [#1489](https://github.com/deckhouse/deckhouse/pull/1489)
 - **[openvpn]** The module is available in the Deckhouse Community Edition (is in experimental state). [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[openvpn]** Added support for UDP protocol. [#1432](https://github.com/deckhouse/deckhouse/pull/1432)
 - **[prometheus]** Added token auth for Prometheus remote write. [#1586](https://github.com/deckhouse/deckhouse/pull/1586)
 - **[prometheus]** Prefer to schedule the main and the long-term Prometheus on different nodes. [#1551](https://github.com/deckhouse/deckhouse/pull/1551)
    The Prometheus main and the Prometheus long-term will restart.
 - **[prometheus]** Grafana 8.5.2 [#1536](https://github.com/deckhouse/deckhouse/pull/1536)
 - **[prometheus]** Create table with enabled Deckhouse web interfaces on the Grafana home page [#1415](https://github.com/deckhouse/deckhouse/pull/1415)
 - **[secret-copier]** The module is available in the Deckhouse Community Edition and enabled by default. [#1488](https://github.com/deckhouse/deckhouse/pull/1488)
 - **[terraform-manager]** Upgrade Yandex Cloud terraform provider to 0.74.0 [#1649](https://github.com/deckhouse/deckhouse/pull/1649)
 - **[upmeter]** Added probe for Grafana Pods. [#1658](https://github.com/deckhouse/deckhouse/pull/1658)
 - **[upmeter]** Added probe for OpenVPN Pods. [#1658](https://github.com/deckhouse/deckhouse/pull/1658)
 - **[upmeter]** Added probe for Longterm Prometheus Pods and basic API response. [#1658](https://github.com/deckhouse/deckhouse/pull/1658)
 - **[upmeter]** Added probe for Kubernetes Dashboard Pods. [#1658](https://github.com/deckhouse/deckhouse/pull/1658)
 - **[upmeter]** Added probe for Dex Pods and basic API response. [#1658](https://github.com/deckhouse/deckhouse/pull/1658)
 - **[upmeter]** Added kubelet metrics check to the probe "monitoring-and-autoscaling/key-metrics-presence". [#1658](https://github.com/deckhouse/deckhouse/pull/1658)
 - **[user-authn]** Use Gitlab refresh token, call refresh method of any connector only once. [#995](https://github.com/deckhouse/deckhouse/pull/995)

## Fixes


 - **[candi]** Enabled the `GracefulNodeShutdown` flag on K8s 1.20 to fix kubelet not starting. [#1777](https://github.com/deckhouse/deckhouse/pull/1777)
 - **[candi]** Fix build of the AWS cloud controller manager [#1716](https://github.com/deckhouse/deckhouse/pull/1716)
 - **[candi]** Remove master node `coreFraction` setting from YandexClusterConfiguration openapi spec [#1617](https://github.com/deckhouse/deckhouse/pull/1617)
 - **[candi]** Migrate to cgroupfs on containerd installations. [#1386](https://github.com/deckhouse/deckhouse/pull/1386)
 - **[ceph-csi]** Fixed missing registry secret. [#1733](https://github.com/deckhouse/deckhouse/pull/1733)
 - **[chrony]** Refactored alerts. [#1903](https://github.com/deckhouse/deckhouse/pull/1903)
 - **[chrony]** Chrony systemd unit on a node is added to stop list. [#1776](https://github.com/deckhouse/deckhouse/pull/1776)
 - **[cilium-hubble]** Copy custom certificate into d8-cni-cilium if it is used. [#1879](https://github.com/deckhouse/deckhouse/pull/1879)
 - **[cloud-provider-aws]** Fix LoadBalancer type none target group creation. [#1741](https://github.com/deckhouse/deckhouse/pull/1741)
 - **[cloud-provider-openstack]** Support for volume type with spaces in the name. [#1872](https://github.com/deckhouse/deckhouse/pull/1872)
 - **[cloud-provider-vsphere]** Fix error in Terraform for static nodes in setups without nested resource pools. [#1785](https://github.com/deckhouse/deckhouse/pull/1785)
 - **[cloud-provider-yandex]** Revert checksum calculation for `platformID`. [#1846](https://github.com/deckhouse/deckhouse/pull/1846)
 - **[cloud-provider-yandex]** Rollback changes to set `simple-bridge`  as default CNI for Yandex. [#1582](https://github.com/deckhouse/deckhouse/pull/1582)
 - **[cni-cilium]** The `enable_node_routes` hook now bails if a value is present in config. [#1792](https://github.com/deckhouse/deckhouse/pull/1792)
 - **[deckhouse-controller]** Restore exponential backoff for delays between failed hooks restarts. [#1790](https://github.com/deckhouse/deckhouse/pull/1790)
 - **[helm]** Fix namespace detection for helm releases with unsupported/deprecated resources. [#1882](https://github.com/deckhouse/deckhouse/pull/1882)
 - **[helm]** Avoid failing on incorrect helm releases. [#1754](https://github.com/deckhouse/deckhouse/pull/1754)
 - **[helm]** Avoid hook failure on errors [#1523](https://github.com/deckhouse/deckhouse/pull/1523)
 - **[ingress-nginx]** Handle IngressClass resources when migrating from controller < 1.0 version to > 1.0 version [#1892](https://github.com/deckhouse/deckhouse/pull/1892)
 - **[ingress-nginx]** Fix HPA calculating for `IngressNginxController`. Calculate average CPU load instead of summary. [#1889](https://github.com/deckhouse/deckhouse/pull/1889)
 - **[ingress-nginx]** Fix build of the ingress-nginx 0.33 controller. [#1757](https://github.com/deckhouse/deckhouse/pull/1757)
 - **[ingress-nginx]** Fix workability of 0.33 controller with IngressClass resource. [#1753](https://github.com/deckhouse/deckhouse/pull/1753)
 - **[ingress-nginx]** move to ingressClassName spec [#1671](https://github.com/deckhouse/deckhouse/pull/1671)
    IngressNginx controllers 0.25 and 0.26 are removed. Ingress controller version 1.1 will restart.
 - **[ingress-nginx]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Ingress controllers version 0.33+ will be restarted.
 - **[istio]** Remove outdated kiali's MonitoringDashboard resources. [#1956](https://github.com/deckhouse/deckhouse/pull/1956)
 - **[istio]** Update kiali to 1.49. [#1942](https://github.com/deckhouse/deckhouse/pull/1942)
 - **[istio]** Data-plane metrics scrape fix for fresh istio versions. [#1907](https://github.com/deckhouse/deckhouse/pull/1907)
 - **[istio]** Istio `globalVersion` detection fix. [#1769](https://github.com/deckhouse/deckhouse/pull/1769)
 - **[kube-dns]** Updated CoreDNS to v1.9.1 [#1537](https://github.com/deckhouse/deckhouse/pull/1537)
 - **[kube-dns]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Components of the `kube-dns` module will be restarted.
 - **[linstor]** Make the `linstor-controller` listen on IPv4-only address. [#1830](https://github.com/deckhouse/deckhouse/pull/1830)
 - **[linstor]** Fix monitoring of the `piraeus-operator`. [#1818](https://github.com/deckhouse/deckhouse/pull/1818)
 - **[linstor]** LINSTOR updated to 1.18.1, DRBD module to 9.1.7, linstor-csi to 0.19.0, linstor-scheduler to v0.3.0 [#1559](https://github.com/deckhouse/deckhouse/pull/1559)
 - **[log-shipper]** Fix integration of the File source with the Elasticsearch destination. [#1625](https://github.com/deckhouse/deckhouse/pull/1625)
 - **[log-shipper]** Provide structural schemas for log-shipper CRDs [#1612](https://github.com/deckhouse/deckhouse/pull/1612)
 - **[log-shipper]** Add the `rateLimit` option to the `ClusterLogsDestination` CRD. [#1498](https://github.com/deckhouse/deckhouse/pull/1498)
 - **[log-shipper]** Migrate deprecated elasticsearch fields [#1453](https://github.com/deckhouse/deckhouse/pull/1453)
 - **[log-shipper]** Send reloading signal to all vector processes in a container on config change. [#1430](https://github.com/deckhouse/deckhouse/pull/1430)
 - **[monitoring-kubernetes]** Fixed handling absent containerd directory· [#1894](https://github.com/deckhouse/deckhouse/pull/1894)
 - **[monitoring-kubernetes]** Make the `kubelet-eviction-threshold-exporter` workable in managed Kubernetes platforms (AWS, GKE, Yandex, etc). [#1866](https://github.com/deckhouse/deckhouse/pull/1866)
 - **[monitoring-kubernetes]** Fixes alert `UnsupportedContainerRuntimeVersion` to support the newest versions of containerd - 1.5.* and 1.6.* and docker 20.*. [#1506](https://github.com/deckhouse/deckhouse/pull/1506)
 - **[monitoring-kubernetes]** Fix kubelet alerts [#1471](https://github.com/deckhouse/deckhouse/pull/1471)
 - **[monitoring-kubernetes]** 1. Detect proper version of a ebpf program to run on a given kernel.
    2. If a program fails to compile or attach to the kernel tracing facilities, do not crash the ebpf_exporter. [#1120](https://github.com/deckhouse/deckhouse/pull/1120)
 - **[node-local-dns]** Changed priority-class to `cluster-medium`. [#1747](https://github.com/deckhouse/deckhouse/pull/1747)
 - **[node-local-dns]** Updated CoreDNS to v1.9.1 [#1537](https://github.com/deckhouse/deckhouse/pull/1537)
 - **[openvpn]** Fixes a bug that does not allow you to set additional domains. [#1764](https://github.com/deckhouse/deckhouse/pull/1764)
 - **[openvpn]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Openvpn and admin panel will be restarted.
 - **[prometheus]** Enable legacy alerting system for Grafana [#1795](https://github.com/deckhouse/deckhouse/pull/1795)
 - **[prometheus]** Use new metrics names in alert rules. [#1627](https://github.com/deckhouse/deckhouse/pull/1627)
 - **[prometheus]** Removed the old prometheus_storage_class_change shell hook which has already been replaced by Go hooks. [#1396](https://github.com/deckhouse/deckhouse/pull/1396)
 - **[prometheus]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Prometheus will be restarted.
 - **[snapshot-controller]** Reduce the number of replicas for snapshot-controller to a maximum of 2. [#1864](https://github.com/deckhouse/deckhouse/pull/1864)
 - **[snapshot-controller]** The `snapshot-controller` does not allow install CRD until `v1alpha1` is preserved in cluster. [#1808](https://github.com/deckhouse/deckhouse/pull/1808)
 - **[snapshot-controller]** snapshot-controller does not allow install crd until `v1alpha1` is preserved in cluster. [#1802](https://github.com/deckhouse/deckhouse/pull/1802)
 - **[upmeter]** Fixed potential error loops in remote write exporter [#1579](https://github.com/deckhouse/deckhouse/pull/1579)
    If a storage responds with 4xx error, the unaccepted metrics will not be re-sent.
 - **[upmeter]** Added missing User-Agent header to remote write exporter, defined as `Upmeter/1.0 (Deckhouse <edition> <version>)` [#1579](https://github.com/deckhouse/deckhouse/pull/1579)
 - **[upmeter]** Fix the correctness of neighbor-via-service probe by using ClusterIP service type. [#1549](https://github.com/deckhouse/deckhouse/pull/1549)
 - **[upmeter]** UI shows only present data [#1405](https://github.com/deckhouse/deckhouse/pull/1405)
 - **[upmeter]** Use finite timeout in agent insecure HTTP client [#1334](https://github.com/deckhouse/deckhouse/pull/1334)
 - **[upmeter]** Fixed slow data loading in [#1257](https://github.com/deckhouse/deckhouse/pull/1257)
 - **[user-authn]** Change dex-authenticator's port name from `http` to `https` [#1566](https://github.com/deckhouse/deckhouse/pull/1566)

## Chore


 - **[cert-manager]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Components of the `cert-manager` module will be restarted.
 - **[chrony]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `chrony` module will be restarted.
 - **[cilium-hubble]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Cilium Hubble will be restarted.
 - **[cloud-provider-aws]** Restarting the csi-controller after cloud config changes. [#1571](https://github.com/deckhouse/deckhouse/pull/1571)
    The csi-controller will restart.
 - **[cloud-provider-aws]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cloud-provider-aws` module will be restarted.
 - **[cloud-provider-azure]** Restarting the csi-controller after cloud config changes. [#1571](https://github.com/deckhouse/deckhouse/pull/1571)
    The csi-controller will restart.
 - **[cloud-provider-azure]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cloud-provider-azure` module will be restarted.
 - **[cloud-provider-gcp]** Restarting csi-controller after cloud config changes. [#1571](https://github.com/deckhouse/deckhouse/pull/1571)
    All csi-controllers in all cloud-powered clusters will restart.
 - **[cloud-provider-gcp]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cloud-provider-gcp` module will be restarted.
 - **[cloud-provider-openstack]** Restarting the csi-controller after cloud config changes. [#1571](https://github.com/deckhouse/deckhouse/pull/1571)
    The csi-controller will restart.
 - **[cloud-provider-vsphere]** Restarting the csi-controller after cloud config changes. [#1571](https://github.com/deckhouse/deckhouse/pull/1571)
    The csi-controller will restart.
 - **[cloud-provider-vsphere]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cloud-provider-vsphere` module will be restarted.
 - **[cloud-provider-yandex]** Restarting the csi-controller after cloud config changes. [#1571](https://github.com/deckhouse/deckhouse/pull/1571)
    The csi-controller will restart.
 - **[cloud-provider-yandex]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cloud-provider-yandex` module components will be restarted.
 - **[cni-cilium]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `operator` of the `cni-cilium` module will be restarted.
 - **[cni-flannel]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cni-flannel` module will be restarted.
 - **[cni-simple-bridge]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `cni-simple-bridge` module will be restarted.
 - **[control-plane-manager]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Control plane components will be restarted.
 - **[dashboard]** Dashboard upgrade from 2.2.0 to 2.5.1 [#1383](https://github.com/deckhouse/deckhouse/pull/1383)
 - **[dashboard]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Kubernetes Dashboard will be restarted.
 - **[deckhouse]** Updated `BASE_ALPINE` and `BASE_ALPINE_3_15` variables to change versions of the Alpine Linux in base images from 3.12.1 to 3.12.12, and from 3.15 to 3.15.4. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
 - **[deckhouse-controller]** Stop excess log messages about WaitForSynchronization tasks [#1763](https://github.com/deckhouse/deckhouse/pull/1763)
 - **[descheduler]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `descheduler` module will be restarted.
 - **[docs]** Suggest gp3 for bastion instance in AWS-based 'Getting started' [#1495](https://github.com/deckhouse/deckhouse/pull/1495)
 - **[extended-monitoring]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `extended-monitoring` module will be restarted.
 - **[flant-integration]** Removed unused "flantIntegration.team" field from values schema [#1514](https://github.com/deckhouse/deckhouse/pull/1514)
 - **[ingress-nginx]** Bump GoGo dependency for the protobuf-exporter to prevent improper input. [#1519](https://github.com/deckhouse/deckhouse/pull/1519)
 - **[istio]** Anti-affinity for istiod Pods in HA installations. `proxyConfig.holdApplicationUntilProxyStarts` global flag to guarantee that istio sidecar starts before application container. `enableHTTP10` flag to allow HTTP/1.0 requests handling. [#1665](https://github.com/deckhouse/deckhouse/pull/1665)
 - **[istio]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `istio` module will be restarted.
 - **[kube-proxy]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12 and from 3.15 to 3.15.4. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `kube-proxy` module will be restarted.
 - **[local-path-provisioner]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `local-path-provisioner` module will be restarted.
 - **[metallb]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `metallb` module will be restarted.
 - **[monitoring-kubernetes]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Components of the `monitoring-kubernetes` module will be restarted.
 - **[network-policy-engine]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `kube-router` of the `network-policy-engine` will be restarted.
 - **[node-local-dns]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `node-local-dns` module will be restarted.
 - **[node-manager]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `bashible-apiserver`, `cluster-autoscaler` and `machine-controller-manager` components of the `node-manager` module will be restarted.
 - **[operator-prometheus]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `operator-prometheus` module will be restarted.
 - **[pod-reloader]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `pod-reloader` module will be restarted.
 - **[prometheus]** Added hack for fix lens Cluster and Nodes metrics showing. [#1797](https://github.com/deckhouse/deckhouse/pull/1797)
 - **[prometheus-metrics-adapter]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `prometheus-metrics-adapter` module will be restarted.
 - **[prometheus-pushgateway]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `prometheus-pushgateway` module will be restarted.
 - **[registrypackages]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
 - **[snapshot-controller]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `snapshot-controller` module will be restarted.
 - **[terraform-manager]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `terraform-manager` module will be restarted.
 - **[upmeter]** Turned off smoke-mini storage classes, so it will not order PVs by default and only serve us network checks. [#1635](https://github.com/deckhouse/deckhouse/pull/1635)
 - **[upmeter]** Revert the deletion of HTTP handlers for e2e tests [#1600](https://github.com/deckhouse/deckhouse/pull/1600)
 - **[upmeter]** Switched the logging of upmeter metrics on info level while exporting. [#1579](https://github.com/deckhouse/deckhouse/pull/1579)
 - **[upmeter]** Renamed current state of groups and probes according to SLA in the Terms of Service [#1534](https://github.com/deckhouse/deckhouse/pull/1534)
 - **[upmeter]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `upmeter` module will be restarted.
 - **[user-authn]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    Components of the `user-authn` module will be restarted. All dex-authenticator Pods will be restarted.
 - **[vertical-pod-autoscaler]** The Alpine Linux version in the base image has been bumped from 3.12.1 to 3.12.12. [#1374](https://github.com/deckhouse/deckhouse/pull/1374)
    The `vertical-pod-autoscaler` module will be restarted.

