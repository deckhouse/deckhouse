# Changelog v1.77.3

## Know before update


 - Before, such an entry stopped the webhook from reloading the rules, so subjects of rules added later, and every subject after a restart of the webhook, were not limited to their namespaces. Subjects of a rule whose only entry is invalid now get no namespace through the entries of that rule.
 - Exec and attach requests in user namespaces reach Gatekeeper again, as in Deckhouse 1.73 and 1.74.
    Every constraint that matches them is evaluated again, including a constraint without `match.kinds`
    or with `kinds: ["*"]` whose policy does not check `input.review.operation`.
    Before updating, check that such constraints do not deny `kubectl exec` and `kubectl attach`
    for users who need them.
 - In a cluster with the `proxy` parameter of `ClusterConfiguration` set, the `cert-manager` controller
    sends requests to the ACME server, HashiCorp Vault and the DNS provider APIs through the proxy server.
    The `HTTP-01` self-check also requests the domain through the proxy server.
    If the proxy server cannot reach the issuer or the public address of the cluster, add these addresses to `noProxy`.
 - Machines are not recreated on update. Machines created before the update keep the standard network type and no labels until the NodeGroup is rolled out (change a rolling InstanceClass field or set the `manual-rollout-id` annotation on the NodeGroup). Machines stuck in provisioning because of `additionalSubnets` become ready without recreation.
 - The grant webhooks are now removed together with the module. On a cluster where `multitenancy-manager` is already disabled at the update, the `cluster-objects-grants-defaulting` and `cluster-objects-grants-validator` webhook configurations left by earlier versions stay, and writes to grantable resources in project namespaces (for example, removing a PVC finalizer) keep failing with `service "multitenancy-manager" not found`. Delete them by hand:
    `d8 k delete mutatingwebhookconfiguration cluster-objects-grants-defaulting`
    `d8 k delete validatingwebhookconfiguration cluster-objects-grants-validator`
    Nothing has to be done on clusters where the module is enabled.

## Features


 - **[admission-policy-engine]** Report the availability of the module components with alerts.
 - **[cert-manager]** Add the `highAvailability` parameter to control the HA mode of the module independently of the global setting.
 - **[cert-manager]** `cert-manager` uses the cluster-wide proxy server from the `proxy` parameter of `ClusterConfiguration`.
    In a cluster with the `proxy` parameter of `ClusterConfiguration` set, the `cert-manager` controller
    sends requests to the ACME server, HashiCorp Vault and the DNS provider APIs through the proxy server.
    The `HTTP-01` self-check also requests the domain through the proxy server.
    If the proxy server cannot reach the issuer or the public address of the cluster, add these addresses to `noProxy`.
 - **[cloud-provider-dvp]** Makes CCM create NPs on every lb creation while using networkPolicy - Restricted
 - **[cni-cilium]** Added an alert and a release requirement that block the Cilium update while CiliumNetworkPolicy or CiliumClusterwideNetworkPolicy objects use the `fromRequires` or `toRequires` fields.
 - **[multitenancy-manager]** A new alert warns about a user project template named `simple`, which the built-in template of the same name overwrites after the update.
 - **[user-authz]** New `D8UserAuthzLimitNamespacesEntryNeedsRewrite` and `D8UserAuthzLimitNamespacesEntryInvalid` alerts report ClusterAuthorizationRule `limitNamespaces` entries that are anchored only in part or are not valid regular expressions.
 - **[user-authz]** The new `D8UserAuthzLegacyRBACv2CapabilityBindingFound` alert lists the bindings to built-in capabilities that the next minor release renames without aliases, and names the new name of each capability.
 - **[user-authz]** The new `D8UserAuthzLegacyRBACv2CustomCapabilityFound` alert lists custom capabilities that extend the namespace roles only through labels the next role scheme does not select.

