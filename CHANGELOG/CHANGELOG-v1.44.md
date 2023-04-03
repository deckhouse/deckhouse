# Changelog v1.44

## Features


 - **[admission-policy-engine]** OperationPolicy could be run in the `Dryrun` or `Warn` mode. Fix OperationPolicy label selectors [#3900](https://github.com/deckhouse/deckhouse/pull/3900)
 - **[candi]** Upgraded patch versions of Kubernetes images: `v1.23.16`, `v1.24.10`, and `v1.25.6`. [#3606](https://github.com/deckhouse/deckhouse/pull/3606)
    Kubernetes control-plane components will restart, and kubelet will restart.
 - **[cni-cilium]** Use predictable MAC-addresses generation. [#3889](https://github.com/deckhouse/deckhouse/pull/3889)
    All new veth interfaces for Pods will be created with stable MAC-address, which is not changing during the live-migration.
 - **[deckhouse]** Added bash wrapper for handling USR signals. [#3660](https://github.com/deckhouse/deckhouse/pull/3660)
 - **[deckhouse]** Added Python environment to support Python hooks. [#3523](https://github.com/deckhouse/deckhouse/pull/3523)
 - **[deckhouse-config]** Support statuses for external modules. [#3531](https://github.com/deckhouse/deckhouse/pull/3531)
 - **[deckhouse-controller]** Use the `lib-helm` instead of the `helm_lib` directory. [#3665](https://github.com/deckhouse/deckhouse/pull/3665)
 - **[extended-monitoring]** Added a tip about how to find problem nodes for unscheduled DaemonSet replicas. [#3705](https://github.com/deckhouse/deckhouse/pull/3705)
 - **[external-module-manager]** Add the new module for loading external modules in runtime. [#3629](https://github.com/deckhouse/deckhouse/pull/3629)
 - **[flow-schema]** The new module which adds flow schema to prevent API overloading. [#3674](https://github.com/deckhouse/deckhouse/pull/3674)
 - **[istio]** Add istio version `1.16.2`. [#3595](https://github.com/deckhouse/deckhouse/pull/3595)
    In environments where legacy versions of istio are used, the `D8IstioDeprecatedIstioVersionInstalled` alert will be fired.
 - **[log-shipper]** Alert if log-shipper cannot send or collect logs. [#3149](https://github.com/deckhouse/deckhouse/pull/3149)
 - **[openvpn]** Add high availability configuration for openvpn server. [#3820](https://github.com/deckhouse/deckhouse/pull/3820)
 - **[openvpn]** Added `pmacct` JSON-log audit support for OpenVPN. [#3686](https://github.com/deckhouse/deckhouse/pull/3686)
 - **[operator-trivy]** The new module. [#3858](https://github.com/deckhouse/deckhouse/pull/3858)
 - **[runtime-audit-engine]** The new module to collect security events about possible threats in the cluster. [#3477](https://github.com/deckhouse/deckhouse/pull/3477)
 - **[snapshot-controller]** Add support for snapshots using `ceph-csi` driver. [#2002](https://github.com/deckhouse/deckhouse/pull/2002)
    `ceph-csi` now enables `snapshot-controller` by default and automatically configures `VolumeSnapshotClasses`.
 - **[user-authn]** Add robots.txt for Dex [#3926](https://github.com/deckhouse/deckhouse/pull/3926)
 - **[virtualization]** Kubevirt `v0.58.1`. [#3989](https://github.com/deckhouse/deckhouse/pull/3989)

## Fixes


 - **[admission-policy-engine]** Refactor `admission-policy-engine` monitoring rules. [#3901](https://github.com/deckhouse/deckhouse/pull/3901)
 - **[candi]** Reorder swap disabling steps. [#3772](https://github.com/deckhouse/deckhouse/pull/3772)
 - **[cloud-provider-gcp]** Update `kube-proxy` configuration to set listen address to `0.0.0.0/0` when using GCP cloud provider. [#3914](https://github.com/deckhouse/deckhouse/pull/3914)
    `kube-proxy` Pods will be recreated.
 - **[cloud-provider-openstack]** Support for offline resize. Fix no effect after enable `ignoreVolumeMicroversion`. [#3909](https://github.com/deckhouse/deckhouse/pull/3909)
 - **[cloud-provider-vsphere]** Stop depending on CCM to uniquely identify instance ID. Fixes a couple of bugs. [#3721](https://github.com/deckhouse/deckhouse/pull/3721)
 - **[cni-cilium]** Use predefined MAC-addresses for virtualization workloads. [#4071](https://github.com/deckhouse/deckhouse/pull/4071)
 - **[cni-cilium]** Perform routing lookup for custom tables. [#4046](https://github.com/deckhouse/deckhouse/pull/4046)
 - **[containerized-data-importer]** Make CDI working with `customCertificate`. [#3985](https://github.com/deckhouse/deckhouse/pull/3985)
 - **[deckhouse-config]** Remove `nodeSelector` for `deckhouse-config-webhook`. [#4192](https://github.com/deckhouse/deckhouse/pull/4192)
 - **[deckhouse-config]** Temporarily set the `Recreate` strategy for `deckhouse-config-webhook` Deployment. [#4191](https://github.com/deckhouse/deckhouse/pull/4191)
 - **[deckhouse-config]** Place the `deckhouse-config-webhook` on the same node as Deckhouse. [#4014](https://github.com/deckhouse/deckhouse/pull/4014)
 - **[go_lib]** Remove the `go_lib/hooks/delete_not_matching_certificate_secret/hook.go` hook. [#3777](https://github.com/deckhouse/deckhouse/pull/3777)
 - **[ingress-nginx]** Improve rollout hook to avoid concurrent controller pod deletion [#3915](https://github.com/deckhouse/deckhouse/pull/3915)
 - **[ingress-nginx]** Fix `HostWithFailover` inlet to work with cilium CNI. [#3834](https://github.com/deckhouse/deckhouse/pull/3834)
    All `proxy-<ingress-name>-failover` daemonsets will be restarted.
 - **[istio]** D8IstioDeprecatedIstioVersionInstalled alert description clarification. [#4010](https://github.com/deckhouse/deckhouse/pull/4010)
 - **[istio]** Added check of istiod operation before controller starts upgrading required resources. [#3710](https://github.com/deckhouse/deckhouse/pull/3710)
 - **[log-shipper]** Fix throttling alert labels. [#4222](https://github.com/deckhouse/deckhouse/pull/4222)
 - **[log-shipper]** Add job label selector to alerts query. [#4051](https://github.com/deckhouse/deckhouse/pull/4051)
 - **[log-shipper]** Fix the exclude clause for unschedulable nodes in the RateLimit alert. [#4018](https://github.com/deckhouse/deckhouse/pull/4018)
 - **[log-shipper]** Bump `librdkafka` to `v2.0.2` to make log-shipper read the full CA certificates chain for Kafka. [#3693](https://github.com/deckhouse/deckhouse/pull/3693)
 - **[monitoring-kubernetes]** Fix regex in the `node_exporter`. [#3799](https://github.com/deckhouse/deckhouse/pull/3799)
    All `node_exporter` Pods will be restarted.
 - **[node-manager]** Stop deleting Yandex Cloud preemptible instances if percent of Ready Machines in a NodeGroup dips below 90%. Algorithm is simplified. [#3589](https://github.com/deckhouse/deckhouse/pull/3589)
 - **[openvpn]** Use the same tunnel network for TCP and UDP. [#3749](https://github.com/deckhouse/deckhouse/pull/3749)
 - **[prometheus]** Fix the time interval for Prometheus longterm. [#4174](https://github.com/deckhouse/deckhouse/pull/4174)
 - **[prometheus]** Increase Prometheus self sample limit. [#4066](https://github.com/deckhouse/deckhouse/pull/4066)
 - **[prometheus]** Change resources determination for Prometheus. [#3848](https://github.com/deckhouse/deckhouse/pull/3848)
 - **[prometheus-metrics-adapter]** Use relative CPU metrics query interval to fix an issue with flaky CPU metrics if a scrape interval is higher than 30s. [#3846](https://github.com/deckhouse/deckhouse/pull/3846)
 - **[runtime-audit-engine]** Fix `K8sAudit` -> `k8s_audit` source convert action. [#4134](https://github.com/deckhouse/deckhouse/pull/4134)
 - **[virtualization]** Fix nil pointer exception and `volumeAttachments` status. [#4164](https://github.com/deckhouse/deckhouse/pull/4164)

## Chore


 - **[admission-policy-engine]** Change recommended `imagePullPolicy` to `Always`. [#3940](https://github.com/deckhouse/deckhouse/pull/3940)
 - **[cloud-provider-aws]** Added `etcdDisk.sizeGb` and `etcdDisk.type` parameters to `AWSClusterConfiguration`. [#2369](https://github.com/deckhouse/deckhouse/pull/2369)
 - **[cni-cilium]** Split `cilium` and `virt-cilium`. [#4088](https://github.com/deckhouse/deckhouse/pull/4088)
    All cilium agent Pods will be restarted.
 - **[cni-cilium]** Bump cilium to `v1.11.14`. [#3870](https://github.com/deckhouse/deckhouse/pull/3870)
    All `cilium` Pods will be restarted.
 - **[cni-cilium]** Bump cilium to `v1.11.13` [#3837](https://github.com/deckhouse/deckhouse/pull/3837)
    All cilium Pods will be restarted.
 - **[control-plane-manager]** Kubernetes version 1.21 support will be remove in the next (1.45) Deckhouse release. An alert have been added to keep you from forgetting about it. [#3921](https://github.com/deckhouse/deckhouse/pull/3921)
 - **[deckhouse]** Rename `Outdated` status to `Superseded` in `DeckhouseRelease`. [#3878](https://github.com/deckhouse/deckhouse/pull/3878)
 - **[docs]** Fix broken links. [#3969](https://github.com/deckhouse/deckhouse/pull/3969)
 - **[log-shipper]** Update vector to `0.27.0`. [#3605](https://github.com/deckhouse/deckhouse/pull/3605)
 - **[monitoring-kubernetes]** The `DeprecatedDockerContainerRuntime` alert is switched on — it is time to use containerd now. [#3763](https://github.com/deckhouse/deckhouse/pull/3763)
 - **[operator-prometheus]** Bump Prometheus operator to v0.62.0 and alertmanager to v0.25.0. Sending alerts to Telegram is native without proxies now. [#3757](https://github.com/deckhouse/deckhouse/pull/3757)
    Prometheus and Prometheus operator Pods will restart.
 - **[prometheus]** Added `longtermNodeSelector` and `longtermTolerations` options to the module. [#3711](https://github.com/deckhouse/deckhouse/pull/3711)

