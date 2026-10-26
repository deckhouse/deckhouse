---
title: "Basic authorization model"
permalink: en/admin/configuration/access/authorization/rbac-current.html
description: "Configure the basic RBAC authorization model in Deckhouse Platform. User-authz module setup, ClusterRole management, and role-based access control configuration."
---

To use the basic role model, the [`user-authz`](/modules/user-authz/) module must be enabled in the cluster.
This module creates a set of cluster roles (ClusterRole) suitable for most user and group access management tasks.

{% alert level="warning" %}
The module implements two role-based models: the [granular](rbac-experimental.html) one (recommended) and the basic one (this page), built around the ClusterAuthorizationRule and AuthorizationRule resources — its support will be discontinued in future releases.

The models are not resource-compatible — automatic conversion is impossible — but they can be used at the same time: the permissions of both models are summed up.
{% endalert %}

Key features of the basic role model:

- Implements a role-based end-to-end authorization subsystem that extends the standard RBAC functionality.
- Access is configured with the custom resources [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) and [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule), on top of Kubernetes RBAC.
- Controls access to scaling tools (via the `allowScale` parameter in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-allowscale) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-allowscale) resource).
- Controls access to port forwarding (via the `portForwarding` parameter in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-portforwarding) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-portforwarding) resource).
- Manages the list of allowed namespaces using the labelSelector format (via the `namespaceSelector` parameter in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector) resource).
- Namespace restrictions (multitenancy mode, `namespaceSelector`, `limitNamespaces`) are available in the DP Ultimate/EE, Certified Lite, Certified Pro, BE, SE, SE+, and CSE editions.

## High-level roles used in the basic model

In addition to RBAC, the basic role model implemented via the [`user-authz`](/modules/user-authz/) module
provides a convenient set of high-level roles:

| Role | Examples of allowed actions | Limitations |
|------|-----------------------------|-------------|
| **User** | View Pods, logs, and Deployments | Can't access secrets, ports, or containers |
| **PrivilegedUser** | Access containers (via `kubectl exec`), view secrets, remove Pods | Can't modify a Deployment or Service |
| **Editor** | Create and remove a Deployment, Service, or ConfigMap | Can't access a ReplicaSet or ClusterRoles |
| **Admin** | Create and modify Pods, create ServiceAccount tokens (`serviceaccounts/token`), manage Role, RoleBinding, and AuthorizationRule objects, delete auxiliary objects (for example, ReplicaSet and the challenges and orders of the `acme.cert-manager.io` group) | Can't access cluster-wide resources |
| **ClusterEditor** | Manage a limited set of cluster-wide objects used in application tasks: DaemonSet, StorageClass, CustomResourceDefinition, GatewayClass, and the objects that modules add at this level (for example, KeepalivedInstance) | Can't manage namespaces, quotas, ClusterRole (read-only access), and ClusterRoleBinding |
| **ClusterAdmin** | Manage cluster-wide service objects: namespaces, quotas, NetworkPolicy, the proxy subresources of nodes, ClusterAuthorizationRule, ClusterRole, and ClusterRoleBinding | Can create ClusterAuthorizationRule objects only up to the `ClusterAdmin` level, and ClusterRoleBinding objects only to roles whose permissions it already holds |
| **SuperAdmin** | Any actions are allowed (including `*` in RBAC), but with consideration to `limitNamespaces` | Limitations can be applied only using cluster policies |

{% alert level="warning" %}
Multitenancy mode restricts access to namespaced resources only. Subjects with the `ClusterAdmin` or `SuperAdmin` access level can still create cluster-wide bindings. The authorization webhook does not check control plane components, kubelets, and the ServiceAccounts of `kube-system` and of the namespaces whose names start with `d8-`, so the namespace restrictions do not apply to them.
{% endalert %}

