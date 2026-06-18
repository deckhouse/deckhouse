# Changelog v1.76

## Know before update


 - A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources, replacing `wait` blocks. After upgrading, the OpenTofu plan will include adding the new resources and removing `wait` blocks. Running `converge` is required to apply the changes and is safe: it does not modify existing cloud resources. During migration, readiness checks are skipped for resources older than 5 days. Related warnings may appear and can be safely ignored.
 - Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by `cloud-provider-dvp` and avoid unnecessary or destructive plan changes when data sources depend on readiness checks. Other cloud providers are not affected. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - Cilium agents will be restarted during the update.
 - Custom edits to the local-path-config ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - During migration to the new Go-based implementation of apiserver-proxy, connection flaps to the API server may occur. This change exposes a new hostPort `6480` for health checks and upstreams statistics.
 - Istiod now enforces trust domain validation. Each remote root CA is now scoped to its declared trust domain in the meshConfig caCertificates. Verify that all IstioFederation resources have correct `trustDomain` values matching the remote cluster configuration.
 - Previously, the fencing-agent would crash with "permission denied" on /dev/watchdog
    when the node had a maintenance annotation (e.g. during Deckhouse updates).
    Now the agent skips watchdog arming during maintenance and arms it automatically
    when maintenance ends.
 - ServiceEntry and DestinationRule resources for federated public services will be recreated with new names. This causes a brief traffic interruption for cross-cluster federated service routing during the first reconciliation after the update.
 - The `local-path-provisioner` Pod is restarted during the update. Custom edits to the `local-path-config` ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected.
 - This update triggers a rolling update of the flannel pods.
 - This update triggers a rolling update of the kube-proxy pods.
 - This update triggers a rolling update of the network-policy-engine pods.
 - Unnecessary or destructive plan updates that could occur when updating labels and annotations via OpenTofu should be prevented now. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - When using containerdV2, the performance of istio-cni breaks when mounting internal paths.

