# Changelog v1.73

## [MALFORMED]


 - #13577 unknown section "runtime-audit-engine"
 - #14536 unknown section "runtime-audit-engine"
 - #15040 unknown section "runtime-audit-engine"
 - #15301 unknown section "runtime-audit-engine"
 - #15381 unknown section "dvp"

## Know before update


 - ALL pods of the ingress-nginx module will be restarted.
 - Fixes multiple security vulnerabilities that could affect authentication components
 - This update fixes a security vulnerability in the `user-authn` module (CVE-2025-22868)
    that could potentially allow bypass of authentication validation.
 - Users should update to this patch release to mitigate known security vulnerabilities. No breaking changes expected.

## Features


 - **[admission-policy-engine]** Add OperationPolicy knob pods.disallowedTolerations and enable DELETE admission by default in Gatekeeper webhook [#15457](https://github.com/deckhouse/deckhouse/pull/15457)
 - **[candi]** Added support of the new module csi-vsphere. [#14549](https://github.com/deckhouse/deckhouse/pull/14549)
 - **[candi]** Implemented adding metadata to VCD objects such as networks, virtual machines, and disks. [#14505](https://github.com/deckhouse/deckhouse/pull/14505)
 - **[cloud-provider-dvp]** allow creating and attaching additional disks to VM [#15121](https://github.com/deckhouse/deckhouse/pull/15121)
 - **[cloud-provider-dvp]** Now cilium module is enabled if others CNIs not provided [#15084](https://github.com/deckhouse/deckhouse/pull/15084)
 - **[cloud-provider-dvp]** fix single-node LoadBalancer bug and add multi-LB per node support [#14883](https://github.com/deckhouse/deckhouse/pull/14883)
 - **[cloud-provider-huaweicloud]** If no MC CNI specified run cni module by default [#15097](https://github.com/deckhouse/deckhouse/pull/15097)
 - **[cloud-provider-vcd]** Implemented adding metadata to VCD objects such as networks, virtual machines, and disks. [#14505](https://github.com/deckhouse/deckhouse/pull/14505)
 - **[cloud-provider-zvirt]** Added support zvirt cloud provider to cse. [#14683](https://github.com/deckhouse/deckhouse/pull/14683)
 - **[deckhouse]** Add alert for deprecated modules [#15483](https://github.com/deckhouse/deckhouse/pull/15483)
 - **[deckhouse]** The module source deckhouse is the default source for modules. [#15437](https://github.com/deckhouse/deckhouse/pull/15437)
 - **[deckhouse]** Using nelm as a replacement for Helm [#15373](https://github.com/deckhouse/deckhouse/pull/15373)
 - **[deckhouse]** Add alert about new major versions count relatively to current major version [#15278](https://github.com/deckhouse/deckhouse/pull/15278)
 - **[deckhouse]** Optional module requirements. [#15136](https://github.com/deckhouse/deckhouse/pull/15136)
 - **[deckhouse]** Inject registry to module values. [#14991](https://github.com/deckhouse/deckhouse/pull/14991)
 - **[deckhouse-controller]** LTS channel for modules [#15321](https://github.com/deckhouse/deckhouse/pull/15321)
 - **[deckhouse-controller]** add structured webhook response contract with status codes [#15256](https://github.com/deckhouse/deckhouse/pull/15256)
 - **[deckhouse-controller]** Coordinate Deckhouse restart to wait for all concurrent module deployments to complete [#15156](https://github.com/deckhouse/deckhouse/pull/15156)
    Deckhouse will now restart only after all concurrent module releases finish their ApplyRelease operations, reducing the number of restarts during bulk module updates and improving deployment reliability.
 - **[deckhouse-controller]** add new objects to debug archive [#15047](https://github.com/deckhouse/deckhouse/pull/15047)
 - **[deckhouse-controller]** Add conversion rules exposure in ModuleSettingsDefinition object [#15032](https://github.com/deckhouse/deckhouse/pull/15032)
    ModuleSettingsDefinition objects now include conversion rules in the conversions field, enabling users to preview how module settings will be transformed between versions
 - **[deckhouse-controller]** task queue performance improvements with linked list implementation [#14962](https://github.com/deckhouse/deckhouse/pull/14962)
 - **[deckhouse-controller]** handle errors while processing source modules [#14953](https://github.com/deckhouse/deckhouse/pull/14953)
 - **[ingress-nginx]** The metric `geoip_errors_total` is added, indicating the number of errors when downloading geo ip databases from the MaxMind service. [#14889](https://github.com/deckhouse/deckhouse/pull/14889)
    Ingress-controller pods will restart.
 - **[ingress-nginx]** Added NGINX memory profiling for the Ingress controller. [#14736](https://github.com/deckhouse/deckhouse/pull/14736)
 - **[registry]** Added configurable unmanaged mode [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[registry]** Added registry check modes: default, relax [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[registry]** Disable change registry helper when registry module is enabled [#14820](https://github.com/deckhouse/deckhouse/pull/14820)
 - **[upmeter]** Add the ability to pass headers in RW [#15533](https://github.com/deckhouse/deckhouse/pull/15533)
 - **[user-authn]** Increase Dex AuthRequest flexibility with token-bucket rate-limiting and global ResourceQuota [#15421](https://github.com/deckhouse/deckhouse/pull/15421)
 - **[user-authn]** Dex can run even if one of OIDC providers is not reachable. It resolves the issue when a single unreachable provider can compromise authentication in the cluster. [#15379](https://github.com/deckhouse/deckhouse/pull/15379)
 - **[user-authn]** Propagate proxy envs to dex to allow requesting OIDC discovery endpoints in closed environments. [#15292](https://github.com/deckhouse/deckhouse/pull/15292)
 - **[user-authn]** Add `status.lock` fields (`state`, `reason`, `message`, `until`) to the User CR [#15158](https://github.com/deckhouse/deckhouse/pull/15158)
    User lock information is now available directly in the User CR, improving visibility and integration with external systems

## Fixes


 - **[admission-policy-engine]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[basic-auth]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[candi]** kubectl-exec makes a direct request to the control plane if the proxy is unavailable [#15279](https://github.com/deckhouse/deckhouse/pull/15279)
 - **[candi]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cert-manager]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[chrony]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cilium-hubble]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-aws]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-azure]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-dvp]** fix CVE-2025-22868 [#15396](https://github.com/deckhouse/deckhouse/pull/15396)
 - **[cloud-provider-dvp]** Fix CVE-2025-22869 && CVE-2024-45337. [#15390](https://github.com/deckhouse/deckhouse/pull/15390)
 - **[cloud-provider-dvp]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-dynamix]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-gcp]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-huaweicloud]** fix providerID format and exclude 127.0.0.0/8 in node IP selection [#15183](https://github.com/deckhouse/deckhouse/pull/15183)
 - **[cloud-provider-huaweicloud]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-openstack]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-vcd]** add validation to provider.server in VCDClusterConfiguration to ensure it does not end with '/'. [#15185](https://github.com/deckhouse/deckhouse/pull/15185)
 - **[cloud-provider-vcd]** Fix fetching VM templates from organization catalogs without direct access to organizastion [#14980](https://github.com/deckhouse/deckhouse/pull/14980)
 - **[cloud-provider-vcd]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cloud-provider-yandex]** don't remove machine from LB before drain [#15255](https://github.com/deckhouse/deckhouse/pull/15255)
 - **[cloud-provider-yandex]** Set CSI driver metadata querying timeouts [#15054](https://github.com/deckhouse/deckhouse/pull/15054)
 - **[cloud-provider-yandex]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[cni-cilium]** Optimized EgressGateways controller, controller cpu consumption reduced. [#15509](https://github.com/deckhouse/deckhouse/pull/15509)
 - **[cni-cilium]** fixed egress gateway reselection for case node hard reset [#15090](https://github.com/deckhouse/deckhouse/pull/15090)
 - **[cni-cilium]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[control-plane-manager]** Append audit policies for virtualization before appending custom policies from Secret. [#15603](https://github.com/deckhouse/deckhouse/pull/15603)
 - **[control-plane-manager]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[dashboard]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[deckhouse]** fix shell-operator http client to handle resources correctly [#15182](https://github.com/deckhouse/deckhouse/pull/15182)
 - **[deckhouse]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[deckhouse-controller]** fix panic on snapshotIter [#15385](https://github.com/deckhouse/deckhouse/pull/15385)
 - **[deckhouse-controller]** Reduce deckhouse-controller startup time by optimizing file operations and making cleanup asynchronous [#15250](https://github.com/deckhouse/deckhouse/pull/15250)
 - **[deckhouse-controller]** fixed bug with re-enabled module using old values [#15045](https://github.com/deckhouse/deckhouse/pull/15045)
 - **[deckhouse-controller]** Implement structured releaseQueueDepth calculation with hierarchical version delta tracking [#15031](https://github.com/deckhouse/deckhouse/pull/15031)
    The releaseQueueDepth metric now accurately reflects actionable release gaps with patch version normalization; major version tracking added for future alerting
 - **[deckhouse-tools]** add tolerations support to DexAuthenticator configuration [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[deckhouse-tools]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[descheduler]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[dhctl]** Add more phases to static destroy progress tracker [#15538](https://github.com/deckhouse/deckhouse/pull/15538)
 - **[dhctl]** trigger control-plane pre/post hooks only if a node is being recreated/deleted [#14998](https://github.com/deckhouse/deckhouse/pull/14998)
 - **[dhctl]** Fix output klog. Wrap klog logs and redirect to our logger. [#14195](https://github.com/deckhouse/deckhouse/pull/14195)
 - **[documentation]** add tolerations support to DexAuthenticator configuration [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[documentation]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[extended-monitoring]** Init extended-monitoring-exporter on unavailable API. [#15529](https://github.com/deckhouse/deckhouse/pull/15529)
 - **[extended-monitoring]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[ingress-nginx]** Fixed nginx image build for ingress controller and template tests. [#15464](https://github.com/deckhouse/deckhouse/pull/15464)
 - **[ingress-nginx]** Disabled log messages such as `Error obtaining Endpoints for Service...`. [#15260](https://github.com/deckhouse/deckhouse/pull/15260)
 - **[ingress-nginx]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[istio]** Fixed metrics port for operator 1.25 and newer [#15124](https://github.com/deckhouse/deckhouse/pull/15124)
 - **[istio]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[kube-dns]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[kube-proxy]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[local-path-provisioner]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[log-shipper]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[loki]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[metallb]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes]** Fix gaps on graph [#15479](https://github.com/deckhouse/deckhouse/pull/15479)
 - **[monitoring-kubernetes]** Add `tier=cluster` label [#15290](https://github.com/deckhouse/deckhouse/pull/15290)
 - **[monitoring-kubernetes]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-kubernetes-control-plane]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[monitoring-ping]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[multitenancy-manager]** patch critical CVEs in dependencies [#15312](https://github.com/deckhouse/deckhouse/pull/15312)
    Users should update to this patch release to mitigate known security vulnerabilities. No breaking changes expected.
 - **[multitenancy-manager]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[node-manager]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[openvpn]** ovpn-admin upgraded to fix the validation of static IP addresses, as well as add routes migration during the rotation of client certificates. [#14578](https://github.com/deckhouse/deckhouse/pull/14578)
    the openvpn instances will be restarted.
 - **[openvpn]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[operator-prometheus]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[operator-trivy]** Fixed node-collector pods crasing on startup [#15401](https://github.com/deckhouse/deckhouse/pull/15401)
 - **[operator-trivy]** Added a passtrough for a HTTP(s) proxy parameters from operator to vulnerability scanning jobs processes; [#15401](https://github.com/deckhouse/deckhouse/pull/15401)
 - **[operator-trivy]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[prometheus]** fix template indentation [#15434](https://github.com/deckhouse/deckhouse/pull/15434)
    fix template indentation
 - **[prometheus]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[prometheus-metrics-adapter]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[registry-packages-proxy]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[service-with-healthchecks]** The module's security has been improved. [#15358](https://github.com/deckhouse/deckhouse/pull/15358)
 - **[service-with-healthchecks]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[terraform-manager]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[upmeter]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[user-authn]** Show 'Access Denied' instead of 'Internal Error' for restricted local users [#15593](https://github.com/deckhouse/deckhouse/pull/15593)
    Users will see a clear error message when their login is restricted by allowed group or email, instead of a confusing internal error.
 - **[user-authn]** Ensure validity of names for DexAuthenticator resources (truncate >63 and add deterministic 5-char hash suffix) [#15544](https://github.com/deckhouse/deckhouse/pull/15544)
 - **[user-authn]** Fix CVE-2025-22868 [#15420](https://github.com/deckhouse/deckhouse/pull/15420)
    This update fixes a security vulnerability in the `user-authn` module (CVE-2025-22868)
    that could potentially allow bypass of authentication validation.
 - **[user-authn]** Fix CVE-2025-30204, CVE-2025-22868, and CVE-2024-28180 in the user-authn module [#15208](https://github.com/deckhouse/deckhouse/pull/15208)
    Fixes multiple security vulnerabilities that could affect authentication components
 - **[user-authn]** User now can't create groups with  recursive loops in nested group's hierarchy [#15139](https://github.com/deckhouse/deckhouse/pull/15139)
 - **[user-authn]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)
 - **[vertical-pod-autoscaler]** securityContext improvement [#13577](https://github.com/deckhouse/deckhouse/pull/13577)

## Chore


 - **[admission-policy-engine]** Updated dependencies to fix CVE's [#15459](https://github.com/deckhouse/deckhouse/pull/15459)
 - **[admission-policy-engine]** Fix CVE's [#15237](https://github.com/deckhouse/deckhouse/pull/15237)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.17.0. [#15251](https://github.com/deckhouse/deckhouse/pull/15251)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.16.0. [#15111](https://github.com/deckhouse/deckhouse/pull/15111)
 - **[cni-cilium]** Improved the security of cilium containers of the CNI plugin. [#15494](https://github.com/deckhouse/deckhouse/pull/15494)
    cilium pods will be restarted, network traffic may be interrupted.
 - **[cni-cilium]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-cilium]** Images for 1.17 were refactored to achieve distroless. [#14192](https://github.com/deckhouse/deckhouse/pull/14192)
 - **[cni-flannel]** The readOnlyRootFilesystem security option is set to true for all containers. [#15444](https://github.com/deckhouse/deckhouse/pull/15444)
    Pods of the cni-flannel module will be restarted.
 - **[cni-flannel]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[cni-simple-bridge]** The readOnlyRootFilesystem security option is set to true for all containers. [#15476](https://github.com/deckhouse/deckhouse/pull/15476)
    Pods of the cni-simple-bridge module will be restarted.
 - **[cni-simple-bridge]** The new logic implemented where settings from the ModuleConfig take priority over the "d8-cni-configuration" secret. [#15275](https://github.com/deckhouse/deckhouse/pull/15275)
 - **[deckhouse]** Add check modules sign. [#15450](https://github.com/deckhouse/deckhouse/pull/15450)
 - **[deckhouse-controller]** check lowercased scheme in ChangeRegistry function [#15197](https://github.com/deckhouse/deckhouse/pull/15197)
 - **[ingress-nginx]** The readOnlyRootFilesystem security option is set to true for all containers. [#15496](https://github.com/deckhouse/deckhouse/pull/15496)
    ALL pods of the ingress-nginx module will be restarted.
 - **[ingress-nginx]** Switched to a distroless base image for the ingress controller v1.12, reducing its size and fixing multiple CVEs. [#14469](https://github.com/deckhouse/deckhouse/pull/14469)
    ingress controller pods will restart.
 - **[ingress-nginx]** Ingress controller now runs under the deckhouse user (instead of www-data). [#14245](https://github.com/deckhouse/deckhouse/pull/14245)
    ingress-nginx Controllers will be restarted, which will cause traffic interruption.
 - **[istio]** Correcting permissions of executable files in EE [#15626](https://github.com/deckhouse/deckhouse/pull/15626)
 - **[keepalived]** The readOnlyRootFilesystem security option is set to true for all containers. [#15487](https://github.com/deckhouse/deckhouse/pull/15487)
    Pods of the openvpn module will be restarted.
 - **[kube-dns]** The readOnlyRootFilesystem security option is set to true for all containers. [#15391](https://github.com/deckhouse/deckhouse/pull/15391)
    The kube-dns webhook pod will be restarted.
 - **[kube-proxy]** The readOnlyRootFilesystem security option is set to true for all containers. [#15409](https://github.com/deckhouse/deckhouse/pull/15409)
    Pods of the kube-proxy module will be restarted.
 - **[network-gateway]** The readOnlyRootFilesystem security option is set to true for all containers. [#15414](https://github.com/deckhouse/deckhouse/pull/15414)
    Pods of the network-gateway module will be restarted.
 - **[network-policy-engine]** The readOnlyRootFilesystem security option is set to true for all containers. [#15427](https://github.com/deckhouse/deckhouse/pull/15427)
    Pods of the network-policy-engine module will be restarted.
 - **[node-local-dns]** The readOnlyRootFilesystem security option is set to true for all containers. [#15395](https://github.com/deckhouse/deckhouse/pull/15395)
    The node-local-dns pods will be restarted.
 - **[node-local-dns]** Build refactored and improved observability by adding alerts about resolving issues. [#14364](https://github.com/deckhouse/deckhouse/pull/14364)
 - **[node-manager]** added sign check and integrity check to the registry-packages-proxy [#14685](https://github.com/deckhouse/deckhouse/pull/14685)
 - **[openvpn]** The readOnlyRootFilesystem security option is set to true for all containers. [#15346](https://github.com/deckhouse/deckhouse/pull/15346)
    Pods of the openvpn module will be restarted.
 - **[operator-trivy]** Fix CVE's [#15401](https://github.com/deckhouse/deckhouse/pull/15401)
 - **[prometheus]** Add POD_IP var to config [#15527](https://github.com/deckhouse/deckhouse/pull/15527)
 - **[prometheus]** Migrate from old hook logic for enabling prompp [#15308](https://github.com/deckhouse/deckhouse/pull/15308)
 - **[prometheus]** Deprecate the direct Prometheus access [#14812](https://github.com/deckhouse/deckhouse/pull/14812)
    Accessing Prometheus via ingress is now considered deprecated and will not be possible in future releases.
 - **[registry]** Fixed CVE's: CVE-2020-26160, CVE-2020-8911, CVE-2020-8912, CVE-2022-21698, CVE-2022-2582, CVE-2025-22868, CVE-2025-22869, CVE-2025-22870, CVE-2025-22872, CVE-2025-27144 [#15235](https://github.com/deckhouse/deckhouse/pull/15235)

