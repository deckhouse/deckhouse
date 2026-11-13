# Changelog v1.78

## [MALFORMED]


 - #100 unknown section "registrypackages"
 - #1004 unknown section "registrypackages"
 - #1023 unknown section "registrypackages"
 - #1078 unknown section ""
 - #1079 unknown section ""
 - #1080 unknown section ""
 - #1115 unknown section ""
 - #1116 unknown section ""
 - #1132 unknown section "registrypackages"
 - #1199 unknown section ""
 - #1203 unknown section ""
 - #1206 unknown section ""
 - #1207 unknown section "global"
 - #1214 unknown section "registrypackages"
 - #1221 unknown section "cloud-provider"
 - #1230 unknown section ""
 - #1254 unknown section "registrypackages"
 - #1263 unknown section "registrypackages"
 - #1296 unknown section "cloud-provider-dvp"
 - #1339 unknown section ""
 - #1350 unknown section ""
 - #1354 unknown section "registrypackages"
 - #1418 unknown section "werf"
 - #1493 unknown section ""
 - #177 unknown section ""
 - #185 unknown section "registrypackages"
 - #20660 unknown section "cloud-provider-dvp"
 - #21244 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #21694 unknown section "cloud-provider-dvp"
 - #21837 unknown section "<kebab-case of a module name> | <1st level dir in the repo>"
 - #21914 unknown section "registrypackages"
 - #21918 unknown section "cloud-provider-dvp"
 - #21924 unknown section "registrypackages"
 - #21990 unknown section "registrypackages"
 - #22070 unknown section "registrypackages"
 - #22122 unknown section "registrypackages"
 - #22191 unknown section "registrypackages"
 - #22241 unknown section "cloud-provider-dvp"
 - #22274 unknown section "cloud-provider-dvp"
 - #22294 unknown section "registrypackages"
 - #22351 unknown section ""
 - #22434 unknown section "cloud-provider-dvp"
 - #22443 unknown section ""
 - #22454 unknown section "registrypackages"
 - #22559 unknown section "cloud-provider-dvp"
 - #22582 unknown section "cloud-provider-dvp"
 - #22618 unknown section "cloud-provider-dvp"
 - #22652 unknown section "registrypackages"
 - #22657 unknown section "registrypackages"
 - #22672 unknown section "registrypackages"
 - #22686 unknown section "service-with-healthchecks"
 - #22728 unknown section "registrypackages"
 - #22740 unknown section "service-with-healthchecks"
 - #22744 unknown section "cloud-provider-dvp"
 - #22748 unknown section "cloud-provider-dvp"
 - #22772 unknown section "cloud-provider-dvp"
 - #22830 unknown section "cloud-provider-dvp"
 - #22846 unknown section "cloud-provider-dvp"
 - #22916 unknown section "service-with-healthchecks"
 - #22973 unknown section "service-with-healthchecks"
 - #23029 unknown section "registrypackages"
 - #23042 unknown section "registrypackages"
 - #23062 unknown section "registrypackages"
 - #23084 unknown section "registrypackages"
 - #23118 unknown section "service-with-healthchecks"
 - #23125 unknown section "drain"
 - #23198 unknown section "service-with-healthchecks"
 - #23214 unknown section "cloud-provider-dvp"
 - #23241 unknown section "cloud-provider-dvp"
 - #23340 unknown section "cloud-provider-dvp"
 - #270 unknown section "registrypackages"
 - #283 unknown section "service-with-healthchecks"
 - #299 unknown section "cloud-provider-dvp"
 - #304 unknown section ""
 - #305 unknown section "cloud-provider-dvp"
 - #383 unknown section ""
 - #393 unknown section ""
 - #406 unknown section "cloud-provider-dvp"
 - #421 unknown section "modules"
 - #422 unknown section "cloud-provider-dvp"
 - #432 unknown section ""
 - #537 unknown section ""
 - #547 unknown section ""
 - #551 unknown section ""
 - #631 unknown section ""
 - #644 unknown section ""
 - #648 unknown section ""
 - #649 unknown section "registrypackages"
 - #701 unknown section ""
 - #721 unknown section "registrypackages"
 - #726 unknown section ""
 - #739 unknown section "registrypackages"
 - #78 unknown section "cloud-provider-dvp"
 - #82 unknown section "cloud-provider-dvp"
 - #827 unknown section ""
 - #845 unknown section "cloud-provider-dvp"
 - #847 unknown section ""
 - #875 unknown section ""
 - #885 unknown section "registrypackages"
 - #916 unknown section ""
 - #917 unknown section ""
 - #980 unknown section ""

