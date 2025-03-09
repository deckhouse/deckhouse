# Changelog v1.64


## Know before update


 - Ingress nginx controller will restart.
 - l2-load-balancer module is deprecated. To upgrade to the next DKP version, you must disable the l2-load-balancer module.

## Features


 - **[admission-policy-engine]** Added ability to limit `ingressClassName` for Ingress and `storageClassName` for `PersistentVolumeClaim` for projects, namespaces, etc. [#9535](https://github.com/deckhouse/deckhouse/pull/9535)
 - **[candi]** Stricter permissions (0700/0600) applied to kubelet configuration and PKI files to improve security. [#9868](https://github.com/deckhouse/deckhouse/pull/9868)
 - **[candi]** Add support for Astra Linux 1.8. Support for Astra Linux 1.8 ensures compatibility with the latest OS version, providing updated packages and configurations. [#9296](https://github.com/deckhouse/deckhouse/pull/9296)
 - **[candi]** Stricter permissions (0700/0600) applied to kubelet configuration and PKI files to improve security. [#9494](https://github.com/deckhouse/deckhouse/pull/9494)
 - **[candi]** Add new module `deckhouse-tools`. [#9140](https://github.com/deckhouse/deckhouse/pull/9140)
 - **[candi]** Added debugging information in case of error in bb-package-fetch-blob. [#9018](https://github.com/deckhouse/deckhouse/pull/9018)
 - **[cloud-provider-zvirt]** Allow Zvirt volume expansion. [#9667](https://github.com/deckhouse/deckhouse/pull/9667)
 - **[control-plane-manager]** Stricter permissions (0700/0600) applied to kubelet configuration and PKI files to improve security. [#9868](https://github.com/deckhouse/deckhouse/pull/9868)
 - **[control-plane-manager]** Stricter permissions (0700/0600) applied to kubelet configuration and PKI files to improve security. [#9494](https://github.com/deckhouse/deckhouse/pull/9494)
 - **[control-plane-manager]** Added ability to connect kube-scheduler extenders. [#9303](https://github.com/deckhouse/deckhouse/pull/9303)
    kube-scheduler should be restarted every time when extender config is added.
 - **[control-plane-manager]** Add new module `deckhouse-tools`. [#9140](https://github.com/deckhouse/deckhouse/pull/9140)
 - **[dashboard]** Add auth.allowedUserGroups option. Now it is possible to authorize user access based on their groups. [#10068](https://github.com/deckhouse/deckhouse/pull/10068)
 - **[deckhouse]** Add bootstrapped extender. [#9425](https://github.com/deckhouse/deckhouse/pull/9425)
 - **[deckhouse]** Added validation of `update.windows` module parameter. [#9185](https://github.com/deckhouse/deckhouse/pull/9185)
 - **[deckhouse]** Add `deckhouseVersion` and `kubernetesVersion` extenders. [#8997](https://github.com/deckhouse/deckhouse/pull/8997)
 - **[deckhouse-controller]** Add mechanism to check that desired modules are disabled before deckhouse update. [#10176](https://github.com/deckhouse/deckhouse/pull/10176)
 - **[deckhouse-controller]** Reflect info about applied extenders in modules' statuses. [#9527](https://github.com/deckhouse/deckhouse/pull/9527)
 - **[deckhouse-controller]** add additional debugging information [#9508](https://github.com/deckhouse/deckhouse/pull/9508)
 - **[deckhouse-controller]** Added ability to send update notifications for Deckhouse patch releases. [#9319](https://github.com/deckhouse/deckhouse/pull/9319)
    The format of webhook notifications about updates has been changed: the value in the version field has been changed from "x.y" to "x.y.z".
 - **[deckhouse-controller]** Restore absent releases from a registry. [#9263](https://github.com/deckhouse/deckhouse/pull/9263)
 - **[deckhouse-controller]** Restart controller if a `ModuleRelease` is `Deployed` phase is deleted. [#9241](https://github.com/deckhouse/deckhouse/pull/9241)
 - **[deckhouse-controller]** Add new module `deckhouse-tools`. [#9140](https://github.com/deckhouse/deckhouse/pull/9140)
 - **[deckhouse-controller]** Add a field about the type of update in the notification. [#9082](https://github.com/deckhouse/deckhouse/pull/9082)
 - **[deckhouse-tools]** Add new module `deckhouse-tools`. [#9140](https://github.com/deckhouse/deckhouse/pull/9140)
 - **[dhctl]** Options to skip preflight checks for dhctl-server operations. [#10043](https://github.com/deckhouse/deckhouse/pull/10043)
 - **[dhctl]** Add cleanup resources confirmation on destroy. [#9515](https://github.com/deckhouse/deckhouse/pull/9515)
 - **[dhctl]** Support for localhost bootstrapping. [#9404](https://github.com/deckhouse/deckhouse/pull/9404)
 - **[dhctl]** Add new Status grpc Service. [#9256](https://github.com/deckhouse/deckhouse/pull/9256)
 - **[dhctl]** The `dhctl bootstrap` command will wait until the status of each resource in the `--config` option changes to `Ready`. [#8328](https://github.com/deckhouse/deckhouse/pull/8328)
 - **[docs]** Added ability to connect kube-scheduler extenders. [#9303](https://github.com/deckhouse/deckhouse/pull/9303)
    kube-scheduler should be restarted every time when extender config is added.
 - **[docs]** Add new module `deckhouse-tools`. [#9140](https://github.com/deckhouse/deckhouse/pull/9140)
 - **[go_lib]** Added heritage label to all crds. [#9273](https://github.com/deckhouse/deckhouse/pull/9273)
 - **[ingress-nginx]** Added additional validation of inlet-related parameters. [#9208](https://github.com/deckhouse/deckhouse/pull/9208)
 - **[l2-load-balancer]** Set the l2-load-balancer module to deprecated status. [#9768](https://github.com/deckhouse/deckhouse/pull/9768)
    l2-load-balancer module is deprecated. To upgrade to the next DKP version, you must disable the l2-load-balancer module.
 - **[loki]** Configurable Loki limits. [#9437](https://github.com/deckhouse/deckhouse/pull/9437)
    Loki pod will be restarted. Short disruption will occur.
 - **[multitenancy-manager]** Add virtual projects and used namespaces to status. [#9463](https://github.com/deckhouse/deckhouse/pull/9463)
 - **[multitenancy-manager]** Add separate controller for projects. [#9291](https://github.com/deckhouse/deckhouse/pull/9291)
    In rare cases, there may be problems with backward compatibility.
 - **[node-manager]** Update `capi-controller-manager` to version `1.7`. [#9587](https://github.com/deckhouse/deckhouse/pull/9587)
 - **[node-manager]** Add alert about unavailable CAPS instances. [#9195](https://github.com/deckhouse/deckhouse/pull/9195)
 - **[registrypackages]** Add new module `deckhouse-tools`. [#9140](https://github.com/deckhouse/deckhouse/pull/9140)
 - **[secret-copier]** Delete ArgoCD labels on copied secrets [#9670](https://github.com/deckhouse/deckhouse/pull/9670)
 - **[user-authn]** Add claimMappingOverride option for OIDC Dex provider. [#9974](https://github.com/deckhouse/deckhouse/pull/9974)
 - **[user-authn]** dex support Base64 encoded and PEM encoded certs. [#9894](https://github.com/deckhouse/deckhouse/pull/9894)
 - **[user-authz]** RBAC v2. The new RBAC model. [#8538](https://github.com/deckhouse/deckhouse/pull/8538)

## Fixes


 - **[candi]** fix resize partition step [#9950](https://github.com/deckhouse/deckhouse/pull/9950)
 - **[candi]** Improve catch errors in bootstrap-network scripts. [#9695](https://github.com/deckhouse/deckhouse/pull/9695)
 - **[candi]** Disable and remove unattended upgrades early for Debian, Ubuntu and Astra. [#9574](https://github.com/deckhouse/deckhouse/pull/9574)
 - **[candi]** Fix bootstrap script for static nodes in hybrid clusters to avoid using cloud metadata. [#9502](https://github.com/deckhouse/deckhouse/pull/9502)
 - **[candi]** Fix network configuration in OpenStack when using DirectRoutingWithPortSecurityEnabled. [#9402](https://github.com/deckhouse/deckhouse/pull/9402)
 - **[candi]** Fix externalIP detaching before deleting for master node in Yandex Cloud. [#9154](https://github.com/deckhouse/deckhouse/pull/9154)
 - **[chrony]** Reduce over-requested memory. [#9206](https://github.com/deckhouse/deckhouse/pull/9206)
 - **[cloud-provider-aws]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-aws]** Update `VolumeSnapshotClass.apiVersion` from `v1beta1` to `v1`. [#9257](https://github.com/deckhouse/deckhouse/pull/9257)
 - **[cloud-provider-aws]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-azure]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-azure]** Update `VolumeSnapshotClass.apiVersion` from `v1beta1` to `v1`. [#9257](https://github.com/deckhouse/deckhouse/pull/9257)
 - **[cloud-provider-azure]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-gcp]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-gcp]** Update `VolumeSnapshotClass.apiVersion` from `v1beta1` to `v1`. [#9257](https://github.com/deckhouse/deckhouse/pull/9257)
 - **[cloud-provider-gcp]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-openstack]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-openstack]** Create one server group for all masters. [#9806](https://github.com/deckhouse/deckhouse/pull/9806)
 - **[cloud-provider-openstack]** Update `VolumeSnapshotClass.apiVersion` from `v1beta1` to `v1`. [#9257](https://github.com/deckhouse/deckhouse/pull/9257)
 - **[cloud-provider-openstack]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-vcd]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-vcd]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-vsphere]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-vsphere]** Update `VolumeSnapshotClass.apiVersion` from `v1beta1` to `v1`. [#9257](https://github.com/deckhouse/deckhouse/pull/9257)
 - **[cloud-provider-vsphere]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-yandex]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-yandex]** Skipping migration `diskSizeGB` for dev branch. [#9365](https://github.com/deckhouse/deckhouse/pull/9365)
 - **[cloud-provider-yandex]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cloud-provider-zvirt]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cloud-provider-zvirt]** Minimize RBAC permissions by removing the wildcard("*") from ClusterRole rules. [#8969](https://github.com/deckhouse/deckhouse/pull/8969)
 - **[cni-cilium]** Disabling the upload of the service image `base-cilium-dev` to the final container registry. [#9987](https://github.com/deckhouse/deckhouse/pull/9987)
    All cilium-agent pods will be restarted.
 - **[cni-cilium]** Wiping unwanted iptables-legacy rules. [#9971](https://github.com/deckhouse/deckhouse/pull/9971)
    All cilium-agent pods will be restarted.
 - **[cni-cilium]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cni-cilium]** iptables-wrapper fix for cilium pods. [#9856](https://github.com/deckhouse/deckhouse/pull/9856)
    The cilium pods will be restarted.
 - **[cni-flannel]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[cni-simple-bridge]** cni-simple-bridge use the same iptables binaries as on the host to prevent incompatibility. [#9254](https://github.com/deckhouse/deckhouse/pull/9254)
 - **[common]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[common]** Fixed the displayed version of shell-operator. [#9281](https://github.com/deckhouse/deckhouse/pull/9281)
 - **[control-plane-manager]** D8EtcdExcessiveDatabaseGrowth alert fix [#9773](https://github.com/deckhouse/deckhouse/pull/9773)
 - **[control-plane-manager]** Two new etcd alerts with a low `severity_level` and an increase in the `severity_level` for one existing alert. [#9464](https://github.com/deckhouse/deckhouse/pull/9464)
 - **[deckhouse]** Fix module requirements parsing. [#11753](https://github.com/deckhouse/deckhouse/pull/11753)
 - **[deckhouse]** Fix validation policy for update windows in kubernetes 1.26 [#10235](https://github.com/deckhouse/deckhouse/pull/10235)
 - **[deckhouse]** Fix `ValidatingAdmissionPolicy` for checking update windows. [#10151](https://github.com/deckhouse/deckhouse/pull/10151)
 - **[deckhouse]** Fixed the `deckhouse-leader` and `deckhouse` Services with multiple ports broken by Helm. [#9573](https://github.com/deckhouse/deckhouse/pull/9573)
 - **[deckhouse]** Fix parsing Kubernetes version. [#9458](https://github.com/deckhouse/deckhouse/pull/9458)
 - **[deckhouse]** Fix `ValidatingAdmissionPolicy` so that a cluster with CAPI rosurces can be deleted successfully. [#9426](https://github.com/deckhouse/deckhouse/pull/9426)
 - **[deckhouse]** Restricted actions on `cluster.x-k8s.io/machine.sapcloud.io`. [#9026](https://github.com/deckhouse/deckhouse/pull/9026)
    Unauthorized users will be unable to manage `cluster.x-k8s.io/machine.sapcloud.io` resources (`machines`, `machinesets`, `machinedeployments`).
 - **[deckhouse-controller]** Fixed a bug related to the fact that the state of the release object was not updated. [#9838](https://github.com/deckhouse/deckhouse/pull/9838)
 - **[deckhouse-controller]** Fixed panic when processing release with nil applyAfter. [#9657](https://github.com/deckhouse/deckhouse/pull/9657)
 - **[deckhouse-controller]** Trim ending slash of registry in helper change-registry command. [#9396](https://github.com/deckhouse/deckhouse/pull/9396)
 - **[deckhouse-controller]** Adding basic-auth support for change-registry helper. [#9336](https://github.com/deckhouse/deckhouse/pull/9336)
 - **[deckhouse-controller]** Fixed `release.deckhouse.io/isUpdating` annotation. [#9081](https://github.com/deckhouse/deckhouse/pull/9081)
 - **[deckhouse-controller]** Use the default `ModuleUpdatePolicy` if the `ModuleUpdatePolicy`, referenced in `ModuleRelease`, has been deleted. [#9035](https://github.com/deckhouse/deckhouse/pull/9035)
 - **[deckhouse-controller]** Correct module validation. [#8989](https://github.com/deckhouse/deckhouse/pull/8989)
 - **[deckhouse-tools]** Fix custom certs copying. [#9840](https://github.com/deckhouse/deckhouse/pull/9840)
 - **[delivery]** Fixed the `argocd-repo-server` and `argocd-server` Services with multiple ports broken by Helm. [#9573](https://github.com/deckhouse/deckhouse/pull/9573)
 - **[dhctl]** Fix empty registry credentials preflight check failure. [#10226](https://github.com/deckhouse/deckhouse/pull/10226)
 - **[dhctl]** Do not return error if deckhouse release exists. [#10164](https://github.com/deckhouse/deckhouse/pull/10164)
 - **[dhctl]** Only one resource will create for namespace if it namespace does not exist. [#10159](https://github.com/deckhouse/deckhouse/pull/10159)
 - **[dhctl]** Fix panic during creation resources and add timestamps to debug log. [#10070](https://github.com/deckhouse/deckhouse/pull/10070)
 - **[dhctl]** Fix ensure required namespaces. [#9714](https://github.com/deckhouse/deckhouse/pull/9714)
 - **[dhctl]** Fix sshBastionPort spec type [#9990](https://github.com/deckhouse/deckhouse/pull/9990)
 - **[dhctl]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[dhctl]** Fix SSH client startup in Deckhouse installation phase. [#9628](https://github.com/deckhouse/deckhouse/pull/9628)
 - **[dhctl]** Retry loop for `ensureRequiredNamespacesExist` function. [#9620](https://github.com/deckhouse/deckhouse/pull/9620)
 - **[dhctl]** Added back missing preflight check for sudo access. [#9290](https://github.com/deckhouse/deckhouse/pull/9290)
 - **[docs]** Add CEF format example in docs log-shipper [#9875](https://github.com/deckhouse/deckhouse/pull/9875)
 - **[docs]** Added steps to configure integrity level for Astra Linux SE to prevent permission issues. [#9442](https://github.com/deckhouse/deckhouse/pull/9442)
 - **[global-hooks]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[go_lib]** Canceling migration from d8-cni-configuration secret to proper CNI module configs. [#9900](https://github.com/deckhouse/deckhouse/pull/9900)
 - **[go_lib]** Skipping migration `diskSizeGB` for dev branch. [#9365](https://github.com/deckhouse/deckhouse/pull/9365)
 - **[helm_lib]** Check a helm client's capabilities before applying validatingadmissionpolicies. [#9705](https://github.com/deckhouse/deckhouse/pull/9705)
 - **[ingress-nginx]** Bump ingress-nginx to `1.10.4`. [#9513](https://github.com/deckhouse/deckhouse/pull/9513)
    Ingress nginx controller will restart.
 - **[istio]** Fix supported Kubernetes version in the documentation. [#10148](https://github.com/deckhouse/deckhouse/pull/10148)
 - **[istio]** Improved validation of the ModuleConfig [#9912](https://github.com/deckhouse/deckhouse/pull/9912)
 - **[istio]** Fixed the `kiali` Service with multiple ports broken by Helm. [#9573](https://github.com/deckhouse/deckhouse/pull/9573)
 - **[istio]** Fixed an issue with automatically applying new custom certificates for mTLS issuing. [#9335](https://github.com/deckhouse/deckhouse/pull/9335)
 - **[kube-dns]** Fixed the `d8-kube-dns` and `d8-kube-dns-redirect` Services with multiple ports broken by Helm. [#9573](https://github.com/deckhouse/deckhouse/pull/9573)
 - **[loki]** Removed migrator init containers from modules. [#10150](https://github.com/deckhouse/deckhouse/pull/10150)
 - **[metallb]** Restore AddressPool CRD of MetalLB module. [#9724](https://github.com/deckhouse/deckhouse/pull/9724)
 - **[monitoring-ping]** Fix `monitoring-ping` pods crashing. [#9533](https://github.com/deckhouse/deckhouse/pull/9533)
 - **[multitenancy-manager]** Fix 'namespace not found' problem. [#9891](https://github.com/deckhouse/deckhouse/pull/9891)
 - **[multitenancy-manager]** Add verify namespace object for messageExpression in ValidatingAdmissionPolicy [#9849](https://github.com/deckhouse/deckhouse/pull/9849)
 - **[multitenancy-manager]** Regeneration RBAC for multitenancy-manager. [#9547](https://github.com/deckhouse/deckhouse/pull/9547)
 - **[node-manager]** Fix CAPS bootstrap multiple nodes. [#9672](https://github.com/deckhouse/deckhouse/pull/9672)
 - **[node-manager]** Fix `CertificateSigningRequest` validation in the auto approve hook. [#9509](https://github.com/deckhouse/deckhouse/pull/9509)
 - **[node-manager]** Fix panic in mcm when deleting several NodeGroups. [#9499](https://github.com/deckhouse/deckhouse/pull/9499)
 - **[node-manager]** Fix `ValidatingAdmissionPolicy` so that a cluster with CAPI rosurces can be deleted successfully. [#9426](https://github.com/deckhouse/deckhouse/pull/9426)
 - **[node-manager]** Restricted actions on `cluster.x-k8s.io/machine.sapcloud.io`. [#9026](https://github.com/deckhouse/deckhouse/pull/9026)
    Unauthorized users will be unable to manage `cluster.x-k8s.io/machine.sapcloud.io` resources (`machines`, `machinesets`, `machinedeployments`).
 - **[prometheus]** Removed migrator init containers from modules. [#10150](https://github.com/deckhouse/deckhouse/pull/10150)
 - **[prometheus]** Fix Grafana root URL. [#10076](https://github.com/deckhouse/deckhouse/pull/10076)
    Grafana will be restarted.
 - **[prometheus]** Increase aggregation-proxy timeout. [#9579](https://github.com/deckhouse/deckhouse/pull/9579)
    Aggregation-proxy deployment will restart.
 - **[prometheus]** Fixed the `memcached` Service with multiple ports broken by Helm. [#9573](https://github.com/deckhouse/deckhouse/pull/9573)
 - **[registrypackages]** Check more kernel modules that are needed to detect the nft support of iptables. [#9601](https://github.com/deckhouse/deckhouse/pull/9601)
 - **[registrypackages]** Add kernel version check for installing iptables. [#9254](https://github.com/deckhouse/deckhouse/pull/9254)
 - **[runtime-audit-engine]** Fix monitoring RBAC permissions. [#9470](https://github.com/deckhouse/deckhouse/pull/9470)
 - **[upmeter]** Removed migrator init containers from modules. [#10150](https://github.com/deckhouse/deckhouse/pull/10150)
 - **[upmeter]** Fix `D8UpmeterSmokeMiniMoreThanOnePVxPVC` alert. [#10026](https://github.com/deckhouse/deckhouse/pull/10026)
 - **[user-authn]** Allow system-users with + symbol in email. [#9846](https://github.com/deckhouse/deckhouse/pull/9846)

## Chore


 - **[admission-policy-engine]** Increase constraint violations limit. [#9551](https://github.com/deckhouse/deckhouse/pull/9551)
    Gatekeeper-audit pod will be recreated.
 - **[admission-policy-engine]** Fix validation webhook match expressions. [#9439](https://github.com/deckhouse/deckhouse/pull/9439)
 - **[candi]** Bump patch versions of Kubernetes images: `v1.28.14`, `v1.29.9`, `v1.30.5` [#9917](https://github.com/deckhouse/deckhouse/pull/9917)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Bump patch versions of Kubernetes images: `v1.28.13`, `v1.29.8`, `v1.30.4` [#9495](https://github.com/deckhouse/deckhouse/pull/9495)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[cilium-hubble]** Improved VM pod appearance in Hubble UI. Now it isn't an "Unknown App", but some VM with name and proper icon. [#9381](https://github.com/deckhouse/deckhouse/pull/9381)
 - **[cloud-provider-aws]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-azure]** Add `Microsoft.CognitiveServices` to allowed values of serviceEndpoint module parameter. [#9399](https://github.com/deckhouse/deckhouse/pull/9399)
 - **[cloud-provider-azure]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-gcp]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-openstack]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-vcd]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-vsphere]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-yandex]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cloud-provider-zvirt]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cni-cilium]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
 - **[cni-cilium]** Enable internal TLS authentication between Ingress controller and Hubble UI. [#9298](https://github.com/deckhouse/deckhouse/pull/9298)
 - **[cni-cilium]** Set VXLAN port into allowed range. [#9089](https://github.com/deckhouse/deckhouse/pull/9089)
    In new installations, the Cilium VXLAN ports will be set to 4299 or 4298 (with virtualization).
 - **[cni-flannel]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
    While previously you could use ModuleConfig`cni-cilium` without settings (only to enable module `cni-cilium`) to bootstrap a cluster, now you must specify the settings explicitly in the module configuration.
 - **[cni-flannel]** Added deletion of stale Cilium CNI configuration file from the host file system when starting flannel. [#9234](https://github.com/deckhouse/deckhouse/pull/9234)
 - **[common]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
 - **[common]** Update base images in `common/shell-operator`. [#9289](https://github.com/deckhouse/deckhouse/pull/9289)
 - **[deckhouse]** Add `EXTERNAL_MODULES_DIR` env for backward compatibility. [#9443](https://github.com/deckhouse/deckhouse/pull/9443)
 - **[deckhouse]** Update Python in images `webhook-handler`. [#9289](https://github.com/deckhouse/deckhouse/pull/9289)
 - **[deckhouse]** Remove external-module-manager module and deprecate `external` notion. [#9051](https://github.com/deckhouse/deckhouse/pull/9051)
 - **[dhctl]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
 - **[dhctl]** Remove mirroring features. [#9197](https://github.com/deckhouse/deckhouse/pull/9197)
    Mirroring via dhctl is deprecated since v1.61 and all of it's functions were moved into the Deckhouse CLI as `d8 mirror` family of commands. Users are expected to migrate to `d8 mirror`.
 - **[docs]** Set VXLAN port into allowed range. [#9089](https://github.com/deckhouse/deckhouse/pull/9089)
    In new installations, the Cilium VXLAN ports will be set to 4299 or 4298 (with virtualization).
 - **[global-hooks]** Removed a deprecated CRD of MetalLB module. [#9466](https://github.com/deckhouse/deckhouse/pull/9466)
 - **[global-hooks]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
 - **[go_lib]** Migration from `d8-cni-configuration` secret to proper CNI module configs. [#9347](https://github.com/deckhouse/deckhouse/pull/9347)
 - **[go_lib]** Added some unit tests. [#9230](https://github.com/deckhouse/deckhouse/pull/9230)
 - **[ingress-nginx]** Added an optional loadBalancerClass setting to IngressNginxController resource. [#9585](https://github.com/deckhouse/deckhouse/pull/9585)
 - **[istio]** Add migration hook for kiali ingresses. [#9748](https://github.com/deckhouse/deckhouse/pull/9748)
 - **[istio]** Enable internal TLS authentication between Ingress controller and Kiali. [#9298](https://github.com/deckhouse/deckhouse/pull/9298)
 - **[istio]** Removed wildcard RBAC from `istio-operator` and control plane. `istio-operator` discharged from deploying RBACs, we handle them from our templates. [#9191](https://github.com/deckhouse/deckhouse/pull/9191)
 - **[log-shipper]** Bump vector to `0.40.1`. [#9520](https://github.com/deckhouse/deckhouse/pull/9520)
    All log-shipper pods will restart.
 - **[log-shipper]** Update vector to 0.40.0 release. [#9384](https://github.com/deckhouse/deckhouse/pull/9384)
    All log-shipper pods will restart.
 - **[operator-trivy]** Fix trivy-server image building procedure. [#9746](https://github.com/deckhouse/deckhouse/pull/9746)
    Trivy-server pod'll be recreated.
 - **[prometheus]** Update Python in images `grafana-dashboard-provisioner`. [#9289](https://github.com/deckhouse/deckhouse/pull/9289)
 - **[registrypackages]** Add standalone docker-registry package. [#9353](https://github.com/deckhouse/deckhouse/pull/9353)
 - **[runtime-audit-engine]** Allow falco to match multiple rules on same event. [#9652](https://github.com/deckhouse/deckhouse/pull/9652)
 - **[runtime-audit-engine]** Update falco to `0.38.1`. [#9189](https://github.com/deckhouse/deckhouse/pull/9189)
    runtime-audit-engine will restart.