## Fixes


 - **[admission-policy-engine]** Disabling the module deletes the webhook configurations before Gatekeeper, so the uninstall no longer blocks `exec` and the Deckhouse queue.
 - **[admission-policy-engine]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` and CVE-2026-56851 in `golang.org/x/text`.
 - **[admission-policy-engine]** Fixed Gatekeeper policies for `kubectl exec` and `kubectl attach` being skipped in user namespaces.
    Exec and attach requests in user namespaces reach Gatekeeper again, as in Deckhouse 1.73 and 1.74.
    Every constraint that matches them is evaluated again, including a constraint without `match.kinds`
    or with `kinds: ["*"]` whose policy does not check `input.review.operation`.
    Before updating, check that such constraints do not deny `kubectl exec` and `kubectl attach`
    for users who need them.
 - **[candi]** New nodes reboot on the first bashible run again when a step requests it. Only the first master skips this reboot.
 - **[cert-manager]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`.
 - **[cert-manager]** The `spec.certificateOwnerRef` field of a certificate no longer causes an endless Secret update loop when it differs from the global owner reference setting that `cleanupOrphanSecrets` enables in the `CertManager` HTTPS mode, and is applied to an already issued Secret.
 - **[cert-manager]** The webhook keeps serving requests during a rollout and during a rotation of its CA, and the controller waits for the API server at startup instead of restarting.
 - **[cloud-provider-dvp]** Stop infinite check/converge loop caused by the DVP controller annotation on the cloud-init Secret.
 - **[cloud-provider-dvp]** The CSI driver reports a volume as detached from a node only after the disk is released by the node virtual machine, so with virtualization that keeps the VM listed on the disk until it is released, the volume is not attached to two nodes at once.
 - **[cloud-provider-dvp]** The CSI driver reports the volume limit of a node, so the scheduler does not place more volumes on a node than its virtual machine can attach.
 - **[cloud-provider-vsphere]** Fixed master node creation when the datastore is set as a path with parent folders.
 - **[cloud-provider-yandex]** Restored `additionalSubnets`, `networkType` and VM labels (`nodes.parameters.labels`, `additionalLabels`) for CAPI-managed Yandex nodes.
    Machines are not recreated on update. Machines created before the update keep the standard network type and no labels until the NodeGroup is rolled out (change a rolling InstanceClass field or set the `manual-rollout-id` annotation on the NodeGroup). Machines stuck in provisioning because of `additionalSubnets` become ready without recreation.
 - **[cloud-provider-zvirt]** VMs are now actually removed from zVirt on NodeGroup scale-down and cluster destroy
 - **[common]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` in kube-rbac-proxy.
 - **[common]** Fixed a `kube-apiserver` panic on JSON Patch requests whose `add`/`replace` of the whole document or `test` operation lacks the `value` field; such requests are now rejected with a validation error.
 - **[common]** kube-apiserver returns a single short validation error for an invalid `fieldManager` instead of an error for every invalid character.
 - **[control-plane-manager]** Keep `resource.k8s.io/v1beta1` enabled in `kube-apiserver` 1.34.
 - **[deckhouse-controller]** Deleting an Application now also deletes objects with the `werf.io/ownership: anyone` or `werf.io/resource-policy` annotation, such as Jobs deployed only on install, if the package lists their kinds in `orphanResources`. Only `helm.sh/resource-policy: keep` protects an object.
 - **[deckhouse]** Restore kubectl plugin lookup in d8 k and d8 kubectl
 - **[deckhouse]** Update runtime libraries and VEX exceptions for known vulnerabilities
 - **[dhctl]** Destroy and abort of a static cluster now fail instead of reporting success when a control-plane node was not cleaned because of an SSH error or cleanup timeout.
 - **[dhctl]** Preserve existing SSH keys when switching to the temporary converge user.
 - **[helm_lib]** Size the resources of a module whose `resourcesManagement` names a fractional quantity, instead of rendering a zero limit.
    Bumping `deckhouse_lib_helm` from `1.72.11` to `1.72.24` also drops the `drbd.linbit.com` taint
    tolerations from `helm_lib_tolerations`, so pods rendered through it no longer tolerate
    `lost-quorum`, `force-io-error` and `ignore-fail-over`, and aligns the VPA `maxAllowed` memory to
    the recommender's 64Mi quantum.
 - **[multitenancy-manager]** Disabling the module now removes the cluster-objects grant webhooks, which used to stay behind and block writes in project namespaces.
    The grant webhooks are now removed together with the module. On a cluster where `multitenancy-manager` is already disabled at the update, the `cluster-objects-grants-defaulting` and `cluster-objects-grants-validator` webhook configurations left by earlier versions stay, and writes to grantable resources in project namespaces (for example, removing a PVC finalizer) keep failing with `service "multitenancy-manager" not found`. Delete them by hand:
    `d8 k delete mutatingwebhookconfiguration cluster-objects-grants-defaulting`
    `d8 k delete validatingwebhookconfiguration cluster-objects-grants-validator`
    Nothing has to be done on clusters where the module is enabled.
 - **[multitenancy-manager]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`.
 - **[multitenancy-manager]** The Project conversions of the next release are registered in advance, so reads and writes of projects keep working while webhook-handler is updated in HA mode.
 - **[multitenancy-manager]** The controller takes its leader lease with any number of replicas, so a rollout of a single-replica installation no longer has two pods upgrading project Helm releases at once.
    In single-replica installations a new controller pod reconciles projects only after the previous pod has stopped and released the lease. If the previous pod is lost without a shutdown, for example together with its node, the new pod starts reconciling projects 60 to about 78 seconds after it starts.
 - **[multitenancy-manager]** The webhooks no longer leave zombie processes in webhook-handler until it runs out of pids.
 - **[node-manager]** A StaticInstance whose StaticMachine was deleted is returned to Pending and becomes available for bootstrapping again instead of staying out of the pool forever.
 - **[node-manager]** Fixed cluster-autoscaler not scaling CAPI node groups up from zero for pods requesting ephemeral storage, and putting the whole scale-up of a multi-zone node group into a single zone.
 - **[node-manager]** Fixed cluster-autoscaler stopping autoscaling of all node groups and restarting every two hours when an MCM node group has `minPerZone` equal to `maxPerZone`.
 - **[node-manager]** Fixed cluster-autoscaler using node group names without the cluster prefix after a Deckhouse restart, which stopped autoscaling and triggered the `D8ClusterAutoscalerTooManyErrors` alert.
 - **[node-manager]** Fixed the CAPI MachineHealthCheck replacing cloud nodes that take longer than 5 minutes to become Ready after registering, and limited remediation to when at most half of the CAPI machines are unhealthy.
    The cloud MachineHealthCheck is updated in existing clusters. A node in Ready=False is replaced after 10 minutes instead of 5 (Ready=Unknown still after 5), and remediation stops while more than 50% of the cluster's CAPI machines are unhealthy.
 - **[user-authn]** A DexAuthenticator with a name longer than 45 characters no longer logs its users out on every run of the hook.
 - **[user-authn]** A DexClient or a DexAuthenticator in a namespace whose name YAML reads as a number or a boolean no longer breaks the user-authn release.
 - **[user-authn]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`.
 - **[user-authn]** Protected applications keep serving users during a short dex outage and during restarts.
 - **[user-authn]** Restrict the Job that issues the `basic-auth-proxy` certificate to the read-only front-proxy CA files and cover it with a SecurityPolicyException.
 - **[user-authn]** The `idTokenTTL` description and the `D8UserAuthnIDTokenTTLTooLong` alert say that a value of 6 hours or more is replaced with `5h59m` by the next settings version, and what lowering it does to DexAuthenticator sessions and issued ID tokens.
 - **[user-authn]** The webhooks no longer leave zombie processes in webhook-handler until it runs out of pids.
 - **[user-authz]** An AuthorizationRule in a namespace whose name YAML reads as a number, a boolean or null no longer breaks the module's bindings.
 - **[user-authz]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` and CVE-2026-56851 in `golang.org/x/text`.
 - **[user-authz]** Managers who assign roles by range can again change DexProviders, UserOperations, Users and Groups of identities that hold a ClusterAuthorizationRule.
 - **[user-authz]** The RBACv2 roles d8:manage:* and d8:use:role:* keep the rights of the modules that have already switched to the capability labels of the next release while the upgrade to it is in progress.
 - **[user-authz]** The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert lists every custom role that selects capabilities by the `rbac.deckhouse.io/kind: manage|use` label or by the aggregation label of a lineage the next role model does not collect, or that loses its RoleBindings in the namespaces of the modules, and the FAQ describes how to prepare such a role before the update without losing access.
 - **[user-authz]** The authorization webhook answers the requests in flight when it restarts instead of denying them.
 - **[user-authz]** The authorization webhook keeps applying ClusterAuthorizationRules when a `limitNamespaces` entry is not a valid regular expression; the entry is left out, and its rule covers fewer namespaces.
    Before, such an entry stopped the webhook from reloading the rules, so subjects of rules added later, and every subject after a restart of the webhook, were not limited to their namespaces. Subjects of a rule whose only entry is invalid now get no namespace through the entries of that rule.
 - **[user-authz]** The authorization webhook no longer restarts while the API server is unreachable, which could deadlock the control plane on bootstrap or an API server restart with multitenancy enabled.
 - **[user-authz]** The identity-assign check treats an identity with every permission across the cluster, such as a subject of `cluster-admin` bound by a ClusterRoleBinding, as SuperAdmin.
    Such identities can write ClusterAuthorizationRules, Users and Groups that carry `SuperAdmin` or `cluster-admin`. A ClusterAuthorizationRule gives this right only without `namespaceSelector` and `limitNamespaces` and, with multi-tenancy on, only with `allowAccessToSystemNamespaces: true`, even if it grants `cluster-admin`.
 - **[user-authz]** The webhooks no longer leave zombie processes in webhook-handler until it runs out of pids.
 - **[user-authz]** a `kube-apiserver` health probes no longer fail when `authz-webhook` is unavailable.
