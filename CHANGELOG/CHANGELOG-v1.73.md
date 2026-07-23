# Changelog v1.73

## [MALFORMED]


 - #14820 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #14984 unknown section "runtime-audit-engine"
 - #14995 unknown section "['cni-cilium']"
 - #15113 unknown section "runtime-audit-engine"
 - #15410 unknown section "runtime-audit-engine"
 - #16177 unknown section ""
 - #16905 invalid type "сhore"
 - #17803 unknown section "docs-builder"
 - #18950 unknown section "Fix dhctl cloud tests"
 - #19751 missing summary
 - #19751 unknown section "nodeManager"
 - #19751 unknown section "registryPackagesProxy cve fixes"
 - #20137 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #20375 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"

## Know before update


 - ALL pods of the ingress-nginx module will be restarted.
 - Cilium agents will be restarted during the update.
 - Custom edits to the local-path-config ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - Deckhouse now has privileged mode and runs as root.
 - Fixes a bug where creating new User resources failed due to missing oldObject in validation; password immutability is still enforced on update.
 - If wireguard interface is present on nodes, then cilium-agent upgrade will stuck. Upgrading the linux kernel to 6.8 is required.
 - The `local-path-provisioner` Pod is restarted during the update. Custom edits to the `local-path-config` ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected.
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected. After the update the provisioner refuses to create a HelperPod whose template (loaded from the `local-path-config` ConfigMap) declares privileged containers, hostPath/custom volumes, host namespaces, added Linux capabilities or other security-sensitive fields, so any pre-existing manual override of `helperPod.yaml` that uses one of these fields must be removed before the upgrade.
 - The runtime-audit-engine module has been moved to external. All pods of the module will be restarted.
 - This update fixes a security vulnerability in the `user-authn` module (CVE-2025-22868) that could potentially allow bypass of authentication validation.
 - This update triggers a rolling update of the flannel pods.
 - This update triggers a rolling update of the kube-proxy pods.
 - This update triggers a rolling update of the network-policy-engine pods.
 - When using containerdV2, the performance of istio-cni breaks when mounting internal paths.

