# Changelog v1.32

## Know before update


 - Add alerts with the recommended course of action to monitor LINSTOR, Piraeus-operator, capacity of storage-pools and resources states
 - Added Grafana dashboard to monitor LINSTOR cluster and DRBD resources
 - By default, all Kubernetes v1.19 clusters will update to v1.21
 - Etcd will be restarted.
 - Multimaster clusters will automatically turn LINSTOR into HA-mode
 - Now LVM pools will automatically be added to the LINSTOR cluster and StorageClasses generated
 - OpenVPN will be migrated from using PVC to store certificates to Kubernetes secrets. PVC will still remain in the cluster as a backup. If you don't need it, you should manually delete it from the cluster.
 - This module enables snapshots functionality in Kubernetes cluster

## Features


 - **[candi]** Automatic update of Kubernetes version from 1.19 to 1.21. [#1288](https://github.com/deckhouse/deckhouse/pull/1288)
    By default, all Kubernetes v1.19 clusters will update to v1.21
 - **[ceph-csi]** Added new module ceph-csi [#426](https://github.com/deckhouse/deckhouse/pull/426)
    CephCSI allows dynamically provisioning Ceph volumes (RBD and CephFS) and attaching them to workloads.
 - **[ingress-nginx]** Add 1.1 IngressNginxController version which is "must have" for clusters with k8s version > 1.21 [#1209](https://github.com/deckhouse/deckhouse/pull/1209)
 - **[linstor]** Added more alerts for LINSTOR. [#1055](https://github.com/deckhouse/deckhouse/pull/1055)
 - **[linstor]** Grafana dashboard for LINSTOR [#1035](https://github.com/deckhouse/deckhouse/pull/1035)
    Added Grafana dashboard to monitor LINSTOR cluster and DRBD resources
 - **[linstor]** Alerts for LINSTOR [#1035](https://github.com/deckhouse/deckhouse/pull/1035)
    Add alerts with the recommended course of action to monitor LINSTOR, Piraeus-operator, capacity of storage-pools and resources states
 - **[linstor]** Autoimport LVM pools based on tags [#923](https://github.com/deckhouse/deckhouse/pull/923)
    Now LVM pools will automatically be added to the LINSTOR cluster and StorageClasses generated
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
    This module enables snapshots functionality in Kubernetes cluster

## Fixes


 - **[candi]** Added imagePullPolicy: IfNotPresent to kubernetes-api-proxy static pod. Fixed kubernetes-api-proxy run in docker envs. [#1297](https://github.com/deckhouse/deckhouse/pull/1297)
 - **[cloud-provider-aws]** The necessary IAM policies for creating a peering connection have been added to the documentation. [#504](https://github.com/deckhouse/deckhouse/pull/504)
 - **[cloud-provider-vsphere]** Correct behavior of nestedHardwareVirtualization parameter for VsphereInstanceClass. [#1331](https://github.com/deckhouse/deckhouse/pull/1331)
    Node groups with VsphereInstanceClass runtimeOptions.nestedHardwareVirtualization set to false have to be manually updated for this setting to take place. New nodes will be created with disabled nested hardware virtualization if it is disabled in configuration.
 - **[control-plane-manager]** Add the `--experimental-initial-corrupt-check` flag for etcd. [#1267](https://github.com/deckhouse/deckhouse/pull/1267)
    Etcd will be restarted.
 - **[linstor]** LINSTOR module now supports high-availability [#1147](https://github.com/deckhouse/deckhouse/pull/1147)
    Multimaster clusters will automatically turn LINSTOR into HA-mode
 - **[node-local-dns]** Reworked health checking logic [#388](https://github.com/deckhouse/deckhouse/pull/388)
    Now Pods shouldn't crash unexpectedly now due to poor implementation of locking/probing.
 - **[openvpn]** Fixed OpenAPI [#1307](https://github.com/deckhouse/deckhouse/pull/1307)
 - **[openvpn]** Web interface changed to https://github.com/flant/ovpn-admin. Persistent storage has been replaced with Kubernetes secrets. Added HostPort inlet. [#522](https://github.com/deckhouse/deckhouse/pull/522)
    OpenVPN will be migrated from using PVC to store certificates to Kubernetes secrets. PVC will still remain in the cluster as a backup. If you don't need it, you should manually delete it from the cluster.
 - **[prometheus]** Set Grafana sample limit to 5000 [#1215](https://github.com/deckhouse/deckhouse/pull/1215)
 - **[upmeter]** Upmeter no longer exposes DNS queries to the Internet [#1256](https://github.com/deckhouse/deckhouse/pull/1256)
 - **[upmeter]** Fixed the calculation of groups uptime [#1144](https://github.com/deckhouse/deckhouse/pull/1144)

## Chore


 - **[cert-manager]** Bump to version 1.7.1. Fix a possible bug with ACME solvers when you don't have a default ingress class like nginx (a very rare case). Minor bug fixes [#1082](https://github.com/deckhouse/deckhouse/pull/1082)
 - **[docs]** Remove ingressClassName from the site Ingress templates [#1336](https://github.com/deckhouse/deckhouse/pull/1336)
 - **[upmeter]** Remove redundant smoke-mini Ingress [#1237](https://github.com/deckhouse/deckhouse/pull/1237)
 - **[upmeter]** Add User-Agent header to all requests [#1213](https://github.com/deckhouse/deckhouse/pull/1213)

