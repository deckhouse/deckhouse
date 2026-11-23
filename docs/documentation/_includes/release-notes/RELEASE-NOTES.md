## Version 1.78

### Key changes

Key changes in this version:

- [Deckhouse Engine](#release-1-78-engine): A type 1 hypervisor for Deckhouse Platform nodes with an immutable root file system and declarative management.
- [Bare-metal node management via BMC](#release-1-78-cloud-providers): The new [`infra-provider-baremetal`](https://deckhouse.io/modules/infra-provider-baremetal/v1.78/) module supports creating nodes on physical servers.
- [New registry implementation](#release-1-78-registry): Image sources and access keys are validated before changes are applied. New capabilities include image pull recovery when Deckhouse Platform is unavailable and a highly available in-cluster cache with seamless switchover in isolated environments.
- [Automatic selection of resource requests for control plane components](#release-1-78-control-plane): CPU and memory requests for etcd, `kube-apiserver`, `kube-controller-manager`, and `kube-scheduler` are calculated from actual consumption.
- [Reworked fencing](#release-1-78-fencing): The workload is moved from a failed node only after the node is confirmed to be stopped. This protects StatefulSet applications and pods with ReadWriteOnce volumes from running on two nodes at the same time.
- [Changing resource requests without recreating pods](#release-1-78-vpa): The default mode for new VPA objects. If resource requests cannot be changed without recreating the pod, the pod is recreated.
- [Updated role model](#release-1-78-iam): A new set of subsystems and changes to the permissions of platform managers.
- [User session management](#release-1-78-sessions): Viewing active sessions and ending individual sessions or all sessions of a user.
- [Project management](#release-1-78-projects): Structured templates and, in commercial editions, multiple namespaces in one project with a shared quota.
- [TokenExchangeProvider](#release-1-78-token-exchange): Access of external systems, such as CI/CD, to the Kubernetes API by exchanging tokens of a trusted issuer, without static credentials.
- [ClusterMesh](#release-1-78-clustermesh): Experimental support for direct pod-to-pod network communication between clusters.
- [Kubernetes 1.37 (preview)](#release-1-78-kubernetes) and [Istio 1.29](#release-1-78-istio-versions): New supported versions. Support for Kubernetes 1.32 has been discontinued. The default Kubernetes version has changed from 1.34 to 1.35.

### Updating to 1.78

Check the update requirements and prepare the components and integrations you use.
When planning the work, take the restarts and temporary limitations into account. After the update, perform the migrations and checks listed below.

Contents of the section:

- [Impact on the cluster](#release-1-78-upgrade-impact)
- [Update requirements and limitations](#release-1-78-upgrade-requirements)
- [Preparing for the update](#release-1-78-before-upgrade)
- [Checks and migrations after the update](#release-1-78-after-upgrade)

#### Impact on the cluster {#release-1-78-upgrade-impact}

When choosing the update time, take the following changes into account:

- All Deckhouse Platform components will be restarted during the update.

- Because of the switch to Topology Manager profiles, kubelet will be restarted on all nodes, and the memory available to pods (allocatable) on them will change slightly. Nodes with a policy other than `None` that run Guaranteed pods will be drained according to the `disruptions` rules of their NodeGroup. In the `Manual` mode, the drain starts only after approval. For details, see [Topology Manager profiles](#release-1-78-topology-manager).

- On nodes with containerd v1, containerd and kubelet may be restarted during the registry migration. The number of restarts depends on the initial mode and on whether `Managed` is enabled afterwards.
  Migration from `Direct` and migration on nodes with containerd v2 do not restart containerd. For details, see [restarts during the registry migration](https://deckhouse.io/modules/registry/v1.78/faq.html#does-the-migration-restart-containerd).

- Validating webhooks of the [`admission-policy-engine`](https://deckhouse.io/modules/admission-policy-engine/v1.78/), [`user-authz`](https://deckhouse.io/modules/user-authz/v1.78/), and [`user-authn`](https://deckhouse.io/modules/user-authn/v1.78/) modules will be unavailable for 3–4 minutes.
  During this time, these webhooks do not check security policies, RBAC objects (including protection against privilege escalation), users, and groups. Schedule the update for a time when these objects do not have to be changed.

- Users of applications behind DexAuthenticator will have to sign in again once. For details, see [DexAuthenticator client IDs](#release-1-78-dexauthenticator-ids).

#### Update requirements and limitations {#release-1-78-upgrade-requirements}

Check the component versions and whether the configuration is ready for the update:

- Update Deckhouse Platform to 1.78 only from version 1.77.1 or later. Version 1.77.1 introduced the check that the registry is ready for migration.

- If the cluster runs Kubernetes 1.32, switch it to Kubernetes 1.33 or later. For details, see [supported Kubernetes versions](#release-1-78-kubernetes).

- <a id="release-1-78-registry-migration"></a>If the cluster uses the old registry implementation (the `Direct`, `Proxy`, or `Local` mode in the [`registry` section of the `deckhouse` ModuleConfig](https://deckhouse.io/modules/deckhouse/v1.78/configuration.html#parameters-registry)), prepare it for handing control over to the `registry` module according to the [migration instructions](https://deckhouse.io/modules/registry/v1.78/faq.html#how-does-the-migration-to-the-registry-module-work). Do the preparation on version 1.77, before the update. The module takes control automatically after the update to 1.78. For a cluster that is not prepared, the update will be blocked. For details, see [the new registry implementation](#release-1-78-registry).

- If the cluster uses Istio versions earlier than 1.25, switch them to version 1.25 or later. For details, see [supported Istio versions](#release-1-78-istio-versions).

- If the cluster uses the [`sds-replicated-volume`](https://deckhouse.io/modules/sds-replicated-volume/) module, update it to version 0.9.5 or later.

- If the cluster uses the [`cloud-provider-dvp`](https://deckhouse.io/modules/cloud-provider-dvp/) module, make sure that it is available from a [module source (ModuleSource)](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#modulesource). Otherwise, the update is impossible. For details, see [modules from external sources](#release-1-78-external-modules).

- If the cluster was deployed on a Deckhouse Platform version earlier than 1.55, set the [`settings.podSecurityStandards.defaultPolicy`](https://deckhouse.io/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-defaultpolicy) parameter in the [`admission-policy-engine`](https://deckhouse.io/modules/admission-policy-engine/v1.78/) ModuleConfig. Without it, the update will be blocked. To keep the current behavior, specify `Privileged`. For details, see [the default Pod Security Standards policy](#release-1-78-pss-default).

- If the cluster is deployed on Basis Dynamix, plan its reinstallation. The integration supports only Basis Dynamix [version 4.6 and later](https://deckhouse.io/modules/cloud-provider-dynamix/v1.78/environment.html#platform-version). In Deckhouse Platform 1.78, when a cluster on Basis Dynamix is deleted, its resources, including the disks of persistent volumes, are deleted permanently, without being moved to the recycle bin. Save the data you need outside the infrastructure being deleted. For details, see [changes in the integration with Basis Dynamix](#release-1-78-dynamix).

#### Preparing for the update {#release-1-78-before-upgrade}

Perform the checks and actions that apply to your configuration so that components and integrations keep working after the update.

##### Network

Check firewall rules, NetworkPolicy objects, and proxy settings:

- If traffic to master nodes is restricted by a firewall, open ports `4281/TCP` and `4283/TCP` for the container registry instead of `5001` and `5003`. For details, see [container registry ports](#release-1-78-registry-ports).

- If traffic in the DexAuthenticator namespace is restricted by a NetworkPolicy, allow traffic from DexAuthenticator to the `revocation-feed` service (`user-authn-controller` pods, port `8443/TCP`) in the `d8-user-authn` namespace. For details, see [DexAuthenticator requests to `revocation-feed`](#release-1-78-revocation-feed).

- If [`proxy`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#clusterconfiguration-proxy) is set in ClusterConfiguration and the proxy has no access to the certificate authority address or to the public address of the cluster, add them to `noProxy`. For details, see [`cert-manager` operation through a proxy](#release-1-78-cert-manager-proxy).

##### Nodes

Check the NodeGroup settings and node dependencies:

- If the `master` NodeGroup has a taint with a key from [`global.modules.placement.customTolerationKeys`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-modules-placement-customtolerationkeys), remove this key before the update. For details, see [placement of registry components](#release-1-78-registry-tolerations).

- If there are static nodes in AWS or Azure (a NodeGroup with `nodeType: Static`), check whether they depend on the cloud provider settings: `ecr-credential-provider`, the EROFS kernel module, or udev rules for Azure NVMe disks. For details, see [static nodes in the cloud](#release-1-78-static-cloud-nodes).

- If master nodes have no access to NTP servers, for example in air-gapped clusters, specify available `ntpServers` or disable the check with the `earlyTimeSync.enabled: false` parameter. For details, see [time synchronization before kubelet starts](#release-1-78-early-time-sync).

- If a NodeGroup has [`kubelet.memorySwap.swapBehavior: LimitedSwap`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-memoryswap-swapbehavior) set and the nodes of the group have no cgroup v2, remove `swapBehavior: LimitedSwap` from this group. For details, see [LimitedSwap validation](#release-1-78-limited-swap).

- If fencing is enabled in the cluster, open ports `8500/TCP` and `8500/UDP` between the InternalIP addresses of all cluster nodes.
  For groups with the `Watchdog` mode, check whether the response time of the `Slow` mode, which they will switch to after the update, is acceptable. For details, see [fencing](#release-1-78-fencing).

- If a NodeGroup has the `spec.kubelet.topologyManager` parameter set, check how the switch to Topology Manager profiles affects the group:
  - To keep the previous behavior of groups with `enabled: false` and a policy other than `None`, set `policy: None`.
  - To keep CPU pinning in groups with `enabled: true` and `policy: None`, set `policy: BestEffort`.
  - For the `Restricted` and `SingleNumaNode` policies on nodes with SMT (Hyper-Threading), fix the requests of Guaranteed containers with an odd number of CPUs.

  For details, see [Topology Manager profiles](#release-1-78-topology-manager).

##### Security policies and certificates

Check the workloads, policies, and certificates that will go through the changed checks:

- If the cluster has workloads with `seLinuxOptions`, check them and create a SecurityPolicyException for them if necessary. For details, see [the change in the SELinux check](#release-1-78-selinux).

- If the cluster has constraints without `match.kinds` or with `kinds: ["*"]` that do not check `input.review.operation`, make sure that they do not deny exec and attach to the users who need these operations. For details, see [policies for `exec` and `attach`](#release-1-78-exec-attach).

- If CA certificates and a key are set in the [`istio`](https://deckhouse.io/modules/istio/v1.78/) module settings, check that they are valid. With invalid data, the module cannot apply the configuration. For details, see [the CA check in Istio](#release-1-78-istio-ca).

##### User and application access

Check user permissions and the compatibility of integrations:

- If users are assigned the `d8:system:manager`, `d8:subsystem:<SUBSYSTEM>:manager`, or deprecated `d8:manage:*:manager` roles, review how they work in module namespaces.
  After the update, these roles grant only the viewer level there: users will no longer be able to read secrets, run `exec`, `attach`, and `port-forward`, or delete and scale pods.
  Operations beyond the viewer level require the superadmin role of the subsystem. This role does not lift the restrictions on operations with protected `heritage: deckhouse` objects described in the next item. Managers of the system and of the `iam` subsystem will also no longer be able to assign the Admin, ClusterAdmin, and SuperAdmin levels through ClusterAuthorizationRule.
  For details, see [changes in manager permissions](#release-1-78-iam).

- If maintenance scenarios use `exec`, `attach`, `port-forward`, or `debug` on pods with the `heritage: deckhouse` label, or scale workloads with this label, adapt them.
  After the update, these operations are available only to system users. For details, see [access restrictions](https://deckhouse.io/modules/deckhouse/v1.78/faq.html#who-can-run-exec-into-deckhouse-platform-pods-and-scale-deckhouse-platform-objects).

- If a DexClient has the `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation, remove it where the application does not need access to the Kubernetes API (except for the `cluster-<UUID>` DexClient objects created by Deckhouse Commander). To find such DexClient objects, run the following command:

  ```bash
  d8 k get dexclients.deckhouse.io -A -o json | jq -r '.items[] | select((.metadata.annotations["dexclient.deckhouse.io/allow-access-to-kubernetes"] // "") | ascii_downcase | IN("1","t","true")) | "\(.metadata.namespace)/\(.metadata.name)"'
  ```

  For details, see [the `allow-access-to-kubernetes` annotation](#release-1-78-dexclient-annotation).

- Warn the users of applications behind DexAuthenticator that they will have to sign in again once after the update. If an application checks the `aud` claim of DexAuthenticator ID tokens, prepare it to accept the new value `dex-authenticator-<NAME>@<NAMESPACE>`. For details, see [DexAuthenticator client IDs](#release-1-78-dexauthenticator-ids).

- If integrations call `user-api`, adapt them. After the update, the API accepts only ID tokens of the Deckhouse web UI, and the password reset endpoint is removed. For details, see [changes in `user-api`](#release-1-78-user-api).

- If kubeconfig clients exchange tokens of an external provider for Dex tokens, reconfigure them. After the update, this exchange is unavailable. For details, see [token exchange in kubeconfig](#release-1-78-kubeconfig-token-exchange).

##### Projects

Check the templates, administrator permissions, and the state of projects:

- If you use a ProjectTemplate with a Helm template in `resourcesTemplate`, prepare the recovery scenarios. After the update, restoring such a template from a backup, a GitOps bootstrap, and recreation will be rejected.
  Existing templates keep working and can be edited. To restore or recreate a template, move it to structured fields and `manifests`.
  For details, see [moving a template from a Helm text](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#moving-a-template-from-a-helm-text).

- If the permissions of project administrators are extended with custom ClusterRole objects with the `user-authz.deckhouse.io/access-level` annotation, prepare a replacement for these extensions.
  When projects on the built-in `default`, `secure`, and `secure-with-dedicated-nodes` templates are switched automatically, the `d8:project:admin` role replaces the AuthorizationRule of the Admin level and does not include these extensions.
  To keep the access, use a custom role bound through ProjectRoleBinding. For details, see [switching a project from Helm templates](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#switching-a-project-from-a-helm-text).

- If you move ClusterResourceGrantPolicy manifests from v1alpha1 to v1alpha2, rename `projectSelector` to `namespaceSelector`.
  Changing only `apiVersion` changes which namespaces are selected. If the manifests stay on v1alpha1, no action is required. For details, see [compatibility of ClusterResourceGrantPolicy versions](#release-1-78-grant-policy).

- If the cluster has projects that have lost their main namespace, find them and delete the ones you do not need. Such projects will be restored during the update. To find them, run the following command:

  ```bash
  d8 k get projects.deckhouse.io,namespaces -o json | jq -r '(INDEX(.items[] | select(.kind == "Namespace"); .metadata.name)) as $ns | .items[] | select(.kind == "Project" and .metadata.labels["projects.deckhouse.io/virtual-project"] != "true" and .spec.projectTemplateName != "virtual" and .metadata.deletionTimestamp == null) | .metadata.name | select($ns[.] == null or $ns[.].metadata.deletionTimestamp != null)'
  ```

  For details, see [restoring the main namespace of a project](#release-1-78-project-namespace-restore).

##### The dhctl installer

Check the automation that uses the CLI or the gRPC API of the installer:

- If you use `dhctl bootstrap-phase abort` or call the `execute-bashible-bundle` and `exec-post-bootstrap` subcommands in automation, update the scripts. For details, see [changes in `dhctl bootstrap-phase abort`](#release-1-78-dhctl-abort).

- If you develop or maintain a `dhctl` client or an integration that reads the list of phases, update the consumers of `dhctl` phases. For details, see [the change in the set of phases](#release-1-78-dhctl-phases).

- If you set `--download-dir` manually and reuse this directory between `dhctl` versions, use a separate directory for each version or clean the old one before the update. For details, see [the `dhctl` download directory](#release-1-78-dhctl-download-dir).

##### Cloud providers

Check the conditions for creating and recreating cloud nodes:

- If the cluster is deployed in vSphere, [enable DRS](https://deckhouse.io/modules/cloud-provider-vsphere/v1.78/#vsphere-requirement) for nodes on CAPI, because new node groups are created on CAPI by default. For details, see [the switch of vSphere to CAPI](#release-1-78-vsphere-capi).

- If Yandex Cloud has node groups on CAPI with the [`preemptible: true` parameter](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/cr.html#yandexinstanceclass-v1-spec-preemptible), note that their nodes will become preemptible after the next recreation. For details, see [preemptible nodes in Yandex Cloud](#release-1-78-yandex-preemptible).

#### Checks and migrations after the update {#release-1-78-after-upgrade}

Perform the configuration migrations and check the result of the automatic changes for the components you use.

##### Configuration migrations

Move the settings of the integration with Yandex Cloud and zVirt to the [`cloud-provider-yandex`](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/configuration.html) and [`cloud-provider-zvirt`](https://deckhouse.io/modules/cloud-provider-zvirt/v1.78/) ModuleConfig according to the [migration instructions](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).
For clusters installed through [Deckhouse Commander](https://deckhouse.io/modules/commander/), the migration is not required until Deckhouse Commander 1.22. Virtual machines and disks are not recreated during the migration.
Until it is done, the [D8CloudProviderYandexCloudMigrationPending](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) and [D8CloudProviderZvirtMigrationPending](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending) alerts fire.
For details, see [the unified ModuleConfig contract](#release-1-78-cloud-moduleconfig).

##### Result checks

Check the result of the changes:

- Make sure that the `registry` module has taken control of image pull paths. The check procedure is given in the [Which implementation is my cluster running?](https://deckhouse.io/modules/registry/v1.78/faq.html#which-implementation-is-my-cluster-running) section.

- Check the warnings about Pod Security Standards violations in system namespaces (`d8-*`, `kube-*`). These namespaces are checked against the `Restricted` policy in the `warn` mode regardless of labels.
  In Deckhouse Platform 1.79, these namespaces will be switched to the `deny` mode.

- Check that the automatic switch of projects on the built-in `default`, `secure`, and `secure-with-dedicated-nodes` templates to structured fields has completed.
  Project administrators are switched from the AuthorizationRule of the Admin level to the `d8:project:admin` role (the `d8-administrators` ProjectRoleBinding), and the `all-pods` quota is replaced with `d8-project-quota`.
  Access is handed over without interruption. To keep custom access extensions, create a custom role and bind it through ProjectRoleBinding.
  If the switch has not completed within 30 minutes, the [MultitenancyManagerProjectHandoverStuck](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#multitenancy-manager-multitenancymanagerprojecthandoverstuck) alert fires.
  For details, see [switching a project from Helm templates](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#switching-a-project-from-a-helm-text).

### Deprecated and removed interfaces

The status of deprecated settings, roles, commands, and APIs, and the replacements for removed interfaces.

Still work but are deprecated:

- The Kubernetes version, network, and domain fields in ClusterConfiguration. Move the settings to the [`control-plane-manager` ModuleConfig](#release-1-78-cluster-config).
- Token exchange through DexProvider. Switch to [TokenExchangeProvider](#release-1-78-legacy-token-exchange) before the old mechanism is removed in future versions.
- Helm templates of projects in `resourcesTemplate`. Switch to [structured fields and `manifests`](#release-1-78-helm-project-templates). Creating and restoring old templates is already restricted.
- The previous names of RBACv2 subsystem roles, including `d8:manage:*`. They will be removed in 1.79. Use the [`d8:subsystem` roles](#release-1-78-rbac-subsystems) after checking the scope of permissions.
- The `Watchdog` mode in `spec.fencing.mode` of a NodeGroup. In 1.78, it works as `Slow`. After the update, replace `Watchdog` with `Slow` to keep the behavior of version 1.78. To choose another response time, use the supported modes according to the [fencing configuration instructions](https://deckhouse.io/modules/node-manager/v1.78/faq.html#how-do-i-set-up-fencing-for-a-node-group).

Accepted but ignored:

- `spec.fencing.watchdog.timeout`. After the update, remove the parameter from the NodeGroup when [configuring fencing](#release-1-78-fencing).
- `spec.kubelet.topologyManager.enabled`. Set the profile with the [`policy` parameter](#release-1-78-topology-manager).
- The `--force-abort-from-cache` parameter of the `dhctl bootstrap-phase abort` command. Check the [abort scenarios](#release-1-78-dhctl-abort).
- The `--download-cache-dir` parameter of the `dhctl` installer. Use the [`--download-dir` parameter](#release-1-78-dhctl-download-dir).

Removed or no longer supported:

- Support for Kubernetes 1.32 and Istio 1.21 has been discontinued. Switch the components to supported versions according to the [update requirements](#release-1-78-upgrade-requirements).
- The previous registry implementation in the `deckhouse` ModuleConfig has been removed. Do the [preparation on 1.77](#release-1-78-registry-migration). After the update, control is handed over automatically.
- The `dhctl execute-bashible-bundle` and `dhctl exec-post-bootstrap` commands have been removed. Adapt the automation to the [installer changes](#release-1-78-dhctl-abort).
- The password reset endpoint in `user-api` has been removed. Check the integrations against the [changes in `user-api`](#release-1-78-user-api).
- Exchange of external provider tokens through kubeconfig clients is unavailable. Adapt the integrations to the [changes in access to the Kubernetes API](#release-1-78-kubeconfig-token-exchange).

### Changes by area

Contents of the section:

- [Infrastructure and nodes](#release-1-78-infrastructure)
- [Kubernetes, workload placement and scaling](#release-1-78-kubernetes-scheduling)
- [Users, access, and projects](#release-1-78-access-projects)
- [Security and certificates](#release-1-78-security)
- [Network and cross-cluster communication](#release-1-78-network)
- [Platform and module management](#release-1-78-platform)

#### Infrastructure and nodes {#release-1-78-infrastructure}

Changes in cluster installation, infrastructure integrations, and node management.

##### Nodes on Deckhouse Engine {#release-1-78-engine}

Deckhouse Engine is a type 1 hypervisor for Deckhouse Platform nodes with an immutable root file system. Deckhouse Engine nodes are combined into groups in the same way as other cluster nodes and work in one cluster with groups on regular Linux distributions (Generic OS). The node management method is set for each NodeGroup separately. A node is managed by the `nodelet` agent: it brings the node to the state described in [NodeConfig](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodeconfig) and reports the result in its status.

The state of such a node cannot be changed manually bypassing NodeConfig, so the configuration of the nodes of a group does not drift over time. The absence of a shell, an SSH server, and a package manager reduces the attack surface, and a failed operating system update is rolled back automatically.

Differences between Deckhouse Engine nodes and Generic OS nodes:

- The root file system is read-only. Its integrity is checked by dm-verity, and the signature of the root hash of the image is checked by the kernel. A node does not apply an image with an invalid signature.
- The operating system has no shell, SSH server, or package manager. The node state is defined by the operating system image and NodeConfig, so the nodes of one group are configured identically.
- A node is configured declaratively; SSH access and bashible are not used for this. The `nodelet` node agent brings the node to the state described in NodeConfig: network, disks, kernel parameters and modules, extensions, and static pods.
- Node components, such as containerd and kubelet, are delivered as signed system extensions (systemd-sysext). Additional extensions, for example with kernel modules, are installed with NodeExtensionRequest.
- The operating system is updated as a whole. A node pulls a new image from the container registry and reboots according to the update rules of its NodeGroup. If booting with the new image fails, the node automatically returns to the previous image.

For details about Deckhouse Engine, see the [Deckhouse Platform documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/deckhouse-engine.html).

##### Node resources

Changes in the accounting and reservation of node resources:

- Node capacity is updated automatically after CPU and memory hot-plug. Every 5 minutes, kubelet compares the node resources with the ones it had at startup. If at least one CPU or at least 100 MiB of memory has been added, kubelet restarts itself. Previously, kubelet read the CPU and memory of a node only at startup, so after resources were hot-plugged into a virtual machine, the node capacity did not change until kubelet was restarted manually.

- In the `Auto` mode of the [`spec.kubelet.resourceReservation`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-resourcereservation) parameter, the CPU reservation for the system is calculated from the number of node cores instead of the fixed `70m`. The reservation is 6% of the first core, 1% of the second, 0.5% each of the third and fourth, and 0.25% of each subsequent core. The new value is applied only if the pods of the node fit into the new amount of allocatable resources.

- <a id="release-1-78-topology-manager"></a>**Topology Manager profiles and per-node calculation of kubelet NUMA settings.** The [`spec.kubelet.topologyManager.policy`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-topologymanager-policy) parameter of a NodeGroup sets the full kubelet profile: CPU Manager, Memory Manager, and Topology Manager. The [`enabled`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-topologymanager-enabled) parameter is deprecated and ignored. NUMA-dependent kubelet settings are calculated on each node separately. If the CPU Manager or Memory Manager checkpoint is outdated, for example after the CPU or memory of the node has changed, kubelet resets it and starts without stopping the running containers. Containers whose memory may have lost its binding to a NUMA node are shown by the NUMACheckpointReset event and the [D8KubeletCheckpointStateUnpinnedContainers](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#node-manager-d8kubeletcheckpointstateunpinnedcontainers) alert. The binding is restored after the pod is recreated.

  The switch to Topology Manager profiles changes the behavior of node groups:
  - In groups with `enabled: false` (set explicitly or by default) and a policy other than `None`, the policy takes effect: CPU and memory of Guaranteed pods are pinned to NUMA nodes. kubelet starts rejecting pods that cannot be placed. With any policy, a pod is rejected with `UnexpectedAdmissionError` if the memory does not fit. With `Restricted` and `SingleNumaNode`, a pod is also rejected with `TopologyAffinityError` if the CPUs cannot be aligned. The scheduler does not take NUMA nodes into account, so a recreated pod can get to the same node again. To keep the previous behavior, set `policy: None`.
  - Groups with `enabled: true` and `policy: None` stop pinning CPUs. To keep the pinning, set `policy: BestEffort`.
  - For the `Restricted` and `SingleNumaNode` policies, the full-pcpus-only option of CPU Manager is enabled. A container gets only whole physical cores, so on nodes with SMT (Hyper-Threading), Guaranteed containers with an odd number of CPUs are rejected with `SMTAlignmentError`.

  For details, see the [How do I give pods dedicated CPUs and NUMA alignment?](https://deckhouse.io/modules/node-manager/v1.78/faq.html#how-do-i-give-pods-dedicated-cpus-and-numa-alignment) section.

- <a id="release-1-78-limited-swap"></a>Validation of [`kubelet.memorySwap.swapBehavior: LimitedSwap`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-kubelet-memoryswap-swapbehavior) in NodeGroup has been fixed. The validation requires cgroup v2. A NodeGroup created with `LimitedSwap` on nodes without cgroup v2 can get stuck on deletion, because the update and the removal of the finalizer are rejected.

##### Node maintenance and fault tolerance

Changes in fencing, drain, and node maintenance:

- <a id="release-1-78-fencing"></a>The reworked [fencing](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/glossary.html#fencing) moves the workload from a failed node only after the node is confirmed to be stopped.
  This protects StatefulSet applications and pods with ReadWriteOnce volumes from the old and the new instance running on different nodes at the same time.
  Previously, pods were deleted 60 seconds after the last Lease update, without confirmation that the node had stopped.

  The `Critical`, `Normal`, and `Slow` modes let you choose the failure response speed for each NodeGroup with the [`spec.fencing.mode`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-fencing-mode) parameter.
  The target fencing response time is about 12, 30, and 60 seconds respectively, with a tolerance of ±20%.
  A faster response increases the risk of stopping a node by mistake when the network or the control plane is delayed. The `Critical` mode is available only in commercial editions.

  Fencing is disabled by default. It requires ports `8500/TCP` and `8500/UDP` to be open between the InternalIP addresses of all cluster nodes.
  The `Watchdog` mode is deprecated and works as `Slow`, and the [`spec.fencing.watchdog.timeout`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-fencing-watchdog-timeout) parameter is ignored.
  For details, see the [How do I set up fencing for a node group?](https://deckhouse.io/modules/node-manager/v1.78/faq.html#how-do-i-set-up-fencing-for-a-node-group) section.

- In a cluster with one master node, fencing cannot be enabled for an existing NodeGroup. When a NodeGroup with fencing is created, Deckhouse Platform shows a warning. For details, see the [Fencing in a cluster with one master node](https://deckhouse.io/modules/node-manager/v1.78/faq.html#fencing-in-a-cluster-with-one-master-node) section.

- **Correct drain status.** A drain error or timeout is no longer recorded as successful completion. The node keeps the draining annotation, gets `update.node.deckhouse.io/drain-failed` with the error text, and retries. The NodeGroup update stops until the drain succeeds, and NodeStuckInDraining fires after five minutes. Removing the draining annotation cancels the pod eviction and returns the node to the scheduler. `status.eviction` of NodeOperation shows which pods the operation is still waiting for.

- **Protection of NodeGroup and CAPI objects.** The master group cannot be deleted, and if it is deleted bypassing the check, it is restored with the previous parameters, labels, and annotations. Deleting the CAPI Cluster in `d8-cloud-instance-manager` is forbidden.

- `node-controller` keeps the taints of other components when it processes Static and CloudPermanent nodes for the first time.

- When kubelet is updated, static pods are restarted only if their manifest has changed. Previously, kubelet forcibly terminated all static pods of a node even if the manifest had not changed.

- **Diagnostics of bashible errors.** The [Instance](https://deckhouse.io/modules/node-manager/v1.78/cr.html#instance) message shows the stderr of the failed step instead of the general exit code of the shell. The cause of an error can be found without connecting to the node over SSH.

- An interrupted bootstrap of a static node continues from the saved state.

- <a id="release-1-78-early-time-sync"></a>**Time synchronization before kubelet starts.** On master nodes, `nodelet` checks the time over NTP at every boot and corrects it before kubelet starts. The `earlyTimeSync.enabled` setting is enabled by default. If NTP servers are unavailable, the boot of a master node can take an additional 30 seconds. The default timeout is 30 seconds; on bashible nodes, it can be increased to 60 seconds with the `earlyTimeSync.timeoutSeconds` parameter. After a failed check, kubelet starts, and the [NodeTimeNotSynchronizedAtBoot](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#chrony-nodetimenotsynchronizedatboot) alert reports the problem. For details, see the [Boot-time synchronization on master nodes](https://deckhouse.io/modules/chrony/v1.78/#boot-time-synchronization-on-master-nodes) section.

##### Cluster installation

Changes in the `dhctl` installer and the installation process:

- **Declarative phases and operation of the CLI installer (`dhctl`) without SSH.** The `--skip-phase` parameter skips the specified bootstrap phases, for example when an interrupted installation is restarted. gRPC operations accept a kubeconfig and can run without available master nodes and without an SSH connection. The bootstrap, destroy, and converge phases are described by a single tree, so they run and are displayed identically in the CLI and gRPC. Separate state caches for static and managed clusters and a configuration without ClusterConfiguration are available.

- <a id="release-1-78-dhctl-phases"></a>The set of phases that `dhctl` declares has changed in all operations. `FirstMaster` and `WaitForControlPlaneManagerReadiness` have become top-level phases. The format of gRPC messages has not changed, but clients with a fixed list or number of phases have to be updated.

- <a id="release-1-78-dhctl-abort"></a>The behavior of `dhctl bootstrap-phase abort` has changed. The command rolls back the installation only from the state cache and does not delete the resources of a running cluster. To delete a cluster, use `dhctl destroy`. The `--force-abort-from-cache` parameter is accepted and ignored. The `execute-bashible-bundle` and `exec-post-bootstrap` commands have been removed. When aborting a static cluster, pass the same set of `--ssh-host` values as for the bootstrap.

- <a id="release-1-78-dhctl-download-dir"></a>Do not use a shared download directory for different `dhctl` versions. `dhctl` unpacks candi, OpenTofu, terraform-manager, and the provider bundle into directories whose names contain the image digest. The `--download-dir` parameter applies to all commands, and `--download-cache-dir` is accepted and ignored.

- **User SSH certificates in `dhctl`.** For each key from `--ssh-agent-private-keys`, `dhctl` reads the `<KEY>-cert.pub` certificate from the same directory. SSH CA authentication works both for target servers and for the bastion. The regular key is offered first, then its certificate and the other keys from `ssh-agent`.

- NodeGroup and InstanceClass objects for CloudPermanent nodes are applied before the nodes are created, so an interrupted installation no longer leaves groups with zero replicas. The provider network is configured before bootstrap packages are installed, which makes it possible to download them from a master node in another zone.

- Before running `rpp-get`, bashible checks the binary against the image digest from the bundle. `bashible-apiserver` accepts only TLS 1.2 and later with modern ciphers.

##### Cloud and infrastructure providers {#release-1-78-cloud-providers}

Changes in the configuration and modules of cloud and infrastructure providers:

- **Cloud resource prefix in the `global` ModuleConfig.** The `cloud.prefix` parameter of ClusterConfiguration has been moved to [`spec.settings.prefix`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-prefix). A one-time automatic migration is performed during the update. After it, cloud-controller-manager, the names of CAPI/MCM MachineDeployment objects, and cluster-autoscaler priorities read the prefix from the ModuleConfig. Changing the prefix in a running cluster is forbidden, because it would cause nodes to be recreated and the underlying infrastructure to be renamed.

- **Provider selection at the NodeGroup level.** The [`spec.providerType`](https://deckhouse.io/modules/node-manager/v1.78/cr.html#nodegroup-v1-spec-providertype) parameter defines the provider type for each node group. `bashible-apiserver` already selects the provider configuration by NodeGroup. Groups with `nodeType: Static` no longer get cloud provider configuration steps.

- <a id="release-1-78-static-cloud-nodes"></a>For static nodes (a NodeGroup with `nodeType: Static`) located in AWS and Azure, some of the cloud provider settings are no longer applied:
  - AWS: `ecr-credential-provider` is not installed and is not added to the kubelet settings. If it was the only authentication method, nodes that are already configured will stop pulling private images from Amazon ECR. On Ubuntu, the EROFS kernel module is also no longer installed if it is needed for containerd v2.
  - Azure: udev rules for discovering Azure NVMe disks are not installed. Rules that are already installed are not removed.

- <a id="release-1-78-cloud-moduleconfig"></a>The configuration of the [`cloud-provider-yandex`](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/) and [`cloud-provider-zvirt`](https://deckhouse.io/modules/cloud-provider-zvirt/v1.78/) modules has moved to the unified ModuleConfig contract. Nodes of all types are configured in the same way through InstanceClass and NodeGroup, and a separate ProviderClusterConfiguration is not needed. For details, see the [How to migrate a cloud provider to ModuleConfig-based configuration?](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat) section.

- Changes in the [`cloud-provider-yandex`](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/) module:
  - The [disk block size](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/configuration.html#parameters-storage-parameters-provisionedstorageclasses) is set in StorageClass, which makes it possible to create disks of up to 256 TiB.
  - The size of network-ssd-nonreplicated and network-ssd-io-m3 disks is checked in advance, and an error is detected before the infrastructure is created.
  - The `ru-central1-f` availability zone is supported. To use the zone, specify `ru-central1-f` in the [`zones`](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/configuration.html#parameters-nodes-parameters-zones) parameter instead of one of the default zones. The configuration procedure is described in the [Using the ru-central1-f zone](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/examples.html#using-the-ru-central1-f-zone) section.
  - <a id="release-1-78-yandex-preemptible"></a>Existing nodes of groups on CAPI in Yandex Cloud with the [`preemptible: true` parameter](https://deckhouse.io/modules/cloud-provider-yandex/v1.78/cr.html#yandexinstanceclass-v1-spec-preemptible) will become preemptible after the [next recreation of the group nodes](https://deckhouse.io/modules/node-manager/v1.78/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration). Previously, such nodes were created as regular virtual machines.

- In the [`cloud-provider-zvirt`](https://deckhouse.io/modules/cloud-provider-zvirt/v1.78/) module, the [`storageDomainID`](https://deckhouse.io/modules/cloud-provider-zvirt/v1.78/cr.html#zvirtinstanceclass-v1-spec-storagedomainid) and [`etcdDiskSizeGb`](https://deckhouse.io/modules/cloud-provider-zvirt/v1.78/cr.html#zvirtinstanceclass-v1-spec-etcddisksizegb) parameters are available in ZvirtInstanceClass. They let you choose the storage domain and the size of the etcd disk on master nodes.

- Changes in the [`cloud-provider-vsphere`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.78/) module:
  - <a id="release-1-78-vsphere-capi"></a>The vSphere provider is based on Cluster API (CAPI) instead of Machine Controller Manager (MCM). Existing node groups stay on MCM, and new ones are created on CAPI. For nodes on CAPI in a vSphere cluster, [enable DRS](https://deckhouse.io/modules/cloud-provider-vsphere/v1.78/#vsphere-requirement). The procedure for moving an existing group is given in the [How to migrate node groups to Cluster API (CAPI)?](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/faq.html#how-to-migrate-node-groups-to-cluster-api-capi) section.
  - The certificate of a corporate certificate authority for vCenter and NSX-T is set with the [`provider.caBundle`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.78/cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle) and [`nsxt.caBundle`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.78/cluster_configuration.html#vsphereclusterconfiguration-nsxt-cabundle) parameters. Therefore, TLS certificate verification does not have to be disabled.

- Changes in the [`cloud-provider-openstack`](https://deckhouse.io/modules/cloud-provider-openstack/v1.78/) module:
  - A pre-created Octavia load balancer or a floating IP can be used for Ingress. This makes the Ingress address predictable. For details, see the [How do I set up LoadBalancer?](https://deckhouse.io/modules/cloud-provider-openstack/v1.78/faq.html#how-do-i-set-up-loadbalancer) section.
  - Support for [preemptible nodes](https://deckhouse.io/modules/cloud-provider-openstack/v1.78/cr.html#openstackinstanceclass-v1-spec-preemptible) in the Selectel cloud for node groups on Cluster API.
  - For the SimpleWithInternalNetwork layout, the internal subnet can be specified [by ID](https://deckhouse.io/modules/cloud-provider-openstack/v1.78/cluster_configuration.html#openstackclusterconfiguration-simplewithinternalnetwork-internalsubnetid), for example if the subnet has no name.
  - Specifying `tenantName` and `tenantID` at the same time is forbidden.
  - In OpenStack clusters with the [SimpleWithInternalNetwork](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/admin/integrations/private/openstack/configuration-and-layout-scheme.html#simplewithinternalnetwork) layout, the first run of `dhctl converge` after the update shows a change in the internal subnet lookup parameters in the plan. Nodes are not recreated.

- <a id="release-1-78-dynamix"></a>Changes in the [`cloud-provider-dynamix`](https://deckhouse.io/modules/cloud-provider-dynamix/v1.78/) module:
  - The integration supports only Basis Dynamix [version 4.6 and later](https://deckhouse.io/modules/cloud-provider-dynamix/v1.78/environment.html#platform-version). Clusters on Basis Dynamix created in previous Deckhouse Platform versions have to be reinstalled.
  - When a cluster on Basis Dynamix is deleted, its resources, including the disks of persistent volumes, are deleted permanently, without being moved to the recycle bin.
  - Disks are placed according to the Basis Dynamix [storage policy](https://deckhouse.io/modules/cloud-provider-dynamix/v1.78/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy), which can be overridden for an individual node group. StorageClass objects are created for each available storage policy. The cluster account requires [access to the storage_policy API group](https://deckhouse.io/modules/cloud-provider-dynamix/v1.78/environment.html#account-permissions).
  - For the Standard layout, DNS servers from the [`nameservers` parameter](https://deckhouse.io/modules/cloud-provider-dynamix/v1.78/cluster_configuration.html#dynamixclusterconfiguration-nameservers) are configured on all nodes.
  - Basis Dynamix objects are looked up by exact name match, which rules out choosing the wrong object when names are similar. The `imageName` and `externalNetwork` values must be specified in full.
  - Synchronization of the load balancer with nodes, deletion of LoadBalancer services, and SSH access to nodes connected to both ViNS and an external network have been fixed.

- Changes in the [`cloud-provider-aws`](https://deckhouse.io/modules/cloud-provider-aws/v1.78/) module:
  - Mandatory use of IMDSv2 on all EC2 instances of the cluster is enabled with the [`imdsv2` parameter](https://deckhouse.io/modules/cloud-provider-aws/v1.78/cluster_configuration.html#awsclusterconfiguration-imdsv2). This protects instance metadata from SSRF attacks. After it is enabled, the metadata service becomes unavailable to pods that do not use the host network.
  - terraform-provider-aws has been updated to version 6.69.1, which adds support for new AWS regions, for example ap-southeast-6.

- In the [`cloud-provider-huaweicloud`](https://deckhouse.io/modules/cloud-provider-huaweicloud/v1.78/) module, a single Huawei Cloud API error no longer stops the scaling of node groups.

- The new [`infra-provider-baremetal`](https://deckhouse.io/modules/infra-provider-baremetal/v1.78/) module manages cluster nodes on bare-metal servers via BMC.

- In the modules of the AWS, Azure, GCP, Yandex Cloud, OpenStack, VCD, and vSphere cloud providers, the [`csi-node`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/architecture/storage/csi-drivers/csi-driver.html#driver-components) DaemonSet uses a separate ServiceAccount to access the Kubernetes API. This eliminates authorization errors of CSI drivers when they read node information.

- A hang of cluster installation in OpenStack and VCD at the stage of starting CAPI controllers has been fixed.

#### Kubernetes, workload placement and scaling {#release-1-78-kubernetes-scheduling}

Changes in control plane operation, pod placement, and management of pod resources.

##### Control plane operation and update {#release-1-78-control-plane}

Changes in Kubernetes versions and in the resources and update of control plane components:

- <a id="release-1-78-kubernetes"></a>Kubernetes 1.37 is supported in the preview status. Support for Kubernetes 1.32 has been discontinued. The default version has changed from 1.34 to 1.35 (the `Default` value of the [`kubernetesVersion`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-kubernetesversion) parameter). Supported versions are listed in the [Supported Kubernetes and OS versions](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/supported_versions.html) section. Version management is described in the [Version control](https://deckhouse.io/modules/control-plane-manager/v1.78/#version-control) section.

- **Automatic selection of resource requests for control plane components.** CPU and memory requests for etcd, `kube-apiserver`, `kube-controller-manager`, and `kube-scheduler` are selected from the actual consumption over 7 days with a 10% margin and are recalculated once a day. The scheduler takes the real control plane load on master nodes into account, and `resourcesRequests` does not have to be tuned manually. If the new requests do not fit on the smallest master node, they are not applied, and the [D8ControlPlaneComponentsResourcesInsufficientCapacity](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#control-plane-manager-d8controlplanecomponentsresourcesinsufficientcapacity) alert fires. If [`resourcesRequests.cpu`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-resourcesrequests-cpu) or [`resourcesRequests.memory`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-resourcesrequests-memory) is set in the `control-plane-manager` ModuleConfig, automatic selection for that resource is disabled. For details, see the [Configuring Control Plane Resource Requests](https://deckhouse.io/modules/control-plane-manager/v1.78/#configuring-control-plane-resource-requests) section.

- **Smooth control plane update.** `kube-apiserver` instances are updated one by one, so the majority of master nodes serve requests at all times. A `kube-apiserver` that is stopping has time to hand its clients over to other master nodes, and API requests are not interrupted. `kube-controller-manager` and `kube-scheduler` connect to the API through `kubernetes-api-proxy`, so a restart of `kube-apiserver` on their master node no longer causes their leaders to change. The leaders of etcd, `kube-controller-manager`, and `kube-scheduler` change once per update and move straight to an updated instance. In clusters with an etcd arbiter, a master node becomes the leader, and the arbiter does not.

- etcd backups on master nodes are created in turn, with a 15-minute shift. Previously, backups were created on all master nodes at the same time, and the disk load could cause lease loss and restarts of `kube-controller-manager` and `kube-scheduler`. The most recent backup is stored on the master node with the lowest name, for example master-0. For details, see the [etcd backup and restore](https://deckhouse.io/modules/control-plane-manager/v1.78/faq.html#etcd-backup-and-restore) section.

- **Health checks and shutdown of `kubernetes-api-proxy`.** Clients that access the Kubernetes API through `kubernetes-api-proxy` switch faster to a working `kube-apiserver` when one restarts or hangs. Slow but working `kube-apiserver` instances stay in rotation. Stopping, restarting, and hung instances stop receiving new connections, and hung client connections are closed so that they reconnect to a working instance. If no `kube-apiserver` is ready, the proxy checks each instance every 500 ms for 30 seconds. The state is available at `/upstreams`, and health checks are available at `127.0.0.1:6480` by default. After SIGTERM, the proxy keeps serving `127.0.0.1:6445` for 3 more seconds. When a master node is stopped, kubelet gives node-critical pods 25 seconds to terminate instead of 5 seconds.

- Changes in auditing (the [`control-plane-manager`](https://deckhouse.io/modules/control-plane-manager/v1.78/) module):
  - The basic audit policy records pods/exec in all namespaces again, as it did before Deckhouse Platform 1.77.
  - The basic audit policy records pod status updates from kubelet with the request body, so container terminations and their reasons get into the audit log.

- <a id="release-1-78-cluster-config"></a>The following cluster parameters have been moved from ClusterConfiguration to the [`control-plane-manager` ModuleConfig](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-kubernetesversion):
  - Kubernetes version. It is set in [`settings.kubernetesVersion`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-kubernetesversion): a specific version or `Default`. The `Automatic` value is allowed only in the deprecated ClusterConfiguration field.
  - Pod address space ([`podSubnetCIDR`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-network-podsubnetcidr)).
  - Address space of cluster services ([`serviceSubnetCIDR`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-network-servicesubnetcidr)).
  - Prefix size of the pod subnet allocated to each node ([`podSubnetNodeCIDRPrefix`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-network-podsubnetnodecidrprefix)).
  - Cluster domain ([`clusterDomain`](https://deckhouse.io/modules/control-plane-manager/v1.78/configuration.html#parameters-network-clusterdomain)).

  The corresponding ClusterConfiguration fields are deprecated but keep working. While they are filled in, the [D8ObsoleteNetworkFieldsInClusterConfiguration](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#control-plane-manager-d8obsoletenetworkfieldsinclusterconfiguration) and [D8ObsoleteClusterDomainInClusterConfiguration](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#control-plane-manager-d8obsoleteclusterdomaininclusterconfiguration) alerts fire and provide migration instructions. Move the parameters to the `control-plane-manager` ModuleConfig according to these instructions. After the cluster is installed, validation does not allow changing the network parameters and the cluster domain. The network parameters can be changed only with the `deckhouse.io/allow-unsafe=true` annotation.

##### Pod scheduling {#release-1-78-scheduling}

Changes in the `kube-scheduler` scheduler in the [`control-plane-manager`](https://deckhouse.io/modules/control-plane-manager/v1.78/) module:

- **Simulation of pod placement in `kube-scheduler`.** A `POST /scheduler/simulate` request to the `control-plane-proxy` Service shows on which nodes a pod or a virtual machine can be placed and why the other nodes are not suitable. Access is granted by the `d8:control-plane-manager:scheduler-simulate` ClusterRole. For details, see [pod placement simulation](https://deckhouse.io/modules/control-plane-manager/v1.78/faq.html).

- `kube-scheduler` metrics and the new "Scheduling" dashboard show requests to extenders and node selection decisions.

- **Suspension of an unavailable extender.** If 10 requests in a row to an extender with `failurePolicy: Ignore` have failed, `kube-scheduler` suspends this extender for 30 seconds, and the workload is placed without it.

- The [`sendPlacements`](https://deckhouse.io/modules/control-plane-manager/v1.78/cr.html#kubeschedulerwebhookconfiguration-v1alpha1-webhooks-sendplacements) parameter of KubeSchedulerWebhookConfiguration passes to the extender the pods that `kube-scheduler` has just placed. For details, see [extending the scheduler logic](https://deckhouse.io/modules/control-plane-manager/v1.78/faq.html#how-to-change-or-extend-the-scheduler-logic).

##### Pod rescheduling

Changes in the [`descheduler`](https://deckhouse.io/modules/descheduler/v1.78/) module:

- **Check of the target node before eviction.** The LowNodeUtilization strategy uses [placement simulation](#release-1-78-scheduling) and evicts a workload only if `kube-scheduler` can place it on an underutilized node. For details, see [the eviction target check](https://deckhouse.io/modules/descheduler/v1.78/#eviction-target-check).

- **Node load estimation by the average CPU utilization over 15 minutes from Prometheus.** To use it, set `metricsSource: Prometheus`. For details, see [CPU utilization over 15 minutes](https://deckhouse.io/modules/descheduler/v1.78/#cpu-utilization-averaged-over-15-minutes).

- **Balancing relative to the average cluster load.** The [`useDeviationThresholds`](https://deckhouse.io/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-usedeviationthresholds) parameter of the [LowNodeUtilization](https://deckhouse.io/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization) strategy sets the `thresholds` and `targetThresholds` as a deviation from the average load of cluster nodes. The [`evictionLimits.node`](https://deckhouse.io/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-evictionlimits-node) parameter limits the number of pods evicted from one node. With [`metricsSource: Prometheus`](https://deckhouse.io/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization-metricssource), it sets how many pods to evict from a node per cycle. If the parameter is not set, one pod is evicted.

- **Protection of pods with local disks from eviction.** Pods with a PersistentVolumeClaim can be protected from eviction by listing their StorageClass in [`spec.protectedStorageClasses`](https://deckhouse.io/modules/descheduler/v1.78/cr.html#descheduler-v1alpha2-spec-protectedstorageclasses).

- The [`logLevel`](https://deckhouse.io/modules/descheduler/v1.78/configuration.html#parameters-loglevel) parameter sets the logging level.

##### Vertical scaling {#release-1-78-vpa}

Changes in the [`vertical-pod-autoscaler`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.78/) module:

- By default, VPA changes pod resource requests without a restart. If the [`updateMode` parameter](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.78/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode) is not set, VerticalPodAutoscaler objects get the `InPlaceOrRecreate` mode instead of `Recreate` when they are created. In this mode, VPA changes pod resource requests without recreation, including for applications with one replica, and recreates a pod only if the requests could not be changed this way. Existing VPA objects keep their mode and are not changed.

- **Factor for VPA recommendations.** The [`recommendationFactor`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.78/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies-recommendationfactor) parameter in [`spec.resourcePolicy.containerPolicies[]`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.78/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies) of VerticalPodAutoscaler multiplies the VPA recommendations for CPU and memory by the specified factor (no more than 10).

- The [`logLevel`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.78/configuration.html#parameters-loglevel) parameter sets the VPA logging level.

#### Users, access, and projects {#release-1-78-access-projects}

Changes in access permissions, authentication of users and applications, and project management.

##### Authorization {#release-1-78-iam}

Changes in the role model and access permissions:

- <a id="release-1-78-rbac-subsystems"></a>A new list of [subsystems of the role-based model](https://deckhouse.io/modules/user-authz/v1.78/#subsystems-of-the-role-based-model) in RBACv2. Subsystem roles have the form `d8:subsystem:<SUBSYSTEM>:<LEVEL>`. The subsystems are `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability`, and `managed-services`. The `deckhouse`, `infrastructure`, and `kubernetes` subsystems have been merged into `cluster`, and `networking` has been renamed to `network`. The [`user-authn`](https://deckhouse.io/modules/user-authn/v1.78/), [`user-authz`](https://deckhouse.io/modules/user-authz/v1.78/), and [`multitenancy-manager`](https://deckhouse.io/modules/multitenancy-manager/v1.78/) modules have been moved to `iam`. The manager of the `iam` subsystem also assigns roles. The deprecated `d8:manage:*` roles keep working but will be removed in 1.79. For details, see the [Deprecated role names](https://deckhouse.io/modules/user-authz/v1.78/#deprecated-role-names) section. The `d8:subsystem:cluster:*` replacement role grants more permissions than the previous roles, so before recreating the bindings, check which permissions the subjects need. Custom capabilities and modules with old aggregation labels trigger the [D8UserAuthzRetiredAggregationLabel](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#user-authz-d8userauthzretiredaggregationlabel) and [D8UserAuthzLegacyUseCapability](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#user-authz-d8userauthzlegacyusecapability) alerts.

- The permissions of system and subsystem managers have been reduced:
  - Users with the `d8:system:manager`, `d8:subsystem:<SUBSYSTEM>:manager`, and deprecated `d8:manage:*:manager` roles get the viewer level in the namespaces of their modules. They can no longer read secrets, run exec, attach, and port-forward, or delete and scale pods. Operations above the viewer level require the superadmin role of the subsystem. It does not lift the [restrictions on operations with protected platform objects](https://deckhouse.io/modules/deckhouse/v1.78/faq.html#who-can-run-exec-into-deckhouse-platform-pods-and-scale-deckhouse-platform-objects), including pods with the `heritage: deckhouse` label.
  - Custom system and subsystem roles are projected into module namespaces at the viewer level at most. The roles affected by this are shown by the `d8_user_authz_projection_capped` metric.
  - The `d8:system:manager` and `d8:subsystem:iam:manager` managers assign roles up to the manager and ClusterEditor levels. They cannot create ClusterAuthorizationRule objects of the Admin, ClusterAdmin, and SuperAdmin levels or ones that open access to system namespaces. Existing rules are kept. Such changes are made by a ClusterAdmin or SuperAdmin.

- Listing objects with the `d8 k get` command and the `--scope` parameter supports aggregated APIs that declare this with the `scope.deckhouse.io/resources` annotation on APIService, as well as the `roles` and `rolebindings` resources.

- The `d8:namespace:*` and `d8:project:*` roles grant access to namespaced Gateway API resources: read access from the viewer level and write access from the manager level.

- On master nodes (host network), ports `4233/TCP` (metrics of the authorization webhook) and `4243/TCP` (the `kube-rbac-proxy` upstream on `127.0.0.1`, no firewall rule is required) are in use.

- Only the Deckhouse Commander agent can create and change ClusterRole objects with the `d8:commander:` prefix.

- **Hiding sensitive fields in the API.** Passwords and auth in RegistryConfig and RegistryUpstream, as well as `spec.passwordHash` of NodeUser, are hidden from users without access to the `/sensitive` subresources. For other readers, the API returns `<omitted>`, and the values are also masked in the audit.

##### Users and sessions {#release-1-78-sessions}

Changes in the management of users and their sessions:

- **User session management.** Locking, a password or 2FA reset, deletion of a user, and the new Logout operation end the user's sessions in Dex, DexAuthenticator, and the Kubernetes API. Basic Authentication to the Kubernetes API through LDAP checks the password with Dex on each request, so `Logout` does not end it. For this, the `Logout` and `RequirePasswordChange` types are available in [UserOperation](https://deckhouse.io/modules/user-authn/v1.78/cr.html#useroperation). In the multitenancy mode on Kubernetes 1.34 and later, the API server stops accepting revoked ID tokens within several seconds.

- **User devices.** [UserAccount.status.sessions](https://deckhouse.io/modules/user-authn/v1.78/cr.html#useraccount-v1alpha1-status-sessions) shows active sessions. The `Logout` operation ends one session or all sessions except the current one.

- **Dex browser sessions.** The [`sessions`](https://deckhouse.io/modules/user-authn/v1.78/configuration.html#parameters-sessions) parameters enable single sign-on to all web interfaces of the platform, sign-out from the browser session in all DexAuthenticator instances, and back-channel logout. Browser sessions are disabled by default.

- The [`postLogoutRedirectURIs`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#dexclient-v1-spec-postlogoutredirecturis), [`backchannelLogoutURI`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#dexclient-v1-spec-backchannellogouturi), and [`refreshTokenLifetime`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#dexclient-v1-spec-refreshtokenlifetime) parameters are available in DexClient, and [`upstreamLogout`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#dexprovider-v1-spec-oidc-upstreamlogout) is available in the OIDC DexProvider.

- Locking ([Lock](https://deckhouse.io/modules/user-authn/v1.78/cr.html#useroperation-v1-spec-lock) with [`spec.target`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#useroperation-v1-spec-target)) works for users of any provider, including OIDC, SAML, GitHub, GitLab, and Bitbucket Cloud. Signing in through a browser no longer removes a lock set by an administrator.

- 2FA secrets and locks of Dex users are stored in UserIdentity. UserAccount is maintained for users of all providers.

- The Lock and ResetPassword operations no longer reset the 2FA secret. To make a user register the authenticator app again, additionally [run Reset2FA (d8 iam user reset2fa)](https://deckhouse.io/modules/user-authn/v1.78/usage.html#the-second-factor-of-an-account).

- Manual deletion of UserIdentity is forbidden. To delete a local user, delete the User, and to reset 2FA or a lock, use UserOperation. For details, see the [Can I delete a UserIdentity object?](https://deckhouse.io/modules/user-authn/v1.78/faq.html#can-i-delete-a-useridentity-object) section.

- The request rate limit (`rateLimit`) applies to the 2FA steps and the password change page. With 2FA enabled, Dex requests the code before the password change. For details, see the [How is Dex protected against credential brute-forcing?](https://deckhouse.io/modules/user-authn/v1.78/faq.html#how-is-dex-protected-against-credential-brute-forcing) section.

- Active Directory users with [`idAttr: objectGUID`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#dexprovider-v1-spec-ldap-usersearch-idattr) can sign in.

- <a id="release-1-78-user-api"></a>`user-api` accepts only ID tokens of the Deckhouse web UI. The password reset endpoint has been removed.

##### Application access to the Kubernetes API

Changes in the access of applications and external systems to the Kubernetes API:

- <a id="release-1-78-token-exchange"></a>The [TokenExchangeProvider](https://deckhouse.io/modules/user-authn/v1.78/cr.html#tokenexchangeprovider) custom resource describes a trusted issuer whose tokens Dex exchanges for Kubernetes API tokens. It lets you configure access from CI/CD (for example, GitLab CI) without static credentials.

- <a id="release-1-78-dexclient-annotation"></a>The `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation of a DexClient is always in effect. The application of such a DexClient gets ID tokens that `kube-apiserver` accepts and can act in the Kubernetes API on behalf of signed-in users. Previously, the annotation had no effect without a DexAuthenticator with access to Kubernetes, `publishAPI`, or `kubeconfigGenerator`. For details about using the annotation, see the [Granting an application access to the Kubernetes API](https://deckhouse.io/modules/user-authn/v1.78/usage.html#granting-an-application-access-to-the-kubernetes-api) section.

- <a id="release-1-78-kubeconfig-token-exchange"></a>kubeconfig clients can no longer exchange tokens of an external provider for Dex tokens.

- <a id="release-1-78-legacy-token-exchange"></a>Token exchange through DexProvider is deprecated. In existing clusters that have a DexClient with the `dexclient.deckhouse.io/allow-access-to-kubernetes` annotation, the exchange through OIDC and GitLab DexProvider objects stays enabled with the `spec.tokenExchange.enabled` parameter, and it can only be disabled. When a token has been exchanged through a DexProvider during the last 24 hours, the [D8UserAuthnDexProviderTokenExchangeDeprecated](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#user-authn-d8userauthndexprovidertokenexchangedeprecated) alert fires. Move the exchange to TokenExchangeProvider before this feature is removed after the next LTS release. The restriction for kubeconfig clients is already in effect in 1.78 regardless of the DexProvider setting. For details, see [TokenExchangeProvider](#release-1-78-token-exchange).

##### Authentication for applications

Changes in the operation of DexAuthenticator and Dex:

- DexAuthenticator has been moved to `oauth2-proxy` 7.15.5. It verifies the Dex TLS certificate, uses PKCE, does not end user sessions when Dex is briefly unavailable, and supports signing in from several tabs. Signing out of an application revokes the refresh token in Dex. How DexAuthenticator works is described in the [Authentication flow with DexAuthenticator](https://deckhouse.io/modules/user-authn/v1.78/faq.html#authentication-flow-with-dexauthenticator) section.

- <a id="release-1-78-dexauthenticator-ids"></a>DexAuthenticator clients have got unambiguous IDs. After the update, users of applications behind DexAuthenticator have to sign in again once. The `aud` claim of DexAuthenticator ID tokens changes to `dex-authenticator-<NAME>@<NAMESPACE>`. An application that checks `aud` must accept the new value. For details, see the [JWT token contents and processing considerations](https://deckhouse.io/modules/user-authn/v1.78/faq.html#jwt-token-contents-and-processing-considerations) section.

- <a id="release-1-78-revocation-feed"></a>DexAuthenticator calls the `revocation-feed` service (`user-authn-controller` pods, port `8443/TCP`) in the `d8-user-authn` namespace. If outgoing traffic in the DexAuthenticator namespace is restricted by a NetworkPolicy, allow this traffic.

- DexAuthenticator events (sign-ins, sign-outs, failed attempts, CSRF errors) are sent as security events when the [`security-events-manager` module](https://deckhouse.io/modules/security-events-manager/) is enabled.

- The fault tolerance of Dex and protected applications has been improved:
  - Dex does not restart in a loop when the issuer of an OIDC provider is unavailable and starts with the other providers.
  - Dex does not restart when the node is short of CPU.
  - Protected applications keep working when Dex is briefly unavailable and while it restarts.

- DexAuthenticator replicas with [`spec.highAvailability`](https://deckhouse.io/modules/user-authn/v1.78/cr.html#dexauthenticator-v1-spec-highavailability) are spread across different nodes.

- DexAuthenticator works in isolated projects and in projects with policies that require resource requests to be set.

##### Project management {#release-1-78-projects}

Changes in project quotas, templates, and namespaces:

- Multiple namespaces in one project. In commercial editions, additional namespaces are created through [ProjectNamespace](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#projectnamespace). Permissions granted through ProjectRoleBinding apply across all project namespaces. In Deckhouse Platform Open (CE), a project has only its main namespace.

- The project quota, set in [Project.spec.quota](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#project-v1alpha3-spec-quota), is a single quota for the whole project.
  In commercial editions, the quota limits the total usage across all project namespaces. In Deckhouse Platform Open (CE), it limits usage in the project's only namespace, its main namespace.
  Usage is shown in the read-only [ProjectQuotaStatus](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#projectquotastatus) object.
  Creating an object that exceeds the project quota is rejected. The sum over the namespaces is enforced only when the [`control-plane-manager`](https://deckhouse.io/modules/control-plane-manager/v1.78/) module is enabled.
  The quota can limit the virtual machines of a project (cores, guaranteed CPU, memory, GPU) and load balancers (by class, pool, and external address).

- Changing [the project quota](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#project-v1alpha3-spec-quota) requires the `manage_quotas` capability. Quota usage is checked by the `quota-ledger.multitenancy-manager.deckhouse.io` webhook. While no `multitenancy-manager` replica responds, creating counted objects in the namespaces of projects with a quota fails with error 500 (`failed calling webhook`); error 503 means that the quota ledger is briefly unavailable, for example while its leader changes, and the request can be retried. When the [`control-plane-manager`](https://deckhouse.io/modules/control-plane-manager/v1.78/) module is enabled, the module runs in two replicas.

- With [Project.spec.grants](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#project-v1alpha3-spec-grants), a project can be granted cluster-wide resources in addition to the ones granted by its template. The granting mechanism scales to thousands of objects.

- A project template of version v1alpha2 is described with structured fields: pod security profile, network isolation, log collection, monitoring, and image vulnerability scanning. The controller creates objects from these fields in each namespace of the project and keeps them up to date, so a Helm template does not have to be written and maintained. For details, see the [Structured templates](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#structured-templates) section.

- <a id="release-1-78-project-manifests"></a>The [`manifests`](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#projecttemplate-v1alpha2-spec-manifests) field of ProjectTemplate v1alpha2 sets templated namespaced objects that the controller applies on behalf of a project user with the `d8:project:admin` role, not with its own permissions. With `manifests`, you can create additional project namespaces (ProjectNamespace) and place objects in them. The procedure for moving a template to `manifests` is described in the [Moving a template from a Helm text](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#moving-a-template-from-a-helm-text) section.

- When the [`multitenancy-manager`](https://deckhouse.io/modules/multitenancy-manager/v1.78/) module is enabled, the ways to create a namespace bypassing projects are closed: only system components and cluster administrators can create namespaces with names longer than 61 characters, with the `upmeter-` prefix, or with the name `deckhouse`. Namespaces left without a project are shown by the [D8NamespaceNotAdopted](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#multitenancy-manager-d8namespacenotadopted) alert.

- Deleting a project requires the permission to delete its namespaces. For details, see the [Additional project namespaces](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#additional-project-namespaces) section.

- Labels and annotations of a namespace adopted into a project are no longer copied to the Project parameters, and the project release does not roll them back. Additional project namespaces do not inherit them. Set the labels you need in `spec.parameters.namespace.labels` of the project. A namespace adopted into a project keeps its Pod Security Standard.

- The [`allowNamespacesWithoutProjects`](https://deckhouse.io/modules/multitenancy-manager/v1.78/configuration.html#parameters-allownamespaceswithoutprojects) parameter is no longer considered deprecated. The restriction is implemented with ValidatingAdmissionPolicy.

- <a id="release-1-78-project-namespace-restore"></a>The main namespace of a project deleted bypassing the policy (for example, as `system:sudouser`) is restored by the controller together with the template objects. During the update, all projects that have lost their main namespace will be restored.

- <a id="release-1-78-helm-project-templates"></a>Helm templates of projects (`resourcesTemplate`) are deprecated. A [ProjectTemplate](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#projecttemplate) can no longer be created through `deckhouse.io/v1alpha1`, and a new `resourcesTemplate` cannot be set either. Existing templates keep working and can be edited. However, restoring such a template from a backup, a GitOps bootstrap, or recreation will be rejected until the template is moved to structured fields and `manifests`. Such templates are shown by the [MultitenancyManagerHelmProjectTemplate](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#multitenancy-manager-multitenancymanagerhelmprojecttemplate) alert. AuthorizationRule objects of the Admin level are forbidden in `manifests`; set project administrators in `spec.administrators`. For details, see [the `manifests` field](#release-1-78-project-manifests).

- <a id="release-1-78-grant-policy"></a>[ClusterResourceGrantPolicy](https://deckhouse.io/modules/multitenancy-manager/v1.78/cr.html#clusterresourcegrantpolicy) is stored in version v1alpha2. Version v1alpha2 selects projects by Project labels and namespaces by a separate `namespaceSelector`. Policies written through v1alpha1 keep selecting the same namespaces. When moving a manifest to v1alpha2, rename `projectSelector` to `namespaceSelector`. If you change only `apiVersion`, the selector starts matching Project labels, and the policy stops applying to the namespaces you need. Additional namespaces inherit the template labels. For details, see the [How a policy selects namespaces](https://deckhouse.io/modules/multitenancy-manager/v1.78/usage.html#how-a-policy-selects-namespaces) section.

#### Security and certificates {#release-1-78-security}

Changes in security policies and certificate management.

##### Security policies

Changes in the [`admission-policy-engine`](https://deckhouse.io/modules/admission-policy-engine/v1.78/) module:

- <a id="release-1-78-pss-default"></a>The default Pod Security Standards policy is always [Baseline](https://deckhouse.io/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-defaultpolicy).
  In clusters deployed on a Deckhouse Platform version earlier than 1.55, the `Privileged` policy was the default.

  In such clusters, the update is blocked until the [`settings.podSecurityStandards.defaultPolicy`](https://deckhouse.io/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-defaultpolicy) parameter is set in the [`admission-policy-engine`](https://deckhouse.io/modules/admission-policy-engine/v1.78/) ModuleConfig.

  To keep the current behavior, specify `Privileged`. Before switching to `Baseline` or `Restricted`, make sure that the workloads in non-system namespaces comply with the chosen policy.

  If only individual workloads do not meet the requirements, do not lower the policy for the whole cluster. Create [SecurityPolicyException](https://deckhouse.io/modules/admission-policy-engine/v1.78/#security-policy-exceptions) exceptions for them at the level of a pod or an individual container.

- <a id="release-1-78-selinux"></a>The SELinux check in Pod Security Standards has changed. The `Baseline` and `Restricted` policies reject pods whose SELinux type differs from `container_t`, `container_init_t`, `container_kvm_t`, and `container_engine_t`, as well as pods with an SELinux user or role set. Before the update, such pods pass the check. An example of a policy with allowed SELinux types is given in the [How to allow some Pod Security Standards policies without disabling whole list?](https://deckhouse.io/modules/admission-policy-engine/v1.78/faq.html#how-to-allow-some-pod-security-standards-policies-without-disabling-whole-list) section.

- [Pod Security Standards](https://deckhouse.io/modules/admission-policy-engine/v1.78/#pod-security-standards) rules are published as the `d8-pod-security-baseline` and `d8-pod-security-restricted` SecurityPolicy objects. The `d8-pod-security-`, `d8-system-default-`, and `d8-controllers-` name prefixes for SecurityPolicy and OperationPolicy are reserved for the module.

- **Controller validation.** The check of controllers can be configured for each rule: [`podSecurityStandards.controllerValidationExcludedPolicies`](https://deckhouse.io/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-controllervalidationexcludedpolicies) for Pod Security Standards and `spec.controllerValidation` for [OperationPolicy](https://deckhouse.io/modules/admission-policy-engine/v1.78/cr.html#operationpolicy-v1alpha1-spec-controllervalidation) and [SecurityPolicy](https://deckhouse.io/modules/admission-policy-engine/v1.78/cr.html#securitypolicy-v1alpha1-spec-controllervalidation). For a transition period, a controller that creates pods and violates a policy with `enforcementAction: Deny` gets a warning instead of a rejection. The pods themselves are still rejected. To reject the controller as well, set [`controllerValidationEnforcement: Policy`](https://deckhouse.io/modules/admission-policy-engine/v1.78/configuration.html#parameters-podsecuritystandards-controllervalidationenforcement) or `spec.controllerValidation.mode: Enforce`.

- In [SecurityPolicyException](https://deckhouse.io/modules/admission-policy-engine/v1.78/cr.html#securitypolicyexception) exceptions for hostPath, nested paths can be allowed with the `pathType` parameter set to `Prefix`. SecurityPolicyException also lets you set exceptions for sysctl, AppArmor Localhost profiles, and shared hostPath volumes.

- <a id="release-1-78-exec-attach"></a>Policies for `exec` and `attach` are applied in user namespaces again, as in Deckhouse Platform 1.73–1.74. For details, see the [How to deny kubectl exec and kubectl attach to specific Pods?](https://deckhouse.io/modules/admission-policy-engine/v1.78/faq.html#how-to-deny-kubectl-exec-and-kubectl-attach-to-specific-pods) section.

- Container registry credentials for image signature verification are cached for 5 minutes instead of 12 hours. The caching time is set by the [`imageSignatureVerification.registryCredentialsCacheTTLSeconds`](https://deckhouse.io/modules/admission-policy-engine/v1.78/configuration.html#parameters-imagesignatureverification-registrycredentialscachettlseconds) parameter.

- Alerts about policy violations contain more details and fire separately for each policy and namespace.

- Alerts report the unavailability of the `admission-policy-engine` module components and of the admission webhook. For details, see the [What to do if the validating webhook is unavailable?](https://deckhouse.io/modules/admission-policy-engine/v1.78/faq.html#what-to-do-if-the-validating-webhook-is-unavailable) section.

- Policies stay in effect during an update that adds a constraint template. The Gatekeeper webhook stops sending requests to an unavailable node after 30 seconds instead of 5 minutes.

- When the `admission-policy-engine` module is disabled, exec into pods and the Deckhouse Platform queue are no longer blocked.

##### Certificate management

Changes in the [`cert-manager`](https://deckhouse.io/modules/cert-manager/v1.78/) module:

- <a id="release-1-78-cert-manager-proxy"></a>`cert-manager` uses the proxy from the [`proxy`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#clusterconfiguration-proxy) parameter of ClusterConfiguration. Requests to the ACME server, HashiCorp Vault, DNS provider APIs, and the HTTP-01 self-check go through the proxy. For details, see the [How does cert-manager work behind a proxy server?](https://deckhouse.io/modules/cert-manager/v1.78/faq.html#how-does-cert-manager-work-behind-a-proxy-server) section.

- The [`highAvailability`](https://deckhouse.io/modules/cert-manager/v1.78/configuration.html#parameters-highavailability) parameter controls the high availability mode of the `cert-manager` module regardless of the global setting.

- New parameters of the `cert-manager` module:
  - [`enableACMERenewalInfo`](https://deckhouse.io/modules/cert-manager/v1.78/configuration.html#parameters-enableacmerenewalinfo): Enables ACME Renewal Information (ARI). `cert-manager` reissues a certificate in the window assigned by the ACME server, for example Let's Encrypt.
  - [`http01WaitInsteadOfSelfCheck`](https://deckhouse.io/modules/cert-manager/v1.78/configuration.html#parameters-http01waitinsteadofselfcheck): Replaces the HTTP-01 self-check of the Let's Encrypt ClusterIssuer with a wait. Certificates are issued even in clusters that cannot reach their own public address, for example because of NAT without hairpinning or split-horizon DNS.
  - [`certificateRequestMaximumBackoffDuration`](https://deckhouse.io/modules/cert-manager/v1.78/configuration.html#parameters-certificaterequestmaximumbackoffduration): Limits the delay between issuance attempts after a failed CertificateRequest. With a lower value, `cert-manager` retries the issuance sooner when the issuer is available again, for example after CA maintenance.

- The `cert-manager` webhook keeps serving requests during the update and rotation of its CA. The mutating webhook is called only for new CertificateRequest objects.

- The `spec.certificateOwnerRef` field of the standard `cert-manager` Certificate custom resource no longer causes an endless Secret update loop.

#### Network and cross-cluster communication {#release-1-78-network}

Changes in the publishing of system interfaces, Istio, and pod communication between clusters.

##### Publishing system interfaces

The global Deckhouse Platform parameter [`settings.modules.ingress`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-modules-ingress) enables or disables the creation of Ingress resources for all modules that support it at once. An individual module can override it with its own `ingress.enabled` parameter.

##### Service mesh based on Istio

Changes in the [`istio`](https://deckhouse.io/modules/istio/v1.78/) module:

- <a id="release-1-78-istio-versions"></a>Istio 1.29 is supported. Support for Istio 1.21 has been discontinued. The minimum supported version is 1.25.

- <a id="release-1-78-istio-ca"></a>Before applying the settings, Istio checks the CA certificates and key. If the previously set data is invalid, the module cannot apply the configuration until it is fixed.

- For Istio 1.27 and later, the native sidecar is used by default.

- The CA certificate and key for Istio can be stored in a Secret, and the `istio` module settings can reference it instead of containing the values.

- In the Ultimate/EE edition, the Istio configuration is checked regularly, with notifications about errors.

- An external HTTP service can be used to authorize requests in Istio.

- [OpenTelemetry and Zipkin](https://deckhouse.io/modules/istio/v1.78/examples.html#tracing-with-telemetry-api) are available as tracing providers for sending the traces of individual applications to the corresponding collectors.

- IstioMulticluster statuses describe the causes of metadata exchange errors more precisely.

- Istio proxy injection templates have been fixed. In some configurations, they prevented pods with a native sidecar from starting or did not add the required security settings and health check to the gateway proxy.

- Detection of the Istio version for pods has been fixed. In some configurations, this eliminates false warnings about version mismatch and lets the sidecar be updated automatically.

- The Istio 1.25 operator starts on nodes with containerd v2 when the container file system is read-only.

- A bug has been fixed that could leave istiod 1.25 running after the Istio version was changed or the module was disabled.

- If the Istio 1.25 Helm release gets stuck during installation, update, or deletion, the operator restores it and keeps applying the configuration.

- Adding or removing an Istio version no longer causes a `kube-apiserver` restart.

- The compatibility check before a Deckhouse Platform update also takes into account Istio versions whose removal has not completed yet.

- In Istio 1.25, istiod replicas are placed on different nodes.

- The User role can view [WaypointInstance](https://deckhouse.io/modules/istio/v1.78/cr.html#waypointinstance) objects, and the Editor role can manage them.

- **Protection of metadata exchange between Istio clusters.** If the ID of a neighboring cluster changes, Deckhouse Platform stops exchanging private data and reports it. If a cluster is replaced intentionally, the connection has to be created again.

##### Cross-cluster pod communication through ClusterMesh {#release-1-78-clustermesh}

ClusterMesh in the `cni-cilium` module (available in the Ultimate/EE editions) is an experimental feature that provides direct pod-to-pod communication between clusters joined with the peering module. For details, see the [Cluster peering](https://deckhouse.io/modules/cni-cilium/v1.78/advanced_usage.html#cluster-peering) section.

#### Platform and module management {#release-1-78-platform}

Changes in the storage of platform component images, module delivery, and access to module documentation.

##### Managing the storage of Deckhouse Platform images {#release-1-78-registry}

Changes in the [`registry`](https://deckhouse.io/modules/registry/v1.78/) module:

- **Management of the pull paths of Deckhouse Platform component images.** The upstream address and the use of the in-cluster cache change without rewriting image references and without updating all nodes, because `registry-agent` on each node configures containerd with a permanent in-cluster address. Both parameters are set independently in the [`registry` ModuleConfig](https://deckhouse.io/modules/registry/v1.78/configuration.html). With the cache, all nodes get images from the registry on master nodes. Without an upstream, the cluster works in isolation, and the cache becomes the only source of images. Before the upstream is removed, the module checks that the cache contains the expected set of images. The module takes control automatically after the update to 1.78 if the cluster was [prepared on version 1.77](#release-1-78-registry-migration).

- <a id="release-1-78-registry-ports"></a>The container registry on master nodes uses port `4281/TCP` for pulls by agents and for replication and port `4283/TCP` for publishing through the registry-push Ingress, instead of `5001` and `5003`. `registry-agent` stays on `127.0.0.1:5001`, and the `registry.d8-system.svc:5001` references do not change. Publishing and pulling through the cache are allowed only in repositories under `system/deckhouse`. For details, see the [The in-cluster cache](https://deckhouse.io/modules/registry/v1.78/#the-in-cluster-cache) section.

- <a id="release-1-78-registry-tolerations"></a>`registry-controller`, `registry-storage`, and `registry-packages-proxy` no longer take into account the keys from [`global.modules.placement.customTolerationKeys`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/global.html#parameters-modules-placement-customtolerationkeys).

- **Maintenance of the in-cluster cache of the `registry` module.** Garbage collection starts after the set of images is updated and when a write to the storage fails. It can be started manually through RegistryOperation, and storage replicas report the expected amount of space to be freed. Corrupted blobs are checked and restored in the background.

- **Diagnostics of the registry cache.** The RegistryImage object shows the cache contents. `registry-agent` metrics are collected through the textfile collector of `node-exporter`. For details, see the [The cache keeps growing. What reclaims it?](https://deckhouse.io/modules/registry/v1.78/faq.html#the-cache-keeps-growing-what-reclaims-it) section.

- **RegistryUpstream without `registry-agent`.** Additional container registries, for example vendor registries or your own ones, declared in RegistryUpstream are available to nodes both when the `registry` module is disabled and in the `Unmanaged` mode. The settings are applied on Generic OS and Deckhouse Engine nodes. `registry-agent` takes the `proxy` parameter of ClusterConfiguration into account when it accesses the upstream registry. For details, see the [Additional registries](https://deckhouse.io/modules/registry/v1.78/#additional-registries) section.

- **Update and switchover of the registry storage.** The storage elects a new leader when a master node is lost or when the registry or a disk fails. The cache address on nodes is not switched prematurely during a leader change. The agent does not wait for an unavailable leader on every image pull. The cache update works in an air-gapped cluster with one master node, and the reason for a block is shown by the [D8RegistryStorageUpdateBlocked](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/alerts.html#registry-d8registrystorageupdateblocked) alert. For details, see the [How is the cache updated?](https://deckhouse.io/modules/registry/v1.78/faq.html#how-is-the-cache-updated) section.

- A hang of `registry-packages-proxy` after a cache error and the serving of corrupted packages, which blocked node updates, have been fixed.

##### Modules from external sources {#release-1-78-external-modules}

The following modules are loaded from external sources and are developed in a separate release cycle:

- The [`cloud-provider-dvp`](https://deckhouse.io/modules/cloud-provider-dvp/) module. Deckhouse Platform can be updated with this module in use only if the module is available from a [module source (ModuleSource)](https://deckhouse.io/products/kubernetes-platform/documentation/v1.78/reference/api/cr.html#modulesource).
- The [`service-with-healthchecks`](https://deckhouse.io/modules/service-with-healthchecks/) module. The module is installed automatically if it is enabled in the cluster.

##### Module documentation

Module documentation can be opened for any version, regardless of the version installed in the cluster. To view a specific version, select "View other versions" in the "Channel/version" menu on the module documentation page.

### Vulnerability fixes

Security-related fixes:

- Proxy settings in ClusterConfiguration no longer allow running arbitrary shell commands during the bootstrap.

- DexAuthenticator no longer logs the request cookie, including the refresh token, on a CSRF error (GHSA-hhqp-vx7f-5c6m).

- Vulnerabilities in Istio and Kiali components have been fixed.

- Vulnerabilities in `golang.org/x/net`, `golang.org/x/text`, `go-jose/v4`, `go-chi/chi/v5`, and `klauspost/compress` have been fixed in the [`user-authn`](https://deckhouse.io/modules/user-authn/v1.78/), [`user-authz`](https://deckhouse.io/modules/user-authz/v1.78/), and [`multitenancy-manager`](https://deckhouse.io/modules/multitenancy-manager/v1.78/) modules and in the modules of the Security subsystem.

### Component version updates

For components with several delivery variants, the updated versions of each variant are listed. Supported Kubernetes versions and the default version are given in the [description of control plane changes](#release-1-78-kubernetes).

New versions of Deckhouse Platform components:

- Amazon EC2 Utils: 2.3.0
- Amazon ECR credential provider: v1.33.4, v1.34.3, v1.35.2, v1.36.0, v1.37.0
- Amazon Web Services Cloud provider: v1.33.4, v1.34.3, v1.35.2, v1.36.0, v1.37.0
- Amazon Web Services CSI driver: v1.54.0, v1.66.0
- Amazon Web Services Terraform provider: 6.69.1
- cert-manager: 1.21.2
- cfssl: 1.7.1
- Cluster API: 1.12.11
- Cluster Autoscaler: v1.33.1, v1.34.4, v1.35.2, v1.36.1
- containerd: 1.7.36, 2.2.10
- Descheduler: 0.37.0
- Dex: 2.46.0
- Dynamix Terraform provider: 4.12.2
- etcd: 3.6.15
- Gatekeeper: 3.23.1
- Google Cloud Platform Cloud provider: ccm/v33.4.0, ccm/v34.2.0, ccm/v35.0.5
- Google Cloud Platform CSI driver: v1.15.4, v1.27.0
- Istio: 1.29.6
- Kiali: 2.21.0
- Kubernetes: 1.33.13, 1.34.13, 1.35.10, 1.36.6, 1.37.2
- Kubernetes Autoscaler: 1.8.1
- Microsoft Azure Cloud provider: v1.33.6, v1.34.3, v1.36.1, v1.37.0
- Microsoft Azure CSI driver: v1.33.8, v1.34.2, v1.36.0
- OAuth2 Proxy: 7.15.5
- OpenStack Cloud provider: v1.33.1, v1.34.1, v1.35.0, v1.36.0
- OpenStack CSI driver: v1.33.1, v1.34.1, v1.35.0, v1.36.0
- QEMU guest agent: 9.2.4
- Ratify: 1.4.6
- runc: 1.3.6
- VMware vSphere Cloud provider: v1.33.1, v1.34.0, v1.35.1, v1.37.0
- VMware vSphere Cluster API provider: v1.15.3
- VMware vSphere CSI driver: v3.7.1, v3.7.3
- VPA: 1.8.1

The [`user-authn`](https://deckhouse.io/modules/user-authn/v1.78/), [`user-authz`](https://deckhouse.io/modules/user-authz/v1.78/), [`multitenancy-manager`](https://deckhouse.io/modules/multitenancy-manager/v1.78/), and [`admission-policy-engine`](https://deckhouse.io/modules/admission-policy-engine/v1.78/) modules use the distroless base image.

The full list of changes is available in the [changelog for version 1.78](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.78.md).

## Version 1.77

### Important

- Support for Kubernetes 1.36 has been added, while support for Kubernetes 1.31 has been discontinued.
  The default Kubernetes version (used when the [`kubernetesVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`) has been changed to 1.34.

- Control plane components will be restarted during the update.

- All Ingress NGINX Controller pods will be restarted.

- If the [`enableMultiTenancy`](https://deckhouse.io/modules/user-authz/v1.77/configuration.html#parameters-enablemultitenancy) setting of the `user-authz` module is enabled,
  existing RoleBinding and ClusterRoleBinding objects that were previously ignored
  because of ClusterAuthorizationRule limitations become effective after the update.
  Before updating, review the RoleBinding and ClusterRoleBinding objects assigned to users,
  and modify or remove those that should not grant access.

- If the DP cluster uses custom roles based on the legacy RBACv2 scheme,
  the [`D8UserAuthzLegacyRBACv2CustomRoleFound`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#user-authz-d8userauthzlegacyrbacv2customrolefound) alert will fire after the update.
  Such roles must be migrated to the new scheme before updating to DP 1.78, otherwise the update will be blocked.
  The migration procedure is described in the [`user-authz` module FAQ](https://deckhouse.io/modules/user-authz/v1.77/faq.html#how-do-i-migrate-custom-roles-to-the-new-scheme-in-dp-178).

- If the cluster uses the v1 implementation of the container registry management (the `registry` section of the `deckhouse` ModuleConfig) in the `Proxy` or `Local` mode,
  or in the `Direct` mode without the `registry` ModuleConfig configured,
  it must be migrated to the [`registry`](https://deckhouse.io/modules/registry/v1.77/) module before updating to DP 1.78, otherwise the update will be blocked.
  The migration procedures are described in the [`registry` module FAQ](https://deckhouse.io/modules/registry/v1.78/faq.html#how-does-the-migration-to-the-registry-module-work).

- If the [`metallb`](https://deckhouse.io/modules/metallb/v1.77/) module configuration is managed through GitOps,
  temporarily disable synchronization from your repository before the update.
  After the update, replace the BGP configuration in your repository from ModuleConfig
  with the [MetalLoadBalancerConfiguration](https://deckhouse.io/modules/metallb/v1.77/cr.html#metalloadbalancerconfiguration), [MetalLoadBalancerBGPPeer](https://deckhouse.io/modules/metallb/v1.77/cr.html#metalloadbalancerbgppeer), and [MetalLoadBalancerPool](https://deckhouse.io/modules/metallb/v1.77/cr.html#metalloadbalancerpool) custom resources,
  and then re-enable synchronization.

- If the cluster uses NetworkPolicy objects with the `ipBlock: 0.0.0.0/0` rule,
  after the update they will also apply to intra-cluster traffic.
  It is recommended that you review such policies before the update and adjust them if necessary.

- In the AWS, Azure, and GCP providers, the default CNI has been changed to Cilium.
  Before creating or updating a cluster, make sure that the Linux kernel version is 5.8 or later
  and that network policies and firewalls do not block the UDP ports required for Cilium to operate in VXLAN mode.
  Failure to meet these requirements may result in cluster creation or update failures,
  or in loss of network connectivity between nodes.
  For detailed requirements, refer to ["Network interaction"](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/reference/network_interaction.html).

- The vSphere, Huawei Cloud, OpenStack, and VCD providers have been migrated from Terraform to OpenTofu.
  The migration is performed automatically and requires no manual actions.

- The Yandex Cloud and OpenStack providers have been migrated from Machine Controller Manager (MCM) to Cluster API (CAPI).
  The management engine is chosen per CloudEphemeral node group. Groups that already exist keep running on MCM,
  and new groups are created on CAPI by default, so a single cluster can contain groups of both kinds.
  To move an existing group, refer to ["How to migrate node groups to Cluster API (CAPI)"](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/faq.html#how-to-migrate-node-groups-to-cluster-api-capi).

- The following modules are now loaded from external sources
  and are developed on an independent release cycle (no manual migration actions are required):

  - Monitoring modules:

    - `extended-monitoring`
    - `log-shipper`
    - `loki`
    - `monitoring-custom`
    - `monitoring-deckhouse`
    - `monitoring-kubernetes`
    - `monitoring-ping`
    - `okmeter`
    - `operator-prometheus`
    - `prometheus`
    - `prometheus-metrics-adapter`
    - `prometheus-pushgateway`
    - `upmeter`
  
  - The `ingress-nginx` module.

- The `monitoring-kubernetes-control-plane` module has been merged into the `control-plane-manager` module.
  Monitoring of control plane components is now performed by the `control-plane-manager` module.

### Deckhouse subsystem

- A policy has been added to the [`deckhouse`](https://deckhouse.io/modules/deckhouse/v1.77/) module that prevents non-system users from setting or modifying the `heritage` label.
  This prevents accidental creation of resources that DP may recognize as its own.

- The reliability of updating clusters that use external modules (ModuleSource) has been improved.
  The update starts only after the modules have been preloaded, preventing their temporary unavailability during the update.

- Support has been added to the `registry` module for installing a cluster with the container registry operating in [`Local`](https://deckhouse.io/modules/deckhouse/v1.77/configuration.html#parameters-registry-mode) mode.
  This makes it possible to deploy static clusters without access to an external registry, including in air-gapped environments.

### Kubernetes & Scheduling subsystem

- Changes in the [`descheduler`](https://deckhouse.io/modules/descheduler/v1.77/) module:

  - Added the `descheduler_pod_evictions_total` metric with information about the workload to which the evicted pod belongs.

  - Added the "Workload Happiness" Grafana dashboard, which displays the workload happiness score,
    workload distribution across nodes, and the module activity history (evictions, strategy execution, and timeline).

- Changes in the [`vertical-pod-autoscaler`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.77/) module:

  - Added the [`VerticalPodAutoscalerDeprecatedUpdateModeAuto`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#vertical-pod-autoscaler-verticalpodautoscalerdeprecatedupdatemodeauto) alert and the "VPA Deprecated Update Mode" Grafana dashboard
    to detect VerticalPodAutoscaler objects that use the deprecated `updateMode: Auto mode`.

  - Improved the reliability of the `vertical-pod-autoscaler` webhook deployment.
    Fixed a component startup ordering issue that could cause errors during updates.

- Changes in the [`control-plane-manager`](https://deckhouse.io/modules/control-plane-manager/v1.77/) module:

  - The module has been migrated to a new internal implementation for managing control plane components.
    Added the new [ControlPlaneNode](https://deckhouse.io/modules/control-plane-manager/v1.77/cr.html#controlplanenode) and [ControlPlaneOperation](https://deckhouse.io/modules/control-plane-manager/v1.77/cr.html#controlplaneoperation) custom resources
    to track node state and control plane component update operations.

  - Added automatic etcd defragmentation,
    which is enabled by default for multi-master clusters and runs when fragmentation exceeds 40%.
    A defragmentation schedule can be configured in the [module settings](https://deckhouse.io/modules/control-plane-manager/v1.77/configuration.html#parameters-etcd-defrag).

  - Several settings have been moved to the module ModuleConfig.
    Automatic migration and alerts notifying about the need to update the configuration have also been added:

    - Kubernetes API publishing settings ([`publishAPI`](https://deckhouse.io/modules/control-plane-manager/v1.77/configuration.html#parameters-apiserver-publishapi)) from the `user-authn` module.

    - The [`encryptionAlgorithm`](https://deckhouse.io/modules/control-plane-manager/v1.77/configuration.html#parameters-encryptionalgorithm) parameter from ClusterConfiguration.

    - The [`resourcesRequests`](https://deckhouse.io/modules/control-plane-manager/v1.77/configuration.html#parameters-resourcesrequests) parameter for control plane components from the `global` ModuleConfig.

- Added a patch to `kube-scheduler` that prevents scheduling new pods onto nodes that are undergoing graceful shutdown.

- Accelerated etcd updates in single-master clusters. Eliminated control plane component downtime that occurred during updates.

- Reduced the risk of `kube-controller-manager` throttling under high controller load.

- Accelerated pod startup by enabling parallel pulling of multiple container images.

- Enabled the `KubeletEnsureSecretPulledImages` feature gate by default in the kubelet configuration
  to comply with CIS Benchmark requirements.

### IAM subsystem

- Changes in the [`user-authn`](https://deckhouse.io/modules/user-authn/v1.77/) module:

  - Authentication protection in Dex has been strengthened: [rate limiting](https://deckhouse.io/modules/user-authn/v1.77/configuration.html#parameters-ratelimit) has been added for authentication endpoints,
    LDAP/Crowd accounts are now [automatically blocked](https://deckhouse.io/modules/user-authn/v1.77/configuration.html#parameters-passwordpolicy-lockout) after a series of failed sign-in attempts,
    and external accounts can now be [blocked and unblocked manually](https://deckhouse.io/modules/user-authn/v1.77/cr.html#useroperation-v1-spec-target).

  - The UserOperation resource now supports permanent user locking ([`spec.lock.for: permanent`](https://deckhouse.io/modules/user-authn/v1.77/cr.html#useroperation-v1-spec-lock-for)).
    Audit logs for account operations have also been extended: they now include information about the operation initiator,
    operation type, and target user.

  - Added the [DexProviderCheck](https://deckhouse.io/modules/user-authn/v1.77/cr.html#dexprovidercheck) resource for verifying authentication providers (LDAP, OIDC, and others).
    It allows connection and credential issues to be diagnosed before they affect user sign-in.

  - Added the [`Custom`](https://deckhouse.io/modules/user-authn/v1.77/configuration.html#parameters-passwordpolicy-custom) complexity level to the password policy settings,
    allowing password requirements to be configured flexibly according to corporate standards.

  - Added support for specifying custom labels and annotations in the new [`spec.podMetadata`](https://deckhouse.io/modules/user-authn/v1.77/cr.html#dexauthenticator-v1-spec-podmetadata) field
    of the DexAuthenticator custom resource, for example, annotations for managing the Istio sidecar container.

  - DexAuthenticator now works correctly in namespaces with ResourceQuota
    and policies that require resource limits for all containers.
    Fixed an issue that could prevent the authenticator pod from starting.

- Changes in the [`user-authz`](https://deckhouse.io/modules/user-authz/v1.77/) module:

  - The RBACv2 role model aggregation label scheme changes in DP 1.78.
    Added the [`D8UserAuthzLegacyRBACv2CustomRoleFound`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#user-authz-d8userauthzlegacyrbacv2customrolefound) alert, which detects custom roles based on the legacy scheme.
    Such roles must be migrated to the new scheme before updating to DP 1.78;
    otherwise, they will stop working and the update will be blocked.
    The migration procedure is described in the [module FAQ](https://deckhouse.io/modules/user-authz/v1.77/faq.html#how-do-i-migrate-custom-roles-to-the-new-scheme-in-dp-178).

  - If the [`enableMultiTenancy`](https://deckhouse.io/modules/user-authz/v1.77/configuration.html#parameters-enablemultitenancy) setting is enabled, user permissions are now determined
    by the combination of ClusterAuthorizationRule, AuthorizationRule, and standard RBAC rules (RoleBinding/ClusterRoleBinding).
    After the update, existing RBAC bindings that were previously ignored
    because of ClusterAuthorizationRule limitations may become effective.
    It is recommended to review such bindings before the update.
    For details, refer to ["Using ClusterAuthorizationRule and AuthorizationRule together with RBAC"](https://deckhouse.io/modules/user-authz/v1.77/#using-clusterauthorizationrule-and-authorizationrule-together-with-rbac).

  - Added the [`D8UserAuthzPermissionBrowserUnavailable`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/reference/alerts.html#user-authz-d8userauthzpermissionbrowserunavailable) alert,
    which notifies about the unavailability of the user permissions browsing service (`permission-browser-apiserver`).

### Security subsystem

- Changes in the [`admission-policy-engine`](https://deckhouse.io/modules/admission-policy-engine/v1.77/) module:

  - SecurityPolicy exceptions (SecurityPolicyException) can now be configured at the individual container level
    instead of only for the entire pod.
    This allows security requirements to be relaxed for a specific sidecar or init container
    without removing restrictions from the entire pod.
    For details, see the [module documentation](https://deckhouse.io/modules/admission-policy-engine/v1.77/#security-policy-exceptions).

  - Added the built-in ValidatingAdmissionPolicy [`deny-deckhouse-finalizers.deckhouse.io`](https://deckhouse.io/modules/admission-policy-engine/v1.77/faq.html#built-in-policy-for-finalizers),
    which prevents manual removal of `deckhouse.io` finalizers from resources.
    This prevents incorrect object deletion and the creation of orphan resources.

- Added a mechanism to the `multitenancy-manager` module for [granting cluster-wide resources to projects](https://deckhouse.io/modules/multitenancy-manager/v1.77/usage.html#granting-cluster-scoped-resources-to-projects).
  DP administrators can now define which cluster resources are available to each project and specify default values for them.
  The mechanism can be used by any DP module to publish its own cluster resources.

- Added the ability to list resources across all accessible namespaces without cluster-wide permissions.
  The `d8 k get <resource> -A --scope=<accessible|projects|system|project:NAME>` command returns only the objects
  the user has access to instead of returning a `403 Forbidden` error.
  For details, see ["Resource listing with access-based filtering"](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/admin/configuration/access/authorization/filtered-listing.html).

- ACL filtering of the namespace list now supports `watch` requests.
  The `d8 k get ns -w` command displays changes in real time without requiring the command to be re-run.

- The cainjector component of the `cert-manager` module is now [enabled automatically](https://deckhouse.io/modules/cert-manager/v1.77/configuration.html#parameters-enablecainjector)
  when resources using `cert-manager` CA injection are detected.
  It no longer needs to be enabled manually or kept running at all times.

### Cluster & Infrastructure subsystem

- Changes in `dhctl` (CLI installer):

  - Added a preflight check for disk name length in cloud providers.
    If the name exceeds the allowed limit, the installation is aborted before infrastructure provisioning begins.

  - Improved the interactive mode of `dhctl`: added a dedicated area for logs of current operations,
    and made the output more compact by displaying progress instead of a stream of service messages.
    Full logs are still available in debug mode and during non-interactive execution.

  - Improved the preflight check for the `deckhouse` user.
    Repeated bootstrap now correctly handles an existing user, and additional checks help prevent permission-related issues.

  - Fixed issues with cleaning up temporary processes and SSH connections after running the `converge` and `destroy` commands.

  - Fixed the `dhctl lock release` command in interactive mode.

  - Fixed issues in the `dhctl converge-migration` command that could cause migration failures in certain scenarios.

  - Improved the reliability of control plane `converge` operations.
    Checks are now performed in a more appropriate order, and etcd status messages have become more informative.

- Changes in the [`node-manager`](https://deckhouse.io/modules/node-manager/v1.77/) module:

  - Added [Instance API v1alpha2](https://deckhouse.io/modules/node-manager/v1.77/cr.html#instance).
    Instance status and error information is now consolidated into a single resource, simplifying diagnostics.

  - Reduced the wait time for MachineDeployment creation for AWS Spot instances.
    If AWS cannot allocate Spot instances because of insufficient capacity, retries begin sooner.

  - Added the [`spec.kubelet.seccompDefault`](https://deckhouse.io/modules/node-manager/v1.77/cr.html#nodegroup-v1-spec-kubelet-seccompdefault) parameter to the NodeGroup resource,
    allowing the RuntimeDefault seccomp profile to be enabled by default for all workloads without explicit seccomp settings.

- Reduced the probability of false restarts of the CSI Controller in all cloud provider modules
  during short delays by increasing the liveness probe timeout.

- Changes in the [`cloud-provider-dvp`](https://deckhouse.io/modules/cloud-provider-dvp/v1.77/) module:

  - Added support for live disk migration between StorageClass objects without stopping the virtual machine.
    Migration is performed automatically when the StorageClass changes.

  - The module configuration has been migrated to the unified ModuleConfig contract.
    Permanent and Ephemeral nodes are now configured in the same way through InstanceClass and NodeGroup,
    without a separate ProviderClusterConfiguration.
    This simplifies cluster configuration, enables schema versioning and conversion without breaking compatibility,
    and prepares cloud providers for an independent release cycle outside the DP core.
    For details, refer to ["Cluster and Infrastructure"](https://deckhouse.io/products/kubernetes-platform/documentation/v1.77/faq.html#subsystem-cluster-infrastructure) in the DP FAQ.

  - Fixed an issue that could leave LoadBalancer Services in the `Pending` state without creating the parent load balancer.
    Improved conflict handling during ServiceWithHealthchecks updates and load balancer status synchronization.

  - StorageClass objects created through Cluster API clusters now always use the `WaitForFirstConsumer` mode
    regardless of the parent cluster configuration.
    During updates, managed StorageClass objects with an incompatible `volumeBindingMode` are also recreated automatically.
    This eliminates an issue that could block cluster updates and synchronization because of the immutable `volumeBindingMode` field.

- The `cloud-provider-openstack`, `cloud-provider-huaweicloud`, `cloud-provider-vsphere`, and `cloud-provider-vcd` modules now
  use OpenTofu instead of Terraform.
  This makes it possible to use an open infrastructure platform compatible with Terraform,
  reduces dependency on Terraform licensing changes, and aligns these providers
  with other cloud provider modules that already use OpenTofu.

- The OpenStack and Yandex Cloud providers now use Cluster API,
  the industry-standard framework for machine management in Kubernetes.
  To ensure a smooth transition, the legacy mode based on Machine Controller Manager (MCM) remains supported.
  Existing node groups keep running on MCM, while new ones are created on Cluster API.

- Improved virtual machine deletion handling in the [`cloud-provider-huaweicloud`](https://deckhouse.io/modules/cloud-provider-huaweicloud/v1.77/) module,
  increasing resource cleanup reliability and preventing false deletion errors.
  Virtual machines that remain in the deleting state for more than 24 hours
  are now marked with the corresponding status, simplifying diagnostics.

- Updated the CSI plugin and CSI Syncer versions in the [`cloud-provider-vsphere`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.77/) module,
  and fixed known vulnerabilities in Cloud Controller Manager, the CSI plugin, and Terraform Manager.
  This improves integration security and ensures the use of up-to-date component versions.

- Added support for the `vcd.cpi.flant.com/load-balancer-ip` annotation for LoadBalancer Services in the [`cloud-provider-vcd`](https://deckhouse.io/modules/cloud-provider-vcd/v1.77/) module.
  It allows assigning a predefined external IP address to the load balancer.
  If both the annotation and the `spec.loadBalancerIP` field are specified, the annotation takes precedence.

### Network subsystem

- Changes in the [`istio`](https://deckhouse.io/modules/istio/v1.77/) module:

  - Added support for Istio 1.27.
    Users now have access to the new features of the latest Istio release using an operatorless mode.

  - Expanded ambient mesh support.
    [New settings](https://deckhouse.io/modules/istio/v1.77/configuration.html#parameters-ambient) provide more explicit control over ambient mesh, allow using waypoint proxies for Layer 7 traffic processing,
    and enable the HBONE protocol for encrypting traffic between pods without requiring a sidecar container to run in every pod.

- Changes in the [`cni-cilium`](https://deckhouse.io/modules/cni-cilium/v1.77/) module:

  - Changed the behavior of NetworkPolicy objects with the `ipBlock: 0.0.0.0/0` rule in the new module version.
    The rule now also applies to intra-cluster traffic.

  - Reduced CPU consumption by Cilium agents. The freed resources become available to user workloads.

- Changes in the [`ingress-nginx`](https://deckhouse.io/modules/ingress-nginx/) module:

  - Added support for IngressNginxController v1.15.

  - Updated the load calculation formula for Ingress NGINX Controller pods used for horizontal scaling of LoadBalancer,
    LoadBalancerWithProxyProtocol, and LoadBalancerWithSSLPassthrough inlets.
    The controller now adds and removes replicas in a timely manner, resulting in more stable behavior under load.

  - Updated the IngressNginxController API to v2.
    The `spec.resourcesRequests` resource consumption settings have been replaced
    with the extended [`spec.resourcesManagement`](https://deckhouse.io/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-resourcesmanagement) settings, including support for resource limits.
    Compatibility with the v1 API is preserved automatically.

  - Implemented isolated configuration validation, making it possible
    to restore full NGINX configuration validation without reducing the security level.

- Added authorization for the Hubble web interface in the [`cilium-hubble`](https://deckhouse.io/modules/cilium-hubble/v1.77/) module.
  Access to cluster network telemetry can now be restricted by user groups to improve security.

- In the [`metallb`](https://deckhouse.io/modules/metallb/v1.77/) module, BGP mode configuration has been moved from ModuleConfig
  to the dedicated [MetalLoadBalancerPool](https://deckhouse.io/modules/metallb/v1.77/cr.html#metalloadbalancerpool), [MetalLoadBalancerBGPPeer](https://deckhouse.io/modules/metallb/v1.77/cr.html#metalloadbalancerbgppeer), and [MetalLoadBalancerConfiguration](https://deckhouse.io/modules/metallb/v1.77/cr.html#metalloadbalancerconfiguration) custom resources.
  The existing configuration will be migrated automatically, so no manual changes in the cluster are required.
  All new BGP settings must be configured through these resources.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.77.md) on GitHub.

## Version 1.76

### Important

- The [fencing-agent component](https://deckhouse.io/modules/node-manager/v1.76/cr.html#nodegroup-v1-spec-fencing-mode) of the `node-manager` module now uses a gossip-based protocol ([memberlist](https://github.com/hashicorp/memberlist) library)
  for distributed node health monitoring.
  This reduces the risk of false worker node reboots when the control plane is unavailable or the API server is under heavy load.

- Fixed an issue where `cluster-autoscaler` could get stuck after a failed machine creation in the cloud
  (for errors other than `ResourceExhausted`).
  The cluster now automatically recovers its ability to scale after temporary cloud provider failures.

### Deckhouse subsystem

- Added support for deploying a DP cluster using the [`Proxy`](https://deckhouse.io/modules/deckhouse/v1.76/configuration.html#parameters-registry-proxy) mode for accessing the container registry.

- `Kubectl` on cluster nodes has been replaced with the `d8 k` alias ([Deckhouse CLI utility](https://deckhouse.io/products/kubernetes-platform/documentation/v1.76/cli/d8/)).

- Changes in `dhctl` (CLI installer):

  - Added support for running `dhctl` as a standalone binary without downloading the full image with all dependencies.
    Required components are pulled from the registry during execution, reducing startup time.

  - Added validation of public SSH keys. Invalid keys are now detected earlier, before they can cause connection issues.

  - Improved handling of TLS certificates for registries. The system certificate pool is now used together with a custom CA.

  - Added validation of InstanceClass against the selected cloud provider before installation, allowing early detection and installation abort in case of misconfiguration.

  - Fixed namespace configuration during bootstrap. `Dhctl` now updates settings even if the namespace already exists.

- Improved multi-master resiliency.
  Kubernetes versions are now correctly detected even when some master nodes are unavailable.
  Kube-apiserver no longer routes requests to etcd nodes in the learner state.

### Kubernetes & Scheduling subsystem

- The [`d8-cluster-kubernetes`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.76/admin/configuration/platform-scaling/control-plane/updating-and-versioning.html#monitoring-kubernetes-update-progress) ConfigMap now includes supported Kubernetes versions (`supportedVersions`),
  versions available for upgrade and downgrade (`availableVersions`), the current automatic version (`automaticVersion`).

- Changes in the [`descheduler`](https://deckhouse.io/modules/descheduler/v1.76/) module:

  - Upgraded to version 0.35.1.
    Added namespace filtering via [label selectors](https://deckhouse.io/modules/descheduler/v1.76/cr.html#descheduler-v1alpha2-spec-namespacelabelselector) and other improvements (see the [module changelog](https://github.com/kubernetes-sigs/descheduler/releases/tag/v0.35.1) for details).

  - Added the [`deschedulingInterval`](https://deckhouse.io/modules/descheduler/v1.76/configuration.html#parameters-deschedulinginterval) parameter to control how often descheduling runs: `Frequent`, `Moderate`, or `Rare`.

  - Added the [`RemovePodsHavingTooManyRestarts`](https://deckhouse.io/modules/descheduler/v1.76/cr.html#descheduler-v1alpha2-spec-strategies-removepodshavingtoomanyrestarts) strategy, which evicts pods exceeding a restart threshold,
    freeing node resources from pods in `CrashLoopBackOff` state.

  - The module now [automatically uses the Kubernetes Metrics API](https://deckhouse.io/modules/descheduler/v1.76/#metrics-provider) when the `metrics.k8s.io` API group is available in the cluster.
    This provides real resource consumption data for strategies to use.

- The [`vertical-pod-autoscaler`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.76/) module updated to version 1.6.0. Key changes:

  - The [`InPlaceOrRecreate`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.76/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode) mode is now used by default.
    In this mode, VPA attempts to adjust pod requests and limits without recreating pods (in-place updates).

  - Reduced likelihood of pod restarts when in-place updates are not possible (via the `--in-place-skip-disruption-budget` flag),
    also [`minReplicas`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.76/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-minreplicas) value is no longer considered for in-place updates.

- VPA memory recommendations are now rounded to 64 MiB,
  improving readability and reducing unnecessary pod restarts caused by minor recommendation fluctuations.

- For Kubernetes 1.34 and later, the [`DRAExtendedResource`](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#DRAExtendedResource) feature gate is enabled by default.

- Added the [`D8EtcdHighRequestsLatency`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.76/reference/alerts.html#control-plane-manager-d8etcdhighrequestslatency) alert to detect high etcd request latency, which can lead to API server timeouts.
  A corresponding metric has been added to the "Control Plane Status" dashboard.

### IAM subsystem

- Changes in the [`user-authn`](https://deckhouse.io/modules/user-authn/v1.76/) module:

  - Disabled the insecure [OAuth 2.0 Implicit Flow](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics#name-implicit-grant).
    In that flow, access tokens are passed in URL fragments, posing leakage risks (for example, via the `Referer` header, browser history, or logs).
    Any integrations using the Authorization Code Flow are unaffected.

  - Added SAML 2.0 support via the [DexProvider](https://deckhouse.io/modules/user-authn/v1.76/cr.html#dexprovider-v1-spec-saml) resource.
    DP clusters can now be connected to such identity providers as AD FS, Okta, Keycloak, OneLogin, and Shibboleth.

  - Added refresh token support for SAML in Dex.
    This allows DexAuthenticator и kubeconfig-generator to refresh tokens without a manual user login.

  - Added Single Logout (SLO) support. The identity provider can end user sessions via `/saml/slo/{connector}`.

  - Fixed stale cache issue on the login page by adding a Dex build-based `cache` parameter to the CSS URL.

- In [multitenancy mode](https://deckhouse.io/modules/user-authz/v1.76/configuration.html#parameters-enablemultitenancy), users without permissions defined via ClusterAuthorizationRule
  no longer see all namespaces in the Deckhouse web UI.
  The default behavior is now deny-by-default: users get an empty namespace list unless access is explicitly granted.
  This does not affect privileged groups (`system:masters`, `kubeadm:cluster-admins`).
  Documentation of the `user-authz` module now includes an [example for granting access to all namespaces](https://deckhouse.io/modules/user-authz/v1.76/usage.html#example-of-granting-access-to-all-namespaces).

- Added support for the [`service-with-health-checks`](https://deckhouse.io/modules/service-with-healthchecks/v1.76/) module in isolated projects.
  A NetworkPolicy exception for the `d8-service-with-healthchecks` namespace has been added to the project template,
  which allows module agents to verify service availability and correctly operate under conditions of network isolation.

### Security subsystem

- Gatekeeper upgraded to 3.22.0 and Ratify to 1.4.0.
  Key changes include a unified CEL and Rego policy behavior, added namespace context support in policies,
  and a CLI tool for policy benchmarking.

- The [`denyVulnerableImages`](https://deckhouse.io/modules/operator-trivy/configuration.html#parameters-denyvulnerableimages) setting has been moved from `admission-policy-engine` to `operator-trivy`,
  making it the single owner of this logic.

- The `cert-manager` module upgraded to 1.20.0.
  Key changes include added support for Azure DNS Private Zones in the DNS01 challenge,
  added annotation `acme.cert-manager.io/http01-ingress-ingressclassname` for redefining IngressClasses in HTTP-01 solvers, and security fixes.

- Updated vulnerable Go dependencies in `multitenancy-manager` (`go-jose`, `golang.org/x/crypto`, `grpc`, etc.).

### Cluster & Infrastructure subsystem

- Changes in the [`node-manager`](https://deckhouse.io/modules/node-manager/v1.76/) module:

  - Improved draining hook event generation for better observability of the node eviction process.

  - Fixed StaticInstance issue causing infinite infrastructure deletion loops. That prevents infinite cleanup and queueing cycles.

  - Apiserver-proxy rewritten from NGINX to native Go implementation for better stability and reconnection speed.

- Changes in the [`cloud-provider-dvp`](https://deckhouse.io/modules/cloud-provider-dvp/v1.76/) module:

  - Added hybrid cluster support. Static master nodes and cloud worker nodes can now be used in a single cluster.

  - Improved converge reliability.
    Reworked the mechanism of resource readiness check and added the fail-fast behavior for possible errors
    when creating the infrastructure.

  - Added default StorageClass inheritance for Cluster API child clusters.

  - Improved VM lifecycle handling, reducing potential problems when deleting VMs and associated objects.

  - Added support for the [`masterNodeGroup.instanceClass.additionalDisks`](https://deckhouse.io/modules/cloud-provider-dvp/v1.76/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-additionaldisks) parameter
    that lets you configure additional disks for master nodes.

- Changes in the [`cloud-provider-aws`](https://deckhouse.io/modules/cloud-provider-aws/v1.76/) module:

  - Spot node draining logic was moved into DP, making the spot node handling more unified.

  - Fixed handling of regions without `DescribeInstanceTopology`, reducing the number of false IAM errors.

- Changes in the [`cloud-provider-openstack`](https://deckhouse.io/modules/cloud-provider-openstack/v1.76/) module:

  - Added `ipMode: Proxy` support for LoadBalancer in Kubernetes 1.32 and later.

  - Added the [`csiDriver.fsGroupPolicy`](https://deckhouse.io/modules/cloud-provider-openstack/v1.76/configuration.html#parameters-csidriver-fsgrouppolicy) parameter for the CSI driver,
    which defines whether the volume supports changing owner and permissions before mounting.

- Changes in the [`cloud-provider-vsphere`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.76/) module:

  - Enabled the CSI snapshotter, enabling volume snapshotting.

  - Fixed zone and datastore detection.
    Now the only considered zones are the ones specified in the provider configuration.

- In the [`cloud-provider-azure`](https://deckhouse.io/modules/cloud-provider-azure/v1.76/) module, added support for NVMe disks on Gen2 VMs.

- In the `cloud-provider-gcp` module, added [`enableNestedVirtualization`](https://deckhouse.io/modules/cloud-provider-gcp/v1.76/cr.html#gcpinstanceclass-v1-spec-enablenestedvirtualization) and [`additionalDisks`](https://deckhouse.io/modules/cloud-provider-gcp/v1.76/cr.html#gcpinstanceclass-v1-spec-additionaldisks) parameters to GCPInstanceClass,
  which let you configure nodes for workloads using virtualization and storage scenarios.

- In the [`cloud-provider-yandex`](https://deckhouse.io/modules/cloud-provider-yandex/v1.76/) module, fixed converge failure when removing externalIPAddresses.

- DVP, Huawei Cloud, and zVirt providers were migrated to Cluster API `v1beta2`.

- For Huawei Cloud and VCD providers, added SecurityPolicyException resources and enabled security policy validations.

### Network subsystem

- Added support for handling incoming traffic via Gateway API using the [`alb`](https://deckhouse.io/modules/alb/) module.

- Changes in the [`cni-cilium`](https://deckhouse.io/modules/cni-cilium/v1.76/) module:

  - Added conntrack table synchronization for live VM migration.

  - Added ICMP (ping) support via ExternalIP for LoadBalancer services with MetalLB.

- Added support for [switching between supported CNI plugins](guides/cni-migration.html) in DP clusters.

- In the `node-local-dns` module, added [`disableIPv6`](https://deckhouse.io/modules/node-local-dns/v1.76/configuration.html#parameters-disableipv6) parameter to disable IPv6 DNS resolution.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.76.md) on GitHub.

## Version 1.75

### Important

- Support for Kubernetes 1.35 has been added, while support for Kubernetes 1.30 has been discontinued.
  In future DKP releases, support for Kubernetes 1.31 will be removed.
  The default Kubernetes version (used when the [`kubernetesVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`) has been changed to 1.33.

- Update to DKP 1.75 will be blocked if the cluster uses one of the following deprecated component versions (they are no longer supported):
  - IngressNginxController v1.9
  - Istio v1.19

- In DKP 1.75.0–1.75.3, the `PodSecurityStandardsViolation` alert may fire immediately after cluster installation.
  This happens when the `d8-system` namespace is missing the `heritage=deckhouse` label,
  causing Pod Security Standards policies to be mistakenly applied to DKP system pods.
  The issue is fixed in DKP 1.75.4.

  For clusters installed on DKP 1.75.0–1.75.3, add the required label manually using the following command:

  ```shell
  d8 k --as=system:sudouser label namespace d8-system heritage=deckhouse
  ```

- Istio version support changes:
  - Support for Istio 1.19 has been discontinued.
  - Istio 1.21 is now considered deprecated, and support for this version will be discontinued in the upcoming DKP releases.
    To update Istio, follow the [instruction](https://deckhouse.io/modules/istio/v1.75/examples.html#upgrading-istio).

- Starting with Kubernetes 1.35, cluster nodes must use cgroup v2.
  The previous version (cgroup v1) is [considered deprecated](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.35.md#no-really-you-must-read-this-before-you-upgrade-1).
  If cgroup v2 is not supported on a node, the [D8NodeCgroupV2NotSupported](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/reference/alerts.html#node-manager-d8nodecgroupv2notsupported) alert will be triggered.
  See the [Kubernetes documentation](https://kubernetes.io/docs/concepts/architecture/cgroups/#migrating-cgroupv2) for cgroup v2 migration details.

- Starting with Kubernetes 1.36, cluster nodes must use containerd v2 as the container runtime.
  Support for containerd v1 [will be discontinued](https://kubernetes.io/blog/2025/08/27/kubernetes-v1-34-release/#kubernetes-to-end-containerd-1-x-support-in-v1-36).
  If containerd v2 is not supported on a node, the [D8NodeContainerdV2NotSupported](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/reference/alerts.html#node-manager-d8nodecontainerdv2notsupported) alert will be triggered.

- The `vertical-pod-autoscaler` module has been updated from version 1.4.1 to [1.5.1](https://github.com/kubernetes/autoscaler/releases/tag/vertical-pod-autoscaler-1.5.1).
  Starting from version 1.5.0, the `updateMode: Auto` mode is [considered deprecated](https://github.com/kubernetes/autoscaler/pull/8426).
  Users are advised to replace it with [`updateMode: InPlaceOrRecreate`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.75/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode).
  See the [vertical scaling documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/architecture/kubernetes-and-scheduling/vpa.html#vpa-operating-modes) for details.

- When issuing new certificates using the [`cert-manager`](https://deckhouse.io/modules/cert-manager/v1.75/) module, the certificate's private key is now re-generated as well.
  This change mitigates potential security risks.

- Pods handling HTTP-01 challenges via Issuer and ClusterIssuer resources now have default resource requests defined.
  As a result, these pods consume part of the cluster and project resource quotas upon creation.

- The [operator-trivy](https://deckhouse.io/modules/operator-trivy/) module is now delivered from an external source
  and follows an independent release cycle. No manual migration steps are required.

### Deckhouse subsystem

- The ModuleSource resource now includes the [`scanInterval`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/reference/api/cr.html#modulesource-v1alpha1-spec-scaninterval) parameter
  to control how frequently the container registry is checked for new modules and versions.

- A dedicated node mode with the `node.deckhouse.io/etcd-arbiter` label has been introduced.
  In this mode, only etcd runs on the node (no control plane components).
  This enables a highly available cluster with two master nodes plus a dedicated etcd node,
  reducing resource requirements compared to the traditional three full master nodes configuration.
  See the [High Availability documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/admin/configuration/high-reliability-and-availability/enable.html#configuring-ha-mode-with-two-master-nodes-and-an-arbiter-node) for details.

- New [operating modes](https://deckhouse.io/modules/registry/v1.75/#description) have been introduced in the `registry` module: **Proxy** (with image caching) and **Local** (for isolated environments).
  These modes are currently supported only for already deployed static clusters and are not yet available during DKP cluster installation.

- Container registry access parameters can now be [configured](https://deckhouse.io/modules/deckhouse/v1.75/configuration.html#parameters-registry) via the deckhouse module during DKP cluster installation.
  Support for the DKP cluster deployment via [Direct](https://deckhouse.io/modules/deckhouse/v1.75/configuration.html#parameters-registry-direct) and [Unmanaged](https://deckhouse.io/modules/deckhouse/v1.75/configuration.html#parameters-registry-unmanaged) modes has been added.
  The Legacy Unmanaged mode (via [InitConfiguration](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo)) is preserved for compatibility.
  If no explicit configuration is provided, the mode is selected automatically.

- Added user shell validation: if a shell other than bash is used, the `dhctl` utility now displays a clear error message.

- Added StaticInstance validation before DKP installation using the corresponding SSHCredentials,
  preventing `dhctl` from reporting readiness before the cluster deployment is actually complete.

- Extended preflight test coverage to improve cluster deployment predictability.

- Partially reduced usage of netcat in bashible, lowering the number of security system triggers.

- Fixed an issue where cluster installation could not be resumed after interruption.

- Fixed Node object deletion logic during `converge`.
  The node is now properly removed from the cluster after the corresponding VM is destroyed.

- Fixed the `--skip-resources` flag behavior in `dhctl destroy` command. Specified resources are now correctly skipped.

### Kubernetes & Scheduling subsystem

- A new `update-observer` component in the [`control-plane-manager`](https://deckhouse.io/modules/control-plane-manager/v1.75/) module enables real-time tracking of control plane component updates (kube-apiserver, kube-scheduler, kube-controller-manager) and kubelet on nodes. This helps detect discrepancies and eliminates "blind spots" during Kubernetes updates.

- For all VPA resources managed by DKP, `updateMode: Auto` has been automatically replaced with [`updateMode: InPlaceOrRecreate`](https://deckhouse.io/modules/vertical-pod-autoscaler/v1.75/cr.html#verticalpodautoscaler-v1-spec-updatepolicy-updatemode).
  For clusters running Kubernetes 1.32 or earlier, the `InPlacePodVerticalScaling` [feature gate](https://deckhouse.io/modules/control-plane-manager/v1.75/#feature-gates) is automatically enabled in control plane components (kube-apiserver, kube-scheduler, kube-controller-manager) and kubelet.
  This allows pod resource changes without recreation.

- The [descheduler](https://deckhouse.io/modules/descheduler/v1.75/) module has been updated to version 0.34. Key changes:
  - Priority-based pod eviction: pods with a higher restart count are evicted first, improving cluster stability during load redistribution.
  - DRA support: proper handling of resources managed via Dynamic Resource Allocation.
  - Default `cpu`, `memory`, and `pods` thresholds removed from [`lowNodeUtilization`](https://deckhouse.io/modules/descheduler/v1.75/cr.html#descheduler-v1alpha2-spec-strategies-lownodeutilization) and [`highNodeUtilization`](https://deckhouse.io/modules/descheduler/v1.75/cr.html#descheduler-v1alpha2-spec-strategies-highnodeutilization) strategies to align with upstream behavior.

### IAM subsystem

- Namespace listing behavior has been changed.
  When requesting all cluster namespaces, users now see only the namespaces they have access to, even without permission to list all namespaces.
  This improves working with multitenancy and UI.

- The ProjectTemplate resource in the `multitenancy-manager` module now supports the unmanaged and skip-heritage options (set via annotations),
  allowing administrators to adjust the DKP control degree over resources:
  - [`unmanaged`](https://deckhouse.io/modules/multitenancy-manager/v1.75/usage.html#excluding-resources-from-management-by-multitenancy-manager): The module does not manage the lifecycle of objects.
  - [`skip-heritage`](https://deckhouse.io/modules/multitenancy-manager/v1.75/usage.html#skipping-creation-of-the-heritage-multitenancy-manager-label): DKP retains management but allows modification by external controllers.

- Added support for authenticating to the Kubernetes API via LDAP using Basic authentication
  instead of OIDC tokens (via the [`ldap.enableBasicAuth`](https://deckhouse.io/modules/user-authn/v1.75/cr.html#dexprovider-v1-spec-ldap-enablebasicauth) parameter in the DexProvider resource).

- The authorization webhook in the [`user-authz`](https://deckhouse.io/modules/user-authz/v1.75/) module now operates in fail-closed mode: if it's unavailable, authorization is denied.
  For improved reliability, webhook communication with the apiserver is performed via the cluster node network.

- The `user-authn` module now supports a global user session lifetime limit (via [`refreshTokenAbsoluteLifetime`](https://deckhouse.io/modules/user-authn/v1.75/configuration.html#parameters-refreshtokenabsolutelifetime)).
  Re-authentication is required after expiration.

- The `user-authn` module now supports [Kerberos (SPNEGO) authentication](https://deckhouse.io/modules/user-authn/v1.75/cr.html#dexprovider-v1-spec-ldap-kerberos) in DKP.

- Updated kube-apiserver TLS cipher suites.
  Removed the following insecure ciphers from the `--tls-cipher-suites` flag: TLS_RSA_WITH_AES_256_GCM_SHA384 and TLS_RSA_WITH_AES_128_GCM_SHA256 (considered unsafe due to RSA key exchange).
  Added TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256 and TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256.

- The anonymous access restriction mechanism for the API server has been modified.
  Instead of using the `--anonymous-auth=false` flag and a sidecar proxy,
  the AuthenticationConfiguration resource is now used to allow anonymous access only to `/livez`, `/readyz`, and `/healthz`.
  This improves cluster resource efficiency. Anonymous probe access remains restricted when public API access is enabled via [`publishAPI.enabled`](https://deckhouse.io/modules/user-authn/v1.75/configuration.html#parameters-publishapi-enabled).

### Security subsystem

- Fixed serialization of empty arrays and objects in [SecurityPolicy](https://deckhouse.io/modules/admission-policy-engine/v1.75/cr.html#securitypolicy) and [OperationPolicy](https://deckhouse.io/modules/admission-policy-engine/v1.75/cr.html#operationpolicy) resources of the `admission-policy-engine` module.
  Previously, some settings could be applied incorrectly.

- Revised ModSecurity extension implementation.
  [OWASP Core Rule Set (CRS)](https://owasp.org/www-project-modsecurity-core-rule-set/) functionality is now available in all supported IngressNginxController versions.

### Cluster & Infrastructure subsystem

- [Lifecycle stages](https://deckhouse.io/products/kubernetes-platform/documentation/v1.75/architecture/module-development/versioning/#module-lifecycle) updated for cloud provider integration modules:
  - [`cloud-provider-dvp`](https://deckhouse.io/modules/cloud-provider-dvp/) (DVP), [`cloud-provider-vcd`](https://deckhouse.io/modules/cloud-provider-vcd/) (VMware Cloud Director), [`cloud-provider-huaweicloud`](https://deckhouse.io/modules/cloud-provider-huaweicloud/) (Huawei Cloud), and [`cloud-provider-zvirt`](https://deckhouse.io/modules/cloud-provider-zvirt/) (zVirt) modules are now General Availability (previously Experimental).

- Added support for swap memory usage by pods.
  Instead of immediate OOM-kill, a pod can temporarily use swap (if configured and available on the node) and continue running after peak usage.
  Requires cgroup v2 and sufficient node resources. For details, see the [`kubelet.memorySwap`](https://deckhouse.io/modules/node-manager/v1.75/cr.html#nodegroup-v1-spec-kubelet-memoryswap) parameter in the NodeGroup resource documentation.

- In the DVP cloud provider:
  - Added the [`networkPolicy`](https://deckhouse.io/modules/cloud-provider-dvp/v1.75/cluster_configuration.html#dvpclusterconfiguration-provider-networkpolicy) parameter to define network traffic rules for workloads within a Project resource.
  - Added `managed-by`, `cluster-uuid`, and `vm_name` labels to all infrastructure objects.
  - Improved CSI driver diagnostics. Diagnostic messages are now displayed directly in PersistentVolumeClaim status.
  - Implemented automatic cleanup of VMBDA objects when a VM is deleted.
  - Fixed an issue where DeckhouseMachine objects could remain in deleting state after VM removal.
  - Fixed an issue where Cloud Controller Manager (CCM) could recreate external load balancers after the associated Kubernetes Service was deleted.

- In Yandex Cloud, fixed a false NAT warning displayed during installation in configurations where NAT is not used.

- In the [`node-manager`](https://deckhouse.io/modules/node-manager/v1.75/) module:
  - Cluster API (CAPI) has been updated from 1.10.6 to 1.11.3, improving stability and provider compatibility.
  - Added a dedicated controller for NodeGroup hooks (extracted from DKP), reducing load on master nodes and improving responsiveness and UI feedback.
  - Added per-GPU MIG configuration support. You can now define a [distinct set of MIG partitions](https://deckhouse.io/modules/node-manager/v1.75/cr.html#nodegroup-v1-spec-gpu-mig-partedconfig) for each physical GPU on a node.
    Previously, MIG configuration was applied at the node level and identical across all GPUs.
    This enables more flexible workload resource allocation on a single node.

### Network subsystem

- In IngressNginxController, the `acceptClientIPHeadersFrom` parameter has been added for the following inlets: [HostPort](https://deckhouse.io/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-hostport-acceptclientipheadersfrom), [HostPortWithProxyProtocol](https://deckhouse.io/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-hostportwithproxyprotocol-acceptclientipheadersfrom), [LoadBalancer](https://deckhouse.io/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-loadbalancer-acceptclientipheadersfrom), and [LoadBalancerWithProxyProtocol](https://deckhouse.io/modules/ingress-nginx/v1.75/cr.html#ingressnginxcontroller-v1-spec-loadbalancerwithproxyprotocol-acceptclientipheadersfrom).
  The parameter defines trusted CIDR ranges for `X-Forwarded-*` headers that should not be overwritten during proxying.

- In the `cni-cilium` module, network observability capabilities have been extended:
  - Added extended metrics for monitoring interactions between specific objects (Pods, Services, Namespaces),
    configurable via the [`extendedMetrics`](https://deckhouse.io/modules/cni-cilium/v1.75/cr.html#hubblemonitoringconfig-v1alpha1-spec-extendedmetrics) parameter of the HubbleMonitoringConfig resource.
  - Implemented export of traffic flow logs for analyzing network interactions,
    configurable via the [`flowLogs`](https://deckhouse.io/modules/cni-cilium/v1.75/cr.html#hubblemonitoringconfig-v1alpha1-spec-flowlogs) parameter of the HubbleMonitoringConfig resource.

- Fixed an issue where, after VM migration in DVP, a LoadBalancer IP address could stop being announced when using MetalLB in BGP mode.

- [IngressNginxController](https://deckhouse.io/modules/ingress-nginx/v1.75/) version support changes:
  - Support for v1.9 has been discontinued.
  - Support for v1.14 has been added.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.75.md) on GitHub.

## Version 1.74

### Important

- Support for Kubernetes 1.34 has been added, while support for Kubernetes 1.29 has been discontinued.
  In future DKP releases, support for Kubernetes 1.30 will be removed.
  The default Kubernetes version (used when the [`kubernetesVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.74/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`) has been changed to [1.32](https://deckhouse.io/products/kubernetes-platform/documentation/v1.74/reference/supported_versions.html#kubernetes).

### Major changes

- Added support for managing experimental and alpha Kubernetes features ([feature gates](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/))
  using the [`enabledFeatureGates`](https://deckhouse.io/modules/control-plane-manager/v1.74/configuration.html#parameters-enabledfeaturegates) parameter of the `control-plane-manager` module.
  For details on managing these features in DKP, refer to the [module documentation](https://deckhouse.io/modules/control-plane-manager/v1.74/#feature-gates).

- The `--terminated-pod-gc-threshold` parameter (the termination threshold for evicted pods)
  is now calculated automatically based on the number of nodes in the cluster.
  This provides a balance between cluster stability and the ability to perform diagnostics.
  Threshold values are available in the [`control-plane-manager` module documentation](https://deckhouse.io/modules/control-plane-manager/v1.74/#dynamic-terminated-pod-garbage-collection-threshold).

- The maximum number of pods ([`maxPods`](https://deckhouse.io/modules/node-manager/v1.74/cr.html#nodegroup-v1-spec-kubelet-maxpods))
  for a node group is now set automatically based on the [`podSubnetNodeCIDRPrefix`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.74/reference/api/cr.html#clusterconfiguration-podsubnetnodecidrprefix) parameter from ClusterConfiguration (unless explicitly specified).
  This ensures stable cluster operation.

- Added the [`vipAddress`](https://deckhouse.io/modules/cloud-provider-huaweicloud/v1.74/cr.html#huaweicloudinstanceclass-v1-spec-vipaddress) parameter
  to the HuaweiCloudInstanceClass resource of the Huawei Cloud provider,
  allowing you to specify a virtual IP address for all nodes in the instance class.

- The Huawei Cloud provider configuration now allows overriding the primary network for the main network interface ([`mainNetwork`](https://deckhouse.io/modules/cloud-provider-huaweicloud/v1.74/cluster_configuration.html#huaweicloudclusterconfiguration-nodegroups-instanceclass-mainnetwork)) and specifying subnets for additional network interfaces ([`additionalNetworks`](https://deckhouse.io/modules/cloud-provider-huaweicloud/v1.74/cluster_configuration.html#huaweicloudclusterconfiguration-nodegroups-instanceclass-additionalnetworks)) on both CloudPermanent and CloudEphemeral nodes.

  **Warning**. When upgrading to DKP 1.74 on Huawei Cloud, a bulk recreation of CloudEphemeral nodes may occur. Due to a temporary node shortage, some Pods may fail to start.

- The archive with debugging data is now collected using the [`d8`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.74/cli/d8/) tool.
  If you need to collect diagnostic data, follow the [instructions](https://deckhouse.io/modules/deckhouse/v1.74/faq.html#how-to-collect-debug-info).

- Now when hook execution fails, all metrics collected by that hook are preserved.
  This prevents loss of debugging information and simplifies the following root-cause analysis.

- Fixed a Deckhouse queue freeze in hybrid clusters that occurred when module configuration parameters were incomplete.

### Security

- Implemented module integrity control based on the EROFS filesystem to improve security and prevent unauthorized modifications.
  The new system requires kernel support for dm-verity and EROFS.

### Network

- Added support for the SCTP (Stream Control Transmission Protocol).
  Applications using this protocol can now run in a Deckhouse cluster.

- Added a `geoproxy` auxiliary microservice to the [`ingress-nginx`](https://deckhouse.io/modules/ingress-nginx/) module.
  This service is aimed at improving the stability of the Ingress NGINX Controller when working with GeoIP databases
  and provides the following features:
  - MaxMind license saving (databases are downloaded from a single point once a day).
  - Persistent data storage (if components are restarted, it doesn't require accessing the MaxMind servers).
  - Lets you specify a custom mirror for downloading databases.

- Added the Prometheus metric `bpf_progs_complexity_max_verified_insts` to assess the number of instructions in eBPF programs
  loaded into the kernel on cluster nodes.
  This metric helps evaluate the compatibility of the networking subsystem with the node kernels.
  The metric requires Linux kernel version 5.16 or newer.

- Added a new metric (`geoip_version`) and dashboards for monitoring GeoIP functionality in the cluster.

- Fixed issues with simultaneous updates of the [`node-local-dns`](https://deckhouse.io/modules/node-local-dns/) module
  and the networking subsystem.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.74.md) on GitHub.

## Version 1.73

### Important

- This release includes several important security improvements. Multiple known vulnerabilities have been fixed, including one in the user-authn module (CVE-2025-22868) that could potentially allow bypassing authentication checks. It is recommended that you schedule this update. See the [Security](#security) section for details.

- The `dashboard` module will be removed in future DKP versions. Use the [Deckhouse web UI](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/user/web/ui.html) instead (requires the [`console`](https://deckhouse.io/modules/console/) module to be enabled).

- The `runtime-audit-engine` module is now loaded from an external source (the `deckhouse` ModuleSource).

- All DKP components will be restarted during the update.

### Major changes

- Added support for changing the registry settings (via the [`mode`](https://deckhouse.io/modules/deckhouse/v1.73/configuration.html#parameters-registry-mode) parameter) in `Unmanaged` mode, similar to `Direct` mode. A new parameter [`checkMode`](https://deckhouse.io/modules/deckhouse/v1.73/configuration.html#parameters-registry-direct-checkmode) has been added to control registry validation behavior:
  - `Default`: Verifies the availability of all system module images and the deckhouse-controller images.
  - `Relax`: Verifies only the deckhouse-controller images.

- Updated requirements for the `email` field of the [User](https://deckhouse.io/modules/user-authn/v1.73/cr.html#user) object: the email address must now be in lowercase. Existing users will not be affected.

- Dex updated to **v2.44.0**. It now allows authentication through available identity providers if one of them is down, and supports authentication via identity providers through a proxy.

- The [User](https://deckhouse.io/modules/user-authn/v1.73/cr.html#user) object status now displays the reason for user lockout (controlled by the [`lockout`](https://deckhouse.io/modules/user-authn/v1.73/configuration.html#parameters-passwordpolicy-lockout) parameter).

- Added the [`additionalDisks`](https://deckhouse.io/modules/cloud-provider-dvp/v1.73/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-additionaldisks) parameter for the Deckhouse Virtualization Platform integration provider, allowing creation and attachment of additional disks to VMs in a NodeGroup (`size` and StorageClass must be specified). This simplifies data distribution across multiple disks.

- Added support for [`additionalMetadata`](https://deckhouse.io/modules/cloud-provider-vcd/v1.73/cr.html#vcdinstanceclass-v1-spec-additionalmetadata) in objects (networks, VMs, disks) for the VMware Cloud Director integration provider. Metadata is merged with existing data, with `additionalMetadata` values taking precedence. Changing `additionalMetadata` triggers recreation of CloudEphemeral node groups that use the affected VCDInstanceClass.

- For the VMware vSphere integration provider, you can now specify an SPBM storage policy ID (via the [`storagePolicyID`](https://deckhouse.io/modules/cloud-provider-vsphere/v1.73/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) parameter) and configure automatic creation of a StorageClass for each available SPBM policy. You can now explicitly select a policy for master and worker nodes and use the corresponding storage classes.

- Added alerts to help plan module deprecation or migration:
  - [`ModuleIsDeprecated`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73//reference/alerts.html#monitoring-deckhouse-moduleisdeprecated): Notifies when a module is deprecated and nearing end of support.
  - [`D8ModuleOutdatedByMajorVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73//reference/alerts.html#monitoring-deckhouse-d8moduleoutdatedbymajorversion): Notifies when a module is behind by one or more major versions.

- Added [`GeoIPDownloadErrorDetected`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/reference/alerts.html#ingress-nginx-geoipdownloaderrordetected) alert to notify about MaxMind GeoIP database download issues.

- The [update notification workflow](https://deckhouse.io/modules/deckhouse/v1.73/usage.html#deckhouse-update-notifications) has changed — a release is applied only after the notification is successfully delivered to the configured webhook. If delivery fails, the update is paused until the webhook is restored.

- Reorganized in-cluster documentation. All module documentation (including connected ones) is now located under the [Modules section](https://deckhouse.io/modules/). Search has been updated.

- For Ingress NGINX Controller v1.10, added the option to enable the profiler (via the [`nginxProfilingEnabled`](https://deckhouse.io/modules/ingress-nginx/v1.73/cr.html#ingressnginxcontroller-v1-spec-nginxprofilingenabled) parameter). Enabling the profiler increases resource consumption but may be useful for debugging controller issues.

- Added support for custom HTTP authentication headers (via the [`headers`](https://deckhouse.io/modules/upmeter/v1.73/cr.html#upmeterremotewrite-v1-spec-config-headers) parameter of UpmeterRemoteWrite) when sending SLA monitoring metrics via Prometheus Remote Write protocol.

- Optimized DKP core module loading, reducing startup time in clusters with many modules.

- Audit logs now show which OIDC provider issued the authentication token

- Deckhouse CLI (`d8`) updated to v0.23.0:
  - Added the [`d8 status`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-status) command, which provides a quick cluster summary (nodes, releases, Deckhouse pods, alerts, registry, Deckhouse settings, CNI, queue state).
  - Added the [`d8 k debug`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-k-debug) command, which runs the DKP built-in debug container (image can be overridden via `--image`). This simplifies interactive pod debugging.
  - Added the `--watch` flag to the [`d8 system queue list`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.73/cli/d8/reference/#d8-system-queue-list) command to track queue state changes.

### Security

- Improved baseline container security. Updated security profiles for the following modules to restrict privileges and access rights to the minimum required: `cni-cilium`, `cni-flannel`, `cni-simple-bridge`, `ingress-nginx`, `istio`, `keepalived`, `kube-dns`, `kube-proxy`, `node-local-dns`, `network-gateway`, `network-policy-engine`, `open-vpn`.

- Added the [`allowRbacWildcards`](https://deckhouse.io/modules/admission-policy-engine/v1.73/cr.html#securitypolicy-v1alpha1-spec-policies-allowrbacwildcards) flag to the SecurityPolicy, controlling whether wildcards are allowed in Role and RoleBinding definitions (set to `true` by default). Security policies can now also restrict interactive connections to pods (`CONNECT` for `pods/exec` and `pods/attach`) within namespaces.

- Added support for preventing creation of pods with specific tolerations from a list ([`policies.disallowedTolerations`](https://deckhouse.io/modules/admission-policy-engine/v1.73/cr.html#operationpolicy-v1alpha1-spec-policies-disallowedtolerations) parameter in the operational policy). This helps prevent user workloads from running on nodes reserved for special tasks.

- Enhanced security in Ingress NGINX Controller v1.12 (distroless image, vulnerability fixes, and other improvements).

- Fixed known vulnerabilities in the following modules: `operator-trivy`, `registry`, `user-authn`, `cloud-provider-dvp`, `multitenancy-manager`, `admission-policy-engine`, `ingress-nginx`, `alertmanager`, `metallb`, `istio`, `node-local-dns`, `kube-apiserver`.

### Network

- For the VMware Cloud Director integration provider, added support for [LoadBalancer](https://deckhouse.io/modules/cloud-provider-vcd/v1.73/environment.html#using-the-loadbalancer) backed by VMware NSX Advanced Load Balancer (ALB/Avi) when using `NSX-T`. Requires the Load Balancer feature to be enabled on the Edge Gateway. If LoadBalancer is enabled after cluster creation, DKP automatically detects and applies the change within an hour. For open ports, DKP creates corresponding `Pool + Virtual Service` pairs. If there is a firewall, add allow rules for the LoadBalancer’s external IP address and relevant ports.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.73.md) on GitHub.

## Version 1.72

### Important

- All DKP components will be restarted during the update.
- To use [experimental modules](https://deckhouse.io/products/kubernetes-platform/documentation/v1.72/architecture/module-development/versioning/#module-lifecycle) in the cluster, you now need to explicitly enable the [allowExperimentalModules](https://deckhouse.io/modules/deckhouse/configuration.html#parameters-allowexperimentalmodules) parameter. By default, experimental modules are disabled. Modules that were enabled before the update will not be automatically disabled. However, if an experimental module enabled prior to the update is manually disabled during the update process, you will need to grant permission to use experimental modules again in order to re-enable it.
- If there are WireGuard interfaces on the cluster nodes, you must update the Linux kernel to version 6.8 or higher.

### Major changes

- Added a new [registry module](https://deckhouse.io/modules/registry/) and the ability to adjust container registry parameters without restarting all DKP components. [Two modes](https://deckhouse.io/modules/deckhouse/configuration.html#parameters-registry-mode) for working with the container registry are now available in DKP: `Unmanaged` (the approach used in previous versions) and `Direct` (a new mode). In `Direct` mode, DKP creates a virtual container registry address in the cluster that all DKP components use. Changing the container registry address (for example, switching to a different registry or changing the DKP edition in the cluster) in this mode does not trigger a forced restart of all DKP components.

- Added support for recursive DNS servers (configured via the [recursiveSettings](https://deckhouse.io/modules/cert-manager/configuration.html#parameters-recursivesettings) section of the `cert-manager` module). They are used to verify the existence of a DNS record before starting the ACME DNS-01 domain ownership validation process. This is useful if the same domain is used both publicly and within the cluster, or if the domain has dedicated authoritative DNS servers.

- Introduced separation of modules into critical and functional using the `critical` flag in `module.yaml`. Critical modules are started first. Functional modules are started after the bootstrap process is complete. Their tasks run in parallel and do not block the queue in case of failure. This speeds up cluster installation and improves fault tolerance when starting modules.

- You can now enable logging of all DNS queries (the [enableLogs](https://deckhouse.io/modules/node-local-dns/configuration.html#parameters-enablelogs) parameter of the `node-local-dns` module).

- In the `cloud-provider-vcd` module, a new [WithNAT layout](https://deckhouse.io/modules/cloud-provider-vcd/layouts.html#withnat) has been added for cluster deployment. It automatically configures NAT and, if necessary, firewall rules for accessing nodes through a bastion host. It also supports both `NSX-T` and `NSX-V`. This makes it possible to deploy a cluster in VMware Cloud Director without pre-configuring the environment (unlike the `Standard` layout).

### Security

- Added the fields `user-authn.deckhouse.io/name` and `user-authn.deckhouse.io/preferred_username` to [Kubernetes audit log events](https://deckhouse.io/modules/control-plane-manager/#auditing). These fields display user claims from the OIDC provider, improving authentication monitoring and troubleshooting.

- Kubernetes versions 1.30–1.33 have been updated to the latest patch releases.

- For the AWS provider, added the ability to disable the creation of default security groups (the [disableDefaultSecurityGroup](https://deckhouse.io/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-disabledefaultsecuritygroup) parameter). When disabled, security groups must be created manually and explicitly specified in AWSClusterConfiguration, AWSInstanceClass, and NodeGroup. This new feature provides greater control over security settings.

- Added support for password policies for local users (configured in the [passwordPolicy](https://deckhouse.io/modules/user-authn/configuration.html#parameters-passwordpolicy) section). You can now enforce a minimum password complexity, set password expiration, require password rotation, prevent reuse of old passwords, and lock accounts after a specified number of failed login attempts. These changes allow administrators to centrally enforce password requirements and improve cluster security.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.72.md) on GitHub.

## Version 1.71

### Important

- Prometheus has been replaced with Deckhouse Prom++. If you want to keep using Prometheus, disable the `prompp` module manually before upgrading DKP by running the command `d8 system module disable prompp`.

- Support for Kubernetes 1.33 has been added, while support for Kubernetes 1.28 has been discontinued. In future DKP releases, support for Kubernetes 1.29 will be removed. The default Kubernetes version (used when the [`kubernetesVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/installing/configuration.html#clusterconfiguration-kubernetesversion) parameter is set to `Automatic`) has been changed to [1.31](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/supported_versions.html#kubernetes).

- Upgrading the cluster to Kubernetes 1.31 requires a sequential update of all nodes, with each node drained. You can control how node updates requiring workload disruptions are applied using the [`disruptions`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions) parameter section.

- The built-in `snapshot-controller` and `static-routing-manager` modules will now be replaced with their external counterparts of the same name, sourced via ModuleSource `deckhouse`.

- The new version of Cilium requires nodes to run Linux kernel version 5.8 or newer. If any node in the cluster has a kernel older than 5.8, the Deckhouse Kubernetes Platform upgrade will be blocked. Cilium Pods will be restarted.

- All DKP components will be restarted during the update.

### Major changes

- You can now enforce two-factor authentication for static users. This is configured via the [`staticUsers2FA`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/user-authn/configuration.html#parameters-staticusers2fa) parameter section of the `user-authn` module.

- Added support for GPUs on nodes. Three GPU resource sharing modes are now available: Exclusive (no sharing), TimeSlicing (time-based sharing), and MIG (a single GPU split into multiple instances). The NodeGroup [spec.gpu](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-gpu) parameter section is used to configure the GPU resource sharing mode. Using a GPU on a node requires installing the NVIDIA Container Toolkit and the GPU driver.

- When enabling a module (with `d8 system module enable`) or editing a ModuleConfig resource, a warning is now displayed if multiple module sources are found. In such a case, explicitly specify the module source using the [source](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/cr.html#moduleconfig-v1alpha1-spec-source) parameter in the module’s configuration.

- Improved error handling for module configuration. Module-related errors no longer block DKP operations. Instead, they are now displayed in the status fields of Module and ModuleRelease objects.

- Improved virtualization support:
  - Added a provider for integration with [Deckhouse Virtualization Platform (DVP)](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/cloud-provider-dvp/), enabling deployment of DKP clusters on top of DVP.
  - Added support for nested virtualization on nodes in the `cni-cilium` module.

- The `node-manager` module now includes several enhancements for improved node reliability and manageability:
  - You can now prevent a node from restarting if it still hosts critical Pods (labeled with `pod.deckhouse.io/inhibit-node-shutdown`). This can be necessary for workloads with stateful components, such as long-running data migrations.
  - Introduced API version `v1alpha2` for the [SSHCredential](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#sshcredentials) resource, where the [`sudoPasswordEncoded`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#sshcredentials-v1alpha2-spec-sudopasswordencoded) parameter allows specifying the `sudo` password in Base64 format.
  - The [`capiEmergencyBrake`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/configuration.html#parameters-capiemergencybrake) parameter allows you to disable Cluster API (CAPI) in emergency scenarios, preventing potentially destructive changes. Its behavior is similar to the existing [`mcmEmergencyBrake`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/configuration.html#parameters-mcmemergencybrake) setting.

- Added a pre-installation check to verify connectivity to the DKP container image registry.

- Improved the log file rotation mechanism when using short-term log storage (via the `loki` module). Added the [`LokiInsufficientDiskForRetention`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/alerts.html#loki-lokiinsufficientdiskforretention) alert to warn about insufficient disk space for log retention.

- The documentation now includes a [reference for the Deckhouse CLI](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/deckhouse-cli/reference/) (`d8` utility) commands and parameters.

- When using CEF encoding for collecting logs from [Apache Kafka](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-kafka-encoding-cef) or [socket](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/log-shipper/cr.html#clusterlogdestination-v1alpha1-spec-socket-encoding-cef) sources, you can now configure auxiliary CEF fields such as Device Product, Device Vendor, and Device ID.

- The [`passwordHash`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodeuser-v1-spec-passwordhash) field in the NodeUser resource is no longer required. This allows you to create users without passwords — for example, in clusters that use external authentication systems (such as PAM or LDAP).

- Added support for CRI Containerd v2 with CgroupsV2. The new version introduces a different configuration format and includes a mechanism to migrate between Containerd v1 and v2. You can change the CRI type used on nodes via the [`cri.type`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) parameter and configure it using [`cri.containerdV2`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/node-manager/cr.html#nodegroup-v1-spec-cri-containerdv2).

### Security

- [Container image signature verification](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) is now available in DKP SE+. This feature is now supported in DKP SE+ and EE.

- The `log-shipper`, `deckhouse-controller`, and `Istio` (version 1.21) modules have been migrated to distroless builds. This improves security and ensures a more transparent and controlled build process.

- New audit rules have been added to track interactions with containerd. The following are now monitored: access to the `/run/containerd/containerd.sock` socket, modifications to the `/etc/containerd` and `/var/lib/containerd` directories and the `/opt/deckhouse/bin/containerd` file.

- Known vulnerabilities have been fixed in the following modules: `loki`, `extended-monitoring`, `operator-prometheus`, `prometheus`, `prometheus-metrics-adapter`, `user-authn`, and `cloud-provider-zvirt`.

### Network

- Added support for Istio version 1.25.2, which uses the Sail operator instead of the deprecated Istio Operator. Also added support for Kiali version 2.7, without Ambient Mesh support. Istio version 1.19 is now considered deprecated.

- Added support for encrypting traffic between nodes and Pods using the WireGuard protocol (via the [`encryption.mode`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/cni-cilium/configuration.html#parameters-encryption-mode) parameter).

- Fixed the logic for determining service readiness in the [ServiceWithHealthcheck](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/service-with-healthchecks/cr.html#servicewithhealthchecks) resource. Previously, Pods without an IP address (for example, in `Pending` state) could be mistakenly included in the load balancing list.

- Added support for the least-conn load balancing algorithm. This algorithm directs traffic to the service backend with the fewest active connections, improving performance for connection-heavy applications (such as WebSocket services). To use this algorithm, enable the [`extraLoadBalancerAlgorithmsEnabled`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/modules/cni-cilium/configuration.html#parameters-extraloadbalanceralgorithmsenabled) parameter in the `cni-cilium` module settings and use the `service.cilium.io/lb-algorithm` annotation on the service and set it to a supported value: random, maglev, or least-conn.

- Fixed an issue in Cilium 1.17 `cilium-operator` where IP addresses were not reused after a `CiliumEndpoint` was deleted. The issue was caused by improper cleanup of priority filters, which could lead to IP pool exhaustion in large clusters.

- Refined the [list of ports used for networking](https://deckhouse.io/products/kubernetes-platform/documentation/v1.71/network_security_setup.html):
  - Added and updated:
    - `4287/UDP`: WireGuard port used for CNI Cilium traffic encryption.
    - `4295-4297/UDP`: Used by the `cni-cilium` module for VXLAN encapsulation of inter-pod traffic in multiple nested virtualization — when DKP with the `virtualization` module enabled is deployed inside virtual machines that are also created in DKP with the `virtualization` module enabled.
    - `4298/UDP`: Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods if the cluster was deployed on DKP version starting from **1.71** (for clusters deployed on DKP versions up to **1.71**, see the note for ports `4299/UDP`, `8469/UDP`, and `8472/UDP`).
    - `4299/UDP`: Port **for clusters deployed on DKP versions 1.64–1.70.** Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods. Updating DKP to newer versions will not change the port used unless the `virtualization` module is enabled.
    - `8469/UDP`: Port **for clusters deployed on DKP version 1.63 and below with the `virtualization` module enabled prior to DKP version 1.63.** Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods. Updating DKP to newer versions will not change the occupied port
    - `8472/UDP`: Port **for clusters deployed on DKP version 1.63 and below.** Used by the `cni-cilium` module for VXLAN encapsulation of traffic between pods. Updating DKP to newer versions will not change the occupied port if the `virtualization` module is not enabled. **Note** that in such clusters, enabling the `virtualization` module on DKP before version 1.70 changes the port:
      - Enabling the `virtualization` module on DKP version 1.63 and below will change it to `8469/UDP` and will not change with subsequent DKP updates
      - Enabling the `virtualization` module on DKP starting from version 1.64 will change it to `4298/UDP` and will not change with subsequent DKP updates
  - Removed:
    - `49152`, `49153/TCP`: Previously used for live migration of virtual machines (in the `virtualization` module). Migration now occurs over the Pod network.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.71.md) on GitHub.

## Version 1.70

### Important

- The `ceph-csi` module has been removed. Use the `csi-ceph` module instead. Deckhouse will not be updated as long as `ceph-csi` is enabled in the cluster. For `csi-ceph` migration instructions, refer to the [module documentation](https://deckhouse.io/products/kubernetes-platform/modules/csi-ceph/stable/).

- Version 1.12 of the Ingress NGINX Controller has been added. The default controller version has been changed to 1.10. All Ingress controllers that do not have an explicitly specified version (via the [`controllerVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerversion) parameter in the IngressNginxController resource or the [`defaultControllerVersion`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/configuration.html#parameters-defaultcontrollerversion) parameter in the `ingress-nginx` module) will be restarted.

- The `falco_events` metric (from the `runtime-audit-engine` module) has been removed. The `falco_events` metric was considered deprecated since DKP 1.68. Use the [`falcosecurity_falcosidekick_falco_events_total`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/runtime-audit-engine/faq.html#how-to-create-an-alert) metric instead. Dashboards and alerts based on the `falco_events` metric may stop working.

- All DKP components will be restarted during the update.

### Major changes

- In the `Auto` [update mode](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/configuration.html#parameters-update-mode), patch version updates (for example, from `v1.70.1` to `v1.70.2`) are now applied taking into account the update windows, if they are set. Previously, in this update mode, only minor version updates (for example, from `v1.69.x` to `v1.70.x`) were applied with consideration to update windows, while patch version updates were applied as they appeared on a release channel.
- A node can now be rebooted if the corresponding Node object has the `update.node.deckhouse.io/reboot` annotation set.
- When cleaning up a static node, any local users created by Deckhouse Kubernetes Platform are now also removed.
- Added synchronization monitoring for Istio in multi-cluster configurations. A new alert [`D8IstioRemoteClusterNotSynced`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#istio-d8istioremoteclusternotsynced) has been introduced and triggers in the following cases:
  - The remote cluster is offline.
  - The remote API endpoint is not reachable.
  - The remote `ServiceAccount` token is invalid or expired.
  - There is a TLS or certificate issue between the clusters.

- The `deckhouse-controller collect-debug-info` command now also collects [debug information](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/deckhouse/faq.html#how-to-collect-debug-info) for `Istio`, including:
  - Resources in the `d8-istio` namespace.
  - CRDs from the `istio.io` and `gateway.networking.k8s.io` groups.
  - `Istio` logs.
  - `Sidecar` logs of a single randomly selected user application.

- A new monitoring dashboard has been added to display OpenVPN certificate status. Upon expiration, server certificates will now be reissued, and client certificates will be removed. The following alerts have been added: \
  - [`OpenVPNClientCertificateExpired`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnclientcertificateexpired): Warns about expired client certificates.
  - [`OpenVPNServerCACertificateExpired`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpired): Warns about an expired OpenVPN CA certificate.
  - [`OpenVPNServerCACertificateExpiringSoon`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpiringsoon) and [`OpenVPNServerCACertificateExpiringInAWeek`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercacertificateexpiringinaweek): Warn when an OpenVPN CA certificate is expiring in less than 30 or 7 days, respectively.
  - [`OpenVPNServerCertificateExpired`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpired): Warns about an expired OpenVPN server certificate.
  - [`OpenVPNServerCertificateExpiringSoon`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpiringsoon) and [`OpenVPNServerCertificateExpiringInAWeek`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/alerts.html#openvpn-openvpnservercertificateexpiringinaweek): Warn when an OpenVPN server certificate is expiring in less than 30 or 7 days, respectively.

- Monitoring dashboards have been renamed and updated:
  - "L2LoadBalancer" renamed to "MetalLB L2"; pool and column filtering added.
  - "Metallb" renamed to "MetalLB BGP"; pool and column filtering added. The ARP request panel has been removed.
  - "L2LoadBalancer / Pools" renamed to "MetalLB / Pools".

- The `upmeter` module’s PVC size has been increased to accommodate data retention for 13 months. In some cases, the previous PVC size was insufficient.

- The [ModuleSource](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/cr.html#modulesource) resource status now includes information about module versions in the source.

- The [Module](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/cr.html#module) resource status now includes information about the module’s lifecycle stage. A module can move through the following stages in its lifecycle: Experimental, Preview, General Availability, and Deprecated. For details on module lifecycle stages and how to evaluate its stability, refer to the [corresponding section in the documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/module-development/versioning/#how-do-i-figure-out-how-stable-a-module-is).

- It is now possible to use stronger or more modern encryption algorithms (such as `RSA-3072`, `RSA-4096`, or `ECDSA-P256`) for control plane cluster certificates instead of the default `RSA-2048`. You can use the [`encryptionAlgorithm`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/installing/configuration.html#clusterconfiguration-encryptionalgorithm) parameter in the ClusterConfiguration resource to configure this.

- The `descheduler` module can now be configured to evict pods that are using local storage. Use the [`evictLocalStoragePods`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/descheduler/cr.html#descheduler-v1alpha2-spec-evictlocalstoragepods) parameter in the module configuration to adjust this.

- You can now adjust the logging level of the Ingress controller using the [`controllerLogLevel`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.70/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-controllerloglevel) parameter in the IngressNginxController resource. The default log level is `Info`. Controlling the logging level can help prevent log collector overload during Ingress controller restarts.

### Security

- The severity level of alerts indicating security policy violations has been raised from 7 to 3.

- The configuration for `Yandex Cloud`, `Zvirt`, and `Basis Dynamix` providers now uses `OpenTofu` instead of `Terraform`. This enables easier provider updates, such as applying fixes for known vulnerabilities (CVEs).

- CVE vulnerabilities have been fixed in the following modules: `chrony`, `descheduler`, `dhctl`, `node-manager`, `registry-packages-proxy`, `falco`, `cni-cilium`, and `vertical-pod-autoscaler`.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.70.md) on GitHub.

## Version 1.69

### Important

- Support for Kubernetes 1.32 has been added, while support for Kubernetes 1.27 has been discontinued.
  The default Kubernetes version has been changed to [1.30](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/supported_versions.html#kubernetes).
  In future DKP releases, support for Kubernetes 1.28 will be removed.

- All DKP components will be restarted during the update.

### Major changes

- The `ceph-csi` module is now deprecated.
  Plan to migrate to the [`csi-ceph`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/reference/mc/csi-ceph/) module instead.
  For details, refer to the [Ceph documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/storage/admin/external/ceph.html).

- You can now grant access to Deckhouse web interfaces using user names via the `auth.allowedUserEmails` field.
  Access restriction is configured together with the `auth.allowedUserGroups` parameter
  in configuration of the following modules with web interfaces: `cilium-hubble`, `dashboard`, `deckhouse-tools`,
  `documentation`, `istio`, `openvpn`, `prometheus`, and `upmeter` ([example for `prometheus`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/prometheus/configuration.html#parameters-auth-alloweduseremails)).

- A new dashboard **Cilium Nodes Connectivity Status & Latency** has been added to Grafana in the `cni-cilium` module.
  It helps monitor node network connectivity issues.
  The dashboard displays a connectivity matrix similar to the `cilium-health status` command,
  using metrics that are already available in Prometheus.

- A new [`D8KubernetesStaleTokensDetected`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/alerts.html#control-plane-manager-d8kubernetesstaletokensdetected) alert has been added in the `control-plane-manager` module
  that is triggered when stale service account tokens are detected in the cluster.

- You can now create a Project from an existing namespace and adopt existing objects into it.
  To do this, annotate the namespace and its resources with `projects.deckhouse.io/adopt`.
  This lets you switch to using Projects without recreating cluster resources.

- A `Terminating` status has been added to ModuleSource and ModuleRelease resources.
  The new status will be displayed when an attempt to delete one of them fails.

- The installer container now automatically configures cluster access after a successful bootstrap.
  A `kubeconfig` file is generated in `~/.kube/config`, and a local TCP proxy is set up through an SSH tunnel.
  This allows you to use kubectl locally right away without manually connecting to the control-plane node via SSH.

- Changes to Kubernetes resources in multi-cluster and federation setups are now tracked directly via Kubernetes API.
  This enables faster synchronization between clusters and eliminates the use of outdated certificates.
  In addition, mounting of ConfigMap and Secret resources into Pods has been removed
  to eliminate family system compromise risks.

- A new [dynamicforward](https://github.com/coredns/coredns/pull/7105) plugin has been added to CoreDNS, improving DNS query processing in the cluster.
  It integrates with `node-local-dns`, continuously monitors `kube-dns` endpoints,
  and automatically updates the list of DNS forwarders.
  If the control-plane node is unavailable,
  DNS queries are still forwarded to available endpoints, improving cluster stability.

- A new log rotation approach has been introduced in the `loki` module.
  Now, old logs are automatically removed when disk usage exceeds a threshold:
  either 95% of PVC size or PVC size minus the size required to store two minutes of log data
  at the configured ingestion rate ([`ingestionRateMB`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/loki/configuration.html#parameters-lokiconfig-ingestionratemb)).
  The [`retentionPeriodHours`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/loki/configuration.html#parameters-retentionperiodhours) parameter no longer controls the data retention and is used for monitoring alerts only.
  If `loki` begins removing old logs before the set period is reached,
  a [`LokiRetentionPerionViolation`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/alerts.html#loki-lokiretentionperionviolation) alert will be triggered,
  informing the user that they must reduce the value of `retentionPeriodHours` or increase the PVC size.

- A new [`nodeDrainTimeoutSecond`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/node-manager/cr.html#nodegroup-v1-spec-nodedraintimeoutsecond) parameter lets you set the maximum timeout
  when attempting to drain a node (in seconds) for each NodeGroup resource.
  Previously, you could only use the default value (10 minutes)
  or reduce it to 5 minutes using the `quickShutdown` parameter, which is now deprecated.

- The `openvpn` module now includes a [`defaultClientCertExpirationDays`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.69/modules/openvpn/configuration.html#parameters-clientcertexpirationdays) parameter,
  allowing you to define the lifetime of client certificates.

### Security

- Known vulnerabilities have been addressed in the following modules:
`ingress-nginx`, `istio`, `prometheus`, and `local-path-provisioner`.

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.69.md) on GitHub.

## Version 1.68

### Important

- After the update,
  the UID will change for all Grafana data sources created using the GrafanaAdditionalDatasource resource.
  If a data source was referenced by UID, that reference will no longer be valid.

### Major changes

- A new parameter, [`iamNodeRole`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-iamnoderole),
  has been introduced for the AWS provider.
  It lets you specify the name of the IAM role to bind to all AWS instances of cluster nodes.
  This can come in handy if you need to grant additional permissions (for example, access to ECR, etc.).

- Creating nodes of the [CloudPermanent type](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype)
  now takes less time.
  Now, CloudPermanent nodes are created in parallel.
  Previously, they were created in parallel only within a single group.

- Monitoring changes:
  - Support for monitoring certificates in secrets of the `Opaque` type has been added.
  - Support for monitoring images in Amazon ECR has been added.
  - A bug that could cause partial loss of metrics when Prometheus instances were restarted has been fixed.

- When using a multi-cluster Istio configuration or federation,
  you can now explicitly specify the list of addresses used for inter-cluster requests.
  Previously, these addresses were determined automatically;
  however, in some configurations, they could not be resolved.

- The DexAuthenticator resource now has a [`highAvailability`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/user-authn/cr.html#dexauthenticator-v1-spec-highavailability) parameter
  that controls high availability mode.
  In high availability mode, multiple replicas of the authenticator are launched.
  Previously, high availability mode of all authenticators was determined by a [global parameter](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/deckhouse-configure-global.html#parameters-highavailability)
  or by the `user-authn` module.
  All authenticators deployed by DKP now inherit the high availability mode of the corresponding module.

- Node labels can now be added, removed, or modified
  using files stored on the node in the `/var/lib/node_labels` directory and its subdirectories.
  The full set of applied labels is stored in the `node.deckhouse.io/last-applied-local-labels` annotation.

- Support for the [Huawei Cloud provider](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/cloud-provider-huaweicloud/) has been added.

- The new [`keepDeletedFilesOpenedFor`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/log-shipper/cr.html#clusterloggingconfig-v1alpha1-spec-kubernetespods-keepdeletedfilesopenedfor) parameter
  in the `log-shipper` module allows you to configure the period to keep the deleted log files open.
  This way, you can continue reading logs from deleted pods for some time if log storage is temporarily unavailable.

- TLS encryption for log collectors (Elasticsearch, Vector, Loki, Splunk, Logstash, Socket, Kafka)
  can now be configured using secrets, rather than by storing certificates in the ClusterLogDestination resources.
  The secret must reside in the `d8-log-shipper` namespace and have the `log-shipper.deckhouse.io/watch-secret: true` label.

- In the [project](https://deckhouse.io/products/kubernetes-platform/documentation/v1.68/modules/multitenancy-manager/cr.html#project) status under the `resources` section,
  you can now see which project resources have been installed.
  Those resources are marked with `installed: true`.

- A new parameter, `--tf-resource-management-timeout`, has been added to the installer.
  It controls the resource creation timeout in cloud environments.
  By default, the timeout is set to 10 minutes.
  This parameter applies only to the following clouds: AWS, Azure, GCP, OpenStack.

### Security

Known vulnerabilities have been addressed in the following modules:

- `admission-policy-engine`
- `chrony`
- `cloud-provider-azure`
- `cloud-provider-gcp`
- `cloud-provider-openstack`
- `cloud-provider-yandex`
- `cloud-provider-zvirt`
- `cni-cilium`
- `control-plane-manager`
- `extended-monitoring`
- `descheduler`
- `documentation`
- `ingress-nginx`
- `istio`
- `loki`
- `metallb`
- `monitoring-kubernetes`
- `monitoring-ping`
- `node-manager`
- `operator-trivy`
- `pod-reloader`
- `prometheus`
- `prometheus-metrics-adapter`
- `registrypackages`
- `runtime-audit-engine`
- `terraform-manager`
- `user-authn`
- `vertical-pod-autoscaler`
- `static-routing-manager`

### Mandatory component restart

The following components will be restarted after updating DKP to 1.68:

- Kubernetes Control Plane
- Ingress controller
- Prometheus, Grafana
- `admission-policy-engine`
- `chrony`
- `cloud-provider-azure`
- `cloud-provider-gcp`
- `cloud-provider-openstack`
- `cloud-provider-yandex`
- `cloud-provider-zvirt`
- `cni-cilium`
- `control-plane-manager`
- `descheduler`
- `documentation`
- `extended-monitoring`
- `ingress-nginx`
- `istio`
- `kube-state-metrics`
- `log-shipper`
- `loki`
- `metallb`
- `monitoring-kubernetes`
- `monitoring-ping`
- `node-manager`
- `openvpn`
- `operator-trivy`
- `prometheus`
- `prometheus-metrics-adapter`
- `pod-reloader`
- `registrypackages`
- `runtime-audit-engine`
- `service-with-healthchecks`
- `static-routing-manager`
- `terraform-manager`
- `user-authn`
- `vertical-pod-autoscaler`

The complete list of changes, including the updated components,
is available in the [changelog](https://github.com/deckhouse/deckhouse/blob/main/CHANGELOG/CHANGELOG-v1.68.md) on GitHub.
