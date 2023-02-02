# Changelog v1.43

## Know before update


 - Components will be restarted in the following modules:
    * every module using `csi-external-attacher`, `csi-external-provisioner`, `csi-external-resizer`, `csi-external-snapshotter`, `csi-livenessprobe`, `csi-node-registrar`, `kube-rbac-proxy`
    * `basic-auth`
    * `chrony`
    * `cilium-hubble`
    * `cloud-provider-aws`
    * `cloud-provider-azure`
    * `cloud-provider-gcp`
    * `cloud-provider-openstack`
    * `cloud-provider-vsphere`
    * `cni-cilium`
    * `control-plane-manager`
    * `dashboard`
    * `deckhouse`
    * `deckhouse-web`
    * `extended-monitoring`
    * `flant-integration`
    * `ingress-nginx`
    * `istio`
    * `keepalived`
    * `kube-dns`
    * `kube-proxy`
    * `linstor`
    * `log-shipper`
    * `metallb`
    * `monitoring-kubernetes`
    * `monitoring-ping`
    * `network-gateway`
    * `node-local-dns`
    * `node-manager`
    * `openvpn`
    * `prometheus`
    * `registrypackages`
    * `terraform-manager`
    * `upmeter`
    * `user-authn`
    * `user-authz`

## Features


 - **[admission-policy-engine]** Bump gatekeeper version to `3.10.0` to close CVE. [#3420](https://github.com/deckhouse/deckhouse/pull/3420)
 - **[candi]** Add support for merging additional configs to `containerd.toml`. [#3596](https://github.com/deckhouse/deckhouse/pull/3596)
    All `containerd` daemons will restart.
 - **[candi]** Updated containerd version to `1.6.14`.
    Added Deckhouse release requirement about minimal Ubuntu OS version. [#3388](https://github.com/deckhouse/deckhouse/pull/3388)
    All `containerd` daemons will restart.
 - **[candi]** Create bashible events with errors in the default namespace. [#3351](https://github.com/deckhouse/deckhouse/pull/3351)
 - **[cert-manager]** Remove legacy `cert-manager` annotations converter. [#3425](https://github.com/deckhouse/deckhouse/pull/3425)
    `cert-manager` legacy Ingress annotation `certmanager.k8s.io/*`  will no longer be supported.
 - **[cni-cilium]** Added Deckouse config value for cilium entity labels [#3573](https://github.com/deckhouse/deckhouse/pull/3573)
    Cilium Pods should be restarted.
 - **[deckhouse]** Added Deckhouse image validation in the `change-registry.sh` script. [#3499](https://github.com/deckhouse/deckhouse/pull/3499)
 - **[deckhouse]** Added authentication settings to the update notification hook. [#3399](https://github.com/deckhouse/deckhouse/pull/3399)
 - **[deckhouse-web]** Improved search in the documentation. [#3591](https://github.com/deckhouse/deckhouse/pull/3591)
 - **[dhctl]** Wait for the cluster bootstrapped state and output diagnostic messages about cloud ephemeral nodes. [#3075](https://github.com/deckhouse/deckhouse/pull/3075)
 - **[dhctl]** Add version number information to the `dhctl` image. [#2933](https://github.com/deckhouse/deckhouse/pull/2933)
 - **[flant-integration]** Added more node metrics to address issues with the billing for control plane nodes without expected taints. [#3093](https://github.com/deckhouse/deckhouse/pull/3093)
 - **[linstor]** Introduce `linstor-scheduler-admission` for automatically setting `schedulerName` for Pods using linstor volumes. [#3559](https://github.com/deckhouse/deckhouse/pull/3559)
 - **[log-shipper]** Add type field for telemetry metrics. [#3582](https://github.com/deckhouse/deckhouse/pull/3582)
 - **[log-shipper]** Add indexes fields for Splunk destination. [#3566](https://github.com/deckhouse/deckhouse/pull/3566)
 - **[node-manager]** Added `quickShutdown` option to the NodeGroup CR. It will result in Machines draining in 5 minutes, insted of 2 hours, regardless of PDB or other obstacles. [#3429](https://github.com/deckhouse/deckhouse/pull/3429)
 - **[virtualization]** A new module that allows you to run virtual machines. [#1357](https://github.com/deckhouse/deckhouse/pull/1357)

## Fixes


 - **[candi]** Bump `shell-operator` to `1.1.3`. Update base images to mitigate found CVEs. [#3335](https://github.com/deckhouse/deckhouse/pull/3335)
    Components will be restarted in the following modules:
    * every module using `csi-external-attacher`, `csi-external-provisioner`, `csi-external-resizer`, `csi-external-snapshotter`, `csi-livenessprobe`, `csi-node-registrar`, `kube-rbac-proxy`
    * `basic-auth`
    * `chrony`
    * `cilium-hubble`
    * `cloud-provider-aws`
    * `cloud-provider-azure`
    * `cloud-provider-gcp`
    * `cloud-provider-openstack`
    * `cloud-provider-vsphere`
    * `cni-cilium`
    * `control-plane-manager`
    * `dashboard`
    * `deckhouse`
    * `deckhouse-web`
    * `extended-monitoring`
    * `flant-integration`
    * `ingress-nginx`
    * `istio`
    * `keepalived`
    * `kube-dns`
    * `kube-proxy`
    * `linstor`
    * `log-shipper`
    * `metallb`
    * `monitoring-kubernetes`
    * `monitoring-ping`
    * `network-gateway`
    * `node-local-dns`
    * `node-manager`
    * `openvpn`
    * `prometheus`
    * `registrypackages`
    * `terraform-manager`
    * `upmeter`
    * `user-authn`
    * `user-authz`
 - **[chrony]** Use `NTPDaemonOnNodeDoesNotSynchronizeTime` alert only for cluster nodes. [#3577](https://github.com/deckhouse/deckhouse/pull/3577)
 - **[cloud-provider-yandex]** Changes to CCM:
    - Introduced locking to Route Table operations, so that only one operation on a route table can run simultaneously.
    - Disabled useless Route Table updates on ListRoutes(). [#3575](https://github.com/deckhouse/deckhouse/pull/3575)
 - **[control-plane-manager]** Make authn webhook CA optional. [#3538](https://github.com/deckhouse/deckhouse/pull/3538)
 - **[deckhouse]** Temporarily removed the requirement for a minimal Ubuntu node version. [#3714](https://github.com/deckhouse/deckhouse/pull/3714)
 - **[deckhouse-config]** Support integer numbers for settings constrained with the float number in `multipleOf`. [#3612](https://github.com/deckhouse/deckhouse/pull/3612)
 - **[helm]** Change deprecated resources check parameters. Make the load more uniform. [#3590](https://github.com/deckhouse/deckhouse/pull/3590)
 - **[istio]** Using the `iptables-wrapper-installer.sh` script in proxy images. [#3614](https://github.com/deckhouse/deckhouse/pull/3614)
 - **[node-local-dns]** Switched stale cache behavior from `immediate` to `verified`. [#3428](https://github.com/deckhouse/deckhouse/pull/3428)
 - **[node-manager]** fix bashible service checking [#3648](https://github.com/deckhouse/deckhouse/pull/3648)
 - **[prometheus]** Fix Alertmanager CA file (caused Unauthorized error). [#3726](https://github.com/deckhouse/deckhouse/pull/3726)
 - **[prometheus]** Make each Grafana dashboard unique by UID. [#3255](https://github.com/deckhouse/deckhouse/pull/3255)
 - **[registrypackages]** Allow downgrading RPMs from registrypackages in any RPM-distro. [#3358](https://github.com/deckhouse/deckhouse/pull/3358)
 - **[user-authz]** Enabled TLS certificate rotation for the authn webhook. [#3319](https://github.com/deckhouse/deckhouse/pull/3319)
 - **[virtualization]** Bump versions, enable HA and configure placement. [#3650](https://github.com/deckhouse/deckhouse/pull/3650)

## Chore


 - **[cni-cilium]** Bump cilium to `v1.11.12`, hubble to `v0.9.5`, increase `bpf-lb-map-max` value. [#3459](https://github.com/deckhouse/deckhouse/pull/3459)
    All cilium and hubble Pods will be restarted.
 - **[deckhouse]** Changed the `deckhouse_registry` hook to get registry data from the `docker-registry` Secret. The global values of the registry are refactored for all modules. [#3193](https://github.com/deckhouse/deckhouse/pull/3193)
 - **[linstor]** Update LINSTOR to v1.20.3 + other components version. [#3658](https://github.com/deckhouse/deckhouse/pull/3658)
 - **[monitoring-kubernetes]** Attempt to fix `oom_kills:normalized` for cgroupfs driver. [#3410](https://github.com/deckhouse/deckhouse/pull/3410)
 - **[secret-copier]** Add annotation with create/update timestamp to copied Secrets. [#3618](https://github.com/deckhouse/deckhouse/pull/3618)
 - **[terraform-manager]** Rebuild image only if OpenAPI spec is changed. [#3432](https://github.com/deckhouse/deckhouse/pull/3432)