## Know before update


 - A NodeGroup created while the check was dead — `swapBehavior: LimitedSwap` with nodes that do not
    support cgroup v2 — is now rejected on every update, including the finalizer removal that a
    delete needs, so such a NodeGroup can hang in Terminating. Removing `swapBehavior: LimitedSwap`
    from its spec passes and unblocks it.
 - A ProjectTemplate can no longer be created through `deckhouse.io/v1alpha1`, and no template can get a new `resourcesTemplate` through either version. Existing templates with a Helm text keep working and can be changed, but a backup restore, a GitOps bootstrap or a re-creation of such a template is refused until it is moved to structured fields and `manifests`. The `MultitenancyManagerHelmProjectTemplate` alert names these templates.
    An AuthorizationRule of the `Admin` access level in `manifests` is refused when it is created or when an update raises a rule to this level, and the project switches to `Error`. A write of a project whose `manifests` render such a rule, and a write of a template whose `manifests` describe an AuthorizationRule, are admitted with a warning. Set the administrators of a project in `.spec.administrators` of the Project instead.
 - A SecurityPolicy that set `allowHostNetwork` without `allowedHostPorts` stopped denying anything at all, including the `hostNetwork` it forbade. Such a policy is enforced again. Review the policies that set only `allowHostNetwork`: workloads that ran under them may start being denied.
 - A SecurityPolicy that set only one of the two keys denied nothing at all.
    It now denies the sysctls it lists, so workloads that used a forbidden
    sysctl and were admitted before will be rejected. Check `forbiddenSysctls`
    in your SecurityPolicy resources before updating.
 - A StaticInstance whose connectivity check fails now keeps its reservation for the whole bootstrap window instead of returning to the pool on every failed attempt, and the bootstrap (20 min) and cleanup (10 min) timeouts, which previously never fired, are now reachable.
    If nothing ever ran on the host, the bootstrap timeout returns the instance to the pool after 20 minutes as before.
    If the host was already bootstrapped in part, the instance returns only through MachineHealthCheck remediation (nodeStartupTimeoutSeconds: 1200) plus the cleanup timeout, so it becomes available again roughly 20 + 10 minutes after the failure, and remediation reboots the host as part of the cleanup.
 - A namespace adopted into a project no longer has its own labels and annotations copied into the Project parameters.
    The project release neither reverts a direct change to such a label (for example `istio.io/rev`, `team`, Argo CD or Flux tracking labels) nor restores a removed one.
    The additional namespaces of the project do not inherit these labels; declare one in `spec.parameters.namespace.labels` of the Project, or set it on an additional namespace directly, where it is needed.
    The node-selector and tolerations annotations of an adopted namespace are still copied into `spec.parameters.namespace.annotations` and are changed there.
    `extended-monitoring.deckhouse.io/enabled` and `security-scanning.deckhouse.io/enabled` keep the value `"true"` the namespace had instead of being rewritten to `""`, and any other value, `"false"` included, becomes `""`.
    A ClusterResourceGrantPolicy that selects namespaces by such a label with `namespaceSelector` no longer covers the new additional namespaces of an adopted project; declare the label in `spec.parameters.namespace.labels` of the Project or select the project with `projectSelector`.
 - A namespace named `d8-*` or `kube-*`, and a namespace labeled `heritage: deckhouse` whatever it is called, is treated as the platform's own. Such a namespace is checked against the `restricted` Pod Security Standard: a violation is reported in the audit and in Deckhouse Console and the workload still starts, unless the module that owns the namespace has opted it into enforcement. None of this is configurable.
    
    User policies only warn there. An OperationPolicy or a SecurityPolicy with `enforcementAction: Deny` reports a violation in a system namespace and does not block the workload, whatever the namespace is labeled with, including a namespace a module has opted into enforcement.
    
    Enforcement in `kube-*` namespaces is relaxed: previously the exclusion matched `d8-*` only, so `kube-system` and the other `kube-*` namespaces were subject to user policies in full.
    
    Enforcement in system namespaces no longer depends on `defaultPolicy`. With `defaultPolicy: Restricted` the restricted constraint was previously not created, and with `defaultPolicy: Privileged` the baseline one, so a namespace that had opted into enforcement was checked against the weaker standard only.
    
    Gatekeeper mutations no longer run in `d8-*` and `kube-*` namespaces.
    
    The name prefixes `d8-system-default-` and `d8-pod-security-` are reserved: creating or updating an OperationPolicy or SecurityPolicy with such a name is rejected, as is a name longer than 235 characters, which would push a derived constraint name past the limit Kubernetes sets for an object name.
    
    Expect more audit data. Every pod in a system namespace is now measured against the full `restricted` set, and control-plane static pods violate most of it, so the constraints report violations that no constraint used to see. Each violation is one Prometheus series from `constraint-exporter` and one entry in the constraint status, up to the audit limit of 200 per constraint. None of it alerts: the audit rules fire on `deny` only.
 - A new fail-closed webhook, quota-ledger.multitenancy-manager.deckhouse.io, checks the status writes of d8-project-quota. While no replica of multitenancy-manager answers, counted creates in the namespaces of projects with a quota fail with 503; deletions and other namespaces are not affected. multitenancy-manager runs two replicas where control-plane-manager is enabled.
 - A node whose drain keeps failing now keeps the `draining` annotation, carries
    `update.node.deckhouse.io/drain-failed` with the error, and is retried. Its NodeGroup
    stops updating until the drain succeeds, instead of updating a node that still runs its
    pods. `NodeStuckInDraining` fires five minutes after the first failure and now covers
    drains requested by update approval, by a cloud spot termination and by hand. The
    `d8_node_draining` metric lost its `message` label, the error is read from the node.
 - A pre-existing Assign/AssignMetadata/ModifySet/AssignImage resource that violates one of these restrictions (see the module FAQ) starts failing `kubectl apply`/`helm upgrade` after the upgrade. Fix the resource before upgrading, or drop it if it was already a silent no-op, most commonly a mutator missing `spec.match.kinds`, which never took effect anyway.
 - A subject who can create UserOperations cannot reset, lock, or unlock an
    identity whose email or group membership already carries roles they cannot
    assign. Nested Group membership counts. Deleting a ClusterAuthorizationRule
    is allowed only if the requester could assign those roles. Creating or
    reconnecting a DexProvider without spec.allowedIdentities on a cluster that
    already has a SuperAdmin grant requires SuperAdmin; ClusterAdmin and the
    security subsystem manager keep rotating secrets, toggling, renaming,
    narrowing and re-applying existing providers, and connect providers whose
    allowedIdentities reach only roles they can assign. GitOps that adds an
    unlimited provider or removes a SuperAdmin rule must run as a SuperAdmin.
 - A subject who can manage Users or Groups cannot delete or recreate an identity
    whose email or group name already carries roles they cannot assign. The same
    subject cannot create or update a ClusterAuthorizationRule whose accessLevel
    or additionalRoles sit outside their can-assign range. A security subsystem
    manager can still assign ClusterAdmin and cannot assign SuperAdmin.
    The collision annotation does not bypass this check. GitOps ServiceAccounts
    that are not exempt cannot create a User after a SuperAdmin CAR for that email.
 - A user who may delete projects but not namespaces can no longer delete a project whose namespaces exist, and the refusal names the namespace. This affects holders of `d8:subsystem:deckhouse:manager` without `d8:subsystem:kubernetes:manager` or `d8:subsystem:infrastructure:manager`. Deleting a project deletes its namespaces with all workloads in them, so grant `delete` on `namespaces` to the users who remove projects. Only the delete of a Project is checked. A holder of the cluster-wide `d8:system-capability:multitenancy-manager:edit` can still delete the additional namespaces of a project through their ProjectNamespace objects.
 - Affects clusters where `publishAPI` is enabled together with `enableBasicAuth` on a
    Crowd, OIDC or LDAP `DexProvider`. Both are disabled by default, so a default
    installation is not affected.
    
    Previously the proxy copied directory group names into the `X-Remote-Group` header
    verbatim. Because kube-apiserver trusts that header from this proxy, a user in a
    directory group literally named `system:masters` was granted cluster-admin. Group
    names are chosen by the directory administrator, not by Kubernetes.
    
    After updating, any group whose name begins with `system:` is dropped before the
    request reaches kube-apiserver, and each drop is logged by the basic-auth-proxy pod
    with the user and the group name. A user authenticating with a `system:`-prefixed
    username is refused with `403 Forbidden`. This matches the `userValidationRules`
    already enforced on the OIDC/JWT path.
    
    Before updating, audit your directory for groups whose name begins with `system:`
    and check whether anyone currently relies on one to reach the Kubernetes API through
    basic auth. Such users will lose those privileges on update and must be granted
    access through a normal group bound by RBAC instead. Check the basic-auth-proxy logs
    after updating for `dropping group` messages.
 - After the update, every project on the built-in templates `default`, `secure` and `secure-with-dedicated-nodes` switches from their former Helm texts to structured fields. Its administrators move without a gap from AuthorizationRule objects with the `Admin` access level (`user-authz:admin` plus `user-authz:admin:custom`) to the `d8:project:admin` role of the `d8-administrators` ProjectRoleBinding, and its `all-pods` quota is replaced by `d8-project-quota`. The two roles do not grant the same permissions. In particular, custom ClusterRoles with the `user-authz.deckhouse.io/access-level` annotation no longer add to the access of these administrators; create a custom role and bind it with a ProjectRoleBinding instead.
    
    - While the old objects are kept, the Project has the `HandoverPending` condition. If they are still there after 30 minutes, the `MultitenancyManagerProjectHandoverStuck` alert fires.
    - On a later switch of a user template, every other object of the Helm text that the structured fields do not render (rules and bindings of other subjects, other quotas, network policies, workloads) is left in place instead of being deleted. It keeps working, is no longer managed by the project, carries the `projects.deckhouse.io/kept-from-helm-template` label, under which only a requester who may make the same change across the cluster may change or delete it, and is listed in `status.keptObjects`, and the `MultitenancyManagerProjectKeepsHelmTemplateObjects` alert fires until it is deleted, unlabelled or rendered by the template again.
    - Administrators and quotas that stay in `.spec.parameters` because they could not be moved to `.spec.administrators` and `.spec.quota` are not handed over, and a switch leaves their objects in place.
 - An Application that declares `spec.resourceRequests` is now installed from a chart
    rendered with the requested replicas and container resources applied, instead of
    from the package chart directly. Applications without `resourceRequests`, and all
    modules, are unaffected. Updating restarts the deckhouse Deployment.
 - An update to 1.78 from DP 1.77.1 or later is blocked while the cluster runs Proxy or Local of the v1 registry implementation, or Direct without the registry ModuleConfig.
    Migrate the cluster to the registry module on 1.77 before updating (see the registry module FAQ), and update to 1.78 only from DP 1.77.1 or later.
 - Applying the change restarts control-plane components.
    Listing projects without RBAC for them now returns the accessible subset instead of 403;
    a project is visible when any of its namespaces is.
 - Basis Dynamix 4.6 made `storage_policy_id` a mandatory field of `disks/create` and
    `kvmx86/create`, and it is the first version that answers on the storage policy API the provider
    now depends on. Earlier platform versions are no longer supported: virtual machines cannot be
    created there at all.
    Check the platform version before updating. `dhctl bootstrap` refuses to deploy a new cluster on
    an older platform, but an existing cluster on one has no working path forward.
    The account permissions change with it: the `sep` user API group is no longer used at all, and
    the `storage_policy` group is now required.
 - Basis Dynamix chooses where a disk lives when the virtual machine is created and the provider
    never moves a disk afterwards, so a new storage policy reaches a node only by recreating it.
    Editing the cluster-wide `storagePolicy` recreates the CloudEphemeral nodes of every node group —
    including groups that set their own `storagePolicy` in their instanceClass and whose own
    configuration did not change.
    Master and static nodes are not recreated: they stay in the policy they were created in, and a new
    policy reaches them only through a node created anew.
 - Before, an update of such a Deployment, StatefulSet or DaemonSet that set a new kubectl.kubernetes.io/restartedAt annotation skipped the d8-multitenancy-manager admission policy with every other change it carried. Now the policy refuses it unless that annotation is the only change, so a tool that restarts a workload and changes it in one update gets a refusal; change such a workload through the ProjectTemplate and restart it with kubectl rollout restart.
 - Breaking change for clusters with custom ProjectTemplates written as Helm strings.
    
    - `spec.resourcesTemplate` no longer exists in `ProjectTemplate` v1alpha2; a manifest that carries it is refused by the schema.
    - A template stored as v1alpha1 with a Helm string is converted without the string and marked with the `projects.deckhouse.io/legacy-helm-template: "true"` annotation. Projects of such a template switch to the `Error` state with the `ProjectTemplateUsable` condition and are not rendered; their existing objects are not touched. Rewrite the template with structured fields and remove the annotation to resume.
    - The built-in `simple` template renders the required-requests OperationPolicy only when the new `requiredRequests` parameter is `true`; existing projects on `simple` keep rendering the namespace only.
    - The `TemplateResourcesFiltered` and `TemplateRolesAllowed` project conditions and the `projects.deckhouse.io/unmanaged` and `projects.deckhouse.io/skip-heritage-label` labels are removed together with the Helm-string path.
 - Breaking change of the DynamixClusterConfiguration and DynamixInstanceClass contract. There is no
    migration and no upgrade path: a cluster created on an earlier version has to be reinstalled.
    `storageEndpoint` and `pool` are removed, and `storagePolicy` replaces them: mandatory at the
    root of DynamixClusterConfiguration, optional as an override in every instanceClass, including
    `masterNodeGroup.instanceClass` and DynamixInstanceClass.
    The configuration cannot be prepared in advance: the schema a running cluster validates against
    rejects `storagePolicy` as an unknown field, so the fix cannot be made before the update, and
    after it the stored configuration no longer passes validation and the cluster stops converging.
    StorageClasses are now created one per `ENABLED` storage policy of the account instead of one per
    storage endpoint, so their names change and their parameters no longer carry `location`, `pool` or
    `storageEndpoint`. Review any PersistentVolumeClaim that names a StorageClass explicitly: the
    names of the reinstalled cluster are not the names of the old one. A policy that the account
    loses or that stops being `ENABLED` takes its StorageClass with it; an answer with no `ENABLED`
    policy at all is taken for a glitch and keeps the StorageClasses the cluster has. The module, which had no settings before, now takes `storageClass.exclude`
    to keep some of those StorageClasses out of the cluster, and a deprecated `storageClass.default`.
    Two DynamixMachine defaults move with the CRD: `rootDiskSizeGb` defaults to 30 instead of 20, and
    `cpu` is no longer a required field.
    The CSI driver no longer reports storage capacity, and the cloud-data-discoverer,
    cloud-controller-manager, CSI driver and CAPI controller images are all updated together.
 - Bring the registry to `Unmanaged` BEFORE updating.
    
    This release renders none of the previous registry implementation's objects. A cluster that
    arrives still in `Direct` or `Proxy` loses them on the first reconciliation — the in-cluster
    proxy, the `registry` Service and the per-node configuration they served — while its nodes go
    on pointing at that address. Images already on a node keep working; anything that has to be
    pulled does not, and the migration cannot be completed from that state. The way back is to
    return to the previous release, bring the cluster to `Unmanaged` there, and update again.
    
    A cluster already in `Unmanaged` is unaffected. The handover happens on its own, and the new
    implementation manages nothing until it is configured, so the update changes no behaviour.
    
    An air-gapped cluster in `Local` has a procedure of its own, which also starts before the
    update: an OCI registry stood up outside Deckhouse's namespaces, loaded with `d8 mirror pull`
    and `d8 mirror push`, declared as the upstream for the length of the migration and removed once
    the in-cluster storage holds the image set. It costs a second copy of that set on disk while it
    exists. See "How do I migrate an air-gapped Local cluster?" in the registry module's FAQ.
    
    From this release the platform records which implementation a cluster runs, so a later release
    can refuse to install on a cluster still running the previous one.
 - Built-in role names now say what they reach: `d8:namespace:<level>`, `d8:project:<level>`,
    `d8:subsystem:<subsystem>:<level>` and `d8:system:<level>` replace `d8:use:role:<level>` and
    `d8:manage:<subsystem|all>:<level>`. Rules live in capabilities that roles aggregate, so a module
    ships access to its own resources and a cluster administrator composes custom roles under the
    reserved `d8:custom:` prefix.
    
    Every old name keeps working for ONE release: it is served as an alias aggregating the same
    capabilities, marked `rbac.deckhouse.io/deprecated`, and the alerts
    D8UserAuthzDeprecatedRBACv2RoleInUse and D8UserAuthzDeprecatedRBACv2CapabilityInUse name the
    bindings that still reference it. Move those bindings to the new names before the next release --
    `roleRef` is immutable, so a binding has to be recreated rather than edited. One alias is narrower
    than the role it replaces: `d8:use:role:admin` no longer grants `serviceaccounts/token:create` and
    `impersonate`, which moved to superadmin.
    
    Admission now enforces what the model assumes: a ClusterRoleBinding may not carry a
    namespace/project-scoped role or a capability, a custom role may not aggregate system capabilities
    into a tenant scope, and objects marked `deckhouse.io/system-resource` inside user namespaces are
    editable only by superadmin.
 - Changing allowedGroups or allowedEmails deletes refresh tokens that no longer match. Those users must sign in again.
 - Cilium agent and operator pods will be restarted.
 - Cilium agent pods are restarted to apply the fixes.
 - Cilium agent pods will be restarted.
 - Cilium agent pods will be restarted. With DSR, the DSR IPv4 option is stripped before the packet reaches the pod.
 - Cloud provider configuration for DKP clusters deployed in Yandex Cloud needs to be migrated from YandexClusterConfiguration to ModuleConfig settings. For migration instructions, refer to "Cluster and Infrastructure" in the DKP FAQ.
 - ClusterResourceGrantPolicy is stored as multitenancy.deckhouse.io/v1alpha2. A policy written through v1alpha1 keeps selecting the same namespaces: its projectSelector becomes the v1alpha2 namespaceSelector. To select projects by labels on the Project object, use spec.projectSelector of v1alpha2.
    When a policy manifest moves from v1alpha1 to v1alpha2, rename its projectSelector to namespaceSelector. A manifest that changes only its apiVersion keeps projectSelector, which v1alpha2 matches against the labels of the Project, so a selector by a label that only the namespace has matches nothing, and the policy stops applying to those namespaces.
    Labels a ProjectTemplate sets through namespaceMetadata.labels are now also set on additional namespaces, and namespaceMetadata.labels can no longer override the pod security, monitoring and scanning labels of the template.
    Each project is re-rendered once after the update; this resets hand edits of template labels on its main namespace.
    The projects.deckhouse.io/project and projects.deckhouse.io/project-template labels of a Project can no longer be pointed at another project or template. A manifest that replaces a whole Project may leave them out. Such an update is accepted with a warning, and the controller puts the labels back.
    During the update from 1.77, create or change ClusterResourceGrantPolicy through v1alpha1 only after the webhook-handler of the deckhouse module has rolled out: until it registers the conversion, such a write is stored with its selector read as a Project selector.
    All ClusterResourceGrantPolicy objects are stored as v1alpha2 soon after the update, because the controller writes their status. Deckhouse does not support downgrades: a manual rollback to a version without v1alpha2 makes every policy unreadable.
 - Clusters bootstrapped before v1.55 were switched to the `Baseline` default PSS policy in 1.77, which denies privileged workloads in user namespaces.
    After the update, the default policy on these clusters becomes less strict: it is switched back to `Privileged`, and privileged pods in user namespaces pass admission again.
    To keep the `Baseline` policy, set `settings.podSecurityStandards.defaultPolicy: Baseline` explicitly in the `admission-policy-engine` ModuleConfig.
 - Clusters that never set mode now reserve only the platform service hostnames. Set mode Template for the wider reservation. A publicDomainTemplate like %s-cluster.example.com stays on List even if Template is asked for. kube-%s.example.com stays Template when Template is set.
 - Containers attached through the `pods/ephemeralcontainers` subresource were not seen by
    the module's webhook, so no policy was evaluated for them: SecurityPolicy, OperationPolicy,
    Pod Security Standards and image signature verification were all bypassed by `kubectl debug`.
    After the update these requests are validated.
    
    On clusters with restricted Pod Security Standards, `deny-exec` or strict SecurityPolicy,
    typical debug images (root user, writable root filesystem, interactive `-it`) will now be
    rejected. Adjust the policies or use a SecurityPolicyException for the namespaces where
    debugging must stay available.
    
    The webhook uses `failurePolicy: Fail`, so while Gatekeeper is unavailable `kubectl debug`
    will not work in user namespaces. System namespaces are excluded from the webhook and are
    not affected.
 - Control plane components and kubelet will be restarted.
 - CoreDNS pods will be restarted.
 - Creating a `Group` or a `User`, or renaming an existing one, so that `Group.spec.name` matches a group subject or `User.spec.email` matches a user subject of an existing AuthorizationRule or ClusterAuthorizationRule is now rejected, preventing an unintended privilege grant. The email is compared after lowercasing, since that is the form that reaches the token. Set the `user-authz.deckhouse.io/allow-authorization-rule-collision` annotation to `"true"` on the object if the collision is intentional. Already existing objects keep working and only produce a warning. Any declarative flow that manages both the rule and the identity is affected, not only one that deletes and recreates the identity: a first-time apply denies the `Group` or `User` whenever the rule happens to be applied first, and nothing guarantees the order. Add the annotation to such objects before upgrading. Deleting a `Group` or a `User` whose name is still a subject of a rule now produces a warning, since the rule keeps granting that name.
 - Creating a namespace whose name is longer than 61 characters, starts with upmeter- or is deckhouse, or putting the heritage: upmeter label on a namespace, is refused to everyone but system components, cluster administrators (system:masters) and system:sudouser, whatever allowNamespacesWithoutProjects says. A namespace that existed before and belongs to no project, for example one whose Project stays in Error because the namespace belongs to another Helm release, raises the D8NamespaceNotAdopted alert after the upgrade.
 - Custom system or subsystem roles labelled `rbac.deckhouse.io/use-role` above `viewer` are projected as `viewer` after the update; the `d8_user_authz_projection_capped` metric names them, and any later change of such a role is refused until the label is set to `viewer` (without the label the role is not projected). A capability not built into the platform no longer adds its `rbac.deckhouse.io/namespace` to a projection.
 - Dex rejects an authorization request with a code_challenge and the plain method, or with no method. A DexClient application that sends one has to switch to S256. Applications that send no PKCE and DexAuthenticator are not affected. The device flow is not affected.
 - Dex, DexAuthenticator, user-authn-controller, user-api and basic-auth-proxy pods restart. Tokens issued before the update keep working until the first revocation of their user, so nobody has to sign in again.
    A `Lock` of an external user through `spec.target` now ends the user's sessions.
    Deleting a UserIdentity object by hand is refused. To remove a local user, delete the User; to reset 2FA or a lock, use a UserOperation.
    DexAuthenticator receives revocations from the `revocation-feed` Service in `d8-user-authn`. If a NetworkPolicy restricts egress in the namespace of a DexAuthenticator, allow traffic to the `user-authn-controller` pods on port `8443`; otherwise the `D8UserAuthnRevocationFeedNotConsumed` alert fires.
 - During the first `user-authz` release after the update the per-role bindings are protected from the release engine, the aggregated ones are created, and the per-role bindings are deleted right after the release. Permissions granted through annotated ClusterRoles stay in place throughout.
    The `user-authz.deckhouse.io/access-level` label is now set automatically on annotated ClusterRoles.
    In audit logs, access granted through annotated ClusterRoles is attributed to `user-authz:<level>:custom` instead of the individual ClusterRole.
 - During the update to this release, kubelet is reinstalled on all nodes and all static pods (`etcd`, `kube-apiserver`, `kube-controller-manager`, `kube-scheduler`, `kubernetes-api-proxy`) are restarted once without a graceful shutdown; on master nodes this causes an etcd leader election.
 - During the update to this release, the `reserved-policy-names` and `securitypolicies` validating
    webhooks are absent for a few minutes (3–4 minutes on a test cluster), until the module chart
    creates the `ValidationWebhook` resources and `webhook-handler` loads them.
    While they are absent, the API server does not check the names of new policies or the overlap of
    image references in `SecurityPolicy` objects.
    The window occurs once per cluster, during this update.
    Plan the update for a time when you can hold back changes to `SecurityPolicy` and `OperationPolicy`.
 - During the update to this release, the `users-unique` and `groups-unique` validating webhooks are
    absent for a few minutes (3–4 minutes on a test cluster), until the module chart creates the
    `ValidationWebhook` resources and `webhook-handler` loads them.
    While they are absent, the API server does not check `User` and `Group` objects for duplicate
    emails, uppercase emails or group cycles.
    The window occurs once per cluster, during this update.
    Plan the update for a time when you can hold back changes to users and groups.
 - During the update to this release, the validating webhooks of `user-authz` are absent for a few
    minutes (3–4 minutes on a test cluster), until the module chart creates the `ValidationWebhook`
    resources and `webhook-handler` loads them.
    While they are absent, the API server does not run these checks, including the privilege escalation
    guards `d8-user-authz-identity-assign`, `rbacv2-role-escalation`, `rbacv2-system-resource-edit` and
    `rbacv2-system-resource-exec`.
    The window occurs once per cluster, during this update. Later restarts of `webhook-handler` restore
    the webhooks before it starts serving.
    Plan the update for a time when you can hold back changes to RBAC objects, users, groups and
    `ClusterAuthorizationRule` objects.
 - Exec and attach requests in user namespaces reach Gatekeeper again, as in Deckhouse 1.73 and 1.74.
    Every constraint that matches them is evaluated again, including a constraint without `match.kinds`
    or with `kinds: ["*"]` whose policy does not check `input.review.operation`.
    Before updating, check that such constraints do not deny `kubectl exec` and `kubectl attach`
    for users who need them.
 - Existing clusters with a DexClient that has the dexclient.deckhouse.io/allow-access-to-kubernetes annotation keep token exchange through their OIDC and Gitlab DexProviders through the flag, which the module sets on the upgrade. Move the exchange to a TokenExchangeProvider and turn the flag off. The flag and token exchange through a DexProvider will be removed after the next LTS release.
 - Existing huaweicloud clusters that upgraded to any 1.77.x and have `terraform-auto-converger` in Init:CrashLoopBackOff with "Node ... state does not have an OK status" will proceed with converge after this fix. No manual action required on the cluster side. Freshly bootstrapped 1.77.x clusters where `module.kubernetes_data.terraform_data.master` exists in state will have it destroyed on next converge — it is a virtual resource with no cloud-side object, so this is a state cleanup only. Because the plan contains a destructive operation, dhctl will not apply it automatically; operators of such clusters need to approve the converge explicitly.
 - Fencing no longer deletes the pods of a node after 60 seconds without a confirmation that the node stopped. The fencing-controller sets the `node.kubernetes.io/out-of-service` taint only on a stopped node: after a witness confirms the stop, or after the node is silent for a delay of its profile. Then it deletes the pods of the node and the Node of a cloud Cluster API machine or the Machine of an MCM node.
    A node with fencing stops when it does not see the control plane longer than the threshold of its mode (4.15, 9.1 or 20.2 seconds). A loss of the control plane for the whole cluster that is longer than this stops all nodes with fencing.
    On update, Deckhouse replaces all fencing agents at the same time, the nodes do not reboot. The `Watchdog` mode works as `Slow`. The `spec.fencing.watchdog.timeout` parameter of NodeGroup is deprecated and ignored. A rollback to fencing 1.0 is not supported.
    You cannot turn fencing on in a cluster with one master node. You cannot change the fencing mode of a node group while a node of the group has a FencingFailedNodeState.
    To disable the `node-manager` module, turn fencing off in all node groups first and confirm the disabling.
    Open port 8500 TCP and UDP between all cluster nodes.
 - Flannel pods will be restarted.
 - Holders of `d8:system:manager`, `d8:subsystem:iam:manager` and the deprecated `d8:manage:all:manager` and `d8:manage:security:manager` can no longer write a ClusterAuthorizationRule with `Admin`, `ClusterAdmin` or `SuperAdmin` or one that opens the system namespaces (`allowAccessToSystemNamespaces`, `namespaceSelector.matchAny`, a `labelSelector` that does not name its namespaces by `kubernetes.io/metadata.name`, a rule for a request another such rule already lets in, the requester's own included, or any rule while multitenancy mode is disabled, so in CE no ClusterAuthorizationRule at all), cannot create Users, add Group members, run UserOperations or connect DexProviders for the identities of such rules, cannot add a member already let into the system namespaces to a Group with such a rule, need the range for the roles of the groups above a Group they write, cannot turn multitenancy mode off, cannot assign `admin` and `superadmin` roles of the granular model, and cannot write admission webhooks and policies. Existing rules stay; a ClusterAdmin or a SuperAdmin does these.
 - If the configured NTP servers are unreachable, a master boot can wait up to 30 seconds by default. On bashible nodes this limit can be raised to 60 seconds. Kubelet starts after the timeout and NodeTimeNotSynchronizedAtBoot fires after the failure metric is collected. For isolated clusters, configure reachable ntpServers or set earlyTimeSync.enabled to false.
 - If your `multitenancy-manager` ModuleConfig sets `allowNamespacesWithoutProjects`, remove the parameter. It has no effect, and in the next minor release a ModuleConfig that still carries it will fail validation and stop applying.
 - In a cluster that still has the `antiopa` or `loghouse` namespace, a ClusterAuthorizationRule without `allowAccessToSystemNamespaces` now gives its subjects access to these namespaces when it has neither `limitNamespaces` nor `namespaceSelector`, or when a `limitNamespaces` pattern matches them (for example `.*`). Check with `kubectl get ns antiopa loghouse`. These namespaces are left from components that preceded Deckhouse; delete them if nothing uses them. To keep such a namespace closed, list the namespaces of the rule in `limitNamespaces` with patterns that do not match it. Do not use `namespaceSelector` for this: a selector replaces `limitNamespaces` and `allowAccessToSystemNamespaces` and opens every namespace it matches, system namespaces included.
 - In a cluster with the `proxy` parameter of `ClusterConfiguration` set, the `cert-manager` controller and the Yandex Cloud DNS webhook
    send requests to the ACME server, HashiCorp Vault and the DNS provider APIs through the proxy server.
    The `HTTP-01` self-check also requests the domain through the proxy server.
    If the proxy server cannot reach the issuer or the public address of the cluster, add these addresses to `noProxy`.
 - Machines are not recreated on update. Machines created before the update keep the standard network type and no labels until the NodeGroup is rolled out (change a rolling InstanceClass field or set the `manual-rollout-id` annotation on the NodeGroup). Machines stuck in provisioning because of `additionalSubnets` become ready without recreation.
 - NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs until the next rollout (any rollout-triggering InstanceClass change or manual-rollout-id); after that rollout their nodes become preemptible VMs and are rotated every ~20h.
 - NodeGroups with `nodeType: Static` no longer get the cloud provider's node configuration scripts.
    Until now every NodeGroup in a cloud cluster, Static included, was configured with them.
    
    On AWS this removes `007_enable_ecr_credential_provider` from Static nodes: the kubelet unit is
    regenerated without `--image-credential-provider-config`, kubelet restarts and loses access to
    ECR. The same applies to any other provider step a Static node relied on.
 - On Kubernetes 1.33 and later, new VerticalPodAutoscaler objects without spec.updatePolicy get updateMode set to InPlaceOrRecreate. VPA resizes their pods in place where possible and evicts pods only as a fallback. This includes running pods of single-replica workloads, which VPA did not update before. Existing VPA objects keep the update mode already stored in them. The vpa-recommender, vpa-updater and vpa-admission-controller pods are restarted.
 - On a NodeGroup with `spec.gpu`, a failing GPU check used to abort the bashible run and be retried forever, so the node never configured kubelet and never received updates from any other module. Such a node now converges and only its GPU stays unusable. If `/usr/bin/nvidia-container-runtime` is absent, containerd is now configured with `runc` as the default runtime instead of `nvidia`; on a node whose configuration previously selected `nvidia` this changes the containerd config and therefore restarts containerd. Previously such a node could not start any container at all.
 - On a cluster whose user-authn ModuleConfig sets `idTokenTTL` to 6h or more, Dex issues ID tokens for 5h59m after the update, which also lowers the minimum user session lifetime (`keepUsersLoggedInFor` of a DexAuthenticator, `auth.sessionTTL` of the console) to that value. The `D8UserAuthnIDTokenTTLTooLong` alert goes from severity 6 to 4 and keeps firing until the ModuleConfig is updated to settings version 3 with an `idTokenTTL` below 6h.
 - On an adopted (or any project) namespace, `kubectl label`/`annotate` and GitOps re-applies are accepted for keys the module does not own; `heritage`, `projects.deckhouse.io/*`, the template-driven policy labels, the Helm ownership annotations, the node-selector and tolerations annotations, `spec`, finalizers and deletion are still refused and are changed through the Project and its template.
    Creating a namespace with a `projects.deckhouse.io/project` label, or changing that label, is refused for everyone but the controller — cluster administrators included; create a Project or a ProjectNamespace instead.
    `AvailableClusterResource` objects carry `heritage: multitenancy-manager` (relabelled in place) and are refused to everyone but the controller by the admission policy; a project with nothing available keeps an empty catalog object instead of none.
 - On builds carrying
 - On clusters configured with `ZvirtClusterConfiguration`, the module keeps working from it and prepares the migration resources in the `d8-migration-resources` Secret of the `d8-cloud-provider-zvirt` namespace.
    The `D8CloudProviderZvirtMigrationPending` alert fires until these resources are applied; follow the alert description to complete the migration.
    The zVirt credentials move to the `d8-credentials` Secret. A validating webhook now checks the module ModuleConfig, NodeGroups, ZvirtInstanceClasses and the credentials Secret.
    CCM, CSI, cloud-data-discoverer and capz-controller-manager pods of the module may restart. Existing VMs and disks are not recreated.
 - On the `Standard` layout a node has only the external network, which may hand out no DNS server.
    The servers listed in `nameservers` are now written into the cloud-init of every master, static and
    CloudEphemeral node of that layout, and reach only the nodes created afterwards.
    `nameservers` now accepts IPv4 addresses only, in both layouts: a configuration with a host name or
    an IPv6 address in it is rejected by validation. Fix it before updating.
 - On upgrade, an existing inline CA that is malformed, incomplete (e.g. `key` without `cert`), or whose declared `root` does not anchor `cert` will now hard-block the istio module until corrected. Configurations with valid inline CA material are unaffected.
 - On upgrade, clusters that use an inline **intermediate** CA (`ca.cert` is an intermediate, with a distinct `ca.root`) will have the mutating/validating webhook `caBundle` change from the intermediate to the root certificate, and istiod pods may restart if the mounted CA material differs. Self-signed and single-cert CAs are unaffected (`root` == `cert`).
 - On upgrade, every DexAuthenticator that carries the
    `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotation gets a freshly generated client
    secret, because it previously received the client secret of the privileged `kubernetes` OAuth2 client
    instead. Its pods roll once and users of those applications may have to sign in again. Each rotation is
    written to the Deckhouse log as `Rotating DexAuthenticator client secret that reused the shared kubernetes
    client secret`, with the authenticator name and namespace.
    
    If that annotation was ever used outside the `d8-*` and `kube-*` namespaces, the shared secret must be
    considered disclosed to anyone who could read Secrets in those namespaces, and has to be rotated by hand.
    Deleting the Secret alone does not rotate it: while the value is non-empty in memory the owning hook
    re-renders the same one. Rotate it as follows.
    
    1. If a GitOps tool syncs the `d8-user-authn` namespace, exclude
       `Secret/kubernetes-dex-client-app-secret` from that sync first, otherwise it is restored to the old
       value.
    2. Blank the stored value:
       `kubectl -n d8-user-authn patch secret kubernetes-dex-client-app-secret --type merge -p '{"data":{"secret":""}}'`
    3. Drop the in-memory copy: `kubectl -n d8-system rollout restart deployment/deckhouse`
    4. Check that `kubectl -n d8-user-authn get secret kubernetes-dex-client-app-secret -o jsonpath='{.data.secret}'`
       now differs from the previous value. If it does not, the module re-rendered the old value before the
       restart took effect; repeat steps 2 and 3.
    
    The blast radius of that rotation is wider than the `kubernetes` client alone. The same value is the client
    secret of the `kubeconfig-generator`, `kubeconfig-publish-api` and every `kubeconfig-<slug>` OAuth2 client,
    and it is passed to basic-auth-proxy as `--ldap-client-secret`. In-cluster consumers are re-rendered and
    their pods roll automatically, but kubeconfig files already downloaded from the kubeconfig generator carry
    the old client secret and can no longer refresh their tokens, so their users must download a new
    kubeconfig. Already issued ID tokens keep working until they expire, at most `settings.idTokenTTL`.
    
    Deleting and recreating a DexAuthenticator is always a safe way to force a fresh per-authenticator secret:
    the credentials Secret is regenerated with the resource. Its content is owned by Deckhouse, so no GitOps
    change is needed for the per-authenticator rotation itself.
 - On upgrade, node-manager runs a one-time migration that seeds `global` ModuleConfig `spec.settings.prefix` from the existing `ClusterConfiguration.cloud.prefix`; no manual action is required. In-cluster consumers (cloud-controller-manager, CAPI/MCM MachineDeployment names, cluster-autoscaler priorities) now read the prefix from the ModuleConfig. Changing `spec.settings.prefix` in a running cluster is rejected by a new validating admission webhook, because the value names existing cloud infrastructure and changing it would recreate cloud nodes and rename base infrastructure.
 - Only `Reset2FA` (`d8 iam user reset2fa`) removes the 2FA secret. To make a user register the authenticator application again after `Lock` or `ResetPassword`, run `Reset2FA` too.
 - Only a subject allowed to update the user-authn ModuleConfig, or the Deckhouse service account, may add the `dexclient.deckhouse.io/allow-access-to-kubernetes` or `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotation. Objects that already carry it keep working and stay editable by their owners as long as they are updated in place. A GitOps controller that deletes and recreates such an object instead of patching it will have the recreation denied, and the object will come back without access to the Kubernetes API unless the controller's service account is allowed to update the user-authn ModuleConfig.
 - Permission expansion for existing bindings.
    
    - Users already bound to Editor (and above) will gain in-place pod resource resize (pods/resize: get,patch), which changes container requests/limits on running pods without recreating them.
    - Users already bound to Admin (and above) will gain the ability to mint ServiceAccount tokens (serviceaccounts/token: create) in their namespaces."
 - Pod Security Standards `baseline` and `restricted` now reject pods that set an SELinux `type` other than
    `container_t`, `container_init_t`, `container_kvm_t` or `container_engine_t`, or that set an SELinux `user` or `role`.
    Such pods were admitted before the update.
    A `SecurityPolicy` with `seLinux` entries now rejects an SELinux field that the matching entry does not list, except `level`.
    Before the update, review the workloads that set `seLinuxOptions` and cover the ones that need it with a `SecurityPolicyException`.
    A `SecurityPolicyException` for a hostPath with `readOnly: false` now also admits read-only mounts of the path.
 - Pods of the kube-proxy, cni-cilium, cni-flannel, and cni-simple-bridge agent DaemonSets will be restarted because their cni-migration-init-checker init container image is updated.
 - Project administrators (user-authz Admin access level) can no longer create, update or delete SecurityPolicyException resources; the permission now requires ClusterAdmin. Clusters that granted accessLevel Admin to untrusted tenants over RBAC v1 were exposed to node-level privilege escalation. RBAC v2 use/manage roles were never affected.
 - Project and cluster administrators can no longer change or delete objects labelled `projects.deckhouse.io/source` with the value `manifests`; change `spec.manifests` of the ProjectTemplate instead. Service accounts whose names start with system:serviceaccount:d8- may still change them. A rollout restart is still allowed when only the kubectl.kubernetes.io/restartedAt annotation of the pod template changes.
 - Project.spec.quota now applies in every namespace of a project, the additional ones included, and changing it needs the manage_quotas capability. A new namespace of a project with a quota refuses the objects of users for a few seconds until its ResourceQuota is calculated.
 - ProjectRoleBinding grants a role across every namespace of one project, ClusterProjectRoleBinding
    across every project including future ones, and ProjectNamespace orders an additional namespace
    inside a project -- replacing per-namespace RoleBindings maintained by hand.
    
    Project v1alpha3 carries `spec.administrators` and `spec.quota` as fields instead of template
    parameters, and reports `status.usage` and `status.namespaces`. ProjectTemplate v1alpha2 describes
    a project through structured fields rendered natively instead of a Helm string; the free-form
    `resourcesTemplate` keeps working. ProjectTemplate v1alpha1 is no longer served, and Project
    v1alpha1 is retired -- objects are converted on read.
    
    The project list now also contains the virtual projects `deckhouse` and `default`, so every
    namespace of the cluster belongs to a project.
 - Remote endpoints of multicluster peers may briefly disappear and come back
    during the upgrade.  Cross-cluster traffic served only by a remote peer can
    fail for a few seconds, so plan the upgrade accordingly if your multicluster
    setup is latency-sensitive.
 - Removing the Ingress class from an existing application configured with Gateway API removes its
    generated Ingress resources.
    
    An existing DexAuthenticator that specifies both an Ingress class and gatewayAPI now also gets an
    HTTPRoute in clusters where no Gateway is configured or discovered for the module. Previously such
    a route was not created at all. If the old behaviour was worked around with a hand-written route
    on /dex-authenticator, both now exist and Gateway API grants the match to the older one.
    
    A ListenerSet set for an application under additionalApplications before this release was dropped
    on write, because the field was missing from the v1 schema, which is the storage version. Adding
    the field does not restore the lost values: re-apply those DexAuthenticator manifests. The
    re-applied manifests are now also checked for domains already taken on the same ListenerSet, and
    for a domain repeated between additional applications.
 - Requests from `system:masters` to Project and ProjectTemplate are now validated like everyone else's (ProjectRoleBinding, ProjectNamespace and ClusterProjectRoleBinding always were). To bypass a Project or ProjectTemplate check deliberately, impersonate `system:sudouser` (`kubectl --as system:sudouser`).
 - Security and operation policies now validate pod-creating controllers
    (Deployment, StatefulSet, DaemonSet, Job, CronJob, ReplicationController)
    in addition to Pods. Set `podSecurityStandards.controllerValidation` to
    `false` to validate only Pods, as before.
    
    Controller checks evaluate the un-mutated pod template, so a workload that
    passes at Pod level only because a mutating component fills a field in may
    be denied at the controller level.
    
    A workload carrying `security.deckhouse.io/skip-pss-check` only on its pod
    template is now checked at the controller level. Set the label on the
    controller's top-level `metadata.labels` as well to exempt it completely.
    
    The webhook now intercepts CREATE, UPDATE and DELETE for these kinds, so
    that a policy can forbid deleting a resource. While Gatekeeper is
    unavailable, deleting such a resource is blocked along with creating it.
 - Setting the annotation no longer requires write access to the user-authn ModuleConfig. Module administrators are unaffected, as their roles already carry the new permission, and so are the platform's own components in `d8-system` and `kube-system`. Any other subject that sets the annotation, a cluster provisioner in particular, has to be granted `create` on the subresource, which for a single namespace is:
    
        apiVersion: rbac.authorization.k8s.io/v1
        kind: Role
        metadata:
          name: allow-access-to-kubernetes
          namespace: <namespace>
        rules:
        - apiGroups: ["deckhouse.io"]
          resources: ["dexclients/allow-access-to-kubernetes", "dexauthenticators/allow-access-to-kubernetes"]
          verbs: ["create"]
    
    bound to the subject with a RoleBinding in the same namespace, or with a ClusterRole and a ClusterRoleBinding to grant it cluster-wide.
 - SimpleWithInternalNetwork base-infrastructure and master-node Terraform state will drift on the next converge for clusters created before this change. The `openstack_networking_subnet_v2` data source selector moves from an unconditional `name = ...` to a conditional `subnet_id = ... / name = ...` pair (the unused side stays `null`). The data source result is identical, but Terraform will show the selector change in the plan and refresh the data block. No instance recreation or network reconfiguration is expected — review the plan before applying `dhctl converge`
 - Starting with this release, every newly created vSphere NodeGroup provisions its
    CloudEphemeral nodes via Cluster API (cluster-api-provider-vsphere, CAPV) instead
    of Machine Controller Manager. Existing NodeGroups keep running on MCM until
    explicitly migrated — see the FAQ "How to migrate node groups to Cluster API (CAPI)".
    
    Required cluster-side prerequisite: the vSphere cluster hosting CAPI nodes must
    have DRS enabled. CAPV relies on DRS for VM placement across hosts; the MCM
    pipeline tolerated clusters without DRS, CAPV does not. The same requirement
    already applied to master and CloudPermanent nodes (provisioned via Terraform).
 - Subjects holding only the user-authn view role or the legacy ClusterAdmin user-authn role can no longer list `passwords.dex.coreos.com` or `offlinesessionses.dex.coreos.com`. Use `User` objects for user inventory instead.
 - The Cilium agent pods, cilium-operator, and egress-gateway-agent will restart after the update.
 - The CoreDNS pods of kube-dns and the node-local-dns DaemonSet pods will restart after the update.
 - The Deckhouse web interface must use the new user-api endpoints for the password change and the sign-out on all devices.
 - The SSH key and sudo password in `SSHCredentials` (used by CAPS to reach static nodes) are now
    returned as `<omitted>` to callers without `get` on the `sshcredentials/sensitive` subresource,
    masked in the audit log, and encrypted in etcd when `apiserver.encryptionEnabled` is on.
    `d8:manage:infrastructure:viewer` and `:manager` no longer see these values. CAPS still does, and
    so does every holder of a `deckhouse.io` wildcard, since `*` matches subresources: `SuperAdmin`,
    the `kubeadm:cluster-admins` group, and the `deckhouse` and `webhook-handler` SAs of `d8-system`.
    
    For masked readers the `last-applied-configuration` annotation disappears from `get -o yaml`,
    and creating an `SSHCredentials` with `<omitted>` fails with `422 Invalid` — editing still works.
    Treat previously exposed keys as leaked and rotate them. With `apiserver.encryptionEnabled` on,
    rewrite existing objects to encrypt them: `d8 k get sshcredentials -o json | d8 k replace -f -`.
 - The `d8:manage:permission:subsystem:kubernetes:view_resources` role no longer grants `nodes/proxy`,
    `nodes/log`, `nodes/metrics` and `nodes/stats`. Users holding `d8:manage:kubernetes:viewer`,
    `d8:manage:infrastructure:viewer` or any role aggregating them lose access to the kubelet API through
    `/api/v1/nodes/<node>/proxy/...`. `nodes` themselves stay readable. Automation that relied on reaching
    the kubelet API with one of these roles must be granted a dedicated role explicitly.
 - The `dexclient.deckhouse.io/allow-access-to-kubernetes` and
    `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotations were previously
    evaluated by key presence, so any value — including "false" — granted the client the
    right to obtain tokens with `aud=kubernetes` and placed it into `trustedPeers` of the
    `kubernetes` OAuth2Client, whose tokens kube-apiserver accepts.
    
    The value is now parsed as a boolean. Only 1, t, T, TRUE, true and True grant the
    capability. Any other value, including "false", "0", "yes", "enabled" and an empty
    string, disables it; unparseable values are rejected with a warning in the
    deckhouse Pod log naming the object and the offending value.
    
    Affected: clusters with a DexClient or DexAuthenticator that carries the annotation
    with a value other than a boolean true. Those objects lose access to the Kubernetes
    API on update, which for most of them is the behaviour their author intended.
    
    Before updating, run
    `kubectl get dexclient,dexauthenticator -A -o json | jq -r '.items[] | select(.metadata.annotations | to_entries[]? | select(.key | endswith("deckhouse.io/allow-access-to-kubernetes")) | .value | ascii_downcase | IN("1","t","true") | not) | "\(.kind)/\(.metadata.namespace)/\(.metadata.name)"'`
    and, for every object listed, either set the annotation to "true" if it genuinely needs
    Kubernetes API access, or remove the annotation to confirm it does not.
 - The `vpa-recommender` pod is restarted and recommendations are recalculated with the new granularity.
    Memory recommendations will generally become lower (less over-provisioning), CPU recommendations become
    multiples of 10m. VPA objects with `updateMode: Auto`/`Recreate` may evict and recreate pods to apply
    the updated requests.
 - The cilium-hubble components (hubble-ui, hubble-relay) will restart after the update.
 - The cni-cilium components (cilium agent, operator) will restart after the update.
 - The control-plane components (kube-apiserver, kube-controller-manager, kube-scheduler), kubelet, and kube-proxy will restart after the update.
 - The controller recreates the main namespace of a Project once it is deleted directly past the
    admission policy, for example as `system:sudouser`, together with the objects of its template and of
    `manifests`. The other content of the namespace does not come back. A Project adopted from a namespace
    that has ownerReferences now carries the same owners, so the garbage collector deletes the Project
    together with the namespace and the namespace is not recreated. Only the controller and the
    identities the policy lets through may change the ownerReferences of a project namespace. An update
    that adds or changes an ownerReference of a Project or a ProjectNamespace is refused to everyone the
    multitenancy-manager webhooks validate, except the controller and Deckhouse.
    On the update, every Project that has lost its main namespace, or whose main namespace is stuck in
    Terminating, gets it back. Before the update, list these Projects and delete the ones you do not need.
    The list may also show Projects that are not restored: Projects in the Error state because their
    template is missing, and leftover Projects of the older model, which the controller deletes.
    d8 k get projects.deckhouse.io,namespaces -o json | jq -r '(INDEX(.items[] | select(.kind == "Namespace"); .metadata.name)) as $ns | .items[] | select(.kind == "Project" and .metadata.labels["projects.deckhouse.io/virtual-project"] != "true" and .spec.projectTemplateName != "virtual" and .metadata.deletionTimestamp == null) | .metadata.name | select($ns[.] == null or $ns[.].metadata.deletionTimestamp != null)'
 - The d8-system and d8-monitoring namespaces will use the privileged Kubernetes Pod Security enforcement level. Access to create workloads in these namespaces must remain restricted.
 - The default Pod Security Standards policy for non-system namespaces is now always `Baseline`.
    Previously, clusters bootstrapped with a Deckhouse version lower than v1.55 or with an unknown bootstrap version used `Privileged`.
    In such clusters, pods that do not comply with `Baseline` are denied when recreated if `enforcementAction` is `Deny`.
    The update is blocked while `settings.podSecurityStandards.defaultPolicy` is not set in the `admission-policy-engine` ModuleConfig
    and the cluster uses the `Privileged` policy derived from the bootstrap version.
    Before updating, set `settings.podSecurityStandards.defaultPolicy` explicitly.
    To keep the current behavior, set it to `Privileged`.
    To switch to `Baseline` or `Restricted`, first make sure that the workloads in non-system namespaces comply with the chosen policy.
 - The flanneld component will restart after the update.
 - The grant webhooks are now removed together with the module. On a cluster where `multitenancy-manager` is already disabled at the update, the `cluster-objects-grants-defaulting` and `cluster-objects-grants-validator` webhook configurations left by earlier versions stay, and writes to grantable resources in project namespaces (for example, removing a PVC finalizer) keep failing with `service "multitenancy-manager" not found`. Delete them by hand:
    `d8 k delete mutatingwebhookconfiguration cluster-objects-grants-defaulting`
    `d8 k delete validatingwebhookconfiguration cluster-objects-grants-validator`
    Nothing has to be done on clusters where the module is enabled.
 - The in-cluster registry storage (`registry` module, `Managed` mode with `storage.cache`) listens on the node address of master nodes on 4281/TCP instead of 5001/TCP and on 4283/TCP instead of 5003/TCP. Both are in the 4200–4299 range that the documented host firewall rules already open. If inter-node traffic is filtered by explicit rules, allow 4281/TCP from all nodes to master nodes and between master nodes, and 4283/TCP from the nodes running the Ingress controller to master nodes. The registry agent stays on 127.0.0.1:5001 on every node, and image references (`registry.d8-system.svc:5001`) do not change. An installation from a bundle uses 4281/TCP on the first master for its bootstrap registry, and the bootstrap preflight now checks that ports 4281 and 4283 are free.
 - The kube-proxy DaemonSet pods will restart after the update.
 - The kube-router component will restart after the update.
 - The kubeconfig generation web UI is removed and the `kubeconfig.<publicDomainTemplate>`
    hostname stops resolving; there is no redirect to a replacement page.
    Kubeconfigs already issued to users keep working: the dex OAuth2 clients behind them
    remain in place until DKP 1.79.
    Users should re-issue their kubeconfig from the Deckhouse console, which provides the
    same functionality.
    The `userAuthn.kubeconfigGenerator` module setting is not affected and keeps working —
    the console reads the API endpoints defined there.
    On clusters that served the UI with `https.mode: CertManager`, the TLS secrets issued for
    the removed hostname (`kubeconfig-ingress-tls`, plus `kubeconfig-httproute-tls` where
    Gateway API was used) remain in the `d8-user-authn` namespace after the upgrade. Their
    Certificates set `certificateOwnerRef: false`, so deleting them does not garbage-collect
    the secrets. The leftovers are unreferenced and no longer renewed; a migration in DKP 1.79
    removes them.
 - The kubelet restarts on every node. NodeGroups with `spec.kubelet.topologyManager` can get CPU and memory pinning turned on or off, and their nodes can be drained:
    - Memory reservations and eviction thresholds are now calculated from the node memory rounded down to GiB, so the allocatable memory changes slightly on every node.
    - `topologyManager.enabled` is deprecated and ignored: the managers are controlled by `policy` alone.
    - Groups with `enabled: false` (explicit or the stored default) and a `policy` other than `None` start pinning. Find them before the update: `d8 k get ng -o json | jq -r '.items[] | select(.spec.kubelet.topologyManager.enabled == false and (.spec.kubelet.topologyManager.policy // "None") != "None") | .metadata.name'`. Set `policy: None` where pinning must stay off.
    - Groups with `enabled: true` and `policy: None` stop pinning: Guaranteed pods lose their exclusive CPUs when recreated. Set `policy: BestEffort` to keep pinning.
    - On nodes with a `policy` other than `None`, `reservedMemory` is split between NUMA nodes, so nodes with Guaranteed pods are drained according to `disruptions` (`Manual` waits for approval).
    - `Restricted` and `SingleNumaNode` now give whole physical cores: on nodes with SMT, Guaranteed containers with an odd CPU count are rejected with `SMTAlignmentError`. Fix their requests before the update.
 - The metallb components (controller, speaker, l2lb) will restart after the update.
 - The minimum supported version of Kubernetes is now 1.33. All control plane components will restart.
 - The new `imdsv2` setting in `AWSClusterConfiguration` is opt-in and does nothing until it is set
    explicitly, so existing clusters are unaffected until then.
    
    Once `imdsv2: true` is set, DKP also sets the IMDS hop limit of every instance it manages to `1`,
    lowering it if the AWS account defaults or the AMI provide a higher value. With a hop limit of `1`
    the instance metadata service becomes unreachable from Pods that do not use the host network,
    including Pods whose applications support IMDSv2. DKP components are not affected, they run in the
    host network. Removing the setting afterwards stops DKP from managing the metadata options, but it
    does not restore the previous hop limit on instances that already have it: that has to be done
    through the AWS API.
    
    Applying the setting to master, static and bastion nodes requires `dhctl converge`; CloudEphemeral
    nodes are re-created automatically, which rolls every node of every affected NodeGroup.
 - The node-local-dns DaemonSet pods and the node-local-dns components will restart after the update.
 - The node-local-dns DaemonSet pods will restart after the update.
 - The openvpn components will restart after the update.
 - The resource group, the ViNS and the master node disks are deleted permanently, and the resource
    group is deleted by force: everything still inside it, including the disks of persistent volumes,
    is deleted with it and cannot be restored. Move the data you need out of the cluster before
    destroying it. The etcd disk of a master node is detached and deleted permanently whenever that
    master is removed, including by a converge that lowers `masterNodeGroup.replicas`.
 - The selector is evaluated against the union of Project and namespace labels, the namespace winning a shared key. Existing policies keyed on namespace labels (`projects.deckhouse.io/project`, `projects.deckhouse.io/project-namespace`, `projects.deckhouse.io/project-template`) behave as before.
    The CRDs now refuse `allowed: []`, an empty `allowedSelector`/`deniedSelector` and `apiGroups: ["*"]` in a GrantableClusterResourceReference; such objects were accepted before and either opened the resource or intercepted nothing. Existing objects are validated on their next write.
    `defaultFrom` counts only an annotation with the value `true`; a StorageClass carrying `storageclass.kubernetes.io/is-default-class: "false"` is no longer treated as the default.
    Policies carry a new `SelectorsValid` condition.
 - The settings of the `basic-auth` module are converted to version 2. The conversion deletes the following values:
    - a `whitelist` element that is not an IPv4 or IPv6 address or subnet, for example `all` or a host name;
      a location left without addresses requires a password, or denies every request when it has no `users`;
    - a `location` that is neither `/` nor a single path segment of Latin letters, digits, `.`, `_` and `-`,
      or equals `htpasswd`;
    - a repeated location, such as `/test` next to `test`.
    To keep access without a password, replace such `whitelist` values with addresses or subnets before updating.
    The `/healthz` path and, when `locations` has no `/`, every path outside the configured locations
    no longer answer `200` without authentication on port `80` of the `basic-auth` service.
    The `nginx` pods in the `kube-basic-auth` namespace restart.
 - The subsystem roles are `d8:subsystem:<subsystem>:<level>` for the eight subsystems. `deckhouse`, `infrastructure` and `kubernetes` are merged into `cluster`, `networking` is renamed to `network`, and user-authz, user-authn and multitenancy-manager move to `iam`, whose manager also assigns roles.
    The deprecated `d8:manage:deckhouse:*`, `d8:manage:infrastructure:*` and `d8:manage:kubernetes:*` roles keep the rights of the modules that were in these subsystems. Their replacement `d8:subsystem:cluster:*` grants more, so check which subsystem roles the subjects need before you recreate the bindings.
    A binding to the deprecated `d8:manage:security:<level>` role is replaced by bindings to both `d8:subsystem:iam:<level>` and `d8:subsystem:security:<level>`, because the rights for user-authn and user-authz and, at the `manager` level, the right to assign roles come with the `iam` role.
    A ClusterRoleBinding `user-authz:<rule>:additional-role:<role>` to a deprecated role is changed in `spec.additionalRoles` of the ClusterAuthorizationRule `<rule>`, and a binding that a project template renders is changed in the ProjectTemplate.
    No role of the new model collects the `rbac.deckhouse.io/aggregate-to-{deckhouse,infrastructure,kubernetes,networking}-as` labels. A custom capability or a module that still carries one reaches only the bindings to the deprecated names, and the `D8UserAuthzRetiredAggregationLabel` alert lists them.
    `d8:namespace:*` and `d8:project:*` do not collect the use capabilities of the previous scheme, `rbac.deckhouse.io/kind: use` with `rbac.deckhouse.io/aggregate-to-kubernetes-as`, nor the use roles of a module with a subsystem of its own, such as `d8:use:role:<level>:virtualization` with `rbac.deckhouse.io/aggregate-to-all-as`, and the `D8UserAuthzLegacyUseCapability` alert lists both. Until the module carries the new labels, give the subjects that need these rights a RoleBinding to the ClusterRole the alert names next to their RoleBinding to `d8:namespace:<level>`. Where the grants webhook of a project namespace refuses it, because the ClusterRole has no `rbac.deckhouse.io/delegatable` label, keep the RoleBinding on `d8:use:role:<level>`; it holds the next release until the module is updated.
    A ClusterRoleBinding to a deprecated `d8:manage:*` role keeps its `d8:use:viewer:binding:<name>` or `d8:use:user:binding:<name>` RoleBindings in the module namespaces while it refers to the old name. The deprecated `d8:use:role:<level>` and `d8:manage:all:<level>` roles keep the rights of the modules that still ship the old labels, such as log-shipper, prometheus-metrics-adapter and virtualization.
    The RoleBindings that the previous release created for a binding to a custom manage role of the old scheme are removed. Once the role is recreated as a `d8:custom:*` role of the system or subsystem scope, they come back in the namespaces of the modules that already carry the new labels. For a module that still carries the old labels, create RoleBindings in its namespace to `d8:namespace:<level>` and to the module ClusterRoles that the `D8UserAuthzLegacyUseCapability` alert names, and delete them once the module carries the new labels. A RoleBinding to `d8:use:role:<level>` there gives the same rights, but it holds the next release.
    A ClusterRole outside Deckhouse can no longer be created with `rbac.deckhouse.io/aggregate-to-all-as`, and an existing one can be updated only to remove that label.
 - Token exchange through a DexProvider is deprecated. If the alert fires, move the exchange to a TokenExchangeProvider as described in the CI/CD access documentation.
 - Users can no longer read or modify MachineDeployments, MachineSets, Machines, machine classes and CAPI infrastructure objects directly; these APIs are controller-owned. Manage nodes through NodeGroup and StaticInstance.
 - Users of applications behind a DexAuthenticator have to sign in again once after the update; the refresh tokens issued under the old client ids are deleted, and UserAccount no longer lists them. The dex-authenticator Deployments restart. The aud claim of the ID tokens of a DexAuthenticator becomes dex-authenticator-NAME@NAMESPACE; an application that checks aud has to accept the new value.
 - Users who run `kubectl exec`, `attach`, `cp`, `port-forward` or `debug` against Pods labeled `heritage: deckhouse`, or `kubectl scale` against Deployments, StatefulSets, ReplicaSets or ReplicationControllers labeled `heritage: deckhouse`, are now denied in every namespace, cluster administrators and superadmins included; only system users and the `system:masters` group (`super-admin.conf`) are exempt. Objects under maintenance can still be scaled. While webhook-handler is unavailable, these operations are denied for every target to everyone but the exempt users. Use `system:sudouser` where such access is needed.
 - Values taken from `Project.spec.parameters` are now quoted where they are substituted into the
    shipped project templates, and a parameter that changes the structure of the rendered manifests
    rather than only their values is refused for any template, including custom ones. An administrator
    name may no longer contain control characters, and `clusterLogDestinationName` must be a Kubernetes
    object name or empty. Both are checked from the reconcile loop as well as at admission, so a
    project already carrying such a value goes into an error state on its next reconcile rather than
    at its next edit; the same holds for a custom template that turns a parameter into several objects
    or into YAML, which is refused when the project is created, edited, or reconciled with a change to
    apply. The module's admission policy now also matches CREATE, so objects labelled
    `heritage: multitenancy-manager` can only be created by the module itself.
 - Values taken from `Project.spec.parameters` are quoted where they are substituted into the shipped
    project templates, and a parameter that changes the structure of the rendered manifests rather than
    only their values is refused for any template, including custom ones. An administrator name may no
    longer contain control characters, and `clusterLogDestinationName` must be a Kubernetes object name
    or empty. Both are checked from the reconcile loop as well as at admission, so a project already
    carrying such a value goes into an error state on its next reconcile rather than at its next edit;
    the same holds for a custom template that turns a parameter into several objects or into YAML,
    which is refused when the project is created, edited, or reconciled with a change to apply. The
    module's admission policy now also matches CREATE, so objects labelled
    `heritage: multitenancy-manager` can only be created by the module itself.
 - With ambient enabled, the ztunnel DaemonSet restarts. Until it has restarted on every node (and the same again on a rollback), traffic from nodes with the old ztunnel bypasses Service waypoints. AuthorizationPolicy, HTTPRoute, retries and timeouts on these waypoints are not applied during this time. Watch the ztunnel rollout until it finishes.
    In Services that mix sidecar-injected and non-injected pods, traffic distribution with `trafficDistribution: PreferClose` may change: non-injected pods now belong to the cluster network.
 - With both a ViNS and an external network, the guest preferred the ViNS default route and SSH to
    the external address never came up. On the first converge after this update, a master or static
    node created before it has both of its networks detached and attached again.
 - `ProjectTemplate` v1alpha1 is served with a deprecation warning that points at v1alpha2, and v1alpha2 carries `resourcesTemplate`. A template with a non-empty `resourcesTemplate` is rendered through Helm as before, from the project name, the template name and the project parameters. The form is deprecated; describe new templates with the structured fields.
    
    - A Helm template whose `parametersSchema` declares `administrators` or `resourceQuota` receives them from `spec.administrators` and `spec.quota`, in the layout of `Project` v1alpha2. Quota values arrive in canonical spelling (`1024Mi` as `1Gi`) unless the schema needs another one, and a zero arrives as the string "0".
    - A `Project` parameter `administrators` or `resourceQuota` that the template `parametersSchema` does not declare, as on the built-in templates, grants no access and sets no quota. `Project` admission refuses a new or changed value of such a parameter, except an empty list and a quota with no value other than the number 0, which pass with a warning. A value that an update keeps also passes with a warning, so a project that the conversion left them in keeps working. Set `spec.administrators` and `spec.quota` instead. In a v1alpha2 quota, leave out a limit of the number 0 to keep it unset, since the string "0" is a hard limit of zero.
    - The ResourceQuota and AuthorizationRule objects a Helm template renders are kept, next to `d8-project-quota` and `d8-administrators`. A pod has to fit every quota of its namespace.
    - The additional namespaces of a project on a Helm template are created and inherit the labels the template puts on the main namespace. Every Helm project release is upgraded once after the update.
    - `ProjectTemplate` admission refuses a non-empty `resourcesTemplate` together with fields that render objects. It refuses emptying the `resourcesTemplate` of a template that projects use, unless the same change sets such fields. It also refuses a write through v1alpha1 that would erase fields only v1alpha2 has.
    - A new administrator name cannot contain control characters or line breaks. A name that is already set is kept, with a warning.
 - `d8:manage:permission:module:multitenancy-manager:edit` no longer grants write access to
    `projecttemplates`. A `ProjectTemplate` renders into a release applied by a ServiceAccount bound
    to `cluster-admin`, and its `spec.resourcesTemplate` is arbitrary, so authoring one is equivalent
    to holding `cluster-admin` and is not a module-level permission.
    
    Reading templates stays in `d8:manage:permission:module:multitenancy-manager:view`, and creating a
    `Project` on an existing template stays in the edit role. The legacy `ClusterAdmin` access level
    and the `kubeadm:cluster-admins` group keep the right. A subject that authored templates through
    the manager role has to be granted one of those instead.
 - `dhctl bootstrap-phase abort` now always rolls back from the state cache and never deletes in-cluster resources, for CLI and gRPC clients alike. Use `dhctl destroy` to delete a running cluster. `--force-abort-from-cache` still parses but has no effect.
    
    The `bootstrap-phase` subcommands `execute-bashible-bundle` and `exec-post-bootstrap` are removed, along with `--internal-node-ip` and `--device-path`.
    
    The announced phase list changed for every operation (bootstrap gains 5 top-level phases, destroy gains 4, converge loses `ScaleToMultiMaster`), and `FirstMaster` / `WaitForControlPlaneManagerReadiness` moved from sub-phases to top-level phases. The gRPC message shape is unchanged; consumers with a hardcoded phase list or count need updating.
    
    Static and managed clusters now name their state-cache directory after the SSH host list instead of sharing one directory per machine. A static `bootstrap-phase abort` must be given the same `--ssh-host` set as its bootstrap.
 - `spec.oidc.scopes` items are now validated against `^\S+$` in both served versions of DexProvider. A scope containing whitespace could never work, because OAuth2 passes scopes as a space-delimited list, but it was previously admitted and silently broke authentication. Stored objects are not invalidated, however a DexProvider holding such a value is rejected on its next write until the value is corrected, which also affects a GitOps flow that reapplies the object unchanged.
 - `tracing.collector.zipkin.address` must now be in strict `<host>:<port>` format. Other values fail validation.
    In Telemetry API mode, the host must be the full service hostname, for example `zipkin.tracing.svc.cluster.local`.
 - cilium-agent pods are restarted on all nodes.
 - cni-flannel pods will be restarted.
 - istiod Pods for Istio 1.27+ restart. Pods created after the update get `istio-proxy` in `spec.initContainers` instead of `spec.containers`. Running Pods keep their sidecar type until they are recreated. Workloads with the `istio.deckhouse.io/auto-upgrade` label may be recreated automatically.
    New Pods don't start their application containers until istiod is available and the proxy is ready.
    The traffic of application init containers now goes through `istio-proxy` and follows the mesh rules: `outboundTrafficPolicyMode`, `Sidecar` egress settings, `AuthorizationPolicy` and mTLS. Before, in the `InitContainer` traffic redirection mode (the default), it bypassed the mesh. Check init containers that connect to services outside the mesh: with `outboundTrafficPolicyMode: RegistryOnly`, register these services with a `ServiceEntry`.
    Check scripts, admission policies, webhooks, alerts and dashboards that look for `istio-proxy` in `spec.containers` instead of `spec.initContainers`.
    The `istio-proxy` image for Istio 1.27+ no longer contains `bash`, `sh`, `curl`, `ss`, `grep`, `xargs`, `wc`, `sleep` and `echo`. Commands that run them in the `istio-proxy` container stop working.
    To temporarily keep regular sidecars, set `ENABLE_NATIVE_SIDECARS: "false"` in the `controlPlane.extraEnvs` parameter.
 - kube-proxy now waits for node-manager to finish applying the node template first, closing the race
 - kube-proxy pods will be restarted.
 - kube-scheduler will be restarted on all control-plane nodes.
    
    Pod placement changes only for pods that explicitly set `schedulerName: high-node-utilization`;
    the `default-scheduler` profile is not affected. Pods using this profile must declare resource
    `requests` — the scheduler scores nodes using `NonZeroRequested`, substituting 100m CPU / 200Mi
    memory for containers without requests, which makes bin packing operate on synthetic numbers.
 - metallb pods will be restarted.
 - network-policy-engine pods will be restarted.
 - node-controller, capi-controller-manager, machine-controller-manager and bashible-apiserver do not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - node-local-dns pods will be restarted.
 - registry-controller and registry-storage do not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - registry-packages-proxy does not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - simple-bridge pods will be restarted.

## Features


 - **[admission-policy-engine]** Add per-rule controller validation settings, `podSecurityStandards.controllerValidationExcludedPolicies` for the Pod Security Standards and `spec.controllerValidation` for OperationPolicy and SecurityPolicy.
 - **[admission-policy-engine]** Add the `allowSubPaths` parameter to SecurityPolicyException `hostPath` exceptions to allow paths nested under the specified one.
 - **[admission-policy-engine]** Add the `imageSignatureVerification.registryCredentialsCacheTTLSeconds` parameter. Registry credentials of image signature verification are now cached for 5 minutes instead of 12 hours.
 - **[admission-policy-engine]** Added a controller-level check to all rules that check pods. [#21556](https://github.com/deckhouse/deckhouse/pull/21556)
    Security and operation policies now validate pod-creating controllers
    (Deployment, StatefulSet, DaemonSet, Job, CronJob, ReplicationController)
    in addition to Pods. Set `podSecurityStandards.controllerValidation` to
    `false` to validate only Pods, as before.
    
    Controller checks evaluate the un-mutated pod template, so a workload that
    passes at Pod level only because a mutating component fills a field in may
    be denied at the controller level.
    
    A workload carrying `security.deckhouse.io/skip-pss-check` only on its pod
    template is now checked at the controller level. Set the label on the
    controller's top-level `metadata.labels` as well to exempt it completely.
    
    The webhook now intercepts CREATE, UPDATE and DELETE for these kinds, so
    that a policy can forbid deleting a resource. While Gatekeeper is
    unavailable, deleting such a resource is blocked along with creating it.
 - **[admission-policy-engine]** Added the `gpuResourceRestriction` policy to `OperationPolicy`, which denies Pods requesting GPU resources in namespaces without the configured label. [#22757](https://github.com/deckhouse/deckhouse/pull/22757)
 - **[admission-policy-engine]** Mutations limited by `spec.applyTo[].operations` no longer send other operations to Gatekeeper.
 - **[admission-policy-engine]** Publish the rules of the Pod Security Standards as the SecurityPolicy objects `d8-pod-security-baseline` and `d8-pod-security-restricted`.
 - **[admission-policy-engine]** Report the availability of the module components with alerts.
 - **[admission-policy-engine]** The alerts on policy violations fire for each policy and namespace, and an alert on a namespaced object is shown in its namespace.
    `PodSecurityStandardsViolation`, `OperationPolicyViolation` and `SecurityPolicyViolation` fire once for each policy and namespace with the `policy` and `namespace` labels instead of once for the cluster. With the observability module, an alert on a namespaced object is an ObservabilityAlert in that namespace instead of a ClusterObservabilityAlert; update the notification routing that relies on the cluster-wide alerts.
 - **[admission-policy-engine]** The default Pod Security Standards policy is `Baseline` regardless of the Deckhouse version the cluster was bootstrapped with.
    The default Pod Security Standards policy for non-system namespaces is now always `Baseline`.
    Previously, clusters bootstrapped with a Deckhouse version lower than v1.55 or with an unknown bootstrap version used `Privileged`.
    In such clusters, pods that do not comply with `Baseline` are denied when recreated if `enforcementAction` is `Deny`.
    The update is blocked while `settings.podSecurityStandards.defaultPolicy` is not set in the `admission-policy-engine` ModuleConfig
    and the cluster uses the `Privileged` policy derived from the bootstrap version.
    Before updating, set `settings.podSecurityStandards.defaultPolicy` explicitly.
    To keep the current behavior, set it to `Privileged`.
    To switch to `Baseline` or `Restricted`, first make sure that the workloads in non-system namespaces comply with the chosen policy.
 - **[candi]** Added golang v1.27.0 [#22452](https://github.com/deckhouse/deckhouse/pull/22452)
 - **[candi]** Added support for Kubernetes 1.37 and discontinued support for Kubernetes 1.32. Default Kubernetes version was changed 1.34->1.35.
    The minimum supported version of Kubernetes is now 1.33. All control plane components will restart.
 - **[candi]** Base images updated to v3 [#22986](https://github.com/deckhouse/deckhouse/pull/22986)
 - **[candi]** Deprecate `ClusterConfiguration.cloud.prefix` and cap its length at 45 characters. [#21533](https://github.com/deckhouse/deckhouse/pull/21533)
 - **[candi]** bashible installs nodelet on every node it manages, puts it back where it is missing, and waits for the platform's registry packages instead of installing them; static node cleanup removes the agent.
    A new node's bootstrap installs nodelet and waits for its NodeGroup's NodeConfigTemplate, served by node-controller, before it requests the bashible bundle.
 - **[cert-manager]** Add first-class Yandex Cloud DNS ACME DNS-01 support via ModuleConfig settings. [#22971](https://github.com/deckhouse/deckhouse/pull/22971)
 - **[cert-manager]** Add the `enableACMERenewalInfo`, `http01WaitInsteadOfSelfCheck` and `certificateRequestMaximumBackoffDuration` parameters to enable ACME Renewal Information, skip the HTTP-01 self-check of the Let's Encrypt ClusterIssuers and cap the retry backoff of failed CertificateRequests.
 - **[cert-manager]** Add the `highAvailability` parameter to control the HA mode of the module independently of the global setting.
 - **[cert-manager]** Update cert-manager to 1.21.2 and fix vulnerabilities in its images. The CRDs gain the fields of 1.20 and 1.21, such as Certificate renewal policies and `waitInsteadOfSelfCheck` of the ACME solvers.
    The `cert-manager`, `cainjector` and `webhook` pods will be restarted.
    Issuers and ClusterIssuers that set `zoneType` of the Azure DNS solver must be applied again: the API server pruned the field before the update.
    With `enableCAInjector: true`, cainjector always uses server-side apply and takes over the managed fields of the objects annotated with `cert-manager.io/inject-ca-*`.
 - **[cert-manager]** `cert-manager` uses the cluster-wide proxy server from the `proxy` parameter of `ClusterConfiguration`.
    In a cluster with the `proxy` parameter of `ClusterConfiguration` set, the `cert-manager` controller and the Yandex Cloud DNS webhook
    send requests to the ACME server, HashiCorp Vault and the DNS provider APIs through the proxy server.
    The `HTTP-01` self-check also requests the domain through the proxy server.
    If the proxy server cannot reach the issuer or the public address of the cluster, add these addresses to `noProxy`.
 - **[chrony]** Master nodes check and correct their clocks from NTP before kubelet starts.
    If the configured NTP servers are unreachable, a master boot can wait up to 30 seconds by default. On bashible nodes this limit can be raised to 60 seconds. Kubelet starts after the timeout and NodeTimeNotSynchronizedAtBoot fires after the failure metric is collected. For isolated clusters, configure reachable ntpServers or set earlyTimeSync.enabled to false.
 - **[chrony]** The `d8-chrony` namespace enforces the restricted Pod Security Standard, with a SecurityPolicyException for the chrony pods.
    The `chrony` and `chrony-master` pods are restarted on all nodes.
 - **[cloud-provider-aws]** IMDS v2 support. The new `imdsv2` setting in `AWSClusterConfiguration` enforces token-based IMDSv2 on all EC2 instances of the cluster. [#21988](https://github.com/deckhouse/deckhouse/pull/21988)
    The new `imdsv2` setting in `AWSClusterConfiguration` is opt-in and does nothing until it is set
    explicitly, so existing clusters are unaffected until then.
    
    Once `imdsv2: true` is set, DKP also sets the IMDS hop limit of every instance it manages to `1`,
    lowering it if the AWS account defaults or the AMI provide a higher value. With a hop limit of `1`
    the instance metadata service becomes unreachable from Pods that do not use the host network,
    including Pods whose applications support IMDSv2. DKP components are not affected, they run in the
    host network. Removing the setting afterwards stops DKP from managing the metadata options, but it
    does not restore the previous hop limit on instances that already have it: that has to be done
    through the AWS API.
    
    Applying the setting to master, static and bastion nodes requires `dhctl converge`; CloudEphemeral
    nodes are re-created automatically, which rolls every node of every affected NodeGroup.
 - **[cloud-provider-dynamix]** Basis Dynamix disk placement is set by the name of a storage policy instead of a storage endpoint and a pool.
    Breaking change of the DynamixClusterConfiguration and DynamixInstanceClass contract. There is no
    migration and no upgrade path: a cluster created on an earlier version has to be reinstalled.
    `storageEndpoint` and `pool` are removed, and `storagePolicy` replaces them: mandatory at the
    root of DynamixClusterConfiguration, optional as an override in every instanceClass, including
    `masterNodeGroup.instanceClass` and DynamixInstanceClass.
    The configuration cannot be prepared in advance: the schema a running cluster validates against
    rejects `storagePolicy` as an unknown field, so the fix cannot be made before the update, and
    after it the stored configuration no longer passes validation and the cluster stops converging.
    StorageClasses are now created one per `ENABLED` storage policy of the account instead of one per
    storage endpoint, so their names change and their parameters no longer carry `location`, `pool` or
    `storageEndpoint`. Review any PersistentVolumeClaim that names a StorageClass explicitly: the
    names of the reinstalled cluster are not the names of the old one. A policy that the account
    loses or that stops being `ENABLED` takes its StorageClass with it; an answer with no `ENABLED`
    policy at all is taken for a glitch and keeps the StorageClasses the cluster has. The module, which had no settings before, now takes `storageClass.exclude`
    to keep some of those StorageClasses out of the cluster, and a deprecated `storageClass.default`.
    Two DynamixMachine defaults move with the CRD: `rootDiskSizeGb` defaults to 30 instead of 20, and
    `cpu` is no longer a required field.
    The CSI driver no longer reports storage capacity, and the cloud-data-discoverer,
    cloud-controller-manager, CSI driver and CAPI controller images are all updated together.
 - **[cloud-provider-dynamix]** Changing the Basis Dynamix storage policy recreates the CloudEphemeral nodes it applies to.
    Basis Dynamix chooses where a disk lives when the virtual machine is created and the provider
    never moves a disk afterwards, so a new storage policy reaches a node only by recreating it.
    Editing the cluster-wide `storagePolicy` recreates the CloudEphemeral nodes of every node group —
    including groups that set their own `storagePolicy` in their instanceClass and whose own
    configuration did not change.
    Master and static nodes are not recreated: they stay in the policy they were created in, and a new
    policy reaches them only through a node created anew.
 - **[cloud-provider-dynamix]** Nodes of the Basis Dynamix `Standard` layout get their DNS servers from `nameservers`.
    On the `Standard` layout a node has only the external network, which may hand out no DNS server.
    The servers listed in `nameservers` are now written into the cloud-init of every master, static and
    CloudEphemeral node of that layout, and reach only the nodes created afterwards.
    `nameservers` now accepts IPv4 addresses only, in both layouts: a configuration with a host name or
    an IPv6 address in it is rejected by validation. Fix it before updating.
 - **[cloud-provider-dynamix]** The Basis Dynamix provider requires platform version 4.6 or newer.
    Basis Dynamix 4.6 made `storage_policy_id` a mandatory field of `disks/create` and
    `kvmx86/create`, and it is the first version that answers on the storage policy API the provider
    now depends on. Earlier platform versions are no longer supported: virtual machines cannot be
    created there at all.
    Check the platform version before updating. `dhctl bootstrap` refuses to deploy a new cluster on
    an older platform, but an existing cluster on one has no working path forward.
    The account permissions change with it: the `sep` user API group is no longer used at all, and
    the `storage_policy` group is now required.
 - **[cloud-provider-openstack]** Support using a precreated Octavia load balancer or a preallocated Floating IP for ingress LoadBalancer Services. [#21736](https://github.com/deckhouse/deckhouse/pull/21736)
 - **[cloud-provider-openstack]** add `preemptible` field to OpenStackInstanceClass for Selectel-hosted preemptible instances [#21866](https://github.com/deckhouse/deckhouse/pull/21866)
 - **[cloud-provider-vsphere]** Add custom CA bundle support for vSphere [#21245](https://github.com/deckhouse/deckhouse/pull/21245)
 - **[cloud-provider-vsphere]** ship capv-controller-manager v1.15.3 with external CRDs and validating webhooks
    Starting with this release, every newly created vSphere NodeGroup provisions its
    CloudEphemeral nodes via Cluster API (cluster-api-provider-vsphere, CAPV) instead
    of Machine Controller Manager. Existing NodeGroups keep running on MCM until
    explicitly migrated — see the FAQ "How to migrate node groups to Cluster API (CAPI)".
    
    Required cluster-side prerequisite: the vSphere cluster hosting CAPI nodes must
    have DRS enabled. CAPV relies on DRS for VM placement across hosts; the MCM
    pipeline tolerated clusters without DRS, CAPV does not. The same requirement
    already applied to master and CloudPermanent nodes (provisioned via Terraform).
 - **[cloud-provider-yandex]** Add ru-central1-f zone support via explicit zone selection with a four-zone limit.
    Existing clusters are unaffected; the default zone set (ru-central1-a, ru-central1-b, ru-central1-d, ru-central1-e) is statically pinned. To use ru-central1-f, list it in `zones` instead of one of the default zones; the dropped zone's subnet is replaced by a new subnet in ru-central1-f and the nodes in it are recreated.
 - **[cloud-provider-yandex]** Added the `storageClass.provision` parameter, which allows creating additional StorageClasses and setting the block size of the disks they provision. [#22311](https://github.com/deckhouse/deckhouse/pull/22311)
 - **[cloud-provider-yandex]** Migrated module configuration from YandexClusterConfiguration to ModuleConfig. [#21653](https://github.com/deckhouse/deckhouse/pull/21653)
    Cloud provider configuration for DKP clusters deployed in Yandex Cloud needs to be migrated from YandexClusterConfiguration to ModuleConfig settings. For migration instructions, refer to "Cluster and Infrastructure" in the DKP FAQ.
 - **[cloud-provider-yandex]** Validate that `network-ssd-nonreplicated` and `network-ssd-io-m3` disks are sized in multiples of 93 GiB.
 - **[cloud-provider-zvirt]** The module is configured through ModuleConfig settings, NodeGroups, ZvirtInstanceClasses and a credentials Secret instead of ZvirtClusterConfiguration.
    On clusters configured with `ZvirtClusterConfiguration`, the module keeps working from it and prepares the migration resources in the `d8-migration-resources` Secret of the `d8-cloud-provider-zvirt` namespace.
    The `D8CloudProviderZvirtMigrationPending` alert fires until these resources are applied; follow the alert description to complete the migration.
    The zVirt credentials move to the `d8-credentials` Secret. A validating webhook now checks the module ModuleConfig, NodeGroups, ZvirtInstanceClasses and the credentials Secret.
    CCM, CSI, cloud-data-discoverer and capz-controller-manager pods of the module may restart. Existing VMs and disks are not recreated.
 - **[cni-cilium]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    Cilium agent pods will be restarted. With DSR, the DSR IPv4 option is stripped before the packet reaches the pod.
 - **[cni-cilium]** Added an alert and a release requirement that block the Cilium update while CiliumNetworkPolicy or CiliumClusterwideNetworkPolicy objects use the `fromRequires` or `toRequires` fields.
 - **[cni-cilium]** Experimental Network Peering feature.
 - **[cni-cilium]** Support network.deckhouse.io/networks-spec annotation [#21885](https://github.com/deckhouse/deckhouse/pull/21885)
 - **[cni-flannel]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    Flannel pods will be restarted.
 - **[cni-simple-bridge]** Added a CVE-2026-98432 mitigation that filters IPv4 TCP SYN packets with IP options in the host and pod network namespaces (`ipv4OptionsGuard`; `Enforce` for new clusters, `Audit` for existing ones).
    simple-bridge pods will be restarted.
 - **[common]** Aggregated APIs that opt in with the `scope.deckhouse.io/resources` annotation on their APIService support scoped listings (`d8 k get -A --scope`).
    kube-apiserver image is rebuilt; control-plane components restart during the update.
 - **[control-plane-manager]** A control plane component is updated on a node only after its new images are pulled there, so it no longer stays down while kubelet pulls them.
 - **[control-plane-manager]** A scheduler extender can receive the pods kube-scheduler placed since its previous prioritize call (`sendPlacements` in KubeSchedulerWebhookConfiguration).
 - **[control-plane-manager]** Cluster network parameters podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix are migrated to mc control-plane-manager spec.settings.network; these parameters in `cluster-configuration` are deprecated. [#22688](https://github.com/deckhouse/deckhouse/pull/22688)
 - **[control-plane-manager]** Control-plane components requests are autotuned daily from 7-day average usage instead of a fixed split of a combined budget; setting `resourcesRequests.cpu` or `.memory` in ModuleConfig disables autotune for that measurement. [#21543](https://github.com/deckhouse/deckhouse/pull/21543)
    restart of control-plane static pods (`kube-apiserver`, `etcd`, `kube-controller-manager`, `kube-scheduler`).
 - **[control-plane-manager]** ControlPlaneOperation status shows the diffs of files changed by the operation, except files that may contain sensitive data.
 - **[control-plane-manager]** The basic audit policy logs Pod status updates from kubelet with the request body, so container terminations and their reasons get into the audit log.
 - **[control-plane-manager]** The cluster Kubernetes version moves from `ClusterConfiguration.kubernetesVersion` to `settings.kubernetesVersion` in ModuleConfig `control-plane-manager`. [#21828](https://github.com/deckhouse/deckhouse/pull/21828)
 - **[control-plane-manager]** `ControlPlaneNode` component conditions are now pod-based; new `<Component>InSync` and `<Component>PodReady` conditions, `kubectl get cpn` columns show `<Component>Ready`.
 - **[control-plane-manager]** kube-scheduler exposes metrics of scheduler extender requests and the list of configured extenders, named after their KubeSchedulerWebhookConfiguration.
 - **[control-plane-manager]** kube-scheduler exposes which score plugin or extender decided each node choice, metrics of the simulate endpoint, and an opt-in decision log published as `/scheduler/decisions`.
 - **[control-plane-manager]** kube-scheduler pauses calls to an ignorable scheduler extender for 30 seconds after 10 failed requests in a row.
    A scheduler extender with `failurePolicy - Ignore` that fails 10 filter or prioritize requests in a row gets no filter and prioritize requests for 30 seconds; pods are scheduled without its filter and scores meanwhile.
 - **[control-plane-manager]** kube-scheduler serves a dry-run scheduling endpoint through control-plane-proxy; access is granted by RBAC.
    kube-scheduler restarts once to get the new environment variable. control-plane-proxy runs on master nodes also when the prometheus module is disabled. While a simulation runs, real scheduling on that replica waits for it.
 - **[csi-vsphere]** Adds Pod Security Standards labels and SecurityPolicyExceptions for CSI pods, and makes cloud-data-discoverer compliant with the restricted policy; CSI and cloud-data-discoverer pods restart once on upgrade.
 - **[deckhouse-controller]** Add the `x-deckhouse-immutable` OpenAPI extension to mark settings fields as editable only at application creation. [#22876](https://github.com/deckhouse/deckhouse/pull/22876)
 - **[deckhouse-controller]** Add the `x-deckhouse-ui-group` OpenAPI extension to render a settings field inside a named group in the web console form. [#22975](https://github.com/deckhouse/deckhouse/pull/22975)
 - **[deckhouse-controller]** An Application ServiceAccount with the `packages.deckhouse.io/application-endpoint-description` annotation adds the in-cluster Kubernetes API address to `status.urls`.
 - **[deckhouse-controller]** An application package version can set `versionCompatibilityRules.upgrade.from` in `package.yaml`, a SemVer constraint on the versions an Application may be upgraded to it from; the constraint takes priority over the `step` limits, and an invalid one forbids upgrading to the version.
 - **[deckhouse-controller]** Application package settings schemas support the x-deckhouse-ui-text-area and x-deckhouse-ui-secure-field UI marks.
 - **[deckhouse-controller]** When an Application is deleted, Deckhouse also deletes the objects of the kinds listed in the package's `orphanResources` value that the application left in its namespace.
 - **[deckhouse]** Added repositoryName to usedBy for AP/MP. [#22749](https://github.com/deckhouse/deckhouse/pull/22749)
 - **[deckhouse]** Added the `D8DeckhouseModuleImageDownloadInterrupted` alert that fires when the download of a module image is interrupted.
 - **[deckhouse]** Added the experimental `enableModuleV2` setting, which turns on the Module v2 controller and installs the Module v2 CRD. [#23273](https://github.com/deckhouse/deckhouse/pull/23273)
 - **[deckhouse]** Application `spec.resourceRequests` accepts autoscaling ranges (`minReplicas`/`maxReplicas` and container `min`/`max`) that the package's HorizontalPodAutoscaler and VerticalPodAutoscaler apply; a range without an autoscaler falls back to fixed values and raises an alert.
 - **[deckhouse]** Application `status.urls` now includes endpoints of HTTPRoutes annotated with `packages.deckhouse.io/application-endpoint-description`.
 - **[deckhouse]** Application packages can be granted to projects with ClusterResourceGrantPolicy; a project sees the packages available to it, and an Application of any other package is rejected.
    With the `multitenancy-manager` module enabled, its webhooks check Application objects in project namespaces. While the module controller is unavailable, Application objects in project namespaces cannot be created or changed.
 - **[deckhouse]** Application.spec.resourceRequests is now honoured by default.
    An Application that declares `spec.resourceRequests` is now installed from a chart
    rendered with the requested replicas and container resources applied, instead of
    from the package chart directly. Applications without `resourceRequests`, and all
    modules, are unaffected. Updating restarts the deckhouse Deployment.
 - **[deckhouse]** Applications (and modules with EnableModuleV2) run on a new package engine that keeps the running version during updates, applies update windows to applications and does not reinstall packages on restart.
 - **[deckhouse]** Applications in the `Auto` and `AutoPatch` update modes are updated automatically along a chain of compatible versions; the plan and the next version are shown in `status.update`.
 - **[deckhouse]** Auto-restart kubelet on resources added by hotplug
 - **[deckhouse]** Deleting an application whose package requires disable confirmation now needs the `packages.deckhouse.io/allow-disabling` annotation; with Module v2 modules use the same annotation, which is removed once the module is disabled.
    With Module v2 enabled, disabling a module that requires confirmation needs `packages.deckhouse.io/allow-disabling` instead of `modules.deckhouse.io/allow-disabling`.
 - **[deckhouse]** Enable Pod Security Standards restricted checks in warn mode on the d8-monitoring namespace. [#22024](https://github.com/deckhouse/deckhouse/pull/22024)
 - **[deckhouse]** Every authenticated user can read ApplicationPackageVersion objects.
 - **[deckhouse]** Implement module v2 controller. [#21857](https://github.com/deckhouse/deckhouse/pull/21857)
 - **[deckhouse]** Make packages nelm operation timeout configurable [#21604](https://github.com/deckhouse/deckhouse/pull/21604)
 - **[deckhouse]** Makes CCM create NPs on every lb creation while using networkPolicy - Restricted
 - **[deckhouse]** Module v2 gets its own Grafana dashboard and last-run, phase and hook lane metrics.
    With enableModuleV2 the hook run metrics are labelled by package instead of module, and the task queue, compaction and helm operation metrics are no longer exported.
 - **[deckhouse]** Packages can declare upgrade and downgrade compatibility limits; the Application webhook rejects incompatible version changes, with defaults allowing patch changes and upgrades to the next minor version.
 - **[deckhouse]** Support application settings conversions.
 - **[deckhouse]** The `d8a5e-` object name prefix is reserved for the ansible module. [#22411](https://github.com/deckhouse/deckhouse/pull/22411)
 - **[deckhouse]** The `d8vt-` object name prefix is reserved for the virtualization-transfer module.
 - **[deckhouse]** The `spec.resourceRequests` values of an Application are also applied to the HorizontalPodAutoscaler and VerticalPodAutoscaler deployed for the workload, so the autoscalers no longer override them.
 - **[deckhouse]** With the new package engine, a module runs from a registry tag set by the `packages.deckhouse.io/dev-tag` annotation; existing ModulePullOverrides are migrated to it.
 - **[descheduler]** Added the `protectedStorageClasses` parameter to the Descheduler custom resource, which forbids evicting Pods that use a PersistentVolumeClaim from the listed StorageClasses.
 - **[descheduler]** LowNodeUtilization evicts a pod only if kube-scheduler would place it on an underutilized node.
    On clusters where control-plane-manager runs kube-scheduler, LowNodeUtilization skips pods whose target node is not underutilized, and pauses evictions while kube-scheduler does not answer.
 - **[descheduler]** The LowNodeUtilization strategy of a Descheduler can count thresholds from the average node utilization (`useDeviationThresholds`) and limit evictions per node (`evictionLimits.node`).
 - **[dhctl]** Added a preflight check that prevents static cluster bootstrap when pod or service CIDRs overlap with the target host network. [#21815](https://github.com/deckhouse/deckhouse/pull/21815)
 - **[dhctl]** Added support for Yandex Cloud as an external cloud provider — its candi, OpenTofu plugin and validator are downloaded at runtime from the module bundle. [#21653](https://github.com/deckhouse/deckhouse/pull/21653)
    Cloud provider configuration for DKP clusters deployed in Yandex Cloud needs to be migrated from YandexClusterConfiguration to ModuleConfig settings. For migration instructions, refer to "Cluster and Infrastructure" in the DKP FAQ.
 - **[dhctl]** An Immutable first master can be bootstrapped with any registry mode, not only Unmanaged.
 - **[dhctl]** Bootstrap phases are driven by one declarative tree, with `--skip-phase`, per-cluster state-cache identities, and support for clusters without ClusterConfiguration; `bootstrap-phase abort` no longer destroys a running cluster. [#22182](https://github.com/deckhouse/deckhouse/pull/22182)
    `dhctl bootstrap-phase abort` now always rolls back from the state cache and never deletes in-cluster resources, for CLI and gRPC clients alike. Use `dhctl destroy` to delete a running cluster. `--force-abort-from-cache` still parses but has no effect.
    
    The `bootstrap-phase` subcommands `execute-bashible-bundle` and `exec-post-bootstrap` are removed, along with `--internal-node-ip` and `--device-path`.
    
    The announced phase list changed for every operation (bootstrap gains 5 top-level phases, destroy gains 4, converge loses `ScaleToMultiMaster`), and `FirstMaster` / `WaitForControlPlaneManagerReadiness` moved from sub-phases to top-level phases. The gRPC message shape is unchanged; consumers with a hardcoded phase list or count need updating.
    
    Static and managed clusters now name their state-cache directory after the SSH host list instead of sharing one directory per machine. A static `bootstrap-phase abort` must be given the same `--ssh-host` set as its bootstrap.
 - **[dhctl]** Bootstrapping a Basis Dynamix cluster is refused when the platform is older than 4.6 or the storage policy is not visible to the cluster account.
 - **[dhctl]** Refactor preflights in dhctl. [#23020](https://github.com/deckhouse/deckhouse/pull/23020)
 - **[dhctl]** Resolve the cloud prefix from the `global` ModuleConfig at bootstrap/converge/destroy without writing it into the in-cluster `d8-cluster-configuration` secret. [#21533](https://github.com/deckhouse/deckhouse/pull/21533)
 - **[dhctl]** Teach dhctl to accept and use cloud provider resources (the new format that replaces ProviderClusterConfiguration) in attach, check, converge, destroy and detach.
 - **[dhctl]** When the master cannot reach the registry or the cloud API, dhctl says whether it is DNS that fails.
 - **[dhctl]** dhctl uses the SSH user certificate from <key>-cert.pub next to each --ssh-agent-private-keys key, so it can log in to servers and bastions that trust only an SSH CA.
 - **[docs]** Documented the replica limit annotations and the `x-deckhouse-ui-text-area`, `x-deckhouse-ui-secure-field`, and `x-deckhouse-ui-sizing-memory` settings schema extensions for application packages.
 - **[docs]** On the site you can read the documentation of any version of any module, not only the versions of the release channels.
 - **[documentation]** In the cluster documentation you can read the documentation of any version of any module, enabled or not, before you enable or update it.
 - **[infra-provider-baremetal]** A new `infra-provider-baremetal` module has been added for integration with bare-metal servers.
 - **[istio]** Added `envoyExtAuthzHttp`, `opentelemetry` and `zipkin` types to `dataPlane.extensionProviders`.
 - **[istio]** Added support for Istio 1.29. [#21693](https://github.com/deckhouse/deckhouse/pull/21693)
 - **[istio]** Bind multicluster peer CAs to the mesh trust domain [#22441](https://github.com/deckhouse/deckhouse/pull/22441)
 - **[istio]** IstioFederation and IstioMulticluster report precise reasons for metadata exchange failures in status conditions.
 - **[istio]** Kiali can be read by in-cluster clients over a verified TLS connection
 - **[istio]** Periodic Istio configuration analysis monitoring (EE) [#21349](https://github.com/deckhouse/deckhouse/pull/21349)
 - **[istio]** Support providing a custom Istio CA via a Secret reference (`ca.secretRef`), with strict validation and safe rotation handling. [#21219](https://github.com/deckhouse/deckhouse/pull/21219)
 - **[istio]** Use Kubernetes native sidecars for Istio 1.27+ by default.
    istiod Pods for Istio 1.27+ restart. Pods created after the update get `istio-proxy` in `spec.initContainers` instead of `spec.containers`. Running Pods keep their sidecar type until they are recreated. Workloads with the `istio.deckhouse.io/auto-upgrade` label may be recreated automatically.
    New Pods don't start their application containers until istiod is available and the proxy is ready.
    The traffic of application init containers now goes through `istio-proxy` and follows the mesh rules: `outboundTrafficPolicyMode`, `Sidecar` egress settings, `AuthorizationPolicy` and mTLS. Before, in the `InitContainer` traffic redirection mode (the default), it bypassed the mesh. Check init containers that connect to services outside the mesh: with `outboundTrafficPolicyMode: RegistryOnly`, register these services with a `ServiceEntry`.
    Check scripts, admission policies, webhooks, alerts and dashboards that look for `istio-proxy` in `spec.containers` instead of `spec.initContainers`.
    The `istio-proxy` image for Istio 1.27+ no longer contains `bash`, `sh`, `curl`, `ss`, `grep`, `xargs`, `wc`, `sleep` and `echo`. Commands that run them in the `istio-proxy` container stop working.
    To temporarily keep regular sidecars, set `ENABLE_NATIVE_SIDECARS: "false"` in the `controlPlane.extraEnvs` parameter.
 - **[multitenancy-manager]** A Project can grant its namespaces cluster-wide resources of its own in spec.grants, on top of the grants of its template.
 - **[multitenancy-manager]** A ProjectQuota can limit the load balancers of a project by class, by pool and by external address.
 - **[multitenancy-manager]** A ProjectQuota can limit the virtual machines of a project by their cores, guaranteed CPU, memory and GPUs.
 - **[multitenancy-manager]** A namespace that would stay outside every project cannot be created directly any more, and a namespace that stays without a project raises the D8NamespaceNotAdopted alert.
    Creating a namespace whose name is longer than 61 characters, starts with upmeter- or is deckhouse, or putting the heritage: upmeter label on a namespace, is refused to everyone but system components, cluster administrators (system:masters) and system:sudouser, whatever allowNamespacesWithoutProjects says. A namespace that existed before and belongs to no project, for example one whose Project stays in Error because the namespace belongs to another Helm release, raises the D8NamespaceNotAdopted alert after the upgrade.
 - **[multitenancy-manager]** A project is a unit of access with its own bindings, additional namespaces and structured templates. [#21846](https://github.com/deckhouse/deckhouse/pull/21846)
    ProjectRoleBinding grants a role across every namespace of one project, ClusterProjectRoleBinding
    across every project including future ones, and ProjectNamespace orders an additional namespace
    inside a project -- replacing per-namespace RoleBindings maintained by hand.
    
    Project v1alpha3 carries `spec.administrators` and `spec.quota` as fields instead of template
    parameters, and reports `status.usage` and `status.namespaces`. ProjectTemplate v1alpha2 describes
    a project through structured fields rendered natively instead of a Helm string; the free-form
    `resourcesTemplate` keeps working. ProjectTemplate v1alpha1 is no longer served, and Project
    v1alpha1 is retired -- objects are converted on read.
    
    The project list now also contains the virtual projects `deckhouse` and `default`, so every
    namespace of the cluster belongs to a project.
 - **[multitenancy-manager]** ClusterResourceGrantPolicy v1alpha2 selects projects by Project labels and namespaces by a separate namespaceSelector; additional namespaces inherit the labels of the project template.
    ClusterResourceGrantPolicy is stored as multitenancy.deckhouse.io/v1alpha2. A policy written through v1alpha1 keeps selecting the same namespaces: its projectSelector becomes the v1alpha2 namespaceSelector. To select projects by labels on the Project object, use spec.projectSelector of v1alpha2.
    When a policy manifest moves from v1alpha1 to v1alpha2, rename its projectSelector to namespaceSelector. A manifest that changes only its apiVersion keeps projectSelector, which v1alpha2 matches against the labels of the Project, so a selector by a label that only the namespace has matches nothing, and the policy stops applying to those namespaces.
    Labels a ProjectTemplate sets through namespaceMetadata.labels are now also set on additional namespaces, and namespaceMetadata.labels can no longer override the pod security, monitoring and scanning labels of the template.
    Each project is re-rendered once after the update; this resets hand edits of template labels on its main namespace.
    The projects.deckhouse.io/project and projects.deckhouse.io/project-template labels of a Project can no longer be pointed at another project or template. A manifest that replaces a whole Project may leave them out. Such an update is accepted with a warning, and the controller puts the labels back.
    During the update from 1.77, create or change ClusterResourceGrantPolicy through v1alpha1 only after the webhook-handler of the deckhouse module has rolled out: until it registers the conversion, such a write is stored with its selector read as a Project selector.
    All ClusterResourceGrantPolicy objects are stored as v1alpha2 soon after the update, because the controller writes their status. Deckhouse does not support downgrades: a manual rollback to a version without v1alpha2 makes every policy unreadable.
 - **[multitenancy-manager]** Every namespace outside a project is adopted into a full project with a template inferred from the namespace, and the project managed by its namespace is gone. [#22609](https://github.com/deckhouse/deckhouse/pull/22609)
    Breaking changes for clusters that relied on the previous model.
    
    - An auto-wrapped namespace used to be freely deletable, and deleting it deleted its project. Its namespace is an ordinary project namespace now, so the admission policy refuses to delete or edit it; delete the `Project` instead.
    - Namespaces that used to be orphans are project namespaces now, so the cluster-resource grant allow-list applies to them. RoleBindings to non-delegatable ClusterRoles in those namespaces start being denied.
    - The spec of a previously namespace-managed project is no longer controller-owned and can be edited directly; its labels and annotations are no longer mirrored from the namespace.
    - The `projects.deckhouse.io/adopt` annotation no longer does anything; adoption is automatic.
    - `allowNamespacesWithoutProjects` is deprecated and ignored, and will be removed in the next release.
    - A leftover wrap whose namespace is already gone or Terminating is deleted instead of Helm-recreating the namespace.
 - **[multitenancy-manager]** GrantableClusterResourceDefinition gets the `reserved` field for resources that are unavailable to projects by default and are granted only by an explicit `allowed` or `allowedSelector` in ClusterResourceGrantPolicy.
 - **[multitenancy-manager]** GrantableClusterResourceReference picks the `fieldPaths` item by its `match` condition too, and `fieldPaths[].message` adds a text to the rejection.
 - **[multitenancy-manager]** In Enforce, creating an object that takes the usage of a whole project over its ProjectQuota is denied.
    A new fail-closed webhook, quota-ledger.multitenancy-manager.deckhouse.io, checks the status writes of d8-project-quota. While no replica of multitenancy-manager answers, counted creates in the namespaces of projects with a quota fail with 503; deletions and other namespaces are not affected. multitenancy-manager runs two replicas where control-plane-manager is enabled.
 - **[multitenancy-manager]** In the CE edition, a project has only its main namespace, and the creation of a ProjectNamespace is refused.
    Additional namespaces of a project are available in the commercial editions. After a switch from a commercial edition to CE, the existing additional namespaces leave their projects: the controller stops managing them, the objects of the template in them stay as they are, and their RoleBindings only lose what a binding revokes.
 - **[multitenancy-manager]** Objects that `spec.manifests` of a ProjectTemplate creates can be changed only through the template, as the objects of the structured fields can.
    Project and cluster administrators can no longer change or delete objects labelled `projects.deckhouse.io/source` with the value `manifests`; change `spec.manifests` of the ProjectTemplate instead. Service accounts whose names start with system:serviceaccount:d8- may still change them. A rollout restart is still allowed when only the kubectl.kubernetes.io/restartedAt annotation of the pod template changes.
 - **[multitenancy-manager]** ProjectQuota limits a project as a whole, with a ResourceQuota in every namespace of the project and the usage summed over them.
    Project.spec.quota now applies in every namespace of a project, the additional ones included, and changing it needs the manage_quotas capability. A new namespace of a project with a quota refuses the objects of users for a few seconds until its ResourceQuota is calculated.
 - **[multitenancy-manager]** ProjectTemplate `deckhouse.io/v1alpha2` gets the `manifests` field, a template string whose namespaced objects the controller applies to each project as a user of the project with the `d8:project:admin` role, not with its own rights.
    The objects of `manifests` get the `projects.deckhouse.io/project` and `projects.deckhouse.io/source: manifests` labels, are listed in `.status.manifestObjects` of the project and are deleted when the template stops rendering them. The `ManifestsApplied` condition of the project names the objects that are refused, with the code of each refusal. Cluster-scoped objects, objects in other namespaces and existing objects that are not the project's are not applied. An object that a switch from a Helm text left in place is taken over when `manifests` renders it under the same kind, namespace and name.
    
    A new `resourcesTemplate` can no longer be set through `deckhouse.io/v1alpha2`. A create with a non-empty one, and an update that sets one on a template that has none, are refused. A template that has a Helm text keeps it and can change it, and requests through `deckhouse.io/v1alpha1` work as before. A Helm-text template saved as `deckhouse.io/v1alpha2`, for example by `kubectl get -o yaml` or in a backup, is created again through `deckhouse.io/v1alpha1`, and its `title`, `resources` or `grantPolicies` are then added through `deckhouse.io/v1alpha2`.
    
    Only the multitenancy-manager controller and `system:sudouser` may bind users and groups whose names start with `system:multitenancy-manager:`, and such names are refused as project administrators.
 - **[multitenancy-manager]** ProjectTemplate v1alpha2 is structured-only; the Helm resourcesTemplate string is removed from the API and a template that carried one is parked for rewriting instead of being rendered. [#23011](https://github.com/deckhouse/deckhouse/pull/23011)
    Breaking change for clusters with custom ProjectTemplates written as Helm strings.
    
    - `spec.resourcesTemplate` no longer exists in `ProjectTemplate` v1alpha2; a manifest that carries it is refused by the schema.
    - A template stored as v1alpha1 with a Helm string is converted without the string and marked with the `projects.deckhouse.io/legacy-helm-template: "true"` annotation. Projects of such a template switch to the `Error` state with the `ProjectTemplateUsable` condition and are not rendered; their existing objects are not touched. Rewrite the template with structured fields and remove the annotation to resume.
    - The built-in `simple` template renders the required-requests OperationPolicy only when the new `requiredRequests` parameter is `true`; existing projects on `simple` keep rendering the namespace only.
    - The `TemplateResourcesFiltered` and `TemplateRolesAllowed` project conditions and the `projects.deckhouse.io/unmanaged` and `projects.deckhouse.io/skip-heritage-label` labels are removed together with the Helm-string path.
 - **[multitenancy-manager]** The MultitenancyManagerDeprecatedConfigParameter alert fires while a ModuleConfig still sets the deprecated and ignored allowNamespacesWithoutProjects parameter, which is removed in the next minor release. [#23124](https://github.com/deckhouse/deckhouse/pull/23124)
    If your `multitenancy-manager` ModuleConfig sets `allowNamespacesWithoutProjects`, remove the parameter. It has no effect, and in the next minor release a ModuleConfig that still carries it will fail validation and stop applying.
 - **[multitenancy-manager]** The `manifests` of a ProjectTemplate can make additional namespaces of the project with ProjectNamespace objects and put objects into them; such objects are applied once the namespace exists.
 - **[multitenancy-manager]** The new `MultitenancyManagerHelmProjectTemplate` alert names every user ProjectTemplate that still renders its projects from a Helm `resourcesTemplate`, and says how to move it to structured fields and what a switch does to the objects of its projects.
 - **[multitenancy-manager]** The objects of `manifests` of a project template cannot include an AuthorizationRule of the `Admin` access level, and a project template is created through `deckhouse.io/v1alpha2` only.
    A ProjectTemplate can no longer be created through `deckhouse.io/v1alpha1`, and no template can get a new `resourcesTemplate` through either version. Existing templates with a Helm text keep working and can be changed, but a backup restore, a GitOps bootstrap or a re-creation of such a template is refused until it is moved to structured fields and `manifests`. The `MultitenancyManagerHelmProjectTemplate` alert names these templates.
    An AuthorizationRule of the `Admin` access level in `manifests` is refused when it is created or when an update raises a rule to this level, and the project switches to `Error`. A write of a project whose `manifests` render such a rule, and a write of a template whose `manifests` describe an AuthorizationRule, are admitted with a warning. Set the administrators of a project in `.spec.administrators` of the Project instead.
 - **[node-manager]** A NodeOperation shows in `status.eviction` how many pods its eviction still waits for and which of them are terminating.
 - **[node-manager]** Add cluster-autoscaler 1.35 support with ported Deckhouse patches and CVE dependency bumps. [#21777](https://github.com/deckhouse/deckhouse/pull/21777)
 - **[node-manager]** Add support for cloud-provider-specific Bashible lifecycle steps stored in Kubernetes Secrets. [#22108](https://github.com/deckhouse/deckhouse/pull/22108)
 - **[node-manager]** Added a release requirement that can block updates while nodes are still running containerd v1.x. [#21994](https://github.com/deckhouse/deckhouse/pull/21994)
 - **[node-manager]** Added spec.providerType field for NodeGroup to support future migration to per-NodeGroup provider resolution. Static groups no longer apply cloud provider scripts. Bashible-apiserver now resolves cloud provider configuration per NodeGroup. [#22050](https://github.com/deckhouse/deckhouse/pull/22050)
    NodeGroups with `nodeType: Static` no longer get the cloud provider's node configuration scripts.
    Until now every NodeGroup in a cloud cluster, Static included, was configured with them.
    
    On AWS this removes `007_enable_ecr_credential_provider` from Static nodes: the kubelet unit is
    regenerated without `--image-credential-provider-config`, kubelet restarts and loses access to
    ECR. The same applies to any other provider step a Static node relied on.
 - **[node-manager]** Alert when a NodeGroup uses the deprecated built-in GPU support while the `gpu` module is disabled. [#22142](https://github.com/deckhouse/deckhouse/pull/22142)
 - **[node-manager]** CAPI machine templates use the v2 provider contract - one file per provider, generation-numbered names, and a rollout decision made by comparing InstanceClass values instead of hashing serialized bytes. [#21793](https://github.com/deckhouse/deckhouse/pull/21793)
 - **[node-manager]** Deckhouse Engine nodes get a maintenance web UI and console screen with a temporary network edit (nodelet v0.1.23, Engine image v0.1.20).
    Deckhouse Engine nodes reboot into the new OS image by their NodeGroup update rules. On these nodes the new agent restarts containerd once.
 - **[node-manager]** Deckhouse Engine nodes move to Engine v0.1.16, whose kernel has the softdog watchdog built in for fencing.
 - **[node-manager]** Fencing 2.0 replaces fencing 1.0. The modes are Normal, Slow and Critical; Critical is available only in the commercial editions. Fencing evacuates a failed node only after the node is guaranteed to be stopped.
    Fencing no longer deletes the pods of a node after 60 seconds without a confirmation that the node stopped. The fencing-controller sets the `node.kubernetes.io/out-of-service` taint only on a stopped node: after a witness confirms the stop, or after the node is silent for a delay of its profile. Then it deletes the pods of the node and the Node of a cloud Cluster API machine or the Machine of an MCM node.
    A node with fencing stops when it does not see the control plane longer than the threshold of its mode (4.15, 9.1 or 20.2 seconds). A loss of the control plane for the whole cluster that is longer than this stops all nodes with fencing.
    On update, Deckhouse replaces all fencing agents at the same time, the nodes do not reboot. The `Watchdog` mode works as `Slow`. The `spec.fencing.watchdog.timeout` parameter of NodeGroup is deprecated and ignored. A rollback to fencing 1.0 is not supported.
    You cannot turn fencing on in a cluster with one master node. You cannot change the fencing mode of a node group while a node of the group has a FencingFailedNodeState.
    To disable the `node-manager` module, turn fencing off in all node groups first and confirm the disabling.
    Open port 8500 TCP and UDP between all cluster nodes.
 - **[node-manager]** Migrate defaultCRI from ClusterConfiguration to node-manager ModuleConfig [#21482](https://github.com/deckhouse/deckhouse/pull/21482)
 - **[node-manager]** Migrate the cloud object prefix from `ClusterConfiguration.cloud.prefix` to the `global` ModuleConfig `spec.settings.prefix`. [#21533](https://github.com/deckhouse/deckhouse/pull/21533)
    On upgrade, node-manager runs a one-time migration that seeds `global` ModuleConfig `spec.settings.prefix` from the existing `ClusterConfiguration.cloud.prefix`; no manual action is required. In-cluster consumers (cloud-controller-manager, CAPI/MCM MachineDeployment names, cluster-autoscaler priorities) now read the prefix from the ModuleConfig. Changing `spec.settings.prefix` in a running cluster is rejected by a new validating admission webhook, because the value names existing cloud infrastructure and changing it would recreate cloud nodes and rename base infrastructure.
 - **[node-manager]** Node-controller supplies the chrony NTP servers to Deckhouse Engine master nodes for boot-time clock synchronization.
 - **[node-manager]** NodeStaticPodRequest delivers a static pod to the nodes of selected Deckhouse Engine NodeGroups. The pause image comes with the containerd extension and is never pulled. [#23158](https://github.com/deckhouse/deckhouse/pull/23158)
    Every Engine node's NodeConfig is re-rendered once, and containerd restarts once on every Engine node.
 - **[node-manager]** NodeUser uid uniqueness, StaticInstance address uniqueness and InstanceClass deletion protection are now validated by the node-controller admission webhook instead of shell hooks. [#21654](https://github.com/deckhouse/deckhouse/pull/21654)
 - **[node-manager]** Prevent deletion of CAPI Cluster resources in the d8-cloud-instance-manager namespace. [#22551](https://github.com/deckhouse/deckhouse/pull/22551)
 - **[node-manager]** Qemu guest agent/sysext [#23339](https://github.com/deckhouse/deckhouse/pull/23339)
 - **[node-manager]** `kubernetes-api-proxy` starts its probe rounds on a fixed schedule (every second, as before), probes every 500ms for up to 30s while no kube-apiserver is ready, and shows the probe state in `/upstreams`.
    The `kubernetes-api-proxy` container restarts on every node. While no kube-apiserver is ready, each node sends `/readyz` to every kube-apiserver up to 2 times a second for up to 30s. While a kube-apiserver answers slowly, each node probes every kube-apiserver up to 1.5 times as often as before.
 - **[node-manager]** `spec.kubelet.topologyManager.policy` now selects a complete kubelet profile, and NUMA-dependent kubelet settings are calculated on each node.
    The kubelet restarts on every node. NodeGroups with `spec.kubelet.topologyManager` can get CPU and memory pinning turned on or off, and their nodes can be drained:
    - Memory reservations and eviction thresholds are now calculated from the node memory rounded down to GiB, so the allocatable memory changes slightly on every node.
    - `topologyManager.enabled` is deprecated and ignored: the managers are controlled by `policy` alone.
    - Groups with `enabled: false` (explicit or the stored default) and a `policy` other than `None` start pinning. Find them before the update: `d8 k get ng -o json | jq -r '.items[] | select(.spec.kubelet.topologyManager.enabled == false and (.spec.kubelet.topologyManager.policy // "None") != "None") | .metadata.name'`. Set `policy: None` where pinning must stay off.
    - Groups with `enabled: true` and `policy: None` stop pinning: Guaranteed pods lose their exclusive CPUs when recreated. Set `policy: BestEffort` to keep pinning.
    - On nodes with a `policy` other than `None`, `reservedMemory` is split between NUMA nodes, so nodes with Guaranteed pods are drained according to `disruptions` (`Manual` waits for approval).
    - `Restricted` and `SingleNumaNode` now give whole physical cores: on nodes with SMT, Guaranteed containers with an odd CPU count are rejected with `SMTAlignmentError`. Fix their requests before the update.
 - **[node-manager]** added olcedar image [#22059](https://github.com/deckhouse/deckhouse/pull/22059)
 - **[node-manager]** learn about shutdown without logind for engine [#23072](https://github.com/deckhouse/deckhouse/pull/23072)
 - **[node-manager]** migrate get_crds hook into node-controller and move machine objects along. [#21372](https://github.com/deckhouse/deckhouse/pull/21372)
 - **[node-manager]** node-controller arbitrates `RegistryUpstream` objects without the registry agent and routes them through `NodeConfig`; nodelet applies them on both node types, bashible inline on containerd v1.
    nodelet is updated to v0.1.21 and restarts on every node that runs it; workloads are not affected.
 - **[registry]** Bound the cache store by `storage.size` and a node disk reserve, with alerts; fix fill, replication and push routing to the leader; describe a cluster pulling through the agent as the in-cluster registry only; stabilize multi-master bootstrap.
 - **[registry]** Garbage collection of the registry cache now also runs once the cache holds a new image set and when the store refuses writes, and can be started by hand with a RegistryOperation object. Each storage replica exports an estimate of what a collection would free.
    The `registry-controller` and `registry-storage` pods restart once.
 - **[registry]** Migration part of new registry implementation. [#22886](https://github.com/deckhouse/deckhouse/pull/22886)
 - **[registry]** New registry implementation — the pull path on every node is managed by an agent, with an optional in-cluster cache and an air-gap transition. [#21788](https://github.com/deckhouse/deckhouse/pull/21788)
    Bring the registry to `Unmanaged` BEFORE updating.
    
    This release renders none of the previous registry implementation's objects. A cluster that
    arrives still in `Direct` or `Proxy` loses them on the first reconciliation — the in-cluster
    proxy, the `registry` Service and the per-node configuration they served — while its nodes go
    on pointing at that address. Images already on a node keep working; anything that has to be
    pulled does not, and the migration cannot be completed from that state. The way back is to
    return to the previous release, bring the cluster to `Unmanaged` there, and update again.
    
    A cluster already in `Unmanaged` is unaffected. The handover happens on its own, and the new
    implementation manages nothing until it is configured, so the update changes no behaviour.
    
    An air-gapped cluster in `Local` has a procedure of its own, which also starts before the
    update: an OCI registry stood up outside Deckhouse's namespaces, loaded with `d8 mirror pull`
    and `d8 mirror push`, declared as the upstream for the length of the migration and removed once
    the in-cluster storage holds the image set. It costs a second copy of that set on disk while it
    exists. See "How do I migrate an air-gapped Local cluster?" in the registry module's FAQ.
    
    From this release the platform records which implementation a cluster runs, so a later release
    can refuse to install on a cluster still running the previous one.
 - **[registry]** Removed the v1 registry implementation (used up to and including DP 1.77); the Proxy and Local modes of the deckhouse ModuleConfig can be neither installed nor updated to 1.78.
    An update to 1.78 from DP 1.77.1 or later is blocked while the cluster runs Proxy or Local of the v1 registry implementation, or Direct without the registry ModuleConfig.
    Migrate the cluster to the registry module on 1.77 before updating (see the registry module FAQ), and update to 1.78 only from DP 1.77.1 or later.
 - **[registry]** The images of the in-cluster cache are listed by the RegistryImage resource; the cache checks its blobs in the background and repairs broken ones on its own.
    The registry cache replicas are recreated one at a time, and registry-controller restarts.
 - **[registry]** The managed registry (cache, proxy, air-gap) now works on Deckhouse Engine (Immutable) nodes.
 - **[registry]** The registry-agent's `d8_registry_agent_*` metrics are collected by Prometheus through the node exporter textfile collector (`job="node-exporter"`, label `node`).
 - **[registry]** `RegistryUpstream` fields are validated the way the nodes read them and are size-limited; an invalid object gets `InvalidSpec`, one over the cluster budget gets `LimitExceeded`.
 - **[registry]** `RegistryUpstream` objects are applied on the nodes when the module is disabled or in `Unmanaged` mode.
 - **[user-authn]** A DexProvider can put a prefix in front of every group it asserts (spec.groupPrefix); the groups of providers stay apart when every provider has its own prefix.
    Setting or changing groupPrefix renames the groups of the provider and deletes its refresh tokens, so its users sign in again; rewrite the rules that name its groups before setting it.
 - **[user-authn]** A UserOperation of the `Lock` type with `spec.target` locks a user of any identity provider, including OIDC, SAML, GitHub, GitLab, and Bitbucket Cloud.
    The lock applies to one pair of a provider and a user and is checked at sign-in, at refresh-token requests, and at token exchange. The lock applies to the user ID that the provider returns, so a SAML provider must request NameID format `persistent` or `emailAddress` (`nameIDPolicyFormat`), not `transient`. Basic Authentication through a Crowd or OIDC provider does not go through Dex, so the lock does not apply to it.
 - **[user-authn]** A security key (WebAuthn) can be the second factor, alone or after the code (twoFactor.methods, require: All), for all users or by the type of provider; one key per account, and Reset2FA can remove one method.
 - **[user-authn]** Add TokenExchangeProvider, a trusted issuer whose tokens Dex exchanges for Dex tokens accepted by the Kubernetes API.
 - **[user-authn]** Add the D8UserAuthnDexProviderTokenExchangeDeprecated alert for clusters that exchange tokens through a DexProvider.
    Token exchange through a DexProvider is deprecated. If the alert fires, move the exchange to a TokenExchangeProvider as described in the CI/CD access documentation.
 - **[user-authn]** Added `postLogoutRedirectURIs`, `backchannelLogoutURI` and `refreshTokenLifetime` to DexClient and `upstreamLogout` to OIDC DexProvider.
 - **[user-authn]** Added browser sessions of Dex (the `sessions` parameters, off by default) with single sign-on for the web interfaces of the platform and a sign-out that ends the session of the browser in every DexAuthenticator.
    With `sessions.enabled`, kubeconfig and applications of DexClient objects that request offline access still get tokens only after an interactive sign-in. An application of a DexAuthenticator signs out with a navigation of the browser: a sign-out by a script request does not end the session of Dex.
 - **[user-authn]** Added the devices of a user: `UserAccount.status.sessions`, `logout.grant` and `logout.exceptGrant` of the `Logout` UserOperation, and the user-api endpoints for listing the devices and signing out of one device or of the others.
    A user signed in to one application in several browsers or with several kubeconfig files stays signed in on all of them, up to 10 per application; the 11th sign-in signs the least recently used device out of that application. During the update, a device other than the last one signed in to an application may have to sign in again once. In the web interface, signing out on all devices and changing the password are available only on a device whose browser first signed in to the account at least 24 hours ago; signing out and in again from the same browser keeps that time.
 - **[user-authn]** Dex is updated to v2.46.0.
    Dex pods restart. 2FA (TOTP) secrets, locks, and refresh tokens issued before the update keep working, so users keep their authenticator application, and kubeconfig files and applications behind DexAuthenticator do not require a new sign-in.
    With 2FA enabled, Dex no longer shows the consent screen at sign-in.
    UserAccount shows the time of the last sign-in and whether the user has set up 2FA.
 - **[user-authn]** Dex sends the back-channel logout of a browser session ended by a UserOperation, by the self-service of the web interface or by the deletion of the user.
 - **[user-authn]** DexAuthenticator logins, logouts, failed and denied logins, CSRF mismatches and failed session refreshes are delivered as security events when the `security-events-manager` module is enabled.
 - **[user-authn]** DexAuthenticator pages use the Deckhouse Dex design and the language of the browser (Russian or English).
    All DexAuthenticator pods restart.
 - **[user-authn]** DexAuthenticator runs oauth2-proxy 7.15.5, checks the TLS certificate of Dex, uses PKCE, and answers script requests without a session with `401` instead of a redirect to Dex.
    All DexAuthenticator pods restart. While a DexAuthenticator rolls out, its old and new replicas cannot read each other's session cookies and sign-in state. A user whose request reaches an old replica after a new one served them is signed out once, and a sign-in in progress may end with the "Login Failed" page once; signing in again works. With two replicas this affects most active users of the DexAuthenticator once. During the rollout the authenticator logs "CSRF token mismatch" auth failures. A `fetch` or XHR request without a session gets `401` with the `{}` body instead of a redirect to Dex that failed with a CORS error. To send the user to sign in, the application must reload the page when it gets `401`.
 - **[user-authn]** DexClient and DexAuthenticator can limit the providers users sign in through with spec.allowedProviders.
 - **[user-authn]** DexProvider of the LDAP type can follow nested groups (recursionGroupAttr) and take several attributes for the username (usernames).
 - **[user-authn]** Every device in UserAccount `status.sessions` and in the user-api device list shows when its session ends (`endsAt`).
 - **[user-authn]** Lock, password and 2FA resets, the new Logout operation and user deletion end the sessions of the user at once in Dex, DexAuthenticator, the Kubernetes API and Basic Authentication through LDAP, and cancel sign-ins in progress.
    Dex, DexAuthenticator, user-authn-controller, user-api and basic-auth-proxy pods restart. Tokens issued before the update keep working until the first revocation of their user, so nobody has to sign in again.
    A `Lock` of an external user through `spec.target` now ends the user's sessions.
    Deleting a UserIdentity object by hand is refused. To remove a local user, delete the User; to reset 2FA or a lock, use a UserOperation.
    DexAuthenticator receives revocations from the `revocation-feed` Service in `d8-user-authn`. If a NetworkPolicy restricts egress in the namespace of a DexAuthenticator, allow traffic to the `user-authn-controller` pods on port `8443`; otherwise the `D8UserAuthnRevocationFeedNotConsumed` alert fires.
 - **[user-authn]** New UserOperation types `Logout` and `RequirePasswordChange`; `spec.target` needs only `email`.
 - **[user-authn]** Reconcile users, passwords, operations, and Dex provider checks in a dedicated controller instead of module hooks. [#22151](https://github.com/deckhouse/deckhouse/pull/22151)
 - **[user-authn]** Signing out of an application behind DexAuthenticator revokes the refresh token of the session in Dex.
 - **[user-authn]** The `Lock` and `ResetPassword` operations no longer reset the user's two-factor authentication (2FA) secret.
    Only `Reset2FA` (`d8 iam user reset2fa`) removes the 2FA secret. To make a user register the authenticator application again after `Lock` or `ResetPassword`, run `Reset2FA` too.
 - **[user-authn]** The module copies the 2FA (TOTP) secrets and locks of Dex users into Dex v2.46 UserIdentity objects and keeps a UserAccount for users of every identity provider.
    UserAccount objects appear for users of OIDC, SAML, GitHub, GitLab, and Bitbucket Cloud providers, with `providerType` set to `spec.type` of the DexProvider.
    The controller creates UserIdentity objects (`useridentities.dex.coreos.com`) for users who have signed in; the `D8UserAuthnIdentityMigrationIncomplete` alert fires if it cannot copy their data for 30 minutes.
 - **[user-authn]** The second factor (TOTP) can be required from the users of LDAP and Crowd providers with the twoFactor parameter, and Reset2FA works for them.
    The staticUsers2FA parameter is renamed twoFactor; the module converts it, so existing settings keep working. twoFactor.providerTypes with LDAP or Crowd is refused while a provider of that type has enableBasicAuth, and the other way round. Turning the second factor on does not end the sessions already issued: end them with a Logout UserOperation. A version without twoFactor refuses a version 4 ModuleConfig of user-authn with twoFactor in its webhook, and Deckhouse stops with a Fatal error; before rolling back to such a version, switch the ModuleConfig to version 3 and remove twoFactor (staticUsers2FA sets the second factor of the local users there).
 - **[user-authn]** Token exchange through a DexProvider is off by default and can only be turned off with `spec.tokenExchange.enabled`.
    Existing clusters with a DexClient that has the dexclient.deckhouse.io/allow-access-to-kubernetes annotation keep token exchange through their OIDC and Gitlab DexProviders through the flag, which the module sets on the upgrade. Move the exchange to a TokenExchangeProvider and turn the flag off. The flag and token exchange through a DexProvider will be removed after the next LTS release.
 - **[user-authn]** Update the Dex authentication page to the new Deckhouse branding, with light and dark themes selected by the user's system settings. [#23093](https://github.com/deckhouse/deckhouse/pull/23093)
 - **[user-authn]** With twoFactor.require: Any each user chooses the second factor, a one-time code or a security key, and registers it at the first sign-in.
    A version without require: Any refuses a ModuleConfig of user-authn that sets it, and Deckhouse stops with a Fatal error; before rolling back to such a version, switch require to All or keep one method.
 - **[user-authz]** Bindings of ClusterAuthorizationRule and AuthorizationRule are reconciled by the new `user-authz-controller` instead of the module Helm chart; rules get a status subresource with a `Ready` condition. [#22828](https://github.com/deckhouse/deckhouse/pull/22828)
    The module deploys a new `user-authz-controller` Deployment in `d8-user-authz`. Existing bindings are adopted in place; access is not interrupted.
    `kubectl get clusterauthorizationrules` and `kubectl get authorizationrules` show READY and BINDINGS columns; a rule with an unsupported `accessLevel` or an invalid `additionalRoles` entry gets `Ready=False` instead of breaking the module and no longer blocks the other rules.
    Disabling the module now removes the bindings of the rules, as it did when the chart rendered them.
    
    The release that performs the migration costs one pass in which the release engine fetches every
    object of the previous manifest to see that the bindings must be kept. It is proportional to the
    number of bindings and happens exactly once. Up to about 5000 bindings it just makes that one
    release slower; above that it can exceed the 20-minute module release timeout, so run the upgrade
    with `HELM_TIMEOUT=60m` or on the previous release engine (`USE_NELM=false`) and put the setting
    back afterwards. Count with
    `kubectl get clusterrolebindings -l heritage=deckhouse,module=user-authz --no-headers | wc -l`.
    If the marks cannot be set the release does not start and the bindings are left untouched, which
    is deliberate: a release that ran without them would delete the bindings.
 - **[user-authz]** In multi-tenancy mode on Kubernetes 1.34 and later, the API server refuses ID tokens revoked by user-authn within seconds.
    The new `D8UserAuthzRevocationCheckNotSynced` alert fires when a user-authz webhook instance has not loaded the revocations for 10 minutes; revoked Dex tokens keep working through that master until they expire.
 - **[user-authz]** Metrics and alerts for `user-authz-controller` — desired/actual bindings and their drift per kind, invalid rules by name, controller availability, aggregated custom roles that stay empty. [#22837](https://github.com/deckhouse/deckhouse/pull/22837)
    New alerts: `D8UserAuthzControllerUnavailable`, `D8UserAuthzControllerTargetDown`, `D8UserAuthzControllerReconcileErrorsHigh`, `D8UserAuthzBindingsDrift`, `D8UserAuthzAuthorizationRulesInvalid`, `D8UserAuthzAuthorizationRuleInvalid`, `D8UserAuthzCustomRolesNotAggregated`.
 - **[user-authz]** New authorization API answers who can perform an action and what a subject is allowed to do. [#21846](https://github.com/deckhouse/deckhouse/pull/21846)
    The `authorization.deckhouse.io/v1alpha1` group serves BulkSubjectAccessReview (many access checks
    in one request), AccessibleNamespace (the namespaces a user may see, without cluster-wide
    `list namespaces`), WhoCan (the subjects allowed to perform an action) and SubjectAccessReport
    (everything a subject is allowed to do, with the origin of each row). Reports about other subjects
    require the `nonself` subresource, granted by dedicated checker roles.
 - **[user-authz]** Occupying an identity that already carries a grant, or writing a ClusterAuthorizationRule, now requires covering permissions or an explicit can-assign range. [#22667](https://github.com/deckhouse/deckhouse/pull/22667)
    A subject who can manage Users or Groups cannot delete or recreate an identity
    whose email or group name already carries roles they cannot assign. The same
    subject cannot create or update a ClusterAuthorizationRule whose accessLevel
    or additionalRoles sit outside their can-assign range. A security subsystem
    manager can still assign ClusterAdmin and cannot assign SuperAdmin.
    The collision annotation does not bypass this check. GitOps ServiceAccounts
    that are not exempt cannot create a User after a SuperAdmin CAR for that email.
 - **[user-authz]** Resetting an occupied identity, deleting its ClusterAuthorizationRule, or connecting a DexProvider that can assert it now requires a can-assign range; levels are trusted only on platform-labelled roles. [#22694](https://github.com/deckhouse/deckhouse/pull/22694)
    A subject who can create UserOperations cannot reset, lock, or unlock an
    identity whose email or group membership already carries roles they cannot
    assign. Nested Group membership counts. Deleting a ClusterAuthorizationRule
    is allowed only if the requester could assign those roles. Creating or
    reconnecting a DexProvider without spec.allowedIdentities on a cluster that
    already has a SuperAdmin grant requires SuperAdmin; ClusterAdmin and the
    security subsystem manager keep rotating secrets, toggling, renaming,
    narrowing and re-applying existing providers, and connect providers whose
    allowedIdentities reach only roles they can assign. GitOps that adds an
    unlimited provider or removes a SuperAdmin rule must run as a SuperAdmin.
 - **[user-authz]** SelfScopedAccessReview answers reads of aggregated API resources opted in to scoped listings with the namespaces where they are allowed.
 - **[user-authz]** The RBACv2 role model is organised by scope, and the rights of a subject can be queried through the API. [#21846](https://github.com/deckhouse/deckhouse/pull/21846)
    Built-in role names now say what they reach: `d8:namespace:<level>`, `d8:project:<level>`,
    `d8:subsystem:<subsystem>:<level>` and `d8:system:<level>` replace `d8:use:role:<level>` and
    `d8:manage:<subsystem|all>:<level>`. Rules live in capabilities that roles aggregate, so a module
    ships access to its own resources and a cluster administrator composes custom roles under the
    reserved `d8:custom:` prefix.
    
    Every old name keeps working for ONE release: it is served as an alias aggregating the same
    capabilities, marked `rbac.deckhouse.io/deprecated`, and the alerts
    D8UserAuthzDeprecatedRBACv2RoleInUse and D8UserAuthzDeprecatedRBACv2CapabilityInUse name the
    bindings that still reference it. Move those bindings to the new names before the next release --
    `roleRef` is immutable, so a binding has to be recreated rather than edited. One alias is narrower
    than the role it replaces: `d8:use:role:admin` no longer grants `serviceaccounts/token:create` and
    `impersonate`, which moved to superadmin.
    
    Admission now enforces what the model assumes: a ClusterRoleBinding may not carry a
    namespace/project-scoped role or a capability, a custom role may not aggregate system capabilities
    into a tenant scope, and objects marked `deckhouse.io/system-resource` inside user namespaces are
    editable only by superadmin.
 - **[user-authz]** The RBACv2 subsystem roles follow the DKP subsystem list `iam`, `security`, `cluster`, `delivery`, `network`, `storage`, `observability` and `managed-services`, and the in-tree modules carry the labels of their new subsystems.
    The subsystem roles are `d8:subsystem:<subsystem>:<level>` for the eight subsystems. `deckhouse`, `infrastructure` and `kubernetes` are merged into `cluster`, `networking` is renamed to `network`, and user-authz, user-authn and multitenancy-manager move to `iam`, whose manager also assigns roles.
    The deprecated `d8:manage:deckhouse:*`, `d8:manage:infrastructure:*` and `d8:manage:kubernetes:*` roles keep the rights of the modules that were in these subsystems. Their replacement `d8:subsystem:cluster:*` grants more, so check which subsystem roles the subjects need before you recreate the bindings.
    A binding to the deprecated `d8:manage:security:<level>` role is replaced by bindings to both `d8:subsystem:iam:<level>` and `d8:subsystem:security:<level>`, because the rights for user-authn and user-authz and, at the `manager` level, the right to assign roles come with the `iam` role.
    A ClusterRoleBinding `user-authz:<rule>:additional-role:<role>` to a deprecated role is changed in `spec.additionalRoles` of the ClusterAuthorizationRule `<rule>`, and a binding that a project template renders is changed in the ProjectTemplate.
    No role of the new model collects the `rbac.deckhouse.io/aggregate-to-{deckhouse,infrastructure,kubernetes,networking}-as` labels. A custom capability or a module that still carries one reaches only the bindings to the deprecated names, and the `D8UserAuthzRetiredAggregationLabel` alert lists them.
    `d8:namespace:*` and `d8:project:*` do not collect the use capabilities of the previous scheme, `rbac.deckhouse.io/kind: use` with `rbac.deckhouse.io/aggregate-to-kubernetes-as`, nor the use roles of a module with a subsystem of its own, such as `d8:use:role:<level>:virtualization` with `rbac.deckhouse.io/aggregate-to-all-as`, and the `D8UserAuthzLegacyUseCapability` alert lists both. Until the module carries the new labels, give the subjects that need these rights a RoleBinding to the ClusterRole the alert names next to their RoleBinding to `d8:namespace:<level>`. Where the grants webhook of a project namespace refuses it, because the ClusterRole has no `rbac.deckhouse.io/delegatable` label, keep the RoleBinding on `d8:use:role:<level>`; it holds the next release until the module is updated.
    A ClusterRoleBinding to a deprecated `d8:manage:*` role keeps its `d8:use:viewer:binding:<name>` or `d8:use:user:binding:<name>` RoleBindings in the module namespaces while it refers to the old name. The deprecated `d8:use:role:<level>` and `d8:manage:all:<level>` roles keep the rights of the modules that still ship the old labels, such as log-shipper, prometheus-metrics-adapter and virtualization.
    The RoleBindings that the previous release created for a binding to a custom manage role of the old scheme are removed. Once the role is recreated as a `d8:custom:*` role of the system or subsystem scope, they come back in the namespaces of the modules that already carry the new labels. For a module that still carries the old labels, create RoleBindings in its namespace to `d8:namespace:<level>` and to the module ClusterRoles that the `D8UserAuthzLegacyUseCapability` alert names, and delete them once the module carries the new labels. A RoleBinding to `d8:use:role:<level>` there gives the same rights, but it holds the next release.
    A ClusterRole outside Deckhouse can no longer be created with `rbac.deckhouse.io/aggregate-to-all-as`, and an existing one can be updated only to remove that label.
 - **[user-authz]** The SelfScopedAccessReview resource answers in one query where within a list scope (accessible, projects, system, project) the caller may perform actions: in the whole cluster, in how many namespaces and in which ones.
 - **[user-authz]** The `antiopa` and `loghouse` namespaces are no longer system namespaces of ClusterAuthorizationRule.
    In a cluster that still has the `antiopa` or `loghouse` namespace, a ClusterAuthorizationRule without `allowAccessToSystemNamespaces` now gives its subjects access to these namespaces when it has neither `limitNamespaces` nor `namespaceSelector`, or when a `limitNamespaces` pattern matches them (for example `.*`). Check with `kubectl get ns antiopa loghouse`. These namespaces are left from components that preceded Deckhouse; delete them if nothing uses them. To keep such a namespace closed, list the namespaces of the rule in `limitNamespaces` with patterns that do not match it. Do not use `namespaceSelector` for this: a selector replaces `limitNamespaces` and `allowAccessToSystemNamespaces` and opens every namespace it matches, system namespaces included.
 - **[vertical-pod-autoscaler]** Add the recommendationFactor field to VerticalPodAutoscaler container policies to scale the CPU and memory recommendations of a workload.

## Fixes


 - **[admission-policy-engine]** A SecurityPolicy that only forbids hostNetwork denies it again. [#23003](https://github.com/deckhouse/deckhouse/pull/23003)
    A SecurityPolicy that set `allowHostNetwork` without `allowedHostPorts` stopped denying anything at all, including the `hostNetwork` it forbade. Such a policy is enforced again. Review the policies that set only `allowHostNetwork`: workloads that ran under them may start being denied.
 - **[admission-policy-engine]** A SecurityPolicyException with several allowedValues now honours all of them, not only the first. [#21556](https://github.com/deckhouse/deckhouse/pull/21556)
 - **[admission-policy-engine]** Admission policies are now evaluated for ephemeral containers added with `kubectl debug`. [#22398](https://github.com/deckhouse/deckhouse/pull/22398)
    Containers attached through the `pods/ephemeralcontainers` subresource were not seen by
    the module's webhook, so no policy was evaluated for them: SecurityPolicy, OperationPolicy,
    Pod Security Standards and image signature verification were all bypassed by `kubectl debug`.
    After the update these requests are validated.
    
    On clusters with restricted Pod Security Standards, `deny-exec` or strict SecurityPolicy,
    typical debug images (root user, writable root filesystem, interactive `-it`) will now be
    rejected. Adjust the policies or use a SecurityPolicyException for the namespaces where
    debugging must stay available.
    
    The webhook uses `failurePolicy: Fail`, so while Gatekeeper is unavailable `kubectl debug`
    will not work in user namespaces. System namespaces are excluded from the webhook and are
    not affected.
 - **[admission-policy-engine]** D8AutomountServiceAccountToken now denies a pod that mounts the service account token explicitly, even when automountServiceAccountToken is set to false. [#21556](https://github.com/deckhouse/deckhouse/pull/21556)
 - **[admission-policy-engine]** Disabling the module deletes the webhook configurations before Gatekeeper, so the uninstall no longer blocks `exec` and the Deckhouse queue.
 - **[admission-policy-engine]** Enforce the SELinux part of Pod Security Standards, and accept SecurityPolicyException entries for sysctls, AppArmor `Localhost` profiles and shared hostPath volumes.
    Pod Security Standards `baseline` and `restricted` now reject pods that set an SELinux `type` other than
    `container_t`, `container_init_t`, `container_kvm_t` or `container_engine_t`, or that set an SELinux `user` or `role`.
    Such pods were admitted before the update.
    A `SecurityPolicy` with `seLinux` entries now rejects an SELinux field that the matching entry does not list, except `level`.
    Before the update, review the workloads that set `seLinuxOptions` and cover the ones that need it with a `SecurityPolicyException`.
    A `SecurityPolicyException` for a hostPath with `readOnly: false` now also admits read-only mounts of the path.
 - **[admission-policy-engine]** Fixed CVE-2026-114437 in `github.com/go-chi/chi/v5`, CVE-2026-116585, CVE-2026-116586 and CVE-2026-116587 in `github.com/go-jose/go-jose/v4` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[admission-policy-engine]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` and CVE-2026-56851 in `golang.org/x/text`.
 - **[admission-policy-engine]** Fixed CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net` of gatekeeper.
 - **[admission-policy-engine]** Fixed CVEs in the `gatekeeper` and `ratify` images. [#23291](https://github.com/deckhouse/deckhouse/pull/23291)
 - **[admission-policy-engine]** Fixed D8AllowedSysctls ignoring a policy that sets only one of allowedUnsafeSysctls and forbiddenSysctls. [#21556](https://github.com/deckhouse/deckhouse/pull/21556)
    A SecurityPolicy that set only one of the two keys denied nothing at all.
    It now denies the sysctls it lists, so workloads that used a forbidden
    sysctl and were admitted before will be rejected. Check `forbiddenSysctls`
    in your SecurityPolicy resources before updating.
 - **[admission-policy-engine]** Fixed Gatekeeper policies for `kubectl exec` and `kubectl attach` being skipped in user namespaces.
    Exec and attach requests in user namespaces reach Gatekeeper again, as in Deckhouse 1.73 and 1.74.
    Every constraint that matches them is evaluated again, including a constraint without `match.kinds`
    or with `kinds: ["*"]` whose policy does not check `input.review.operation`.
    Before updating, check that such constraints do not deny `kubectl exec` and `kubectl attach`
    for users who need them.
 - **[admission-policy-engine]** Mutators (Assign, AssignImage, AssignMetadata, ModifySet), external data providers and connections now report their status per Gatekeeper pod.
 - **[admission-policy-engine]** Policies stay enforced during an update that adds a constraint template or while Gatekeeper reprocesses its templates.
 - **[admission-policy-engine]** Reject Assign/AssignMetadata/ModifySet/AssignImage resources that violate Gatekeeper's own content restrictions (invalid `location`, missing `match.kinds`, etc.) at `kubectl apply` time instead of silently ignoring or failing them later. [#22758](https://github.com/deckhouse/deckhouse/pull/22758)
    A pre-existing Assign/AssignMetadata/ModifySet/AssignImage resource that violates one of these restrictions (see the module FAQ) starts failing `kubectl apply`/`helm upgrade` after the upgrade. Fix the resource before upgrading, or drop it if it was already a silent no-op, most commonly a mutator missing `spec.match.kinds`, which never took effect anyway.
 - **[admission-policy-engine]** Restored the `Privileged` default PSS policy for clusters bootstrapped before v1.55 (without the `install-data` ConfigMap). [#23253](https://github.com/deckhouse/deckhouse/pull/23253)
    Clusters bootstrapped before v1.55 were switched to the `Baseline` default PSS policy in 1.77, which denies privileged workloads in user namespaces.
    After the update, the default policy on these clusters becomes less strict: it is switched back to `Privileged`, and privileged pods in user namespaces pass admission again.
    To keep the `Baseline` policy, set `settings.podSecurityStandards.defaultPolicy: Baseline` explicitly in the `admission-policy-engine` ModuleConfig.
 - **[admission-policy-engine]** Restrict SecurityPolicyException write access to ClusterAdmin, closing a tenant-to-node privilege escalation. [#22412](https://github.com/deckhouse/deckhouse/pull/22412)
    Project administrators (user-authz Admin access level) can no longer create, update or delete SecurityPolicyException resources; the permission now requires ClusterAdmin. Clusters that granted accessLevel Admin to untrusted tenants over RBAC v1 were exposed to node-level privilege escalation. RBAC v2 use/manage roles were never affected.
 - **[admission-policy-engine]** Scope policies in system namespaces by namespace name. [#23003](https://github.com/deckhouse/deckhouse/pull/23003)
    A namespace named `d8-*` or `kube-*`, and a namespace labeled `heritage: deckhouse` whatever it is called, is treated as the platform's own. Such a namespace is checked against the `restricted` Pod Security Standard: a violation is reported in the audit and in Deckhouse Console and the workload still starts, unless the module that owns the namespace has opted it into enforcement. None of this is configurable.
    
    User policies only warn there. An OperationPolicy or a SecurityPolicy with `enforcementAction: Deny` reports a violation in a system namespace and does not block the workload, whatever the namespace is labeled with, including a namespace a module has opted into enforcement.
    
    Enforcement in `kube-*` namespaces is relaxed: previously the exclusion matched `d8-*` only, so `kube-system` and the other `kube-*` namespaces were subject to user policies in full.
    
    Enforcement in system namespaces no longer depends on `defaultPolicy`. With `defaultPolicy: Restricted` the restricted constraint was previously not created, and with `defaultPolicy: Privileged` the baseline one, so a namespace that had opted into enforcement was checked against the weaker standard only.
    
    Gatekeeper mutations no longer run in `d8-*` and `kube-*` namespaces.
    
    The name prefixes `d8-system-default-` and `d8-pod-security-` are reserved: creating or updating an OperationPolicy or SecurityPolicy with such a name is rejected, as is a name longer than 235 characters, which would push a derived constraint name past the limit Kubernetes sets for an object name.
    
    Expect more audit data. Every pod in a system namespace is now measured against the full `restricted` set, and control-plane static pods violate most of it, so the constraints report violations that no constraint used to see. Each violation is one Prometheus series from `constraint-exporter` and one entry in the constraint status, up to the audit limit of 200 per constraint. None of it alerts: the audit rules fire on `deny` only.
 - **[admission-policy-engine]** The Gatekeeper webhook leaves an unreachable node after 30 seconds instead of five minutes.
 - **[admission-policy-engine]** The Trivy provider now checks images of ephemeral containers, so kubectl debug is denied for an image with disallowed vulnerabilities. [#21556](https://github.com/deckhouse/deckhouse/pull/21556)
 - **[admission-policy-engine]** fix cve [#21817](https://github.com/deckhouse/deckhouse/pull/21817)
 - **[basic-auth]** Restrict `whitelist` to IP addresses and subnets, restrict `location` to a single path segment, deny paths outside the configured locations, and serve `/healthz` on a port the `Service` does not publish.
    The settings of the `basic-auth` module are converted to version 2. The conversion deletes the following values:
    - a `whitelist` element that is not an IPv4 or IPv6 address or subnet, for example `all` or a host name;
      a location left without addresses requires a password, or denies every request when it has no `users`;
    - a `location` that is neither `/` nor a single path segment of Latin letters, digits, `.`, `_` and `-`,
      or equals `htpasswd`;
    - a repeated location, such as `/test` next to `test`.
    To keep access without a password, replace such `whitelist` values with addresses or subnets before updating.
    The `/healthz` path and, when `locations` has no `/`, every path outside the configured locations
    no longer answer `200` without authentication on port `80` of the `basic-auth` service.
    The `nginx` pods in the `kube-basic-auth` namespace restart.
 - **[candi]** A GPU problem no longer blocks node convergence. [#22142](https://github.com/deckhouse/deckhouse/pull/22142)
    On a NodeGroup with `spec.gpu`, a failing GPU check used to abort the bashible run and be retried forever, so the node never configured kubelet and never received updates from any other module. Such a node now converges and only its GPU stays unusable. If `/usr/bin/nvidia-container-runtime` is absent, containerd is now configured with `runc` as the default runtime instead of `nvidia`; on a node whose configuration previously selected `nvidia` this changes the containerd config and therefore restarts containerd. Previously such a node could not start any container at all.
 - **[candi]** A new cloud node sets up the provider network before it installs bootstrap packages, so it can download them from a master in another zone.
 - **[candi]** Bashible keeps the configured node group when the `node.deckhouse.io/group` label is missing on the node.
 - **[candi]** Bashible now disables rpcbind regardless of its runtime state and compares node uptime numerically.
 - **[candi]** Bump base images to v0.5.85, moving the Go builders to 1.25.13 and clearing Go stdlib CVEs in prebuilt images. [#22672](https://github.com/deckhouse/deckhouse/pull/22672)
 - **[candi]** Enable the DRADeviceTaints feature gate by default for Kubernetes 1.33–1.35. [#21720](https://github.com/deckhouse/deckhouse/pull/21720)
 - **[candi]** Exclude the kubelet root dir from the partition resize step, so disks of foreign persistent volumes are no longer grown. [#23138](https://github.com/deckhouse/deckhouse/pull/23138)
 - **[candi]** Explicitly set kubelet nodeStatusReportFrequency to 1m so the full Node object is no longer rewritten in etcd every 10s. [#22175](https://github.com/deckhouse/deckhouse/pull/22175)
    Kubelet config is updated and kubelet is reloaded on all nodes. Node .status fields (capacity, allocatable, images, condition heartbeat timestamps) now refresh once per minute instead of every 10s
 - **[candi]** Fix a flaky containerd-integrity check that could trigger spurious full containerd wipes and reboots. [#22597](https://github.com/deckhouse/deckhouse/pull/22597)
 - **[candi]** Fix hanging sysctl tuner on nodes with many loop and dm devices [#22750](https://github.com/deckhouse/deckhouse/pull/22750)
 - **[candi]** Fix node bootstrap hanging forever when a systemd swap unit is regenerated while being stopped (zram-generator). [#22843](https://github.com/deckhouse/deckhouse/pull/22843)
 - **[candi]** Fixed an intermittent bootstrap hang where kube-apiserver never started because kubelet read its manifest while it was being written.
 - **[candi]** Fixed moduleSource vars injection in bashible steps. [#22512](https://github.com/deckhouse/deckhouse/pull/22512)
 - **[candi]** GPU compute capability check no longer requires `bc`. [#22142](https://github.com/deckhouse/deckhouse/pull/22142)
 - **[candi]** Install kubernetes-cni 1.9.1 on nodes. [#21924](https://github.com/deckhouse/deckhouse/pull/21924)
 - **[candi]** New nodes reboot on the first bashible run again when a step requests it. Only the first master skips this reboot.
 - **[candi]** Persist `net.core.bpf_jit_harden` for GPU nodes in an own sysctl drop-in instead of editing distribution files that may be absent. [#22142](https://github.com/deckhouse/deckhouse/pull/22142)
 - **[candi]** Prevent shell command injection through proxy settings in ClusterConfiguration. [#22760](https://github.com/deckhouse/deckhouse/pull/22760)
 - **[candi]** Revert feature gate DRADeviceTaints [#21724](https://github.com/deckhouse/deckhouse/pull/21724)
 - **[candi]** Svace enabled in builder/golang from base-images
 - **[candi]** The first master bootstrap waits for `kube-apiserver` readiness instead of failing the control-plane step on `connection refused`.
 - **[candi]** The static node cleanup script now removes the `deckhouse` group, so the node can be bootstrapped again.
 - **[candi]** bashible verifies the rpp-get binary against the image digest from its bundle before running it; a substituted bootstrap proxy on a master can no longer hand a binary to the nodes.
    A node that joins while a registry-packages-proxy pod of the previous release is still running on a master waits until a pod of this release answers.
 - **[candi]** containerd v2 no longer starts without the `erofs` snapshotter, and loads the `erofs` module before start. [#22861](https://github.com/deckhouse/deckhouse/pull/22861)
 - **[cert-manager]** Fix the build of the CRD check stage of cert-manager-controller: `pm` is not available in `builder/src`.
 - **[cert-manager]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`.
 - **[cert-manager]** Fixed CVEs in the cert-manager images. [#23289](https://github.com/deckhouse/deckhouse/pull/23289)
 - **[cert-manager]** Move namespaced Issuer write from Editor to Admin. [#22494](https://github.com/deckhouse/deckhouse/pull/22494)
    Users with the Editor role can no longer create, update, or delete
    namespaced Issuer objects. Use the Admin role to manage Issuers.
    Certificate objects and ClusterIssuer permissions are unchanged.
    Existing Issuer objects keep serving until edited.
 - **[cert-manager]** Restore the `cert-manager` and `cainjector` permissions that were lost against the upstream chart, including access to `ListenerSet` resources. [#23066](https://github.com/deckhouse/deckhouse/pull/23066)
 - **[cert-manager]** The `spec.certificateOwnerRef` field of a certificate no longer causes an endless Secret update loop when it differs from the global owner reference setting that `cleanupOrphanSecrets` enables in the `CertManager` HTTPS mode, and is applied to an already issued Secret.
 - **[cert-manager]** The metrics proxy of the controller is no longer restarted by its liveness probe while its node is short of CPU.
    The `cert-manager` controller pod restarts.
 - **[cert-manager]** The mutating webhook is called only for new CertificateRequests, so most writes of the controller cost one webhook call instead of two.
 - **[cert-manager]** The webhook keeps serving requests during a rollout and during a rotation of its CA, and the controller waits for the API server at startup instead of restarting.
 - **[cert-manager]** fix cve [#21817](https://github.com/deckhouse/deckhouse/pull/21817)
 - **[chrony]** Fixed the unit-detection condition in the `disable-ntp-on-node.sh` node step. [#22777](https://github.com/deckhouse/deckhouse/pull/22777)
 - **[chrony]** revert chrony_exporter to 0.11.0 to stop the exporter crash-looping on failed probes, keeping the CVE fix as a dependency bump [#22170](https://github.com/deckhouse/deckhouse/pull/22170)
    On builds carrying
 - **[cilium-hubble]** Bump the cel-go dependency in the hubble-ui backend to fix known CVEs. [#21861](https://github.com/deckhouse/deckhouse/pull/21861)
    The cilium-hubble components (hubble-ui, hubble-relay) will restart after the update.
 - **[cilium-hubble]** Fixed CVE-2026-78661 and CVE-2026-97045 in the hubble-relay and hubble-ui images.
 - **[cilium-hubble]** Fixed a known vulnerability in the Hubble relay image.
 - **[cilium-hubble]** Fixed known vulnerabilities in the Hubble relay and UI images.
 - **[cloud-provider-aws]** Do not fail the node bootstrap or update when `linux-modules-extra` is unavailable for the running kernel. [#21917](https://github.com/deckhouse/deckhouse/pull/21917)
 - **[cloud-provider-aws]** Run the csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-aws]** fix GetCapacity not implemented error spam in logs [#22748](https://github.com/deckhouse/deckhouse/pull/22748)
 - **[cloud-provider-azure]** Fix multimaster bootstrap by falling back to the `d8-masters-kubernetes-data-device-path` Secret when the master data device LUN is not provided via cloud-init. [#23032](https://github.com/deckhouse/deckhouse/pull/23032)
 - **[cloud-provider-azure]** Run the csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-azure]** fix GetCapacity not implemented error spam in logs [#22748](https://github.com/deckhouse/deckhouse/pull/22748)
 - **[cloud-provider-dynamix]** Deleting a Service of type LoadBalancer no longer gets stuck when its Basis Dynamix balancer is already gone.
 - **[cloud-provider-dynamix]** Destroying a Basis Dynamix cluster deletes what it created instead of moving it to the recycle bin.
    The resource group, the ViNS and the master node disks are deleted permanently, and the resource
    group is deleted by force: everything still inside it, including the disks of persistent volumes,
    is deleted with it and cannot be restored. Move the data you need out of the cluster before
    destroying it. The etcd disk of a master node is detached and deleted permanently whenever that
    master is removed, including by a converge that lowers `masterNodeGroup.replicas`.
 - **[cloud-provider-dynamix]** Master and static nodes of Basis Dynamix put the external network on their first NIC.
    With both a ViNS and an external network, the guest preferred the ViNS default route and SSH to
    the external address never came up. On the first converge after this update, a master or static
    node created before it has both of its networks detached and attached again.
 - **[cloud-provider-dynamix]** The Basis Dynamix load balancer keeps its backend servers in line with the cluster nodes.
 - **[cloud-provider-gcp]** Run the csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-gcp]** fix GetCapacity not implemented error spam in logs [#22748](https://github.com/deckhouse/deckhouse/pull/22748)
 - **[cloud-provider-huaweicloud]** Recreated the etcd disk when a master VM is recreated. [#20801](https://github.com/deckhouse/deckhouse/pull/20801)
 - **[cloud-provider-huaweicloud]** Revert the etcd data disk `replace_triggered_by` from PR
    Existing huaweicloud clusters that upgraded to any 1.77.x and have `terraform-auto-converger` in Init:CrashLoopBackOff with "Node ... state does not have an OK status" will proceed with converge after this fix. No manual action required on the cluster side. Freshly bootstrapped 1.77.x clusters where `module.kubernetes_data.terraform_data.master` exists in state will have it destroyed on next converge — it is a virtual resource with no cloud-side object, so this is a state cleanup only. Because the plan contains a destructive operation, dhctl will not apply it automatically; operators of such clusters need to approve the converge explicitly.
 - **[cloud-provider-huaweicloud]** cloud-data-discoverer retries transient HuaweiCloud API failures with exponential backoff, so a single flap no longer leaves InstanceTypesCatalog empty for the full 1h reconcile interval.
 - **[cloud-provider-huaweicloud]** inline-bind EIP to compute port to avoid APIGW throttling
 - **[cloud-provider-openstack]** Add the missing with-uninitialized toleration strategy for capo-controller-manager. [#21697](https://github.com/deckhouse/deckhouse/pull/21697)
 - **[cloud-provider-openstack]** CAPI now mirrors MCM — `configDrive` honors `externalNetworkDHCP`, and additional networks always produce ports
 - **[cloud-provider-openstack]** Fixes cluster bootstrap getting stuck when `capo-controller-manager` cannot reach the API server and DNS before the CNI is ready. [#23340](https://github.com/deckhouse/deckhouse/pull/23340)
 - **[cloud-provider-openstack]** Run the csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-openstack]** accept subnet UUID via new internalSubnetID in SimpleWithInternalNetwork layout
    SimpleWithInternalNetwork base-infrastructure and master-node Terraform state will drift on the next converge for clusters created before this change. The `openstack_networking_subnet_v2` data source selector moves from an unconditional `name = ...` to a conditional `subnet_id = ... / name = ...` pair (the unused side stays `null`). The data source result is identical, but Terraform will show the selector change in the plan and refresh the data block. No instance recreation or network reconfiguration is expected — review the plan before applying `dhctl converge`
 - **[cloud-provider-openstack]** fix CloudEphemeral node creation in non-default availability zones via automatic Cinder-AZ discovery and new OpenStackInstanceClass.volumeTypeMap field
 - **[cloud-provider-vcd]** Batch VM cache refresh on cache miss [#23172](https://github.com/deckhouse/deckhouse/pull/23172)
 - **[cloud-provider-vcd]** Fixes an intermittent "network still in use" error during cluster deletion with the WithNAT placement strategy. [#22306](https://github.com/deckhouse/deckhouse/pull/22306)
 - **[cloud-provider-vcd]** Fixes cluster bootstrap getting stuck when `capcd-controller-manager` cannot reach the API server and DNS before the CNI is ready. [#23340](https://github.com/deckhouse/deckhouse/pull/23340)
 - **[cloud-provider-vcd]** Fixes the module getting stuck in Error on upgrade from 1.76 because of the immutable selector of the `capcd-controller-manager` Deployment.
 - **[cloud-provider-vcd]** Run the csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-vsphere]** Fixed master node creation when the datastore is set as a path with parent folders.
 - **[cloud-provider-vsphere]** Run the legacy csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-yandex]** A Node that cannot be resolved to a Yandex Instance no longer blocks target group synchronization for the whole cluster. [#22310](https://github.com/deckhouse/deckhouse/pull/22310)
 - **[cloud-provider-yandex]** Preemptible nodes are created as preemptible VMs and rotated before the 24h limit when a NodeGroup is managed by the CAPI engine.
    NodeGroups that already have preemptible: true under the CAPI engine keep their current regular VMs until the next rollout (any rollout-triggering InstanceClass change or manual-rollout-id); after that rollout their nodes become preemptible VMs and are rotated every ~20h.
 - **[cloud-provider-yandex]** Restored `additionalSubnets`, `networkType` and VM labels (`nodes.parameters.labels`, `additionalLabels`) for CAPI-managed Yandex nodes.
    Machines are not recreated on update. Machines created before the update keep the standard network type and no labels until the NodeGroup is rolled out (change a rolling InstanceClass field or set the `manual-rollout-id` annotation on the NodeGroup). Machines stuck in provisioning because of `additionalSubnets` become ready without recreation.
 - **[cloud-provider-yandex]** Run the csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[cloud-provider-yandex]** fix "Address in use" failures when replacing nodes and when removing external IP addresses [#22236](https://github.com/deckhouse/deckhouse/pull/22236)
 - **[cloud-provider-yandex]** fix no-cyrillic exclude rules in cloud provider yandex [#23200](https://github.com/deckhouse/deckhouse/pull/23200)
 - **[cloud-provider-yandex]** fix target group reconciliation [#21900](https://github.com/deckhouse/deckhouse/pull/21900)
 - **[cloud-provider-zvirt]** Prevent capz-controller-manager from crashlooping without diagnostics when zVirt tags cannot be created. [#22279](https://github.com/deckhouse/deckhouse/pull/22279)
 - **[cloud-provider-zvirt]** Resize the boot disk of a recreated VM, let a replaced VM actually power off, and recreate the VM when customNetworkConfig changes. [#22926](https://github.com/deckhouse/deckhouse/pull/22926)
 - **[cloud-provider-zvirt]** VMs are now actually removed from zVirt on NodeGroup scale-down and cluster destroy
 - **[cloud-provider-zvirt]** migrate legacy `IsDefault` storage domain key in discovery data to the lowercase `isDefault` before schema validation, unblocking Helm sync on clusters upgraded from pre-#13386 versions.
 - **[cni-cilium]** Bump cel-go and gopacket dependencies to fix known CVEs. [#21861](https://github.com/deckhouse/deckhouse/pull/21861)
    The cni-cilium components (cilium agent, operator) will restart after the update.
 - **[cni-cilium]** Cilium DHCP server is now sends MTU as option, so DHCP clients can apply it for their own interfaces [#21330](https://github.com/deckhouse/deckhouse/pull/21330)
 - **[cni-cilium]** Fix agent-wide deadlock in ipcache label injection and a nil-pointer crash in policy cache under identity churn. [#22221](https://github.com/deckhouse/deckhouse/pull/22221)
    Cilium agent pods are restarted to apply the fixes.
 - **[cni-cilium]** Fixed CVE-2026-78661, CVE-2026-97045, CVE-2026-116585, CVE-2026-116586, and CVE-2026-116587 in the cilium agent, cilium-operator, safe-agent-updater, and egress-gateway-agent images.
    The Cilium agent pods, cilium-operator, and egress-gateway-agent will restart after the update.
 - **[cni-cilium]** Fixed a known vulnerability in the Cilium images.
    Cilium agent and operator pods will be restarted.
 - **[cni-cilium]** Fixed file descriptor, connection and lock leaks in Cilium found by static analysis.
 - **[cni-cilium]** Fixed known vulnerabilities in the Cilium images.
    Cilium agent and operator pods will be restarted.
 - **[cni-cilium]** Restore cilium-agent startup on nodes with BPF JIT hardening enabled by tolerating ErrRestrictedKernel in the eBPF probe. [#22627](https://github.com/deckhouse/deckhouse/pull/22627)
    cilium-agent pods are restarted on all nodes.
 - **[cni-cilium]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so Cilium agents start on Deckhouse Engine nodes.
    Cilium agent pods will be restarted.
 - **[cni-flannel]** Bump Go dependencies in the flanneld image to fix known CVEs. [#21822](https://github.com/deckhouse/deckhouse/pull/21822)
    The flanneld component will restart after the update.
 - **[cni-flannel]** Fixed known vulnerabilities in the flanneld image.
    cni-flannel pods will be restarted.
 - **[cni-flannel]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so Flannel pods start on Deckhouse Engine nodes.
    Flannel pods will be restarted.
 - **[cni-simple-bridge]** The ipv4-options-guard sidecar no longer writes to the host `/etc`, so simple-bridge pods start on Deckhouse Engine nodes.
    simple-bridge pods will be restarted.
 - **[common]** A SelfScopedAccessReview result that does not allow the action no longer grants its namespaces to scoped listings.
 - **[common]** Backported kubernetes#129079 to kubelet 1.33-1.36. With `full-pcpus-only`, a kubelet restart no longer rejects running pods with `SMTAlignmentError`.
 - **[common]** Bump Go dependencies (cel-go, golang.org/x/net, golang.org/x/text, grpc) in the Kubernetes binaries to fix known CVEs. [#21833](https://github.com/deckhouse/deckhouse/pull/21833)
    The control-plane components (kube-apiserver, kube-controller-manager, kube-scheduler), kubelet, and kube-proxy will restart after the update.
 - **[common]** Bump golang.org/x/net, golang.org/x/text, golang.org/x/sys and google.golang.org/grpc in CSI sidecar patches to fix the vulnerabilities they carry. [#22596](https://github.com/deckhouse/deckhouse/pull/22596)
 - **[common]** Bumped CSI sidecars to newer upstream releases and updated `golang.org/x/net`, `golang.org/x/text` and `google.golang.org/grpc` in them to fix CVEs.
    CSI controller and node pods of all storage drivers that use the common CSI sidecar images restart.
 - **[common]** Fix CVEs in the `kube-rbac-proxy` image. [#23286](https://github.com/deckhouse/deckhouse/pull/23286)
 - **[common]** Fix UTF-8 locale initialization and Cyrillic text handling in Deckhouse and installer container terminals.
 - **[common]** Fix kubelet podSandbox order [#23209](https://github.com/deckhouse/deckhouse/pull/23209)
 - **[common]** Fixed CVE-2026-78661 and CVE-2026-97045 in the CoreDNS image.
    The CoreDNS pods of kube-dns and the node-local-dns DaemonSet pods will restart after the update.
 - **[common]** Fixed a `kube-apiserver` panic on JSON Patch requests whose `add`/`replace` of the whole document or `test` operation lacks the `value` field; such requests are now rejected with a validation error.
 - **[common]** Fixed a kubelet panic when decreasing a container memory limit in place while pod-level memory usage stats are unavailable. [#22608](https://github.com/deckhouse/deckhouse/pull/22608)
 - **[common]** Fixed known vulnerabilities in the CNI migration images.
 - **[common]** Fixed known vulnerabilities in the cni-migration-controller and cni-migration-init-checker images.
    Pods of the kube-proxy, cni-cilium, cni-flannel, and cni-simple-bridge agent DaemonSets will be restarted because their cni-migration-init-checker init container image is updated.
 - **[common]** In `hostNetwork` Pods `hostAliases` now take priority over the node `/etc/hosts`, so manual entries on the node no longer shadow them.
    kubelet will be restarted on every node. Running `hostNetwork` Pods with `hostAliases` get the new `/etc/hosts` order after their containers restart.
 - **[common]** Mask sensitive CRD fields (x-kubernetes-sensitive-data) in create/update/patch responses, not only reads. [#22198](https://github.com/deckhouse/deckhouse/pull/22198)
    x-kubernetes-sensitive-data fix will restarts the kube-apiserver (control-plane) component.
 - **[common]** Prevented system Deployment rollouts from getting stuck during the transition from HA to non-HA. [#21898](https://github.com/deckhouse/deckhouse/pull/21898)
 - **[common]** Projects a user has no access to are hidden instead of being listed to everyone. [#21672](https://github.com/deckhouse/deckhouse/pull/21672)
    Applying the change restarts control-plane components.
    Listing projects without RBAC for them now returns the accessible subset instead of 403;
    a project is visible when any of its namespaces is.
 - **[common]** Scoped listings (`d8 k get -A --scope`) get their access boundary from one SelfScopedAccessReview, refuse the project scopes without multitenancy-manager, close a watch after 60 seconds without a refresh and no longer treat an unlabelled namespace as a project.
    During the update to this version, while kube-apiserver is already updated and user-authz is not yet, scoped lists (`d8 k --scope` and the pages of the web interface that use it) can answer 403 for a short time for users without a cluster-wide list permission. Administrators are not affected. In a cluster where control-plane-manager has a `webhookURL` set by hand to the user-authz webhook while `controlPlaneConfigurator` is off, a user limited by a ClusterAuthorizationRule gets 403 instead of the filtered list. While permission-browser does not answer in time or is overloaded, a scoped request of a user without a cluster-wide permission gets 503 with `Retry-After` instead of 403; clients based on client-go, `d8 k` included, retry such an answer by themselves.
 - **[common]** Scoped listings (`d8 k get -A --scope`) of `roles` and `rolebindings` return the objects of the allowed namespaces instead of `403 Forbidden`.
    kube-apiserver image is rebuilt; control-plane components restart during the update.
 - **[common]** Static pods (control plane, `kubernetes-api-proxy`) are no longer killed by kubelet on a Kubernetes minor version change.
    During the update to this release, kubelet is reinstalled on all nodes and all static pods (`etcd`, `kube-apiserver`, `kube-controller-manager`, `kube-scheduler`, `kubernetes-api-proxy`) are restarted once without a graceful shutdown; on master nodes this causes an etcd leader election.
 - **[common]** fix cve [#21817](https://github.com/deckhouse/deckhouse/pull/21817)
 - **[common]** kube-apiserver returns a single short validation error for an invalid `fieldManager` instead of an error for every invalid character.
 - **[common]** kubelet no longer fails to start on an unusable CPU or memory manager checkpoint, and the new `D8KubeletCheckpointStateUnpinnedContainers` alert reports containers whose NUMA zone may no longer match. [#22869](https://github.com/deckhouse/deckhouse/pull/22869)
 - **[control-plane-manager]** Added the `d8_module` label to etcd, kube-apiserver, kube-controller-manager, kube-scheduler and certificate alerts, so the console shows them among control plane alerts.
 - **[control-plane-manager]** Control plane components are no longer restarted in parallel when a node is NotReady or after a certificate renewal.
 - **[control-plane-manager]** Disabled kube-scheduler profiling, which stayed enabled because the `--profiling=false` flag is ignored when the scheduler starts with `--config`.
    Kube-scheduler stops serving `/debug/pprof/*` on port 10259 and no longer calls `runtime.SetBlockProfileRate(1)`. The scheduler static pod restarts once after the update.
 - **[control-plane-manager]** Fix CVEs in the `etcd` and `control-plane-manager` images, `etcd` is updated to 3.6.15. [#23349](https://github.com/deckhouse/deckhouse/pull/23349)
 - **[control-plane-manager]** Fix dhctl bootstrap failing on CSE clusters with 'apiserver.signature Enforce' on control-plane-manager moduleConfig [#21958](https://github.com/deckhouse/deckhouse/pull/21958)
 - **[control-plane-manager]** Fix kubectl exec through Kubernetes API published with Gateway API. [#21680](https://github.com/deckhouse/deckhouse/pull/21680)
 - **[control-plane-manager]** Fix the `high-node-utilization` kube-scheduler profile, which spread pods across nodes instead of packing them onto the most loaded ones. [#22511](https://github.com/deckhouse/deckhouse/pull/22511)
    kube-scheduler will be restarted on all control-plane nodes.
    
    Pod placement changes only for pods that explicitly set `schedulerName: high-node-utilization`;
    the `default-scheduler` profile is not affected. Pods using this profile must declare resource
    `requests` — the scheduler scores nodes using `NonZeroRequested`, substituting 100m CPU / 200Mi
    memory for containers without requests, which makes bin packing operate on synthetic numbers.
 - **[control-plane-manager]** Fixed ControlPlaneOperation failures on clusters whose root CAs have a non-default subject.
 - **[control-plane-manager]** Fixed a goroutine leak in the helm release deprecation scan when listing releases fails.
 - **[control-plane-manager]** In multi-master clusters a kube-apiserver restart or crash no longer moves the kube-controller-manager and kube-scheduler leaders to another master; their leader lease is 30s.
    kube-controller-manager and kube-scheduler restart once on every master of a multi-master cluster.
 - **[control-plane-manager]** In multi-master clusters control-plane-manager restarts one kube-apiserver at a time (a minority of masters), waits for it to be ready, updates kube-controller-manager and kube-scheduler only after all kube-apiservers, and restarts their leaders last.
 - **[control-plane-manager]** In multi-master clusters kube-apiserver keeps serving for 15s after it is asked to stop, so clients move to other masters without errors during control-plane rollouts.
    kube-apiserver restarts once on every master of a multi-master cluster.
 - **[control-plane-manager]** Keep `resource.k8s.io/v1beta1` enabled in `kube-apiserver` 1.34.
 - **[control-plane-manager]** Only Deckhouse components may set `resource-quota-overrides.deckhouse.io/*` labels in Pod and PVC templates of workload controllers, so these labels no longer let users bypass ResourceQuota.
 - **[control-plane-manager]** Preserve etcd learners while new control-plane nodes are joining.
 - **[control-plane-manager]** Prevent unsupported feature gates from blocking staged Kubernetes upgrades.
 - **[control-plane-manager]** The `D8ControlPlaneComponentsResourcesInsufficientCapacity` alert now resolves once control-plane components no longer need a raise.
 - **[control-plane-manager]** The basic audit policy no longer writes kube-apiserver health probes (`/readyz`, `/livez`, `/healthz`) to the audit log. Requests to these paths that fail authentication are still logged.
    With `basicAuditPolicyEnabled` (the default), authenticated and anonymous requests to `/readyz`, `/livez`, `/healthz` and their subpaths are not audited, even when a custom policy in the `audit-policy` Secret logs all requests. To audit these requests, disable `basicAuditPolicyEnabled` and define the full policy in the Secret. The kube-apiserver pods restart to apply the new policy.
 - **[control-plane-manager]** The basic audit policy records an exec into a pod (`pods/exec`) in all namespaces again, as it did before Deckhouse 1.77.
 - **[control-plane-manager]** The publish API certificate hook no longer deletes the active TLS secret (kube-system/kubernetes-tls) on deckhouse restart. [#21889](https://github.com/deckhouse/deckhouse/pull/21889)
 - **[control-plane-manager]** The requests of `kubernetes-api-proxy` no longer go to the authorization webhook, so the proxy does not mark a working kube-apiserver as not ready and keeps its list of upstreams up to date when the webhook is unavailable.
    In clusters with the authorization webhook (`user-authz` multitenancy or `apiserver.authz.webhookURL`), kube-apiserver restarts on the master nodes to apply the new authorization configuration.
 - **[control-plane-manager]** Update Go dependencies of `control-plane-manager` and `etcd` images and record VEX statements for `GO-2026-5932`. [#22122](https://github.com/deckhouse/deckhouse/pull/22122)
 - **[control-plane-manager]** Updated Go dependencies of Kubernetes control plane components to fix known vulnerabilities.
    Control plane components and kubelet will be restarted.
 - **[control-plane-manager]** etcd backups on master nodes now start with a 15-minute offset from each other instead of running simultaneously.
 - **[control-plane-manager]** kube-apiserver no longer rejects Dex tokens without the `name` claim.
    kube-apiserver restarts on every master.
 - **[csi-vsphere]** Run the legacy csi-node DaemonSet under the dedicated `csi` ServiceAccount instead of `default`. [#22862](https://github.com/deckhouse/deckhouse/pull/22862)
 - **[deckhouse-controller]** A failed package repository scan now reports a clear reason and puts the repository into phase `Error` instead of showing a raw error under `Active`. [#23044](https://github.com/deckhouse/deckhouse/pull/23044)
 - **[deckhouse-controller]** A module enabled in the bundle is no longer disabled when its ModuleConfig does not set `enabled`.
 - **[deckhouse-controller]** A pending module release no longer loses its Kubernetes, Deckhouse and module requirements, so an update that the cluster does not satisfy stays pending instead of disabling the module.
 - **[deckhouse-controller]** Deleting an Application now also deletes objects with the `werf.io/ownership: anyone` annotation, such as Jobs deployed only on install, if the package lists their kinds in `orphanResources`.
 - **[deckhouse-controller]** Do not commit the package repository registry checksum when no application could be annotated, which left them all on stale registry settings. [#23112](https://github.com/deckhouse/deckhouse/pull/23112)
 - **[deckhouse-controller]** Fixed Deckhouse startup hanging or crash-looping when a module image download is interrupted or a partially built erofs image is left on the node.
 - **[deckhouse-controller]** Fixed a bug when PackageRepository always used a HTTPS scheme. [#22481](https://github.com/deckhouse/deckhouse/pull/22481)
 - **[deckhouse-controller]** Honor a channel-level release suspend for clusters that reach the suspended version through a step-by-step update. [#22656](https://github.com/deckhouse/deckhouse/pull/22656)
    A Deckhouse release suspended on its release channel is no longer applied by clusters that are behind and reach it through a step-by-step update. The suspend flag lives only in the release-channel image; previously it was dropped when the target release was built from its per-version image, so lagging clusters updated to a suspended release anyway.
 - **[deckhouse-controller]** Ignore auxiliary files when loading settings conversions and support versioned .yaml and .yml files.
 - **[deckhouse-controller]** Module releases rendered with nelm no longer raise false absent-resource alerts. [#21796](https://github.com/deckhouse/deckhouse/pull/21796)
 - **[deckhouse-controller]** Module sources now scan the registry every 30 minutes by default, reducing registry load
 - **[deckhouse-controller]** ModuleDocumentation will not be created for embedded modules. [#21662](https://github.com/deckhouse/deckhouse/pull/21662)
 - **[deckhouse-controller]** Prevent module hooks from running with unavailable snapshots while waiting for Deckhouse to restart after a module update
 - **[deckhouse-controller]** Stop the module source registry fan-out from replaying and flooding the main queue with duplicate moduleRun tasks. [#23112](https://github.com/deckhouse/deckhouse/pull/23112)
 - **[deckhouse-controller]** The `change-registry` helper works again when run in the deckhouse pod as documented, and when the `registry` module is in the `Unmanaged` mode; in the `Managed` mode it refuses and explains how to change the registry in the `registry` ModuleConfig.
 - **[deckhouse-controller]** nelm now takes over fields left by the old Helm 3 engine, so stale fields get removed on module upgrade. [#21796](https://github.com/deckhouse/deckhouse/pull/21796)
 - **[deckhouse]** A module whose deployed ModuleRelease is deleted is now shown as not installed instead of keeping the Ready status and the version of the removed release.
 - **[deckhouse]** Add verbosity to veritysetup format and delete damaged hash. [#22127](https://github.com/deckhouse/deckhouse/pull/22127)
 - **[deckhouse]** Allow editing and deleting the resources of an application under maintenance. [#23107](https://github.com/deckhouse/deckhouse/pull/23107)
 - **[deckhouse]** An unset `settings.update.mode` now defaults to `AutoPatch` instead of silently running as `Auto`. [#21903](https://github.com/deckhouse/deckhouse/pull/21903)
 - **[deckhouse]** Bump containerd to v2.2.10 and dependencies to fix CVEs
 - **[deckhouse]** Configure the d8-system and d8-monitoring namespaces with the privileged Pod Security enforcement level. [#22795](https://github.com/deckhouse/deckhouse/pull/22795)
    The d8-system and d8-monitoring namespaces will use the privileged Kubernetes Pod Security enforcement level. Access to create workloads in these namespaces must remain restricted.
 - **[deckhouse]** Default reservedPublicHosts.mode to List; Template no longer covers publicDomainTemplate values where %s is only a prefix of the first label. [#22532](https://github.com/deckhouse/deckhouse/pull/22532)
    Clusters that never set mode now reserve only the platform service hostnames. Set mode Template for the wider reservation. A publicDomainTemplate like %s-cluster.example.com stays on List even if Template is asked for. kube-%s.example.com stays Template when Template is set.
 - **[deckhouse]** Exec, attach, port-forward and debug into Pods labeled `heritage: deckhouse`, and scaling Deployments, StatefulSets, ReplicaSets and ReplicationControllers with that label, are now denied for everyone but system users.
    Users who run `kubectl exec`, `attach`, `cp`, `port-forward` or `debug` against Pods labeled `heritage: deckhouse`, or `kubectl scale` against Deployments, StatefulSets, ReplicaSets or ReplicationControllers labeled `heritage: deckhouse`, are now denied in every namespace, cluster administrators and superadmins included; only system users and the `system:masters` group (`super-admin.conf`) are exempt. Objects under maintenance can still be scaled. While webhook-handler is unavailable, these operations are denied for every target to everyone but the exempt users. Use `system:sudouser` where such access is needed.
 - **[deckhouse]** Fix ModuleConfig admission rejecting a module with a disabled conditional dependency [#22640](https://github.com/deckhouse/deckhouse/pull/22640)
 - **[deckhouse]** Fix RBAC permissions for deckhouse user. [#22238](https://github.com/deckhouse/deckhouse/pull/22238)
 - **[deckhouse]** Fix a minor Deckhouse release being auto-applied as a patch when no Deployed release object is present. [#21909](https://github.com/deckhouse/deckhouse/pull/21909)
 - **[deckhouse]** Fix an empty package status after a successful install [#22504](https://github.com/deckhouse/deckhouse/pull/22504)
 - **[deckhouse]** Fix random startup failures with 'waitid - no child processes' when restoring erofs modules.
 - **[deckhouse]** Fixed deletion of load balancers backed by ServiceWithHealthchecks, which failed because the child Service is protected by the `heritage - deckhouse` label in the parent cluster.
 - **[deckhouse]** Gate the package status summary on the workload health report [#22714](https://github.com/deckhouse/deckhouse/pull/22714)
 - **[deckhouse]** Module and Application status conditions now re-sync periodically, so a stale Scaled condition is repaired automatically instead of persisting until the workload changes. [#23165](https://github.com/deckhouse/deckhouse/pull/23165)
 - **[deckhouse]** Objects labeled `heritage: deckhouse` can no longer be modified by attaching changes to the pod template restart annotation or to a ModuleSource scan interval change. [#22257](https://github.com/deckhouse/deckhouse/pull/22257)
 - **[deckhouse]** Prevent NELM resource monitor reflector retries and handle resource namespaces correctly. [#22086](https://github.com/deckhouse/deckhouse/pull/22086)
 - **[deckhouse]** Restore ClusterAdmin read access to ModuleSettingsDefinition so the console can show module settings again.
 - **[deckhouse]** Restore kubectl plugin lookup in d8 k and d8 kubectl
 - **[deckhouse]** Show the Terminating phase while package repositories are being deleted
 - **[deckhouse]** The `Managed` condition of applications and modules becomes `False` with the `NoResourceReconciliation` reason once maintenance mode is applied. [#23377](https://github.com/deckhouse/deckhouse/pull/23377)
 - **[deckhouse]** The reserved public hosts policies no longer make every aggregated apiserver in the cluster watch all ConfigMaps, which left permission-browser-apiserver permanently unready and stopped namespace deletion cluster-wide. [#22444](https://github.com/deckhouse/deckhouse/pull/22444)
 - **[deckhouse]** Update d8 CLI and operator dependencies to address known vulnerabilities
 - **[deckhouse]** Update runtime libraries and VEX exceptions for known vulnerabilities
 - **[deckhouse]** bump addon-operator to run the module startup phase unconditionally [#22400](https://github.com/deckhouse/deckhouse/pull/22400)
 - **[dhctl]** Allow scaling the master node group down to one replica. [#22479](https://github.com/deckhouse/deckhouse/pull/22479)
 - **[dhctl]** An enabled cloud provider ModuleConfig without settings is no longer ignored during external provider validation.
 - **[dhctl]** Apply CloudPermanent NodeGroups and their instance classes before building the nodes. [#22164](https://github.com/deckhouse/deckhouse/pull/22164)
    In the ModuleConfig flow (no ProviderClusterConfiguration, e.g. DVP) these objects reached the cluster only in the last phase of bootstrap, so until then the CloudPermanent NodeGroups described no replicas. A bootstrap interrupted before that phase left them that way, and the next converge read every replica count as zero and deleted the nodes of those groups, the control plane included.
 - **[dhctl]** Avoid waiting for bootstrap Secret updates during regular master scale-up and refresh cloud configuration before destructive master VM replacement.
 - **[dhctl]** Bootstrap creates the CloudPermanent nodes and the additional masters for providers configured through ModuleConfig. [#21948](https://github.com/deckhouse/deckhouse/pull/21948)
 - **[dhctl]** Bootstrap of a cluster without ClusterConfiguration no longer requires podSubnetCIDR and serviceSubnetCIDR.
 - **[dhctl]** Bumped lib-connection to v0.16.1. SSH commands no longer leak file descriptors and goroutines, and stopping an SSH client no longer kills every kubectl and d8 k proxy process on the node.
 - **[dhctl]** Configure the d8-system namespace with the privileged Pod Security enforcement level during Deckhouse bootstrap. [#22795](https://github.com/deckhouse/deckhouse/pull/22795)
 - **[dhctl]** Destroy and abort of a static cluster fail instead of reporting success when a master was not cleaned up because of an ssh error or a cleanup timeout.
 - **[dhctl]** Detect conflicts with the reserved deckhouse UID/GID during preflight.
 - **[dhctl]** Fix CAPI machine drain timeout override during Deckhouse deletion
 - **[dhctl]** Fix bootstrap process to avoid race between RPP starting and cloud-permanent nodes bootstrapping.
 - **[dhctl]** Fix cri preflight checks in dhctl.
 - **[dhctl]** Fix dhctl UI/UX. [#23133](https://github.com/deckhouse/deckhouse/pull/23133)
 - **[dhctl]** Fix dhctl converge issues. [#21751](https://github.com/deckhouse/deckhouse/pull/21751)
 - **[dhctl]** Fix the static-instances-ssh-access preflight check failing with "read-only file system" when the installer runs in a pod. [#21957](https://github.com/deckhouse/deckhouse/pull/21957)
 - **[dhctl]** Fixed `dhctl destroy` hanging on CAPI Machines blocked by the node-manager pre-drain hook.
 - **[dhctl]** Fixed destructive `dhctl converge` failures by isolating per-node readiness SSH clients and stopping them before node removal. [#21328](https://github.com/deckhouse/deckhouse/pull/21328)
 - **[dhctl]** Fixed dhctl converge failing to connect through a bastion host when separate SSH keys are used.
 - **[dhctl]** Fixed the names of labels applied via "dhctl edit"
    default
 - **[dhctl]** Fixed the terraform-auto-converger init container failing on clusters already migrated to OpenTofu - the migration check now skips planning when no Terraform state is stored in the cluster.
 - **[dhctl]** In commander mode dhctl reads the cluster settings that live in ModuleConfigs from the cluster, so a prefix set only in ModuleConfig global no longer fails with Empty ClusterPrefix.
 - **[dhctl]** Increased the immutable control-plane node registration timeout to prevent bootstrap failures while nodes are still starting. [#23370](https://github.com/deckhouse/deckhouse/pull/23370)
 - **[dhctl]** Installation from a bundle no longer fails on images published as OCI image indexes.
 - **[dhctl]** Provider-specific preflight checks are skipped when the cloud provider is configured through its ModuleConfig.
 - **[dhctl]** Refactor retries in dhctl. [#21341](https://github.com/deckhouse/deckhouse/pull/21341)
 - **[dhctl]** Refresh control-plane SSH hosts from infrastructure states during master node converge.
 - **[dhctl]** Restoring of handling api_server_url and api_server_token in dhctl [#23292](https://github.com/deckhouse/deckhouse/pull/23292)
 - **[dhctl]** The bootstrap terminal no longer shows glued lines, FAILED for bashible step attempts that recovered, or "Unhandled Error" from client-go.
 - **[dhctl]** When dhctl cannot pull the provider bundle from an air-gapped cluster's own store (for example during destroy), the error says that this dhctl may be of another release than the cluster or that the store lacks the image, and how to provide the bundle, instead of pointing at registry credentials from a configuration that was not given.
 - **[dhctl]** dhctl bootstrap-phase install-deckhouse now restores releaseChannel and modules.https after installation.
 - **[dhctl]** dhctl can bootstrap a cluster from a registry served over plain HTTP [#22818](https://github.com/deckhouse/deckhouse/pull/22818)
 - **[dhctl]** dhctl config edit provider-cluster-configuration validates against the cluster's provider bundle again instead of failing with "schema not found".
 - **[dhctl]** dhctl no longer draws the live progress block or asks for confirmation when the `CI` environment variable is set; set `CI=false` to turn the check off.
 - **[dhctl]** dhctl treats `spec.enabled` of a ModuleConfig as optional and a ModuleConfig without it as disabled, which fixes terraform-auto-converger, terraform-state-exporter and `dhctl converge` on clusters whose provider ModuleConfig has no `spec.enabled`.
 - **[dhctl]** dhctl unpacks every image it downloads (candi, opentofu, terraform-manager, provider bundle) into a directory named by the image digest and never trusts files left by another build.
    A download directory shared between dhctl builds no longer serves stale or foreign candi, opentofu, plugins or provider bundles. `--download-dir` now applies to every command; `--download-cache-dir` is accepted and ignored. Do not share one download directory between this and an older dhctl release.
 - **[dhctl]** fixed DKP installation with registries that omit the Docker-Distribution-API-Version header by validating registry access through Deckhouse image resolution. [#22261](https://github.com/deckhouse/deckhouse/pull/22261)
 - **[dhctl]** trim grpc check response, increase grpc message size [#21757](https://github.com/deckhouse/deckhouse/pull/21757)
 - **[docs]** The documentation of external cloud provider modules shows the provider cluster configuration (for example, `DVPClusterConfiguration`).
 - **[documentation]** Fix a memory leak in the documentation builder so memory is released between builds; add CPU/memory limits and an opt-in profiling endpoint. [#21816](https://github.com/deckhouse/deckhouse/pull/21816)
 - **[helm_lib]** Size the resources of a module whose `resourcesManagement` names a fractional quantity, instead of rendering a zero limit.
 - **[infra-provider-baremetal]** complete RBAC 2.0 support
 - **[istio]** Add missing tools to proxyv2 images so the application-aware proxy termination hook works correctly. [#22046](https://github.com/deckhouse/deckhouse/pull/22046)
 - **[istio]** Align the cluster ID used for multicluster peers with the one used by the
    local cluster, for better support of ambient in multicluster setups. [#22884](https://github.com/deckhouse/deckhouse/pull/22884)
    Remote endpoints of multicluster peers may briefly disappear and come back
    during the upgrade.  Cross-cluster traffic served only by a remote peer can
    fail for a few seconds, so plan the upgrade accordingly if your multicluster
    setup is latency-sensitive.
 - **[istio]** Allow multicluster api-proxy to read ConfigMaps in `d8-istio`, so istiod can read the mesh config of remote clusters with Istio 1.29. [#22902](https://github.com/deckhouse/deckhouse/pull/22902)
 - **[istio]** Analysis issue fix [#21860](https://github.com/deckhouse/deckhouse/pull/21860)
 - **[istio]** Deckhouse update requirements now take into account Istio control planes that are still being removed.
 - **[istio]** Detect terminating namespaces and objects by the `metadata.deletionTimestamp` field, not by an annotation. [#23238](https://github.com/deckhouse/deckhouse/pull/23238)
 - **[istio]** Do not restart kube-apiserver when the set of installed Istio versions changes. [#22800](https://github.com/deckhouse/deckhouse/pull/22800)
 - **[istio]** Fix default Prometheus datasource in the Istio Configuration Health dashboard [#23071](https://github.com/deckhouse/deckhouse/pull/23071)
 - **[istio]** Fix east-west gateway to track mesh config changes. [#23031](https://github.com/deckhouse/deckhouse/pull/23031)
 - **[istio]** Fix graceful draining of established HTTP connections when application pods terminate. [#22068](https://github.com/deckhouse/deckhouse/pull/22068)
 - **[istio]** Fix injected sidecars and gateways not getting a seccompProfile and an istio-proxy livenessProbe in some configurations. [#23210](https://github.com/deckhouse/deckhouse/pull/23210)
 - **[istio]** Fix istio-cni SecurityPolicyException. [#23045](https://github.com/deckhouse/deckhouse/pull/23045)
 - **[istio]** Fix leaking istiod control plane after removing an Istio version or disabling the module.
 - **[istio]** Fix operator 1.25 startup with containerd v2 and RO rootfs [#21772](https://github.com/deckhouse/deckhouse/pull/21772)
 - **[istio]** Fixed CVEs in go-jose and golang.org/x/net in Istio, Kiali and alliance images; migrated from go-jose v3 to v4.
    Istio module components and ingress gateways will restart. Restart application pods to update Istio sidecars.
 - **[istio]** Fixed Zipkin tracing in Telemetry API mode, which made istiod reject the whole mesh config.
    `tracing.collector.zipkin.address` must now be in strict `<host>:<port>` format. Other values fail validation.
    In Telemetry API mode, the host must be the full service hostname, for example `zipkin.tracing.svc.cluster.local`.
 - **[istio]** Fixed `alliance-healthcheck` keeping the sidecar of the previous Istio version after a `globalVersion` change.
 - **[istio]** Fixed ambient mode on nodes with legacy iptables, where `istio-cni-node` crashed on Istio 1.25 and used nftables instead of legacy iptables on Istio 1.27 and later.
    In ambient mode with Istio 1.27 or later, on nodes with legacy iptables, `istio-cni-node` switches from nftables to legacy iptables. Pods that were added to the mesh before the update keep their nftables rules. Restart these pods after the update. Otherwise, removing such a pod from the mesh without a restart leaves its old redirect rules in place, and the pod loses network connectivity.
 - **[istio]** Fixed image pulling for waypoint pods in namespaces without sidecar-injected workloads.
 - **[istio]** Fixed istio module installation with nelm v2 when Istio is managed by the istio operator (versions below 1.27).
 - **[istio]** Inline `ca.*` CA material is now validated (single `CA:TRUE` cert, matching key, root anchoring the cert through the chain) and hard-blocks the module when invalid, instead of being copied verbatim to istiod. [#21219](https://github.com/deckhouse/deckhouse/pull/21219)
    On upgrade, an existing inline CA that is malformed, incomplete (e.g. `key` without `cert`), or whose declared `root` does not anchor `cert` will now hard-block the istio module until corrected. Configurations with valid inline CA material are unaffected.
 - **[istio]** Peer clusters keep the last known gateway addresses when a gateway temporarily has no address.
 - **[istio]** Preserve the custom TLS certificate used by Kiali when multicluster is enabled.
 - **[istio]** Resolve the desired istio revision and application namespaces the same way the sidecar-injector webhooks do. [#23238](https://github.com/deckhouse/deckhouse/pull/23238)
 - **[istio]** Restore Istiod pod anti-affinity for Istio 1.25 installations managed by the Sail Operator. [#22157](https://github.com/deckhouse/deckhouse/pull/22157)
 - **[istio]** Restrict metadata discovery server-side request forgery and pin remote cluster UUID [#22696](https://github.com/deckhouse/deckhouse/pull/22696)
 - **[istio]** The Istio network name is now consistent in every mode, not only in ambient multicluster.
    With ambient enabled, the ztunnel DaemonSet restarts. Until it has restarted on every node (and the same again on a rollback), traffic from nodes with the old ztunnel bypasses Service waypoints. AuthorizationPolicy, HTTPRoute, retries and timeouts on these waypoints are not applied during this time. Watch the ztunnel rollout until it finishes.
    In Services that mix sidecar-injected and non-injected pods, traffic distribution with `trafficDistribution: PreferClose` may change: non-injected pods now belong to the cluster network.
 - **[istio]** The Istio network name of new installations no longer depends on the cluster domain and is always a valid label value. Existing installations keep their current name.
 - **[istio]** The Istio operator now recovers Helm releases stuck in the `pending-*` or `uninstalling` state.
 - **[istio]** Users with the `User` access level can now view `WaypointInstance` resources, and users with the `Editor` access level can manage them.
 - **[istio]** Webhook `caBundle` now uses the root certificate (`ca.root`) instead of the signing certificate, matching istiod's advertised `root-cert.pem`. [#21219](https://github.com/deckhouse/deckhouse/pull/21219)
    On upgrade, clusters that use an inline **intermediate** CA (`ca.cert` is an intermediate, with a distinct `ca.root`) will have the mutating/validating webhook `caBundle` change from the intermediate to the root certificate, and istiod pods may restart if the mounted CA material differs. Self-signed and single-cert CAs are unaffected (`root` == `cert`).
 - **[kube-dns]** Fixed CVE-2026-78661 and CVE-2026-97045 in the sts-pods-hosts-appender-webhook image.
 - **[kube-dns]** Fixed a known vulnerability in the CoreDNS image.
    CoreDNS pods will be restarted.
 - **[kube-dns]** Fixed known vulnerabilities in the CoreDNS and sts-pods-hosts-appender images.
    CoreDNS pods will be restarted.
 - **[kube-dns]** Set resource limits for the `render-etc-hosts-with-cluster-domain-aliases` init container so that pods can be created in namespaces with a ResourceQuota on `limits.cpu`/`limits.memory`. [#23266](https://github.com/deckhouse/deckhouse/pull/23266)
 - **[kube-proxy]** Bump Go dependencies in the kube-proxy init-container and document non-exploitable CVEs via VEX to fix known CVEs. [#21833](https://github.com/deckhouse/deckhouse/pull/21833)
    The kube-proxy DaemonSet pods will restart after the update.
 - **[kube-proxy]** Fixed known vulnerabilities in the kube-proxy images.
    kube-proxy pods will be restarted.
 - **[kube-proxy]** Fixed known vulnerabilities in the kube-proxy init-container image.
    kube-proxy pods will be restarted.
 - **[kube-proxy]** Fixed kube-proxy failing to start on Kubernetes 1.36 and higher because of the removed `TopologyAwareHints` feature gate. [#23320](https://github.com/deckhouse/deckhouse/pull/23320)
    The `d8-kube-proxy` pods will be restarted on all nodes.
 - **[kube-proxy]** kube-proxy no longer races with node-manager when deciding which address to bind NodePort services to on a freshly created node. [#22599](https://github.com/deckhouse/deckhouse/pull/22599)
    kube-proxy now waits for node-manager to finish applying the node template first, closing the race
 - **[local-path-provisioner]** Helper pods drop all capabilities except `DAC_OVERRIDE` and `FOWNER`, and the provisioner and its helper pods are covered by SecurityPolicyExceptions to pass the restricted Pod Security policy for system namespaces.
 - **[local-path-provisioner]** `config.json` of the provisioner is serialized from a structure, and the CRD accepts in `spec.path` only an absolute path without quotes, backslashes, whitespace and control characters. A crafted path can no longer add manifests to the module release.
 - **[metallb]** Fixed BGP migration to preserve pool allocation flags and allow users to manage migrated resources after switching ModuleConfig to version 3.
 - **[metallb]** Fixed CVE-2026-78661 and CVE-2026-97045 in the metallb and l2lb images.
    The metallb components (controller, speaker, l2lb) will restart after the update.
 - **[metallb]** Fixed creation of `BGPPeer` objects on Kubernetes 1.36+, which blocked BGP sessions and LoadBalancer IP announcements in BGP mode. [#23304](https://github.com/deckhouse/deckhouse/pull/23304)
 - **[metallb]** Fixed known vulnerabilities in the MetalLB images.
    metallb pods will be restarted.
 - **[multitenancy-manager]** A PersistentVolumeClaim restored from a snapshot or cloned from a volume keeps its StorageClass instead of getting the project default; an unavailable class is rejected with an explanation.
 - **[multitenancy-manager]** A Project can no longer set `spec.administrators` together with `parameters.administrators`, or `spec.quota` together with `parameters.resourceQuota`. An update is checked only for the standard field values it changes, and a memory or storage quota of 0 needs no unit.
 - **[multitenancy-manager]** A ProjectNamespace whose namespace was created by someone else stays not ready instead of taking that namespace into the project.
 - **[multitenancy-manager]** A ProjectRoleBinding gives no rights to ServiceAccounts outside the namespaces of its project and reports them in the SubjectsBound condition, while the administrators in spec.administrators of a Project get the rights whatever namespace their ServiceAccounts are in; a new project or ProjectNamespace cannot take a name while a ClusterProjectRoleBinding, or a ProjectRoleBinding of another project that the name extends with a dash, still names ServiceAccounts of a namespace with that name.
 - **[multitenancy-manager]** A ProjectTemplate with a podSecurityStandard or networkPolicy.mode literal outside the accepted values is refused by the admission webhook instead of rendering into nothing. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
 - **[multitenancy-manager]** A namespace adopted into a project keeps its Pod Security Standard; without the `security.deckhouse.io/pod-policy` label it stays under the admission-policy-engine default policy, and an existing label keeps its value.
 - **[multitenancy-manager]** A namespace that becomes a project keeps its own labels and annotations, and the project release no longer reverts or restores them.
    A namespace adopted into a project no longer has its own labels and annotations copied into the Project parameters.
    The project release neither reverts a direct change to such a label (for example `istio.io/rev`, `team`, Argo CD or Flux tracking labels) nor restores a removed one.
    The additional namespaces of the project do not inherit these labels; declare one in `spec.parameters.namespace.labels` of the Project, or set it on an additional namespace directly, where it is needed.
    The node-selector and tolerations annotations of an adopted namespace are still copied into `spec.parameters.namespace.annotations` and are changed there.
    `extended-monitoring.deckhouse.io/enabled` and `security-scanning.deckhouse.io/enabled` keep the value `"true"` the namespace had instead of being rewritten to `""`, and any other value, `"false"` included, becomes `""`.
    A ClusterResourceGrantPolicy that selects namespaces by such a label with `namespaceSelector` no longer covers the new additional namespaces of an adopted project; declare the label in `spec.parameters.namespace.labels` of the Project or select the project with `projectSelector`.
 - **[multitenancy-manager]** A namespace with an empty projects.deckhouse.io/project label is no longer counted towards the project of the same name, and the empty value can no longer be set by hand.
    Grant policies stop selecting such a namespace by the labels of the same-name Project, and its AvailableClusterResource catalog is removed. The namespace is still not adopted into a project. Setting the empty value on a namespace by hand is refused, including re-creating a namespace from a manifest that has it.
 - **[multitenancy-manager]** A project parameter can no longer inject objects into the rendered project manifests. [#21756](https://github.com/deckhouse/deckhouse/pull/21756)
    Values taken from `Project.spec.parameters` are now quoted where they are substituted into the
    shipped project templates, and a parameter that changes the structure of the rendered manifests
    rather than only their values is refused for any template, including custom ones. An administrator
    name may no longer contain control characters, and `clusterLogDestinationName` must be a Kubernetes
    object name or empty. Both are checked from the reconcile loop as well as at admission, so a
    project already carrying such a value goes into an error state on its next reconcile rather than
    at its next edit; the same holds for a custom template that turns a parameter into several objects
    or into YAML, which is refused when the project is created, edited, or reconciled with a change to
    apply. The module's admission policy now also matches CREATE, so objects labelled
    `heritage: multitenancy-manager` can only be created by the module itself.
 - **[multitenancy-manager]** A project parameter can no longer inject objects into the rendered project manifests. [#21846](https://github.com/deckhouse/deckhouse/pull/21846)
    Values taken from `Project.spec.parameters` are quoted where they are substituted into the shipped
    project templates, and a parameter that changes the structure of the rendered manifests rather than
    only their values is refused for any template, including custom ones. An administrator name may no
    longer contain control characters, and `clusterLogDestinationName` must be a Kubernetes object name
    or empty. Both are checked from the reconcile loop as well as at admission, so a project already
    carrying such a value goes into an error state on its next reconcile rather than at its next edit;
    the same holds for a custom template that turns a parameter into several objects or into YAML,
    which is refused when the project is created, edited, or reconciled with a change to apply. The
    module's admission policy now also matches CREATE, so objects labelled
    `heritage: multitenancy-manager` can only be created by the module itself.
 - **[multitenancy-manager]** A project whose release switches from a Helm `resourcesTemplate` to structured fields keeps the access of its administrators and its quota: the AuthorizationRule and ResourceQuota objects that the standard fields replace are deleted only once `d8-administrators` and `d8-project-quota` work, and the other objects of the Helm text are left in place.
    After the update, every project on the built-in templates `default`, `secure` and `secure-with-dedicated-nodes` switches from their former Helm texts to structured fields. Its administrators move without a gap from AuthorizationRule objects with the `Admin` access level (`user-authz:admin` plus `user-authz:admin:custom`) to the `d8:project:admin` role of the `d8-administrators` ProjectRoleBinding, and its `all-pods` quota is replaced by `d8-project-quota`. The two roles do not grant the same permissions. In particular, custom ClusterRoles with the `user-authz.deckhouse.io/access-level` annotation no longer add to the access of these administrators; create a custom role and bind it with a ProjectRoleBinding instead.
    
    - While the old objects are kept, the Project has the `HandoverPending` condition. If they are still there after 30 minutes, the `MultitenancyManagerProjectHandoverStuck` alert fires.
    - On a later switch of a user template, every other object of the Helm text that the structured fields do not render (rules and bindings of other subjects, other quotas, network policies, workloads) is left in place instead of being deleted. It keeps working, is no longer managed by the project, carries the `projects.deckhouse.io/kept-from-helm-template` label, under which only a requester who may make the same change across the cluster may change or delete it, and is listed in `status.keptObjects`, and the `MultitenancyManagerProjectKeepsHelmTemplateObjects` alert fires until it is deleted, unlabelled or rendered by the template again.
    - Administrators and quotas that stay in `.spec.parameters` because they could not be moved to `.spec.administrators` and `.spec.quota` are not handed over, and a switch leaves their objects in place.
 - **[multitenancy-manager]** A rollout restart of a workload that the project release renders passes the heritage admission policy only when the restartedAt annotation of the pod template is the only change.
    Before, an update of such a Deployment, StatefulSet or DaemonSet that set a new kubectl.kubernetes.io/restartedAt annotation skipped the d8-multitenancy-manager admission policy with every other change it carried. Now the policy refuses it unless that annotation is the only change, so a tool that restarts a workload and changes it in one update gets a refusal; change such a workload through the ProjectTemplate and restart it with kubectl rollout restart.
 - **[multitenancy-manager]** A user project with the projects.deckhouse.io/virtual-project label is no longer treated as virtual, and the label can no longer be added to one.
    A user Project that already carries projects.deckhouse.io/virtual-project=true becomes an ordinary project: ClusterProjectRoleBinding extends to it, and ProjectRoleBinding and ProjectNamespace can be created in it. Creating a Project with the label is refused, including re-creating one from a backup or a GitOps manifest that still has it: remove the label from the manifest. An existing Project keeps the label with a warning.
 - **[multitenancy-manager]** ClusterResourceGrantPolicy projectSelector matches the labels of the Project object as well as the namespace labels; a policy written against a Project label now covers the main and the additional namespaces of the project. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
    The selector is evaluated against the union of Project and namespace labels, the namespace winning a shared key. Existing policies keyed on namespace labels (`projects.deckhouse.io/project`, `projects.deckhouse.io/project-namespace`, `projects.deckhouse.io/project-template`) behave as before.
    The CRDs now refuse `allowed: []`, an empty `allowedSelector`/`deniedSelector` and `apiGroups: ["*"]` in a GrantableClusterResourceReference; such objects were accepted before and either opened the resource or intercepted nothing. Existing objects are validated on their next write.
    `defaultFrom` counts only an annotation with the value `true`; a StorageClass carrying `storageclass.kubernetes.io/is-default-class: "false"` is no longer treated as the default.
    Policies carry a new `SelectorsValid` condition.
 - **[multitenancy-manager]** Deleting a Project now blocks until its namespace is actually gone, as deleting the namespace used to; the finalizer is kept while the namespace exists. [#23124](https://github.com/deckhouse/deckhouse/pull/23124)
 - **[multitenancy-manager]** Deleting a project requires the right to delete each of its namespaces.
    A user who may delete projects but not namespaces can no longer delete a project whose namespaces exist, and the refusal names the namespace. This affects holders of `d8:subsystem:deckhouse:manager` without `d8:subsystem:kubernetes:manager` or `d8:subsystem:infrastructure:manager`. Deleting a project deletes its namespaces with all workloads in them, so grant `delete` on `namespaces` to the users who remove projects. Only the delete of a Project is checked. A holder of the cluster-wide `d8:system-capability:multitenancy-manager:edit` can still delete the additional namespaces of a project through their ProjectNamespace objects.
 - **[multitenancy-manager]** Disabling the module now removes the cluster-objects grant webhooks, which used to stay behind and block writes in project namespaces.
    The grant webhooks are now removed together with the module. On a cluster where `multitenancy-manager` is already disabled at the update, the `cluster-objects-grants-defaulting` and `cluster-objects-grants-validator` webhook configurations left by earlier versions stay, and writes to grantable resources in project namespaces (for example, removing a PVC finalizer) keep failing with `service "multitenancy-manager" not found`. Delete them by hand:
    `d8 k delete mutatingwebhookconfiguration cluster-objects-grants-defaulting`
    `d8 k delete validatingwebhookconfiguration cluster-objects-grants-validator`
    Nothing has to be done on clusters where the module is enabled.
 - **[multitenancy-manager]** During a rollout, the controller waits up to 10 minutes for the pods of the previous revision to stop before it reconciles, so that two controller versions do not write the Helm releases of projects at the same time. After 10 minutes the controllers start anyway, and the new `MultitenancyManagerPreviousRevisionStillRunning` alert fires while pods of two revisions keep running.
 - **[multitenancy-manager]** Fixed CVE-2026-116585, CVE-2026-116586 and CVE-2026-116587 in `github.com/go-jose/go-jose/v4`, CVE-2026-63209 in `github.com/klauspost/compress` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[multitenancy-manager]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`.
 - **[multitenancy-manager]** Fixed a vulnerability in the `multitenancy-manager` image.
 - **[multitenancy-manager]** Fixed vulnerabilities in the `multitenancy-manager` image. [#23285](https://github.com/deckhouse/deckhouse/pull/23285)
 - **[multitenancy-manager]** Labels and annotations that the module does not own can be changed directly on a project namespace again; the project ownership label can be set only by the controller; the per-project catalog is protected fail-closed. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
    On an adopted (or any project) namespace, `kubectl label`/`annotate` and GitOps re-applies are accepted for keys the module does not own; `heritage`, `projects.deckhouse.io/*`, the template-driven policy labels, the Helm ownership annotations, the node-selector and tolerations annotations, `spec`, finalizers and deletion are still refused and are changed through the Project and its template.
    Creating a namespace with a `projects.deckhouse.io/project` label, or changing that label, is refused for everyone but the controller — cluster administrators included; create a Project or a ProjectNamespace instead.
    `AvailableClusterResource` objects carry `heritage: multitenancy-manager` (relabelled in place) and are refused to everyone but the controller by the admission policy; a project with nothing available keeps an empty catalog object instead of none.
 - **[multitenancy-manager]** Project names reserve only the namespaces that ProjectNamespaces claim; a project named like an additional namespace of another project is allowed with a warning and reported by an event and an alert.
 - **[multitenancy-manager]** ProjectTemplate v1alpha1 is served again as a deprecated version, and a template with a Helm `resourcesTemplate` keeps rendering the objects it rendered before the update.
    `ProjectTemplate` v1alpha1 is served with a deprecation warning that points at v1alpha2, and v1alpha2 carries `resourcesTemplate`. A template with a non-empty `resourcesTemplate` is rendered through Helm as before, from the project name, the template name and the project parameters. The form is deprecated; describe new templates with the structured fields.
    
    - A Helm template whose `parametersSchema` declares `administrators` or `resourceQuota` receives them from `spec.administrators` and `spec.quota`, in the layout of `Project` v1alpha2. Quota values arrive in canonical spelling (`1024Mi` as `1Gi`) unless the schema needs another one, and a zero arrives as the string "0".
    - A `Project` parameter `administrators` or `resourceQuota` that the template `parametersSchema` does not declare, as on the built-in templates, grants no access and sets no quota. `Project` admission refuses a new or changed value of such a parameter, except an empty list and a quota with no value other than the number 0, which pass with a warning. A value that an update keeps also passes with a warning, so a project that the conversion left them in keeps working. Set `spec.administrators` and `spec.quota` instead. In a v1alpha2 quota, leave out a limit of the number 0 to keep it unset, since the string "0" is a hard limit of zero.
    - The ResourceQuota and AuthorizationRule objects a Helm template renders are kept, next to `d8-project-quota` and `d8-administrators`. A pod has to fit every quota of its namespace.
    - The additional namespaces of a project on a Helm template are created and inherit the labels the template puts on the main namespace. Every Helm project release is upgraded once after the update.
    - `ProjectTemplate` admission refuses a non-empty `resourcesTemplate` together with fields that render objects. It refuses emptying the `resourcesTemplate` of a template that projects use, unless the same change sets such fields. It also refuses a write through v1alpha1 that would erase fields only v1alpha2 has.
    - A new administrator name cannot contain control characters or line breaks. A name that is already set is kept, with a warning.
 - **[multitenancy-manager]** Projects on custom templates whose `parametersSchema` still declares `administrators` and `resourceQuota` pass validation again, and a memory or storage quota value without a unit is accepted with a warning that it is read as bytes when an update keeps it or a create through `deckhouse.io/v1alpha2` writes it.
 - **[multitenancy-manager]** Quote digit-only Project administrator names in built-in templates. [#21998](https://github.com/deckhouse/deckhouse/pull/21998)
    Prevents invalid YAML when an administrator name consists only of digits (for example 008).
 - **[multitenancy-manager]** Refresh grant webhook CA bundles after the admission certificate is rotated. [#22793](https://github.com/deckhouse/deckhouse/pull/22793)
 - **[multitenancy-manager]** The ClusterResourceGrantPolicyViolation metric is computed by the controller with the same union-of-policies rule as the admission webhook, so an object allowed by one of several policies no longer raises a phantom alert. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
    The `d8_cluster_objects_grant_violated` series is now scraped from the multitenancy-manager controller (port 9091, PodMonitor `multitenancy-manager`) instead of the module hook; the metric name, labels, alert and dashboard are unchanged.
 - **[multitenancy-manager]** The Project and ProjectTemplate admission webhooks no longer exempt `system:masters`; cluster components and `system:sudouser` bypass them, as they do the heritage ValidatingAdmissionPolicy. [#23124](https://github.com/deckhouse/deckhouse/pull/23124)
    Requests from `system:masters` to Project and ProjectTemplate are now validated like everyone else's (ProjectRoleBinding, ProjectNamespace and ClusterProjectRoleBinding always were). To bypass a Project or ProjectTemplate check deliberately, impersonate `system:sudouser` (`kubectl --as system:sudouser`).
 - **[multitenancy-manager]** The Project conversion to `v1alpha3` lifts `parameters.administrators` and `parameters.resourceQuota` into the standard fields only in shapes it can give back, and leaves any other shape in the parameters instead of failing the conversion, granting `d8:project:admin` to an entry with an `accessLevel` or dropping quota keys. A quota with the number `0`, which the 1.77 templates read as no limit, stays in the parameters too instead of becoming a hard limit of 0.
 - **[multitenancy-manager]** The controller's metrics endpoint is served through kube-rbac-proxy instead of plaintext. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
    The d8_cluster_objects_grant_violated series name projects and the objects inside them; the endpoint now requires authentication and the get verb on deployments/prometheus-metrics of the multitenancy-manager deployment. Prometheus is allowed by the module itself; anything else scraping the port directly must be granted that permission.
 - **[multitenancy-manager]** The main namespace of a project deleted past the admission policy is recreated, unless an owner of that namespace was deleted, and a ClusterProjectRoleBinding or ProjectRoleBinding reports its status even when a project namespace refuses the binding.
    The controller recreates the main namespace of a Project once it is deleted directly past the
    admission policy, for example as `system:sudouser`, together with the objects of its template and of
    `manifests`. The other content of the namespace does not come back. A Project adopted from a namespace
    that has ownerReferences now carries the same owners, so the garbage collector deletes the Project
    together with the namespace and the namespace is not recreated. Only the controller and the
    identities the policy lets through may change the ownerReferences of a project namespace. An update
    that adds or changes an ownerReference of a Project or a ProjectNamespace is refused to everyone the
    multitenancy-manager webhooks validate, except the controller and Deckhouse.
    On the update, every Project that has lost its main namespace, or whose main namespace is stuck in
    Terminating, gets it back. Before the update, list these Projects and delete the ones you do not need.
    The list may also show Projects that are not restored: Projects in the Error state because their
    template is missing, and leftover Projects of the older model, which the controller deletes.
    d8 k get projects.deckhouse.io,namespaces -o json | jq -r '(INDEX(.items[] | select(.kind == "Namespace"); .metadata.name)) as $ns | .items[] | select(.kind == "Project" and .metadata.labels["projects.deckhouse.io/virtual-project"] != "true" and .spec.projectTemplateName != "virtual" and .metadata.deletionTimestamp == null) | .metadata.name | select($ns[.] == null or $ns[.].metadata.deletionTimestamp != null)'
 - **[multitenancy-manager]** The module becomes Ready again after it is disabled and enabled; the controller waits for the CRD conversion webhook itself instead of holding the Helm release.
 - **[multitenancy-manager]** The module manager role no longer grants authoring of project templates. [#22307](https://github.com/deckhouse/deckhouse/pull/22307)
    `d8:manage:permission:module:multitenancy-manager:edit` no longer grants write access to
    `projecttemplates`. A `ProjectTemplate` renders into a release applied by a ServiceAccount bound
    to `cluster-admin`, and its `spec.resourcesTemplate` is arbitrary, so authoring one is equivalent
    to holding `cluster-admin` and is not a module-level permission.
    
    Reading templates stays in `d8:manage:permission:module:multitenancy-manager:view`, and creating a
    `Project` on an existing template stays in the edit role. The legacy `ClusterAdmin` access level
    and the `kubeadm:cluster-admins` group keep the right. A subject that authored templates through
    the manager role has to be granted one of those instead.
 - **[multitenancy-manager]** With `allowNamespacesWithoutProjects` set to `false`, namespaces are still created only as a part of a project. A ValidatingAdmissionPolicy now enforces it instead of the module webhook, the parameter is no longer deprecated, and the `MultitenancyManagerDeprecatedConfigParameter` alert is removed.
 - **[multitenancy-manager]** fix concurrent map writes panic in webhook when copying ProjectTemplate from cache [#22181](https://github.com/deckhouse/deckhouse/pull/22181)
 - **[multitenancy-manager]** fix cve [#21817](https://github.com/deckhouse/deckhouse/pull/21817)
 - **[network-gateway]** Fix dnsmasq (DHCP) and snat containers crash-loop. [#22132](https://github.com/deckhouse/deckhouse/pull/22132)
 - **[network-gateway]** Fixed user and group in ConfigMap [#22295](https://github.com/deckhouse/deckhouse/pull/22295)
    network-gateway pod's will be restarted
 - **[network-policy-engine]** Bump Go dependencies in the kube-router image to fix known CVEs. [#21829](https://github.com/deckhouse/deckhouse/pull/21829)
    The kube-router component will restart after the update.
 - **[network-policy-engine]** Fixed known vulnerabilities in the kube-router image.
    network-policy-engine pods will be restarted.
 - **[node-local-dns]** Bump Go dependencies in the coredns helper image to fix known CVEs. [#21851](https://github.com/deckhouse/deckhouse/pull/21851)
    The node-local-dns DaemonSet pods will restart after the update.
 - **[node-local-dns]** Fixed CVE-2026-78661 and CVE-2026-97045 in the coredns helper, safe-updater, and stale-dns-connections-cleaner images.
    The node-local-dns DaemonSet pods and the node-local-dns components will restart after the update.
 - **[node-local-dns]** Fixed DNS errors during node-local-dns pod restarts in clusters with Cilium.
 - **[node-local-dns]** Fixed a known vulnerability in the node-local-dns CoreDNS image.
    node-local-dns pods will be restarted.
 - **[node-local-dns]** Fixed known vulnerabilities in the node-local-dns images.
    node-local-dns pods will be restarted.
 - **[node-manager]** A StaticInstance whose StaticMachine was deleted is returned to Pending and becomes available for bootstrapping again instead of staying out of the pool forever.
 - **[node-manager]** A change of `ClusterConfiguration.proxy` on a running cluster now reaches the containerd and kubelet proxy settings on the nodes.
 - **[node-manager]** A drain that fails or times out is no longer recorded as `drained`.
    A node whose drain keeps failing now keeps the `draining` annotation, carries
    `update.node.deckhouse.io/drain-failed` with the error, and is retried. Its NodeGroup
    stops updating until the drain succeeds, instead of updating a node that still runs its
    pods. `NodeStuckInDraining` fires five minutes after the first failure and now covers
    drains requested by update approval, by a cloud spot termination and by hand. The
    `d8_node_draining` metric lost its `message` label, the error is read from the node.
 - **[node-manager]** A new kubernetes-api-proxy image now takes effect on containerd nodes when Deckhouse is updated, without a node reboot.
 - **[node-manager]** A node-manager release no longer waits for every capi-controller-manager replica on a bootstrapped cluster; new alerts report missing replicas.
 - **[node-manager]** An interrupted bootstrap of a static node is completed on the next attempt instead of being refused. [#22944](https://github.com/deckhouse/deckhouse/pull/22944)
 - **[node-manager]** Annotate CAPI MachineDeployments with CPU/memory capacity for scale-from-zero [#21820](https://github.com/deckhouse/deckhouse/pull/21820)
 - **[node-manager]** Approve only kubelet-serving CSRs in the kubelet_csr_approver hook. [#22901](https://github.com/deckhouse/deckhouse/pull/22901)
 - **[node-manager]** Fix CVEs in fencing-agent and cluster-autoscaler images (CVE-2026-56852, GHSA-hrxh-6v49-42gf, CVE-2026-46600). [#22377](https://github.com/deckhouse/deckhouse/pull/22377)
 - **[node-manager]** Fix NodeCapacity calculation. [#22675](https://github.com/deckhouse/deckhouse/pull/22675)
 - **[node-manager]** Fix create_rbac_and_certificate_for_kubernetes_api_proxy hook behavior. [#21622](https://github.com/deckhouse/deckhouse/pull/21622)
 - **[node-manager]** Fix d8-shutdown-inhibitor failing to start on nodes where another package overrides the logind inhibit delay. [#22513](https://github.com/deckhouse/deckhouse/pull/22513)
 - **[node-manager]** Fixed cluster-autoscaler not scaling CAPI node groups up from zero for pods requesting ephemeral storage, and putting the whole scale-up of a multi-zone node group into a single zone.
 - **[node-manager]** Fixed cluster-autoscaler stopping autoscaling of all node groups and restarting every two hours when an MCM node group has `minPerZone` equal to `maxPerZone`.
 - **[node-manager]** Fixed cluster-autoscaler using node group names without the cluster prefix after a Deckhouse restart, which stopped autoscaling and triggered the `D8ClusterAutoscalerTooManyErrors` alert.
 - **[node-manager]** Fixed the CAPI MachineHealthCheck replacing cloud nodes that take longer than 5 minutes to become Ready after registering, and limited remediation to when at most half of the CAPI machines are unhealthy.
    The cloud MachineHealthCheck is updated in existing clusters. A node in Ready=False is replaced after 10 minutes instead of 5 (Ready=Unknown still after 5), and remediation stops while more than 50% of the cluster's CAPI machines are unhealthy.
 - **[node-manager]** Hide MCM and Cluster API resources from the node-manager ClusterAdmin role. [#22060](https://github.com/deckhouse/deckhouse/pull/22060)
    Users can no longer read or modify MachineDeployments, MachineSets, Machines, machine classes and CAPI infrastructure objects directly; these APIs are controller-owned. Manage nodes through NodeGroup and StaticInstance.
 - **[node-manager]** Hide the CAPS SSH key and sudo password in SSHCredentials from users without the `sshcredentials/sensitive` subresource. [#22572](https://github.com/deckhouse/deckhouse/pull/22572)
    The SSH key and sudo password in `SSHCredentials` (used by CAPS to reach static nodes) are now
    returned as `<omitted>` to callers without `get` on the `sshcredentials/sensitive` subresource,
    masked in the audit log, and encrypted in etcd when `apiserver.encryptionEnabled` is on.
    `d8:manage:infrastructure:viewer` and `:manager` no longer see these values. CAPS still does, and
    so does every holder of a `deckhouse.io` wildcard, since `*` matches subresources: `SuperAdmin`,
    the `kubeadm:cluster-admins` group, and the `deckhouse` and `webhook-handler` SAs of `d8-system`.
    
    For masked readers the `last-applied-configuration` annotation disappears from `get -o yaml`,
    and creating an `SSHCredentials` with `<omitted>` fails with `422 Invalid` — editing still works.
    Treat previously exposed keys as leaked and rotate them. With `apiserver.encryptionEnabled` on,
    rewrite existing objects to encrypt them: `d8 k get sshcredentials -o json | d8 k replace -f -`.
 - **[node-manager]** Hooks `discover_standby_ng` and `set_ng_priorities` run in the module queue, so one NodeGroup change no longer queues a second module run.
 - **[node-manager]** Increase the capi-controller-manager VPA maximum memory recommendation to prevent health-check failures and restarts under memory pressure. [#23184](https://github.com/deckhouse/deckhouse/pull/23184)
 - **[node-manager]** Master controllers leave an unreachable master after about 100 seconds, fit on the masters left and are not evicted from a single master.
 - **[node-manager]** Master controllers no longer tolerate keys from `customTolerationKeys`, so a key that repeats a built-in one no longer blocks pod updates.
    node-controller, capi-controller-manager, machine-controller-manager and bashible-apiserver do not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - **[node-manager]** NodeUser password hashes are now hidden from unauthorized API readers.
 - **[node-manager]** Nodes trust the registry CA from a ModuleSource that has `dockerCfg` set again.
    bashible-apiserver no longer skips such ModuleSources, so their CA is added to the node configuration. Bashible will be rerun on all nodes.
 - **[node-manager]** On a master node shutdown or reboot kubelet gives kube-apiserver, etcd and other node-critical pods 25s instead of 5s to stop.
 - **[node-manager]** Read InstanceClass through the API version published by the cloud provider instead of resolving it from discovery. [#21768](https://github.com/deckhouse/deckhouse/pull/21768)
 - **[node-manager]** Restricted bashible-apiserver to TLS 1.2 or later and modern cipher suites.
 - **[node-manager]** Revert defaultCRI migration to the node-manager ModuleConfig (#21482); ClusterConfiguration.defaultCRI is the source of truth again, fixing bootstrap hanging on "Waiting for Registry to become Ready" when defaultCRI is omitted. [#21830](https://github.com/deckhouse/deckhouse/pull/21830)
 - **[node-manager]** Sped up installing Deckhouse Engine from the ISO - the installer no longer writes zeros over the whole system disk before partitioning it.
 - **[node-manager]** Stop caps-controller-manager from rewriting StaticInstance objects in a hot loop when a connectivity check fails, which caused a sustained load on etcd. [#23104](https://github.com/deckhouse/deckhouse/pull/23104)
    A StaticInstance whose connectivity check fails now keeps its reservation for the whole bootstrap window instead of returning to the pool on every failed attempt, and the bootstrap (20 min) and cleanup (10 min) timeouts, which previously never fired, are now reachable.
    If nothing ever ran on the host, the bootstrap timeout returns the instance to the pool after 20 minutes as before.
    If the host was already bootstrapped in part, the instance returns only through MachineHealthCheck remediation (nodeStartupTimeoutSeconds: 1200) plus the cleanup timeout, so it becomes available again roughly 20 + 10 minutes after the failure, and remediation reboots the host as part of the cleanup.
 - **[node-manager]** The `NodeGroup` CRD now rejects `spec.kubelet.containerLogMaxFiles` values below 2, which kubelet refuses to start with.
 - **[node-manager]** The discover_standby_ng hook patches NodeGroup status only when the standby value changes and ignores changes of NodeGroups without standby.
 - **[node-manager]** Validate NodeGroup InstanceClass kinds dynamically using installed cloud-provider registrations. [#22018](https://github.com/deckhouse/deckhouse/pull/22018)
 - **[node-manager]** `kubernetes-api-proxy` closes a stuck probe connection after the probe timeout, sends the client-go logs to its JSON log, and serves its health endpoints on `127.0.0.1:6480` by default, as in its static pod manifest.
 - **[node-manager]** `kubernetes-api-proxy` keeps serving `127.0.0.1:6445` for 3s after `SIGTERM`, so that kubelet and node-critical pods can write their last state on a node shutdown.
 - **[node-manager]** kubernetes-api-proxy keeps slow kube-apiservers in rotation, stops sending connections to a kube-apiserver that is shutting down, restarting or hung, and closes client connections to a hung kube-apiserver so that clients reconnect to a live one.
    The kubernetes-api-proxy container restarts on every node; for about a second connections to kube-apiserver through it on that node are refused and clients reconnect.
 - **[node-manager]** node-controller no longer removes taints set by other components on the first reconcile of Static and CloudPermanent nodes. [#22958](https://github.com/deckhouse/deckhouse/pull/22958)
 - **[node-manager]** node-controller no longer treats an unreadable source as an absent one, which could publish a NodeGroup without its instanceClass and re-run bashible on every node [#22103](https://github.com/deckhouse/deckhouse/pull/22103)
 - **[node-manager]** refuse adding staticInstances.labelSelector to a NodeGroup when it would leave already-bound StaticInstances outside the new selector
 - **[node-manager]** the validating webhook now enforces that kubelet.memorySwap.swapBehavior LimitedSwap requires cgroup v2, a check that never fired because the field was misspelled in the Go type [#22103](https://github.com/deckhouse/deckhouse/pull/22103)
    A NodeGroup created while the check was dead — `swapBehavior: LimitedSwap` with nodes that do not
    support cgroup v2 — is now rejected on every update, including the finalizer removal that a
    delete needs, so such a NodeGroup can hang in Terminating. Removing `swapBehavior: LimitedSwap`
    from its spec passes and unblocks it.
 - **[openvpn]** Bump Go dependencies in the ovpn-admin and easyrsa-migrator images to fix known CVEs. [#21826](https://github.com/deckhouse/deckhouse/pull/21826)
    The openvpn components will restart after the update.
 - **[openvpn]** Fixed known vulnerabilities in the ovpn-admin and easyrsa-migrator images.
 - **[registry-packages-proxy]** Allow node drains in non-HA mode and add a module-level high-availability override.
 - **[registry-packages-proxy]** Fixed registry-packages-proxy hanging after a cache error and serving corrupted packages, which blocked node updates.
 - **[registry-packages-proxy]** The bootstrap port serves the manifest and the layer of the rpp-get image, so a node can verify the binary it installs.
 - **[registry-packages-proxy]** The proxy serves `deckhouse-cli` and `deckhouse-cli/plugins/<name>` images from the registry root above the cluster's edition repository. [#22324](https://github.com/deckhouse/deckhouse/pull/22324)
 - **[registry-packages-proxy]** deckhouse-cli and plugin downloads now work in registries filled by `d8 mirror push`; the proxy probes the cluster repo first, then the edition-trimmed root, and remembers the working one. [#22535](https://github.com/deckhouse/deckhouse/pull/22535)
 - **[registry-packages-proxy]** registry-packages-proxy leaves an unreachable master after about 100 seconds, runs a quorum of masters with HA on and is not evicted from a single master.
 - **[registry-packages-proxy]** registry-packages-proxy no longer tolerates keys from `customTolerationKeys`.
    registry-packages-proxy does not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - **[registry]** A RegistryOperation no longer stays active for 24 hours when a storage replica is gone mid-operation. A replica that stops holding the collection lease while it collects, or whose node is not Ready for ten minutes while it waits, fails with the reason `ReplicaLost`.
 - **[registry]** Add SecurityPolicyException for registry v2
    Registry storage will restart once ofter update
 - **[registry]** Fix the order in which the Unmanaged params are stored for the fallback to the Legacy mode. [#22924](https://github.com/deckhouse/deckhouse/pull/22924)
 - **[registry]** Garbage collection of the registry cache now actually frees disk space. Its deletions were refused, its sweep failed on repositories filled through the cache, and blobs left by a failed sweep were never removed. It no longer removes the platform manifests of multi-arch images.
 - **[registry]** In `Managed` mode, the credentials of a `RegistryUpstream` whose `match` has a port are stored under a valid Secret key.
 - **[registry]** Registry credentials in RegistryConfig and RegistryUpstream are now hidden from unauthorized API readers.
 - **[registry]** Require brackets around IPv6 upstream registry URL hosts and accept bracketed addresses with or without a port.
 - **[registry]** The cluster goes air-gap only after no platform workload, including the registry storage itself, still pulls from the upstream registry.
 - **[registry]** The in-cluster cache elects a new storage leader when the leader's master is lost or its registry or disk stops working, instead of waiting for the Node or pod to be deleted.
 - **[registry]** The in-cluster registry cache accepts its accounts only under `system/deckhouse`, the publication account can no longer delete, and registry-controller and registry-packages-proxy no longer have access to the cache's write account.
    Pulling through the cache under a repository outside `system/deckhouse` is refused. `d8 mirror push` must target `<publication endpoint>/system/deckhouse`, as documented.
    registry-controller and registry-packages-proxy read the new Secret `d8-system/registry-storage-client` instead of `registry-storage-access`.
 - **[registry]** The in-cluster registry cache can now be updated in an air-gapped cluster with a single master, and a blocked cache update is explained and alerted on.
    A new DaemonSet `d8-system/registry-storage-image-holder` runs on every master while the cache is on (4 containers running `/pause`, 10m CPU / 10Mi memory each).
    On a single-master air-gapped cluster the cache replica is replaced once its new images are on the master. While it restarts, for a few tens of seconds, the cluster cannot pull images.
    The new alert `D8RegistryStorageUpdateBlocked` fires when the cache has not been updated for an hour, with the reason.
 - **[registry]** The in-cluster registry storage gets liveness and startup probes and a VPA in `Initial` mode, its disruption budget no longer lets the only replica be evicted while the master count is unknown, and the registry-controller VPA covers kube-rbac-proxy.
 - **[registry]** The registry agent no longer sends the node's cache credentials for repositories outside `system/deckhouse` requested by a cache replica address.
    Pulls of images pushed into the in-cluster cache outside `system/deckhouse` as `<master IP>:5001/<repository>` are no longer authorized with the node's credentials and are refused.
 - **[registry]** The registry agent no longer waits for an unreachable cache leader on every pull. Recently failed registries are tried last, connections time out faster, and dead HTTP/2 connections are detected by pings.
 - **[registry]** The registry cache endpoint on nodes no longer moves to the first replica while the storage leader changes; it stays put until the new leader takes over.
 - **[registry]** The registry storage on master nodes now serves on ports 4281/TCP (pulls by node agents and replication) and 4283/TCP (publishing through the `registry-push` Ingress) instead of 5001 and 5003.
    The in-cluster registry storage (`registry` module, `Managed` mode with `storage.cache`) listens on the node address of master nodes on 4281/TCP instead of 5001/TCP and on 4283/TCP instead of 5003/TCP. Both are in the 4200–4299 range that the documented host firewall rules already open. If inter-node traffic is filtered by explicit rules, allow 4281/TCP from all nodes to master nodes and between master nodes, and 4283/TCP from the nodes running the Ingress controller to master nodes. The registry agent stays on 127.0.0.1:5001 on every node, and image references (`registry.d8-system.svc:5001`) do not change. An installation from a bundle uses 4281/TCP on the first master for its bootstrap registry, and the bootstrap preflight now checks that ports 4281 and 4283 are free.
 - **[registry]** The registry storage update no longer replaces a replica while nothing can serve its image, and no longer counts a terminating replica as serving.
 - **[registry]** The registry-agent static pod requests CPU and memory, and on nodes configured by bashible it now measures data the in-cluster cache left behind, so `D8RegistryStaleCacheData` can fire there.
    The registry-agent restarts once on every node. After moving from the v1 registry implementation (DP up to and including 1.77) to the registry module without the in-cluster cache, `D8RegistryStaleCacheData` may fire on masters for the data left in `/opt/deckhouse/registry/local_data`.
 - **[registry]** The store's reserve now follows the live size of its filesystem and is smaller on a dedicated disk. The replica status reports an unmounted store disk, and a store the registry cannot write to is reported as `NotWritable` and alerted on.
 - **[registry]** Tune registry agent for downloading a huge images. [#23257](https://github.com/deckhouse/deckhouse/pull/23257)
 - **[registry]** registry-agent now reaches upstream registries through the proxy set in `ClusterConfiguration.proxy`.
    When a proxy is configured, the registry-agent static pod restarts once on every node, and again whenever the proxy settings change.
 - **[registry]** registry-controller and registry-storage no longer tolerate keys from `customTolerationKeys`, so a key that repeats a built-in one no longer blocks pod updates.
    registry-controller and registry-storage do not run on a master tainted with a key from `global.modules.placement.customTolerationKeys`. Remove such taints from the master NodeGroup before the update.
 - **[registry]** registry-controller leaves an unreachable master after about 100 seconds, registry-storage stays on it, and neither is evicted from a single master.
 - **[terraform-manager]** Fix RBAC. [#21751](https://github.com/deckhouse/deckhouse/pull/21751)
 - **[terraform-manager]** Fixed the `to-tofu-migrator` init container of `terraform-auto-converger` failing to resolve the provider bundle digest, which kept auto-converge from starting.
 - **[user-authn]** A DexAuthenticator refreshes a session once at a time under load instead of refreshing the same token again for every request of the session.
    Dex and the pods of every DexAuthenticator restart.
 - **[user-authn]** A DexAuthenticator whose namespace is terminating no longer stalls the Deckhouse queue; the `D8DexAuthenticatorNamespaceUnusable` alert points at it. [#23131](https://github.com/deckhouse/deckhouse/pull/23131)
    Such an authenticator is left out of the module values, so the user-authn release stops failing on
    objects that the API server cannot create, and the main queue no longer stops behind the retrying
    module. The authenticator is exported as `d8_user_authn_dex_authenticator_skipped` and the
    `D8DexAuthenticatorNamespaceUnusable` alert fires after 10 minutes with the steps to finish the
    namespace deletion. The authenticator is deleted together with its namespace; to keep it, recreate
    it in a live namespace.
 - **[user-authn]** A DexAuthenticator with a name longer than 45 characters no longer logs its users out on every run of the hook.
 - **[user-authn]** A DexAuthenticator with one replica, including one with `highAvailability: false` in a cluster with high availability, keeps serving while it rolls out.
 - **[user-authn]** A DexClient in a namespace whose name YAML reads as a number or a boolean no longer breaks the user-authn release.
 - **[user-authn]** A Logout of a session or grant the user does not have succeeds and says so, instead of failing.
 - **[user-authn]** A User with an email longer than 150 bytes, which could never sign in, is refused, and an existing one gets an `EmailTooLong` event instead of an endless reconcile error.
    The User CRD refuses an email longer than 150 bytes.
 - **[user-authn]** A `ResetPassword` UserOperation keeps the replaced password in the history of `passwordPolicy`, so the user cannot set it again at the forced change after the reset.
 - **[user-authn]** A browser sign-in no longer lifts a lock set by an administrator.
 - **[user-authn]** A failed CSRF check on the DexAuthenticator callback no longer writes the request cookies, including the refresh token, to the log (GHSA-hhqp-vx7f-5c6m).
 - **[user-authn]** A new DexAuthenticator whose name is not a DNS-1035 label of at most 63 characters (for example, starts with a digit or has a dot) is refused instead of breaking the user-authn release.
    A new DexAuthenticator can no longer have a dot in its name, including a long name with a dot that worked before. An existing DexAuthenticator with such a name keeps working and can be updated.
 - **[user-authn]** A short Dex outage (no connection, a timeout, `429` or `5xx`) no longer signs users out of applications behind DexAuthenticator.
    While Dex is unavailable, a request that needs a session refresh gets an error page instead of a redirect to sign in; a reload works once Dex is back.
 - **[user-authn]** A user in no group can sign in through a SAML provider, an LDAP search without groups is a debug line, and the UserIdentity of a deleted DexProvider is deleted 30 days after the orphaned-since mark unless the account is locked or a revocation of the pair is still kept.
 - **[user-authn]** A user locked by an administrator sees that the account is locked by an administrator instead of the "Too Many Requests" page of an automatic lockout.
    Dex answers a sign-in of such a user with `403` and the "Account locked by an administrator" page, and the token endpoint answers `access_denied` with `403`. An automatic lockout after failed password attempts keeps its `429` answers.
 - **[user-authn]** Added settings version 3 of the user-authn ModuleConfig; an `idTokenTTL` of 6h or more in a ModuleConfig of version 1 or 2 is converted to `5h59m`, and version 3 refuses it. [#23268](https://github.com/deckhouse/deckhouse/pull/23268)
    On a cluster whose user-authn ModuleConfig sets `idTokenTTL` to 6h or more, Dex issues ID tokens for 5h59m after the update, which also lowers the minimum user session lifetime (`keepUsersLoggedInFor` of a DexAuthenticator, `auth.sessionTTL` of the console) to that value. The `D8UserAuthnIDTokenTTLTooLong` alert goes from severity 6 to 4 and keeps firing until the ModuleConfig is updated to settings version 3 with an `idTokenTTL` below 6h.
 - **[user-authn]** Adding the allow-access-to-kubernetes annotation to a DexClient or a DexAuthenticator now requires cluster-level authority over the user-authn module. [#22219](https://github.com/deckhouse/deckhouse/pull/22219)
    Only a subject allowed to update the user-authn ModuleConfig, or the Deckhouse service account, may add the `dexclient.deckhouse.io/allow-access-to-kubernetes` or `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotation. Objects that already carry it keep working and stay editable by their owners as long as they are updated in place. A GitOps controller that deletes and recreates such an object instead of patching it will have the recreation denied, and the object will come back without access to the Kubernetes API unless the controller's service account is allowed to update the user-authn ModuleConfig.
 - **[user-authn]** Administrator lock now blocks login, password grant, and refresh without passwordPolicy.lockout. [#22483](https://github.com/deckhouse/deckhouse/pull/22483)
 - **[user-authn]** Allow DexAuthenticator applications using Gateway API to omit the Ingress class. [#23226](https://github.com/deckhouse/deckhouse/pull/23226)
    Removing the Ingress class from an existing application configured with Gateway API removes its
    generated Ingress resources.
    
    An existing DexAuthenticator that specifies both an Ingress class and gatewayAPI now also gets an
    HTTPRoute in clusters where no Gateway is configured or discovered for the module. Previously such
    a route was not created at all. If the old behaviour was worked around with a hand-written route
    on /dex-authenticator, both now exist and Gateway API grants the match to the older one.
    
    A ListenerSet set for an application under additionalApplications before this release was dropped
    on write, because the field was missing from the v1 schema, which is the storage version. Adding
    the field does not restore the lost values: re-apply those DexAuthenticator manifests. The
    re-applied manifests are now also checked for domains already taken on the same ListenerSet, and
    for a domain repeated between additional applications.
 - **[user-authn]** Applications behind one DexAuthenticator can be signed in to in several tabs at the same time; every sign-in keeps its own CSRF cookie.
 - **[user-authn]** Cap idTokenTTL below 6h and make User.spec.password immutable. [#21998](https://github.com/deckhouse/deckhouse/pull/21998)
    Existing ModuleConfigs with idTokenTTL >= 6h keep working but raise D8UserAuthnIDTokenTTLTooLong until the value is lowered below 6h.
    Patching User.spec.password after create is rejected; reset passwords via UserOperation or `d8 iam user reset-password`.
 - **[user-authn]** Commander releases older than 1.18 can grant Kubernetes API access on DexClient again. [#22518](https://github.com/deckhouse/deckhouse/pull/22518)
 - **[user-authn]** Deleting a User, manually or by `ttl`, also deletes its 2FA (TOTP) secret, so a user created again with the same name does not inherit it.
 - **[user-authn]** Dex answers a refresh-token request that the identity provider refuses with `invalid_grant` instead of a server error.
    When the provider reports that the user no longer exists, has lost access or has left the groups the DexProvider requires, kubeconfig clients and applications behind DexAuthenticator ask the user to sign in again instead of failing with `500`. A sign-in refused for the provider groups shows a `403` page instead of `500`.
 - **[user-authn]** Dex deletes expired refresh tokens on the kubernetes storage instead of logging "list refresh tokens: not implemented" every hour.
 - **[user-authn]** Dex does not start when an authenticator of the second factor cannot be built, instead of letting every login through without it.
 - **[user-authn]** Dex gets an API priority level of its own, so a burst of sign-ins or refreshes is no longer throttled together with every other service account.
 - **[user-authn]** Dex is no longer restarted when it answers its probes slowly because its node is short of CPU, so logins recover as soon as the node does.
    The `dex` pods restart.
 - **[user-authn]** Dex no longer restarts in a loop or hangs at startup when the issuer of an OIDC provider is unreachable or does not answer; it skips that provider after 10 seconds and starts with the other providers and the local users, if there are any.
    Dex logs "opening connector" with the connector ID before it opens each identity provider at startup. If no provider can be opened and there are no local users, Dex exits with "failed to open all connectors" instead of hanging.
 - **[user-authn]** Dex reads a user from an external provider again only when the stored data is older than `sessions.identityRefresh.maxAge` (`idTokenTTL` by default), with sessions on or off, so a user disabled at the provider is noticed within that time.
    Dex pods restart. Without sessions, a user disabled at the external provider keeps refreshing tokens until the stored data is older than `sessions.identityRefresh.maxAge` (10 minutes by default) instead of being refused at the next refresh.
 - **[user-authn]** Dex refuses the refresh of a locked user with `invalid_grant` (HTTP 400) instead of `access_denied` with HTTP 429 or 403, so that applications send the user to sign in, and a Dex that signs in through another Deckhouse Dex takes the old answer for a refusal too.
    A client of Dex that expects HTTP 429 or 403 on the refresh of a locked user gets HTTP 400 `invalid_grant` with the same description. The password and token exchange grants and the sign-in pages answer as before.
 - **[user-authn]** Dex shows "The sign-in has expired" instead of "Requested resource does not exist" when a sign-in step comes more than 10 minutes after the sign-in started.
    Dex restarts.
 - **[user-authn]** DexAuthenticator containers always have cpu and memory requests, so an authenticator runs in projects whose policies require requests (e.g. the default project template) and no longer fails the user-authn release there.
    DexAuthenticator pods restart once where their requests change: with VPA enabled and no spec.resources, or for redis when spec.resources has no redis section.
 - **[user-authn]** DexAuthenticator now gets its own NetworkPolicy, so sign-in works in projects with isolated network policies and an ingress controller in hostNetwork can reach the authenticator.
    Authenticator pods now accept connections only on 8443 (plus the Istio ports 15008 and 15020 when the istio module is enabled) and may reach only dex, the revocation feed, istiod and DNS; a sidecar injected into them that needs other ports or destinations loses them.
 - **[user-authn]** DexAuthenticator pods are labeled `heritage` and are protected from exec, attach, port-forward and edits by namespace users.
    Every DexAuthenticator restarts once. Users of the namespace of a DexAuthenticator can no longer exec, attach or port-forward into its pods or delete them; pod logs stay available.
 - **[user-authn]** DexAuthenticator shows the "The sign-in has expired" page with a "Sign in again" button, or with the Dex sessions on starts the sign-in again, instead of the 403 "Unable to find a valid CSRF token" page when the sign-in took longer than 10 minutes.
 - **[user-authn]** DexClient and DexAuthenticator allow-lists are now enforced on password grant and token refresh. [#22484](https://github.com/deckhouse/deckhouse/pull/22484)
    Changing allowedGroups or allowedEmails deletes refresh tokens that no longer match. Those users must sign in again.
 - **[user-authn]** DexClient and DexAuthenticator now honour the value of the allow-access-to-kubernetes annotation instead of merely its presence. [#21897](https://github.com/deckhouse/deckhouse/pull/21897)
    The `dexclient.deckhouse.io/allow-access-to-kubernetes` and
    `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotations were previously
    evaluated by key presence, so any value — including "false" — granted the client the
    right to obtain tokens with `aud=kubernetes` and placed it into `trustedPeers` of the
    `kubernetes` OAuth2Client, whose tokens kube-apiserver accepts.
    
    The value is now parsed as a boolean. Only 1, t, T, TRUE, true and True grant the
    capability. Any other value, including "false", "0", "yes", "enabled" and an empty
    string, disables it; unparseable values are rejected with a warning in the
    deckhouse Pod log naming the object and the offending value.
    
    Affected: clusters with a DexClient or DexAuthenticator that carries the annotation
    with a value other than a boolean true. Those objects lose access to the Kubernetes
    API on update, which for most of them is the behaviour their author intended.
    
    Before updating, run
    `kubectl get dexclient,dexauthenticator -A -o json | jq -r '.items[] | select(.metadata.annotations | to_entries[]? | select(.key | endswith("deckhouse.io/allow-access-to-kubernetes")) | .value | ascii_downcase | IN("1","t","true") | not) | "\(.kind)/\(.metadata.namespace)/\(.metadata.name)"'`
    and, for every object listed, either set the annotation to "true" if it genuinely needs
    Kubernetes API access, or remove the annotation to confirm it does not.
 - **[user-authn]** Drop the dead per-DexAuthenticator redirect URI from the privileged kubernetes OAuth2 client. [#22216](https://github.com/deckhouse/deckhouse/pull/22216)
 - **[user-authn]** Fix Dex token refresh with upstream providers that rotate refresh tokens (GitLab), which logged users out every `idTokenTTL`. [#21615](https://github.com/deckhouse/deckhouse/pull/21615)
 - **[user-authn]** Fixed CVE-2026-116585, CVE-2026-116586 and CVE-2026-116587 in `github.com/go-jose/go-jose/v4` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[user-authn]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`.
 - **[user-authn]** Fixed CVEs in `golang.org/x/net` and `go-jose` of dex-authenticator.
 - **[user-authn]** Fixed CVEs in the module images. [#23290](https://github.com/deckhouse/deckhouse/pull/23290)
 - **[user-authn]** Fixed vulnerabilities in Dex (golang.org/x/net v0.60.0): CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669, CVE-2026-97032.
 - **[user-authn]** In the authorization code flow, Dex accepts only the S256 PKCE method.
    Dex rejects an authorization request with a code_challenge and the plain method, or with no method. A DexClient application that sends one has to switch to S256. Applications that send no PKCE and DexAuthenticator are not affected. The device flow is not affected.
 - **[user-authn]** Issue a dedicated OAuth2 client secret per DexAuthenticator instead of reusing the shared cluster client secret. [#22216](https://github.com/deckhouse/deckhouse/pull/22216)
    On upgrade, every DexAuthenticator that carries the
    `dexauthenticator.deckhouse.io/allow-access-to-kubernetes` annotation gets a freshly generated client
    secret, because it previously received the client secret of the privileged `kubernetes` OAuth2 client
    instead. Its pods roll once and users of those applications may have to sign in again. Each rotation is
    written to the Deckhouse log as `Rotating DexAuthenticator client secret that reused the shared kubernetes
    client secret`, with the authenticator name and namespace.
    
    If that annotation was ever used outside the `d8-*` and `kube-*` namespaces, the shared secret must be
    considered disclosed to anyone who could read Secrets in those namespaces, and has to be rotated by hand.
    Deleting the Secret alone does not rotate it: while the value is non-empty in memory the owning hook
    re-renders the same one. Rotate it as follows.
    
    1. If a GitOps tool syncs the `d8-user-authn` namespace, exclude
       `Secret/kubernetes-dex-client-app-secret` from that sync first, otherwise it is restored to the old
       value.
    2. Blank the stored value:
       `kubectl -n d8-user-authn patch secret kubernetes-dex-client-app-secret --type merge -p '{"data":{"secret":""}}'`
    3. Drop the in-memory copy: `kubectl -n d8-system rollout restart deployment/deckhouse`
    4. Check that `kubectl -n d8-user-authn get secret kubernetes-dex-client-app-secret -o jsonpath='{.data.secret}'`
       now differs from the previous value. If it does not, the module re-rendered the old value before the
       restart took effect; repeat steps 2 and 3.
    
    The blast radius of that rotation is wider than the `kubernetes` client alone. The same value is the client
    secret of the `kubeconfig-generator`, `kubeconfig-publish-api` and every `kubeconfig-<slug>` OAuth2 client,
    and it is passed to basic-auth-proxy as `--ldap-client-secret`. In-cluster consumers are re-rendered and
    their pods roll automatically, but kubeconfig files already downloaded from the kubeconfig generator carry
    the old client secret and can no longer refresh their tokens, so their users must download a new
    kubeconfig. Already issued ID tokens keep working until they expire, at most `settings.idTokenTTL`.
    
    Deleting and recreating a DexAuthenticator is always a safe way to force a fresh per-authenticator secret:
    the credentials Secret is regenerated with the resource. Its content is owned by Deckhouse, so no GitOps
    change is needed for the per-authenticator rotation itself.
 - **[user-authn]** Parallel Basic Authentication requests of one user through an LDAP provider no longer fail with `500`.
 - **[user-authn]** Protected applications keep serving users during a short dex outage and during restarts.
 - **[user-authn]** Quote DexProvider-derived values rendered into the basic-auth-proxy Deployment and the Dex configuration file to prevent YAML injection. [#22214](https://github.com/deckhouse/deckhouse/pull/22214)
 - **[user-authn]** Reject whitespace in DexProvider OIDC scopes, which the OAuth2 protocol cannot represent. [#22214](https://github.com/deckhouse/deckhouse/pull/22214)
    `spec.oidc.scopes` items are now validated against `^\S+$` in both served versions of DexProvider. A scope containing whitespace could never work, because OAuth2 passes scopes as a space-delimited list, but it was previously admitted and silently broke authentication. Stored objects are not invalidated, however a DexProvider holding such a value is rejected on its next write until the value is corrected, which also affects a GitOps flow that reapplies the object unchanged.
 - **[user-authn]** Remove list access to Dex password and offline-session storage from the user-authn read-only and cluster-admin roles. [#22215](https://github.com/deckhouse/deckhouse/pull/22215)
    Subjects holding only the user-authn view role or the legacy ClusterAdmin user-authn role can no longer list `passwords.dex.coreos.com` or `offlinesessionses.dex.coreos.com`. Use `User` objects for user inventory instead.
 - **[user-authn]** Render a valid Deployment for a DexAuthenticator whose `resources` limits are fractional or below the init container requests.
    The `self-signed-generator` init container now carries the fixed `10m` / `25Mi` it was always
    meant to have, the same as the identical init container of `user-api`, instead of the sum of the
    main container limits. A DexAuthenticator that set large limits no longer gets that sum as burst:
    generating the self-signed certificate then takes 3 to 4 seconds instead of under a second. It
    happens once per pod start and needs no action.
 - **[user-authn]** Replicas of a DexAuthenticator with `spec.highAvailability` different from the cluster high availability mode are spread over different nodes.
    Pods of DexAuthenticators whose `spec.highAvailability` differs from the cluster high availability mode are restarted.
 - **[user-authn]** Restrict the Job that issues the `basic-auth-proxy` certificate to the read-only front-proxy CA files and cover it with a SecurityPolicyException.
 - **[user-authn]** The API server rejects a new or changed DexAuthenticator with an invalid `keepUsersLoggedInFor` value, so such a value no longer fails the release of the whole `user-authn` module.
    A DexAuthenticator saved earlier with an invalid value still fails the module release until the value is fixed.
 - **[user-authn]** The CSE copies of the DexProvider CRD and of the values schema follow the module again.
 - **[user-authn]** The Dex password change page opens only inside the sign-in that led to it, in the same browser, and asks only for the new password.
    After a forced password change, the sign-in continues to the application without a second sign-in. A link to the password change page outside a sign-in no longer works, and the page no longer redirects to an address taken from the request. The page states why the password must be changed: it was reset, it does not meet the password policy, or it has expired.
 - **[user-authn]** The Dex password change page refuses an empty new password with any password policy, including `complexityLevel: None` and no `passwordPolicy`.
 - **[user-authn]** The Group webhook refuses only a membership cycle that the change itself creates; other Group changes are accepted with a warning that names an existing cycle, and removing a member of a cycle is allowed.
 - **[user-authn]** The `D8UserAuthnIDTokenTTLTooLong` alert keeps severity 6, because the settings conversion already replaces an `idTokenTTL` of 6h or more with `5h59m` and only the ModuleConfig is left to update.
 - **[user-authn]** The `Lock`, `ResetPassword` and `Reset2FA` operations of a local user no longer end the sessions of a user of another provider with the same name.
 - **[user-authn]** The `rateLimit` parameter also limits failed attempts on the two-factor authentication (2FA) steps and on the password change page.
 - **[user-authn]** The credentials Secret and the OAuth2Client of a DexClient, and trustedPeers of the kubernetes OAuth2Client, are updated by the controllers of the module right after a DexClient or DexAuthenticator changes.
    The Secret keeps its client secret on upgrade. It gets an ownerReference to the DexClient, and secretMetadata can no longer override the labels app, name, heritage and module. A DexClient whose secretMetadata overrode app or name no longer gets a new client secret on every change of any DexClient. The kubernetes OAuth2Client is also created when only a DexClient is allowed access to Kubernetes. A new Deployment dexclient-controller in d8-user-authn writes the credentials of DexClients; it is the only component of the module with rights on Secrets in other namespaces.
 - **[user-authn]** The kubeconfig clients can no longer exchange tokens of an upstream identity provider for Dex tokens.
 - **[user-authn]** The legacy publish API certificate hook no longer deletes TLS secrets in d8-user-authn on deckhouse restart. [#21889](https://github.com/deckhouse/deckhouse/pull/21889)
 - **[user-authn]** The module settings refuse a zero `idTokenTTL`; with it DexAuthenticator never checked sessions with Dex.
    A ModuleConfig with a zero `idTokenTTL` (for example, `0s`) fails validation after the update. Set a positive value.
 - **[user-authn]** The policy gating the allow-access-to-kubernetes annotation no longer applies to platform components, and proves authority with a create permission on the dexclients/allow-access-to-kubernetes and dexauthenticators/allow-access-to-kubernetes subresources instead of write access to the user-authn ModuleConfig. [#22388](https://github.com/deckhouse/deckhouse/pull/22388)
    Setting the annotation no longer requires write access to the user-authn ModuleConfig. Module administrators are unaffected, as their roles already carry the new permission, and so are the platform's own components in `d8-system` and `kube-system`. Any other subject that sets the annotation, a cluster provisioner in particular, has to be granted `create` on the subresource, which for a single namespace is:
    
        apiVersion: rbac.authorization.k8s.io/v1
        kind: Role
        metadata:
          name: allow-access-to-kubernetes
          namespace: <namespace>
        rules:
        - apiGroups: ["deckhouse.io"]
          resources: ["dexclients/allow-access-to-kubernetes", "dexauthenticators/allow-access-to-kubernetes"]
          verbs: ["create"]
    
    bound to the subject with a RoleBinding in the same namespace, or with a ClusterRole and a ClusterRoleBinding to grant it cluster-wide.
 - **[user-authn]** The user-authn controller no longer reads every Password at each pass of a user, so its start and every sign-in no longer slow down with the number of users.
 - **[user-authn]** Two DexAuthenticators whose name and namespace differ only in the position of a dash no longer break the release of the module.
    Users of applications behind a DexAuthenticator have to sign in again once after the update; the refresh tokens issued under the old client ids are deleted, and UserAccount no longer lists them. The dex-authenticator Deployments restart. The aud claim of the ID tokens of a DexAuthenticator becomes dex-authenticator-NAME@NAMESPACE; an application that checks aud has to accept the new value.
 - **[user-authn]** Users are sent to sign in, instead of an HTTP 500 page on every reload, when the identity provider cannot refresh their session.
    A session whose refresh Dex cannot complete now ends: at once when the identity provider refuses the refresh (for example, an OIDC provider that issued no refresh token), and one `idTokenTTL` after the first failure when the provider or Dex does not answer. Until then requests get 503, as before.
 - **[user-authn]** Users of an LDAP DexProvider for Active Directory with `idAttr: objectGUID` (or `mS-DS-ConsistencyGuid`) can sign in; their Dex user ID becomes the GUID string.
    A user whose GUID happened to be valid UTF-8 and who could sign in before gets a new `sub` (an application that tells users apart by `sub` sees a new user) and enrolls TOTP again; for every other such user sign-in did not work before.
 - **[user-authn]** With `rateLimit` enabled, Dex counts the client address only from the platform Ingress controller and Gateway, and refresh-token requests no longer count as failed attempts.
    A pod that calls the Dex Service directly is counted by its own address and can no longer forge the address in the rate limit and in the sign-in log, unless it can read the HTTPRoute object of the module in `d8-user-authn` or create an Ingress with configuration snippets on the ingress-nginx controller. The Dex Ingresses make ingress-nginx present its client certificate to Dex; Dex asks every client for a certificate and requires none.
 - **[user-authn]** With `rateLimit` enabled, failed Basic Authentication attempts through an LDAP provider are counted per client instead of in one bucket for basic-auth-proxy.
 - **[user-authn]** With two-factor authentication (2FA) enabled, Dex asks for the code before the password change page, so a password alone is not enough to change it.
    A user whose password must change enters the 2FA code first and then sets the new password.
 - **[user-authn]** basic-auth-proxy no longer forwards reserved `system:` groups and usernames from the external directory to kube-apiserver. [#21896](https://github.com/deckhouse/deckhouse/pull/21896)
    Affects clusters where `publishAPI` is enabled together with `enableBasicAuth` on a
    Crowd, OIDC or LDAP `DexProvider`. Both are disabled by default, so a default
    installation is not affected.
    
    Previously the proxy copied directory group names into the `X-Remote-Group` header
    verbatim. Because kube-apiserver trusts that header from this proxy, a user in a
    directory group literally named `system:masters` was granted cluster-admin. Group
    names are chosen by the directory administrator, not by Kubernetes.
    
    After updating, any group whose name begins with `system:` is dropped before the
    request reaches kube-apiserver, and each drop is logged by the basic-auth-proxy pod
    with the user and the group name. A user authenticating with a `system:`-prefixed
    username is refused with `403 Forbidden`. This matches the `userValidationRules`
    already enforced on the OIDC/JWT path.
    
    Before updating, audit your directory for groups whose name begins with `system:`
    and check whether anyone currently relies on one to reach the Kubernetes API through
    basic auth. Such users will lose those privileges on update and must be granted
    access through a normal group bound by RBAC instead. Check the basic-auth-proxy logs
    after updating for `dropping group` messages.
 - **[user-authn]** basic-auth-proxy reaches Dex through its Service and verifies the certificate of Dex, so Basic Authentication through an LDAP provider works in clusters where pods cannot reach the public address of Dex.
    basic-auth-proxy pods restart.
 - **[user-authn]** dex-authenticator no longer writes the request headers, cookies included, to the log when the CSRF cookie is missing.
 - **[user-authn]** fix cve [#21817](https://github.com/deckhouse/deckhouse/pull/21817)
 - **[user-authn]** user-api accepts only the ID tokens of the Deckhouse web interface and can only sign the user out everywhere or require a password change; the password reset endpoint is removed.
    The Deckhouse web interface must use the new user-api endpoints for the password change and the sign-out on all devices.
 - **[user-authz]** A ClusterRole that carries a platform aggregation label without declaring itself a custom capability is now refused; such an object used to be merged into the platform role of that lineage for every holder in the cluster. [#23124](https://github.com/deckhouse/deckhouse/pull/23124)
 - **[user-authz]** A `kube-apiserver` health probes no longer fail when `authz-webhook` is unavailable.
 - **[user-authz]** A custom system or subsystem role is projected into the namespaces of its modules as `viewer` at most; a higher `rbac.deckhouse.io/use-role` label is refused by the webhook and ignored by the controller with a `ProjectionCapped` event.
    Custom system or subsystem roles labelled `rbac.deckhouse.io/use-role` above `viewer` are projected as `viewer` after the update; the `d8_user_authz_projection_capped` metric names them, and any later change of such a role is refused until the label is set to `viewer` (without the label the role is not projected). A capability not built into the platform no longer adds its `rbac.deckhouse.io/namespace` to a projection.
 - **[user-authz]** A namespace denial no longer reveals whether the namespace exists. [#21672](https://github.com/deckhouse/deckhouse/pull/21672)
 - **[user-authz]** Added missing RBAC v1 rules for pods/resize (Editor+), serviceaccounts/token (Admin+) and namespaces/finalize (ClusterAdmin). [#22470](https://github.com/deckhouse/deckhouse/pull/22470)
    Permission expansion for existing bindings.
    
    - Users already bound to Editor (and above) will gain in-place pod resource resize (pods/resize: get,patch), which changes container requests/limits on running pods without recreating them.
    - Users already bound to Admin (and above) will gain the ability to mint ServiceAccount tokens (serviceaccounts/token: create) in their namespaces."
 - **[user-authz]** Below the superadmin level, creating an object with the `deckhouse.io/system-resource` label, or putting the label into the pod template of a workload, is denied, as changing or deleting such an object already was.
 - **[user-authz]** Create the user-authz-controller namespace and registry secret regardless of MultiTenancy, so the module converges with MultiTenancy disabled. [#22921](https://github.com/deckhouse/deckhouse/pull/22921)
 - **[user-authz]** Creating ModuleConfig user-authz without enableMultiTenancy no longer fails with an internal webhook error.
 - **[user-authz]** Fix multitenancy check hooks [#22158](https://github.com/deckhouse/deckhouse/pull/22158)
 - **[user-authz]** Fix multitenancy validatiing webhook to read effective enableMultiTenancy from the Module `v1alpha2` CR [#21936](https://github.com/deckhouse/deckhouse/pull/21936)
 - **[user-authz]** Fixed CVE-2026-63209 in `github.com/klauspost/compress` and CVE-2026-78661 and CVE-2026-97045 in `golang.org/x/net`.
 - **[user-authz]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` and CVE-2026-56851 in `golang.org/x/text`.
 - **[user-authz]** Fixed CVEs in the `permission-browser-apiserver` image. [#23287](https://github.com/deckhouse/deckhouse/pull/23287)
 - **[user-authz]** Grant custom ClusterRoles (annotated with `user-authz.deckhouse.io/access-level`) through one aggregated ClusterRole per access level, so the number of bindings per ClusterAuthorizationRule/AuthorizationRule no longer depends on the number of such roles. [#22824](https://github.com/deckhouse/deckhouse/pull/22824)
    During the first `user-authz` release after the update the per-role bindings are protected from the release engine, the aggregated ones are created, and the per-role bindings are deleted right after the release. Permissions granted through annotated ClusterRoles stay in place throughout.
    The `user-authz.deckhouse.io/access-level` label is now set automatically on annotated ClusterRoles.
    In audit logs, access granted through annotated ClusterRoles is attributed to `user-authz:<level>:custom` instead of the individual ClusterRole.
 - **[user-authz]** Group the module alerts under a group named apart from the alerts themselves, so the incident shelf accepts them. [#22928](https://github.com/deckhouse/deckhouse/pull/22928)
 - **[user-authz]** Managers who assign roles by range can again change DexProviders, UserOperations, Users and Groups of identities that hold a ClusterAuthorizationRule.
 - **[user-authz]** Only the Commander agent may create or change ClusterRoles named with the `d8:commander:` prefix; they are no longer exempt from the RBAC v2 admission checks for other users.
    A `d8:commander:*` ClusterRole created or changed by anyone except system:serviceaccount:d8-commander-agent:commander-agent is now refused, including a re-apply of such a role by a GitOps tool or kubectl. Roles already in the cluster keep working and can still be deleted.
 - **[user-authz]** Refuse to disable `enableMultiTenancy` while a ClusterAuthorizationRule sets only `namespaceSelector`.
 - **[user-authz]** Reject empty subject names in AuthorizationRule and ClusterAuthorizationRule. [#21998](https://github.com/deckhouse/deckhouse/pull/21998)
 - **[user-authz]** Reject local Group names and User emails that collide with a subject already granted privileges by an authorization rule. [#22218](https://github.com/deckhouse/deckhouse/pull/22218)
    Creating a `Group` or a `User`, or renaming an existing one, so that `Group.spec.name` matches a group subject or `User.spec.email` matches a user subject of an existing AuthorizationRule or ClusterAuthorizationRule is now rejected, preventing an unintended privilege grant. The email is compared after lowercasing, since that is the form that reaches the token. Set the `user-authz.deckhouse.io/allow-authorization-rule-collision` annotation to `"true"` on the object if the collision is intentional. Already existing objects keep working and only produce a warning. Any declarative flow that manages both the rule and the identity is affected, not only one that deletes and recreates the identity: a first-time apply denies the `Group` or `User` whenever the rule happens to be applied first, and nothing guarantees the order. Add the annotation to such objects before upgrading. Deleting a `Group` or a `User` whose name is still a subject of a rule now produces a warning, since the rule keeps granting that name.
 - **[user-authz]** Remove node subresources from the RBACv2 viewer permission role [#22386](https://github.com/deckhouse/deckhouse/pull/22386)
    The `d8:manage:permission:subsystem:kubernetes:view_resources` role no longer grants `nodes/proxy`,
    `nodes/log`, `nodes/metrics` and `nodes/stats`. Users holding `d8:manage:kubernetes:viewer`,
    `d8:manage:infrastructure:viewer` or any role aggregating them lose access to the kubelet API through
    `/api/v1/nodes/<node>/proxy/...`. `nodes` themselves stay readable. Automation that relied on reaching
    the kubelet API with one of these roles must be granted a dedicated role explicitly.
 - **[user-authz]** ServiceAccounts of all `d8-*` namespaces bypass the system-resource protection, not only those of `d8-system` and `kube-system`.
    They may create, change and delete objects labeled `deckhouse.io/system-resource: "true"` and run `exec`, `attach` and `port-forward` in such pods. permission-browser reports them as superadmin in `SubjectAccessReport`.
 - **[user-authz]** Speed up permission-browser BulkSAR for users with namespace filters [#22731](https://github.com/deckhouse/deckhouse/pull/22731)
 - **[user-authz]** Subjects that hold a project role through ProjectRoleBinding or ClusterProjectRoleBinding receive the read-only dictionaries (d8:dict) like namespace-role holders; the exec webhook binding excludes the aggregation controller and the multitenancy-manager controller at the API server. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
 - **[user-authz]** The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert reports every custom role that still selects capabilities by the old `rbac.deckhouse.io/kind` label or by the aggregation label of a lineage no built-in role collects, whatever its name, no longer counts frozen roles and capabilities, and describes how to restore the access of such a role.
 - **[user-authz]** The authorization webhook answers the requests in flight when it restarts instead of denying them.
 - **[user-authz]** The deprecated d8:use:role:* aliases can no longer be the target of a new ProjectRoleBinding or ClusterProjectRoleBinding; existing RoleBindings to them keep working. [#23022](https://github.com/deckhouse/deckhouse/pull/23022)
    A ProjectRoleBinding or ClusterProjectRoleBinding that names a `d8:use:role:*` alias is refused with a pointer to the `d8:namespace:*` role; migrate such bindings to the new names.
 - **[user-authz]** The identity-assign check treats an identity with every permission across the cluster, such as a subject of `cluster-admin` bound by a ClusterRoleBinding, as SuperAdmin.
    Such identities can write ClusterAuthorizationRules, Users, Groups, UserOperations and DexProviders that reach `SuperAdmin` or `cluster-admin`. A ClusterAuthorizationRule gives this right only through its bindings, only without `namespaceSelector` and `limitNamespaces`, and, with multi-tenancy on, only with `allowAccessToSystemNamespaces: true`, even if it grants `cluster-admin`. Project role bindings never do.
 - **[user-authz]** The system and IAM managers assign roles up to the manager level and ClusterEditor, and cannot open the system namespaces through a ClusterAuthorizationRule.
    Holders of `d8:system:manager`, `d8:subsystem:iam:manager` and the deprecated `d8:manage:all:manager` and `d8:manage:security:manager` can no longer write a ClusterAuthorizationRule with `Admin`, `ClusterAdmin` or `SuperAdmin` or one that opens the system namespaces (`allowAccessToSystemNamespaces`, `namespaceSelector.matchAny`, a `labelSelector` that does not name its namespaces by `kubernetes.io/metadata.name`, a rule for a request another such rule already lets in, the requester's own included, or any rule while multitenancy mode is disabled, so in CE no ClusterAuthorizationRule at all), cannot create Users, add Group members, run UserOperations or connect DexProviders for the identities of such rules, cannot add a member already let into the system namespaces to a Group with such a rule, need the range for the roles of the groups above a Group they write, cannot turn multitenancy mode off, cannot assign `admin` and `superadmin` roles of the granular model, and cannot write admission webhooks and policies. Existing rules stay; a ClusterAdmin or a SuperAdmin does these.
 - **[user-authz]** fix cve [#21817](https://github.com/deckhouse/deckhouse/pull/21817)
 - **[vertical-pod-autoscaler]** Fix a panic in vpa-updater and vpa-admission-controller when a pod with a container without resources is capped to a LimitRange of type Pod.
 - **[vertical-pod-autoscaler]** On Kubernetes 1.33 and later, VPA objects without updateMode now use the InPlaceOrRecreate mode, as the documentation describes, instead of Recreate.
    On Kubernetes 1.33 and later, new VerticalPodAutoscaler objects without spec.updatePolicy get updateMode set to InPlaceOrRecreate. VPA resizes their pods in place where possible and evicts pods only as a fallback. This includes running pods of single-replica workloads, which VPA did not update before. Existing VPA objects keep the update mode already stored in them. The vpa-recommender, vpa-updater and vpa-admission-controller pods are restarted.
 - **[vertical-pod-autoscaler]** Reduce VPA memory recommendation rounding from 64Mi to 16Mi and round CPU recommendations up to 10m. [#22509](https://github.com/deckhouse/deckhouse/pull/22509)
    The `vpa-recommender` pod is restarted and recommendations are recalculated with the new granularity.
    Memory recommendations will generally become lower (less over-provisioning), CPU recommendations become
    multiples of 10m. VPA objects with `updateMode: Auto`/`Recreate` may evict and recreate pods to apply
    the updated requests.

## Chore


 - **[admission-policy-engine]** The `reserved-policy-names` and `securitypolicies` checks keep their rules and messages, now served from ValidationWebhook resources.
    During the update to this release, the `reserved-policy-names` and `securitypolicies` validating
    webhooks are absent for a few minutes (3–4 minutes on a test cluster), until the module chart
    creates the `ValidationWebhook` resources and `webhook-handler` loads them.
    While they are absent, the API server does not check the names of new policies or the overlap of
    image references in `SecurityPolicy` objects.
    The window occurs once per cluster, during this update.
    Plan the update for a time when you can hold back changes to `SecurityPolicy` and `OperationPolicy`.
 - **[admission-policy-engine]** Update Gatekeeper to 3.23.1 and Ratify to 1.4.6, and fix vulnerabilities in their images.
 - **[candi]** Base images updated to v3.1.4
 - **[candi]** Bump Kubernetes patch versions to 1.34.13, 1.35.10, 1.36.6 and 1.37.2.
    Kubernetes control plane components and kubelet will be restarted.
 - **[candi]** Bump patch versions of Kubernetes images. [#22663](https://github.com/deckhouse/deckhouse/pull/22663)
    Kubernetes control-plane components will restart, kubelet will restart
 - **[candi]** Enable the `DRADeviceTaints` feature gate by default for Kubernetes 1.34+. [#21675](https://github.com/deckhouse/deckhouse/pull/21675)
    kube-apiserver, kube-scheduler, and kube-controller-manager will restart on clusters running Kubernetes 1.34 or newer.
 - **[candi]** Migrate the build to container-base v4.0.5-alpha.
 - **[candi]** Switch `distroless` and `scratch` base images to the container-base ones.
 - **[candi]** The Basis Dynamix infrastructure toolchain is updated — the decort provider from 4.7.3 to 4.12.2, and OpenTofu from 1.12.0 to 1.12.5.
 - **[candi]** Updated base images to v1.3.22 to address yq CVEs. [#21849](https://github.com/deckhouse/deckhouse/pull/21849)
 - **[candi]** base images updated to v3.1.11
 - **[candi]** base images updated to v3.1.15
 - **[candi]** base images updated to v4.0.14
 - **[candi]** bump cosign to 3.1.3/2.6.5 [#22713](https://github.com/deckhouse/deckhouse/pull/22713)
 - **[candi]** inherit common-base and builder/src from container-base, ship erofs with pm
 - **[candi]** minget removed from alt_base_images [#22676](https://github.com/deckhouse/deckhouse/pull/22676)
 - **[candi]** remove kube 1.31 in cse [#21705](https://github.com/deckhouse/deckhouse/pull/21705)
 - **[candi]** update base images [#22118](https://github.com/deckhouse/deckhouse/pull/22118)
 - **[cert-manager]** Module's templates were updated to use helm snippets. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[chrony]** Images are now based on base/distroless.
 - **[cilium-hubble]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[cilium-hubble]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[cloud-provider-aws]** Build the module images on Container Base (`pm`, `distroless`).
 - **[cloud-provider-vsphere]** move csi-vsphere-syncer image from 000-common into the cloud-provider-vsphere module [#21150](https://github.com/deckhouse/deckhouse/pull/21150)
 - **[cloud-provider-yandex]** Build the module images on Container Base (`pm`, `distroless`).
 - **[cloud-provider-yandex]** The StorageClass hook is moved to the shared storage class helpers without behaviour changes.
 - **[cni-cilium]** BPF conntrack, NAT, neighbor and socket reverse NAT map sizes can now be set per node with CiliumNodeConfig. [#23372](https://github.com/deckhouse/deckhouse/pull/23372)
    `cilium-agent` Pods will be restarted.
 - **[cni-cilium]** Build check-kernel-version with true from pm.
 - **[cni-flannel]** Fixed securityContext and added SecurityPolicyException [#21667](https://github.com/deckhouse/deckhouse/pull/21667)
 - **[cni-simple-bridge]** Fixed securityContext and added SecurityPolicyException [#21670](https://github.com/deckhouse/deckhouse/pull/21670)
 - **[common]** Remove `etcdctl` and `etcdutl` from the debug container image. [#21995](https://github.com/deckhouse/deckhouse/pull/21995)
 - **[common]** Switch leftover Go image builds to builder/golang and pm instead of Alpine/apk [#21568](https://github.com/deckhouse/deckhouse/pull/21568)
 - **[control-plane-manager]** Fixed upstream Kubernetes tests broken by the Deckhouse patches.
    Control plane components (kube-apiserver, kube-controller-manager, kube-scheduler) will be restarted.
 - **[control-plane-manager]** Images are now based on base/distroless.
 - **[control-plane-manager]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[control-plane-manager]** Update Kubernetes patch versions to 1.34.10, 1.35.7 and 1.36.3 [#21910](https://github.com/deckhouse/deckhouse/pull/21910)
 - **[control-plane-manager]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[deckhouse-controller]** Bump nelm to v1.31.0. The `status.tracking` report of Application and Module now has the operation graph (`id`, `category`, `dependsOn`), the `Canceled` status, and no `waitingFor`. [#23190](https://github.com/deckhouse/deckhouse/pull/23190)
 - **[deckhouse-tools]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[deckhouse-tools]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[deckhouse]** Added deckhouse-modules PackageRepository configuration. [#22190](https://github.com/deckhouse/deckhouse/pull/22190)
 - **[deckhouse]** Allow ClusterAdmin manage ModuleSettingsDefinitions with RBAC. [#21741](https://github.com/deckhouse/deckhouse/pull/21741)
 - **[deckhouse]** Global openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
    OpenAPI version for global values is updated to 3.
 - **[deckhouse]** Removed the `app` short name from the `Application` resource. [#22107](https://github.com/deckhouse/deckhouse/pull/22107)
 - **[deckhouse]** Report deletion in the Application and Module status. [#22759](https://github.com/deckhouse/deckhouse/pull/22759)
 - **[deckhouse]** The module has been moved to a dedicated repository.
    On DKP 1.78 the module will be installed from an external repository.
 - **[deckhouse]** Update Go dependencies. [#22501](https://github.com/deckhouse/deckhouse/pull/22501)
 - **[deckhouse]** Update nelm version to v1.30.3. [#22990](https://github.com/deckhouse/deckhouse/pull/22990)
 - **[deckhouse]** `registry.mode: Proxy` and `Local` in the `deckhouse` ModuleConfig are deprecated and can no longer be set on any cluster. The `D8DeckhouseRegistryModeUnsupported` alert reports a `deckhouse` ModuleConfig that still stores `registry.mode: Proxy` or `Local`. The secret `d8-system/registry-config` is no longer created.
 - **[deckhouse]** add tests for the application controller [#22211](https://github.com/deckhouse/deckhouse/pull/22211)
 - **[descheduler]** Added the logLevel setting to control how detailed the descheduler logs are.
    The descheduler Pod restarts once on upgrade. By default, logs are now much shorter (`Warning` level).
 - **[docs]** Added ability to to specify localized title and description in OpenAPI example to render them in the documentation. [#21636](https://github.com/deckhouse/deckhouse/pull/21636)
 - **[docs]** Docs about publishAPI parameter (in according to PR 18628 and 23108 ) was edited, info about kubeconfig generator domain was removed. [#23041](https://github.com/deckhouse/deckhouse/pull/23041)
 - **[docs]** Make PublicDomainTemplate for Applications with two placeholders instead of three. [#21747](https://github.com/deckhouse/deckhouse/pull/21747)
 - **[documentation]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[documentation]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[istio]** Alliance-healthcheck now auto-restarts when globalVersion has been changed [#21766](https://github.com/deckhouse/deckhouse/pull/21766)
 - **[istio]** Fix and extend module known_vulnerabilities.vex for config-analyzer and 1.29 images. [#21989](https://github.com/deckhouse/deckhouse/pull/21989)
 - **[istio]** Fixed CVE in waipoint-controller. [#22188](https://github.com/deckhouse/deckhouse/pull/22188)
 - **[istio]** Fixed prometheus, oras-go and go CVEs. [#21874](https://github.com/deckhouse/deckhouse/pull/21874)
 - **[istio]** Increase east-west gateway default resource requests and VPA limits. [#23031](https://github.com/deckhouse/deckhouse/pull/23031)
 - **[istio]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[istio]** Remove Istio 1.21 support and make Istio 1.25 the minimum supported version. [#22180](https://github.com/deckhouse/deckhouse/pull/22180)
 - **[istio]** Run non-dataplane Istio components as the deckhouse user and clear imageSpec.config.user [#21863](https://github.com/deckhouse/deckhouse/pull/21863)
    Restarts istiod, istio-operator, Kiali, istio-cni, and config-analyzer.
 - **[istio]** Set readOnlyRootFilesystem to true in the istio-init container. [#21626](https://github.com/deckhouse/deckhouse/pull/21626)
 - **[istio]** istioctl now always uses the revision associated with the binary [#22283](https://github.com/deckhouse/deckhouse/pull/22283)
 - **[istio]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[istio]** prepared module for PSS restricted checks [#21825](https://github.com/deckhouse/deckhouse/pull/21825)
 - **[istio]** refactor istioctl default namespace patch [#22244](https://github.com/deckhouse/deckhouse/pull/22244)
 - **[keepalived]** Fixed securityContext and added SecurityPolicyException [#21583](https://github.com/deckhouse/deckhouse/pull/21583)
 - **[kube-proxy]** Fixed securityContext and added SecurityPolicyException [#21665](https://github.com/deckhouse/deckhouse/pull/21665)
 - **[kube-proxy]** Rebuild kube-proxy with the new Kubernetes patch versions.
    kube-proxy pods will be restarted.
 - **[kube-proxy]** Rebuilt kube-proxy with the updated Kubernetes patches.
    kube-proxy pods will be restarted.
 - **[local-path-provisioner]** Images are now based on base/distroless.
 - **[network-gateway]** Build the relocate artifact of iptables-wrapper-init on the pm based builder image.
 - **[network-gateway]** Fixed securityContext and added SecurityPolicyException [#21584](https://github.com/deckhouse/deckhouse/pull/21584)
 - **[network-policy-engine]** Build kube-router on the pm based base image.
 - **[network-policy-engine]** Fixed securityContext and added SecurityPolicyException [#21668](https://github.com/deckhouse/deckhouse/pull/21668)
 - **[node-local-dns]** Build the relocate artifact of iptables-wrapper-init on the pm based builder image.
 - **[node-manager]** Containerd v1 is removed from the CSE edition [#22085](https://github.com/deckhouse/deckhouse/pull/22085)
 - **[node-manager]** Go hooks are grouped into packages by domain (core, scheduling, cloud, kubernetes, gpu, upmeter); hook names now include the folder. [#23086](https://github.com/deckhouse/deckhouse/pull/23086)
 - **[node-manager]** Images are now based on base/distroless.
 - **[node-manager]** Update cluster-autoscaler to gardener v1.34.4, v1.35.2 and v1.36.1.
 - **[node-manager]** build registrypackages from container-factory with pm, and verify package completeness at build time
 - **[node-manager]** cluster-autoscaler versions for each Kubernetes version are now defined in the module's `oss.yaml`.
 - **[openvpn]** Build openvpn, pmacct and ovpn-admin on the pm based images.
 - **[openvpn]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[openvpn]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[registry-packages-proxy]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[registry-packages-proxy]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[registry]** Images are now based on base/distroless.
 - **[registry]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[registry]** The registry module documentation describes the in-cluster cache as it works now (replication, leader election, the publication endpoint, store sizing, containerd's direct path, containerd restarts during migration), and the disk requirement guides account for the store on master nodes.
 - **[registry]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[terraform-manager]** Images are now based on base/distroless.
 - **[user-authn]** Module's openAPI was updated to support disabling creating Ingress/Gateway API resources. [#22280](https://github.com/deckhouse/deckhouse/pull/22280)
 - **[user-authn]** Remove the kubeconfig-generator web UI and the kubeconfig.<publicDomainTemplate> hostname. [#23108](https://github.com/deckhouse/deckhouse/pull/23108)
    The kubeconfig generation web UI is removed and the `kubeconfig.<publicDomainTemplate>`
    hostname stops resolving; there is no redirect to a replacement page.
    Kubeconfigs already issued to users keep working: the dex OAuth2 clients behind them
    remain in place until DKP 1.79.
    Users should re-issue their kubeconfig from the Deckhouse console, which provides the
    same functionality.
    The `userAuthn.kubeconfigGenerator` module setting is not affected and keeps working —
    the console reads the API endpoints defined there.
    On clusters that served the UI with `https.mode: CertManager`, the TLS secrets issued for
    the removed hostname (`kubeconfig-ingress-tls`, plus `kubeconfig-httproute-tls` where
    Gateway API was used) remain in the `d8-user-authn` namespace after the upgrade. Their
    Certificates set `certificateOwnerRef: false`, so deleting them does not garbage-collect
    the secrets. The leftovers are unreferenced and no longer renewed; a migration in DKP 1.79
    removes them.
 - **[user-authn]** Several warnings the User and Group checks return for one request are now shown as one line separated by `; `.
    During the update to this release, the `users-unique` and `groups-unique` validating webhooks are
    absent for a few minutes (3–4 minutes on a test cluster), until the module chart creates the
    `ValidationWebhook` resources and `webhook-handler` loads them.
    While they are absent, the API server does not check `User` and `Group` objects for duplicate
    emails, uppercase emails or group cycles.
    The window occurs once per cluster, during this update.
    Plan the update for a time when you can hold back changes to users and groups.
 - **[user-authn]** module Ingress resources migrated from unsafe NGINX snippet annotations
 - **[user-authz]** The `d8:use:dict` grants and the projections of manage bindings into namespaced use RoleBindings are reconciled by `user-authz-controller` instead of module hooks. [#22832](https://github.com/deckhouse/deckhouse/pull/22832)
    Two behaviour changes of the experimental role model: subjects of use RoleBindings whose identity differs only after the 55th character used to share one `d8:use:dict` grant and now each get their own; the namespaces a manage role reaches are resolved through its aggregation rule to any depth (including `matchExpressions`), not only one level down, so a deeper role hierarchy projects into every module namespace it aggregates.
 - **[user-authz]** The user-authz validating webhooks keep their rules and messages, now served from ValidationWebhook resources.
    During the update to this release, the validating webhooks of `user-authz` are absent for a few
    minutes (3–4 minutes on a test cluster), until the module chart creates the `ValidationWebhook`
    resources and `webhook-handler` loads them.
    While they are absent, the API server does not run these checks, including the privilege escalation
    guards `d8-user-authz-identity-assign`, `rbacv2-role-escalation`, `rbacv2-system-resource-edit` and
    `rbacv2-system-resource-exec`.
    The window occurs once per cluster, during this update. Later restarts of `webhook-handler` restore
    the webhooks before it starts serving.
    Plan the update for a time when you can hold back changes to RBAC objects, users, groups and
    `ClusterAuthorizationRule` objects.
 - **[vertical-pod-autoscaler]** Images are now based on base/distroless.
 - **[vertical-pod-autoscaler]** Update VPA to upstream 1.8.1.
