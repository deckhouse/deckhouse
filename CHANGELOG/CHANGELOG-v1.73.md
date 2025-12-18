# Changelog v1.73

## [MALFORMED]


 - #13577 unknown section "operator-trivy"
 - #15401 unknown section "operator-trivy"
 - #15401 unknown section "operator-trivy"
 - #15401 unknown section "operator-trivy"
 - #15401 unknown section "operator-trivy"
 - #15637 unknown section "operator-trivy"
 - #15669 unknown section "operator-trivy"
 - #15881 unknown section "operator-trivy"
 - #15909 unknown section "operator-trivy"
 - #15977 unknown section "operator-trivy"
 - #15992 unknown section "operator-trivy"
 - #16006 unknown section "operator-trivy"
 - #16085 unknown section "operator-trivy"
 - #16194 unknown section "operator-trivy"
 - #16277 unknown section "operator-trivy"
 - #16338 unknown section "operator-trivy"
 - #16445 unknown section "operator-trivy"
 - #16489 unknown section "operator-trivy"

## Know before update


 - ALL pods of the ingress-nginx module will be restarted.
 - Deckhouse now has privileged mode and runs as root.
 - Fixed multiple security vulnerabilities that could affect authentication components.
 - The runtime-audit-engine module has been moved to external. All pods of the module will be restarted.
 - This update fixes a security vulnerability in the `user-authn` module (CVE-2025-22868) that could potentially allow bypass of authentication validation.
 - Users should update to this patch release to mitigate known security vulnerabilities. No breaking changes expected.

