# Changelog v1.77

## [MALFORMED]


 - #19799 unknown section "codeowners"

## Features


 - **[admission-policy-engine]** Global refactor of constraints and tests, add support for container-level SecurityPolicyException [#18668](https://github.com/deckhouse/deckhouse/pull/18668)
 - **[candi]** migrate to unified builder with package manager and Go toolchain [#19882](https://github.com/deckhouse/deckhouse/pull/19882)
 - **[candi]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cilium-hubble]** Added authorization to UI [#18895](https://github.com/deckhouse/deckhouse/pull/18895)
 - **[cloud-provider-aws]** migrate to unified builder with package manager and Go toolchain [#19616](https://github.com/deckhouse/deckhouse/pull/19616)
 - **[cloud-provider-aws]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-azure]** migrate to shared lib-helm templates for CCM and CDD deployments [#19264](https://github.com/deckhouse/deckhouse/pull/19264)
 - **[cloud-provider-azure]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-dvp]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-dvp]** enable security policy check and add SecurityPolicyException for DVP CSI and CCM [#18873](https://github.com/deckhouse/deckhouse/pull/18873)
 - **[cloud-provider-dynamix]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-dynamix]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-gcp]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-gcp]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-huaweicloud]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-openstack]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-openstack]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vcd]** Migrate Dynamix, GCP and OpenStack templates to define from helm-lib [#19267](https://github.com/deckhouse/deckhouse/pull/19267)
 - **[cloud-provider-vcd]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vsphere]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-vsphere]** enable security policy check and add SecurityPolicyException rendering for vSphere CCM and CSI components [#18905](https://github.com/deckhouse/deckhouse/pull/18905)
 - **[cloud-provider-yandex]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-yandex]** enable security policy check and add SecurityPolicyException rendering for Yandex CCM and CSI components [#18899](https://github.com/deckhouse/deckhouse/pull/18899)
 - **[cloud-provider-zvirt]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[cloud-provider-zvirt]** enable security policy check and add SecurityPolicyException rendering for zVirt CCM and CSI components [#18902](https://github.com/deckhouse/deckhouse/pull/18902)
 - **[cni-cilium]** Added RBACv1 roles (User and ClusterEditor) for HubbleMonitoringConfig resource. [#19629](https://github.com/deckhouse/deckhouse/pull/19629)
 - **[common]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[control-plane-manager]** Publish API ingress settings are moved to control-plane-manager module, ingress resource is moved to kube-system namespace. [#18628](https://github.com/deckhouse/deckhouse/pull/18628)
 - **[deckhouse]** Add package health monitor. [#19711](https://github.com/deckhouse/deckhouse/pull/19711)
 - **[deckhouse]** Check registry pagination to enable incremental scan. [#18898](https://github.com/deckhouse/deckhouse/pull/18898)
 - **[dhctl]** Add global flag dhctl, improve interactive logging. [#19879](https://github.com/deckhouse/deckhouse/pull/19879)
 - **[dhctl]** add basic OpenTelemetry support to dhctl [#19738](https://github.com/deckhouse/deckhouse/pull/19738)
 - **[dhctl]** Logging refactoring for dhctl. [#19422](https://github.com/deckhouse/deckhouse/pull/19422)
 - **[docs]** Add new security events documentation page [#19706](https://github.com/deckhouse/deckhouse/pull/19706)
 - **[docs]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[ingress-nginx]** Isolated configuration validation was implemented. [#18145](https://github.com/deckhouse/deckhouse/pull/18145)
    All Ingress-NGINX controller pods will be restarted.
 - **[istio]** Change API-proxy VPA update mode to fix an interruption in work [#19602](https://github.com/deckhouse/deckhouse/pull/19602)
 - **[istio]** Add explicit configuration option `ambient.enabled` to manage Istio ambient mesh. [#19318](https://github.com/deckhouse/deckhouse/pull/19318)
 - **[istio]** Changed preferred istio default subdomain for metadata-exporter [#19299](https://github.com/deckhouse/deckhouse/pull/19299)
    matadata-exporter pod will be restarted
 - **[istio]** Implement graceful metadata secret renewal for multiclusters. [#19278](https://github.com/deckhouse/deckhouse/pull/19278)
 - **[metallb]** Added RBACv1 roles (User and ClusterAdmin) for MetalLoadBalancerClass resource. [#19627](https://github.com/deckhouse/deckhouse/pull/19627)
 - **[node-manager]** Added Instance API v1alpha2 with unified machine and bashible status model [#18795](https://github.com/deckhouse/deckhouse/pull/18795)
 - **[registrypackages]** Add patches for containerd 2.2.3 with integrity logic [#19076](https://github.com/deckhouse/deckhouse/pull/19076)
 - **[registrypackages]** Add oss.yaml files for cloud provider modules [#18989](https://github.com/deckhouse/deckhouse/pull/18989)
 - **[service-with-healthchecks]** Added RBACv1 roles (User and Editor) for ServiceWithHealthchecks resource. [#19625](https://github.com/deckhouse/deckhouse/pull/19625)
 - **[user-authn]** Brute-force protection for Dex — per-IP rate limit on password endpoints and account lockout for LDAP/Crowd connectors. [#19542](https://github.com/deckhouse/deckhouse/pull/19542)

## Fixes


 - **[admission-policy-engine]** Changed default PSS policy to Baseline for unrecognized deckhouse versions [#19663](https://github.com/deckhouse/deckhouse/pull/19663)
 - **[admission-policy-engine]** gatekeeper pods now tolerate csi-not-bootstrapped taint to prevent webhook deadlock during worker node replacement [#19383](https://github.com/deckhouse/deckhouse/pull/19383)
 - **[candi]** retry kube API errors in rpp-get during registry packages discovery [#19673](https://github.com/deckhouse/deckhouse/pull/19673)
 - **[candi]** remove Python requirement from bashible bootstrap and switch Registry Packages Proxy package installation to static binaries. [#18626](https://github.com/deckhouse/deckhouse/pull/18626)
 - **[cloud-provider-aws]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-azure]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-dvp]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-dvp]** fix LoadBalancer stuck in pending state — retry on conflict when updating ServiceWithHealthchecks and propagate IP to child cluster service status [#19590](https://github.com/deckhouse/deckhouse/pull/19590)
 - **[cloud-provider-dynamix]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-gcp]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-huaweicloud]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-openstack]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-vcd]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-vcd]** Fix SecurityPolicyException in CAPCD [#19539](https://github.com/deckhouse/deckhouse/pull/19539)
 - **[cloud-provider-vsphere]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-yandex]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cloud-provider-zvirt]** Bump helm_lib version with liveness probe parameters for CSI controller [#19694](https://github.com/deckhouse/deckhouse/pull/19694)
 - **[cni-cilium]** Fixed infinite reconciliation of EgressGateway objects and improved status reporting. [#19219](https://github.com/deckhouse/deckhouse/pull/19219)
 - **[common]** fix StaticInstance preflight check for fails when SSHCredentials has no private key. [#19527](https://github.com/deckhouse/deckhouse/pull/19527)
 - **[common]** Normalize kernel version from uname to semver [#19329](https://github.com/deckhouse/deckhouse/pull/19329)
    Nodes with Debian 13 kernels (e.g. 6.12.74+deb13+1-cloud-amd64) previously failed the kernel version check and could not join the cluster.
 - **[dhctl]** Fix panic in in-cluster converge-migration run of dhctl. [#19823](https://github.com/deckhouse/deckhouse/pull/19823)
 - **[dhctl]** Fix static preflights for dhctl run outside an install container. [#19809](https://github.com/deckhouse/deckhouse/pull/19809)
 - **[dhctl]** Fix panic in dhctl converge command [#19753](https://github.com/deckhouse/deckhouse/pull/19753)
 - **[dhctl]** fix panic dereference in dhctl destroy command. [#19716](https://github.com/deckhouse/deckhouse/pull/19716)
 - **[dhctl]** Replace app package references with options package in multiple files [#19702](https://github.com/deckhouse/deckhouse/pull/19702)
 - **[dhctl]** add NodeReady wait to dhctl converge and improve etcd check output [#18991](https://github.com/deckhouse/deckhouse/pull/18991)
 - **[istio]** fixed discovery_operator_versions_to_install.go hook to migrate from 1.21 to 1.25 [#19434](https://github.com/deckhouse/deckhouse/pull/19434)
 - **[istio]** ingressGateway advertise FQDN does not create a ServiceEntry due to an error [#19395](https://github.com/deckhouse/deckhouse/pull/19395)
 - **[node-manager]** add rbac policies for persistantvolumes to manage from capi-controller-manager. [#19291](https://github.com/deckhouse/deckhouse/pull/19291)
 - **[node-manager]** fix to caps for use staticmachine creationtimestamp [#18821](https://github.com/deckhouse/deckhouse/pull/18821)
 - **[upmeter]** fix nil pointer panic on shutdown when server initialization is incomplete [#19554](https://github.com/deckhouse/deckhouse/pull/19554)

## Chore


 - **[candi]** migrate bashible bootstrap from kubectl to d8-curl for kubernetes api calls [#19023](https://github.com/deckhouse/deckhouse/pull/19023)
 - **[cloud-provider-vsphere]** Add module hook and template rendering tests for hybrid cluster with vSphere cloud provider [#19209](https://github.com/deckhouse/deckhouse/pull/19209)
 - **[control-plane-manager]** Move Update Observer into Control Plane Manager [#19394](https://github.com/deckhouse/deckhouse/pull/19394)
 - **[deckhouse]** Decouple from dhctl/cmd/commands. [#19624](https://github.com/deckhouse/deckhouse/pull/19624)
 - **[deckhouse]** Finish failed operations. [#19236](https://github.com/deckhouse/deckhouse/pull/19236)
 - **[ingress-nginx]** add controller 1.15 and remove controller 1.10 and its `nginxProfilingEnabled` flag. [#19509](https://github.com/deckhouse/deckhouse/pull/19509)
    users must switch to controller version 1.12 or newer.
 - **[ingress-nginx]** IngressNginxController API is updated to support improved resource management. [#18461](https://github.com/deckhouse/deckhouse/pull/18461)
 - **[istio]** Added HTTPRoute for multiclusters. [#19603](https://github.com/deckhouse/deckhouse/pull/19603)
    low
 - **[istio]** Added ClusterRoles (v1) over module CustomResources [#19587](https://github.com/deckhouse/deckhouse/pull/19587)
 - **[log-shipper]** Update RUSTFLAGS to improve security [#18642](https://github.com/deckhouse/deckhouse/pull/18642)
 - **[node-manager]** migrate node/nodegroup reconciliation hooks to node-controller. [#18481](https://github.com/deckhouse/deckhouse/pull/18481)
 - **[registry]** Codeowners change [#19410](https://github.com/deckhouse/deckhouse/pull/19410)

