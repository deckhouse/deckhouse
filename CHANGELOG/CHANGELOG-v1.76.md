# Changelog v1.76

## [MALFORMED]


 - #17941 unknown section "dashboard"
 - #18678 missing section, missing summary, missing type, unknown section ""
 - #18756 unknown section "docs-builder"
 - #18789 missing section, missing summary, missing type, unknown section ""
 - #18803 unknown section "codeowners"
 - #18977 unknown section "docs-builder"

## Know before update


 - A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources. It replaces the previously used `wait` blocks.
    After updating to this version, the OpenTofu plan will include the creation of `kubernetes_resource_ready_v1` resources and the removal of `wait` blocks. To apply these changes, you must run `converge`.
    The converge operation is safe and does not modify existing cloud resources. In a normal case, the plan should only contain resource creation operations (for example, `Plan: N to add`) and should not include `to change` or `to destroy` actions unless other configuration changes are present.
    During migration, readiness checks are automatically skipped for existing resources older than 5 days. In this case, converge may produce a warning such as the following:
    
    Warning: Resource is too old for checking ready. Skip readiness check.
    
    with module.static-node.kubernetes_resource_ready_v1.vm,
    on ../../../terraform-modules/static-node/main.tf line 138, in resource "kubernetes_resource_ready_v1" "vm":
    138: resource "kubernetes_resource_ready_v1" "vm" {
    
    Resource lifetime is 130h2m20.906973419s. Lifetime for skipping is 120h0m0s.
    
    This warning is expected and does not indicate a problem.
 - Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by the `cloud-provider-dvp`.
    These changes prevent unnecessary or destructive plan updates that could occur when data sources depend on readiness-check resources. The behavior of other cloud providers is not affected.
    If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as dvp, dynamics, zvirt, or yandex), please report them to Deckhouse Technical Support.
 - During migration to new implementation of apiserver-proxy, it's possible to flapping connections for apiserver usage.
    Added new exposed hostPort: 6480 for healthchecks and upstreams statistics

