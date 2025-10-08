---
title: "Current authorization model"
permalink: en/admin/configuration/access/authorization/rbac-current.html
description: "Configure current RBAC authorization model in Deckhouse Kubernetes Platform. User-authz module setup, ClusterRole management, and role-based access control configuration."
---

To use the current role model, the [`user-authz`](/modules/user-authz/) module must be enabled in the cluster.
This module creates a set of cluster roles (ClusterRole) suitable for most user and group access management tasks.

{% alert level="warning" %}
Starting from Deckhouse Kubernetes Platform v1.64, the module includes an experimental role-based access model.
The current role model will continue to function, but it will be deprecated in the future.

The current and experimental role-based access models are incompatible.
Automatic conversion of resources is not possible.
{% endalert %}

Key features of the current role model:

- Implements a role-based end-to-end authorization subsystem that extends the standard RBAC functionality.
- Access rights are managed using the custom resources [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) and [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule).
- Controls access to scaling tools (via the `allowScale` parameter in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-allowscale) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-allowscale) resource).
- Controls access to port forwarding (via the `portForwarding` parameter in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-portforwarding) or [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-portforwarding) resource).
- Manages the list of allowed namespaces using the labelSelector format (via the `namespaceSelector` parameter in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector) resource).

## High-level roles used in the current model

In addition to RBAC, the current role model implemented via the [`user-authz`](/modules/user-authz/) module
provides a convenient set of high-level roles:

| Role | Examples of allowed actions | Limitations |
|------|-----------------------------|-------------|
| **User** | View Pods, logs, and Deployments | Can't access secrets, ports, or containers |
| **PrivilegedUser** | Access containers (via `kubectl exec`), view secrets, remove Pods | Can't modify a Deployment or Service |
| **Editor** | Create and remove a Deployment, Service, or ConfigMap | Can't access a ReplicaSet or ClusterRoles |
| **Admin** | Remove a ReplicaSet and control RBAC in a namespace | Can't access cluster-wide resources |
| **ClusterEditor** | Create DaemonSet, ClusterRole, ClusterXXXMetric, KeepalivedInstance (only those required for applied tasks) | Can't remove MachineSets |
| **ClusterAdmin** | Fully access ClusterRoleBindings, Machines, and OpenstackInstanceClasses | Can elevate their permissions |
| **SuperAdmin** | Any actions are allowed (including `*` in RBAC), but with consideration to `limitNamespaces` | Limitations can be applied only using cluster policies |

{% alert level="warning" %}
The multitenancy mode (namespace-based authorization) is currently implemented as a temporary solution and **does not provide full isolation guarantees**.
{% endalert %}

If the `namespaceSelector` parameter is used in the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resource,
the `limitNamespaces` and `allowAccessToSystemNamespace` parameters are ignored.

If the webhook that implements the authorization system becomes unavailable for any reason,
the `allowAccessToSystemNamespaces`, `namespaceSelector`, and `limitNamespaces` options in custom resources will no longer work,
and users will have access to all namespaces. Once the webhook is available again, the options will resume functioning.

## Default access list for each high-level role

Verb shortcuts:
<!-- start user-authz roles placeholder -->
- read: `get`, `list`, `watch`
- read-write: `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
- write: `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - namespaces
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
    - serviceaccounts
    - services
    - storage.k8s.io/storageclasses
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
read-write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - networking.k8s.io/ingresses
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - serviceaccounts
    - services
write:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - apps/replicasets
    - extensions/replicasets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
read:
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - extensions/daemonsets
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
read-write:
    - deckhouse.io/clusterauthorizationrules
write:
    - limitranges
    - namespaces
    - networking.k8s.io/networkpolicies
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - resourcequotas
```
<!-- end user-authz roles placeholder -->

To get an additional list of access rules for a cluster module role
([existing user roles](granting.html#granting-permissions-using-authorizationrule-and-clusterauthorizationrule-current-role-model) and non-standard rules from other Deckhouse modules),
use the following command:

```bash
D8_ROLE_NAME=Editor
d8 k get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'\n'}{end}" | jq -s add
```

## AuthorizationRule example

Use [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) to set access rules for users within a specific namespace.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: AuthorizationRule
metadata:
  name: beeline
spec:
  accessLevel: Admin
  subjects:
  - kind: Admin
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
    namespace: d8-service-accounts
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # Available only with enableMultiTenancy mode turned on (in Enterprise Edition).
  allowAccessToSystemNamespaces: false
  # Available only with enableMultiTenancy mode turned on (in Enterprise Edition).
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

## Extending permissions for high-level roles

To add permissions to a specific [high-level role](#high-level-roles-used-in-the-current-model),
create a ClusterRole with the annotation `user-authz.deckhouse.io/access-level: <AccessLevel>`.

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
