# Changelog v1.76

## Know before update


 - A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources, replacing `wait` blocks. After upgrading, the OpenTofu plan will include adding the new resources and removing `wait` blocks. Running `converge` is required to apply the changes and is safe: it does not modify existing cloud resources. During migration, readiness checks are skipped for resources older than 5 days. Related warnings may appear and can be safely ignored.
 - Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by `cloud-provider-dvp` and avoid unnecessary or destructive plan changes when data sources depend on readiness checks. Other cloud providers are not affected. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - During migration to the new Go-based implementation of apiserver-proxy, connection flaps to the API server may occur. This change exposes a new hostPort `6480` for health checks and upstreams statistics.
 - Istiod now enforces trust domain validation. Each remote root CA is now scoped to its declared trust domain in the meshConfig caCertificates. Verify that all IstioFederation resources have correct `trustDomain` values matching the remote cluster configuration.
 - ServiceEntry and DestinationRule resources for federated public services will be recreated with new names. This causes a brief traffic interruption for cross-cluster federated service routing during the first reconciliation after the update.
 - Unnecessary or destructive plan updates that could occur when updating labels and annotations via OpenTofu should be prevented now. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.

## Features


 - **[admission-policy-engine]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:admission-policy-engine:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[admission-policy-engine]** Bumped Gatekeeper to 3.22.0 and Ratify to 1.4.0. [#18539](https://github.com/deckhouse/deckhouse/pull/18539)
 - **[candi]** Enabled DRA alpha feature gate `DRAPartitionableDevices`. [#18362](https://github.com/deckhouse/deckhouse/pull/18362)
    Kubelet, api-server, controller-manager and scheduler will be restarted.
 - **[candi]** Added support for `x-kubernetes-sensitive-data` fields in custom resources with RBAC-based filtering and etcd encryption. [#18241](https://github.com/deckhouse/deckhouse/pull/18241)
    Enabling the feature gate `CRDSensitiveData` restarts kube-apiserver.
 - **[candi]** Added multiversion parsing from `oss.yaml` in werf and tests. [#17956](https://github.com/deckhouse/deckhouse/pull/17956)
 - **[cert-manager]** Bumped version to v1.20.0. [#18064](https://github.com/deckhouse/deckhouse/pull/18064)
 - **[cloud-provider-aws]** Moved spot node drain logic from node-termination-handler to Deckhouse. [#18385](https://github.com/deckhouse/deckhouse/pull/18385)
 - **[cloud-provider-aws]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-azure]** Added NVMe disk discovery support for Ubuntu 22.04 Gen2 VMs. [#18839](https://github.com/deckhouse/deckhouse/pull/18839)
 - **[cloud-provider-azure]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-dvp]** Added discovery and propagation of default StorageClass from parent cluster to child clusters in DVP. [#18295](https://github.com/deckhouse/deckhouse/pull/18295)
 - **[cloud-provider-dvp]** Added readiness check resource to prevent lost OpenTofu state if resource is not ready. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources, replacing `wait` blocks. After upgrading, the OpenTofu plan will include adding the new resources and removing `wait` blocks. Running `converge` is required to apply the changes and is safe: it does not modify existing cloud resources. During migration, readiness checks are skipped for resources older than 5 days. Related warnings may appear and can be safely ignored.
 - **[cloud-provider-dvp]** Fail fast on dhctl operations if resources has incorrect status or conditions (like quota exceeded) with some limitations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[cloud-provider-dvp]** Added ServiceWithHealthchecks support to `cloud-provider-dvp`. [#18141](https://github.com/deckhouse/deckhouse/pull/18141)
 - **[cloud-provider-dvp]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-dvp]** Added hybrid cluster support to `cloud-provider-dvp`. [#17861](https://github.com/deckhouse/deckhouse/pull/17861)
 - **[cloud-provider-dynamix]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-gcp]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-gcp]** Added `nestedVirtualization` and `additionalDisks` options to GCPInstanceClass. [#18023](https://github.com/deckhouse/deckhouse/pull/18023)
 - **[cloud-provider-huaweicloud]** Migrated VCD and Huawei Cloud to lib-helm defines; enabled security policy checks for VCD. [#18846](https://github.com/deckhouse/deckhouse/pull/18846)
 - **[cloud-provider-huaweicloud]** Enabled security policy checks and added SecurityPolicyException for Huawei Cloud CSI and CCM. [#18596](https://github.com/deckhouse/deckhouse/pull/18596)
 - **[cloud-provider-huaweicloud]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-huaweicloud]** Migrated CAPI provider to the Cluster API v1beta2 contract. [#17989](https://github.com/deckhouse/deckhouse/pull/17989)
 - **[cloud-provider-openstack]** Added a new optional parameter `csiDriver.fsGroupPolicy`. [#18965](https://github.com/deckhouse/deckhouse/pull/18965)
 - **[cloud-provider-openstack]** Disabled `enable-ingress-hostname` for Kubernetes >=1.32 to use proxy ipMode in LoadBalancer with proxy protocol. [#18524](https://github.com/deckhouse/deckhouse/pull/18524)
 - **[cloud-provider-openstack]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-vcd]** Migrated VCD and Huawei Cloud to lib-helm defines; enabled security policy checks for VCD. [#18846](https://github.com/deckhouse/deckhouse/pull/18846)
 - **[cloud-provider-vcd]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-vsphere]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-yandex]** Switched the default CNI to Cilium with VXLAN networking mode for new clusters to unify the configuration. [#19074](https://github.com/deckhouse/deckhouse/pull/19074)
 - **[cloud-provider-yandex]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-zvirt]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cni-cilium]** Reduced the CPU load in cilium-agent with hubble enabled. [#19669](https://github.com/deckhouse/deckhouse/pull/19669)
 - **[cni-cilium]** Added conntrack import/export HTTP endpoints. [#17429](https://github.com/deckhouse/deckhouse/pull/17429)
 - **[cni-cilium]** Added support for ICMP replies for ExternalIP load balancers. [#17266](https://github.com/deckhouse/deckhouse/pull/17266)
 - **[cni-cilium]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All cilium agents will restart.
 - **[cni-flannel]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All flannel agents will restart.
 - **[cni-simple-bridge]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All simple-bridge agents will restart.
 - **[common]** Add token_namespace and token_name labels to serviceaccount_stale_tokens_total kube-apiserver metric [#19506](https://github.com/deckhouse/deckhouse/pull/19506)
 - **[common]** Added support for using `d8 k` as alias to `kubectl`. [#17033](https://github.com/deckhouse/deckhouse/pull/17033)
 - **[control-plane-manager]** Extend d8:control-plane-manager:admin-kubeconfig-supplement with granular permissions for standard Kubernetes resources not covered by user-authz:cluster-admin. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[control-plane-manager]** Updated RBAC model for admin kubeconfig when `user-authz` is enabled. [#18996](https://github.com/deckhouse/deckhouse/pull/18996)
 - **[control-plane-manager]** Added information to `d8-cluster-kubernetes` about the supported, available, and current `Automatic` versions of Kubernetes. [#18718](https://github.com/deckhouse/deckhouse/pull/18718)
 - **[deckhouse]** Webhook-handler will reload exited shell-operator now. [#19592](https://github.com/deckhouse/deckhouse/pull/19592)
 - **[deckhouse]** Granted RBAC permissions for applications to Deckhouse. [#19385](https://github.com/deckhouse/deckhouse/pull/19385)
 - **[deckhouse]** Added `lastAppliedConfiguration` to Application status. [#19303](https://github.com/deckhouse/deckhouse/pull/19303)
 - **[deckhouse]** Added validation of application settings against the schema from APV. [#19191](https://github.com/deckhouse/deckhouse/pull/19191)
 - **[deckhouse]** Set OpenAPI schemas from release image to APV status. [#19171](https://github.com/deckhouse/deckhouse/pull/19171)
 - **[deckhouse]** Optimized `jq` filter. [#19000](https://github.com/deckhouse/deckhouse/pull/19000)
 - **[deckhouse]** Implemented single-page mode and last cursor for `ListTags`. [#18914](https://github.com/deckhouse/deckhouse/pull/18914)
 - **[deckhouse]** Removed module/application specific fields. [#18885](https://github.com/deckhouse/deckhouse/pull/18885)
 - **[deckhouse]** Changed Deckhouse VPA update mode to `InPlaceOrRecreate`. [#18661](https://github.com/deckhouse/deckhouse/pull/18661)
 - **[deckhouse]** Added hash-checking for webhook-handler rendered files. [#18409](https://github.com/deckhouse/deckhouse/pull/18409)
 - **[deckhouse]** Improved webhook-handler with webhook-operator and CRDs for more complex user flow. [#15160](https://github.com/deckhouse/deckhouse/pull/15160)
 - **[deckhouse-controller]** Added legacy (v1alpha1) PackageRepository module support with observability improvements. [#18522](https://github.com/deckhouse/deckhouse/pull/18522)
 - **[deckhouse-controller]** Prevented enabling multiple CNI modules simultaneously. [#18479](https://github.com/deckhouse/deckhouse/pull/18479)
 - **[deckhouse-controller]** Added credentials to the package repository. [#18264](https://github.com/deckhouse/deckhouse/pull/18264)
 - **[deckhouse-controller]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
 - **[descheduler]** Added `RemovePodsHavingTooManyRestarts` strategy to the `v1alpha2` API for evicting crash-looping pods. [#19122](https://github.com/deckhouse/deckhouse/pull/19122)
    Pods exceeding the configured restart threshold are evicted, freeing node resources and allowing the scheduler to place fresh pods on healthier nodes.
 - **[descheduler]** Added automatic enabling of Kubernetes Metrics API in `descheduler` policy when `metrics.k8s.io` is available in the cluster. [#19064](https://github.com/deckhouse/deckhouse/pull/19064)
    If the cluster serves the `metrics.k8s.io` API (e.g. metrics-server is installed), the `descheduler` policy now includes `metricsProviders` with source KubernetesMetrics, so utilization-related strategies can use Metrics API data. The `descheduler` Pod may restart when this flag or descheduler CR-driven policy changes due to ConfigMap/checksum updates.
 - **[descheduler]** Added configurable descheduling interval presets in ModuleConfig. [#19029](https://github.com/deckhouse/deckhouse/pull/19029)
 - **[descheduler]** Updated descheduler to the 0.35.1 version. [#18781](https://github.com/deckhouse/deckhouse/pull/18781)
 - **[descheduler]** Migrated conversion webhook from bash to ConversionWebhook CR-based mechanism. [#18499](https://github.com/deckhouse/deckhouse/pull/18499)
 - **[descheduler]** Updated `descheduler` to 0.35, with native support for filtering pods by namespace label selector and pod protection based on storage classes. [#18135](https://github.com/deckhouse/deckhouse/pull/18135)
 - **[dhctl]** Added support for standalone binary with on-demand dependency download. [#18482](https://github.com/deckhouse/deckhouse/pull/18482)
 - **[dhctl]** Added support for changing the default OpenTofu backend core and provider log levels with `TF_LOG_CORE` and `TF_LOG_PROVIDER` envs on run dhctl operations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[dhctl]** Added SSH public key validation. [#18119](https://github.com/deckhouse/deckhouse/pull/18119)
 - **[dhctl]** Added ModuleConfig conversions. [#17917](https://github.com/deckhouse/deckhouse/pull/17917)
 - **[docs]** Added an alert for failed documentation renderings. [#18756](https://github.com/deckhouse/deckhouse/pull/18756)
 - **[istio]** Enabled read-only CNI-node root filesystem. [#19334](https://github.com/deckhouse/deckhouse/pull/19334)
 - **[istio]** Added monitoring of the reserved UID `1337` in pods. [#18633](https://github.com/deckhouse/deckhouse/pull/18633)
 - **[istio]** Enabled trust domain validation in Istio control plane for federation and multicluster. [#18502](https://github.com/deckhouse/deckhouse/pull/18502)
    Istiod now enforces trust domain validation. Each remote root CA is now scoped to its declared trust domain in the meshConfig caCertificates. Verify that all IstioFederation resources have correct `trustDomain` values matching the remote cluster configuration.
 - **[istio]** Added validating webhooks for IstioFederation and IstioMulticluster resources. [#18406](https://github.com/deckhouse/deckhouse/pull/18406)
    Creation of IstioFederation is only allowed when Istio federation is enabled in the module configuration. Creation of IstioMulticluster is only allowed when Istio multicluster is enabled in the module configuration.
 - **[istio]** Added support for the `istio.io/rev:default` label. [#18320](https://github.com/deckhouse/deckhouse/pull/18320)
 - **[kube-proxy]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All kube-proxy agents will restart.
 - **[log-shipper]** Added new transformations and parsing features. [#18685](https://github.com/deckhouse/deckhouse/pull/18685)
    log-shipper
 - **[multitenancy-manager]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:multitenancy-manager:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[node-local-dns]** Implemented a new `nodeLocalDns` option to disable IPv6 DNS resolving. [#19282](https://github.com/deckhouse/deckhouse/pull/19282)
 - **[node-manager]** Migrated CAPS controller manager to lib-helm defines. [#18880](https://github.com/deckhouse/deckhouse/pull/18880)
 - **[node-manager]** Added automatic spot-terminated node handling with drain and instance cleanup. [#18385](https://github.com/deckhouse/deckhouse/pull/18385)
 - **[node-manager]** Added gossip-based node failure detection and gRPC API to fencing-agent. [#17771](https://github.com/deckhouse/deckhouse/pull/17771)
    The fencing-agent now uses gossip protocol (`memberlist`) for distributed node health monitoring.
    This prevents incorrect node reboots when control plane is unavailable but worker nodes are healthy.
    A new gRPC API is available via Unix socket at `/tmp/fencing-agent.sock` for querying node membership.
 - **[node-manager]** Replaced NGINX implementation of apiserver-proxy with native Go application. [#17619](https://github.com/deckhouse/deckhouse/pull/17619)
    During migration to the new Go-based implementation of apiserver-proxy, connection flaps to the API server may occur. This change exposes a new hostPort `6480` for health checks and upstreams statistics.
 - **[registry]** Added bootstrap support for `Proxy` registry mode. [#18011](https://github.com/deckhouse/deckhouse/pull/18011)
 - **[terraform-manager]** Suppressed destructive changes when updating labels and annotations for cloud resources via OpenTofu. [#19079](https://github.com/deckhouse/deckhouse/pull/19079)
    Unnecessary or destructive plan updates that could occur when updating labels and annotations via OpenTofu should be prevented now. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - **[terraform-manager]** Skip depends_on meta-argument changes for data sources. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by `cloud-provider-dvp` and avoid unnecessary or destructive plan changes when data sources depend on readiness checks. Other cloud providers are not affected. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - **[user-authn]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authn:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[user-authn]** Updated UI and branding items. [#18834](https://github.com/deckhouse/deckhouse/pull/18834)
 - **[user-authn]** Added self-service password reset via UserOperation. [#18710](https://github.com/deckhouse/deckhouse/pull/18710)
 - **[user-authn]** Added SAML authentication provider support with refresh tokens and Single Logout (SLO). [#18002](https://github.com/deckhouse/deckhouse/pull/18002)
 - **[user-authn]** Added optional Gateway API HTTPRoute/ListenerSet publishing and updated oauth2-proxy auth responses. [#16812](https://github.com/deckhouse/deckhouse/pull/16812)
    All dex-authenticator pods will be restarted
 - **[user-authz]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authz:admin-kubeconfig. [#19420](https://github.com/deckhouse/deckhouse/pull/19420)
 - **[vertical-pod-autoscaler]** Replaced deprecated `--humanize-memory` flag with `--round-memory-bytes=67108864` (64Mi) for human-readable memory recommendations. [#18932](https://github.com/deckhouse/deckhouse/pull/18932)
 - **[vertical-pod-autoscaler]** Updated VPA to 1.6.1, with new `--in-place-skip-disruption-budget` flag, support for skipping min-replica check and `InPlaceOrRecreate` feature now in GA. [#18336](https://github.com/deckhouse/deckhouse/pull/18336)

## Fixes


 - **[candi]** Added deletion of webhook configurations before destroying a Deckhouse deployment. [#19041](https://github.com/deckhouse/deckhouse/pull/19041)
 - **[candi]** Fixed internal node IP discovery for static nodes in DVP clusters. [#18441](https://github.com/deckhouse/deckhouse/pull/18441)
 - **[candi]** Fixed CVEs in `cloud-provider-azure`. [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-aws]** Added a new Bashible step to install `linux-modules-extra` on Ubuntu nodes. [#19415](https://github.com/deckhouse/deckhouse/pull/19415)
 - **[cloud-provider-aws]** Fixed detection of regional limitations versus IAM issues. [#19054](https://github.com/deckhouse/deckhouse/pull/19054)
 - **[cloud-provider-azure]** Fixed CVEs in `cloud-provider-azure`. [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-dvp]** Add skip storage class annotation handling to skip discovery of some storage classes from parent clusters, e.g., local disks. [#19696](https://github.com/deckhouse/deckhouse/pull/19696)
 - **[cloud-provider-dvp]** fix LoadBalancer stuck in pending state — retry on conflict when updating ServiceWithHealthchecks and propagate IP to child cluster service status [#19609](https://github.com/deckhouse/deckhouse/pull/19609)
 - **[cloud-provider-dvp]** Fixed CVEs. [#19362](https://github.com/deckhouse/deckhouse/pull/19362)
 - **[cloud-provider-dvp]** Fixed missing SSH public keys for ephemeral nodes. [#19357](https://github.com/deckhouse/deckhouse/pull/19357)
 - **[cloud-provider-dvp]** Suppressed destructive changes when updating labels and annotations for cloud resources via OpenTofu. [#19079](https://github.com/deckhouse/deckhouse/pull/19079)
 - **[cloud-provider-dvp]** Fixed invalid and unpredictable logic in the DeckhouseMachine controller. [#18715](https://github.com/deckhouse/deckhouse/pull/18715)
 - **[cloud-provider-dvp]** Allowed using `additionalDisks` in master InstanceClasses. [#17352](https://github.com/deckhouse/deckhouse/pull/17352)
 - **[cloud-provider-gcp]** Fixed CVEs in `cloud-provider-gcp`. [#18095](https://github.com/deckhouse/deckhouse/pull/18095)
 - **[cloud-provider-huaweicloud]** Added default values for `elb.class` and `lb-algorithm`, and fixed load balancer creation when `epid` is empty. [#19166](https://github.com/deckhouse/deckhouse/pull/19166)
 - **[cloud-provider-huaweicloud]** Fixed CVEs in `cloud-provider-huaweicloud`. [#18096](https://github.com/deckhouse/deckhouse/pull/18096)
 - **[cloud-provider-openstack]** Fixed CVEs in `cloud-provider-openstack`. [#18099](https://github.com/deckhouse/deckhouse/pull/18099)
 - **[cloud-provider-vcd]** Fixed SecurityPolicyException for VCD components. [#19021](https://github.com/deckhouse/deckhouse/pull/19021)
 - **[cloud-provider-vcd]** Fixed CVEs in `cloud-provider-vcd`. [#18113](https://github.com/deckhouse/deckhouse/pull/18113)
 - **[cloud-provider-vsphere]** normalizes new paths and makes bashible resolve existing paths case-insensitively [#19653](https://github.com/deckhouse/deckhouse/pull/19653)
 - **[cloud-provider-vsphere]** Added filtering discovered zones and datastores by `zones` from provider configurations. [#18378](https://github.com/deckhouse/deckhouse/pull/18378)
 - **[cloud-provider-vsphere]** Enabled the vSphere CSI snapshotter. [#18263](https://github.com/deckhouse/deckhouse/pull/18263)
 - **[cloud-provider-yandex]** Fixed removing public IP addresses from nodes by deleting `externalIPAddresses`. [#18364](https://github.com/deckhouse/deckhouse/pull/18364)
 - **[cloud-provider-zvirt]** Fixed CVEs in `cloud-provider-zvirt`. [#18115](https://github.com/deckhouse/deckhouse/pull/18115)
 - **[cni-cilium]** Fixed constant `invalid sysctl parameter: "net.ipv4.conf..rp_filter"` errors in cilium-agent logs when using Egress Gateway with a Virtual IP. [#18952](https://github.com/deckhouse/deckhouse/pull/18952)
 - **[common]** fix for replace kubectl binary with d8 k alias. [#18514](https://github.com/deckhouse/deckhouse/pull/18514)
 - **[common]** Fixed replacing the `kubectl` binary with the `d8 k` alias. [#18467](https://github.com/deckhouse/deckhouse/pull/18467)
 - **[control-plane-manager]** Skip rebind of ClusterRoleBinding/kubeadm:cluster-admins until the cluster is fully bootstrapped; harden the reconciliation hook. Fixes "cannot change roleRef" on fresh clusters. [#19667](https://github.com/deckhouse/deckhouse/pull/19667)
 - **[control-plane-manager]** Upgraded etcd to 3.6.10. [#19273](https://github.com/deckhouse/deckhouse/pull/19273)
    Etcd will restart.
 - **[control-plane-manager]** Excluded learner etcd members from the kube-apiserver etcd member list. [#19164](https://github.com/deckhouse/deckhouse/pull/19164)
 - **[control-plane-manager]** Fixed incorrect UpdateObserver progress calculation during cluster upgrades. [#19160](https://github.com/deckhouse/deckhouse/pull/19160)
 - **[deckhouse]** Revoke permission to use moduleconfig to user. [#19672](https://github.com/deckhouse/deckhouse/pull/19672)
 - **[deckhouse]** Restore ModuleIsInMaintenanceMode alert by switching to d8_module_config_maintenance sourced from ModuleConfig. [#19352](https://github.com/deckhouse/deckhouse/pull/19352)
 - **[deckhouse]** Fixed module updates skipping patch releases when updating to a new minor version. [#19328](https://github.com/deckhouse/deckhouse/pull/19328)
 - **[deckhouse]** Bumped `shell-operator` to v1.15.3 and webhook-operator dependencies. [#19030](https://github.com/deckhouse/deckhouse/pull/19030)
 - **[deckhouse]** Bumped Hugo and `x/image` to fix CVE-2026-33809, CVE-2026-35166. [#18985](https://github.com/deckhouse/deckhouse/pull/18985)
 - **[deckhouse]** Bumped `nelm` to fix a deadlock. [#18585](https://github.com/deckhouse/deckhouse/pull/18585)
 - **[deckhouse]** Fixed a race condition in ModuleConfig processing during startup. [#18280](https://github.com/deckhouse/deckhouse/pull/18280)
 - **[deckhouse]** Fixed global configuration generation. [#18161](https://github.com/deckhouse/deckhouse/pull/18161)
 - **[deckhouse-controller]** Fixed error logging for MPO validation. [#18698](https://github.com/deckhouse/deckhouse/pull/18698)
 - **[dhctl]** fix SSH preflight check for StaticInstances with password-only auth. [#19560](https://github.com/deckhouse/deckhouse/pull/19560)
 - **[dhctl]** Fix CVEs in `dhctl`. [#19344](https://github.com/deckhouse/deckhouse/pull/19344)
 - **[dhctl]** Fixed `LogInfoLn` behavior for external loggers. [#19234](https://github.com/deckhouse/deckhouse/pull/19234)
 - **[dhctl]** Fixed namespace updates during cluster bootstrap when the namespace already exists. [#19129](https://github.com/deckhouse/deckhouse/pull/19129)
 - **[dhctl]** Switched to using the system certificate pool together with custom registry CAs for registry TLS handling. [#18978](https://github.com/deckhouse/deckhouse/pull/18978)
 - **[dhctl]** Fixed a panic in infrastructure plan processing when the destructive changes report returns an error. [#18908](https://github.com/deckhouse/deckhouse/pull/18908)
 - **[dhctl]** Fixed CVE-2026-33186. [#18805](https://github.com/deckhouse/deckhouse/pull/18805)
 - **[dhctl]** Added a preflight check for validating InstanceClasses against the selected cloud provider. [#18473](https://github.com/deckhouse/deckhouse/pull/18473)
 - **[dhctl]** Excluded `BaseInfraPhase` from the progress phase list for static clusters. [#17856](https://github.com/deckhouse/deckhouse/pull/17856)
 - **[dhctl]** Refactored preflight checks. [#17564](https://github.com/deckhouse/deckhouse/pull/17564)
 - **[docs]** Add info about kernel requirement for containerdv2 migration. [#19505](https://github.com/deckhouse/deckhouse/pull/19505)
 - **[docs]** Updated the `d8 cni-migration` commands in the CNI migration guide to `d8 network cni-migration`. [#18547](https://github.com/deckhouse/deckhouse/pull/18547)
 - **[ingress-nginx]** Nginx is updated up to 1.30.1. [#19846](https://github.com/deckhouse/deckhouse/pull/19846)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Added validating x-forwarded-port and x-forwarded-proto headers when redirecting from www. [#19081](https://github.com/deckhouse/deckhouse/pull/19081)
    All Ingress-NGINX controller pods of 1.12 and 1.14 will be restarted.
 - **[ingress-nginx]** Initial ingress store sync is  fixed. [#19031](https://github.com/deckhouse/deckhouse/pull/19031)
    All Ingress-NGINX controller pods will be restarted.
 - **[istio]** fixed CVEs in module images [#19584](https://github.com/deckhouse/deckhouse/pull/19584)
    module pods will be restarted
 - **[istio]** ingressGateway advertise FQDN does not create a ServiceEntry due to an error [#19528](https://github.com/deckhouse/deckhouse/pull/19528)
 - **[istio]** fixed CVE-2026-39882, CVE-2026-39883 and CVE-2026-35206 [#19085](https://github.com/deckhouse/deckhouse/pull/19085)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-33186 in v1.21.6 images [#18676](https://github.com/deckhouse/deckhouse/pull/18676)
    pods in namespace d8-istio will be restarted
 - **[istio]** fixed CVE-2026-33186 in v1.25.2 images [#18636](https://github.com/deckhouse/deckhouse/pull/18636)
    pods in namespace d8-istio will be restarted
 - **[istio]** Reduce CPU and RAM for regenerate multicluster JWT token and sort ingressGateway [#18554](https://github.com/deckhouse/deckhouse/pull/18554)
 - **[istio]** Deduplicated federation ServiceEntry and DestinationRule resources by hostname across multiple IstioFederation CRs. [#18375](https://github.com/deckhouse/deckhouse/pull/18375)
    ServiceEntry and DestinationRule resources for federated public services will be recreated with new names. This causes a brief traffic interruption for cross-cluster federated service routing during the first reconciliation after the update.
 - **[monitoring-kubernetes]** Resolved port conflict with the runtime-audit-engine module and removed excessive pod privileges [#18868](https://github.com/deckhouse/deckhouse/pull/18868)
 - **[node-local-dns]** Fix name of registry secret in safe-updater deployment [#18673](https://github.com/deckhouse/deckhouse/pull/18673)
 - **[node-manager]** Added cleanup for oversized MCM MachineSet revision history annotation [#19655](https://github.com/deckhouse/deckhouse/pull/19655)
 - **[node-manager]** Improve fencing-agent health monitor logging — warn on fallback feeding, error on watchdog starvation, add diagnostic context to all feeding log messages. [#19400](https://github.com/deckhouse/deckhouse/pull/19400)
    Operators can now detect degraded fencing states (quorum loss, API unreachability) through log levels and diagnostic fields without parsing log messages.
 - **[node-manager]** fix draining hook event generation [#19165](https://github.com/deckhouse/deckhouse/pull/19165)
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
 - **[upmeter]** fix invalid promql expr [#19571](https://github.com/deckhouse/deckhouse/pull/19571)
 - **[upmeter]** fix D8UpmeterProbeGarbagePodsFromDeployments flapping [#19382](https://github.com/deckhouse/deckhouse/pull/19382)
 - **[upmeter]** checks for Observability module in Upmeter + fix Grafana v10 [#18111](https://github.com/deckhouse/deckhouse/pull/18111)
 - **[user-authn]** Add "cache" get parameter to prevent stale caches from breaking login page [#18976](https://github.com/deckhouse/deckhouse/pull/18976)
 - **[user-authn]** Disable implicit flow due to security concerns. [#18288](https://github.com/deckhouse/deckhouse/pull/18288)
 - **[user-authz]** Extend cluster-admin clusterrole  with kubelet-api-admin rights. [#19878](https://github.com/deckhouse/deckhouse/pull/19878)
 - **[user-authz]** Fix multi-tenancy namespace visibility for users without ClusterAuthorizationRules [#18689](https://github.com/deckhouse/deckhouse/pull/18689)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images. [#19778](https://github.com/deckhouse/deckhouse/pull/19778)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Make flag encryption-provider-config-automatic-reload auto enabled when secretEncryptionKey is true [#19287](https://github.com/deckhouse/deckhouse/pull/19287)
    Apiserver will restart if secretEncryptionKey is true
 - **[candi]** Bump patch versions of Kubernetes images, now available 1.33.11, 1.34.7, 1.35.4 [#19271](https://github.com/deckhouse/deckhouse/pull/19271)
    Kubernetes control-plane components will restart, kubelet will restart
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
 - **[deckhouse]** Replace config-values.yaml with settings.yaml. [#19241](https://github.com/deckhouse/deckhouse/pull/19241)
 - **[deckhouse]** Add settings check. [#19116](https://github.com/deckhouse/deckhouse/pull/19116)
 - **[deckhouse]** Enable packages. [#18529](https://github.com/deckhouse/deckhouse/pull/18529)
 - **[deckhouse-controller]** Updated version of shell-operator. [#18648](https://github.com/deckhouse/deckhouse/pull/18648)
 - **[deckhouse-controller]** Convert MUP CRD v1alpha1 not served. [#18222](https://github.com/deckhouse/deckhouse/pull/18222)
 - **[deckhouse-controller]** convert MPO CRD v1alpha1 to not served. [#18010](https://github.com/deckhouse/deckhouse/pull/18010)
 - **[deckhouse-controller]** Converted dashboard module to external source. [#17941](https://github.com/deckhouse/deckhouse/pull/17941)
 - **[dhctl]** add root context propagation, starting from dhctl kingping [#19254](https://github.com/deckhouse/deckhouse/pull/19254)
 - **[docs]** Info about editions for egressgateway has been edited. [#19545](https://github.com/deckhouse/deckhouse/pull/19545)
 - **[ingress-nginx]** The default version of ingress-nginx has been changed to 1.12. [#18612](https://github.com/deckhouse/deckhouse/pull/18612)
    All pods of Ingress-NGINX Controllers using default version  (the controllerVersion is not set) will be restarted and updated from 1.10 to 1.12.
 - **[ingress-nginx]** Missing mount points were added. [#18570](https://github.com/deckhouse/deckhouse/pull/18570)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** An IngressNginxController API migration hook is added. [#18500](https://github.com/deckhouse/deckhouse/pull/18500)
 - **[ingress-nginx]** The werf images are comply with DMT. [#18434](https://github.com/deckhouse/deckhouse/pull/18434)
    All Ingerss-nginx controller pods will be restarted.
 - **[ingress-nginx]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[istio]** changed vex CVE justifications in pilots images [#19572](https://github.com/deckhouse/deckhouse/pull/19572)
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