## Features


 - **[admission-policy-engine]** Bump gatekeeper up to 3.22.0, ratify up to 1.4.0 [#18539](https://github.com/deckhouse/deckhouse/pull/18539)
 - **[candi]** Enable DRA alpha feature gate DRAPartitionableDevices [#18362](https://github.com/deckhouse/deckhouse/pull/18362)
    Kubelet, api-server, controller-manager and scheduler will be restarted.
 - **[candi]** Add mutliversion parsing from oss.yaml file in werf, add tests [#17956](https://github.com/deckhouse/deckhouse/pull/17956)
 - **[cert-manager]** Bump version up to v1.20.0 [#18064](https://github.com/deckhouse/deckhouse/pull/18064)
 - **[cloud-provider-aws]** redesigned spot node drain strategy - moved drain logic from node-termination-handler to Deckhouse [#18385](https://github.com/deckhouse/deckhouse/pull/18385)
 - **[cloud-provider-aws]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-azure]** add NVMe disk discovery support for Ubuntu 22.04 Gen2 VMs [#18839](https://github.com/deckhouse/deckhouse/pull/18839)
 - **[cloud-provider-azure]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-dvp]** discover and propagate default StorageClass from parent DVP cluster to child clusters [#18295](https://github.com/deckhouse/deckhouse/pull/18295)
 - **[cloud-provider-dvp]** Add readiness check resource to prevent lost OpenTofu state if resource is not ready. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources. It replaces the previously used `wait` blocks.
    After updating to this version, the OpenTofu plan will include the creation of `kubernetes_resource_ready_v1` resources and the removal of `wait` blocks. To apply these changes, you must run `converge`.
    The converge operation is safe and does not modify existing cloud resources. In a normal case, the plan should only contain resource creation operations (for example, `Plan: N to add`) and should not include `to change` or `to destroy` actions unless other configuration changes are present.
    During migration, readiness checks are automatically skipped for existing resources older than 5 days. In this case, converge may produce a warning such as the following:
    
    Warning: Resource is too old for checking ready. Skip readiness check.
    
    with module.static-node.kubernetes_resource_ready_v1.vm,
    on ../../../terraform-modules/static-node/main.tf line 138, in resource "kubernetes_resource_ready_v1" "vm":
    138: resource "kubernetes_resource_ready_v1" "vm" {
    
    Resource lifetime is 130h2m20.906973419s. Lifetime for skipping is 120h0m0s.
    
    This warning is expected and does not indicate a problem.
 - **[cloud-provider-dvp]** Fail fast on dhctl operations if resources has incorrect status or conditions (like quota exceeded) with some limitations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[cloud-provider-dvp]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-dvp]** add hybrid cluster support to the DVP cloud provider module. [#17861](https://github.com/deckhouse/deckhouse/pull/17861)
 - **[cloud-provider-dynamix]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-gcp]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-gcp]** add nestedVirtualization and additionalDisks options to GCPInstanceClass [#18023](https://github.com/deckhouse/deckhouse/pull/18023)
 - **[cloud-provider-huaweicloud]** Migrate VCD and HuaweiCloud to lib-helm defines; Enable security policy check for VCD [#18846](https://github.com/deckhouse/deckhouse/pull/18846)
 - **[cloud-provider-huaweicloud]** Enable security policy check and add SecurityPolicyException for HuaweiCloud CSI and CCM [#18596](https://github.com/deckhouse/deckhouse/pull/18596)
 - **[cloud-provider-huaweicloud]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-huaweicloud]** migrate CAPI provider to cluster-api v1beta2 contract [#17989](https://github.com/deckhouse/deckhouse/pull/17989)
 - **[cloud-provider-openstack]** Disable enable-ingress-hostname for Kubernetes >=1.32 to use proxy ipMode in LoadBalancer with proxy protocol. [#18524](https://github.com/deckhouse/deckhouse/pull/18524)
 - **[cloud-provider-openstack]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-vcd]** Migrate VCD and HuaweiCloud to lib-helm defines; Enable security policy check for VCD [#18846](https://github.com/deckhouse/deckhouse/pull/18846)
 - **[cloud-provider-vcd]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-vsphere]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-yandex]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-zvirt]** added secrets with node-manager dependencies [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cni-cilium]** new import/export conntrack http endpoints [#17429](https://github.com/deckhouse/deckhouse/pull/17429)
 - **[cni-cilium]** Added support for reply on ICMP requests for loadbalancer ExternalIPs [#17266](https://github.com/deckhouse/deckhouse/pull/17266)
 - **[cni-cilium]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All cilium agents will restart.
 - **[cni-flannel]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All flannel agents will restart.
 - **[cni-simple-bridge]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All simple-bridge agents will restart.
 - **[common]** use alias kubectl to d8 k. [#17033](https://github.com/deckhouse/deckhouse/pull/17033)
 - **[control-plane-manager]** Add information to `d8-cluster-kubernetes` about supported versions, available versions, and the current "automatic" version. [#18718](https://github.com/deckhouse/deckhouse/pull/18718)
 - **[deckhouse]** Optimize jq filter. [#19000](https://github.com/deckhouse/deckhouse/pull/19000)
 - **[deckhouse]** Implement single-page mode and last cursor for ListTags. [#18914](https://github.com/deckhouse/deckhouse/pull/18914)
 - **[deckhouse]** Remove module/application specific fields. [#18885](https://github.com/deckhouse/deckhouse/pull/18885)
 - **[deckhouse]** Changed deckhouse VPA update mode for requests to InPlaceOrRecreate [#18661](https://github.com/deckhouse/deckhouse/pull/18661)
 - **[deckhouse]** Added hash-checking for webhook-handler rendered files. [#18409](https://github.com/deckhouse/deckhouse/pull/18409)
 - **[deckhouse]** webhook-handler improved with webhook-operator and CRD's for more complex user flow [#15160](https://github.com/deckhouse/deckhouse/pull/15160)
 - **[deckhouse-controller]** Add legacy module (v1alpha1) support for Package Repository with MPV enrichment and foundVersions observability [#18522](https://github.com/deckhouse/deckhouse/pull/18522)
 - **[deckhouse-controller]** Prevented enabling multiple CNI modules simultaneously. [#18479](https://github.com/deckhouse/deckhouse/pull/18479)
 - **[deckhouse-controller]** add credentials to package repository [#18264](https://github.com/deckhouse/deckhouse/pull/18264)
 - **[deckhouse-controller]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
 - **[descheduler]** Migrate conversion webhook from bash to ConversionWebhook CR-based mechanism [#18499](https://github.com/deckhouse/deckhouse/pull/18499)
 - **[descheduler]** Updated descheduler to the 0.35 version.
    Descheduler now supports filtering pods by namespace label selector natively.
    Descheduler now protects pods based on storage classes. [#18135](https://github.com/deckhouse/deckhouse/pull/18135)
 - **[dhctl]** Add an ability to run dhctl w/o dependencies, as single binary file, and download necessary dependencies from registry on the flight. [#18482](https://github.com/deckhouse/deckhouse/pull/18482)
 - **[dhctl]** Add ability for change default opentofu backend core and provider log levels with TF_LOG_CORE and TF_LOG_PROVIDER envs on run dhctl operations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[dhctl]** Add SSH public keys validation to dhctl. [#18119](https://github.com/deckhouse/deckhouse/pull/18119)
 - **[dhctl]** Add conversions for ModuleConfigs to dhctl. [#17917](https://github.com/deckhouse/deckhouse/pull/17917)
 - **[istio]** Monitoring reserved UID 1337 in pods [#18633](https://github.com/deckhouse/deckhouse/pull/18633)
 - **[istio]** Add validating webhooks for IstioFederation and IstioMulticluster resources. [#18406](https://github.com/deckhouse/deckhouse/pull/18406)
    Creation of IstioFederation is only allowed when Istio federation is enabled in the module configuration. Creation of IstioMulticluster is only allowed when Istio multicluster is enabled in the module configuration.
 - **[istio]** Add support of label istio.io/rev:default [#18320](https://github.com/deckhouse/deckhouse/pull/18320)
 - **[kube-proxy]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All kube-proxy agents will restart.
 - **[node-manager]** Migrate CAPS controller manager to lib-helm defines [#18880](https://github.com/deckhouse/deckhouse/pull/18880)
 - **[node-manager]** added automatic spot-terminated node handling with drain and Instance cleanup [#18385](https://github.com/deckhouse/deckhouse/pull/18385)
 - **[node-manager]** Added gossip-based node failure detection and gRPC API to fencing-agent. [#17771](https://github.com/deckhouse/deckhouse/pull/17771)
    The fencing-agent now uses gossip protocol (memberlist) for distributed node health monitoring.
    This prevents incorrect node reboots when control plane is unavailable but worker nodes are healthy.
    A new gRPC API is available via Unix socket at /tmp/fencing-agent.sock for querying node membership.
 - **[node-manager]** replace nginx implementation of apiserver-proxy to native go application with discovery [#17619](https://github.com/deckhouse/deckhouse/pull/17619)
    During migration to new implementation of apiserver-proxy, it's possible to flapping connections for apiserver usage.
    Added new exposed hostPort: 6480 for healthchecks and upstreams statistics
 - **[registry]** Added bootstrap support with `Proxy` registry mode [#18011](https://github.com/deckhouse/deckhouse/pull/18011)
 - **[terraform-manager]** Skip depends_on meta-argument changes for data sources. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by the `cloud-provider-dvp`.
    These changes prevent unnecessary or destructive plan updates that could occur when data sources depend on readiness-check resources. The behavior of other cloud providers is not affected.
    If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as dvp, dynamics, zvirt, or yandex), please report them to Deckhouse Technical Support.
 - **[user-authn]** Update UI and branding items [#18834](https://github.com/deckhouse/deckhouse/pull/18834)
 - **[user-authn]** Add SAML authentication provider support with refresh tokens and Single Logout (SLO) [#18002](https://github.com/deckhouse/deckhouse/pull/18002)
 - **[user-authn]** Added optional Gateway API HTTPRoute/ListenerSet publishing and updated oauth2-proxy auth responses. [#16812](https://github.com/deckhouse/deckhouse/pull/16812)
    All dex-authenticator pods will be restarted
 - **[vertical-pod-autoscaler]** Replace deprecated --humanize-memory flag with --round-memory-bytes=67108864 (64Mi) for human-readable memory recommendations [#18932](https://github.com/deckhouse/deckhouse/pull/18932)
 - **[vertical-pod-autoscaler]** Update vpa to the 1.6.1 version. 
     Add --in-place-skip-disruption-budget flag 
     Add skip min-replica check 
     InPlaceOrRecreate feature now to GA [#18336](https://github.com/deckhouse/deckhouse/pull/18336)

## Fixes


 - **[candi]** Fix internal node IP discovery for static nodes in DVP clusters [#18441](https://github.com/deckhouse/deckhouse/pull/18441)
 - **[candi]** fix CVE in cloud-provider-azure [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-azure]** fix CVE in cloud-provider-azure [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-dvp]** fix invalid and unpredictable logic in DeckhouseMachine controller [#18715](https://github.com/deckhouse/deckhouse/pull/18715)
 - **[cloud-provider-dvp]** allow user to use additionalDisks in master InstanceClass [#17352](https://github.com/deckhouse/deckhouse/pull/17352)
 - **[cloud-provider-gcp]** fix CVEs in cloud-provider-gcp [#18095](https://github.com/deckhouse/deckhouse/pull/18095)
 - **[cloud-provider-huaweicloud]** fix CVEs in cloud-provider-huaweicloud [#18096](https://github.com/deckhouse/deckhouse/pull/18096)
 - **[cloud-provider-openstack]** fix CVE in cloud-provider-openstack module [#18099](https://github.com/deckhouse/deckhouse/pull/18099)
 - **[cloud-provider-vcd]** fix CVE in cloud-provider-vcd module [#18113](https://github.com/deckhouse/deckhouse/pull/18113)
 - **[cloud-provider-vsphere]** Filter discovered zones and datastores by zones from provider configurations. [#18378](https://github.com/deckhouse/deckhouse/pull/18378)
 - **[cloud-provider-vsphere]** Enable vSphere CSI snapshotter [#18263](https://github.com/deckhouse/deckhouse/pull/18263)
 - **[cloud-provider-zvirt]** fix CVE in cloud-provider-zvirt module [#18115](https://github.com/deckhouse/deckhouse/pull/18115)
 - **[cni-cilium]** Fixed constant `invalid sysctl parameter: "net.ipv4.conf..rp_filter"` errors in cilium-agent logs when using Egress Gateway with a Virtual IP. [#18952](https://github.com/deckhouse/deckhouse/pull/18952)
 - **[common]** fix for replace kubectl binary with d8 k alias. [#18467](https://github.com/deckhouse/deckhouse/pull/18467)
 - **[deckhouse]** Bump hugo and x/image to fix CVE-2026-33809, CVE-2026-35166. [#18985](https://github.com/deckhouse/deckhouse/pull/18985)
 - **[deckhouse]** Bump nelm version with deadlock fix. [#18585](https://github.com/deckhouse/deckhouse/pull/18585)
 - **[deckhouse]** Fix race in ModuleConfig processing at the start. [#18280](https://github.com/deckhouse/deckhouse/pull/18280)
 - **[deckhouse-controller]** Fix error logging for MPO validation. [#18698](https://github.com/deckhouse/deckhouse/pull/18698)
 - **[deckhouse-controller]** Fix problem when creating the config for the global. [#18161](https://github.com/deckhouse/deckhouse/pull/18161)
 - **[dhctl]** Add a preflight check that validates `InstanceClass` resources against the selected cloud provider [#18473](https://github.com/deckhouse/deckhouse/pull/18473)
 - **[dhctl]** BaseInfraPhase is excluded from the progress phase list for static clusters [#17856](https://github.com/deckhouse/deckhouse/pull/17856)
 - **[dhctl]** preflight refactor [#17564](https://github.com/deckhouse/deckhouse/pull/17564)
 - **[docs]** Updated the `d8 cni-migration` commands in the CNI migration guide to `d8 network cni-migration`. [#18547](https://github.com/deckhouse/deckhouse/pull/18547)
 - **[ingress-nginx]** Initial ingress store sync is  fixed. [#19031](https://github.com/deckhouse/deckhouse/pull/19031)
    All Ingress-NGINX controller pods will be restarted.
 - **[istio]** fixed CVE-2026-33186 in v1.21.6 images [#18676](https://github.com/deckhouse/deckhouse/pull/18676)
    pods in namespace d8-istio will be restarted
 - **[istio]** fixed CVE-2026-33186 in v1.25.2 images [#18636](https://github.com/deckhouse/deckhouse/pull/18636)
    pods in namespace d8-istio will be restarted
 - **[istio]** Reduce CPU and RAM for regenerate multicluster JWT token and sort ingressGateway [#18554](https://github.com/deckhouse/deckhouse/pull/18554)
 - **[node-local-dns]** Fix name of registry secret in safe-updater deployment [#18673](https://github.com/deckhouse/deckhouse/pull/18673)
 - **[node-manager]** fix Cluster Autoscaler RBAC for CAPI providers, add missing machinedeployments/scale to write rule and patch verb to ClusterRole. [#18818](https://github.com/deckhouse/deckhouse/pull/18818)
 - **[node-manager]** mitigate CVE-2026-33186 [#18649](https://github.com/deckhouse/deckhouse/pull/18649)
 - **[node-manager]** caps fix inconsistent pending staticinstance [#18379](https://github.com/deckhouse/deckhouse/pull/18379)
 - **[node-manager]** Fencing controller no longer deletes Node objects for Notify-mode and Static/CloudStatic nodes. [#18218](https://github.com/deckhouse/deckhouse/pull/18218)
 - **[node-manager]** fix go lint errors in node-controller [#18187](https://github.com/deckhouse/deckhouse/pull/18187)
 - **[node-manager]** Fix cluster-autoscaler deadlock when machine creation fails with a non-ResourceExhausted error, preventing scale-up to alternative node groups. [#18154](https://github.com/deckhouse/deckhouse/pull/18154)
 - **[node-manager]** Fix capacity parsing logic for DVPInstanceClass and add test case for DVPSpecWorker [#17935](https://github.com/deckhouse/deckhouse/pull/17935)
    Capacity values (CPU/memory) for DVPInstanceClass are now correctly extracted according to spec shape. Nested `virtualMachine` fields are used and memory quantities like `Gi` are properly parsed.
 - **[prometheus]** Fix externalLabels handling in conjunction with the PrometheusRemoteWrites [#18608](https://github.com/deckhouse/deckhouse/pull/18608)
 - **[registry]** Updated auth image Go dependencies to fix Go CVEs. [#18234](https://github.com/deckhouse/deckhouse/pull/18234)
    Registry pods will be restarted.
 - **[registrypackages]** Replace symlinks with actual files in kubernetes artifacts for werf 2.57.1 compatibility [#18662](https://github.com/deckhouse/deckhouse/pull/18662)
 - **[user-authn]** Disable implicit flow due to security concerns. [#18288](https://github.com/deckhouse/deckhouse/pull/18288)
 - **[user-authz]** Fix multi-tenancy namespace visibility for users without ClusterAuthorizationRules [#18689](https://github.com/deckhouse/deckhouse/pull/18689)

## Chore


 - **[candi]** Change the way to determinate registry packages proxy addresses during node bootstrap. [#17977](https://github.com/deckhouse/deckhouse/pull/17977)
 - **[candi]** add container-selinux package for selinux policies on rhel based distributions. [#17714](https://github.com/deckhouse/deckhouse/pull/17714)
 - **[cilium-hubble]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[cloud-provider-aws]** module directory localization [#17740](https://github.com/deckhouse/deckhouse/pull/17740)
 - **[cloud-provider-azure]** module direcotry localization [#17749](https://github.com/deckhouse/deckhouse/pull/17749)
 - **[cloud-provider-dvp]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[cloud-provider-dvp]** migrate cloud dvp to beta2 capi [#16844](https://github.com/deckhouse/deckhouse/pull/16844)
 - **[cloud-provider-dvp]** add ownerReferences to VM-related objects (managed by Terraform) [#16777](https://github.com/deckhouse/deckhouse/pull/16777)
 - **[cloud-provider-dynamix]** module directory localization [#17715](https://github.com/deckhouse/deckhouse/pull/17715)
 - **[cloud-provider-gcp]** module directory localization [#17747](https://github.com/deckhouse/deckhouse/pull/17747)
 - **[cloud-provider-huaweicloud]** module directory localization [#17716](https://github.com/deckhouse/deckhouse/pull/17716)
 - **[cloud-provider-openstack]** module directory localization [#17710](https://github.com/deckhouse/deckhouse/pull/17710)
 - **[cloud-provider-vcd]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[cloud-provider-vcd]** module directory localization [#17707](https://github.com/deckhouse/deckhouse/pull/17707)
 - **[cloud-provider-vsphere]** module directory localization [#17718](https://github.com/deckhouse/deckhouse/pull/17718)
 - **[cloud-provider-yandex]** module directory localization [#17743](https://github.com/deckhouse/deckhouse/pull/17743)
 - **[cloud-provider-zvirt]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[cloud-provider-zvirt]** use v1beta2 CAPI contract [#17991](https://github.com/deckhouse/deckhouse/pull/17991)
 - **[cloud-provider-zvirt]** module directory localization [#17717](https://github.com/deckhouse/deckhouse/pull/17717)
 - **[cni-cilium]** metadata.labels render changed to use "helm_lib_module_labels" [#18366](https://github.com/deckhouse/deckhouse/pull/18366)
 - **[cni-cilium]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[cni-cilium]** RBAC has been added for NetworkPolicies and EgressGateways. Now, you need to have the necessary permissions to use them. [#18022](https://github.com/deckhouse/deckhouse/pull/18022)
 - **[cni-flannel]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[deckhouse]** Enable packages. [#18529](https://github.com/deckhouse/deckhouse/pull/18529)
 - **[deckhouse-controller]** Updated version of shell-operator. [#18648](https://github.com/deckhouse/deckhouse/pull/18648)
 - **[deckhouse-controller]** Convert MUP CRD v1alpha1 not served. [#18222](https://github.com/deckhouse/deckhouse/pull/18222)
 - **[deckhouse-controller]** convert MPO CRD v1alpha1 to not served. [#18010](https://github.com/deckhouse/deckhouse/pull/18010)
 - **[ingress-nginx]** The default version of ingress-nginx has been changed to 1.12. [#18612](https://github.com/deckhouse/deckhouse/pull/18612)
    All pods of Ingress-NGINX Controllers using default version  (the controllerVersion is not set) will be restarted and updated from 1.10 to 1.12.
 - **[ingress-nginx]** Missing mount points were added. [#18570](https://github.com/deckhouse/deckhouse/pull/18570)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** An IngressNginxController API migration hook is added. [#18500](https://github.com/deckhouse/deckhouse/pull/18500)
 - **[ingress-nginx]** The werf images are comply with DMT. [#18434](https://github.com/deckhouse/deckhouse/pull/18434)
    All Ingerss-nginx controller pods will be restarted.
 - **[ingress-nginx]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[istio]** changed group names in prometheus-rules of controlplane alerts [#18910](https://github.com/deckhouse/deckhouse/pull/18910)
 - **[istio]** fixed discovery application namespaces test [#18847](https://github.com/deckhouse/deckhouse/pull/18847)
 - **[istio]** Correction of the Istio Federatio documentation on single and multi network [#18507](https://github.com/deckhouse/deckhouse/pull/18507)
 - **[istio]** Added kubernetes v1.31-1.35 in docs supported versions. [#18447](https://github.com/deckhouse/deckhouse/pull/18447)
 - **[istio]** Git clone for images common-v1x21x6, common-v1x25x2, operator-v1x25x2 and proxyv2-v1x21x6 moved to git section of werf.inc.yaml [#18293](https://github.com/deckhouse/deckhouse/pull/18293)
 - **[istio]** replaced proxyv2-v1x25x2 and ztunnel-v1x25x2 images with distroless [#18210](https://github.com/deckhouse/deckhouse/pull/18210)
 - **[istio]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[keepalived]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[kube-dns]** disabled DMT-lint for ommited oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[kube-proxy]** fixed tests of discover_api_endpoints.go [#18270](https://github.com/deckhouse/deckhouse/pull/18270)
 - **[metallb]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[monitoring-kubernetes]** Add OOM kills exporter [#16662](https://github.com/deckhouse/deckhouse/pull/16662)
 - **[multitenancy-manager]** Bumped dependencies to fix CVE's [#18829](https://github.com/deckhouse/deckhouse/pull/18829)
 - **[network-policy-engine]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[node-local-dns]** Improved D8NodeLocalDNSKubeforwardRequestLatencyP95High alert description. [#18317](https://github.com/deckhouse/deckhouse/pull/18317)
 - **[node-manager]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[node-manager]** update cluster-api version in caps to v1.11.5 [#17936](https://github.com/deckhouse/deckhouse/pull/17936)
 - **[openvpn]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[registry]** Update dependencies to fix CVEs [#18600](https://github.com/deckhouse/deckhouse/pull/18600)
 - **[upmeter]** fix go lint warning [#17909](https://github.com/deckhouse/deckhouse/pull/17909)
    upmeter