## Features


 - **[admission-policy-engine]** Added a filter that allows vulnerable images with only specified severity levels to run. [#16049](https://github.com/deckhouse/deckhouse/pull/16049)
 - **[admission-policy-engine]** Validate CONNECT requests for pods/exec and pods/attach in the ValidatingWebhookConfiguration. [#15872](https://github.com/deckhouse/deckhouse/pull/15872)
    Enables Gatekeeper constraints to act on CONNECT (kubectl exec) events; default behavior unchanged unless such constraints are created.
 - **[admission-policy-engine]** Added `allowRbacWildcards` SecurityPolicy flag and Gatekeeper template to restrict `*` in Role and RoleBinding. [#15567](https://github.com/deckhouse/deckhouse/pull/15567)
 - **[admission-policy-engine]** Added OperationPolicy knob pods.disallowedTolerations and enable DELETE admission by default in Gatekeeper webhook. [#15457](https://github.com/deckhouse/deckhouse/pull/15457)
 - **[candi]** Added support of the new module csi-vsphere. [#14549](https://github.com/deckhouse/deckhouse/pull/14549)
 - **[candi]** Added metadata to VCD objects such as networks, virtual machines, and disks. [#14505](https://github.com/deckhouse/deckhouse/pull/14505)
 - **[cloud-provider-dvp]** Changed go_lib path for cse needs. [#15445](https://github.com/deckhouse/deckhouse/pull/15445)
 - **[cloud-provider-dvp]** Added support for the additionalDisks parameter in DVPInstanceClass. [#15121](https://github.com/deckhouse/deckhouse/pull/15121)
 - **[cloud-provider-dvp]** Added default CNI for DVP provider. [#15084](https://github.com/deckhouse/deckhouse/pull/15084)
 - **[cloud-provider-huaweicloud]** Added default CNI for huawei cloud provider. [#15097](https://github.com/deckhouse/deckhouse/pull/15097)
 - **[cloud-provider-vcd]** Enable load balancer feature. [#15934](https://github.com/deckhouse/deckhouse/pull/15934)
 - **[cloud-provider-vcd]** Added metadata to VCD objects such as networks, virtual machines, and disks. [#14505](https://github.com/deckhouse/deckhouse/pull/14505)
 - **[cloud-provider-vsphere]** Added storage_policy support. [#14786](https://github.com/deckhouse/deckhouse/pull/14786)
 - **[cloud-provider-zvirt]** Added support zvirt cloud provider to cse. [#14683](https://github.com/deckhouse/deckhouse/pull/14683)
 - **[control-plane-manager]** Added extra claim `user-authn.deckhouse.io/dex-provider` (from `federated_claims.connector_id`) Request `federated:id` scope in Dex Authenticator, Basic Auth Proxy, and kubeconfig generator to populate. `federated_claims.connector_id` [#15816](https://github.com/deckhouse/deckhouse/pull/15816)
 - **[deckhouse]** Make deckhouse privileged and run as root. [#15664](https://github.com/deckhouse/deckhouse/pull/15664)
    Deckhouse now has privileged mode and runs as root.
 - **[deckhouse]** Added alert for deprecated modules. [#15483](https://github.com/deckhouse/deckhouse/pull/15483)
 - **[deckhouse]** Added Deckhouse release information status. [#15458](https://github.com/deckhouse/deckhouse/pull/15458)
 - **[deckhouse]** Made the module source `deckhouse` the default source. [#15437](https://github.com/deckhouse/deckhouse/pull/15437)
 - **[deckhouse]** Added Nelm integration into Deckhouse as a replacement for Helm. [#15373](https://github.com/deckhouse/deckhouse/pull/15373)
 - **[deckhouse]** Added alert about new major versions count relatively to current major version. [#15278](https://github.com/deckhouse/deckhouse/pull/15278)
 - **[deckhouse]** Added optional module requirements. [#15136](https://github.com/deckhouse/deckhouse/pull/15136)
 - **[deckhouse]** Added inject registry to values. [#14991](https://github.com/deckhouse/deckhouse/pull/14991)
 - **[deckhouse-controller]** Added deckhouse release information into events. [#15547](https://github.com/deckhouse/deckhouse/pull/15547)
 - **[deckhouse-controller]** Tuned DeckhouseHighMemoryUsage alert (namespace grouping, 0.85 threshold, 30s for). [#15543](https://github.com/deckhouse/deckhouse/pull/15543)
 - **[deckhouse-controller]** Added support for an LTS channel for module updates in Deckhouse. [#15321](https://github.com/deckhouse/deckhouse/pull/15321)
 - **[deckhouse-controller]** Added structured webhook response contract with status codes. [#15256](https://github.com/deckhouse/deckhouse/pull/15256)
 - **[deckhouse-controller]** Added coordination logic for Deckhouse restart operations during concurrent module deployments. [#15156](https://github.com/deckhouse/deckhouse/pull/15156)
    Deckhouse will now restart only after all concurrent module releases finish their ApplyRelease operations, reducing the number of restarts during bulk module updates and improving deployment reliability.
 - **[deckhouse-controller]** Added new objects to debug archive. [#15047](https://github.com/deckhouse/deckhouse/pull/15047)
 - **[deckhouse-controller]** Added conversion rules exposure in ModuleSettingsDefinition object. [#15032](https://github.com/deckhouse/deckhouse/pull/15032)
    ModuleSettingsDefinition objects now include conversion rules in the conversions field, enabling users to preview how module settings will be transformed between versions.
 - **[deckhouse-controller]** Updated addon-operator dependency to the latest version. [#14962](https://github.com/deckhouse/deckhouse/pull/14962)
 - **[deckhouse-controller]** Updated the logic of processing modules in the ModuleSource. [#14953](https://github.com/deckhouse/deckhouse/pull/14953)
 - **[deckhouse-controller]** Converting a module to external module. [#14536](https://github.com/deckhouse/deckhouse/pull/14536)
    The runtime-audit-engine module has been moved to external. All pods of the module will be restarted.
 - **[dhctl]** Allowed dhctl to work with readonly root fs. [#15471](https://github.com/deckhouse/deckhouse/pull/15471)
 - **[dhctl]** Added clearer error messages when resource creation times out. [#15310](https://github.com/deckhouse/deckhouse/pull/15310)
    Improved user experience when dhctl cannot create resources due to missing worker nodes.
 - **[docs]** Added manifest for internal LB VK Cloud. [#16057](https://github.com/deckhouse/deckhouse/pull/16057)
 - **[docs]** Added new documentation structure. [#12192](https://github.com/deckhouse/deckhouse/pull/12192)
 - **[documentation]** Bump HuGo to v0.150.1 [#12192](https://github.com/deckhouse/deckhouse/pull/12192)
 - **[ingress-nginx]** The metric `geoip_errors_total` is added, indicating the number of errors when downloading geo ip databases from the MaxMind service. [#14889](https://github.com/deckhouse/deckhouse/pull/14889)
    Ingress-controller pods will restart.
 - **[ingress-nginx]** Added NGINX memory profiling for the Ingress controller. [#14736](https://github.com/deckhouse/deckhouse/pull/14736)
 - **[istio]** Added access log format setting for proxy sidecars. [#16129](https://github.com/deckhouse/deckhouse/pull/16129)
 - **[istio]** Added PSS restriction for api-proxy and  ingressgateway. [#15791](https://github.com/deckhouse/deckhouse/pull/15791)
 - **[node-manager]** Disabled update system packages index during boot cloud ephemeral nodes. [#15859](https://github.com/deckhouse/deckhouse/pull/15859)
 - **[registry]** Added configurable unmanaged mode. [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[registry]** Added registry default and relax check modes. [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[registry]** Disable change registry helper when registry module is enabled. [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[upmeter]** Added the ability to pass headers in RW. [#15533](https://github.com/deckhouse/deckhouse/pull/15533)
 - **[user-authn]** Add DexProvider .spec.enabled flag (default true) and kubectl printer columns; skip disabled providers in Dex connectors. [#16007](https://github.com/deckhouse/deckhouse/pull/16007)
 - **[user-authn]** Enforce lowercase emails for User on create and on email changes; case-insensitive email uniqueness; backward-compatible for legacy uppercase emails. [#15960](https://github.com/deckhouse/deckhouse/pull/15960)
 - **[user-authn]** Increase Dex AuthRequest flexibility with token-bucket rate-limiting and global ResourceQuota. [#15421](https://github.com/deckhouse/deckhouse/pull/15421)
 - **[user-authn]** Dex can run even if one of OIDC providers is not reachable. It resolves the issue when a single unreachable provider can compromise authentication in the cluster. [#15379](https://github.com/deckhouse/deckhouse/pull/15379)
 - **[user-authn]** Propagate proxy envs to dex to allow requesting OIDC discovery endpoints in closed environments. [#15292](https://github.com/deckhouse/deckhouse/pull/15292)
 - **[user-authn]** Add `status.lock` fields (`state`, `reason`, `message`, `until`) to the User CR [#15158](https://github.com/deckhouse/deckhouse/pull/15158)
    User lock information is now available directly in the User CR, improving visibility and integration with external systems.

## Fixes


 - **[admission-policy-engine]** Prohibit only creation or modification for objects with vulnerable images [#16134](https://github.com/deckhouse/deckhouse/pull/16134)
 - **[admission-policy-engine]** Fixed proxy support for trivy-provider [#16113](https://github.com/deckhouse/deckhouse/pull/16113)
 - **[admission-policy-engine]** Fixed GHSA-vrw8-fxc6-2r93. [#16037](https://github.com/deckhouse/deckhouse/pull/16037)
 - **[admission-policy-engine]** Fix CVE. [#15966](https://github.com/deckhouse/deckhouse/pull/15966)
 - **[admission-policy-engine]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[basic-auth]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[candi]** Add missing debian 13 version to detect_bundle [#16617](https://github.com/deckhouse/deckhouse/pull/16617)
 - **[candi]** Reduce auditd pressure around containerd to avoid kernel soft lockups on Linux 5.x nodes. [#15986](https://github.com/deckhouse/deckhouse/pull/15986)
 - **[candi]** Fixed dhctl skipping errors when it's fetching packages. [#15971](https://github.com/deckhouse/deckhouse/pull/15971)
 - **[candi]** Improved check another containerd service on first run. [#15902](https://github.com/deckhouse/deckhouse/pull/15902)
 - **[candi]** Added ContainerdV2 case in tpl kubelet configuration. [#15850](https://github.com/deckhouse/deckhouse/pull/15850)
 - **[candi]** Added exit 1 for check_containerd_v2_support step if set_labels() func failure. [#15792](https://github.com/deckhouse/deckhouse/pull/15792)
 - **[candi]** Fixed segfault in mkfs.erofs. [#15715](https://github.com/deckhouse/deckhouse/pull/15715)
 - **[candi]** Made kubectl-exec a direct request to the control plane if the proxy is unavailable. [#15279](https://github.com/deckhouse/deckhouse/pull/15279)
 - **[candi]** Added missing volumeTypeMap property for nodeGroups. [#15144](https://github.com/deckhouse/deckhouse/pull/15144)
 - **[candi]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cert-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[chrony]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cilium-hubble]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-aws]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-azure]** fix build image for azure ccm [#16560](https://github.com/deckhouse/deckhouse/pull/16560)
 - **[cloud-provider-azure]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-dvp]** Added functionality to wait for a disk to be attached to a VM [#16965](https://github.com/deckhouse/deckhouse/pull/16965)
 - **[cloud-provider-dvp]** add missing field to Cluster [#16399](https://github.com/deckhouse/deckhouse/pull/16399)
 - **[cloud-provider-dvp]** Correct the calculation of the path to the device [#16212](https://github.com/deckhouse/deckhouse/pull/16212)
 - **[cloud-provider-dvp]** Added sshPublicKey to registration secret. [#15859](https://github.com/deckhouse/deckhouse/pull/15859)
 - **[cloud-provider-dvp]** Fixed CVE-2025-22870 && CVE-2025-22872. [#15730](https://github.com/deckhouse/deckhouse/pull/15730)
 - **[cloud-provider-dvp]** Fixed CVE-2025-22868. [#15396](https://github.com/deckhouse/deckhouse/pull/15396)
 - **[cloud-provider-dvp]** Fix CVE-2025-22869 && CVE-2024-45337. [#15390](https://github.com/deckhouse/deckhouse/pull/15390)
 - **[cloud-provider-dvp]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-dynamix]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-gcp]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-huaweicloud]** fix CSI unpublishValidation for non exist ECS instance [#16916](https://github.com/deckhouse/deckhouse/pull/16916)
 - **[cloud-provider-huaweicloud]** fix Provider ID [#16705](https://github.com/deckhouse/deckhouse/pull/16705)
 - **[cloud-provider-huaweicloud]** Fixed providerID format and exclude 127.0.0.0/8 in node IP selection. [#15183](https://github.com/deckhouse/deckhouse/pull/15183)
 - **[cloud-provider-huaweicloud]** Added missing volumeTypeMap property for nodeGroups. [#15144](https://github.com/deckhouse/deckhouse/pull/15144)
 - **[cloud-provider-huaweicloud]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-openstack]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-vcd]** Add ability to define affinity rules for VCD VMs. [#15331](https://github.com/deckhouse/deckhouse/pull/15331)
 - **[cloud-provider-vcd]** Added validation to provider.server in VCDClusterConfiguration to ensure it does not end with '/'. [#15185](https://github.com/deckhouse/deckhouse/pull/15185)
 - **[cloud-provider-vcd]** Fixed fetching VM templates from organization catalogs without direct access to organizastion. [#14980](https://github.com/deckhouse/deckhouse/pull/14980)
 - **[cloud-provider-vcd]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-vsphere]** fix stale session for cloud-data-discoverer [#17089](https://github.com/deckhouse/deckhouse/pull/17089)
 - **[cloud-provider-vsphere]** fix cloud-data-discoverer (SPBM) [#16589](https://github.com/deckhouse/deckhouse/pull/16589)
 - **[cloud-provider-vsphere]** fix vSphere storageClass template [#16275](https://github.com/deckhouse/deckhouse/pull/16275)
 - **[cloud-provider-yandex]** cloud-provider-yandex CVE's was fixed [#16611](https://github.com/deckhouse/deckhouse/pull/16611)
 - **[cloud-provider-yandex]** Terraform auto converger was failed for WithNATInstance layout. [#16427](https://github.com/deckhouse/deckhouse/pull/16427)
 - **[cloud-provider-yandex]** Change machine drain logic and keeps it in LB before drain. [#15255](https://github.com/deckhouse/deckhouse/pull/15255)
 - **[cloud-provider-yandex]** Updated yandex-csi-plugin, set CSI driver metadata querying timeouts. [#15054](https://github.com/deckhouse/deckhouse/pull/15054)
 - **[cloud-provider-yandex]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cni-cilium]** The MTU configuration has been updated. [#16751](https://github.com/deckhouse/deckhouse/pull/16751)
    The MTU will be updated on all interfaces of all pods.
 - **[cni-cilium]** Some issues have been fixed in the EgressGateway. [#16479](https://github.com/deckhouse/deckhouse/pull/16479)
 - **[cni-cilium]** Optimized EgressGateways controller, controller cpu consumption reduced. [#15509](https://github.com/deckhouse/deckhouse/pull/15509)
 - **[cni-cilium]** Fixed egress gateway reselection for case node hard reset. [#15090](https://github.com/deckhouse/deckhouse/pull/15090)
 - **[cni-cilium]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cni-flannel]** Used mode HostGW as default for podNetworkMode. [#15710](https://github.com/deckhouse/deckhouse/pull/15710)
 - **[control-plane-manager]** Fix  “etcd join” phase for control-plane scaling in v1.33. [#16660](https://github.com/deckhouse/deckhouse/pull/16660)
    Allows scaling control-plane from 1→3 in clusters where ControlPlaneKubeletLocalMode=true.
 - **[control-plane-manager]** Add vex for CVE-2025-31133, CVE-2025-52881 . [#16337](https://github.com/deckhouse/deckhouse/pull/16337)
 - **[control-plane-manager]** Append audit policies for virtualization before appending custom policies from Secret. [#15603](https://github.com/deckhouse/deckhouse/pull/15603)
 - **[control-plane-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[dashboard]** Fixed CVE-2025-30204 by updating dashboard components [#16927](https://github.com/deckhouse/deckhouse/pull/16927)
 - **[dashboard]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[deckhouse]** Fix module enabling. [#17043](https://github.com/deckhouse/deckhouse/pull/17043)
 - **[deckhouse]** Fix validation logic for a disabled module [#16385](https://github.com/deckhouse/deckhouse/pull/16385)
 - **[deckhouse]** Automatically set node ip to deckhouse pod during bootstrap phase to no_proxy env. [#15978](https://github.com/deckhouse/deckhouse/pull/15978)
 - **[deckhouse]** Added fixes for resources to nelm usage in DKP. [#15915](https://github.com/deckhouse/deckhouse/pull/15915)
 - **[deckhouse]** Setting embedded source for embedded modules. [#15590](https://github.com/deckhouse/deckhouse/pull/15590)
 - **[deckhouse]** Fixed shell-operator http client to handle resources correctly. [#15182](https://github.com/deckhouse/deckhouse/pull/15182)
 - **[deckhouse]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[deckhouse-controller]** Fix conversions for external modules [#16851](https://github.com/deckhouse/deckhouse/pull/16851)
 - **[deckhouse-controller]** Fix "multiple readiness hooks found" error on hook registration retry after failure. [#16776](https://github.com/deckhouse/deckhouse/pull/16776)
 - **[deckhouse-controller]** Fixed verifying migrated modules [#16693](https://github.com/deckhouse/deckhouse/pull/16693)
 - **[deckhouse-controller]** fix conversion applying for external modules [#16656](https://github.com/deckhouse/deckhouse/pull/16656)
 - **[deckhouse-controller]** Fixed Deckhouse update accidentally minor skip. [#16096](https://github.com/deckhouse/deckhouse/pull/16096)
 - **[deckhouse-controller]** Ensure `afterDeleteHelm` hooks receive Kubernetes snapshots by stopping monitors after hook execution. [#15617](https://github.com/deckhouse/deckhouse/pull/15617)
 - **[deckhouse-controller]** Fixed panic on snapshotIter. [#15385](https://github.com/deckhouse/deckhouse/pull/15385)
 - **[deckhouse-controller]** Reduced memory limit floor for falco containers. [#15301](https://github.com/deckhouse/deckhouse/pull/15301)
 - **[deckhouse-controller]** Reduce deckhouse-controller startup time by optimizing file operations and making cleanup asynchronous. [#15250](https://github.com/deckhouse/deckhouse/pull/15250)
 - **[deckhouse-controller]** Fixed bug with re-enabled module using old values. [#15045](https://github.com/deckhouse/deckhouse/pull/15045)
 - **[deckhouse-controller]** Implement structured releaseQueueDepth calculation with hierarchical version delta tracking. [#15031](https://github.com/deckhouse/deckhouse/pull/15031)
    The releaseQueueDepth metric now accurately reflects actionable release gaps with patch version normalization; major version tracking added for future alerting.
 - **[deckhouse-tools]** Added tolerations support to DexAuthenticator configuration. [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[deckhouse-tools]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[descheduler]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[dhctl]** Fix parallel bootstrap cloud permanent nodes [#16886](https://github.com/deckhouse/deckhouse/pull/16886)
 - **[dhctl]** Fix panic during destroy. Change opentofu log level to INFO. [#16726](https://github.com/deckhouse/deckhouse/pull/16726)
 - **[dhctl]** Fix StaticInstance readiness check and refactoring readiness check for resources. [#16616](https://github.com/deckhouse/deckhouse/pull/16616)
 - **[dhctl]** Fix converge manifests for static cluster in commander. [#16504](https://github.com/deckhouse/deckhouse/pull/16504)
 - **[dhctl]** Validate WithNATInstance Yandex layout params only in bootstrap. [#16427](https://github.com/deckhouse/deckhouse/pull/16427)
 - **[dhctl]** Added nil check to dhctl during converge in migrator [#16289](https://github.com/deckhouse/deckhouse/pull/16289)
 - **[dhctl]** Fix getting passphrase for key from connection config for cli. [#16100](https://github.com/deckhouse/deckhouse/pull/16100)
 - **[dhctl]** Move yandex withNATInstance layout settings from preflights to preparator. [#16100](https://github.com/deckhouse/deckhouse/pull/16100)
 - **[dhctl]** Stop all kube proxies during destroy. Improve Do not lock converge for static clusters and save information about lock in state. [#16059](https://github.com/deckhouse/deckhouse/pull/16059)
 - **[dhctl]** Prompt user about static cluster bootstrap on current host. [#16011](https://github.com/deckhouse/deckhouse/pull/16011)
 - **[dhctl]** Added terminfo for proper terminal behavior in dhctl and deckhouse containers. [#15988](https://github.com/deckhouse/deckhouse/pull/15988)
 - **[dhctl]** Not start client if resources were destroyed. [#15952](https://github.com/deckhouse/deckhouse/pull/15952)
 - **[dhctl]** Fix misbehavior in gossh client. [#15759](https://github.com/deckhouse/deckhouse/pull/15759)
 - **[dhctl]** Skip cluster-admin role creation if it already exists. [#15562](https://github.com/deckhouse/deckhouse/pull/15562)
 - **[dhctl]** Added more phases to static destroy progress tracker. [#15538](https://github.com/deckhouse/deckhouse/pull/15538)
 - **[dhctl]** Fixed trigger control-plane pre/post hooks only if a node is being recreated/deleted. [#14998](https://github.com/deckhouse/deckhouse/pull/14998)
 - **[dhctl]** Fix output klog. Wrap klog logs and redirect to our logger. [#14195](https://github.com/deckhouse/deckhouse/pull/14195)
 - **[docs]** Added description about custom CoreDNS installation. [#16092](https://github.com/deckhouse/deckhouse/pull/16092)
 - **[documentation]** Added tolerations support to DexAuthenticator configuration. [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[documentation]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[extended-monitoring]** drop metrics when extended monitoring is disabled for node(s) [#16446](https://github.com/deckhouse/deckhouse/pull/16446)
    erroneous alerts for node disk usage are fixed
 - **[extended-monitoring]** Fix extended-monitoring.deckhouse.io/enabled label handling [#16372](https://github.com/deckhouse/deckhouse/pull/16372)
    the extended monitoring will only be enabled when the label is explicitly set on a namespace
 - **[extended-monitoring]** Init extended-monitoring-exporter on unavailable API. [#15529](https://github.com/deckhouse/deckhouse/pull/15529)
 - **[extended-monitoring]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[ingress-nginx]** A symlink to the new opentelemetry config path is added. [#16433](https://github.com/deckhouse/deckhouse/pull/16433)
    Ingress-Nginx controller's pods of 1.9 version will be restarted.
 - **[ingress-nginx]** Fixed CVEs [#16432](https://github.com/deckhouse/deckhouse/pull/16432)
 - **[ingress-nginx]** CVEs fixed [#16340](https://github.com/deckhouse/deckhouse/pull/16340)
 - **[ingress-nginx]** Fixed CVEs, found in auxiliary source code. [#16069](https://github.com/deckhouse/deckhouse/pull/16069)
 - **[ingress-nginx]** Fixed CVE CVE-2025-5187. [#15906](https://github.com/deckhouse/deckhouse/pull/15906)
 - **[ingress-nginx]** Fixed CVE's. [#15776](https://github.com/deckhouse/deckhouse/pull/15776)
 - **[ingress-nginx]** Fixed nginx image build for ingress controller and template tests. [#15464](https://github.com/deckhouse/deckhouse/pull/15464)
 - **[ingress-nginx]** Disabled log messages such as `Error obtaining Endpoints for Service...`. [#15260](https://github.com/deckhouse/deckhouse/pull/15260)
 - **[ingress-nginx]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[istio]** fixing the CVE in Kiali [#17045](https://github.com/deckhouse/deckhouse/pull/17045)
 - **[istio]** The same owner is specified for the files that are used to run in the operator container. [#16154](https://github.com/deckhouse/deckhouse/pull/16154)
 - **[istio]** Resolve CVE's. [#15834](https://github.com/deckhouse/deckhouse/pull/15834)
 - **[istio]** Fixed metrics port for operator 1.25 and newer. [#15124](https://github.com/deckhouse/deckhouse/pull/15124)
 - **[istio]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[kube-dns]** Improved /etc/hosts renderer compatibility with admission-policy-engine Restricted mode. [#16599](https://github.com/deckhouse/deckhouse/pull/16599)
 - **[kube-dns]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[kube-proxy]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[local-path-provisioner]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[log-shipper]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[loki]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[metallb]** Fixed CVE's. [#15777](https://github.com/deckhouse/deckhouse/pull/15777)
 - **[metallb]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes]** remove the Docker traces from the module code [#16542](https://github.com/deckhouse/deckhouse/pull/16542)
    node-exporter pods will be rollout restarted during upgrade
 - **[monitoring-kubernetes]** Rollout changes for resources metrics kubelet [#16408](https://github.com/deckhouse/deckhouse/pull/16408)
 - **[monitoring-kubernetes]** fix CVE-2025-52881 for node-exporter [#16376](https://github.com/deckhouse/deckhouse/pull/16376)
 - **[monitoring-kubernetes]** Fixed gaps on graph. [#15479](https://github.com/deckhouse/deckhouse/pull/15479)
 - **[monitoring-kubernetes]** Added `tier=cluster` label. [#15290](https://github.com/deckhouse/deckhouse/pull/15290)
 - **[monitoring-kubernetes]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes-control-plane]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-ping]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[multitenancy-manager]** fix CVE-2024-25621  CVE-2025-64329 [#16360](https://github.com/deckhouse/deckhouse/pull/16360)
 - **[multitenancy-manager]** Patched critical CVEs in dependencies. [#15312](https://github.com/deckhouse/deckhouse/pull/15312)
    Users should update to this patch release to mitigate known security vulnerabilities. No breaking changes expected.
 - **[multitenancy-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[node-local-dns]** Fixed CVE-2025-59530 and updated CoreDNS to version 1.13.1. [#15965](https://github.com/deckhouse/deckhouse/pull/15965)
 - **[node-manager]** Fix panic in registry packages proxy if image not found. [#16425](https://github.com/deckhouse/deckhouse/pull/16425)
 - **[node-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[openvpn]** ovpn-admin upgraded to fix the validation of static IP addresses, as well as add routes migration during the rotation of client certificates, openvpn instances will be restarted. [#14578](https://github.com/deckhouse/deckhouse/pull/14578)
 - **[openvpn]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[operator-prometheus]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[prometheus]** Fix namespace label value in the Ingress Nginx controller and several other metrics [#16720](https://github.com/deckhouse/deckhouse/pull/16720)
    Ingress Nginx controller dashboards are fixed
 - **[prometheus]** Fix description for not usable CVE [#16377](https://github.com/deckhouse/deckhouse/pull/16377)
 - **[prometheus]** Fixed template indentation [#15434](https://github.com/deckhouse/deckhouse/pull/15434)
 - **[prometheus]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[prometheus-metrics-adapter]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[registry]** bump go_lib/registry dependencies [#15985](https://github.com/deckhouse/deckhouse/pull/15985)
 - **[registry-packages-proxy]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[registrypackages]** Update integrity patch for containerd (cse only). [#17028](https://github.com/deckhouse/deckhouse/pull/17028)
 - **[registrypackages]** Update integrity patch for containerd (cse only). [#17000](https://github.com/deckhouse/deckhouse/pull/17000)
 - **[registrypackages]** Update containerd to 1.7.29 / 2.1.5 and runc to 1.3.3 [#16335](https://github.com/deckhouse/deckhouse/pull/16335)
 - **[registrypackages]** Fixes CVE in kubernetes-cni [#16343](https://github.com/deckhouse/deckhouse/pull/16343)
 - **[registrypackages]** Update runc to 1.3.1. [#16263](https://github.com/deckhouse/deckhouse/pull/16263)
 - **[service-with-healthchecks]** Fixed CVEs [#16950](https://github.com/deckhouse/deckhouse/pull/16950)
 - **[service-with-healthchecks]** Improved the module's security [#15358](https://github.com/deckhouse/deckhouse/pull/15358)
 - **[service-with-healthchecks]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[terraform-manager]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[upmeter]** fix securityxontext for statefulset [#16534](https://github.com/deckhouse/deckhouse/pull/16534)
    upmeter check
 - **[upmeter]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[user-authn]** skipApproval no longer bypasses TOTP. When 2FA is enabled, users are sent to /totp before approval, so “auth request does not have an identity for approval” no longer occurs [#16946](https://github.com/deckhouse/deckhouse/pull/16946)
 - **[user-authn]** Fix BadRequest after the change password redirect when password policy is enabled [#16744](https://github.com/deckhouse/deckhouse/pull/16744)
 - **[user-authn]** Fix login error 500 with password policy enabled. [#16703](https://github.com/deckhouse/deckhouse/pull/16703)
 - **[user-authn]** Rollback patch for handling insecureSkipEmailVerified condition [#16347](https://github.com/deckhouse/deckhouse/pull/16347)
 - **[user-authn]** In the latest go versions (1.25.2, 1.24.8) the https://github.com/golang/go/issues/75712, and now Dex fails with an error. This patch makes Dex wrap only IPv6 addresses in brackets, which is more correct. [#15890](https://github.com/deckhouse/deckhouse/pull/15890)
 - **[user-authn]** When insecureSkipEmailVerified is enabled remove the email_verified claim from identity. [#15869](https://github.com/deckhouse/deckhouse/pull/15869)
    When enabled, Dex will remove email_verified from emitted identity/claims.
 - **[user-authn]** Fixed Dex password policy 'Excellent' rule — allow two identical characters in a row, reject three or more. [#15868](https://github.com/deckhouse/deckhouse/pull/15868)
    Fixes incorrect rejection of valid strong passwords.
 - **[user-authn]** Fixed cert generation job deletion. [#15764](https://github.com/deckhouse/deckhouse/pull/15764)
 - **[user-authn]** Show 'Access Denied' instead of 'Internal Error' for restricted local users. [#15593](https://github.com/deckhouse/deckhouse/pull/15593)
    Users will see a clear error message when their login is restricted by allowed group or email, instead of a confusing internal error.
 - **[user-authn]** Ensure validity of names for DexAuthenticator resources (truncate >63 and add deterministic 5-char hash suffix). [#15544](https://github.com/deckhouse/deckhouse/pull/15544)
 - **[user-authn]** Fix CVE-2025-22868 [#15420](https://github.com/deckhouse/deckhouse/pull/15420)
    This update fixes a security vulnerability in the `user-authn` module (CVE-2025-22868) that could potentially allow bypass of authentication validation.
 - **[user-authn]** Fix CVE-2025-30204, CVE-2025-22868, and CVE-2024-28180 in the user-authn module [#15208](https://github.com/deckhouse/deckhouse/pull/15208)
    Fixed multiple security vulnerabilities that could affect authentication components.
 - **[user-authn]** User now can't create groups with  recursive loops in nested group's hierarchy. [#15139](https://github.com/deckhouse/deckhouse/pull/15139)
 - **[user-authn]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[user-authz]** cache namespace label checks in the user-authz webhook via informer to avoid per-request apiserver GETs [#16920](https://github.com/deckhouse/deckhouse/pull/16920)
 - **[vertical-pod-autoscaler]** Update container configurations to use improvement securityContext. [#13577](https://github.com/deckhouse/deckhouse/pull/13577)

## Chore


 - **[admission-policy-engine]** Made trivy-provider set readOnlyRootFilesystem. [#15837](https://github.com/deckhouse/deckhouse/pull/15837)
 - **[admission-policy-engine]** Updated dependencies to fix CVE's. [#15459](https://github.com/deckhouse/deckhouse/pull/15459)
 - **[admission-policy-engine]** Fixed CVE's. [#15237](https://github.com/deckhouse/deckhouse/pull/15237)
 - **[candi]** Bump patch versions of Kubernetes images. [#15422](https://github.com/deckhouse/deckhouse/pull/15422)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[candi]** Update Deckhouse CLI (d8) version to 0.17.0. [#15251](https://github.com/deckhouse/deckhouse/pull/15251)
 - **[candi]** Changes for launching container v2 with signed images. [#15249](https://github.com/deckhouse/deckhouse/pull/15249)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.16.0. [#15111](https://github.com/deckhouse/deckhouse/pull/15111)
 - **[cert-manager]** Increase timeout for admission webhooks to 30 seconds. [#15847](https://github.com/deckhouse/deckhouse/pull/15847)
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
 - **[cni-cilium]** Improved the security of cilium containers of the CNI plugin [#15494](https://github.com/deckhouse/deckhouse/pull/15494)
    cilium pods will be restarted, network traffic may be interrupted.
 - **[cni-cilium]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-cilium]** Images for 1.17 were refactored to achieve distroless. [#14192](https://github.com/deckhouse/deckhouse/pull/14192)
 - **[cni-flannel]** The readOnlyRootFilesystem security option is set to true for all containers. [#15444](https://github.com/deckhouse/deckhouse/pull/15444)
    Pods of the cni-flannel module will be restarted.
 - **[cni-flannel]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-simple-bridge]** The readOnlyRootFilesystem security option is set to true for all containers. [#15476](https://github.com/deckhouse/deckhouse/pull/15476)
    Pods of the cni-simple-bridge module will be restarted.
 - **[cni-simple-bridge]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[common]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[common]** Fixed CVE in kube-apiserver. [#15893](https://github.com/deckhouse/deckhouse/pull/15893)
 - **[deckhouse]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse]** Bump addon-operator. [#15884](https://github.com/deckhouse/deckhouse/pull/15884)
 - **[deckhouse]** Updated d8 to 0.20.3. [#15845](https://github.com/deckhouse/deckhouse/pull/15845)
 - **[deckhouse]** Updated d8 to 0.20.0 [#15817](https://github.com/deckhouse/deckhouse/pull/15817)
 - **[deckhouse]** Added check modules sign. [#15450](https://github.com/deckhouse/deckhouse/pull/15450)
 - **[deckhouse-controller]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[deckhouse-controller]** Check lowercased scheme in ChangeRegistry function. [#15197](https://github.com/deckhouse/deckhouse/pull/15197)
 - **[deckhouse-tools]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[dhctl]** Fix CVE in dhctl go.mod. [#15878](https://github.com/deckhouse/deckhouse/pull/15878)
 - **[docs]** Add NGC examples for automatically installation of NVIDIA drivers. [#16864](https://github.com/deckhouse/deckhouse/pull/16864)
 - **[extended-monitoring]** Migrated to golang. [#15781](https://github.com/deckhouse/deckhouse/pull/15781)
 - **[extended-monitoring]** Added FAQ document. [#15640](https://github.com/deckhouse/deckhouse/pull/15640)
 - **[ingress-nginx]** Fix CVEs in sources [#16190](https://github.com/deckhouse/deckhouse/pull/16190)
 - **[ingress-nginx]** The readOnlyRootFilesystem security option is set to true for all containers. [#15496](https://github.com/deckhouse/deckhouse/pull/15496)
    ALL pods of the ingress-nginx module will be restarted.
 - **[ingress-nginx]** Switched to a distroless base image for the ingress controller v1.12, reducing its size and fixing multiple CVEs. [#14469](https://github.com/deckhouse/deckhouse/pull/14469)
    Ingress controller pods will restart.
 - **[ingress-nginx]** Ingress controller now runs under the deckhouse user (instead of www-data). [#14245](https://github.com/deckhouse/deckhouse/pull/14245)
    Ingress-nginx Controllers will be restarted, which will cause traffic interruption.
 - **[istio]** Fix CVEs in sources [#16191](https://github.com/deckhouse/deckhouse/pull/16191)
 - **[istio]** Fix CVE, add vex, rewrite kiali build. [#15983](https://github.com/deckhouse/deckhouse/pull/15983)
 - **[istio]** Corrected permissions of executable files in EE. [#15626](https://github.com/deckhouse/deckhouse/pull/15626)
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
 - **[node-local-dns]** The readOnlyRootFilesystem security option is set to true for all containers. [#15395](https://github.com/deckhouse/deckhouse/pull/15395)
    The node-local-dns pods will be restarted.
 - **[node-local-dns]** Build refactored and improved observability by adding alerts about resolving issues. [#14364](https://github.com/deckhouse/deckhouse/pull/14364)
 - **[node-manager]** Group get_crd errors and make them more readable. [#15591](https://github.com/deckhouse/deckhouse/pull/15591)
 - **[node-manager]** Added sign check and integrity check to the registry-packages-proxy. [#14685](https://github.com/deckhouse/deckhouse/pull/14685)
 - **[openvpn]** The readOnlyRootFilesystem security option is set to true for all containers. [#15346](https://github.com/deckhouse/deckhouse/pull/15346)
    Pods of the openvpn module will be restarted.
 - **[prometheus]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)
 - **[prometheus]** Add svace analyze for mimir image. [#16068](https://github.com/deckhouse/deckhouse/pull/16068)
 - **[prometheus]** Added pre-created paths create folder. [#15832](https://github.com/deckhouse/deckhouse/pull/15832)
 - **[prometheus]** Added svace analys for apps. [#15658](https://github.com/deckhouse/deckhouse/pull/15658)
 - **[prometheus]** Removed deprecated tls certs. [#15638](https://github.com/deckhouse/deckhouse/pull/15638)
 - **[prometheus]** Added POD_IP var to config. [#15527](https://github.com/deckhouse/deckhouse/pull/15527)
 - **[prometheus]** Migrated from old hook logic for enabling prompp. [#15308](https://github.com/deckhouse/deckhouse/pull/15308)
 - **[prometheus]** Deprecated the direct Prometheus access. [#14812](https://github.com/deckhouse/deckhouse/pull/14812)
    Accessing Prometheus via ingress is now considered deprecated and will not be possible in future releases.
 - **[registry]** Update dependencies to fix CVEs [#16635](https://github.com/deckhouse/deckhouse/pull/16635)
 - **[registry]** Fixed CVE's: CVE-2020-26160, CVE-2020-8911, CVE-2020-8912, CVE-2022-21698, CVE-2022-2582, CVE-2025-22868, CVE-2025-22869, CVE-2025-22870, CVE-2025-22872, CVE-2025-27144 [#15235](https://github.com/deckhouse/deckhouse/pull/15235)
 - **[registry-packages-proxy]** Added separate secret to rpp for imagePullSecrets. [#15783](https://github.com/deckhouse/deckhouse/pull/15783)
 - **[registrypackages]** update pip version [#16228](https://github.com/deckhouse/deckhouse/pull/16228)

