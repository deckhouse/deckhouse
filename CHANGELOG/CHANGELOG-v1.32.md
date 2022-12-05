# Changelog v1.32

## Know before update


 - Add alerts with the recommended course of action to monitor LINSTOR, Piraeus-operator, capacity of storage-pools and resources states
 - Added Grafana dashboard to monitor LINSTOR cluster and DRBD resources
 - Etcd will be restarted.
 - For clusters with automatic Kubernetes version selection, Kubernetes v1.21 becomes the default version.
    * The Kubernetes version update in such clusters will be done during the Deckhouse version update.
    * Updating the Kubernetes version will cause the restart of the cluster Control Plane components.
    * Run the following command to find out if the cluster has enabled automatic Kubernetes version selection: `kubectl -n kube-system get secret d8-cluster-configuration -o json | jq '.data."cluster-configuration.yaml"' -r | base64 -d | grep kubernetesVersion`. If the result is ‘kubernetesVersion: Automatic’ — the cluster has enabled automatic Kubernetes version selection.
 - Multimaster clusters will automatically turn LINSTOR into HA-mode
 - Now LVM pools can automatically be added to the LINSTOR cluster and StorageClasses generated
 - OpenVPN will be migrated from using PVC to store certificates to Kubernetes secrets. PVC will still remain in the cluster as a backup. If you don't need it, you should manually delete it from the cluster.
 - Restart etcd due to update to version 3.5.3.
 - The new module - ceph-csi. Manages the creation of Ceph volumes (RBD and CephFS) and attaches them to workloads.
 - The new module - snapshot-controller. Enables snapshot support for compatible CSI drivers and cloud providers.

