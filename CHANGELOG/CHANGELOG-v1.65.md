# Changelog v1.65

## Know before update


 - Updating openvpn to 2.6.*
 - containerd service will restart.
 - delivery module was removed, check if the module is disabled before update.

## Features


 - **[candi]** Extend regexp in `004_resize_partitions.sh` for detect lvm partition with capital letter and digit in name. [#10233](https://github.com/deckhouse/deckhouse/pull/10233)
 - **[candi]** Add support for openSUSE and mosOS. [#9436](https://github.com/deckhouse/deckhouse/pull/9436)
 - **[candi]** Install CA certificates on nodes using d8-ca-updater, which is installed from the registrypackages. [#9246](https://github.com/deckhouse/deckhouse/pull/9246)
 - **[candi]** Update containerd to 1.7.20. [#9246](https://github.com/deckhouse/deckhouse/pull/9246)
    containerd service will restart.
 - **[ceph-csi]** Make ceph-csi module deprecated. [#10009](https://github.com/deckhouse/deckhouse/pull/10009)
 - **[cloud-provider-aws]** Added the ability to specify your IAM role. [#9530](https://github.com/deckhouse/deckhouse/pull/9530)
 - **[cni-cilium]** Add ability to explicitly specify network interfaces for Virtual IP in EgressGateway. [#10326](https://github.com/deckhouse/deckhouse/pull/10326)
 - **[cni-cilium]** Adding support for configuring each node individually using CiliumNodeConfig resources. [#9754](https://github.com/deckhouse/deckhouse/pull/9754)
 - **[control-plane-manager]** patch etcd to support outputting of snapshots to stdout [#9948](https://github.com/deckhouse/deckhouse/pull/9948)
 - **[control-plane-manager]** Add CronJob that does etcd backup. [#9637](https://github.com/deckhouse/deckhouse/pull/9637)
 - **[deckhouse]** Prohibit to change resources with the label `heritage: deckhouse` even from the `kubernetes-admin` user. [#9852](https://github.com/deckhouse/deckhouse/pull/9852)
 - **[deckhouse]** Get rid of the rbacgen tool. [#9622](https://github.com/deckhouse/deckhouse/pull/9622)
 - **[deckhouse]** Extend Deckhouse update settings. [#9314](https://github.com/deckhouse/deckhouse/pull/9314)
    Changed logic of automatic updates modes (read parameter [settings.update.mode](https://deckhouse.io/products/kubernetes-platform/documentation/v1.65/modules/002-deckhouse/configuration.html#parameters-update-mode) description of module deckhouse ).
 - **[deckhouse-controller]** Added support for Auto Patch mode for Modules Release (configurable in the module update policy object) [#10466](https://github.com/deckhouse/deckhouse/pull/10466)
 - **[deckhouse-controller]** Added `backup.deckhouse.io/cluster-config` label to Deckhouse CRD. [#10111](https://github.com/deckhouse/deckhouse/pull/10111)
 - **[deckhouse-controller]** Add disable confirmation settings for critical modules. [#10098](https://github.com/deckhouse/deckhouse/pull/10098)
 - **[deckhouse-controller]** Ability to watch info about deckhouse release and module releases in the registry from cli. [#10046](https://github.com/deckhouse/deckhouse/pull/10046)
 - **[deckhouse-controller]** Now, if there are several factors limiting deployment, all reasons with the nearest possible moment of deployment will be indicated in the release status. In addition, sending metrics about blocked module releases has been removed if the corresponding module is inactive [#9988](https://github.com/deckhouse/deckhouse/pull/9988)
 - **[deckhouse-controller]** Add discovered GVKs from modules' CRDs to global values. [#9963](https://github.com/deckhouse/deckhouse/pull/9963)
 - **[deckhouse-controller]** adding an alert that manual confirmation is required to install mr [#9943](https://github.com/deckhouse/deckhouse/pull/9943)
 - **[deckhouse-controller]** Get rid of crd modules. [#9593](https://github.com/deckhouse/deckhouse/pull/9593)
 - **[deckhouse-controller]** Improve module validation. [#9293](https://github.com/deckhouse/deckhouse/pull/9293)
 - **[dhctl]** In commander mode connect to controlled clusters via commander agent instead of SSH [#10342](https://github.com/deckhouse/deckhouse/pull/10342)
 - **[dhctl]** Upon editing configuration secrets, create them if they are missing from cluster [#9689](https://github.com/deckhouse/deckhouse/pull/9689)
 - **[dhctl]** Reduces code duplication in the gRPC server message handler and log sender, refactors the graceful shutdown mechanism, and adds support for proper log output for multiple parallel instances of the dhctl server. [#9096](https://github.com/deckhouse/deckhouse/pull/9096)
 - **[dhctl]** Reduce manual operations when converging control plane nodes. [#8380](https://github.com/deckhouse/deckhouse/pull/8380)
 - **[go_lib]** Ability to watch info about deckhouse release and module releases in the registry from cli. [#10046](https://github.com/deckhouse/deckhouse/pull/10046)
 - **[metallb]** Added pre-upgrade compatibility check for metallb configuration. [#10289](https://github.com/deckhouse/deckhouse/pull/10289)
 - **[multitenancy-manager]** Add projects render validation. [#9607](https://github.com/deckhouse/deckhouse/pull/9607)
 - **[operator-trivy]** Add a setting for downloading images from insecure registries. [#10607](https://github.com/deckhouse/deckhouse/pull/10607)
 - **[operator-trivy]** Add support for insecure registries. [#10124](https://github.com/deckhouse/deckhouse/pull/10124)
 - **[operator-trivy]** Bump operator-trivy version to `0.22.0`. [#10045](https://github.com/deckhouse/deckhouse/pull/10045)
 - **[static-routing-manager]** Add the ability to create routes with "via dev" and without specifying a gateway IP. [#10277](https://github.com/deckhouse/deckhouse/pull/10277)
    Pods of static-routing-manager will be restarted.
 - **[user-authn]** Allow device flow for Kubernets API [#10140](https://github.com/deckhouse/deckhouse/pull/10140)
 - **[user-authn]** Refresh groups on updating tokens. [#9598](https://github.com/deckhouse/deckhouse/pull/9598)
 - **[user-authz]** Improve rbacv2 hook to support custom roles, and roles extending and add docs. [#10241](https://github.com/deckhouse/deckhouse/pull/10241)

## Fixes


 - **[candi]** Delete quotes for `primary_mac` value in bootstrap-network script. [#10572](https://github.com/deckhouse/deckhouse/pull/10572)
 - **[candi]** Check for transparent hugepages before trying to disable them in sysctl-tuner [#10294](https://github.com/deckhouse/deckhouse/pull/10294)
 - **[candi]** Add RSA in tls-cipher-suites apiserver for CIS Benchmark 1.6 [#10238](https://github.com/deckhouse/deckhouse/pull/10238)
 - **[candi]** candi/version_map.yml updated to use the latest changes in yandex-cloud-controller-manager [#9855](https://github.com/deckhouse/deckhouse/pull/9855)
 - **[candi]** Step "check_hostname_uniqueness" works without temporary files creation [#9756](https://github.com/deckhouse/deckhouse/pull/9756)
 - **[candi]** Seamless change of clusterDomain. [#9739](https://github.com/deckhouse/deckhouse/pull/9739)
 - **[candi]** Use statically built `lsblk`. [#9666](https://github.com/deckhouse/deckhouse/pull/9666)
 - **[candi]** Added the ability to configure Node DNS servers via the Azure Cloud Provider. [#9554](https://github.com/deckhouse/deckhouse/pull/9554)
 - **[cloud-provider-aws]** Fix for deploying a cluster in a local zone. [#10491](https://github.com/deckhouse/deckhouse/pull/10491)
 - **[cloud-provider-aws]** revert "Added the ability to specify your IAM role" [#10435](https://github.com/deckhouse/deckhouse/pull/10435)
 - **[cloud-provider-vcd]** Fix vCloudDirector catalogs sharing. [#9802](https://github.com/deckhouse/deckhouse/pull/9802)
 - **[cloud-provider-yandex]** Add support a hybrid cluster in yandex CSI driver [#9861](https://github.com/deckhouse/deckhouse/pull/9861)
 - **[cloud-provider-zvirt]** Add to the instance a status about zvirtinstance. [#10236](https://github.com/deckhouse/deckhouse/pull/10236)
 - **[cloud-provider-zvirt]** 401 Unauthorized error fixed in zvirt csi-node. [#10035](https://github.com/deckhouse/deckhouse/pull/10035)
 - **[cni-cilium]** Fixed `excludedCIDRs` option in EgressGatewayPolicies [#10493](https://github.com/deckhouse/deckhouse/pull/10493)
 - **[cni-cilium]** Disable the metrics server in the "egress-gateway-agent" because we don't use it. [#10208](https://github.com/deckhouse/deckhouse/pull/10208)
    The pods of the egress-gateway-agent will be restarted.
 - **[common]** Bump shell-operator image. [#10764](https://github.com/deckhouse/deckhouse/pull/10764)
    Fix webhook-handler memory leak.
 - **[common]** Bump shell-operator image. [#10761](https://github.com/deckhouse/deckhouse/pull/10761)
    Fix webhook-handler memory leak.
 - **[common]** Add `/bin/true` to `init` image [#10372](https://github.com/deckhouse/deckhouse/pull/10372)
 - **[control-plane-manager]** Fix kubeadm template when changing ServiceAcciount Issuer. [#10762](https://github.com/deckhouse/deckhouse/pull/10762)
 - **[control-plane-manager]** Fixed free space sufficiency detection for etcd-backup [#10426](https://github.com/deckhouse/deckhouse/pull/10426)
 - **[control-plane-manager]** Seamless change of clusterDomain. [#9739](https://github.com/deckhouse/deckhouse/pull/9739)
 - **[control-plane-manager]** Automatically regenerate kubeconfig for control plane components if validation fails, preventing crashes. [#9445](https://github.com/deckhouse/deckhouse/pull/9445)
 - **[deckhouse]** Fix module requirements parsing. [#11741](https://github.com/deckhouse/deckhouse/pull/11741)
 - **[deckhouse]** Fix extenders message logs. [#10212](https://github.com/deckhouse/deckhouse/pull/10212)
 - **[deckhouse]** Fix for scaling down of webhook-handler deployment when ha mode is disabled [#9978](https://github.com/deckhouse/deckhouse/pull/9978)
 - **[deckhouse-controller]** Fix deployed module release detection in the ModuleReleaseController. [#10256](https://github.com/deckhouse/deckhouse/pull/10256)
 - **[deckhouse-controller]** Generate empty docker auth for anonymous registry access. [#10210](https://github.com/deckhouse/deckhouse/pull/10210)
 - **[deckhouse-controller]** Fixed a message in the release object about waiting for an annotation about manual confirmation [#10106](https://github.com/deckhouse/deckhouse/pull/10106)
 - **[deckhouse-controller]** Fixed update logic in various modes [#10105](https://github.com/deckhouse/deckhouse/pull/10105)
 - **[deckhouse-controller]** Update the documentation about the list of data the `collect-debug-info` command collects. [#10028](https://github.com/deckhouse/deckhouse/pull/10028)
 - **[deckhouse-controller]** Clean up module documentation when deleting module [#9985](https://github.com/deckhouse/deckhouse/pull/9985)
 - **[deckhouse-tools]** Rebuild d8-cli images when used version changes [#10267](https://github.com/deckhouse/deckhouse/pull/10267)
 - **[dhctl]** Skip remove labels if node was not found during converge. [#10658](https://github.com/deckhouse/deckhouse/pull/10658)
 - **[dhctl]** Do not run converge second time. [#10658](https://github.com/deckhouse/deckhouse/pull/10658)
 - **[dhctl]** Fix random sorting of master in the list. [#10441](https://github.com/deckhouse/deckhouse/pull/10441)
 - **[dhctl]** Deny use defaultCRI type as Docker [#10022](https://github.com/deckhouse/deckhouse/pull/10022)
 - **[dhctl]** Fix lease locking. [#9982](https://github.com/deckhouse/deckhouse/pull/9982)
 - **[dhctl]** Add validation for ClusterConfiguration.cloud.prefix [#9858](https://github.com/deckhouse/deckhouse/pull/9858)
 - **[dhctl]** delete `heritage: deckhouse` label from d8-provider-cluster-configuration and d8-cluster-configuration [#9757](https://github.com/deckhouse/deckhouse/pull/9757)
    users can update secrets by IaC and serviceaccounts
 - **[dhctl]** Added repo check to validateRegistryDockerCfg [#9688](https://github.com/deckhouse/deckhouse/pull/9688)
 - **[dhctl]** Break circle and output error in log on check dependencies if get first error [#9679](https://github.com/deckhouse/deckhouse/pull/9679)
 - **[docs]** Clean up module documentation when deleting module [#9985](https://github.com/deckhouse/deckhouse/pull/9985)
 - **[go_lib]** Clean up module documentation when deleting module [#9985](https://github.com/deckhouse/deckhouse/pull/9985)
 - **[go_lib]** add probe to the cloud-data reconciler [#9915](https://github.com/deckhouse/deckhouse/pull/9915)
 - **[go_lib]** cloud-data-discoverer continues its operation despite temporary issues within the cluster. [#9570](https://github.com/deckhouse/deckhouse/pull/9570)
 - **[ingress-nginx]** Fix ingress validation rule for multiply ingress-controllers. [#10604](https://github.com/deckhouse/deckhouse/pull/10604)
 - **[ingress-nginx]** Add check for existing label. [#10449](https://github.com/deckhouse/deckhouse/pull/10449)
 - **[istio]** SA token path in api-proxy container fixed. [#10454](https://github.com/deckhouse/deckhouse/pull/10454)
 - **[kube-dns]** Seamless change of clusterDomain. [#9739](https://github.com/deckhouse/deckhouse/pull/9739)
 - **[kube-dns]** Graceful rollout of the `kube-dns` deployment without disrupting connections. [#9565](https://github.com/deckhouse/deckhouse/pull/9565)
 - **[log-shipper]** Move cleanup transform to be the last transform for a source. Fixes multiline parsing issue. [#10264](https://github.com/deckhouse/deckhouse/pull/10264)
 - **[monitoring-kubernetes]** add tag main for dashboard [#9677](https://github.com/deckhouse/deckhouse/pull/9677)
    dashbord can be seen on the home page
 - **[monitoring-kubernetes]** Fixed formula for triggering alerts `KubeletNodeFSInodesUsage` and `KubeletImageFSInodesUsage`. [#9436](https://github.com/deckhouse/deckhouse/pull/9436)
 - **[multitenancy-manager]** Fix prometheus labels for ingress traffic in Project templates. [#10117](https://github.com/deckhouse/deckhouse/pull/10117)
 - **[multitenancy-manager]** Change logs format to JSON. [#9955](https://github.com/deckhouse/deckhouse/pull/9955)
 - **[node-manager]** ignore heartbeat annotation on hook [#10368](https://github.com/deckhouse/deckhouse/pull/10368)
 - **[node-manager]** Fixed several RBAC resources in the node-manager module. [#9596](https://github.com/deckhouse/deckhouse/pull/9596)
 - **[operator-trivy]** Fix policies bundle error. [#10199](https://github.com/deckhouse/deckhouse/pull/10199)
 - **[prometheus]** Fix labels for prometheus pod antiAffinity. [#10117](https://github.com/deckhouse/deckhouse/pull/10117)
 - **[prometheus]** Fix stuck GrafanaDashboardDeprecation alerts [#10024](https://github.com/deckhouse/deckhouse/pull/10024)
 - **[user-authn]** Add a patch to fix the problem with offline sessions that are not created/updated properly, which causes random refresh problems. [#10486](https://github.com/deckhouse/deckhouse/pull/10486)
 - **[user-authn]** Trim spaces from email field on the login form. [#10057](https://github.com/deckhouse/deckhouse/pull/10057)
 - **[user-authz]** Get rid of manage admin and user roles, and fix the rbacv2 hook. [#10504](https://github.com/deckhouse/deckhouse/pull/10504)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.28.15`, `v1.29.10`, `v1.30.6` [#10340](https://github.com/deckhouse/deckhouse/pull/10340)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[cloud-provider-yandex]** Changed behavior of externalIPAddresses key in terraform. [#10485](https://github.com/deckhouse/deckhouse/pull/10485)
 - **[cni-cilium]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[cni-cilium]** Updating `cilium` and its components to version 1.14.14 [#9650](https://github.com/deckhouse/deckhouse/pull/9650)
    All cilium pods will be restarted.
 - **[cni-flannel]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[common]** Create image for init containers. [#9992](https://github.com/deckhouse/deckhouse/pull/9992)
 - **[common]** Bump shell-operator to optimize conversion hooks in the webhook-handler. [#9983](https://github.com/deckhouse/deckhouse/pull/9983)
 - **[dashboard]** Change the images to distroless. [#10192](https://github.com/deckhouse/deckhouse/pull/10192)
 - **[dashboard]** Grant create secrets permission. [#10191](https://github.com/deckhouse/deckhouse/pull/10191)
 - **[deckhouse]** delivery module was removed. [#10177](https://github.com/deckhouse/deckhouse/pull/10177)
    delivery module was removed, check if the module is disabled before update.
 - **[deckhouse]** Bump addon-operator to v1.5.0. [#9999](https://github.com/deckhouse/deckhouse/pull/9999)
 - **[deckhouse-controller]** Remove the flant-integration internal module. [#8392](https://github.com/deckhouse/deckhouse/pull/8392)
 - **[dhctl]** Remove support for deprecated `InitConfiguration.configOverrides` parameter. [#9920](https://github.com/deckhouse/deckhouse/pull/9920)
 - **[documentation]** Change images to distroless. [#10040](https://github.com/deckhouse/deckhouse/pull/10040)
 - **[ingress-nginx]** Remove unnecessary validation and warnings. [#10430](https://github.com/deckhouse/deckhouse/pull/10430)
 - **[ingress-nginx]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[ingress-nginx]** Remove v1.6 IngressNginxController. [#9935](https://github.com/deckhouse/deckhouse/pull/9935)
 - **[ingress-nginx]** Update kruise controller to v1.7.2. [#9898](https://github.com/deckhouse/deckhouse/pull/9898)
    kriuse controller will be restarted, pods of an ingress nginx controller of v1.10 will be recreated.
 - **[istio]** Restoring the operation of the audit logs. To see your AuthorizationPolicies AUDIT results, restart application pods with istio sidecars. [#10204](https://github.com/deckhouse/deckhouse/pull/10204)
 - **[istio]** Migrate istio and kiali containers to Alt Linux and Distroless distribution [#9984](https://github.com/deckhouse/deckhouse/pull/9984)
    All pods in the `d8-istio` namespace will be automatically restarted. Other pods with istio-sidecars will apply changes after a manual restart.
 - **[kube-proxy]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[monitoring-deckhouse]** Add CentOS 7 to the `D8NodeHasDeprecatedOSVersion` alert. [#10110](https://github.com/deckhouse/deckhouse/pull/10110)
 - **[monitoring-deckhouse]** Add Debian 10 to the `D8NodeHasDeprecatedOSVersion` alert. [#9798](https://github.com/deckhouse/deckhouse/pull/9798)
 - **[monitoring-kubernetes]** Update kube-state-metrics to 2.13 [#10003](https://github.com/deckhouse/deckhouse/pull/10003)
 - **[network-gateway]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[node-local-dns]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[node-manager]** marked the old api *.deckhouse.io as deprecated [#10021](https://github.com/deckhouse/deckhouse/pull/10021)
 - **[node-manager]** Fix the module's snapshots debugging. [#9995](https://github.com/deckhouse/deckhouse/pull/9995)
 - **[node-manager]** Declarative binding of SSHCredentials and StaticInstance [#9369](https://github.com/deckhouse/deckhouse/pull/9369)
 - **[openvpn]** Replacing iptables with precompiled binaries. [#10103](https://github.com/deckhouse/deckhouse/pull/10103)
    The pods will be restarted.
 - **[openvpn]** changed the images to distroless [#9981](https://github.com/deckhouse/deckhouse/pull/9981)
    Updating openvpn to 2.6.*
 - **[operator-trivy]** Add a setting for downloading images from insecure registries. [#10559](https://github.com/deckhouse/deckhouse/pull/10559)
 - **[prometheus]** Update information about migration Prometheus and Upmeter pods with the local storage to other nodes. [#10194](https://github.com/deckhouse/deckhouse/pull/10194)
 - **[prometheus]** marked the old api *.deckhouse.io as deprecated [#10021](https://github.com/deckhouse/deckhouse/pull/10021)
 - **[prometheus]** move externalLabels to remoteWrite section [#9752](https://github.com/deckhouse/deckhouse/pull/9752)
 - **[upmeter]** Update information about migration Prometheus and Upmeter pods with the local storage to other nodes. [#10194](https://github.com/deckhouse/deckhouse/pull/10194)
 - **[upmeter]** marked the old api *.deckhouse.io as deprecated [#10021](https://github.com/deckhouse/deckhouse/pull/10021)
 - **[user-authn]** marked the old api *.deckhouse.io as deprecated [#10021](https://github.com/deckhouse/deckhouse/pull/10021)
 - **[user-authz]** marked the old api *.deckhouse.io as deprecated [#10021](https://github.com/deckhouse/deckhouse/pull/10021)
 - **[vertical-pod-autoscaler]** marked the old api *.deckhouse.io as deprecated [#10021](https://github.com/deckhouse/deckhouse/pull/10021)