## Features


 - **[admission-policy-engine]** Added OperationPolicy knob pods.disallowedTolerations and enable DELETE admission by default in Gatekeeper webhook. [#15457](https://github.com/deckhouse/deckhouse/pull/15457)
 - **[admission-policy-engine]** Added `allowRbacWildcards` SecurityPolicy flag and Gatekeeper template to restrict `*` in Role and RoleBinding. [#15567](https://github.com/deckhouse/deckhouse/pull/15567)
 - **[admission-policy-engine]** Added a filter that allows vulnerable images with only specified severity levels to run. [#16049](https://github.com/deckhouse/deckhouse/pull/16049)
 - **[admission-policy-engine]** Validate CONNECT requests for pods/exec and pods/attach in the ValidatingWebhookConfiguration. [#15872](https://github.com/deckhouse/deckhouse/pull/15872)
    Enables Gatekeeper constraints to act on CONNECT (kubectl exec) events; default behavior unchanged unless such constraints are created.
 - **[candi]** Added metadata to VCD objects such as networks, virtual machines, and disks. [#14505](https://github.com/deckhouse/deckhouse/pull/14505)
 - **[candi]** Added support of the new module csi-vsphere. [#14549](https://github.com/deckhouse/deckhouse/pull/14549)
 - **[candi]** Bump deckhouse-cli version up to v0.29.28 [#19228](https://github.com/deckhouse/deckhouse/pull/19228)
 - **[candi]** Bump deckhouse-cli version up to v0.29.29 [#19230](https://github.com/deckhouse/deckhouse/pull/19230)
 - **[cloud-provider-dvp]** Added default CNI for DVP provider. [#15084](https://github.com/deckhouse/deckhouse/pull/15084)
 - **[cloud-provider-dvp]** Added support for the additionalDisks parameter in DVPInstanceClass. [#15121](https://github.com/deckhouse/deckhouse/pull/15121)
 - **[cloud-provider-dvp]** Changed go_lib path for cse needs. [#15445](https://github.com/deckhouse/deckhouse/pull/15445)
 - **[cloud-provider-huaweicloud]** Added default CNI for huawei cloud provider. [#15097](https://github.com/deckhouse/deckhouse/pull/15097)
 - **[cloud-provider-vcd]** Added metadata to VCD objects such as networks, virtual machines, and disks. [#14505](https://github.com/deckhouse/deckhouse/pull/14505)
 - **[cloud-provider-vcd]** Enable load balancer feature. [#15934](https://github.com/deckhouse/deckhouse/pull/15934)
 - **[cloud-provider-vsphere]** Added storage_policy support. [#14786](https://github.com/deckhouse/deckhouse/pull/14786)
 - **[control-plane-manager]** Added extra claim `user-authn.deckhouse.io/dex-provider` (from `federated_claims.connector_id`) Request `federated:id` scope in Dex Authenticator, Basic Auth Proxy, and kubeconfig generator to populate. `federated_claims.connector_id` [#15816](https://github.com/deckhouse/deckhouse/pull/15816)
 - **[deckhouse-controller]** Added conversion rules exposure in ModuleSettingsDefinition object. [#15032](https://github.com/deckhouse/deckhouse/pull/15032)
    ModuleSettingsDefinition objects now include conversion rules in the conversions field, enabling users to preview how module settings will be transformed between versions.
 - **[deckhouse-controller]** Added coordination logic for Deckhouse restart operations during concurrent module deployments. [#15156](https://github.com/deckhouse/deckhouse/pull/15156)
    Deckhouse will now restart only after all concurrent module releases finish their ApplyRelease operations, reducing the number of restarts during bulk module updates and improving deployment reliability.
 - **[deckhouse-controller]** Added deckhouse release information into events. [#15547](https://github.com/deckhouse/deckhouse/pull/15547)
 - **[deckhouse-controller]** Added new objects to debug archive. [#15047](https://github.com/deckhouse/deckhouse/pull/15047)
 - **[deckhouse-controller]** Added structured webhook response contract with status codes. [#15256](https://github.com/deckhouse/deckhouse/pull/15256)
 - **[deckhouse-controller]** Added support for an LTS channel for module updates in Deckhouse. [#15321](https://github.com/deckhouse/deckhouse/pull/15321)
 - **[deckhouse-controller]** Converting a module to external module. [#14536](https://github.com/deckhouse/deckhouse/pull/14536)
    The runtime-audit-engine module has been moved to external. All pods of the module will be restarted.
 - **[deckhouse-controller]** ModuleRelease supports from/to update constraints to skip step-by-step upgrades and jump to a target release. [#14978](https://github.com/deckhouse/deckhouse/pull/14978)
    Pending releases between the deployed version and the target endpoint are automatically marked Skipped; the endpoint is processed as a minor update (respects module readiness and update windows)
 - **[deckhouse-controller]** Tuned DeckhouseHighMemoryUsage alert (namespace grouping, 0.85 threshold, 30s for). [#15543](https://github.com/deckhouse/deckhouse/pull/15543)
 - **[deckhouse-controller]** Updated addon-operator dependency to the latest version. [#14962](https://github.com/deckhouse/deckhouse/pull/14962)
 - **[deckhouse-controller]** Updated the logic of processing modules in the ModuleSource. [#14953](https://github.com/deckhouse/deckhouse/pull/14953)
 - **[deckhouse-controller]** ignore migrate module if disabled [#15145](https://github.com/deckhouse/deckhouse/pull/15145)
 - **[deckhouse]** Added Deckhouse release information status. [#15458](https://github.com/deckhouse/deckhouse/pull/15458)
 - **[deckhouse]** Added alert about new major versions count relatively to current major version. [#15278](https://github.com/deckhouse/deckhouse/pull/15278)
 - **[deckhouse]** Added alert for deprecated modules. [#15483](https://github.com/deckhouse/deckhouse/pull/15483)
 - **[deckhouse]** Added inject registry to values. [#14991](https://github.com/deckhouse/deckhouse/pull/14991)
 - **[deckhouse]** Added optional module requirements. [#15136](https://github.com/deckhouse/deckhouse/pull/15136)
 - **[deckhouse]** Made the module source `deckhouse` the default source. [#15437](https://github.com/deckhouse/deckhouse/pull/15437)
 - **[deckhouse]** Make deckhouse privileged and run as root. [#15664](https://github.com/deckhouse/deckhouse/pull/15664)
    Deckhouse now has privileged mode and runs as root.
 - **[dhctl]** Added clearer error messages when resource creation times out. [#15310](https://github.com/deckhouse/deckhouse/pull/15310)
    Improved user experience when dhctl cannot create resources due to missing worker nodes.
 - **[dhctl]** Allowed dhctl to work with readonly root fs. [#15471](https://github.com/deckhouse/deckhouse/pull/15471)
 - **[docs]** Added manifest for internal LB VK Cloud. [#16057](https://github.com/deckhouse/deckhouse/pull/16057)
 - **[docs]** Added new documentation structure. [#12192](https://github.com/deckhouse/deckhouse/pull/12192)
 - **[documentation]** Bump HuGo to v0.150.1 [#12192](https://github.com/deckhouse/deckhouse/pull/12192)
 - **[ingress-nginx]** Added NGINX memory profiling for the Ingress controller. [#14736](https://github.com/deckhouse/deckhouse/pull/14736)
 - **[ingress-nginx]** The metric `geoip_errors_total` is added, indicating the number of errors when downloading geo ip databases from the MaxMind service. [#14889](https://github.com/deckhouse/deckhouse/pull/14889)
    Ingress-controller pods will restart.
 - **[ingress-nginx]** Updated Nginx versions of NGINX Ingress Controller 1.10 and 1.12 to version 1.26.1. [#16476](https://github.com/deckhouse/deckhouse/pull/16476)
    NGINX Ingress Controller pods of versions 1.10 and 1.12 will be restarted.
 - **[istio]** Added PSS restriction for api-proxy and  ingressgateway. [#15791](https://github.com/deckhouse/deckhouse/pull/15791)
 - **[istio]** Added access log format setting for proxy sidecars. [#16129](https://github.com/deckhouse/deckhouse/pull/16129)
 - **[istio]** Allow custom ports in metadataEndpoint URLs for IstioFederation and IstioMulticluster CRDs. [#19323](https://github.com/deckhouse/deckhouse/pull/19323)
 - **[istio]** fixing the CVE in Kiali [#17096](https://github.com/deckhouse/deckhouse/pull/17096)
 - **[node-manager]** Add and increase lease duration timeouts to CAPS. [#15349](https://github.com/deckhouse/deckhouse/pull/15349)
 - **[node-manager]** Backported per-GPU custom MIG configurations via `customConfigs` with `partedConfig: custom` support. [#18999](https://github.com/deckhouse/deckhouse/pull/18999)
 - **[node-manager]** Disabled update system packages index during boot cloud ephemeral nodes. [#15859](https://github.com/deckhouse/deckhouse/pull/15859)
 - **[registry]** Added configurable unmanaged mode. [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[registry]** Added registry default and relax check modes. [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[registry]** Disable change registry helper when registry module is enabled. [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[upmeter]** Added the ability to pass headers in RW. [#15533](https://github.com/deckhouse/deckhouse/pull/15533)
 - **[user-authn]** Add DexProvider .spec.enabled flag (default true) and kubectl printer columns; skip disabled providers in Dex connectors. [#16007](https://github.com/deckhouse/deckhouse/pull/16007)
 - **[user-authn]** Add UserOperation CR (Lock, Unlock, ResetPassword, Reset2FA) for local users and Lock/Unlock for external password-connector accounts. [#19664](https://github.com/deckhouse/deckhouse/pull/19664)
 - **[user-authn]** Added implement password policy logic for local user accounts. Now it is possible to set complexity level of passwords, failed attempts number to block the user, keep password history and force renewing the password after specified amount of time. [#14993](https://github.com/deckhouse/deckhouse/pull/14993)
 - **[user-authn]** Dex can run even if one of OIDC providers is not reachable. It resolves the issue when a single unreachable provider can compromise authentication in the cluster. [#15379](https://github.com/deckhouse/deckhouse/pull/15379)
 - **[user-authn]** Enforce lowercase emails for User on create and on email changes; case-insensitive email uniqueness; backward-compatible for legacy uppercase emails. [#15960](https://github.com/deckhouse/deckhouse/pull/15960)
 - **[user-authn]** Increase Dex AuthRequest flexibility with token-bucket rate-limiting and global ResourceQuota. [#15421](https://github.com/deckhouse/deckhouse/pull/15421)
 - **[user-authn]** Propagate proxy envs to dex to allow requesting OIDC discovery endpoints in closed environments. [#15292](https://github.com/deckhouse/deckhouse/pull/15292)

## Fixes


 - **[admission-policy-engine]** Fix CVE. [#15966](https://github.com/deckhouse/deckhouse/pull/15966)
 - **[admission-policy-engine]** Fix cve for ratify [#18927](https://github.com/deckhouse/deckhouse/pull/18927)
 - **[admission-policy-engine]** Fixed GHSA-vrw8-fxc6-2r93. [#16037](https://github.com/deckhouse/deckhouse/pull/16037)
 - **[admission-policy-engine]** Fixed proxy support for trivy-provider [#16113](https://github.com/deckhouse/deckhouse/pull/16113)
 - **[admission-policy-engine]** Prohibit only creation or modification for objects with vulnerable images [#16134](https://github.com/deckhouse/deckhouse/pull/16134)
 - **[admission-policy-engine]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[admission-policy-engine]** added mount-points in ratify [#17925](https://github.com/deckhouse/deckhouse/pull/17925)
 - **[admission-policy-engine]** cve fixes for ratify [#19274](https://github.com/deckhouse/deckhouse/pull/19274)
 - **[basic-auth]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[candi]** Add cve patches for admission-policy-engine, cert-manager [#18566](https://github.com/deckhouse/deckhouse/pull/18566)
 - **[candi]** Add missing debian 13 version to detect_bundle [#16617](https://github.com/deckhouse/deckhouse/pull/16617)
 - **[candi]** Added ContainerdV2 case in tpl kubelet configuration. [#15850](https://github.com/deckhouse/deckhouse/pull/15850)
 - **[candi]** Added exit 1 for check_containerd_v2_support step if set_labels() func failure. [#15792](https://github.com/deckhouse/deckhouse/pull/15792)
 - **[candi]** Added fallback to dnf package manager from yum install and remove bashbooster func's. [#17012](https://github.com/deckhouse/deckhouse/pull/17012)
 - **[candi]** Added missing volumeTypeMap property for nodeGroups. [#15144](https://github.com/deckhouse/deckhouse/pull/15144)
 - **[candi]** Disable immutable flag on erofs files in cleanup node stage. [#15520](https://github.com/deckhouse/deckhouse/pull/15520)
 - **[candi]** Fix arg `encryption-provider-config ` for kubeadm configuration. [#15521](https://github.com/deckhouse/deckhouse/pull/15521)
 - **[candi]** Fix cve for admission-policy-engine, cert-manager [#19881](https://github.com/deckhouse/deckhouse/pull/19881)
 - **[candi]** Fix cve for admission-policy-engine, cert-manager, kube-rbac-proxy [#18790](https://github.com/deckhouse/deckhouse/pull/18790)
 - **[candi]** Fixed dhctl skipping errors when it's fetching packages. [#15971](https://github.com/deckhouse/deckhouse/pull/15971)
 - **[candi]** Fixed segfault in mkfs.erofs. [#15715](https://github.com/deckhouse/deckhouse/pull/15715)
 - **[candi]** Improved check another containerd service on first run. [#15902](https://github.com/deckhouse/deckhouse/pull/15902)
 - **[candi]** Made kubectl-exec a direct request to the control plane if the proxy is unavailable. [#15279](https://github.com/deckhouse/deckhouse/pull/15279)
 - **[candi]** Reduce auditd pressure around containerd to avoid kernel soft lockups on Linux 5.x nodes. [#15986](https://github.com/deckhouse/deckhouse/pull/15986)
 - **[candi]** Switch node hostname in discover_node_ip.sh to locked hostname [#15799](https://github.com/deckhouse/deckhouse/pull/15799)
 - **[candi]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[candi]** Updated pwru tool to v1.0.11 to fix CVE-2025-68121. [#17975](https://github.com/deckhouse/deckhouse/pull/17975)
 - **[candi]** Updated the bashible step to include Linux kernel versions that address CVE-2025-37999 [#17300](https://github.com/deckhouse/deckhouse/pull/17300)
 - **[candi]** cve fix for admission-policy-engine, cert-manager, user-authn, multitenancy-manager [#18171](https://github.com/deckhouse/deckhouse/pull/18171)
 - **[candi]** cve fix for user-authn module [#18679](https://github.com/deckhouse/deckhouse/pull/18679)
 - **[candi]** fallback to dnf package manager from yum install and remove bashbooster func's [#17061](https://github.com/deckhouse/deckhouse/pull/17061)
 - **[candi]** fix cve node-manager and opentofu. [#19947](https://github.com/deckhouse/deckhouse/pull/19947)
 - **[candi]** fix vex for admission-policy-engine and operator-trivy modules [#20004](https://github.com/deckhouse/deckhouse/pull/20004)
 - **[candi]** increase runtimeRequestTimeout for kubelet [#19511](https://github.com/deckhouse/deckhouse/pull/19511)
 - **[cert-manager]** Mark module as critical for cluster [#16078](https://github.com/deckhouse/deckhouse/pull/16078)
 - **[cert-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[chrony]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[chrony]** mitigate CVE-2025-58181 [#17958](https://github.com/deckhouse/deckhouse/pull/17958)
 - **[cilium-hubble]** Fixed CVE-2026-29181 in hubble-ui-backend  by bumping OpenTelemetry Go to v1.41.0 [#20263](https://github.com/deckhouse/deckhouse/pull/20263)
 - **[cilium-hubble]** Fixed CVE-2026-33186 in the hubble-ui image. [#18721](https://github.com/deckhouse/deckhouse/pull/18721)
 - **[cilium-hubble]** Fixed CVE-2026-41520 in hubble-ui-backend [#20362](https://github.com/deckhouse/deckhouse/pull/20362)
 - **[cilium-hubble]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-aws]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-aws]** fix ssh access sg creation with disableDefaultSecurityGroup passed [#16081](https://github.com/deckhouse/deckhouse/pull/16081)
 - **[cloud-provider-azure]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-azure]** fix build image for azure ccm [#16560](https://github.com/deckhouse/deckhouse/pull/16560)
 - **[cloud-provider-dvp]** Added functionality to wait for a disk to be attached to a VM [#16965](https://github.com/deckhouse/deckhouse/pull/16965)
 - **[cloud-provider-dvp]** Added sshPublicKey to registration secret. [#15859](https://github.com/deckhouse/deckhouse/pull/15859)
 - **[cloud-provider-dvp]** Correct the calculation of the path to the device [#16212](https://github.com/deckhouse/deckhouse/pull/16212)
 - **[cloud-provider-dvp]** Fix CVE-2025-22869 && CVE-2024-45337. [#15390](https://github.com/deckhouse/deckhouse/pull/15390)
 - **[cloud-provider-dvp]** Fix healthCheckNodePort collisions [#16996](https://github.com/deckhouse/deckhouse/pull/16996)
 - **[cloud-provider-dvp]** Fixed CVE-2025-22868. [#15396](https://github.com/deckhouse/deckhouse/pull/15396)
 - **[cloud-provider-dvp]** Fixed CVE-2025-22870 && CVE-2025-22872. [#15730](https://github.com/deckhouse/deckhouse/pull/15730)
 - **[cloud-provider-dvp]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-dvp]** add missing field to Cluster [#16438](https://github.com/deckhouse/deckhouse/pull/16438)
 - **[cloud-provider-dvp]** fix cve [#19057](https://github.com/deckhouse/deckhouse/pull/19057)
 - **[cloud-provider-dvp]** fix single-node LoadBalancer bug and add multi-LB per node support [#14883](https://github.com/deckhouse/deckhouse/pull/14883)
 - **[cloud-provider-dvp]** fixe cve [#18599](https://github.com/deckhouse/deckhouse/pull/18599)
 - **[cloud-provider-dvp]** fixed CVE [#16810](https://github.com/deckhouse/deckhouse/pull/16810)
 - **[cloud-provider-dvp]** make DVP cloud-init provisioning secret creation idempotent to avoid `secret already exists` reconciliation failures [#20168](https://github.com/deckhouse/deckhouse/pull/20168)
 - **[cloud-provider-dvp]** update cloud-controller-manager dependencies and VEX metadata to address current CVE findings [#19272](https://github.com/deckhouse/deckhouse/pull/19272)
 - **[cloud-provider-dynamix]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-gcp]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-huaweicloud]** Added missing volumeTypeMap property for nodeGroups. [#15144](https://github.com/deckhouse/deckhouse/pull/15144)
 - **[cloud-provider-huaweicloud]** Fixed providerID format and exclude 127.0.0.0/8 in node IP selection. [#15183](https://github.com/deckhouse/deckhouse/pull/15183)
 - **[cloud-provider-huaweicloud]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-huaweicloud]** add rule to d8:cloud-provider-huaweicloud:cloud-controller-manager account for list pods as needed [#15678](https://github.com/deckhouse/deckhouse/pull/15678)
 - **[cloud-provider-huaweicloud]** fix CSI unpublishValidation for non exist ECS instance [#16992](https://github.com/deckhouse/deckhouse/pull/16992)
 - **[cloud-provider-huaweicloud]** fix Provider ID [#16705](https://github.com/deckhouse/deckhouse/pull/16705)
 - **[cloud-provider-openstack]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-openstack]** fix openstack ccm deployment [#15601](https://github.com/deckhouse/deckhouse/pull/15601)
 - **[cloud-provider-vcd]** Add ability to define affinity rules for VCD VMs. [#15331](https://github.com/deckhouse/deckhouse/pull/15331)
 - **[cloud-provider-vcd]** Added validation to provider.server in VCDClusterConfiguration to ensure it does not end with '/'. [#15185](https://github.com/deckhouse/deckhouse/pull/15185)
 - **[cloud-provider-vcd]** Fixed fetching VM templates from organization catalogs without direct access to organizastion. [#14980](https://github.com/deckhouse/deckhouse/pull/14980)
 - **[cloud-provider-vcd]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-vsphere]** cloud-data-discoverer fixes [#15507](https://github.com/deckhouse/deckhouse/pull/15507)
 - **[cloud-provider-vsphere]** fix cloud-data-discoverer (SPBM) [#16589](https://github.com/deckhouse/deckhouse/pull/16589)
 - **[cloud-provider-vsphere]** fix hook logick [#16028](https://github.com/deckhouse/deckhouse/pull/16028)
 - **[cloud-provider-vsphere]** fix stale session for cloud-data-discoverer [#17089](https://github.com/deckhouse/deckhouse/pull/17089)
 - **[cloud-provider-vsphere]** fix vSphere storageClass template [#16275](https://github.com/deckhouse/deckhouse/pull/16275)
 - **[cloud-provider-yandex]** Change machine drain logic and keeps it in LB before drain. [#15255](https://github.com/deckhouse/deckhouse/pull/15255)
 - **[cloud-provider-yandex]** Terraform auto converger was failed for WithNATInstance layout. [#16427](https://github.com/deckhouse/deckhouse/pull/16427)
 - **[cloud-provider-yandex]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-yandex]** Updated yandex-csi-plugin, set CSI driver metadata querying timeouts. [#15054](https://github.com/deckhouse/deckhouse/pull/15054)
 - **[cloud-provider-yandex]** cloud-provider-yandex CVE's was fixed [#16611](https://github.com/deckhouse/deckhouse/pull/16611)
 - **[cni-cilium]** Add a compatibility check for the Cilium version and the kernel version, if WireGuard is installed on the node [#15155](https://github.com/deckhouse/deckhouse/pull/15155)
    If wireguard interface is present on nodes, then cilium-agent upgrade will stuck. Upgrading the linux kernel to 6.8 is required.
 - **[cni-cilium]** Added a migration mechanism, which was implemented through the node group disruptive updates with approval. [#14977](https://github.com/deckhouse/deckhouse/pull/14977)
 - **[cni-cilium]** Fix issue in generating CiliumEgressGatewayPolicy CR. [#17949](https://github.com/deckhouse/deckhouse/pull/17949)
    All current connections powered by EgressGateways will be terminated.
 - **[cni-cilium]** Fixed CVE-2026-33186, CVE-2026-27142, and CVE-2026-27139 by updating grpc dependency and Go version, and resolved build compatibility issues. [#18616](https://github.com/deckhouse/deckhouse/pull/18616)
 - **[cni-cilium]** Fixed CVE-2026-41520 for cilium-bugtool util [#20067](https://github.com/deckhouse/deckhouse/pull/20067)
 - **[cni-cilium]** Fixed egress gateway reselection for case node hard reset. [#15090](https://github.com/deckhouse/deckhouse/pull/15090)
 - **[cni-cilium]** Fixed the infinite loop in the "cilium migration" bashible step and improved synchronization between bashible and safe-agent-updater. [#15262](https://github.com/deckhouse/deckhouse/pull/15262)
 - **[cni-cilium]** Some issues have been fixed in the EgressGateway. [#16479](https://github.com/deckhouse/deckhouse/pull/16479)
 - **[cni-cilium]** The MTU configuration has been updated. [#16751](https://github.com/deckhouse/deckhouse/pull/16751)
    The MTU will be updated on all interfaces of all pods.
 - **[cni-cilium]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cni-cilium]** Updated go-jose dependency to v4.1.4 to fix CVE-2026-34986. [#19013](https://github.com/deckhouse/deckhouse/pull/19013)
    Cilium agents will be restarted during the update.
 - **[cni-cilium]** improved migration to 1.17 logic [#15602](https://github.com/deckhouse/deckhouse/pull/15602)
 - **[cni-flannel]** Fixed CVE-2026-33186 by updating google.golang.org/grpc in flanneld. [#19107](https://github.com/deckhouse/deckhouse/pull/19107)
    This update triggers a rolling update of the flannel pods.
 - **[cni-flannel]** Used mode HostGW as default for podNetworkMode. [#15710](https://github.com/deckhouse/deckhouse/pull/15710)
 - **[cni-simple-bridge]** Refactored python image source and pip exclusion. [#19148](https://github.com/deckhouse/deckhouse/pull/19148)
 - **[common]** Fixed CVE-2026-24051 in the CoreDNS image. [#18615](https://github.com/deckhouse/deckhouse/pull/18615)
 - **[common]** Fixed CVE-2026-33186 in the CoreDNS image. [#18724](https://github.com/deckhouse/deckhouse/pull/18724)
    CoreDNS pods will undergo a rolling restart.
 - **[common]** Fixed CVE-2026-40898 in CoreDNS by updating the quic-go dependency. [#20769](https://github.com/deckhouse/deckhouse/pull/20769)
 - **[common]** Latest CVEs are fixed. [#17222](https://github.com/deckhouse/deckhouse/pull/17222)
    All pods running kube-rbac-proxy will be restarted.
 - **[common]** Removed Python completely from the debug-container image as it is no longer needed, resolving corresponding CVEs, and silenced false positives for etcd binaries via VEX. [#18845](https://github.com/deckhouse/deckhouse/pull/18845)
 - **[common]** fix cve's in docker-registry docker_auth image. [#19360](https://github.com/deckhouse/deckhouse/pull/19360)
 - **[common]** fixed CVE-2026-29181 in the CoreDNS [#20261](https://github.com/deckhouse/deckhouse/pull/20261)
 - **[control-plane-manager]** Add vex for CVE-2025-31133, CVE-2025-52881 . [#16337](https://github.com/deckhouse/deckhouse/pull/16337)
 - **[control-plane-manager]** Allow change labels and annotations for secret d8-secret-encryption-key [#20150](https://github.com/deckhouse/deckhouse/pull/20150)
 - **[control-plane-manager]** Append audit policies for virtualization before appending custom policies from Secret. [#15603](https://github.com/deckhouse/deckhouse/pull/15603)
 - **[control-plane-manager]** Fix  “etcd join” phase for control-plane scaling in v1.33. [#16660](https://github.com/deckhouse/deckhouse/pull/16660)
    Allows scaling control-plane from 1→3 in clusters where ControlPlaneKubeletLocalMode=true.
 - **[control-plane-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[control-plane-manager]** upgrade etcd to 3.6.7. [#17535](https://github.com/deckhouse/deckhouse/pull/17535)
    etcd will restart.
 - **[dashboard]** Fixed CVE-2025-22868, CVE-2025-22870, CVE-2025-22872, CVE-2025-47914, CVE-2025-58181 [#17243](https://github.com/deckhouse/deckhouse/pull/17243)
 - **[dashboard]** Fixed CVE-2025-30204 by updating dashboard components [#16927](https://github.com/deckhouse/deckhouse/pull/16927)
 - **[dashboard]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[deckhouse-controller]** A module that conditionally depends on another is no longer disabled when an incompatible version of that dependency is enabled; the enable is rejected instead. [#20341](https://github.com/deckhouse/deckhouse/pull/20341)
 - **[deckhouse-controller]** Ensure `afterDeleteHelm` hooks receive Kubernetes snapshots by stopping monitors after hook execution. [#15617](https://github.com/deckhouse/deckhouse/pull/15617)
 - **[deckhouse-controller]** Fix "multiple readiness hooks found" error on hook registration retry after failure. [#16776](https://github.com/deckhouse/deckhouse/pull/16776)
 - **[deckhouse-controller]** Fix conversions for external modules [#16772](https://github.com/deckhouse/deckhouse/pull/16772)
 - **[deckhouse-controller]** Fixed Deckhouse update accidentally minor skip. [#16096](https://github.com/deckhouse/deckhouse/pull/16096)
 - **[deckhouse-controller]** Fixed an issue where modules enabled through ModuleManager after migration bypassed ModuleConfig release validation. [#16673](https://github.com/deckhouse/deckhouse/pull/16673)
 - **[deckhouse-controller]** Fixed bug with re-enabled module using old values. [#15045](https://github.com/deckhouse/deckhouse/pull/15045)
 - **[deckhouse-controller]** Fixed panic on snapshotIter. [#15385](https://github.com/deckhouse/deckhouse/pull/15385)
 - **[deckhouse-controller]** Fixed verifying migrated modules [#16693](https://github.com/deckhouse/deckhouse/pull/16693)
 - **[deckhouse-controller]** Implement structured releaseQueueDepth calculation with hierarchical version delta tracking. [#15031](https://github.com/deckhouse/deckhouse/pull/15031)
    The releaseQueueDepth metric now accurately reflects actionable release gaps with patch version normalization; major version tracking added for future alerting.
 - **[deckhouse-controller]** Reduce deckhouse-controller startup time by optimizing file operations and making cleanup asynchronous. [#15250](https://github.com/deckhouse/deckhouse/pull/15250)
 - **[deckhouse-controller]** Reduced memory limit floor for falco containers. [#15301](https://github.com/deckhouse/deckhouse/pull/15301)
 - **[deckhouse-controller]** fix conversion applying for external modules [#16656](https://github.com/deckhouse/deckhouse/pull/16656)
 - **[deckhouse-tools]** Added tolerations support to DexAuthenticator configuration. [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[deckhouse-tools]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[deckhouse]** Add VEX entries for CVEs. [#19671](https://github.com/deckhouse/deckhouse/pull/19671)
 - **[deckhouse]** Added fixes for resources to nelm usage in DKP. [#15915](https://github.com/deckhouse/deckhouse/pull/15915)
 - **[deckhouse]** Automatically set node ip to deckhouse pod during bootstrap phase to no_proxy env. [#15978](https://github.com/deckhouse/deckhouse/pull/15978)
 - **[deckhouse]** Fix CVEs and update VEX entries. [#19233](https://github.com/deckhouse/deckhouse/pull/19233)
 - **[deckhouse]** Fix CVEs in docs-builder and add cryptography VEX. [#19343](https://github.com/deckhouse/deckhouse/pull/19343)
 - **[deckhouse]** Fix CVEs in webhook-handler and cleanup VEX files. [#19033](https://github.com/deckhouse/deckhouse/pull/19033)
 - **[deckhouse]** Fix module enabling. [#17043](https://github.com/deckhouse/deckhouse/pull/17043)
 - **[deckhouse]** Fix validation logic for a disabled module [#16385](https://github.com/deckhouse/deckhouse/pull/16385)
 - **[deckhouse]** Fixed shell-operator http client to handle resources correctly. [#15182](https://github.com/deckhouse/deckhouse/pull/15182)
 - **[deckhouse]** Fixes bug when module`s config values unchanged after enabling. [#15887](https://github.com/deckhouse/deckhouse/pull/15887)
 - **[deckhouse]** Remove notified=false annotation reset from runReleaseDeploy in the module release controller. [#19178](https://github.com/deckhouse/deckhouse/pull/19178)
 - **[deckhouse]** Setting embedded source for embedded modules. [#15590](https://github.com/deckhouse/deckhouse/pull/15590)
 - **[deckhouse]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[descheduler]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[dhctl]** Add timeout to ssh client dial and fix stop. [#15539](https://github.com/deckhouse/deckhouse/pull/15539)
 - **[dhctl]** Added nil check to dhctl during converge in migrator [#16289](https://github.com/deckhouse/deckhouse/pull/16289)
 - **[dhctl]** Added terminfo for proper terminal behavior in dhctl and deckhouse containers. [#15988](https://github.com/deckhouse/deckhouse/pull/15988)
 - **[dhctl]** Change default ssh mode [#15773](https://github.com/deckhouse/deckhouse/pull/15773)
 - **[dhctl]** Fix converge manifests for static cluster in commander. [#16504](https://github.com/deckhouse/deckhouse/pull/16504)
 - **[dhctl]** Fix deadlock while reading dhctl singlethreaded server logs [#15512](https://github.com/deckhouse/deckhouse/pull/15512)
 - **[dhctl]** Fix getting passphrase for key from connection config for cli. [#16100](https://github.com/deckhouse/deckhouse/pull/16100)
 - **[dhctl]** Fix misbehavior in gossh client. [#15759](https://github.com/deckhouse/deckhouse/pull/15759)
 - **[dhctl]** Fix output klog. Wrap klog logs and redirect to our logger. [#14195](https://github.com/deckhouse/deckhouse/pull/14195)
 - **[dhctl]** Fix panic during destroy. Change opentofu log level to INFO. [#16726](https://github.com/deckhouse/deckhouse/pull/16726)
 - **[dhctl]** Fix parallel bootstrap cloud permanent nodes [#16886](https://github.com/deckhouse/deckhouse/pull/16886)
 - **[dhctl]** Fix ssh client initialising in Commander Attach and Commander Detach operations [#15380](https://github.com/deckhouse/deckhouse/pull/15380)
 - **[dhctl]** Fix start and keep-alive behavior, do not change host if any kube-proxies has been started. [#15566](https://github.com/deckhouse/deckhouse/pull/15566)
 - **[dhctl]** Fixed the behaviour of the attach operation when there are no keys in SSHConfig, only a password [#15478](https://github.com/deckhouse/deckhouse/pull/15478)
 - **[dhctl]** Fixed trigger control-plane pre/post hooks only if a node is being recreated/deleted. [#14998](https://github.com/deckhouse/deckhouse/pull/14998)
 - **[dhctl]** Fixing bashible steps when it does not respond to the limit on the number of attempts [#15633](https://github.com/deckhouse/deckhouse/pull/15633)
 - **[dhctl]** Move yandex withNATInstance layout settings from preflights to preparator. [#16100](https://github.com/deckhouse/deckhouse/pull/16100)
 - **[dhctl]** Not start client if resources were destroyed. [#15952](https://github.com/deckhouse/deckhouse/pull/15952)
 - **[dhctl]** Prompt user about static cluster bootstrap on current host. [#16011](https://github.com/deckhouse/deckhouse/pull/16011)
 - **[dhctl]** Skip cluster-admin role creation if it already exists. [#15562](https://github.com/deckhouse/deckhouse/pull/15562)
 - **[dhctl]** Static master cleanup no longer reports success when the cleanup script times out; NodeUser update no longer fails on missing resourceVersion [#20581](https://github.com/deckhouse/deckhouse/pull/20581)
 - **[dhctl]** Stop all kube proxies during destroy. Improve Do not lock converge for static clusters and save information about lock in state. [#16059](https://github.com/deckhouse/deckhouse/pull/16059)
 - **[dhctl]** Validate WithNATInstance Yandex layout params only in bootstrap. [#16427](https://github.com/deckhouse/deckhouse/pull/16427)
 - **[dhctl]** Wait for stronghold cluster sync before node deletion [#19794](https://github.com/deckhouse/deckhouse/pull/19794)
 - **[dhctl]** dhctl cve fix [#19359](https://github.com/deckhouse/deckhouse/pull/19359)
 - **[dhctl]** dhctl vex upd [#18894](https://github.com/deckhouse/deckhouse/pull/18894)
 - **[dhctl]** mitigate CVE-2026-33186 [#18610](https://github.com/deckhouse/deckhouse/pull/18610)
 - **[docs]** Added description about custom CoreDNS installation. [#16092](https://github.com/deckhouse/deckhouse/pull/16092)
 - **[docs]** Added steps that patch secret and prevented the image pull fail. [#15166](https://github.com/deckhouse/deckhouse/pull/15166)
 - **[docs]** Outdated instructions for reducing the number of master nodes in cloud clusters have been fixed. Users will correctly perform the converge of cloud clusters. [#16642](https://github.com/deckhouse/deckhouse/pull/16642)
    The instructions for reducing the number of master nodes have not been updated for a long time, and for a long time now, the conversion to reducing the number of master nodes in clusters in a cloud installation is automatic.
 - **[documentation]** Added tolerations support to DexAuthenticator configuration. [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[documentation]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[extended-monitoring]** Exclude PVCs with block volume mode from space and inodes monitoring. [#14859](https://github.com/deckhouse/deckhouse/pull/14859)
    free space monitoring for the PVCs in the Block volumeMode is meaningless and will be disabled
 - **[extended-monitoring]** Fix extended-monitoring.deckhouse.io/enabled label handling [#16372](https://github.com/deckhouse/deckhouse/pull/16372)
    the extended monitoring will only be enabled when the label is explicitly set on a namespace
 - **[extended-monitoring]** Fixed CVE-2025-47914, CVE-2025-58181 [#17576](https://github.com/deckhouse/deckhouse/pull/17576)
 - **[extended-monitoring]** Init extended-monitoring-exporter on unavailable API. [#15529](https://github.com/deckhouse/deckhouse/pull/15529)
 - **[extended-monitoring]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[extended-monitoring]** always use the default CA bundle in the image-availability-exporter [#20404](https://github.com/deckhouse/deckhouse/pull/20404)
 - **[extended-monitoring]** drop metrics when extended monitoring is disabled for node(s) [#16446](https://github.com/deckhouse/deckhouse/pull/16446)
    erroneous alerts for node disk usage are fixed
 - **[ingress-nginx]** A symlink to the new opentelemetry config path is added. [#16433](https://github.com/deckhouse/deckhouse/pull/16433)
    Ingress-Nginx controller's pods of 1.9 version will be restarted.
 - **[ingress-nginx]** CVE-2025-15566 is backported. [#19220](https://github.com/deckhouse/deckhouse/pull/19220)
    All pods of Ingress-NGINX controller will be restarted.
 - **[ingress-nginx]** CVE-2026-3288 fix is backported in all Ingress-Nginx controllers. [#18428](https://github.com/deckhouse/deckhouse/pull/18428)
    All Ingress-Nginx controller pods will be restarted.
 - **[ingress-nginx]** CVEs fixed [#16340](https://github.com/deckhouse/deckhouse/pull/16340)
 - **[ingress-nginx]** Disabled log messages such as `Error obtaining Endpoints for Service...`. [#15260](https://github.com/deckhouse/deckhouse/pull/15260)
 - **[ingress-nginx]** Enable ExecuteHookOnEvents on hook set_annotation_validation_suspended.go [#15839](https://github.com/deckhouse/deckhouse/pull/15839)
 - **[ingress-nginx]** Fixed CVE CVE-2025-5187. [#15906](https://github.com/deckhouse/deckhouse/pull/15906)
 - **[ingress-nginx]** Fixed CVE's. [#15776](https://github.com/deckhouse/deckhouse/pull/15776)
 - **[ingress-nginx]** Fixed CVEs [#16432](https://github.com/deckhouse/deckhouse/pull/16432)
 - **[ingress-nginx]** Fixed CVEs, found in auxiliary source code. [#16069](https://github.com/deckhouse/deckhouse/pull/16069)
 - **[ingress-nginx]** Fixed nginx image build for ingress controller and template tests. [#15464](https://github.com/deckhouse/deckhouse/pull/15464)
 - **[ingress-nginx]** Fixed nginx segfaults when opentelemetry is enabled. [#15466](https://github.com/deckhouse/deckhouse/pull/15466)
    The ingress-nginx pods of 1.10 will be restarted.
 - **[ingress-nginx]** Fixes are backported to 1.73. [#18960](https://github.com/deckhouse/deckhouse/pull/18960)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** Golang and Altlinux updates are reverted. [#18223](https://github.com/deckhouse/deckhouse/pull/18223)
    All modules of the ingress-nginx module will be restarted.
 - **[ingress-nginx]** Latest CVEs are fixed. [#17222](https://github.com/deckhouse/deckhouse/pull/17222)
    All pods running kube-rbac-proxy will be restarted.
 - **[ingress-nginx]** Nginx and module's dependencies are updated. [#18102](https://github.com/deckhouse/deckhouse/pull/18102)
    All ingress-nginx controller pods will be restared.
 - **[ingress-nginx]** Nginx is updated to 1.30.3. [#20785](https://github.com/deckhouse/deckhouse/pull/20785)
    All ingress-nginx pods will be restarted.
 - **[ingress-nginx]** Nginx is updated up to 1.30.1. [#19848](https://github.com/deckhouse/deckhouse/pull/19848)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Nginx was updated to 1.30.2. [#20172](https://github.com/deckhouse/deckhouse/pull/20172)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** The CVE-2026-1580, CVE-2026-24512, CVE-2026-24513, CVE-2026-24514 CVEs fixes are backported. [#17808](https://github.com/deckhouse/deckhouse/pull/17808)
    The ingress nginx controllers' pods will be restated.
 - **[ingress-nginx]** The same order of limit_req_zone statements is maintained when generating new configuration. [#15789](https://github.com/deckhouse/deckhouse/pull/15789)
    The pods of the ingress-nginx module will be restarted.
 - **[ingress-nginx]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[istio]** CNI-node readonly root filesystem enable [#19342](https://github.com/deckhouse/deckhouse/pull/19342)
 - **[istio]** CNI-node readonly root filesystem enable fix [#19690](https://github.com/deckhouse/deckhouse/pull/19690)
    When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - **[istio]** Erroneous option in 1.25 control-plane helm template fixed. [#16412](https://github.com/deckhouse/deckhouse/pull/16412)
 - **[istio]** Fix CVE for Istio version 1.21 and 1.25 [#17298](https://github.com/deckhouse/deckhouse/pull/17298)
 - **[istio]** Fixed AuthorizationPolicy CRD insufficiency for Istio 1.25. [#16605](https://github.com/deckhouse/deckhouse/pull/16605)
 - **[istio]** Fixed metrics port for operator 1.25 and newer. [#15124](https://github.com/deckhouse/deckhouse/pull/15124)
 - **[istio]** Reduce CPU and RAM for regenerate multicluster JWT token and sort ingressGateway [#18334](https://github.com/deckhouse/deckhouse/pull/18334)
 - **[istio]** Reduce RAM for regenerate multicluster JWT token [#15328](https://github.com/deckhouse/deckhouse/pull/15328)
 - **[istio]** Resolve CVE's. [#15834](https://github.com/deckhouse/deckhouse/pull/15834)
 - **[istio]** The metrics-exporter's template is fixed, it blocked the main queue if  `controlPlane.nodeSelector` setting was configured. [#15146](https://github.com/deckhouse/deckhouse/pull/15146)
 - **[istio]** The same owner is specified for the files that are used to run in the operator container. [#16154](https://github.com/deckhouse/deckhouse/pull/16154)
 - **[istio]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[istio]** added iptables wrapper in cni-v1x21x6 [#18955](https://github.com/deckhouse/deckhouse/pull/18955)
    istio-cni-nodes will be restarted
 - **[istio]** fixed CVE in istio module v1.19.7 [#18811](https://github.com/deckhouse/deckhouse/pull/18811)
    istio module pods will be restarted
 - **[istio]** fixed CVE in istio module v1.21.6 [#18688](https://github.com/deckhouse/deckhouse/pull/18688)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-34986 [#18967](https://github.com/deckhouse/deckhouse/pull/18967)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-39882, CVE-2026-39883 and CVE-2026-35206 [#19096](https://github.com/deckhouse/deckhouse/pull/19096)
    istio module pods will be restarted
 - **[istio]** fixed CVEs in module images [#19358](https://github.com/deckhouse/deckhouse/pull/19358)
    module pods will be restarted
 - **[istio]** fixed CVEs in module images [#20008](https://github.com/deckhouse/deckhouse/pull/20008)
 - **[istio]** fixed CVEs in module v1.25.2 images [#18807](https://github.com/deckhouse/deckhouse/pull/18807)
    istio module pods will be restarted
 - **[istio]** fixing the CVE in Kiali [#17045](https://github.com/deckhouse/deckhouse/pull/17045)
 - **[keepalived]** Excluded vulnerable pip-25.3 from keepalived final image to fix CVE-2026-1703 [#19152](https://github.com/deckhouse/deckhouse/pull/19152)
 - **[kube-dns]** Improved /etc/hosts renderer compatibility with admission-policy-engine Restricted mode. [#16599](https://github.com/deckhouse/deckhouse/pull/16599)
 - **[kube-dns]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[kube-proxy]** Fixed CVE-2026-33186 and CVE-2026-24051 in kube-proxy dependencies. [#19120](https://github.com/deckhouse/deckhouse/pull/19120)
    This update triggers a rolling update of the kube-proxy pods.
 - **[kube-proxy]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[local-path-provisioner]** Add wildcard tolerations to the helper pod template so PVC provisioning works on tainted nodes after local-path-provisioner v0.0.32+. [#19449](https://github.com/deckhouse/deckhouse/pull/19449)
 - **[local-path-provisioner]** Backport HelperPod template validation to `local-path-provisioner` v0.0.34 to fix CVE-2026-44543 (HelperPod Template Injection, GHSA-7fxv-8wr2-mfc4, CVSS 8.7 High). [#20329](https://github.com/deckhouse/deckhouse/pull/20329)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected. After the update the provisioner refuses to create a HelperPod whose template (loaded from the `local-path-config` ConfigMap) declares privileged containers, hostPath/custom volumes, host namespaces, added Linux capabilities or other security-sensitive fields, so any pre-existing manual override of `helperPod.yaml` that uses one of these fields must be removed before the upgrade.
 - **[local-path-provisioner]** Bump `local-path-provisioner` to `v0.0.34` to fix CVE-2025-62878 (path traversal via `StorageClass.parameters.pathPattern`, CVSS 10.0). [#19354](https://github.com/deckhouse/deckhouse/pull/19354)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected.
 - **[local-path-provisioner]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to align release-1.73 with the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7) instead of carrying a separate backport patch. [#20455](https://github.com/deckhouse/deckhouse/pull/20455)
    The `local-path-provisioner` Pod is restarted during the update. Custom edits to the `local-path-config` ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to pick up the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7). [#20449](https://github.com/deckhouse/deckhouse/pull/20449)
    Custom edits to the local-path-config ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - **[log-shipper]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[loki]** Change stage to Preview [#15553](https://github.com/deckhouse/deckhouse/pull/15553)
 - **[loki]** Fixed CVE-2025-47914, CVE-2025-58181 [#17555](https://github.com/deckhouse/deckhouse/pull/17555)
 - **[loki]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[loki]** disable send analytics report to stats.grafana.org [#17109](https://github.com/deckhouse/deckhouse/pull/17109)
    config module loki ↓
 - **[metallb]** Fixed CVE's. [#15777](https://github.com/deckhouse/deckhouse/pull/15777)
 - **[metallb]** Fixed the Deckhouse controller queue freezing issue that occurs when Service was deleted, but child resource L2LBService wasn't. [#14966](https://github.com/deckhouse/deckhouse/pull/14966)
 - **[metallb]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes-control-plane]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes]** Added `tier=cluster` label. [#15290](https://github.com/deckhouse/deckhouse/pull/15290)
 - **[monitoring-kubernetes]** Fixed CVE-2025-47914, CVE-2025-58181 [#17571](https://github.com/deckhouse/deckhouse/pull/17571)
 - **[monitoring-kubernetes]** Fixed gaps on graph. [#15479](https://github.com/deckhouse/deckhouse/pull/15479)
 - **[monitoring-kubernetes]** Rollout changes for resources metrics kubelet [#16408](https://github.com/deckhouse/deckhouse/pull/16408)
 - **[monitoring-kubernetes]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes]** fix CVE-2025-52881 for node-exporter [#16376](https://github.com/deckhouse/deckhouse/pull/16376)
 - **[monitoring-kubernetes]** remove the Docker traces from the module code [#16542](https://github.com/deckhouse/deckhouse/pull/16542)
    node-exporter pods will be rollout restarted during upgrade
 - **[monitoring-ping]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[multitenancy-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[multitenancy-manager]** fix CVE-2024-25621  CVE-2025-64329 [#16360](https://github.com/deckhouse/deckhouse/pull/16360)
 - **[network-gateway]** Fixed werf import syntax for compatibility with older werf versions. [#19275](https://github.com/deckhouse/deckhouse/pull/19275)
 - **[network-gateway]** Updated dnsmasq to v2.92-alt2 to address multiple security vulnerabilities (CVE-2026-*) [#19936](https://github.com/deckhouse/deckhouse/pull/19936)
 - **[network-gateway]** Updated python image source and mitigated pip CVE-2026-1703 [#19149](https://github.com/deckhouse/deckhouse/pull/19149)
 - **[network-policy-engine]** Fixed CVE-2026-34040, CVE-2026-33997, and CVE-2026-33186 in network-policy-engine dependencies. [#19109](https://github.com/deckhouse/deckhouse/pull/19109)
    This update triggers a rolling update of the network-policy-engine pods.
 - **[node-local-dns]** Fixed CVE-2025-59530 and updated CoreDNS to version 1.13.1. [#15965](https://github.com/deckhouse/deckhouse/pull/15965)
 - **[node-local-dns]** Fixed CVEs [#17471](https://github.com/deckhouse/deckhouse/pull/17471)
 - **[node-local-dns]** Return stale-dns-connections-cleaner [#18755](https://github.com/deckhouse/deckhouse/pull/18755)
    An additional service daemonset will be added.
 - **[node-manager]** Add rbac permission for getting DaemonSets for CAPI. [#15377](https://github.com/deckhouse/deckhouse/pull/15377)
 - **[node-manager]** Enable SSHLegacyMode if SSHPrivateKey is not empty and fix gossh panic. [#15763](https://github.com/deckhouse/deckhouse/pull/15763)
 - **[node-manager]** Fix panic in cluster-autoscaler caused by nil pointer dereference during node removal simulation. [#17924](https://github.com/deckhouse/deckhouse/pull/17924)
 - **[node-manager]** Fix panic in registry packages proxy if image not found. [#16425](https://github.com/deckhouse/deckhouse/pull/16425)
 - **[node-manager]** Fixed TLS vulnerabilities for capi-controller-manager [#20137](https://github.com/deckhouse/deckhouse/pull/20137)
 - **[node-manager]** Issue the CAPI webhook certificate so strict TLS validators accept it (distinct CA Subject, `server auth` EKU); legacy certificates are re-issued, restarting `capi-controller-manager` [#20349](https://github.com/deckhouse/deckhouse/pull/20349)
    The `capi-controller-manager` pod will be restarted to pick up the re-issued webhook certificate
 - **[node-manager]** Mitigate multiple CVEs [#18867](https://github.com/deckhouse/deckhouse/pull/18867)
 - **[node-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[node-manager]** Upgrade CAPI version and increase lease duration timeouts. [#15349](https://github.com/deckhouse/deckhouse/pull/15349)
 - **[node-manager]** fixed cve in caps [#18892](https://github.com/deckhouse/deckhouse/pull/18892)
 - **[node-manager]** mitigate CVE-2026-33186 [#18649](https://github.com/deckhouse/deckhouse/pull/18649)
 - **[node-manager]** shutdown inhibitor use three-state GracefulShutdownPostpone condition so kubelet waits for the final decision before ending pods [#15575](https://github.com/deckhouse/deckhouse/pull/15575)
 - **[openvpn]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[openvpn]** ovpn-admin upgraded to fix the validation of static IP addresses, as well as add routes migration during the rotation of client certificates, openvpn instances will be restarted. [#14578](https://github.com/deckhouse/deckhouse/pull/14578)
 - **[operator-prometheus]** Fixed CVE-2025-47914, CVE-2025-58181 [#17601](https://github.com/deckhouse/deckhouse/pull/17601)
 - **[operator-prometheus]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[operator-trivy]** Add grep to node-collector and improve error reporting [#16277](https://github.com/deckhouse/deckhouse/pull/16277)
 - **[operator-trivy]** Added a passtrough for a HTTP(s) proxy parameters from operator to vulnerability scanning jobs processes. [#15401](https://github.com/deckhouse/deckhouse/pull/15401)
 - **[operator-trivy]** Backported CVE patches from external module incarnation [#18879](https://github.com/deckhouse/deckhouse/pull/18879)
 - **[operator-trivy]** Fix CIS Benchmark report template [#16489](https://github.com/deckhouse/deckhouse/pull/16489)
 - **[operator-trivy]** Fixed node-collector pods crasing on startup. [#15401](https://github.com/deckhouse/deckhouse/pull/15401)
 - **[operator-trivy]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[prometheus-metrics-adapter]** Fixed CVE-2025-47914, CVE-2025-58181 [#17570](https://github.com/deckhouse/deckhouse/pull/17570)
 - **[prometheus-metrics-adapter]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[prometheus-pushgateway]** Fixed CVE-2025-47914, CVE-2025-58181, CVE-2025-22872, CVE-2025-22868 [#17556](https://github.com/deckhouse/deckhouse/pull/17556)
 - **[prometheus]** Fix description for not usable CVE [#16377](https://github.com/deckhouse/deckhouse/pull/16377)
 - **[prometheus]** Fix namespace label value in the Ingress Nginx controller and several other metrics [#16720](https://github.com/deckhouse/deckhouse/pull/16720)
    Ingress Nginx controller dashboards are fixed
 - **[prometheus]** Fix remote write dropping valid samples after restart due to missing series from snapshot. [#14849](https://github.com/deckhouse/deckhouse/pull/14849)
 - **[prometheus]** Fix securityContext indentation in the Prometheus main and longterm resources. [#15102](https://github.com/deckhouse/deckhouse/pull/15102)
    main and longterm Prometheuses will be rollout-restarted
 - **[prometheus]** Fixed CVE-2025-47914, CVE-2025-58181, CVE-2025-65637 [#17597](https://github.com/deckhouse/deckhouse/pull/17597)
 - **[prometheus]** Fixed template indentation [#15434](https://github.com/deckhouse/deckhouse/pull/15434)
 - **[prometheus]** Suppress Grafana-related alerts when the Grafana is disabled in the ModuleConfig. [#14981](https://github.com/deckhouse/deckhouse/pull/14981)
    default
 - **[prometheus]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[registry-packages-proxy]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[registry]** Omitted the auth field in DockerConfig when credentials (username and password) are empty. [#17332](https://github.com/deckhouse/deckhouse/pull/17332)
 - **[registry]** Updated auth image Go dependencies to fix Go CVEs. [#18231](https://github.com/deckhouse/deckhouse/pull/18231)
    Registry pods will be restarted.
 - **[registry]** bump go_lib/registry dependencies [#15985](https://github.com/deckhouse/deckhouse/pull/15985)
 - **[registrypackages]** Added `which` to RPP. [#19069](https://github.com/deckhouse/deckhouse/pull/19069)
 - **[registrypackages]** Added vex with CVE-2026-33186 for kubeadm and kubelet. [#18783](https://github.com/deckhouse/deckhouse/pull/18783)
 - **[registrypackages]** Added vex with CVE-2026-33186. [#18700](https://github.com/deckhouse/deckhouse/pull/18700)
 - **[registrypackages]** Fix last changes in go.mod patch [#19880](https://github.com/deckhouse/deckhouse/pull/19880)
 - **[registrypackages]** Fixed permissions for directory with cni-plugins in PCI-DSS clusters [#16409](https://github.com/deckhouse/deckhouse/pull/16409)
 - **[registrypackages]** Fixes CVE in kubernetes-cni [#16343](https://github.com/deckhouse/deckhouse/pull/16343)
 - **[registrypackages]** Update containerd to 1.7.29 / 2.1.5 and runc to 1.3.3 [#16335](https://github.com/deckhouse/deckhouse/pull/16335)
 - **[registrypackages]** Update integrity patch for containerd (cse only). [#17000](https://github.com/deckhouse/deckhouse/pull/17000)
 - **[registrypackages]** Update runc to 1.3.1. [#16263](https://github.com/deckhouse/deckhouse/pull/16263)
 - **[registrypackages]** Updated registrypackages/docker-registry image Go dependencies to fix Go CVEs. [#20375](https://github.com/deckhouse/deckhouse/pull/20375)
 - **[registrypackages]** Upgrade containerd to 1.7.30 and 2.1.6. [#17582](https://github.com/deckhouse/deckhouse/pull/17582)
    Containerd will restart.
 - **[service-with-healthchecks]** Fixed CVEs [#16950](https://github.com/deckhouse/deckhouse/pull/16950)
 - **[service-with-healthchecks]** Improved the module's security [#15358](https://github.com/deckhouse/deckhouse/pull/15358)
 - **[service-with-healthchecks]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[terraform-manager]** Fixed critical CVEs in dependencies across all cloud providers [#18599](https://github.com/deckhouse/deckhouse/pull/18599)
 - **[terraform-manager]** Fixed terraform CVE. [#17862](https://github.com/deckhouse/deckhouse/pull/17862)
 - **[terraform-manager]** Implemented automatic VEX file merging from dhctl into terraform-manager images [#18892](https://github.com/deckhouse/deckhouse/pull/18892)
 - **[terraform-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[terraform-manager]** bump moby/spdystream to v0.5.1 in terraform-manager-dvp image to fix CVE [#19722](https://github.com/deckhouse/deckhouse/pull/19722)
 - **[terraform-manager]** fixed cve in terraform manager [#18892](https://github.com/deckhouse/deckhouse/pull/18892)
 - **[terraform-manager]** fixed cve in terraform manager and update version in terraform-manager [#18800](https://github.com/deckhouse/deckhouse/pull/18800)
 - **[terraform-manager]** fixed terraform CVE [#17871](https://github.com/deckhouse/deckhouse/pull/17871)
 - **[upmeter]** Add proper securityContext to the upmeter probe to meet the restricted security profile constraints. [#18743](https://github.com/deckhouse/deckhouse/pull/18743)
 - **[upmeter]** Fixed CVE-2025-47914, CVE-2025-58181, CVE-2025-65637 [#17557](https://github.com/deckhouse/deckhouse/pull/17557)
 - **[upmeter]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[upmeter]** fix securityxontext for statefulset [#16534](https://github.com/deckhouse/deckhouse/pull/16534)
    upmeter check
 - **[user-authn]** Fix BadRequest after the change password redirect when password policy is enabled [#16744](https://github.com/deckhouse/deckhouse/pull/16744)
 - **[user-authn]** Fix CVE-2025-22868 [#15420](https://github.com/deckhouse/deckhouse/pull/15420)
    This update fixes a security vulnerability in the `user-authn` module (CVE-2025-22868) that could potentially allow bypass of authentication validation.
 - **[user-authn]** Fix `ValidatingAdmissionPolicy` for `User` CR to skip password check on create. [#15269](https://github.com/deckhouse/deckhouse/pull/15269)
    Fixes a bug where creating new User resources failed due to missing oldObject in validation; password immutability is still enforced on update.
 - **[user-authn]** Fix critical bug in password connector that caused login failures for LDAP, Crowd, and Keystone connectors. [#15359](https://github.com/deckhouse/deckhouse/pull/15359)
 - **[user-authn]** Fix login error 500 with password policy enabled. [#16703](https://github.com/deckhouse/deckhouse/pull/16703)
 - **[user-authn]** Fixed Dex password policy 'Excellent' rule — allow two identical characters in a row, reject three or more. [#15868](https://github.com/deckhouse/deckhouse/pull/15868)
    Fixes incorrect rejection of valid strong passwords.
 - **[user-authn]** Fixed cert generation job deletion. [#15764](https://github.com/deckhouse/deckhouse/pull/15764)
 - **[user-authn]** In the latest go versions (1.25.2, 1.24.8) the https://github.com/golang/go/issues/75712, and now Dex fails with an error. This patch makes Dex wrap only IPv6 addresses in brackets, which is more correct. [#15890](https://github.com/deckhouse/deckhouse/pull/15890)
 - **[user-authn]** Rollback patch for handling insecureSkipEmailVerified condition [#16347](https://github.com/deckhouse/deckhouse/pull/16347)
 - **[user-authn]** Show 'Access Denied' instead of 'Internal Error' for restricted local users. [#15593](https://github.com/deckhouse/deckhouse/pull/15593)
    Users will see a clear error message when their login is restricted by allowed group or email, instead of a confusing internal error.
 - **[user-authn]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[user-authn]** User now can't create groups with  recursive loops in nested group's hierarchy. [#15139](https://github.com/deckhouse/deckhouse/pull/15139)
 - **[user-authn]** When insecureSkipEmailVerified is enabled remove the email_verified claim from identity. [#15869](https://github.com/deckhouse/deckhouse/pull/15869)
    When enabled, Dex will remove email_verified from emitted identity/claims.
 - **[user-authn]** skipApproval no longer bypasses TOTP. When 2FA is enabled, users are sent to /totp before approval, so “auth request does not have an identity for approval” no longer occurs [#16946](https://github.com/deckhouse/deckhouse/pull/16946)
 - **[user-authz]** cache namespace label checks in the user-authz webhook via informer to avoid per-request apiserver GETs [#16920](https://github.com/deckhouse/deckhouse/pull/16920)
 - **[vertical-pod-autoscaler]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)

## Chore


 - **[admission-policy-engine]** Fixed CVE's. [#15237](https://github.com/deckhouse/deckhouse/pull/15237)
 - **[admission-policy-engine]** Made trivy-provider set readOnlyRootFilesystem. [#15837](https://github.com/deckhouse/deckhouse/pull/15837)
 - **[admission-policy-engine]** Updated VEX entries for trivy-provider [#18951](https://github.com/deckhouse/deckhouse/pull/18951)
 - **[admission-policy-engine]** Updated dependencies to fix CVE's. [#15459](https://github.com/deckhouse/deckhouse/pull/15459)
 - **[candi]** Bump patch versions of Kubernetes images. [#14979](https://github.com/deckhouse/deckhouse/pull/14979)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Bump patch versions of Kubernetes images. [#15422](https://github.com/deckhouse/deckhouse/pull/15422)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Changes for launching container v2 with signed images. [#15249](https://github.com/deckhouse/deckhouse/pull/15249)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.16.0. [#15111](https://github.com/deckhouse/deckhouse/pull/15111)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.17.0. [#15251](https://github.com/deckhouse/deckhouse/pull/15251)
 - **[cert-manager]** Fixes for CVE-2025-22870 CVE-2025-22872 CVE-2025-22869 CVE-2025-22868 CVE-2025-27144 CVE-2025-30204 [#15123](https://github.com/deckhouse/deckhouse/pull/15123)
 - **[cert-manager]** Increase timeout for admission webhooks to 30 seconds. [#15847](https://github.com/deckhouse/deckhouse/pull/15847)
 - **[cilium-hubble]** Added vex with CVE-2026-33726 for hubble [#18922](https://github.com/deckhouse/deckhouse/pull/18922)
 - **[cilium-hubble]** Fixed vex file [#19016](https://github.com/deckhouse/deckhouse/pull/19016)
 - **[cloud-provider-aws]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-azure]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-dvp]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-dynamix]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-gcp]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-huaweicloud]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-openstack]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-vcd]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-vsphere]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-vsphere]** Fixed CVE in vsphere module. [#15867](https://github.com/deckhouse/deckhouse/pull/15867)
 - **[cloud-provider-yandex]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cloud-provider-zvirt]** CVE fixes for cloud-providers. [#15982](https://github.com/deckhouse/deckhouse/pull/15982)
 - **[cni-cilium]** Added SVACE analyze for module. [#15616](https://github.com/deckhouse/deckhouse/pull/15616)
 - **[cni-cilium]** Added vex with CVE-2026-33726 for hubble [#18922](https://github.com/deckhouse/deckhouse/pull/18922)
 - **[cni-cilium]** Fixed vex file [#19016](https://github.com/deckhouse/deckhouse/pull/19016)
 - **[cni-cilium]** Images for 1.17 were refactored to achieve distroless. [#14192](https://github.com/deckhouse/deckhouse/pull/14192)
 - **[cni-cilium]** Improved the security of cilium containers of the CNI plugin [#15494](https://github.com/deckhouse/deckhouse/pull/15494)
    cilium pods will be restarted, network traffic may be interrupted.
 - **[cni-cilium]** Refactor build to use pre-packaged dependencies from envoyproxy_deps repository instead of downloading from GitHub at build time [#18941](https://github.com/deckhouse/deckhouse/pull/18941)
    Cilium agents will be restarted.
 - **[cni-cilium]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-flannel]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-flannel]** The readOnlyRootFilesystem security option is set to true for all containers. [#15444](https://github.com/deckhouse/deckhouse/pull/15444)
    Pods of the cni-flannel module will be restarted.
 - **[cni-simple-bridge]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-simple-bridge]** The readOnlyRootFilesystem security option is set to true for all containers. [#15476](https://github.com/deckhouse/deckhouse/pull/15476)
    Pods of the cni-simple-bridge module will be restarted.
 - **[common]** Fixed CVE in kube-apiserver. [#15893](https://github.com/deckhouse/deckhouse/pull/15893)
 - **[common]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse-controller]** Check lowercased scheme in ChangeRegistry function. [#15197](https://github.com/deckhouse/deckhouse/pull/15197)
 - **[deckhouse-controller]** Removed embedded pod-reloader module. The module was migrated and available as module from the `deckhouse` ModuleSource. [#14343](https://github.com/deckhouse/deckhouse/pull/14343)
 - **[deckhouse-controller]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse-tools]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse]** Added check modules sign. [#15450](https://github.com/deckhouse/deckhouse/pull/15450)
 - **[deckhouse]** Bump addon-operator. [#15884](https://github.com/deckhouse/deckhouse/pull/15884)
 - **[deckhouse]** Remove module weight constraints. [#15131](https://github.com/deckhouse/deckhouse/pull/15131)
 - **[deckhouse]** Updated d8 to 0.20.0 [#15817](https://github.com/deckhouse/deckhouse/pull/15817)
 - **[deckhouse]** Updated d8 to 0.20.3. [#15845](https://github.com/deckhouse/deckhouse/pull/15845)
 - **[deckhouse]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[dhctl]** Fix CVE in dhctl go.mod. [#15878](https://github.com/deckhouse/deckhouse/pull/15878)
 - **[dhctl]** Set default ssh port to 22, to backward compatibility with cli ssh behavior in dhctl. [#16947](https://github.com/deckhouse/deckhouse/pull/16947)
 - **[dhctl]** mitigate cves [#17963](https://github.com/deckhouse/deckhouse/pull/17963)
 - **[docs]** Add NGC examples for automatically installation of NVIDIA drivers. [#16864](https://github.com/deckhouse/deckhouse/pull/16864)
 - **[docs]** Upgrade Hugo to v0.162.1 with API migration. [#20253](https://github.com/deckhouse/deckhouse/pull/20253)
 - **[extended-monitoring]** Added FAQ document. [#15640](https://github.com/deckhouse/deckhouse/pull/15640)
 - **[extended-monitoring]** Migrated to golang. [#15781](https://github.com/deckhouse/deckhouse/pull/15781)
 - **[ingress-nginx]** Fix CVEs in sources [#16190](https://github.com/deckhouse/deckhouse/pull/16190)
 - **[ingress-nginx]** Ingress controller now runs under the deckhouse user (instead of www-data). [#14245](https://github.com/deckhouse/deckhouse/pull/14245)
    Ingress-nginx Controllers will be restarted, which will cause traffic interruption.
 - **[ingress-nginx]** Shrinking 1.10 image is backported. [#19518](https://github.com/deckhouse/deckhouse/pull/19518)
    All pods of the 1.10 Ingress-NGINX controller will be restarted.
 - **[ingress-nginx]** Switched to a distroless base image for the ingress controller v1.12, reducing its size and fixing multiple CVEs. [#14469](https://github.com/deckhouse/deckhouse/pull/14469)
    Ingress controller pods will restart.
 - **[ingress-nginx]** The readOnlyRootFilesystem security option is set to true for all containers. [#15496](https://github.com/deckhouse/deckhouse/pull/15496)
    ALL pods of the ingress-nginx module will be restarted.
 - **[istio]** Corrected permissions of executable files in EE. [#15626](https://github.com/deckhouse/deckhouse/pull/15626)
 - **[istio]** Fix CVE, add vex, rewrite kiali build. [#15983](https://github.com/deckhouse/deckhouse/pull/15983)
 - **[istio]** Fix CVEs in sources [#16191](https://github.com/deckhouse/deckhouse/pull/16191)
 - **[istio]** Fixed CVE-2026-46680 in operator 1.25 [#20209](https://github.com/deckhouse/deckhouse/pull/20209)
 - **[keepalived]** The readOnlyRootFilesystem security option is set to true for all containers. [#15487](https://github.com/deckhouse/deckhouse/pull/15487)
    Pods of the openvpn module will be restarted.
 - **[kube-dns]** Added SVACE analyze for module. [#15648](https://github.com/deckhouse/deckhouse/pull/15648)
 - **[kube-dns]** The readOnlyRootFilesystem security option is set to true for all containers. [#15391](https://github.com/deckhouse/deckhouse/pull/15391)
    The kube-dns webhook pod will be restarted.
 - **[kube-proxy]** The readOnlyRootFilesystem security option is set to true for all containers. [#15409](https://github.com/deckhouse/deckhouse/pull/15409)
    Pods of the kube-proxy module will be restarted.
 - **[metallb]** All required mount points are defined in the mount-points.yaml file. [#15657](https://github.com/deckhouse/deckhouse/pull/15657)
    The pods of the metallb module will be restarted.
 - **[monitoring-kubernetes]** Added missing severity_level label to the PodStatusIsIncorrect alert. [#16549](https://github.com/deckhouse/deckhouse/pull/16549)
 - **[monitoring-kubernetes]** Standardizing graphs appearance. [#15440](https://github.com/deckhouse/deckhouse/pull/15440)
 - **[network-gateway]** The readOnlyRootFilesystem security option is set to true for all containers. [#15414](https://github.com/deckhouse/deckhouse/pull/15414)
    Pods of the network-gateway module will be restarted.
 - **[network-policy-engine]** The readOnlyRootFilesystem security option is set to true for all containers. [#15427](https://github.com/deckhouse/deckhouse/pull/15427)
    Pods of the network-policy-engine module will be restarted.
 - **[node-local-dns]** Build refactored and improved observability by adding alerts about resolving issues. [#14364](https://github.com/deckhouse/deckhouse/pull/14364)
 - **[node-local-dns]** Removed `stale-dns-connections-cleaner`, since the related issue was fixed in `cni-cilium` upstream. [#16447](https://github.com/deckhouse/deckhouse/pull/16447)
 - **[node-local-dns]** The readOnlyRootFilesystem security option is set to true for all containers. [#15395](https://github.com/deckhouse/deckhouse/pull/15395)
    The node-local-dns pods will be restarted.
 - **[node-manager]** Added sign check and integrity check to the registry-packages-proxy. [#14685](https://github.com/deckhouse/deckhouse/pull/14685)
 - **[node-manager]** Group get_crd errors and make them more readable. [#15591](https://github.com/deckhouse/deckhouse/pull/15591)
 - **[node-manager]** mitigate cves [#17976](https://github.com/deckhouse/deckhouse/pull/17976)
 - **[openvpn]** The readOnlyRootFilesystem security option is set to true for all containers. [#15346](https://github.com/deckhouse/deckhouse/pull/15346)
    Pods of the openvpn module will be restarted.
 - **[operator-trivy]** Added VEX manifests to artifacts. [#15992](https://github.com/deckhouse/deckhouse/pull/15992)
 - **[operator-trivy]** Distroless-based node-collector in Trivy Operator. [#16006](https://github.com/deckhouse/deckhouse/pull/16006)
 - **[operator-trivy]** Fix CVE [#19226](https://github.com/deckhouse/deckhouse/pull/19226)
 - **[operator-trivy]** Fix CVE's. [#15401](https://github.com/deckhouse/deckhouse/pull/15401)
 - **[operator-trivy]** Fixed CVE's [#19908](https://github.com/deckhouse/deckhouse/pull/19908)
 - **[operator-trivy]** Fixed CVE-2025-22868 in trivy node-collector image. [#15669](https://github.com/deckhouse/deckhouse/pull/15669)
 - **[operator-trivy]** Use updated trivy patches [#19340](https://github.com/deckhouse/deckhouse/pull/19340)
 - **[prometheus]** Add svace analyze for mimir image. [#16068](https://github.com/deckhouse/deckhouse/pull/16068)
 - **[prometheus]** Added POD_IP var to config. [#15527](https://github.com/deckhouse/deckhouse/pull/15527)
 - **[prometheus]** Added pre-created paths create folder. [#15832](https://github.com/deckhouse/deckhouse/pull/15832)
 - **[prometheus]** Added svace analys for apps. [#15658](https://github.com/deckhouse/deckhouse/pull/15658)
 - **[prometheus]** Deprecated the direct Prometheus access. [#14812](https://github.com/deckhouse/deckhouse/pull/14812)
    Accessing Prometheus via ingress is now considered deprecated and will not be possible in future releases.
 - **[prometheus]** Migrated from old hook logic for enabling prompp. [#15308](https://github.com/deckhouse/deckhouse/pull/15308)
 - **[prometheus]** Removed deprecated tls certs. [#15638](https://github.com/deckhouse/deckhouse/pull/15638)
 - **[prometheus]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[registry-packages-proxy]** Added separate secret to rpp for imagePullSecrets. [#15783](https://github.com/deckhouse/deckhouse/pull/15783)
 - **[registry-packages-proxy]** mitigate cves [#17974](https://github.com/deckhouse/deckhouse/pull/17974)
 - **[registry]** Fixed CVE's: CVE-2020-26160, CVE-2020-8911, CVE-2020-8912, CVE-2022-21698, CVE-2022-2582, CVE-2025-22868, CVE-2025-22869, CVE-2025-22870, CVE-2025-22872, CVE-2025-27144 [#15235](https://github.com/deckhouse/deckhouse/pull/15235)
 - **[registry]** Update dependencies to fix CVEs [#16635](https://github.com/deckhouse/deckhouse/pull/16635)
 - **[registrypackages]** Add vex for etcdl/etcdutil, crictl, containerd. [#18784](https://github.com/deckhouse/deckhouse/pull/18784)
 - **[registrypackages]** Fixed CVEs through vex files [#19014](https://github.com/deckhouse/deckhouse/pull/19014)
 - **[registrypackages]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[user-authz]** Fix CVE-2025-22868 [#15120](https://github.com/deckhouse/deckhouse/pull/15120)