## Features


 - **[candi]** Automatic update of Kubernetes version from 1.19 to 1.21. [#1288](https://github.com/deckhouse/deckhouse/pull/1288)
    For clusters with automatic Kubernetes version selection, Kubernetes v1.21 becomes the default version.
    * The Kubernetes version update in such clusters will be done during the Deckhouse version update.
    * Updating the Kubernetes version will cause the restart of the cluster Control Plane components.
    * Run the following command to find out if the cluster has enabled automatic Kubernetes version selection: `kubectl -n kube-system get secret d8-cluster-configuration -o json | jq '.data."cluster-configuration.yaml"' -r | base64 -d | grep kubernetesVersion`. If the result is ‘kubernetesVersion: Automatic’ — the cluster has enabled automatic Kubernetes version selection.
 - **[ceph-csi]** Added new module ceph-csi [#426](https://github.com/deckhouse/deckhouse/pull/426)
    The new module - ceph-csi. Manages the creation of Ceph volumes (RBD and CephFS) and attaches them to workloads.
 - **[control-plane-manager]** Update etcd to 3.5.3 [#1387](https://github.com/deckhouse/deckhouse/pull/1387)
    Restart etcd due to update to version 3.5.3.
 - **[docs]** Added Deckhouse documentation overview page. [#1467](https://github.com/deckhouse/deckhouse/pull/1467)
 - **[flant-integration]** Describes what information Deckhouse sends out and how it can be disabled. [#1429](https://github.com/deckhouse/deckhouse/pull/1429)
 - **[ingress-nginx]** Updated requirements for Ingress Nginx controller versions. If conditions are not met, then further Deckhouse upgrade is blocked [#1676](https://github.com/deckhouse/deckhouse/pull/1676)
 - **[ingress-nginx]** Add 1.1 IngressNginxController version which is "must have" for clusters with k8s version > 1.21 [#1209](https://github.com/deckhouse/deckhouse/pull/1209)
 - **[linstor]** Added more alerts for LINSTOR. [#1055](https://github.com/deckhouse/deckhouse/pull/1055)
 - **[linstor]** Grafana dashboard for LINSTOR [#1035](https://github.com/deckhouse/deckhouse/pull/1035)
    Added Grafana dashboard to monitor LINSTOR cluster and DRBD resources
 - **[linstor]** Alerts for LINSTOR [#1035](https://github.com/deckhouse/deckhouse/pull/1035)
    Add alerts with the recommended course of action to monitor LINSTOR, Piraeus-operator, capacity of storage-pools and resources states
 - **[linstor]** Autoimport LVM pools based on tags [#923](https://github.com/deckhouse/deckhouse/pull/923)
    Now LVM pools can automatically be added to the LINSTOR cluster and StorageClasses generated
 - **[log-shipper]** Various improvements to the log-shipper module:
    * Update vector to v0.20.0
    * Add the exclude namespaces option to the cluster logs config
    * Change default VPA mode to 'Initial'
    * NodeSelector and Tolerations options for the log-shipper agent pods
    * Rebalance connections among all Logstash instances
    * New dashboard for Grafana
    * Grouping log-shipper alerts
    * Troubleshooting guide [#1106](https://github.com/deckhouse/deckhouse/pull/1106)
 - **[prometheus]** Fixed retention calculation for localstorage.
    prometheus_disk hook rewritten in Go. [#813](https://github.com/deckhouse/deckhouse/pull/813)
 - **[snapshot-controller]** New module: snapshot-controller [#1068](https://github.com/deckhouse/deckhouse/pull/1068)
    The new module - snapshot-controller. Enables snapshot support for compatible CSI drivers and cloud providers.

## Fixes


 - **[candi]** Fix kubeadm registrypackages build. [#1580](https://github.com/deckhouse/deckhouse/pull/1580)
 - **[candi]** Fixed containerd registry package build for CentOS 8. [#1692](https://github.com/deckhouse/deckhouse/pull/1692)
 - **[candi]** Fixed kubernetes-cni install script for CentOS. [#1682](https://github.com/deckhouse/deckhouse/pull/1682)
 - **[candi]** Fix registry packages install scripts for CentOS. [#1621](https://github.com/deckhouse/deckhouse/pull/1621)
    Control-plane components restart on CentOS-based clusters.
 - **[candi]** Prepull the `kubernetes-api-proxy` image to avoid problems when we change from system to static pod `kubernetes-api-proxy`. [#1608](https://github.com/deckhouse/deckhouse/pull/1608)
 - **[candi]** Fix errors in withNAT layout [#1554](https://github.com/deckhouse/deckhouse/pull/1554)
 - **[candi]** Fixed race condition between old the kubernetes-api-proxy-configurator and bashible step. [#1482](https://github.com/deckhouse/deckhouse/pull/1482)
 - **[candi]** Fix startup config in Kubernetes API proxy configuration script. [#1426](https://github.com/deckhouse/deckhouse/pull/1426)
 - **[candi]** Added imagePullPolicy: IfNotPresent to kubernetes-api-proxy static pod. Fixed kubernetes-api-proxy run in docker envs. [#1297](https://github.com/deckhouse/deckhouse/pull/1297)
 - **[ceph-csi]** Fixed nodeSelector for csi-node pods in helm_lib [#1522](https://github.com/deckhouse/deckhouse/pull/1522)
 - **[ceph-csi]** Allow helm_lib_csi_node_manifests to be used for all cluster types for ceph-csi. [#1478](https://github.com/deckhouse/deckhouse/pull/1478)
 - **[chrony]** Remove chronyd stale pid file on start [#1375](https://github.com/deckhouse/deckhouse/pull/1375)
 - **[cloud-provider-aws]** Fixed terraform scheme. [#1710](https://github.com/deckhouse/deckhouse/pull/1710)
 - **[cloud-provider-aws]** Fix OpenAPI specifications. [#1449](https://github.com/deckhouse/deckhouse/pull/1449)
 - **[cloud-provider-aws]** The necessary IAM policies for creating a peering connection have been added to the documentation. [#504](https://github.com/deckhouse/deckhouse/pull/504)
 - **[cloud-provider-azure]** Fixed parameter name `type` -> `skuName`. [#1598](https://github.com/deckhouse/deckhouse/pull/1598)
 - **[cloud-provider-vsphere]** Fix OpenAPI specifications. [#1449](https://github.com/deckhouse/deckhouse/pull/1449)
 - **[cloud-provider-vsphere]** Correct behavior of nestedHardwareVirtualization parameter for VsphereInstanceClass. [#1331](https://github.com/deckhouse/deckhouse/pull/1331)
    Node groups with VsphereInstanceClass runtimeOptions.nestedHardwareVirtualization set to false have to be manually updated for this setting to take place. New nodes will be created with disabled nested hardware virtualization if it is disabled in configuration.
 - **[control-plane-manager]** Add the `--experimental-initial-corrupt-check` flag for etcd. [#1267](https://github.com/deckhouse/deckhouse/pull/1267)
    Etcd will be restarted.
 - **[deckhouse]** Fix kubernetes upgrades with feature gates and limits deckhouse modules revision up to 3 [#1377](https://github.com/deckhouse/deckhouse/pull/1377)
 - **[deckhouse]** Fixed a bug for the case when the storage class is set to "false" [#1364](https://github.com/deckhouse/deckhouse/pull/1364)
 - **[helm_lib]** Update CSI controller without creating a new one. [#1481](https://github.com/deckhouse/deckhouse/pull/1481)
 - **[ingress-nginx]** Updated requirements for Ingress Nginx controller versions. If conditions are not met, then further Deckhouse upgrade is blocked. [#1697](https://github.com/deckhouse/deckhouse/pull/1697)
 - **[linstor]** Fix drbd module building. [#1779](https://github.com/deckhouse/deckhouse/pull/1779)
 - **[linstor]** Add missing spatch dependency and disable SPAAS. [#1726](https://github.com/deckhouse/deckhouse/pull/1726)
 - **[linstor]** Refactored documentation. [#1677](https://github.com/deckhouse/deckhouse/pull/1677)
 - **[linstor]** automatically recover evicted nodes in LINSTOR [#1397](https://github.com/deckhouse/deckhouse/pull/1397)
 - **[linstor]** LINSTOR module now supports high-availability [#1147](https://github.com/deckhouse/deckhouse/pull/1147)
    Multimaster clusters will automatically turn LINSTOR into HA-mode
 - **[log-shipper]** Reduced the amount of exported metrics by log-shipper agents. Fixes metrics leak for dynamic environments. [#1588](https://github.com/deckhouse/deckhouse/pull/1588)
 - **[log-shipper]** Migrate deprecated Elasticsearch fields. [#1454](https://github.com/deckhouse/deckhouse/pull/1454)
 - **[monitoring-kubernetes]** Disabled node-exporter's systemd collector. It was not working correctly, so no one is dependent on it. [#1609](https://github.com/deckhouse/deckhouse/pull/1609)
 - **[namespace-configurator]** Exclude upmeter probe namespaces from namespace-configurator snapshots. [#1439](https://github.com/deckhouse/deckhouse/pull/1439)
 - **[node-local-dns]** Reworked health checking logic [#388](https://github.com/deckhouse/deckhouse/pull/388)
    Now Pods shouldn't crash unexpectedly now due to poor implementation of locking/probing.
 - **[node-manager]** Truncate event message to allowed maximums in update_node_group_status hook. [#1480](https://github.com/deckhouse/deckhouse/pull/1480)
 - **[openvpn]** Improved migration to secrets. [#1521](https://github.com/deckhouse/deckhouse/pull/1521)
 - **[openvpn]** Removing of openvpn.storageClass parameter from deckhouse configmap [#1493](https://github.com/deckhouse/deckhouse/pull/1493)
 - **[openvpn]** Fixed Values references in ConfigMap. [#1463](https://github.com/deckhouse/deckhouse/pull/1463)
 - **[openvpn]** Fixed "loadBalancer" OpenAPI spec. [#1417](https://github.com/deckhouse/deckhouse/pull/1417)
 - **[openvpn]** Fixed statefulSet apiVersion in a migration hook. [#1354](https://github.com/deckhouse/deckhouse/pull/1354)
 - **[openvpn]** Set default value for loadbalancer object in the OpenAPI schema. [#1353](https://github.com/deckhouse/deckhouse/pull/1353)
 - **[openvpn]** Add forgotten param effectiveStorageClass to openapi spec [#1344](https://github.com/deckhouse/deckhouse/pull/1344)
 - **[openvpn]** Fixed OpenAPI [#1307](https://github.com/deckhouse/deckhouse/pull/1307)
 - **[openvpn]** Web interface changed to https://github.com/flant/ovpn-admin. Persistent storage has been replaced with Kubernetes secrets. Added HostPort inlet. [#522](https://github.com/deckhouse/deckhouse/pull/522)
    OpenVPN will be migrated from using PVC to store certificates to Kubernetes secrets. PVC will still remain in the cluster as a backup. If you don't need it, you should manually delete it from the cluster.
 - **[prometheus]** Set disk retention size to 80%. [#1721](https://github.com/deckhouse/deckhouse/pull/1721)
 - **[prometheus]** Fixed PersistentVolumeClaim size calculation for local storage. [#1437](https://github.com/deckhouse/deckhouse/pull/1437)
 - **[prometheus]** Fix null pointer dereference in prometheus_disk.go hook [#1345](https://github.com/deckhouse/deckhouse/pull/1345)
 - **[prometheus]** Set Grafana sample limit to 5000 [#1215](https://github.com/deckhouse/deckhouse/pull/1215)
 - **[upmeter]** Fix non-working upmeter server on emptyDirs [#1524](https://github.com/deckhouse/deckhouse/pull/1524)
 - **[upmeter]** Upmeter no longer exposes DNS queries to the Internet [#1256](https://github.com/deckhouse/deckhouse/pull/1256)
 - **[upmeter]** Fixed the calculation of groups uptime [#1144](https://github.com/deckhouse/deckhouse/pull/1144)

## Chore


 - **[cert-manager]** Bump to version 1.7.1. Fix a possible bug with ACME solvers when you don't have a default ingress class like nginx (a very rare case). Minor bug fixes [#1082](https://github.com/deckhouse/deckhouse/pull/1082)
 - **[cloud-provider-aws]** The `terraform-provider-aws` was updated to version `4.16`. [#1656](https://github.com/deckhouse/deckhouse/pull/1656)
 - **[istio]** Documentation refactoring. [#1281](https://github.com/deckhouse/deckhouse/pull/1281)
 - **[upmeter]** Remove redundant smoke-mini Ingress [#1237](https://github.com/deckhouse/deckhouse/pull/1237)
 - **[upmeter]** Add User-Agent header to all requests [#1213](https://github.com/deckhouse/deckhouse/pull/1213)

