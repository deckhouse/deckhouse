# Changelog v1.69

## [MALFORMED]


 - #12318 unknown section "static-routing-manager"
 - #12510 unknown section "static-routing-manager"
 - #12723 unknown section "ceph-csi"
 - #13174 unknown section "ceph-csi"
 - #13409 unknown section "ceph-csi"

## Know before update


 - For new clusters in Yandex Cloud with the `withNATInstance` layout, `internalSubnetCIDR` or `internalSubnetID` must be specified.
 - In L2 mode, the Public IP address will not be marked as free in the pool after deleting the LoadBalancer Service. As a temporary fix, you can restart the MetalLB controller pods.
 - Ingress-nginx controller pods of v1.9 will be restated.
 - Prevent important data loss when using wide retention intervals.
 - The minimum supported version of Kubernetes is now 1.28. All control plane components will restart.
 - Time-based retention in `loki` is no longer available. See the changelog for details.
 - release upgrade will be blocked on AWS-based clusters where SA doesn't have DescribeAddressesAttribute and DescribeInstanceTopology roles. They are required for new Terraform AWS Provider version.

## Features


 - **[candi]** Add rosa 12.6. [#13369](https://github.com/deckhouse/deckhouse/pull/13369)
 - **[candi]** The `bashible` service now sends events to the `default` namespace when a node update starts and finishes. [#12327](https://github.com/deckhouse/deckhouse/pull/12327)
 - **[candi]** Added support for Kubernetes 1.32 and discontinued support for Kubernetes 1.27. [#11501](https://github.com/deckhouse/deckhouse/pull/11501)
    The minimum supported version of Kubernetes is now 1.28. All control plane components will restart.
 - **[candi]** Removed bundle from bashible-api contexts; nodegroupconfiguration scripts now use an auto-generated header to detect the bundle. [#11479](https://github.com/deckhouse/deckhouse/pull/11479)
 - **[cilium-hubble]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[cloud-provider-yandex]** Improved the `withNATInstance` layout for Yandex Cloud — it now uses a separate NAT instance subnet for new clusters to prevent routing loops. [#12301](https://github.com/deckhouse/deckhouse/pull/12301)
    For new clusters in Yandex Cloud with the `withNATInstance` layout, `internalSubnetCIDR` or `internalSubnetID` must be specified.
 - **[cni-cilium]** Added a new dashboard to visualize node connectivity status across the cluster. [#11960](https://github.com/deckhouse/deckhouse/pull/11960)
 - **[control-plane-manager]** Added alert for detecting stale service account tokens. [#12163](https://github.com/deckhouse/deckhouse/pull/12163)
 - **[dashboard]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[deckhouse]** enable UnmetCloudConditions check [#12957](https://github.com/deckhouse/deckhouse/pull/12957)
    release upgrade will be blocked on AWS-based clusters where SA doesn't have DescribeAddressesAttribute and DescribeInstanceTopology roles. They are required for new Terraform AWS Provider version.
 - **[deckhouse]** Added `sudouser` alias for easier debugging of protected resources. [#12838](https://github.com/deckhouse/deckhouse/pull/12838)
 - **[deckhouse]** Added rollback flag to ModulePullOverride to restore the previous release after deletion. [#12758](https://github.com/deckhouse/deckhouse/pull/12758)
 - **[deckhouse]** Added ModuleSettingsDefinition CRD to store OpenAPI specs for module settings. [#12702](https://github.com/deckhouse/deckhouse/pull/12702)
 - **[deckhouse]** Added `Terminating` status for ModuleSource and ModuleRelease resources. [#12317](https://github.com/deckhouse/deckhouse/pull/12317)
 - **[deckhouse]** Added `disableOptions` field to module properties for controlling disable behavior. [#12312](https://github.com/deckhouse/deckhouse/pull/12312)
 - **[deckhouse]** Added support for module descriptions and tags via annotations and labels. [#12189](https://github.com/deckhouse/deckhouse/pull/12189)
 - **[deckhouse]** Added a hook to disable automatic mounting of tokens for the default `ServiceAccount`. [#11954](https://github.com/deckhouse/deckhouse/pull/11954)
 - **[deckhouse-controller]** sequential processing of module releases [#13216](https://github.com/deckhouse/deckhouse/pull/13216)
 - **[deckhouse-controller]** sequential processing of module releases [#13084](https://github.com/deckhouse/deckhouse/pull/13084)
 - **[deckhouse-controller]** Added support for unmanaged mode in modules that lets you modify module components but lowers the SLA level. [#12686](https://github.com/deckhouse/deckhouse/pull/12686)
 - **[deckhouse-controller]** Merged `priority-class` and `flow-schema` modules with the `deckhouse` module. [#12323](https://github.com/deckhouse/deckhouse/pull/12323)
 - **[deckhouse-tools]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[dhctl]** Extended bootstrap, abort, destroy, and check operations to support cancellation via `context.Context`. [#12672](https://github.com/deckhouse/deckhouse/pull/12672)
 - **[dhctl]** Added generation of a local kubeconfig and setting up a TCP proxy via SSH tunnel for immediate `kubectl` access after a bootstrap. [#12586](https://github.com/deckhouse/deckhouse/pull/12586)
 - **[dhctl]** Made Terraform runner methods to accept `context.Context` for future support of operation cancellation. [#12578](https://github.com/deckhouse/deckhouse/pull/12578)
 - **[dhctl]** Added a time drift check during cluster bootstrap to warn if local and remote server times differ by more than 10 minutes. [#12232](https://github.com/deckhouse/deckhouse/pull/12232)
 - **[dhctl]** Added a preflight check to detect CIDR intersection between `podSubnetCIDR` and `serviceSubnetCIDR`. [#12130](https://github.com/deckhouse/deckhouse/pull/12130)
 - **[dhctl]** Removed bundle from bashible-api contexts; nodegroupconfiguration scripts now use an auto-generated header to detect the bundle. [#11479](https://github.com/deckhouse/deckhouse/pull/11479)
 - **[docs]** Enhanced registry watcher and docs-builder integration, including improved caching, error handling, and added retries. [#12337](https://github.com/deckhouse/deckhouse/pull/12337)
 - **[documentation]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[ingress-nginx]** Added `controllerPodsAdditionalAnnotations` parameter to IngressNginxController for customizing pod annotations. [#11522](https://github.com/deckhouse/deckhouse/pull/11522)
 - **[istio]** Added a validation and an alert when creating a ServiceEntry resource without service ports. [#12133](https://github.com/deckhouse/deckhouse/pull/12133)
 - **[istio]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[istio]** Reworked multi-cluster and federation resource tracking to enable native watch via Kubernetes API and removed ConfigMap or Secret mounting from pods. [#11845](https://github.com/deckhouse/deckhouse/pull/11845)
 - **[kube-dns]** Expanded pattern for `stubZones` to allow underscores (`_`) in domain names. [#12238](https://github.com/deckhouse/deckhouse/pull/12238)
 - **[kube-dns]** Added a dynamic forwader plugin for `node-local-dns` and added details on how to scale `kube-dns` instances to the FAQ. [#11589](https://github.com/deckhouse/deckhouse/pull/11589)
 - **[loki]** Added a `loki` API RBAC manifest to Deckhouse. [#12168](https://github.com/deckhouse/deckhouse/pull/12168)
 - **[loki]** Introduced a disk usage-based cleanup for log retention. [#11993](https://github.com/deckhouse/deckhouse/pull/11993)
    Time-based retention in `loki` is no longer available. See the changelog for details.
 - **[loki]** Added conditional disabling of log deletion API handlers to restrict access when needed. [#11937](https://github.com/deckhouse/deckhouse/pull/11937)
 - **[monitoring-kubernetes-control-plane]** Added support for selecting multiple Kubernetes versions. [#12284](https://github.com/deckhouse/deckhouse/pull/12284)
 - **[multitenancy-manager]** Added namespace adoption mechanism; namespaces with the `projects.deckhouse.io/adopt` annotation are now automatically linked to empty projects. [#12423](https://github.com/deckhouse/deckhouse/pull/12423)
 - **[namespace-configurator]** Added exclusion of `deckhouse` and `multitenancy-manager` namespaces. [#12784](https://github.com/deckhouse/deckhouse/pull/12784)
 - **[node-manager]** enable UnmetCloudConditions check [#12845](https://github.com/deckhouse/deckhouse/pull/12845)
 - **[node-manager]** UnmetCloudConditions requirement and alert [#12530](https://github.com/deckhouse/deckhouse/pull/12530)
 - **[node-manager]** Removed bundle from bashible-api contexts; nodegroupconfiguration scripts now use an auto-generated header to detect the bundle. [#11479](https://github.com/deckhouse/deckhouse/pull/11479)
 - **[node-manager]** Added `nodeDrainTimeoutSecond` parameter to set custom node draining time for each CloudEphemeral NodeGroup. [#10962](https://github.com/deckhouse/deckhouse/pull/10962)
 - **[openvpn]** Added `defaultClientCertExpirationDays` option for setting the expiration time for client certificates. [#12172](https://github.com/deckhouse/deckhouse/pull/12172)
 - **[openvpn]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[prometheus]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[prometheus]** Added a new Grafana plugin `esnet-matrix-panel`. [#11960](https://github.com/deckhouse/deckhouse/pull/11960)
 - **[upmeter]** Added the `auth.allowedUserEmails` option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[user-authn]** Updated `dex-authenticator` to v2.42.0. [#12357](https://github.com/deckhouse/deckhouse/pull/12357)
 - **[user-authn]** Added support for restricting access based on user email in DexClient and DexAuthenticator. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[user-authz]** Added dict support for VirtualMachineClass and ClusterVirtualImage resources. [#12476](https://github.com/deckhouse/deckhouse/pull/12476)
 - **[user-authz]** Added support for dict roles so that namespace-bound users could access shared cluster-wide resources via automatically generated ClusterRoleBindings. [#11943](https://github.com/deckhouse/deckhouse/pull/11943)

## Fixes


 - **[admission-policy-engine]** Excluded the `virtualization` service from the validation by `admission-policy-engine`. [#12803](https://github.com/deckhouse/deckhouse/pull/12803)
 - **[admission-policy-engine]** Changed defaults for `requiredRequests` in OperationPolicy. [#12251](https://github.com/deckhouse/deckhouse/pull/12251)
 - **[admission-policy-engine]** Fixed the behavior when there could be `No data` messages if some metrics couldn't be exported. [#11847](https://github.com/deckhouse/deckhouse/pull/11847)
 - **[candi]** Increased `network_dhcp_wait_seconds` in the `vcd_vapp_vm` resource from 60 to 120 seconds. [#12640](https://github.com/deckhouse/deckhouse/pull/12640)
 - **[candi]** Deleted unnecessary error warnings by the sysctl tuner. [#12297](https://github.com/deckhouse/deckhouse/pull/12297)
 - **[candi]** Disabled old or another `containerd.service` from running to prevent an update freeze. [#10025](https://github.com/deckhouse/deckhouse/pull/10025)
 - **[cert-manager]** Restored the original webhook name to match the regexp from the `cert-manager` library. [#12129](https://github.com/deckhouse/deckhouse/pull/12129)
 - **[cloud-provider-aws]** Set `cloudProviderAws.internal` values individually in the `provider_cluster_configuration` hook. [#12407](https://github.com/deckhouse/deckhouse/pull/12407)
 - **[cloud-provider-azure]** Disabled API call caching in `cloud-controller-manager`. [#12445](https://github.com/deckhouse/deckhouse/pull/12445)
 - **[cloud-provider-dynamix]** Fixed the Terraform `cloudConfig` decoding. [#12493](https://github.com/deckhouse/deckhouse/pull/12493)
 - **[cloud-provider-huaweicloud]** Fixed `EIP` creation in `cloud-controller-manager`. [#12046](https://github.com/deckhouse/deckhouse/pull/12046)
 - **[cloud-provider-openstack]** Fixed empty metadata fields. [#12179](https://github.com/deckhouse/deckhouse/pull/12179)
 - **[cloud-provider-vcd]** Add a hook to set `legacyMode` based on the detected `VCD API` version [#13015](https://github.com/deckhouse/deckhouse/pull/13015)
 - **[cloud-provider-vcd]** Fixed creation of PersistentVolumeClaim. [#12909](https://github.com/deckhouse/deckhouse/pull/12909)
 - **[cloud-provider-vcd]** Implemented a hack to migrate etcd disk to VCD independent disk to prevent deletion of etcd data. [#12651](https://github.com/deckhouse/deckhouse/pull/12651)
    To migrate, you must perform a `converge`, which causes the master server to be recreated. If you are using only one master server with the manual address assignment via the `mainNetworkIPAddresses` parameter, add two more IP addresses for the migration process.
 - **[cloud-provider-vcd]** Added a patch for `cloud-controller-manager` that ignores a node if `providerID` has the `static://` prefix. [#12556](https://github.com/deckhouse/deckhouse/pull/12556)
 - **[cloud-provider-vcd]** Enabled support for legacy API versions below `37.2`. [#12361](https://github.com/deckhouse/deckhouse/pull/12361)
 - **[cloud-provider-vsphere]** Fixed null StorageClasses in vSphere when StorageClasses are excluded from discovery. [#12597](https://github.com/deckhouse/deckhouse/pull/12597)
 - **[cloud-provider-yandex]** fix synchronizing TGs [#13521](https://github.com/deckhouse/deckhouse/pull/13521)
 - **[cloud-provider-yandex]** fix WithNATInstance clusters converge [#13189](https://github.com/deckhouse/deckhouse/pull/13189)
 - **[cloud-provider-yandex]** Fixed LoadBalancer service creation in hybrid clusters. [#12491](https://github.com/deckhouse/deckhouse/pull/12491)
 - **[cloud-provider-zvirt]** Fixed zvirt-csi-driver patching (token refresh fix patch). [#12313](https://github.com/deckhouse/deckhouse/pull/12313)
 - **[cni-cilium]** Fixed race condition when deleting element from ipcache map during VM migration. [#13344](https://github.com/deckhouse/deckhouse/pull/13344)
 - **[control-plane-manager]** audit-log-maxage value to 30 days [#14056](https://github.com/deckhouse/deckhouse/pull/14056)
 - **[control-plane-manager]** Stale service account alert fix. [#13129](https://github.com/deckhouse/deckhouse/pull/13129)
 - **[control-plane-manager]** Fixed `additionalAPIIssuers` and `additionalAPIAudiences` parameters and associated tests. [#12576](https://github.com/deckhouse/deckhouse/pull/12576)
 - **[control-plane-manager]** Fixed `d8-control-plane-manager` containers that were duplicated when updating Kubernetes. [#12561](https://github.com/deckhouse/deckhouse/pull/12561)
 - **[control-plane-manager]** Removed `etcd.externalMembersNames` from ModuleConfig settings. [#12422](https://github.com/deckhouse/deckhouse/pull/12422)
 - **[control-plane-manager]** Fixed the behavior when an etcd member couldn't be promoted from learner state if needed. [#11934](https://github.com/deckhouse/deckhouse/pull/11934)
 - **[dashboard]** fix display workload metrics on dashboard [#13604](https://github.com/deckhouse/deckhouse/pull/13604)
 - **[deckhouse]** Fix obsolete config alert stuck after deleting module config. [#13853](https://github.com/deckhouse/deckhouse/pull/13853)
 - **[deckhouse]** Gracefully restore deployed modules on HA installations. [#13077](https://github.com/deckhouse/deckhouse/pull/13077)
 - **[deckhouse]** Removed duplicated alerts in CNI modules. [#12220](https://github.com/deckhouse/deckhouse/pull/12220)
 - **[deckhouse-controller]** Added validation according to `RFC 1123` for module names added to `ModuleSource`. [#13644](https://github.com/deckhouse/deckhouse/pull/13644)
 - **[deckhouse-controller]** Fix runtime handling for the global config. [#12985](https://github.com/deckhouse/deckhouse/pull/12985)
 - **[deckhouse-tools]** Removed unnecessary secrets and fixed the Deckhouse CLI build. [#12290](https://github.com/deckhouse/deckhouse/pull/12290)
 - **[dhctl]** <Disable caching metaconfig during converge and disable converge deckhouse manifests. [#13230](https://github.com/deckhouse/deckhouse/pull/13230)
 - **[dhctl]** Fix incorrect behavior that fails sudoers preflight check if password contains whitespaces. [#13140](https://github.com/deckhouse/deckhouse/pull/13140)
 - **[dhctl]** fix bootstrap and abort config preparation [#13008](https://github.com/deckhouse/deckhouse/pull/13008)
 - **[dhctl]** Fixed the timeout value when downloading packets. [#12819](https://github.com/deckhouse/deckhouse/pull/12819)
 - **[dhctl]** Added a cleanup of failed or completed Deckhouse pods due to bootstrap. [#12798](https://github.com/deckhouse/deckhouse/pull/12798)
 - **[dhctl]** Added minimal preflight checks to the abort at a bootstrap phase. [#12562](https://github.com/deckhouse/deckhouse/pull/12562)
 - **[dhctl]** Fixed the CloudPermanent node converge process so that it must be drained before removal. [#12389](https://github.com/deckhouse/deckhouse/pull/12389)
 - **[dhctl]** Fixed the behavior when `SudoPassword` from the connection-config wasn't used in dhctl CLI and dhctl server. [#12368](https://github.com/deckhouse/deckhouse/pull/12368)
 - **[dhctl]** Improved logging and operation when performing `converge`. [#11926](https://github.com/deckhouse/deckhouse/pull/11926)
 - **[ingress-nginx]** All necessary shared libraries are added to the container image. [#13124](https://github.com/deckhouse/deckhouse/pull/13124)
    Ingress-nginx controller pods of v1.9 will be restated.
 - **[ingress-nginx]** Fixed patch names in `ingress-nginx`. [#12633](https://github.com/deckhouse/deckhouse/pull/12633)
 - **[ingress-nginx]** Fixed security vulnerabilities. [#12449](https://github.com/deckhouse/deckhouse/pull/12449)
 - **[istio]** The `alliance.ingressGateway.advertise` option was fixed up. [#13924](https://github.com/deckhouse/deckhouse/pull/13924)
 - **[istio]** proxy-buffer-size increased in kiali Ingress. [#13721](https://github.com/deckhouse/deckhouse/pull/13721)
 - **[istio]** Added Kubernetes version check in a Helm chart. [#12503](https://github.com/deckhouse/deckhouse/pull/12503)
 - **[istio]** Refactored secure api-proxy for multiclusters to improve reliability. [#12196](https://github.com/deckhouse/deckhouse/pull/12196)
 - **[keepalived]** fix python [#13617](https://github.com/deckhouse/deckhouse/pull/13617)
 - **[kube-dns]** Fixed release upgrade issue with removed d8-kube-dns-redirect service. [#13487](https://github.com/deckhouse/deckhouse/pull/13487)
 - **[kube-dns]** Expanded pattern for `stubZones` to allow underscores (`_`) in domain names. [#13118](https://github.com/deckhouse/deckhouse/pull/13118)
 - **[loki]** Fix several bugs introduced while implementing loki disk based retention [#14025](https://github.com/deckhouse/deckhouse/pull/14025)
    loki statefulset will be rollout restarted
 - **[loki]** Fix incorrect indices sort function used in disk-based retention. [#13861](https://github.com/deckhouse/deckhouse/pull/13861)
    Prevent important data loss when using wide retention intervals.
 - **[loki]** fix storage capacity calculator hook for Loki [#13003](https://github.com/deckhouse/deckhouse/pull/13003)
    fixes the bug introduced in v1.69.0
 - **[metallb]** Reverted the fix for handling deleted Services and revising the IP pools in L2 mode. The correct fix is under construction. [#13836](https://github.com/deckhouse/deckhouse/pull/13836)
    In L2 mode, the Public IP address will not be marked as free in the pool after deleting the LoadBalancer Service. As a temporary fix, you can restart the MetalLB controller pods.
 - **[metallb]** Fixed deprecated CRD addresspools.metallb.io deletion. [#13553](https://github.com/deckhouse/deckhouse/pull/13553)
 - **[metallb]** Fixed IP pool exhaustion on LoadBalancer deletion. [#13352](https://github.com/deckhouse/deckhouse/pull/13352)
 - **[network-gateway]** Fix python path [#13574](https://github.com/deckhouse/deckhouse/pull/13574)
 - **[node-manager]** fixed update for static clusters [#13962](https://github.com/deckhouse/deckhouse/pull/13962)
 - **[node-manager]** Revert cluster autoscaler [#13416](https://github.com/deckhouse/deckhouse/pull/13416)
 - **[node-manager]** Autoscaler - remove additional cordon node by mcm provider. [#13391](https://github.com/deckhouse/deckhouse/pull/13391)
 - **[node-manager]** Increase verbosity and revert versions and fix bug with unsupported mcm annotation [#13180](https://github.com/deckhouse/deckhouse/pull/13180)
 - **[node-manager]** Fixed kubeconfig generation for `CAPI`. [#12554](https://github.com/deckhouse/deckhouse/pull/12554)
 - **[node-manager]** Improved `handleDraining` hook to ignore timeout errors during node draining. [#12542](https://github.com/deckhouse/deckhouse/pull/12542)
 - **[node-manager]** Added validation of `instanceClass` deletion for being used by a NodeGroup. [#11830](https://github.com/deckhouse/deckhouse/pull/11830)
 - **[operator-trivy]** Add proxy env variables support to the trivy server. [#13036](https://github.com/deckhouse/deckhouse/pull/13036)
 - **[prometheus]** enable WAL for the grafana SQLite database to prevent locking errors, thus fixing in-dashboard alerting. [#13063](https://github.com/deckhouse/deckhouse/pull/13063)
    the grafana deployment will be rollout restarted
 - **[prometheus]** Fixed security vulnerabilities in Grafana. [#12062](https://github.com/deckhouse/deckhouse/pull/12062)
 - **[prometheus]** Fixed security vulnerabilities in `mimir` and `promxy`. [#11978](https://github.com/deckhouse/deckhouse/pull/11978)
 - **[runtime-audit-engine]** Fixed built-in rules for `runtime-audit-engine`. [#12486](https://github.com/deckhouse/deckhouse/pull/12486)
 - **[runtime-audit-engine]** Added support for customization of a built-in rule list of the `runtime-audit-engine` module. [#12185](https://github.com/deckhouse/deckhouse/pull/12185)
 - **[service-with-healthchecks]** Removed unnecessary text data from the executable binary file. [#12492](https://github.com/deckhouse/deckhouse/pull/12492)
 - **[upmeter]** upmeter sa token rotation [#13201](https://github.com/deckhouse/deckhouse/pull/13201)
 - **[user-authn]** Add separate ServiceAccount to basic-auth-proxy. [#13214](https://github.com/deckhouse/deckhouse/pull/13214)
 - **[user-authn]** Fix secret generation on empty data field in the dex client app secret. [#13092](https://github.com/deckhouse/deckhouse/pull/13092)
 - **[user-authn]** Added security context with RuntimeDefault seccomp profile to dex container. [#12197](https://github.com/deckhouse/deckhouse/pull/12197)
 - **[vertical-pod-autoscaler]** Enabled the memory-save option to prevent the VPA recommender from consuming excessive amounts of RAM. [#12077](https://github.com/deckhouse/deckhouse/pull/12077)

## Chore


 - **[candi]** Update Deckhouse CLI to 0.12.1 [#13887](https://github.com/deckhouse/deckhouse/pull/13887)
 - **[candi]** Fixed the priority for `kubernetes_api_proxy`. [#12678](https://github.com/deckhouse/deckhouse/pull/12678)
 - **[candi]** Bumped patch versions of Kubernetes images: `v1.29.14`, `v1.30.1`, `v1.31.6`, `v1.32.2`. [#12080](https://github.com/deckhouse/deckhouse/pull/12080)
    Kubernetes control plane components and kubelet will restart.
 - **[cloud-provider-aws]** Added support for switching between different CNIs. [#12355](https://github.com/deckhouse/deckhouse/pull/12355)
 - **[cloud-provider-azure]** Added support for switching between different CNIs. [#12355](https://github.com/deckhouse/deckhouse/pull/12355)
 - **[cloud-provider-dynamix]** Fixed vulnerabilities and some critical bugs for cloud-provider-zvirt and cloud-provider-dynamix [#13883](https://github.com/deckhouse/deckhouse/pull/13883)
 - **[cloud-provider-gcp]** Added support for switching between different CNIs. [#12355](https://github.com/deckhouse/deckhouse/pull/12355)
 - **[cloud-provider-yandex]** Added support for switching between different CNIs. [#12355](https://github.com/deckhouse/deckhouse/pull/12355)
 - **[cloud-provider-zvirt]** Fixed vulnerabilities and some critical bugs for cloud-provider-zvirt and cloud-provider-dynamix [#13883](https://github.com/deckhouse/deckhouse/pull/13883)
 - **[cloud-provider-zvirt]** Added support for switching between different CNIs. [#12355](https://github.com/deckhouse/deckhouse/pull/12355)
 - **[cni-cilium]** Added an option to enable/disable exclusive management for CNI configuration file (`cni-exclusive`). [#12403](https://github.com/deckhouse/deckhouse/pull/12403)
 - **[control-plane-manager]** fix audit policy rules for virtualization [#13666](https://github.com/deckhouse/deckhouse/pull/13666)
 - **[control-plane-manager]** add audit policy rules for virtualization [#13086](https://github.com/deckhouse/deckhouse/pull/13086)
 - **[dashboard]** Set Grafana dashboard JSON files to render into the new `ClusterObservabilityDashboard` and `ClusterObservabilityPropagatedDashboard` CRs if `observability` module is enabled. [#12614](https://github.com/deckhouse/deckhouse/pull/12614)
 - **[deckhouse]** Removed Alpine image. [#12726](https://github.com/deckhouse/deckhouse/pull/12726)
 - **[deckhouse]** Removed the manual creation of endpoints. [#12412](https://github.com/deckhouse/deckhouse/pull/12412)
 - **[deckhouse]** Set stale modules to be deleted at goroutine. [#12351](https://github.com/deckhouse/deckhouse/pull/12351)
 - **[deckhouse]** Changed Go GC settings to improve memory consumption. [#10216](https://github.com/deckhouse/deckhouse/pull/10216)
    The control plane pods will restart.
 - **[dhctl]** Added optional parallel bootstrap for `terraNodes` using the DHCTL_PARALLEL_CLOUD_PERMANENT_NODES_BOOTSTRAP environment variable (disabled by default). [#12552](https://github.com/deckhouse/deckhouse/pull/12552)
 - **[dhctl]** Added migration to strict configuration validation with schema alignment and alerts. [#12204](https://github.com/deckhouse/deckhouse/pull/12204)
 - **[dhctl]** Extensively refactored the `converge` operation. [#11086](https://github.com/deckhouse/deckhouse/pull/11086)
 - **[docs]** Added module conversion details to the internal module documentation. [#12511](https://github.com/deckhouse/deckhouse/pull/12511)
 - **[extended-monitoring]** Added an alert for when there are no available replicas remaining in Deployment `standby-holder-{{$labels.node_group_name}}` in namespace `d8-cloud-instance-manager`. [#11905](https://github.com/deckhouse/deckhouse/pull/11905)
 - **[ingress-nginx]** Fixed security vulnerabilities for the `ingress-controller-1.9` image. [#12268](https://github.com/deckhouse/deckhouse/pull/12268)
    The Ingress controller will be restarted followed by a temporary interruption of web services.
 - **[istio]** Fixed security vulnerabilities. [#11991](https://github.com/deckhouse/deckhouse/pull/11991)
 - **[log-shipper]** Removed name-based namespace selectors. [#11965](https://github.com/deckhouse/deckhouse/pull/11965)
 - **[loki]** Added new configuration parameters that were made defaults earlier in Loki 2.4.2. [#12373](https://github.com/deckhouse/deckhouse/pull/12373)
 - **[metallb]** Added a hook for deleting the deprecated CRD `addresspools.metallb.io`. [#12378](https://github.com/deckhouse/deckhouse/pull/12378)
 - **[monitoring-custom]** Excluded the `d8-observability` namespace from the `D8CustomPrometheusRuleFoundInCluster` alert. [#12788](https://github.com/deckhouse/deckhouse/pull/12788)
 - **[monitoring-kubernetes]** Restored metrics re-labeling. [#12800](https://github.com/deckhouse/deckhouse/pull/12800)
 - **[multitenancy-manager]** Modified the behavior to use empty template by default. [#12715](https://github.com/deckhouse/deckhouse/pull/12715)
 - **[node-manager]** The `get_crds` hook now fails with an error if the actual number of node groups in the cluster does not match the calculated number. [#12498](https://github.com/deckhouse/deckhouse/pull/12498)
 - **[prometheus]** Increased the scrape timeout in seconds. [#12411](https://github.com/deckhouse/deckhouse/pull/12411)
 - **[prometheus]** Increased the priority class. [#11904](https://github.com/deckhouse/deckhouse/pull/11904)
 - **[prometheus]** Bumped Prometheus version to 2.55.1. [#11426](https://github.com/deckhouse/deckhouse/pull/11426)
 - **[user-authz]** Added RBAC v2 configuration for DynamiX and Huawei Cloud providers. [#12148](https://github.com/deckhouse/deckhouse/pull/12148)

