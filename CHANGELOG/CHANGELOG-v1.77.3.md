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


 - **[admission-policy-engine]** Report the availability of the module components with alerts. [#384](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/384)
 - **[cert-manager]** Add the `highAvailability` parameter to control the HA mode of the module independently of the global setting. [#504](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/504)
 - **[cert-manager]** `cert-manager` uses the cluster-wide proxy server from the `proxy` parameter of `ClusterConfiguration`. [#688](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/688)
    In a cluster with the `proxy` parameter of `ClusterConfiguration` set, the `cert-manager` controller
    sends requests to the ACME server, HashiCorp Vault and the DNS provider APIs through the proxy server.
    The `HTTP-01` self-check also requests the domain through the proxy server.
    If the proxy server cannot reach the issuer or the public address of the cluster, add these addresses to `noProxy`.
 - **[cloud-provider-dvp]** Makes CCM create NPs on every lb creation while using networkPolicy - Restricted [#656](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/656)
 - **[cni-cilium]** Added an alert and a release requirement that block the Cilium update while CiliumNetworkPolicy or CiliumClusterwideNetworkPolicy objects use the `fromRequires` or `toRequires` fields. [#687](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/687)
 - **[multitenancy-manager]** A new alert warns about a user project template named `simple`, which the built-in template of the same name overwrites after the update. [#239](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/239)
 - **[user-authz]** New `D8UserAuthzLimitNamespacesEntryNeedsRewrite` and `D8UserAuthzLimitNamespacesEntryInvalid` alerts report ClusterAuthorizationRule `limitNamespaces` entries that are anchored only in part or are not valid regular expressions. [#240](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/240)
 - **[user-authz]** The new `D8UserAuthzLegacyRBACv2CapabilityBindingFound` alert lists the bindings to built-in capabilities that the next minor release renames without aliases, and names the new name of each capability. [#396](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/396)
 - **[user-authz]** The new `D8UserAuthzLegacyRBACv2CustomCapabilityFound` alert lists custom capabilities that extend the namespace roles only through labels the next role scheme does not select. [#240](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/240)

## Fixes


 - **[admission-policy-engine]** Disabling the module deletes the webhook configurations before Gatekeeper, so the uninstall no longer blocks `exec` and the Deckhouse queue. [#510](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/510)
 - **[admission-policy-engine]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` and CVE-2026-56851 in `golang.org/x/text`. [#733](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/733)
 - **[admission-policy-engine]** Fixed Gatekeeper policies for `kubectl exec` and `kubectl attach` being skipped in user namespaces. [#463](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/463)
    Exec and attach requests in user namespaces reach Gatekeeper again, as in Deckhouse 1.73 and 1.74.
    Every constraint that matches them is evaluated again, including a constraint without `match.kinds`
    or with `kinds: ["*"]` whose policy does not check `input.review.operation`.
    Before updating, check that such constraints do not deny `kubectl exec` and `kubectl attach`
    for users who need them.
 - **[candi]** New nodes reboot on the first bashible run again when a step requests it. Only the first master skips this reboot. [#643](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/643)
 - **[cert-manager]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`. [#732](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/732)
 - **[cert-manager]** The `spec.certificateOwnerRef` field of a certificate no longer causes an endless Secret update loop when it differs from the global owner reference setting that `cleanupOrphanSecrets` enables in the `CertManager` HTTPS mode, and is applied to an already issued Secret. [#688](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/688)
 - **[cert-manager]** The webhook keeps serving requests during a rollout and during a rotation of its CA, and the controller waits for the API server at startup instead of restarting. [#680](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/680)
 - **[cloud-provider-dvp]** Stop infinite check/converge loop caused by the DVP controller annotation on the cloud-init Secret. [#430](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/430)
 - **[cloud-provider-dvp]** The CSI driver reports a volume as detached from a node only after the disk is released by the node virtual machine, so with virtualization that keeps the VM listed on the disk until it is released, the volume is not attached to two nodes at once. [#344](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/344)
 - **[cloud-provider-dvp]** The CSI driver reports the volume limit of a node, so the scheduler does not place more volumes on a node than its virtual machine can attach. [#344](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/344)
 - **[cloud-provider-vsphere]** Fixed master node creation when the datastore is set as a path with parent folders. [#361](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/361)
 - **[cloud-provider-yandex]** Restored `additionalSubnets`, `networkType` and VM labels (`nodes.parameters.labels`, `additionalLabels`) for CAPI-managed Yandex nodes. [#671](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/671)
    Machines are not recreated on update. Machines created before the update keep the standard network type and no labels until the NodeGroup is rolled out (change a rolling InstanceClass field or set the `manual-rollout-id` annotation on the NodeGroup). Machines stuck in provisioning because of `additionalSubnets` become ready without recreation.
 - **[cloud-provider-zvirt]** VMs are now actually removed from zVirt on NodeGroup scale-down and cluster destroy [#205](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/205)
 - **[common]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` in kube-rbac-proxy. [#732](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/732)
 - **[common]** Fixed a `kube-apiserver` panic on JSON Patch requests whose `add`/`replace` of the whole document or `test` operation lacks the `value` field; such requests are now rejected with a validation error. [#324](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/324)
 - **[common]** kube-apiserver returns a single short validation error for an invalid `fieldManager` instead of an error for every invalid character. [#541](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/541)
 - **[control-plane-manager]** Keep `resource.k8s.io/v1beta1` enabled in `kube-apiserver` 1.34. [#415](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/415)
 - **[deckhouse-controller]** Deleting an Application now also deletes objects with the `werf.io/ownership: anyone` or `werf.io/resource-policy` annotation, such as Jobs deployed only on install, if the package lists their kinds in `orphanResources`. Only `helm.sh/resource-policy: keep` protects an object. [#347](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/347)
 - **[deckhouse]** Restore kubectl plugin lookup in d8 k and d8 kubectl [#588](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/588)
 - **[deckhouse]** Update runtime libraries and VEX exceptions for known vulnerabilities [#664](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/664)
 - **[dhctl]** Destroy and abort of a static cluster now fail instead of reporting success when a control-plane node was not cleaned because of an SSH error or cleanup timeout. [#333](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/333)
 - **[dhctl]** Preserve existing SSH keys when switching to the temporary converge user. [#380](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/380)
 - **[helm_lib]** Size the resources of a module whose `resourcesManagement` names a fractional quantity, instead of rendering a zero limit. [#66](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/66)
    Bumping `deckhouse_lib_helm` from `1.72.11` to `1.72.24` also drops the `drbd.linbit.com` taint
    tolerations from `helm_lib_tolerations`, so pods rendered through it no longer tolerate
    `lost-quorum`, `force-io-error` and `ignore-fail-over`, and aligns the VPA `maxAllowed` memory to
    the recommender's 64Mi quantum.
 - **[multitenancy-manager]** Disabling the module now removes the cluster-objects grant webhooks, which used to stay behind and block writes in project namespaces. [#345](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/345)
    The grant webhooks are now removed together with the module. On a cluster where `multitenancy-manager` is already disabled at the update, the `cluster-objects-grants-defaulting` and `cluster-objects-grants-validator` webhook configurations left by earlier versions stay, and writes to grantable resources in project namespaces (for example, removing a PVC finalizer) keep failing with `service "multitenancy-manager" not found`. Delete them by hand:
    `d8 k delete mutatingwebhookconfiguration cluster-objects-grants-defaulting`
    `d8 k delete validatingwebhookconfiguration cluster-objects-grants-validator`
    Nothing has to be done on clusters where the module is enabled.
 - **[multitenancy-manager]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`. [#733](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/733)
 - **[multitenancy-manager]** The Project conversions of the next release are registered in advance, so reads and writes of projects keep working while webhook-handler is updated in HA mode. [#214](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/214)
 - **[multitenancy-manager]** The controller takes its leader lease with any number of replicas, so a rollout of a single-replica installation no longer has two pods upgrading project Helm releases at once. [#215](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/215)
    In single-replica installations a new controller pod reconciles projects only after the previous pod has stopped and released the lease. If the previous pod is lost without a shutdown, for example together with its node, the new pod starts reconciling projects 60 to about 78 seconds after it starts.
 - **[multitenancy-manager]** The webhooks no longer leave zombie processes in webhook-handler until it runs out of pids. [#751](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/751)
 - **[node-manager]** A StaticInstance whose StaticMachine was deleted is returned to Pending and becomes available for bootstrapping again instead of staying out of the pool forever. [#457](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/457)
 - **[node-manager]** Fixed cluster-autoscaler not scaling CAPI node groups up from zero for pods requesting ephemeral storage, and putting the whole scale-up of a multi-zone node group into a single zone. [#576](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/576)
 - **[node-manager]** Fixed cluster-autoscaler stopping autoscaling of all node groups and restarting every two hours when an MCM node group has `minPerZone` equal to `maxPerZone`. [#464](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/464)
 - **[node-manager]** Fixed cluster-autoscaler using node group names without the cluster prefix after a Deckhouse restart, which stopped autoscaling and triggered the `D8ClusterAutoscalerTooManyErrors` alert. [#351](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/351)
 - **[node-manager]** Fixed the CAPI MachineHealthCheck replacing cloud nodes that take longer than 5 minutes to become Ready after registering, and limited remediation to when at most half of the CAPI machines are unhealthy. [#584](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/584)
    The cloud MachineHealthCheck is updated in existing clusters. A node in Ready=False is replaced after 10 minutes instead of 5 (Ready=Unknown still after 5), and remediation stops while more than 50% of the cluster's CAPI machines are unhealthy.
 - **[user-authn]** A DexAuthenticator with a name longer than 45 characters no longer logs its users out on every run of the hook. [#621](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/621)
 - **[user-authn]** A DexClient or a DexAuthenticator in a namespace whose name YAML reads as a number or a boolean no longer breaks the user-authn release. [#604](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/604)
 - **[user-authn]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net`. [#731](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/731)
 - **[user-authn]** Protected applications keep serving users during a short dex outage and during restarts. [#678](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/678)
 - **[user-authn]** Restrict the Job that issues the `basic-auth-proxy` certificate to the read-only front-proxy CA files and cover it with a SecurityPolicyException. [#523](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/523)
 - **[user-authn]** The `idTokenTTL` description and the `D8UserAuthnIDTokenTTLTooLong` alert say that a value of 6 hours or more is replaced with `5h59m` by the next settings version, and what lowering it does to DexAuthenticator sessions and issued ID tokens. [#218](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/218)
 - **[user-authn]** The webhooks no longer leave zombie processes in webhook-handler until it runs out of pids. [#751](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/751)
 - **[user-authz]** An AuthorizationRule in a namespace whose name YAML reads as a number, a boolean or null no longer breaks the module's bindings. [#553](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/553)
 - **[user-authz]** Fixed CVE-2026-78659, CVE-2026-78660, CVE-2026-78663, CVE-2026-78669 and CVE-2026-97032 in `golang.org/x/net` and CVE-2026-56851 in `golang.org/x/text`. [#731](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/731)
 - **[user-authz]** Managers who assign roles by range can again change DexProviders, UserOperations, Users and Groups of identities that hold a ClusterAuthorizationRule. [#619](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/619)
 - **[user-authz]** The RBACv2 roles d8:manage:* and d8:use:role:* keep the rights of the modules that have already switched to the capability labels of the next release while the upgrade to it is in progress. [#372](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/372)
 - **[user-authz]** The `D8UserAuthzLegacyRBACv2CustomRoleFound` alert lists every custom role that selects capabilities by the `rbac.deckhouse.io/kind: manage|use` label or by the aggregation label of a lineage the next role model does not collect, or that loses its RoleBindings in the namespaces of the modules, and the FAQ describes how to prepare such a role before the update without losing access. [#240](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/240)
 - **[user-authz]** The authorization webhook answers the requests in flight when it restarts instead of denying them. [#678](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/678)
 - **[user-authz]** The authorization webhook keeps applying ClusterAuthorizationRules when a `limitNamespaces` entry is not a valid regular expression; the entry is left out, and its rule covers fewer namespaces. [#245](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/245)
    Before, such an entry stopped the webhook from reloading the rules, so subjects of rules added later, and every subject after a restart of the webhook, were not limited to their namespaces. Subjects of a rule whose only entry is invalid now get no namespace through the entries of that rule.
 - **[user-authz]** The authorization webhook no longer restarts while the API server is unreachable, which could deadlock the control plane on bootstrap or an API server restart with multitenancy enabled. [#342](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/342)
 - **[user-authz]** The identity-assign check treats an identity with every permission across the cluster, such as a subject of `cluster-admin` bound by a ClusterRoleBinding, as SuperAdmin. [#217](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/217)
    Such identities can write ClusterAuthorizationRules, Users and Groups that carry `SuperAdmin` or `cluster-admin`. A ClusterAuthorizationRule gives this right only without `namespaceSelector` and `limitNamespaces` and, with multi-tenancy on, only with `allowAccessToSystemNamespaces: true`, even if it grants `cluster-admin`.
 - **[user-authz]** The webhooks no longer leave zombie processes in webhook-handler until it runs out of pids. [#751](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/751)
 - **[user-authz]** a `kube-apiserver` health probes no longer fail when `authz-webhook` is unavailable. [#412](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/412)