The `limitNamespaces` and `allowAccessToSystemNamespaces` parameters of the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resource are deprecated. To restrict a rule to certain namespaces, use [`namespaceSelector`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector). To grant access to all namespaces, including the system ones, use [`namespaceSelector.matchAny: true`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector-matchany). If `namespaceSelector` is set, the deprecated parameters are ignored.

In multitenancy mode, if the authorization webhook becomes unavailable, requests are **denied**, not allowed: the webhook runs before RBAC and is configured with `failurePolicy: Deny`, so a request it cannot answer does not reach RBAC. The exception is the identities listed in `matchConditions` of the AuthorizationConfiguration: control plane components, kubelets, the ServiceAccounts of the `kube-system` and `d8-*` namespaces, and anonymous kube-apiserver health probes (`/livez`, `/readyz`, `/healthz`) bypass the webhook, so the cluster keeps running and Deckhouse can restore the webhook.

Some platform components place their objects into user namespaces (for example, Dex authenticator pods) and label them `deckhouse.io/system-resource: "true"`. Of the basic model levels, only `SuperAdmin` allows modifying and deleting such objects and running `exec`, `attach`, and `port-forward` in such pods: the subjects of a rule with this level receive the `user-authz:super-admin` role and count as cluster administrators. For details, see [Admin level restrictions and superadmin rights](rbac-experimental.html#admin-level-restrictions-and-superadmin-rights).

## Default access list for each high-level role

{% alert level="info" %}
This list is a static snapshot. The actual set of rules also depends on which DP modules are enabled in the cluster.
{% endalert %}

Verb shortcuts:

- read: `get`, `list`, `watch`
- read-write: `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
- write: `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificaterequests
    - cert-manager.io/certificates
    - cert-manager.io/clusterissuers
    - cert-manager.io/issuers
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - configmaps
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/applications
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/deschedulers
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/nodeextensionrequests
    - deckhouse.io/nodegroups
    - deckhouse.io/nodeoperations
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
    - deckhouse.io/vcdaffinityrules
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - expansion.gatekeeper.sh/expansiontemplate
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - externaldata.gatekeeper.sh/providers
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gatewayclasses
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - multitenancy.deckhouse.io/availableclusterresources
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - network.deckhouse.io/servicewithhealthchecks
    - network.deckhouse.io/waypointinstances
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - nodes
    - persistentvolumeclaims
    - persistentvolumes
    - pods
    - pods/log
    - policy/poddisruptionbudgets
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - replicationcontrollers
    - resourcequotas
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - storage.k8s.io/storageclasses
    - syncset.gatekeeper.sh/syncsets
    - telemetry.istio.io/telemetries
    - templates.gatekeeper.sh/constrainttemplates
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `PrivilegedUser` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`):

```text
create:
    - pods/eviction
create,get:
    - pods/attach
    - pods/exec
delete,deletecollection:
    - pods
read:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Editor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`):

```text
get,patch:
    - pods/resize
write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - cert-manager.io/certificates
    - configmaps
    - deckhouse.io/dexauthenticators
    - deckhouse.io/dexclients
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - gateway.networking.k8s.io/backendtlspolicies
    - gateway.networking.k8s.io/gateways
    - gateway.networking.k8s.io/grpcroutes
    - gateway.networking.k8s.io/httproutes
    - gateway.networking.k8s.io/listenersets
    - gateway.networking.k8s.io/referencegrants
    - gateway.networking.k8s.io/tcproutes
    - gateway.networking.k8s.io/tlsroutes
    - gateway.networking.k8s.io/udproutes
    - network.deckhouse.io/servicewithhealthchecks
    - network.deckhouse.io/waypointinstances
    - networking.istio.io/destinationrules
    - networking.istio.io/gateways
    - networking.istio.io/serviceentries
    - networking.istio.io/sidecars
    - networking.istio.io/virtualservices
    - networking.istio.io/workloadentries
    - networking.istio.io/workloadgroups
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - secrets
    - security.istio.io/authorizationpolicies
    - security.istio.io/peerauthentications
    - security.istio.io/requestauthentications
    - serviceaccounts
    - services
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create:
    - serviceaccounts/token
create,patch,update:
    - pods
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - apps/replicasets
    - cert-manager.io/certificaterequests
    - extensions/replicasets
read-write:
    - deckhouse.io/authorizationrules
write:
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/issuers
    - deckhouse.io/applications
    - extensions.istio.io/wasmplugins
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,delete,patch:
    - deckhouse.io/nodeoperations
delete,deletecollection:
    - acme.cert-manager.io/challenges
    - acme.cert-manager.io/orders
    - cert-manager.io/certificaterequests
get,list:
    - templates.internal.deckhouse.io/nodeconfigtemplates
patch,update:
    - nodes
read:
    - deckhouse.io/containerdintegritypolicies
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - install.istio.io/istiooperators
    - multitenancy.deckhouse.io/grantableclusterresourcedefinitions
    - multitenancy.deckhouse.io/grantableclusterresourcereferences
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
read-write:
    - deckhouse.io/nodegroupconfigurations
    - deckhouse.io/staticinstances
    - multitenancy.deckhouse.io/clusterresourcegrantpolicies
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - autoscaling.k8s.io/verticalpodautoscalercheckpoints
    - cert-manager.io/clusterissuers
    - cert-manager.io/issuers
    - deckhouse.io/applications
    - deckhouse.io/hubblemonitoringconfigs
    - deckhouse.io/instances
    - deckhouse.io/keepalivedinstances
    - deckhouse.io/nodeextensionrequests
    - deckhouse.io/nodegroups
    - extensions.istio.io/wasmplugins
    - extensions/daemonsets
    - gateway.networking.k8s.io/gatewayclasses
    - network.deckhouse.io/egressgatewaypolicies
    - network.deckhouse.io/egressgateways
    - storage.k8s.io/storageclasses
    - telemetry.istio.io/telemetries
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
create:
    - authorization.deckhouse.io/bulksubjectaccessreviews
    - authorization.deckhouse.io/roleaccessreports
    - authorization.deckhouse.io/subjectaccessreports
    - deckhouse.io/dexauthenticators/allow-access-to-kubernetes
    - deckhouse.io/dexclients/allow-access-to-kubernetes
get,list,patch,update,watch:
    - control-plane.deckhouse.io/controlplanenodes
patch,update:
    - deckhouse.io/vcdaffinityrules
    - infrastructure.cluster.x-k8s.io/deckhouseclusters
    - infrastructure.cluster.x-k8s.io/deckhousemachines
    - infrastructure.cluster.x-k8s.io/deckhousemachinetemplates
    - infrastructure.cluster.x-k8s.io/dynamixclusters
    - infrastructure.cluster.x-k8s.io/dynamixmachines
    - infrastructure.cluster.x-k8s.io/dynamixmachinetemplates
    - infrastructure.cluster.x-k8s.io/huaweicloudclusters
    - infrastructure.cluster.x-k8s.io/huaweicloudmachines
    - infrastructure.cluster.x-k8s.io/huaweicloudmachinetemplates
    - infrastructure.cluster.x-k8s.io/vcdclusters
    - infrastructure.cluster.x-k8s.io/vcdclustertemplates
    - infrastructure.cluster.x-k8s.io/vcdmachines
    - infrastructure.cluster.x-k8s.io/vcdmachinetemplates
    - infrastructure.cluster.x-k8s.io/zvirtclusters
    - infrastructure.cluster.x-k8s.io/zvirtmachines
    - infrastructure.cluster.x-k8s.io/zvirtmachinetemplates
proxy:
    - nodes
read:
    - control-plane.deckhouse.io/controlplaneoperations
    - deckhouse.io/nodestaticpodrequests
    - nfd.k8s-sigs.io/nodefeaturegroups
    - nfd.k8s-sigs.io/nodefeaturerules
    - nfd.k8s-sigs.io/nodefeatures
read-write:
    - deckhouse.io/clusterauthorizationrules
    - deckhouse.io/deckhousereleases
    - deckhouse.io/dexproviderchecks
    - deckhouse.io/dexproviders
    - deckhouse.io/groups
    - deckhouse.io/moduleconfigs
    - deckhouse.io/moduledocumentations
    - deckhouse.io/modulepulloverrides
    - deckhouse.io/modulereleases
    - deckhouse.io/modules
    - deckhouse.io/modulesources
    - deckhouse.io/moduleupdatepolicies
    - deckhouse.io/nodeusers
    - deckhouse.io/packagerepositories
    - deckhouse.io/packagerepositoryoperations
    - deckhouse.io/sshcredentials
    - deckhouse.io/useraccounts
    - deckhouse.io/useroperations
    - deckhouse.io/users
    - nodes/configz
    - nodes/healthz
    - nodes/log
    - nodes/metrics
    - nodes/pods
    - nodes/proxy
    - nodes/stats
update:
    - namespaces/finalize
write:
    - cilium.io/ciliumclusterwidenetworkpolicies
    - cilium.io/ciliumnetworkpolicies
    - config.gatekeeper.sh/configs
    - connection.gatekeeper.sh/connections
    - constraints.gatekeeper.sh/*
    - deckhouse.io/applicationpackages
    - deckhouse.io/applicationpackageversions
    - deckhouse.io/awsinstanceclasses
    - deckhouse.io/azureinstanceclasses
    - deckhouse.io/clusterprojectrolebindings
    - deckhouse.io/containerdintegritypolicies
    - deckhouse.io/deschedulers
    - deckhouse.io/dvpinstanceclasses
    - deckhouse.io/dynamixinstanceclasses
    - deckhouse.io/gcpinstanceclasses
    - deckhouse.io/huaweicloudinstanceclasses
    - deckhouse.io/ingressistiocontrollers
    - deckhouse.io/istiofederations
    - deckhouse.io/istiomulticlusters
    - deckhouse.io/localpathprovisioners
    - deckhouse.io/openstackinstanceclasses
    - deckhouse.io/operationpolicies
    - deckhouse.io/projectnamespaces
    - deckhouse.io/projectrolebindings
    - deckhouse.io/projects
    - deckhouse.io/projecttemplates
    - deckhouse.io/securitypolicies
    - deckhouse.io/securitypolicyexceptions
    - deckhouse.io/vcdinstanceclasses
    - deckhouse.io/vsphereinstanceclasses
    - deckhouse.io/yandexinstanceclasses
    - deckhouse.io/zvirtinstanceclasses
    - expansion.gatekeeper.sh/expansiontemplate
    - externaldata.gatekeeper.sh/providers
    - install.istio.io/istiooperators
    - limitranges
    - mutations.gatekeeper.sh/assign
    - mutations.gatekeeper.sh/assignimage
    - mutations.gatekeeper.sh/assignmetadata
    - mutations.gatekeeper.sh/modifyset
    - namespaces
    - network.deckhouse.io/metalloadbalancerbgppeers
    - network.deckhouse.io/metalloadbalancerclasses
    - network.deckhouse.io/metalloadbalancerconfigurations
    - network.deckhouse.io/metalloadbalancerpools
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - resourcequotas
    - sailoperator.io/istiocnis
    - sailoperator.io/istiorevisions
    - sailoperator.io/istiorevisiontags
    - sailoperator.io/istios
    - sailoperator.io/ztunnels
    - status.gatekeeper.sh/configpodstatuses
    - status.gatekeeper.sh/connectionpodstatuses
    - status.gatekeeper.sh/constraintpodstatuses
    - status.gatekeeper.sh/constrainttemplatepodstatuses
    - status.gatekeeper.sh/expansiontemplatepodstatuses
    - status.gatekeeper.sh/mutatorpodstatuses
    - status.gatekeeper.sh/providerpodstatuses
    - syncset.gatekeeper.sh/syncsets
    - templates.gatekeeper.sh/constrainttemplates
```

To get an additional list of access rules for a cluster module role
([rules added by the administrator](#extending-permissions-for-high-level-roles) and non-default rules of other Deckhouse modules),
use the following command:

```bash
D8_ROLE_NAME=Editor
d8 k get clusterrole -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```

## AuthorizationRule example

Use [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) to set access rules for users within a specific namespace.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: test-rule
spec:
  accessLevel: Admin
  subjects:
  - kind: User
    name: admin@example.com
```

## ClusterAuthorizationRule example

Use [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) to set access rules for users either cluster-wide or for specific namespaces.

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test-rule
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: ci
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # This option is only available if the enableMultiTenancy parameter is set.
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: stage
        operator: In
        values:
        - test
        - review
      matchLabels:
        team: frontend
```

## How rules are applied

ClusterAuthorizationRule and AuthorizationRule objects grant access through ordinary ClusterRoleBinding and RoleBinding objects. The `user-authz-controller` component creates these bindings for every rule, keeps them in sync with the rule, and writes the result to the rule's `status`. To view the state of the rules, run:

```shell
d8 k get clusterauthorizationrules
d8 k get authorizationrules -A
```

The `READY` column shows whether the bindings match the rule, and the `BINDINGS` column shows how many bindings the rule has. A rule with an unsupported `accessLevel` gets `Ready=False` with the `InvalidSpec` reason. The other reasons and the controller metrics are described in the [`user-authz` module FAQ](/modules/user-authz/faq.html#how-do-i-check-that-the-controller-keeps-the-bindings-in-sync).

If several ClusterAuthorizationRule objects match the same subject, the allowed namespaces are combined, and the effective `accessLevel` is the most powerful among the matching rules. For an example, see the [`user-authz` module FAQ](/modules/user-authz/faq.html#what-if-there-are-two-clusterauthorizationrules-matching-a-single-user).

Creating, updating, and deleting a ClusterAuthorizationRule counts as assigning roles: the request is admitted only if the requester can assign the roles of the rule. An update is checked against both the previous and the new grants, so lowering `SuperAdmin` to `User` still requires being able to assign `SuperAdmin`. Who can assign which levels is described in [Who can assign roles](rbac-experimental.html#who-can-assign-roles).

A `User` or `Group` subject is matched by name against the identity in the token, and a local identity is indistinguishable from one asserted by an external authentication provider. Therefore, you cannot create a [User](/modules/user-authn/cr.html#user) whose `spec.email` or a [Group](/modules/user-authn/cr.html#group) whose `spec.name` matches a subject of an existing ClusterAuthorizationRule or AuthorizationRule. The restriction is checked when a User or Group is created and when its `spec.email` or `spec.name` is changed. If the match is intentional, for example, the rule was written in advance, set the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation on the User or Group. The annotation only acknowledges the name collision: it does not let the requester assign roles they cannot assign otherwise.

## Extending permissions for high-level roles

To add permissions to a specific [high-level role](#high-level-roles-used-in-the-basic-model),
create a ClusterRole with the annotation `user-authz.deckhouse.io/access-level: <AccessLevel>`.

The annotation accepts `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, and `ClusterAdmin`. Other values, including `SuperAdmin`, are ignored. In the name of the aggregated role, the level is written in kebab case, for example, `user-authz:cluster-editor:custom`. The rules of a level are also included in the aggregated roles of every higher level.

Example:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Editor
  name: user-editor
rules:
- apiGroups:
  - kuma.io
  resources:
  - trafficroutes
  - trafficroutes/finalizers
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - flagger.app
  resources:
  - canaries
  - canaries/status
  - metrictemplates
  - metrictemplates/status
  - alertproviders
  - alertproviders/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
```