## Features


 - **[admission-policy-engine]** Bumped Gatekeeper to 3.22.0 and Ratify to 1.4.0. [#18539](https://github.com/deckhouse/deckhouse/pull/18539)
 - **[admission-policy-engine]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:admission-policy-engine:admin-kubeconfig. [#19633](https://github.com/deckhouse/deckhouse/pull/19633)
 - **[candi]** Added multiversion parsing from `oss.yaml` in werf and tests. [#17956](https://github.com/deckhouse/deckhouse/pull/17956)
 - **[candi]** Added support for `x-kubernetes-sensitive-data` fields in custom resources with RBAC-based filtering and etcd encryption. [#18241](https://github.com/deckhouse/deckhouse/pull/18241)
    Enabling the feature gate `CRDSensitiveData` restarts kube-apiserver.
 - **[candi]** Enable DRA alpha feature gates for multi allocations [#17993](https://github.com/deckhouse/deckhouse/pull/17993)
    Kubelet, api-server, controller-manager and scheduler will be restarted.
 - **[candi]** Enabled DRA alpha feature gate `DRAPartitionableDevices`. [#18362](https://github.com/deckhouse/deckhouse/pull/18362)
    Kubelet, api-server, controller-manager and scheduler will be restarted.
 - **[cert-manager]** Add alerts for ACME Challenges stuck in pending or error states [#18439](https://github.com/deckhouse/deckhouse/pull/18439)
 - **[cert-manager]** Bumped version to v1.20.0. [#18064](https://github.com/deckhouse/deckhouse/pull/18064)
 - **[cloud-provider-aws]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-aws]** Moved spot node drain logic from node-termination-handler to Deckhouse. [#18385](https://github.com/deckhouse/deckhouse/pull/18385)
 - **[cloud-provider-azure]** Added NVMe disk discovery support for Ubuntu 22.04 Gen2 VMs. [#18839](https://github.com/deckhouse/deckhouse/pull/18839)
 - **[cloud-provider-azure]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-dvp]** Added ServiceWithHealthchecks support to `cloud-provider-dvp`. [#18141](https://github.com/deckhouse/deckhouse/pull/18141)
 - **[cloud-provider-dvp]** Added discovery and propagation of default StorageClass from parent cluster to child clusters in DVP. [#18295](https://github.com/deckhouse/deckhouse/pull/18295)
 - **[cloud-provider-dvp]** Added hybrid cluster support to `cloud-provider-dvp`. [#17861](https://github.com/deckhouse/deckhouse/pull/17861)
 - **[cloud-provider-dvp]** Added readiness check resource to prevent lost OpenTofu state if resource is not ready. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    A new resource, `kubernetes_resource_ready_v1`, has been introduced to perform readiness checks for cloud resources, replacing `wait` blocks. After upgrading, the OpenTofu plan will include adding the new resources and removing `wait` blocks. Running `converge` is required to apply the changes and is safe: it does not modify existing cloud resources. During migration, readiness checks are skipped for resources older than 5 days. Related warnings may appear and can be safely ignored.
 - **[cloud-provider-dvp]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-dvp]** Fail fast on dhctl operations if resources has incorrect status or conditions (like quota exceeded) with some limitations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[cloud-provider-dynamix]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-gcp]** Added `nestedVirtualization` and `additionalDisks` options to GCPInstanceClass. [#18023](https://github.com/deckhouse/deckhouse/pull/18023)
 - **[cloud-provider-gcp]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-huaweicloud]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-huaweicloud]** Enabled security policy checks and added SecurityPolicyException for Huawei Cloud CSI and CCM. [#18596](https://github.com/deckhouse/deckhouse/pull/18596)
 - **[cloud-provider-huaweicloud]** Migrated CAPI provider to the Cluster API v1beta2 contract. [#17989](https://github.com/deckhouse/deckhouse/pull/17989)
 - **[cloud-provider-huaweicloud]** Migrated VCD and Huawei Cloud to lib-helm defines; enabled security policy checks for VCD. [#18846](https://github.com/deckhouse/deckhouse/pull/18846)
 - **[cloud-provider-openstack]** Added a new optional parameter `csiDriver.fsGroupPolicy`. [#18965](https://github.com/deckhouse/deckhouse/pull/18965)
 - **[cloud-provider-openstack]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-openstack]** Disabled `enable-ingress-hostname` for Kubernetes >=1.32 to use proxy ipMode in LoadBalancer with proxy protocol. [#18524](https://github.com/deckhouse/deckhouse/pull/18524)
 - **[cloud-provider-vcd]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-vcd]** Migrated VCD and Huawei Cloud to lib-helm defines; enabled security policy checks for VCD. [#18846](https://github.com/deckhouse/deckhouse/pull/18846)
 - **[cloud-provider-vsphere]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-yandex]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-yandex]** Switched the default CNI to Cilium with VXLAN networking mode for new clusters to unify the configuration. [#19074](https://github.com/deckhouse/deckhouse/pull/19074)
 - **[cloud-provider-zvirt]** Added secrets with `node-manager` dependencies. [#18112](https://github.com/deckhouse/deckhouse/pull/18112)
 - **[cloud-provider-zvirt]** add customNetworkConfig [#17879](https://github.com/deckhouse/deckhouse/pull/17879)
 - **[cni-cilium]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All cilium agents will restart.
 - **[cni-cilium]** Added conntrack import/export HTTP endpoints. [#17429](https://github.com/deckhouse/deckhouse/pull/17429)
 - **[cni-cilium]** Added support for ICMP replies for ExternalIP load balancers. [#17266](https://github.com/deckhouse/deckhouse/pull/17266)
 - **[cni-cilium]** Reduced the CPU load in cilium-agent with hubble enabled. [#19772](https://github.com/deckhouse/deckhouse/pull/19772)
 - **[cni-flannel]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All flannel agents will restart.
 - **[cni-simple-bridge]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All simple-bridge agents will restart.
 - **[common]** Add token_namespace and token_name labels to serviceaccount_stale_tokens_total kube-apiserver metric [#19529](https://github.com/deckhouse/deckhouse/pull/19529)
 - **[common]** Added support for using `d8 k` as alias to `kubectl`. [#17033](https://github.com/deckhouse/deckhouse/pull/17033)
 - **[common]** add support accesiblenamespaces in k8s v1.35 [#18069](https://github.com/deckhouse/deckhouse/pull/18069)
 - **[control-plane-manager]** Added information to `d8-cluster-kubernetes` about the supported, available, and current `Automatic` versions of Kubernetes. [#18718](https://github.com/deckhouse/deckhouse/pull/18718)
 - **[control-plane-manager]** Extend d8:control-plane-manager:admin-kubeconfig-supplement with granular permissions for standard Kubernetes resources not covered by user-authz:cluster-admin. [#19633](https://github.com/deckhouse/deckhouse/pull/19633)
 - **[control-plane-manager]** Updated RBAC model for admin kubeconfig when `user-authz` is enabled. [#18996](https://github.com/deckhouse/deckhouse/pull/18996)
 - **[deckhouse-controller]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
 - **[deckhouse-controller]** Added credentials to the package repository. [#18264](https://github.com/deckhouse/deckhouse/pull/18264)
 - **[deckhouse-controller]** Added legacy (v1alpha1) PackageRepository module support with observability improvements. [#18522](https://github.com/deckhouse/deckhouse/pull/18522)
 - **[deckhouse-controller]** Added mechanism for blocking deckhouse release if cluster have alerts with high severity. [#20741](https://github.com/deckhouse/deckhouse/pull/20741)
 - **[deckhouse-controller]** Prevented enabling multiple CNI modules simultaneously. [#18479](https://github.com/deckhouse/deckhouse/pull/18479)
 - **[deckhouse]** Add conditions summary to applications. [#20317](https://github.com/deckhouse/deckhouse/pull/20317)
 - **[deckhouse]** Add marketplace features. [#20178](https://github.com/deckhouse/deckhouse/pull/20178)
 - **[deckhouse]** Added `lastAppliedConfiguration` to Application status. [#19303](https://github.com/deckhouse/deckhouse/pull/19303)
 - **[deckhouse]** Added hash-checking for webhook-handler rendered files. [#18409](https://github.com/deckhouse/deckhouse/pull/18409)
 - **[deckhouse]** Added validation of application settings against the schema from APV. [#19191](https://github.com/deckhouse/deckhouse/pull/19191)
 - **[deckhouse]** Changed Deckhouse VPA update mode to `InPlaceOrRecreate`. [#18661](https://github.com/deckhouse/deckhouse/pull/18661)
 - **[deckhouse]** Enhance package requirements. [#20322](https://github.com/deckhouse/deckhouse/pull/20322)
 - **[deckhouse]** Granted RBAC permissions for applications to Deckhouse. [#19385](https://github.com/deckhouse/deckhouse/pull/19385)
 - **[deckhouse]** Implemented single-page mode and last cursor for `ListTags`. [#18914](https://github.com/deckhouse/deckhouse/pull/18914)
 - **[deckhouse]** Improved webhook-handler with webhook-operator and CRDs for more complex user flow. [#15160](https://github.com/deckhouse/deckhouse/pull/15160)
 - **[deckhouse]** Optimized `jq` filter. [#19000](https://github.com/deckhouse/deckhouse/pull/19000)
 - **[deckhouse]** Removed module/application specific fields. [#18885](https://github.com/deckhouse/deckhouse/pull/18885)
 - **[deckhouse]** Set OpenAPI schemas from release image to APV status. [#19171](https://github.com/deckhouse/deckhouse/pull/19171)
 - **[deckhouse]** Webhook-handler will reload exited shell-operator now. [#19610](https://github.com/deckhouse/deckhouse/pull/19610)
 - **[descheduler]** Added `RemovePodsHavingTooManyRestarts` strategy to the `v1alpha2` API for evicting crash-looping pods. [#19122](https://github.com/deckhouse/deckhouse/pull/19122)
    Pods exceeding the configured restart threshold are evicted, freeing node resources and allowing the scheduler to place fresh pods on healthier nodes.
 - **[descheduler]** Added automatic enabling of Kubernetes Metrics API in `descheduler` policy when `metrics.k8s.io` is available in the cluster. [#19064](https://github.com/deckhouse/deckhouse/pull/19064)
    If the cluster serves the `metrics.k8s.io` API (e.g. metrics-server is installed), the `descheduler` policy now includes `metricsProviders` with source KubernetesMetrics, so utilization-related strategies can use Metrics API data. The `descheduler` Pod may restart when this flag or descheduler CR-driven policy changes due to ConfigMap/checksum updates.
 - **[descheduler]** Added configurable descheduling interval presets in ModuleConfig. [#19029](https://github.com/deckhouse/deckhouse/pull/19029)
 - **[descheduler]** Migrated conversion webhook from bash to ConversionWebhook CR-based mechanism. [#18499](https://github.com/deckhouse/deckhouse/pull/18499)
 - **[descheduler]** Updated `descheduler` to 0.35, with native support for filtering pods by namespace label selector and pod protection based on storage classes. [#18135](https://github.com/deckhouse/deckhouse/pull/18135)
 - **[descheduler]** Updated descheduler to the 0.35.1 version. [#18781](https://github.com/deckhouse/deckhouse/pull/18781)
 - **[dhctl]** Added ModuleConfig conversions. [#17917](https://github.com/deckhouse/deckhouse/pull/17917)
 - **[dhctl]** Added SSH public key validation. [#18119](https://github.com/deckhouse/deckhouse/pull/18119)
 - **[dhctl]** Added support for changing the default OpenTofu backend core and provider log levels with `TF_LOG_CORE` and `TF_LOG_PROVIDER` envs on run dhctl operations. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
 - **[dhctl]** Added support for standalone binary with on-demand dependency download. [#18482](https://github.com/deckhouse/deckhouse/pull/18482)
 - **[docs]** Added an alert for failed documentation renderings. [#18756](https://github.com/deckhouse/deckhouse/pull/18756)
 - **[ingress-nginx]** A configuration drift alert is added. [#18342](https://github.com/deckhouse/deckhouse/pull/18342)
    All Ingress-Nginx controller pods will be restarted.
 - **[istio]** Added monitoring of the reserved UID `1337` in pods. [#18633](https://github.com/deckhouse/deckhouse/pull/18633)
 - **[istio]** Added support for the `istio.io/rev:default` label. [#18320](https://github.com/deckhouse/deckhouse/pull/18320)
 - **[istio]** Added validating webhooks for IstioFederation and IstioMulticluster resources. [#18406](https://github.com/deckhouse/deckhouse/pull/18406)
    Creation of IstioFederation is only allowed when Istio federation is enabled in the module configuration. Creation of IstioMulticluster is only allowed when Istio multicluster is enabled in the module configuration.
 - **[istio]** Allow custom ports in metadataEndpoint URLs for IstioFederation and IstioMulticluster CRDs. [#19247](https://github.com/deckhouse/deckhouse/pull/19247)
 - **[istio]** Enabled read-only CNI-node root filesystem. [#19334](https://github.com/deckhouse/deckhouse/pull/19334)
 - **[istio]** Enabled trust domain validation in Istio control plane for federation and multicluster. [#18502](https://github.com/deckhouse/deckhouse/pull/18502)
    Istiod now enforces trust domain validation. Each remote root CA is now scoped to its declared trust domain in the meshConfig caCertificates. Verify that all IstioFederation resources have correct `trustDomain` values matching the remote cluster configuration.
 - **[istio]** Implement graceful metadata secret renewal for multiclusters. [#20202](https://github.com/deckhouse/deckhouse/pull/20202)
 - **[kube-proxy]** Added a mechanism to migrate between CNI plugins (e.g., Flannel to Cilium) in a running cluster. [#16499](https://github.com/deckhouse/deckhouse/pull/16499)
    All kube-proxy agents will restart.
 - **[log-shipper]** Added new transformations and parsing features. [#18685](https://github.com/deckhouse/deckhouse/pull/18685)
    log-shipper
 - **[loki]** Added generation and usage of a dedicated TLS server certificate for Loki kube-rbac-proxy [#18268](https://github.com/deckhouse/deckhouse/pull/18268)
 - **[multitenancy-manager]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:multitenancy-manager:admin-kubeconfig. [#19633](https://github.com/deckhouse/deckhouse/pull/19633)
 - **[node-local-dns]** Implemented a new nodeLocalDns option to disable IPv6 DNS resolving. [#19433](https://github.com/deckhouse/deckhouse/pull/19433)
 - **[node-manager]** Added automatic spot-terminated node handling with drain and instance cleanup. [#18385](https://github.com/deckhouse/deckhouse/pull/18385)
 - **[node-manager]** Added gossip-based node failure detection and gRPC API to fencing-agent. [#17771](https://github.com/deckhouse/deckhouse/pull/17771)
    The fencing-agent now uses gossip protocol (`memberlist`) for distributed node health monitoring.
    This prevents incorrect node reboots when control plane is unavailable but worker nodes are healthy.
    A new gRPC API is available via Unix socket at `/tmp/fencing-agent.sock` for querying node membership.
 - **[node-manager]** Migrated CAPS controller manager to lib-helm defines. [#18880](https://github.com/deckhouse/deckhouse/pull/18880)
 - **[node-manager]** Replaced NGINX implementation of apiserver-proxy with native Go application. [#17619](https://github.com/deckhouse/deckhouse/pull/17619)
    During migration to the new Go-based implementation of apiserver-proxy, connection flaps to the API server may occur. This change exposes a new hostPort `6480` for health checks and upstreams statistics.
 - **[registry]** Added bootstrap support for `Proxy` registry mode. [#18011](https://github.com/deckhouse/deckhouse/pull/18011)
 - **[terraform-manager]** Skip depends_on meta-argument changes for data sources. [#18212](https://github.com/deckhouse/deckhouse/pull/18212)
    Changes were introduced in the OpenTofu integration to support the new `kubernetes_resource_ready_v1` resource used by `cloud-provider-dvp` and avoid unnecessary or destructive plan changes when data sources depend on readiness checks. Other cloud providers are not affected. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - **[terraform-manager]** Suppressed destructive changes when updating labels and annotations for cloud resources via OpenTofu. [#19079](https://github.com/deckhouse/deckhouse/pull/19079)
    Unnecessary or destructive plan updates that could occur when updating labels and annotations via OpenTofu should be prevented now. If you encounter unexpected converge plans or cluster bootstrap issues when using OpenTofu-based providers (such as DVP, DynamiX, zVirt, or Yandex), report them to Deckhouse Technical Support.
 - **[user-authn]** Added SAML authentication provider support with refresh tokens and Single Logout (SLO). [#18002](https://github.com/deckhouse/deckhouse/pull/18002)
 - **[user-authn]** Added optional Gateway API HTTPRoute/ListenerSet publishing and updated oauth2-proxy auth responses. [#16812](https://github.com/deckhouse/deckhouse/pull/16812)
    All dex-authenticator pods will be restarted
 - **[user-authn]** Added self-service password reset via UserOperation. [#18710](https://github.com/deckhouse/deckhouse/pull/18710)
 - **[user-authn]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authn:admin-kubeconfig. [#19633](https://github.com/deckhouse/deckhouse/pull/19633)
 - **[user-authn]** Updated UI and branding items. [#18834](https://github.com/deckhouse/deckhouse/pull/18834)
 - **[user-authz]** Grant kubeadm:cluster-admins group granular full access to module CRDs via dedicated ClusterRole d8:user-authz:admin-kubeconfig. [#19633](https://github.com/deckhouse/deckhouse/pull/19633)
 - **[user-authz]** Restrict user roles from listing namespaces; use AccessibleNamespaces in non-CE editions [#17651](https://github.com/deckhouse/deckhouse/pull/17651)
 - **[vertical-pod-autoscaler]** Replaced deprecated `--humanize-memory` flag with `--round-memory-bytes=67108864` (64Mi) for human-readable memory recommendations. [#18932](https://github.com/deckhouse/deckhouse/pull/18932)
 - **[vertical-pod-autoscaler]** Updated VPA to 1.6.1, with new `--in-place-skip-disruption-budget` flag, support for skipping min-replica check and `InPlaceOrRecreate` feature now in GA. [#18336](https://github.com/deckhouse/deckhouse/pull/18336)

## Fixes


 - **[admission-policy-engine]** Changed default PSS policy to Baseline for unrecognized deckhouse versions [#18322](https://github.com/deckhouse/deckhouse/pull/18322)
 - **[admission-policy-engine]** Fix SecurityPolicyException handling for hostPorts-only exceptions [#18535](https://github.com/deckhouse/deckhouse/pull/18535)
 - **[admission-policy-engine]** Fix high resource consumption for constraint d8denyexecheritage [#19070](https://github.com/deckhouse/deckhouse/pull/19070)
 - **[admission-policy-engine]** Fixed enforcementAction for D8ReplicaLimits, added more template tests. [#18407](https://github.com/deckhouse/deckhouse/pull/18407)
 - **[admission-policy-engine]** Prevent unintended Gatekeeper constraints from being rendered for SecurityPolicy when boolean fields are omitted. [#18007](https://github.com/deckhouse/deckhouse/pull/18007)
    Workload Pods are no longer denied by unrelated SecurityPolicy checks (e.g. hostNetwork/hostPort) when corresponding policy fields are not explicitly set.
 - **[admission-policy-engine]** Revert  Changed default PSS policy to Baseline for unrecognized deckhouse versions [#19187](https://github.com/deckhouse/deckhouse/pull/19187)
 - **[candi]** Added deletion of webhook configurations before destroying a Deckhouse deployment. [#19041](https://github.com/deckhouse/deckhouse/pull/19041)
 - **[candi]** Fixed CVEs in `cloud-provider-azure`. [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[candi]** Fixed internal node IP discovery for static nodes in DVP clusters. [#18441](https://github.com/deckhouse/deckhouse/pull/18441)
 - **[candi]** fix cve node-manager and opentofu. [#19940](https://github.com/deckhouse/deckhouse/pull/19940)
 - **[candi]** fix if node has bashible-uninitialized taint in race condition. [#18133](https://github.com/deckhouse/deckhouse/pull/18133)
 - **[cert-manager]** Disable SecurityPolicyExceptions for cert-manager namespace [#19184](https://github.com/deckhouse/deckhouse/pull/19184)
 - **[cilium-hubble]** Fixed CVE-2026-29181 in hubble-ui-backend  by bumping OpenTelemetry Go to v1.41.0 [#20250](https://github.com/deckhouse/deckhouse/pull/20250)
 - **[cilium-hubble]** Fixed CVE-2026-33186 in the hubble-ui image. [#18657](https://github.com/deckhouse/deckhouse/pull/18657)
 - **[cilium-hubble]** Fixed CVE-2026-41520 in hubble-ui-backend [#20360](https://github.com/deckhouse/deckhouse/pull/20360)
 - **[cloud-provider-aws]** Fixed detection of regional limitations versus IAM issues. [#19054](https://github.com/deckhouse/deckhouse/pull/19054)
 - **[cloud-provider-aws]** Install linux-modules-extra on Ubuntu nodes [#19426](https://github.com/deckhouse/deckhouse/pull/19426)
 - **[cloud-provider-aws]** add information about AWS security group rules limits [#18819](https://github.com/deckhouse/deckhouse/pull/18819)
 - **[cloud-provider-aws]** fix CVE in cloud-provider-aws [#18057](https://github.com/deckhouse/deckhouse/pull/18057)
 - **[cloud-provider-aws]** fix getInstancesByIDs to comply with the describeInstanceBatcher. [#18267](https://github.com/deckhouse/deckhouse/pull/18267)
 - **[cloud-provider-azure]** Fixed CVEs in `cloud-provider-azure`. [#18067](https://github.com/deckhouse/deckhouse/pull/18067)
 - **[cloud-provider-azure]** fix CVEs in cloud-provider-azure [#18240](https://github.com/deckhouse/deckhouse/pull/18240)
 - **[cloud-provider-dvp]** Add skip storage class annotation handling to skip discovery of some storage classes from parent clusters, e.g., local disks. [#19783](https://github.com/deckhouse/deckhouse/pull/19783)
 - **[cloud-provider-dvp]** Allowed using `additionalDisks` in master InstanceClasses. [#17352](https://github.com/deckhouse/deckhouse/pull/17352)
 - **[cloud-provider-dvp]** Fixed CVEs. [#19362](https://github.com/deckhouse/deckhouse/pull/19362)
 - **[cloud-provider-dvp]** Fixed invalid and unpredictable logic in the DeckhouseMachine controller. [#18715](https://github.com/deckhouse/deckhouse/pull/18715)
 - **[cloud-provider-dvp]** Fixed missing SSH public keys for ephemeral nodes. [#19357](https://github.com/deckhouse/deckhouse/pull/19357)
 - **[cloud-provider-dvp]** Suppressed destructive changes when updating labels and annotations for cloud resources via OpenTofu. [#19079](https://github.com/deckhouse/deckhouse/pull/19079)
 - **[cloud-provider-dvp]** add labels to cloudinit secrets in the terraform [#20436](https://github.com/deckhouse/deckhouse/pull/20436)
 - **[cloud-provider-dvp]** fix CVEs in cloud-provider-dvp [#18258](https://github.com/deckhouse/deckhouse/pull/18258)
 - **[cloud-provider-dvp]** fix LoadBalancer stuck in pending state — retry on conflict when updating ServiceWithHealthchecks and propagate IP to child cluster service status [#19609](https://github.com/deckhouse/deckhouse/pull/19609)
 - **[cloud-provider-dvp]** refactored CreateVolume to improve idempotency when disk.status.capacity is not yet reported and standardized gRPC error handling [#17826](https://github.com/deckhouse/deckhouse/pull/17826)
 - **[cloud-provider-gcp]** Fixed CVEs in `cloud-provider-gcp`. [#18095](https://github.com/deckhouse/deckhouse/pull/18095)
 - **[cloud-provider-huaweicloud]** Added default values for `elb.class` and `lb-algorithm`, and fixed load balancer creation when `epid` is empty. [#19166](https://github.com/deckhouse/deckhouse/pull/19166)
 - **[cloud-provider-huaweicloud]** Fixed CVEs in `cloud-provider-huaweicloud`. [#18096](https://github.com/deckhouse/deckhouse/pull/18096)
 - **[cloud-provider-huaweicloud]** fix CVEs in cloud-provider-huaweicloud [#18289](https://github.com/deckhouse/deckhouse/pull/18289)
 - **[cloud-provider-openstack]** Add loadBalancer.enabled flag to prevent CCM crashes on k8s 1.32 without Octavia service [#18228](https://github.com/deckhouse/deckhouse/pull/18228)
 - **[cloud-provider-openstack]** Fixed CVEs in `cloud-provider-openstack`. [#18099](https://github.com/deckhouse/deckhouse/pull/18099)
 - **[cloud-provider-openstack]** Increase interval and timeout for health monitor [#19308](https://github.com/deckhouse/deckhouse/pull/19308)
 - **[cloud-provider-openstack]** fix CVE in cloud-provider-openstack module [#18253](https://github.com/deckhouse/deckhouse/pull/18253)
 - **[cloud-provider-openstack]** fix LB.enabled flag [#18402](https://github.com/deckhouse/deckhouse/pull/18402)
 - **[cloud-provider-vcd]** Fix LogrAdapter panic in VCD infra-controller-manager [#20148](https://github.com/deckhouse/deckhouse/pull/20148)
 - **[cloud-provider-vcd]** Fixed CVEs in `cloud-provider-vcd`. [#18113](https://github.com/deckhouse/deckhouse/pull/18113)
 - **[cloud-provider-vcd]** Fixed SecurityPolicyException for VCD components. [#19021](https://github.com/deckhouse/deckhouse/pull/19021)
 - **[cloud-provider-vcd]** fix vCD CCM TCP health monitors removal [#19089](https://github.com/deckhouse/deckhouse/pull/19089)
 - **[cloud-provider-vsphere]** Added filtering discovered zones and datastores by `zones` from provider configurations. [#18378](https://github.com/deckhouse/deckhouse/pull/18378)
 - **[cloud-provider-vsphere]** Enabled the vSphere CSI snapshotter. [#18263](https://github.com/deckhouse/deckhouse/pull/18263)
 - **[cloud-provider-vsphere]** Fix vSphere privilege matrix and describe instructions for setting up environment via vSphere Client [#18725](https://github.com/deckhouse/deckhouse/pull/18725)
 - **[cloud-provider-vsphere]** normalizes new paths and makes bashible resolve existing paths case-insensitively [#19747](https://github.com/deckhouse/deckhouse/pull/19747)
 - **[cloud-provider-yandex]** Fixed removing public IP addresses from nodes by deleting `externalIPAddresses`. [#18364](https://github.com/deckhouse/deckhouse/pull/18364)
 - **[cloud-provider-yandex]** fix CVEs in cloud-provider-yandex [#18291](https://github.com/deckhouse/deckhouse/pull/18291)
 - **[cloud-provider-zvirt]** Fixed CVEs in `cloud-provider-zvirt`. [#18115](https://github.com/deckhouse/deckhouse/pull/18115)
 - **[cloud-provider-zvirt]** fix CSI token refresh patch apply [#18449](https://github.com/deckhouse/deckhouse/pull/18449)
 - **[cloud-provider-zvirt]** fix CVEs in cloud-provider-zvirt [#18257](https://github.com/deckhouse/deckhouse/pull/18257)
 - **[cni-cilium]** Fixed CVE-2026-33186, CVE-2026-27142, and CVE-2026-27139 by updating grpc dependency and Go version, and resolved build compatibility issues. [#18553](https://github.com/deckhouse/deckhouse/pull/18553)
 - **[cni-cilium]** Fixed CVE-2026-41520 for cilium-bugtool util [#20240](https://github.com/deckhouse/deckhouse/pull/20240)
 - **[cni-cilium]** Fixed constant `invalid sysctl parameter: "net.ipv4.conf..rp_filter"` errors in cilium-agent logs when using Egress Gateway with a Virtual IP. [#18952](https://github.com/deckhouse/deckhouse/pull/18952)
 - **[cni-cilium]** Updated go-jose dependency to v4.1.4 to fix CVE-2026-34986. [#18984](https://github.com/deckhouse/deckhouse/pull/18984)
    Cilium agents will be restarted during the update.
 - **[cni-flannel]** Fixed CVE-2026-33186 by updating google.golang.org/grpc in flanneld. [#18995](https://github.com/deckhouse/deckhouse/pull/18995)
    This update triggers a rolling update of the flannel pods.
 - **[cni-flannel]** Reverted module stage from Deprecated back to General Availability to stop false deprecation alerts. [#20305](https://github.com/deckhouse/deckhouse/pull/20305)
 - **[cni-simple-bridge]** Fix simple bridge script to add ip rule for two NICs nodes. [#20533](https://github.com/deckhouse/deckhouse/pull/20533)
 - **[cni-simple-bridge]** Refactored python image source and pip exclusion. [#19113](https://github.com/deckhouse/deckhouse/pull/19113)
 - **[common]** Fixed CVE-2026-24051 in the CoreDNS image. [#18545](https://github.com/deckhouse/deckhouse/pull/18545)
 - **[common]** Fixed CVE-2026-33186 in the CoreDNS image. [#18656](https://github.com/deckhouse/deckhouse/pull/18656)
    CoreDNS pods will undergo a rolling restart.
 - **[common]** Fixed replacing the `kubectl` binary with the `d8 k` alias. [#18467](https://github.com/deckhouse/deckhouse/pull/18467)
 - **[common]** Removed Python completely from the debug-container image as it is no longer needed, resolving corresponding CVEs, and silenced false positives for etcd binaries via VEX. [#18810](https://github.com/deckhouse/deckhouse/pull/18810)
 - **[common]** fix cve's in docker-registry docker_auth image. [#19356](https://github.com/deckhouse/deckhouse/pull/19356)
 - **[common]** fix for replace kubectl binary with d8 k alias. [#18514](https://github.com/deckhouse/deckhouse/pull/18514)
 - **[common]** fixed CVE-2026-29181 in the CoreDNS [#20257](https://github.com/deckhouse/deckhouse/pull/20257)
 - **[control-plane-manager]** Excluded learner etcd members from the kube-apiserver etcd member list. [#19164](https://github.com/deckhouse/deckhouse/pull/19164)
 - **[control-plane-manager]** Fix order of converge components in control-plane-manager. [#18195](https://github.com/deckhouse/deckhouse/pull/18195)
 - **[control-plane-manager]** Fixed incorrect UpdateObserver progress calculation during cluster upgrades. [#19160](https://github.com/deckhouse/deckhouse/pull/19160)
 - **[control-plane-manager]** Skip rebind of ClusterRoleBinding/kubeadm:cluster-admins until the cluster is fully bootstrapped; harden the reconciliation hook. Fixes "cannot change roleRef" on fresh clusters. [#19744](https://github.com/deckhouse/deckhouse/pull/19744)
 - **[control-plane-manager]** Upgraded etcd to 3.6.10. [#19273](https://github.com/deckhouse/deckhouse/pull/19273)
    Etcd will restart.
 - **[csi-vsphere]** Fixed the Deckhouse queue getting stuck [#20092](https://github.com/deckhouse/deckhouse/pull/20092)
 - **[deckhouse-controller]** A module that conditionally depends on another is no longer disabled when an incompatible version of that dependency is enabled; the enable is rejected instead. [#20344](https://github.com/deckhouse/deckhouse/pull/20344)
 - **[deckhouse-controller]** Fix applications charts rendering issue [#20282](https://github.com/deckhouse/deckhouse/pull/20282)
 - **[deckhouse-controller]** Fix false DeckhouseUpdatingFailed alert on registries without version tags in release-channel repo [#18310](https://github.com/deckhouse/deckhouse/pull/18310)
 - **[deckhouse-controller]** Fixed error logging for MPO validation. [#18698](https://github.com/deckhouse/deckhouse/pull/18698)
 - **[deckhouse-controller]** Fixed validation for switching ClusterConfiguration kubernetesVersion from an explicit version to Automatic. [#20331](https://github.com/deckhouse/deckhouse/pull/20331)
 - **[deckhouse-controller]** added extra validation for kubernets version multiple downgrades scenario [#18794](https://github.com/deckhouse/deckhouse/pull/18794)
 - **[deckhouse]** Allow updating scanInterval on the deckhouse ModuleSource. [#19277](https://github.com/deckhouse/deckhouse/pull/19277)
 - **[deckhouse]** Bumped Hugo and `x/image` to fix CVE-2026-33809, CVE-2026-35166. [#18985](https://github.com/deckhouse/deckhouse/pull/18985)
 - **[deckhouse]** Bumped `nelm` to fix a deadlock. [#18585](https://github.com/deckhouse/deckhouse/pull/18585)
 - **[deckhouse]** Bumped `shell-operator` to v1.15.3 and webhook-operator dependencies. [#19030](https://github.com/deckhouse/deckhouse/pull/19030)
 - **[deckhouse]** Ensure heritage label on d8-system namespace via hook. [#19134](https://github.com/deckhouse/deckhouse/pull/19134)
 - **[deckhouse]** Fix Scaled stuck Unknown on controller startup [#20467](https://github.com/deckhouse/deckhouse/pull/20467)
 - **[deckhouse]** Fix package status deadlock via coalescing workqueue. [#20695](https://github.com/deckhouse/deckhouse/pull/20695)
 - **[deckhouse]** Fixed a race condition in ModuleConfig processing during startup. [#18280](https://github.com/deckhouse/deckhouse/pull/18280)
 - **[deckhouse]** Fixed global configuration generation. [#18161](https://github.com/deckhouse/deckhouse/pull/18161)
 - **[deckhouse]** Fixed module updates skipping patch releases when updating to a new minor version. [#19328](https://github.com/deckhouse/deckhouse/pull/19328)
 - **[deckhouse]** Overwrite currentReleaseImageName on mismatch. [#19412](https://github.com/deckhouse/deckhouse/pull/19412)
 - **[deckhouse]** Remove notified=false annotation reset from runReleaseDeploy in the module release controller. [#19169](https://github.com/deckhouse/deckhouse/pull/19169)
 - **[deckhouse]** Restore ModuleIsInMaintenanceMode alert by switching to d8_module_config_maintenance sourced from ModuleConfig. [#19352](https://github.com/deckhouse/deckhouse/pull/19352)
 - **[deckhouse]** Revoke permission to use moduleconfig to user. [#19698](https://github.com/deckhouse/deckhouse/pull/19698)
 - **[deckhouse]** Use non-controller ownerRefs for multi-source package CRs. [#20463](https://github.com/deckhouse/deckhouse/pull/20463)
 - **[dhctl]** Added a preflight check for validating InstanceClasses against the selected cloud provider. [#18473](https://github.com/deckhouse/deckhouse/pull/18473)
 - **[dhctl]** Added validation of the command execution status code [#18128](https://github.com/deckhouse/deckhouse/pull/18128)
 - **[dhctl]** Excluded `BaseInfraPhase` from the progress phase list for static clusters. [#17856](https://github.com/deckhouse/deckhouse/pull/17856)
 - **[dhctl]** Fix CVEs in `dhctl`. [#19344](https://github.com/deckhouse/deckhouse/pull/19344)
 - **[dhctl]** Fix deadlock in converge. [#18335](https://github.com/deckhouse/deckhouse/pull/18335)
 - **[dhctl]** Fix restart bootstrap during creating additional nodes in cloud permanent node groups. [#18525](https://github.com/deckhouse/deckhouse/pull/18525)
 - **[dhctl]** Fixed CVE-2026-33186. [#18805](https://github.com/deckhouse/deckhouse/pull/18805)
 - **[dhctl]** Fixed `LogInfoLn` behavior for external loggers. [#19234](https://github.com/deckhouse/deckhouse/pull/19234)
 - **[dhctl]** Fixed a panic in infrastructure plan processing when the destructive changes report returns an error. [#18908](https://github.com/deckhouse/deckhouse/pull/18908)
 - **[dhctl]** Fixed namespace updates during cluster bootstrap when the namespace already exists. [#19129](https://github.com/deckhouse/deckhouse/pull/19129)
 - **[dhctl]** Refactored preflight checks. [#17564](https://github.com/deckhouse/deckhouse/pull/17564)
 - **[dhctl]** Skip tmp lock for exporter and auto-converger. [#18736](https://github.com/deckhouse/deckhouse/pull/18736)
 - **[dhctl]** Switched to using the system certificate pool together with custom registry CAs for registry TLS handling. [#18978](https://github.com/deckhouse/deckhouse/pull/18978)
 - **[dhctl]** Use internal node ip if bastion ip was passed in converge. [#18979](https://github.com/deckhouse/deckhouse/pull/18979)
 - **[dhctl]** Wait for stronghold cluster sync before node deletion [#19793](https://github.com/deckhouse/deckhouse/pull/19793)
 - **[dhctl]** add NodeReady wait to dhctl converge and improve etcd check output [#18991](https://github.com/deckhouse/deckhouse/pull/18991)
 - **[dhctl]** fix SSH preflight check for StaticInstances with password-only auth. [#19560](https://github.com/deckhouse/deckhouse/pull/19560)
 - **[dhctl]** fixed the `killall kubectl` command for the `d8 k` alias [#20423](https://github.com/deckhouse/deckhouse/pull/20423)
 - **[dhctl]** fixed the `pkill d8 k proxy` command in `dhctl` [#20460](https://github.com/deckhouse/deckhouse/pull/20460)
 - **[dhctl]** mitigate CVE-2026-33186 [#18625](https://github.com/deckhouse/deckhouse/pull/18625)
 - **[dhctl]** Aix non-strict unmarshalling for metaconfigs. [#18359](https://github.com/deckhouse/deckhouse/pull/18359)
 - **[docs]** Add info about kernel requirement for containerdv2 migration. [#19505](https://github.com/deckhouse/deckhouse/pull/19505)
 - **[docs]** Fix vSphere privilege matrix and describe instructions for setting up environment via vSphere Client [#18725](https://github.com/deckhouse/deckhouse/pull/18725)
 - **[docs]** Updated the `d8 cni-migration` commands in the CNI migration guide to `d8 network cni-migration`. [#18547](https://github.com/deckhouse/deckhouse/pull/18547)
 - **[extended-monitoring]** fix typo in image-availability-exporter template [#18595](https://github.com/deckhouse/deckhouse/pull/18595)
 - **[ingress-nginx]** Added fix for CVE-2026-4342. [#18923](https://github.com/deckhouse/deckhouse/pull/18923)
    All Ingress-NGINX controller pods will be restated.
 - **[ingress-nginx]** Added validating x-forwarded-port and x-forwarded-proto headers when redirecting from www. [#19081](https://github.com/deckhouse/deckhouse/pull/19081)
    All Ingress-NGINX controller pods of 1.12 and 1.14 will be restarted.
 - **[ingress-nginx]** CVE-2025-15566 is fixed in 1.10 and 1.12 controllers. [#19205](https://github.com/deckhouse/deckhouse/pull/19205)
    All pods of Ingress-NGINX controller of 1.10 and 1.12 versions will be restarted.
 - **[ingress-nginx]** CVE-2026-3288 is mitigated in all Ingress-Nginx controllers. [#18387](https://github.com/deckhouse/deckhouse/pull/18387)
    All Ingress-Nginx controller pods will be restarted.
 - **[ingress-nginx]** Fixing mount points in a geoproxy image. [#20126](https://github.com/deckhouse/deckhouse/pull/20126)
    Geoproxy image will be restarted.
 - **[ingress-nginx]** Initial ingress store sync is  fixed. [#19031](https://github.com/deckhouse/deckhouse/pull/19031)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** Nginx is updated up to 1.30.1. [#19865](https://github.com/deckhouse/deckhouse/pull/19865)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Nginx was updated to 1.30.2. [#20200](https://github.com/deckhouse/deckhouse/pull/20200)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** Node-specific parameters are excluded from config hash. [#18489](https://github.com/deckhouse/deckhouse/pull/18489)
    All pods of Ingress-NGINX controller will be restarted.
 - **[istio]** Added CARGO_PROXY to ztunnel image build [#20595](https://github.com/deckhouse/deckhouse/pull/20595)
 - **[istio]** CNI-node readonly root filesystem enable fix [#19920](https://github.com/deckhouse/deckhouse/pull/19920)
    When using containerdV2, the performance of istio-cni breaks when mounting internal paths.
 - **[istio]** Deduplicated federation ServiceEntry and DestinationRule resources by hostname across multiple IstioFederation CRs. [#18375](https://github.com/deckhouse/deckhouse/pull/18375)
    ServiceEntry and DestinationRule resources for federated public services will be recreated with new names. This causes a brief traffic interruption for cross-cluster federated service routing during the first reconciliation after the update.
 - **[istio]** Fixed indent in ztunnel daemonset template [#18256](https://github.com/deckhouse/deckhouse/pull/18256)
 - **[istio]** Reduce CPU and RAM for regenerate multicluster JWT token and sort ingressGateway [#18554](https://github.com/deckhouse/deckhouse/pull/18554)
 - **[istio]** added iptables wrapper in cni-v1x21x6 [#18925](https://github.com/deckhouse/deckhouse/pull/18925)
    istio-cni-nodes will be restarted
 - **[istio]** fixed CVE-2026-33186 in v1.21.6 images [#18676](https://github.com/deckhouse/deckhouse/pull/18676)
    pods in namespace d8-istio will be restarted
 - **[istio]** fixed CVE-2026-33186 in v1.25.2 images [#18636](https://github.com/deckhouse/deckhouse/pull/18636)
    pods in namespace d8-istio will be restarted
 - **[istio]** fixed CVE-2026-34986 [#18972](https://github.com/deckhouse/deckhouse/pull/18972)
    istio module pods will be restarted
 - **[istio]** fixed CVE-2026-39882, CVE-2026-39883 and CVE-2026-35206 [#19085](https://github.com/deckhouse/deckhouse/pull/19085)
    istio module pods will be restarted
 - **[istio]** fixed CVEs in module images [#19584](https://github.com/deckhouse/deckhouse/pull/19584)
    module pods will be restarted
 - **[istio]** fixed discovery_operator_versions_to_install.go hook to migrate from 1.21 to 1.25 [#19648](https://github.com/deckhouse/deckhouse/pull/19648)
 - **[istio]** ingressGateway advertise FQDN does not create a ServiceEntry due to an error [#19528](https://github.com/deckhouse/deckhouse/pull/19528)
 - **[keepalived]** Excluded vulnerable pip-25.3 from keepalived final image to fix CVE-2026-1703 [#19111](https://github.com/deckhouse/deckhouse/pull/19111)
 - **[kube-proxy]** Fixed CVE-2026-33186 and CVE-2026-24051 in kube-proxy dependencies. [#19002](https://github.com/deckhouse/deckhouse/pull/19002)
    This update triggers a rolling update of the kube-proxy pods.
 - **[local-path-provisioner]** Bump `local-path-provisioner` to `v0.0.34` to fix CVE-2025-62878 (path traversal via `StorageClass.parameters.pathPattern`, CVSS 10.0). [#19345](https://github.com/deckhouse/deckhouse/pull/19345)
    The `local-path-provisioner` Pod is restarted during the update. PV provisioning/teardown briefly pauses while the new Pod becomes Ready; existing volumes are not affected.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to pick up the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7). [#20449](https://github.com/deckhouse/deckhouse/pull/20449)
    Custom edits to the local-path-config ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - **[local-path-provisioner]** Update local-path-provisioner to v0.0.36 to pick up the upstream fix for CVE-2026-44543 (HelperPod template injection, CVSS 8.7). [#20456](https://github.com/deckhouse/deckhouse/pull/20456)
    The `local-path-provisioner` Pod is restarted during the update. Custom edits to the `local-path-config` ConfigMap that set unsafe HelperPod fields (privileged, capabilities, host namespaces, initContainers, custom volumes/volumeMounts, container probes/lifecycle, sysctls, etc.) will be rejected by the provisioner at startup. Default Deckhouse installations are unaffected.
 - **[monitoring-kubernetes]** Resolved port conflict with the runtime-audit-engine module and removed excessive pod privileges [#18868](https://github.com/deckhouse/deckhouse/pull/18868)
 - **[multitenancy-manager]** allow DNS queries for default ProjectTemplate [#18572](https://github.com/deckhouse/deckhouse/pull/18572)
 - **[network-gateway]** Updated dnsmasq to v2.92-alt2 to address multiple security vulnerabilities (CVE-2026-*) [#19933](https://github.com/deckhouse/deckhouse/pull/19933)
 - **[network-gateway]** Updated python image source and mitigated pip CVE-2026-1703 [#19114](https://github.com/deckhouse/deckhouse/pull/19114)
 - **[network-policy-engine]** Fixed CVE-2026-34040, CVE-2026-33997, and CVE-2026-33186 in network-policy-engine dependencies. [#19005](https://github.com/deckhouse/deckhouse/pull/19005)
    This update triggers a rolling update of the network-policy-engine pods.
 - **[network-policy-engine]** Reverted module stage from Deprecated back to General Availability to stop false deprecation alerts. [#20305](https://github.com/deckhouse/deckhouse/pull/20305)
 - **[node-local-dns]** Adapt node-local-dns for air-gapped environments. [#18643](https://github.com/deckhouse/deckhouse/pull/18643)
 - **[node-local-dns]** Fix name of registry secret in safe-updater deployment [#18673](https://github.com/deckhouse/deckhouse/pull/18673)
 - **[node-local-dns]** Fix werf manifest [#18738](https://github.com/deckhouse/deckhouse/pull/18738)
 - **[node-local-dns]** Return stale-dns-connections-cleaner [#18707](https://github.com/deckhouse/deckhouse/pull/18707)
    An additional service daemonset will be added.
 - **[node-manager]** Add RBAC rules for node-manager [#19720](https://github.com/deckhouse/deckhouse/pull/19720)
 - **[node-manager]** Added cleanup for oversized MCM MachineSet revision history annotation [#19655](https://github.com/deckhouse/deckhouse/pull/19655)
 - **[node-manager]** Fencing controller no longer deletes Node objects for Notify-mode and Static/CloudStatic nodes. [#18218](https://github.com/deckhouse/deckhouse/pull/18218)
 - **[node-manager]** Fix capacity parsing logic for DVPInstanceClass and add test case for DVPSpecWorker [#17935](https://github.com/deckhouse/deckhouse/pull/17935)
    Capacity values (CPU/memory) for DVPInstanceClass are now correctly extracted according to spec shape. Nested `virtualMachine` fields are used and memory quantities like `Gi` are properly parsed.
 - **[node-manager]** Fix cluster-autoscaler deadlock when machine creation fails with a non-ResourceExhausted error, preventing scale-up to alternative node groups. [#18154](https://github.com/deckhouse/deckhouse/pull/18154)
 - **[node-manager]** Fix fencing-agent crash when starting on a node in maintenance mode. [#20583](https://github.com/deckhouse/deckhouse/pull/20583)
    Previously, the fencing-agent would crash with "permission denied" on /dev/watchdog
    when the node had a maintenance annotation (e.g. during Deckhouse updates).
    Now the agent skips watchdog arming during maintenance and arms it automatically
    when maintenance ends.
 - **[node-manager]** Fixed GPU observability in node-manager for full GPU, MIG, and time-slicing workloads (dashboard links/queries, VRAM semantics, MIG slice visibility), stabilized DCGM profiling metrics pipeline, synced MIG profile config with upstream, and made custom MIG defaults explicit for unspecified GPU indexes. [#18287](https://github.com/deckhouse/deckhouse/pull/18287)
 - **[node-manager]** Improve fencing-agent health monitor logging — warn on fallback feeding, error on watchdog starvation, add diagnostic context to all feeding log messages. [#19514](https://github.com/deckhouse/deckhouse/pull/19514)
    Operators can now detect degraded fencing states (quorum loss, API unreachability) through log levels and diagnostic fields without parsing log messages.
 - **[node-manager]** Include system labels in CAPI MachineDeployment capacity annotation for correct scale-from-zero behavior [#20387](https://github.com/deckhouse/deckhouse/pull/20387)
    On CAPI-based clusters (DVP, VCD, zVirt, Dynamix, HuaweiCloud), scale-from-zero now correctly handles pods with nodeSelector targeting system labels (node.deckhouse.io/group, node.deckhouse.io/type, node-role.kubernetes.io/<ng-name>). Previously such pods remained Pending indefinitely when NodeGroup had minPerZone=0. No user action required — the fix is applied automatically on upgrade.
 - **[node-manager]** add rbac policies for persistantvolumes to manage from capi-controller-manager. [#20646](https://github.com/deckhouse/deckhouse/pull/20646)
 - **[node-manager]** caps fix inconsistent pending staticinstance [#18379](https://github.com/deckhouse/deckhouse/pull/18379)
 - **[node-manager]** deploy capi controller and webhooks before basic resources to prevent race condition during upgrades. [#18754](https://github.com/deckhouse/deckhouse/pull/18754)
 - **[node-manager]** fix Cluster Autoscaler RBAC for CAPI providers, add missing machinedeployments/scale to write rule and patch verb to ClusterRole. [#18818](https://github.com/deckhouse/deckhouse/pull/18818)
 - **[node-manager]** fix draining hook event generation [#19165](https://github.com/deckhouse/deckhouse/pull/19165)
 - **[node-manager]** fix go lint errors in node-controller [#18187](https://github.com/deckhouse/deckhouse/pull/18187)
 - **[node-manager]** fix to caps for use staticmachine creationtimestamp [#18821](https://github.com/deckhouse/deckhouse/pull/18821)
 - **[node-manager]** fix webook validation in node-controller on cri changes in nodegroup. [#20098](https://github.com/deckhouse/deckhouse/pull/20098)
 - **[node-manager]** hook to restore apiVersion on CAPI resources. [#20374](https://github.com/deckhouse/deckhouse/pull/20374)
 - **[node-manager]** mitigate CVE-2026-33186 [#18649](https://github.com/deckhouse/deckhouse/pull/18649)
 - **[node-manager]** optimize node-controller cache resources usage. [#19097](https://github.com/deckhouse/deckhouse/pull/19097)
 - **[node-manager]** rollback inject capi conversion hook in crds as on-before-helm [#19137](https://github.com/deckhouse/deckhouse/pull/19137)
 - **[operator-prometheus]** op_ functions support [#20438](https://github.com/deckhouse/deckhouse/pull/20438)
 - **[prometheus]** Fix externalLabels handling in conjunction with the PrometheusRemoteWrites [#18608](https://github.com/deckhouse/deckhouse/pull/18608)
 - **[registry-packages-proxy]** fix possible deadlock in cache retention policy [#18505](https://github.com/deckhouse/deckhouse/pull/18505)
 - **[registry]** Updated auth image Go dependencies to fix Go CVEs. [#18234](https://github.com/deckhouse/deckhouse/pull/18234)
    Registry pods will be restarted.
 - **[registrypackages]** Added vex with CVE-2026-33186. [#18680](https://github.com/deckhouse/deckhouse/pull/18680)
 - **[registrypackages]** Replace symlinks with actual files in kubernetes artifacts for werf 2.57.1 compatibility [#18662](https://github.com/deckhouse/deckhouse/pull/18662)
 - **[upmeter]** Add proper securityContext to the upmeter probe to meet the restricted security profile constraints. [#18492](https://github.com/deckhouse/deckhouse/pull/18492)
 - **[upmeter]** Switched smoke-mini checks to full service FQDN to reduce unnecessary requests. Added request/session timeouts to prevent hanging probe calls. [#20406](https://github.com/deckhouse/deckhouse/pull/20406)
    upmeter probes
 - **[upmeter]** checks for Observability module in Upmeter + fix Grafana v10 [#19530](https://github.com/deckhouse/deckhouse/pull/19530)
 - **[upmeter]** fix D8UpmeterProbeGarbagePodsFromDeployments flapping [#19533](https://github.com/deckhouse/deckhouse/pull/19533)
 - **[upmeter]** fix invalid promql expr [#19591](https://github.com/deckhouse/deckhouse/pull/19591)
 - **[upmeter]** observability probes no longer fail when run.ID() produces a digits-only hash of the node name [#20327](https://github.com/deckhouse/deckhouse/pull/20327)
 - **[user-authn]** Add "cache" get parameter to prevent stale caches from breaking login page [#18976](https://github.com/deckhouse/deckhouse/pull/18976)
 - **[user-authn]** Disable implicit flow due to security concerns. [#18288](https://github.com/deckhouse/deckhouse/pull/18288)
 - **[user-authn]** Improve basic-auth-proxy request handling, cache implementation, and shutdown behavior. [#20089](https://github.com/deckhouse/deckhouse/pull/20089)
 - **[user-authn]** Restore ContinueOnConnectorFailure flag handling in Dex configuration [#18219](https://github.com/deckhouse/deckhouse/pull/18219)
 - **[user-authz]** Extend cluster-admin clusterrole  with kubelet-api-admin rights. [#19888](https://github.com/deckhouse/deckhouse/pull/19888)
 - **[user-authz]** Fix multi-tenancy namespace visibility for users without ClusterAuthorizationRules [#18689](https://github.com/deckhouse/deckhouse/pull/18689)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images, now available 1.33.11, 1.34.7, 1.35.4 [#19271](https://github.com/deckhouse/deckhouse/pull/19271)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Bump patch versions of Kubernetes images. [#18175](https://github.com/deckhouse/deckhouse/pull/18175)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Make flag encryption-provider-config-automatic-reload auto enabled when secretEncryptionKey is true [#19287](https://github.com/deckhouse/deckhouse/pull/19287)
    Apiserver will restart if secretEncryptionKey is true
 - **[candi]** add container-selinux package for selinux policies on rhel based distributions. [#17714](https://github.com/deckhouse/deckhouse/pull/17714)
 - **[cilium-hubble]** Added vex with CVE-2026-33726 for hubble [#18913](https://github.com/deckhouse/deckhouse/pull/18913)
 - **[cilium-hubble]** Fixed vex file [#19001](https://github.com/deckhouse/deckhouse/pull/19001)
 - **[cilium-hubble]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[cloud-provider-dvp]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[cloud-provider-dvp]** add ownerReferences to VM-related objects (managed by Terraform) [#16777](https://github.com/deckhouse/deckhouse/pull/16777)
 - **[cloud-provider-dvp]** migrate cloud dvp to beta2 capi [#16844](https://github.com/deckhouse/deckhouse/pull/16844)
 - **[cloud-provider-dynamix]** Fixed linter warnings in Dynamix and HuaweiCloud cloud providers [#18202](https://github.com/deckhouse/deckhouse/pull/18202)
 - **[cloud-provider-huaweicloud]** Fixed linter warnings in Dynamix and HuaweiCloud cloud providers [#18202](https://github.com/deckhouse/deckhouse/pull/18202)
 - **[cloud-provider-vcd]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[cloud-provider-zvirt]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[cloud-provider-zvirt]** use v1beta2 CAPI contract [#17991](https://github.com/deckhouse/deckhouse/pull/17991)
 - **[cni-cilium]** Added vex with CVE-2026-33726 for hubble [#18913](https://github.com/deckhouse/deckhouse/pull/18913)
 - **[cni-cilium]** Fixed vex file [#19001](https://github.com/deckhouse/deckhouse/pull/19001)
 - **[cni-cilium]** RBAC has been added for NetworkPolicies and EgressGateways. Now, you need to have the necessary permissions to use them. [#18022](https://github.com/deckhouse/deckhouse/pull/18022)
 - **[cni-cilium]** Refactor build to use pre-packaged dependencies from envoyproxy_deps repository instead of downloading from GitHub at build time [#18915](https://github.com/deckhouse/deckhouse/pull/18915)
    Cilium agents will be restarted.
 - **[cni-cilium]** metadata.labels render changed to use "helm_lib_module_labels" [#18366](https://github.com/deckhouse/deckhouse/pull/18366)
 - **[cni-cilium]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[cni-flannel]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[deckhouse-controller]** Convert MUP CRD v1alpha1 not served. [#18222](https://github.com/deckhouse/deckhouse/pull/18222)
 - **[deckhouse-controller]** Converted dashboard module to external source. [#17941](https://github.com/deckhouse/deckhouse/pull/17941)
 - **[deckhouse-controller]** Updated version of shell-operator. [#18648](https://github.com/deckhouse/deckhouse/pull/18648)
 - **[deckhouse-controller]** convert MPO CRD v1alpha1 to not served. [#18010](https://github.com/deckhouse/deckhouse/pull/18010)
 - **[deckhouse]** Add settings check. [#19116](https://github.com/deckhouse/deckhouse/pull/19116)
 - **[deckhouse]** Enable packages. [#18529](https://github.com/deckhouse/deckhouse/pull/18529)
 - **[deckhouse]** Replace config-values.yaml with settings.yaml. [#19241](https://github.com/deckhouse/deckhouse/pull/19241)
 - **[descheduler]** Grant RBAC for PersistentVolumeClaims so the descheduler can list and watch PVCs [#18787](https://github.com/deckhouse/deckhouse/pull/18787)
 - **[dhctl]** add root context propagation, starting from dhctl kingping [#19254](https://github.com/deckhouse/deckhouse/pull/19254)
 - **[docs]** Info about editions for egressgateway has been edited. [#19564](https://github.com/deckhouse/deckhouse/pull/19564)
 - **[docs]** Network ports for hostNetwork virtualization components actualization [#18139](https://github.com/deckhouse/deckhouse/pull/18139)
 - **[docs]** Upgrade Hugo to v0.161.1. [#20230](https://github.com/deckhouse/deckhouse/pull/20230)
 - **[documentation]** Fix CVEs. Upgrade Hugo to v0.161.1 in documentation builder. [#20230](https://github.com/deckhouse/deckhouse/pull/20230)
 - **[ingress-nginx]** Added unconfined seccomp profile to Ingress-NGINX controller pods. [#18840](https://github.com/deckhouse/deckhouse/pull/18840)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** An IngressNginxController API migration hook is added. [#18500](https://github.com/deckhouse/deckhouse/pull/18500)
 - **[ingress-nginx]** Missing mount points were added. [#18570](https://github.com/deckhouse/deckhouse/pull/18570)
    All Ingress-NGINX controller pods will be restarted.
 - **[ingress-nginx]** The default version of ingress-nginx has been changed to 1.12. [#18612](https://github.com/deckhouse/deckhouse/pull/18612)
    All pods of Ingress-NGINX Controllers using default version  (the controllerVersion is not set) will be restarted and updated from 1.10 to 1.12.
 - **[ingress-nginx]** The werf images are comply with DMT. [#18434](https://github.com/deckhouse/deckhouse/pull/18434)
    All Ingress-nginx controller pods will be restarted.
 - **[ingress-nginx]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[istio]** Added kubernetes v1.31-1.35 in docs supported versions. [#18447](https://github.com/deckhouse/deckhouse/pull/18447)
 - **[istio]** Changing the multi-network Istio documentation [#18591](https://github.com/deckhouse/deckhouse/pull/18591)
 - **[istio]** Correction of the Istio Federatio documentation on single and multi network [#18507](https://github.com/deckhouse/deckhouse/pull/18507)
 - **[istio]** Fix of the vex addition [#20551](https://github.com/deckhouse/deckhouse/pull/20551)
 - **[istio]** Git clone for images common-v1x21x6, common-v1x25x2, operator-v1x25x2 and proxyv2-v1x21x6 moved to git section of werf.inc.yaml [#18293](https://github.com/deckhouse/deckhouse/pull/18293)
 - **[istio]** Warning about the inability to use user 1337 for user applications [#18592](https://github.com/deckhouse/deckhouse/pull/18592)
 - **[istio]** changed group names in prometheus-rules of controlplane alerts [#18910](https://github.com/deckhouse/deckhouse/pull/18910)
 - **[istio]** changed vex CVE justifications in pilots images [#19585](https://github.com/deckhouse/deckhouse/pull/19585)
 - **[istio]** fixed discovery application namespaces test [#18847](https://github.com/deckhouse/deckhouse/pull/18847)
 - **[istio]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[istio]** replaced proxyv2-v1x25x2 and ztunnel-v1x25x2 images with distroless [#18210](https://github.com/deckhouse/deckhouse/pull/18210)
 - **[istio]** version 1.25 set as default for globalVersion [#19324](https://github.com/deckhouse/deckhouse/pull/19324)
 - **[istio]** vex justified CVE-2026-42151 and CVE-2026-42154 in pilot and operator images [#20190](https://github.com/deckhouse/deckhouse/pull/20190)
 - **[keepalived]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[kube-dns]** disabled DMT-lint for ommited oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[kube-proxy]** fixed tests of discover_api_endpoints.go [#18270](https://github.com/deckhouse/deckhouse/pull/18270)
 - **[metallb]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[monitoring-custom]** Add clarification to D8ReservedNodeLabelOrTaintFound alert description. [#16912](https://github.com/deckhouse/deckhouse/pull/16912)
 - **[monitoring-kubernetes]** Add OOM kills exporter [#16662](https://github.com/deckhouse/deckhouse/pull/16662)
 - **[multitenancy-manager]** Bumped dependencies to fix CVE's [#18829](https://github.com/deckhouse/deckhouse/pull/18829)
 - **[network-policy-engine]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[node-local-dns]** Improved D8NodeLocalDNSKubeforwardRequestLatencyP95High alert description. [#18317](https://github.com/deckhouse/deckhouse/pull/18317)
 - **[node-manager]** Fix cloud providers linter warnings. [#18650](https://github.com/deckhouse/deckhouse/pull/18650)
 - **[node-manager]** update cluster-api version in caps to v1.11.5 [#17936](https://github.com/deckhouse/deckhouse/pull/17936)
 - **[openvpn]** open source components versions migrated from werf.inc.yaml to oss.yaml [#18117](https://github.com/deckhouse/deckhouse/pull/18117)
 - **[registry]** Codeowners change [#19410](https://github.com/deckhouse/deckhouse/pull/19410)
 - **[registry]** Update dependencies to fix CVEs [#18600](https://github.com/deckhouse/deckhouse/pull/18600)
 - **[upmeter]** fix go lint warning [#17909](https://github.com/deckhouse/deckhouse/pull/17909)
    upmeter
