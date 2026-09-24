---
title: "Granting permissions to users and service accounts"
permalink: en/admin/configuration/access/authorization/granting.html
description: "Grant RBAC permissions to users and service accounts in Deckhouse Platform. Role and ClusterRole binding configuration for secure access control."
---

To grant permissions in Deckhouse Platform (DP), you need to define a `subjects` block. Its format depends on the resource type:

- In [AuthorizationRule and ClusterAuthorizationRule](#granting-permissions-using-authorizationrule-and-clusterauthorizationrule-basic-role-model) resources (basic role model), as well as [ProjectRoleBinding and ClusterProjectRoleBinding](#granting-permissions-using-clusterrolebinding-and-rolebinding-granular-role-model) of the `multitenancy-manager` module (granular role model, project scope) — use the [`subjects`](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-subjects) block shown below.
- In standard Kubernetes [RoleBinding and ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resources (granular role model, namespace/subsystem/whole-platform scope), the format differs: `kind: User` and `kind: Group` additionally require the `apiGroup: rbac.authorization.k8s.io` field (not needed for `kind: ServiceAccount`). For details, see the examples in [Granting permissions using ClusterRoleBinding and RoleBinding](#granting-permissions-using-clusterrolebinding-and-rolebinding-granular-role-model) below.

For users, it should be specified in the following format:

```yaml
subjects:
- kind: User
  name: <user email>
```

{% alert level="warning" %}
If you are using the [`user-authn`](/modules/user-authn/) module and static users, make sure to specify the user’s email in `subjects`,
not the name of the [User](/modules/user-authn/cr.html#user) resource.
{% endalert %}

Alternatively, you can grant permissions by group:

```yaml
subjects:
- kind: Group
  name: <name of the group the user belongs to>
```

For a service account, the `subjects` block should be specified as follows:

```yaml
subjects:
- kind: ServiceAccount
  name: <service account name>
  namespace: <namespace where the service account is created>
```

## Granting permissions using AuthorizationRule and ClusterAuthorizationRule (basic role model)

When using the basic role model in DP,
you can grant permissions to users via the [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) and [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resources.

{% alert level="warning" %}
Support for the basic role model will be discontinued in future releases. For new roles, use the [granular role model](#granting-permissions-using-clusterrolebinding-and-rolebinding-granular-role-model) below — it can be used at the same time as the basic model, since the permissions they grant are summed up.
{% endalert %}

### Granting permissions to a user within a single namespace

To grant a user permissions within a single namespace, use the [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) resource.
It is applied within a single namespace.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: AuthorizationRule
metadata:
  name: dev-access
  namespace: dev-namespace
spec:
  subjects:
  - kind: User
    name: dev-user@example.com
  accessLevel: Admin
  portForwarding: true
```

### Granting permissions to a user in all namespaces

To grant a user permissions across all namespaces, including system ones (for example, to assign administrator permissions),
use the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resource.
It is applied cluster-wide.

If needed, you can restrict the scope of permissions granted via [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) to one or several namespaces.
To do this, you can set the corresponding restrictions in the manifest.
However, if possible, it is recommended that you use the [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) resource for this purpose.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin-access
spec:
  subjects:
  - kind: User
    name: dev-user@example.com
  # Available only with enableMultiTenancy mode turned on
  # in the user-authz module (Enterprise Edition).
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
  accessLevel: SuperAdmin
  portForwarding: true
```

## Granting permissions using ClusterRoleBinding and RoleBinding (granular role model)

When using the granular role model in DP,
you can grant permissions to users via the [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) and [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resources — or, to grant access across all namespaces of a project at once, the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) resources of the `multitenancy-manager` module.

Roles of the granular model operate in one of four scopes — namespace, project, subsystem, or the whole platform — each with its own role name format and access levels. For details, see [Role scopes](rbac-experimental.html#role-scopes) in the granular role model documentation.

### Assigning cluster administrator permissions (granular role model)

To assign cluster administrator permissions, use the [system role](rbac-experimental.html#system-and-subsystem-roles) `d8:system:manager` in a [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resource.

Example of assigning cluster administrator permissions to the user `jane`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Permissions the user will obtain" %}
The user’s permissions will be limited to namespaces starting with `d8-` or `kube-`.

The user will have the following permissions:

- View, modify, delete, and create Kubernetes resources and DP module resources.
- Modify module configurations (view, edit, delete, and create ModuleConfig resources).
- Run the following commands on pods and services:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`

{% endofftopic %}

### Assigning networking administrator permissions (granular role model)

To assign networking administrator permissions for managing the cluster’s networking subsystem,
use the [subsystem role](rbac-experimental.html#system-and-subsystem-roles) `d8:subsystem:networking:manager` in a [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resource.

Example of assigning networking administrator permissions to the user `jane`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:subsystem:networking:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Permissions the user will obtain" %}
The user’s permissions will be limited to the following DP module namespaces from the networking subsystem
(the actual list depends on the modules enabled in the cluster):

- `d8-cni-cilium`
- `d8-cni-flannel`
- `d8-cni-simple-bridge`
- `d8-ingress-nginx`
- `d8-istio`
- `d8-metallb`
- `d8-network-gateway`
- `d8-openvpn`
- `d8-static-routing-manager`
- `d8-system`
- `kube-system`

The user will have the following permissions:

- View, modify, create, and delete *standard* Kubernetes resources in module namespaces from the `networking` subsystem.

  Example resources the user will be able to manage (not a complete list):

  - Certificate
  - CertificateRequest
  - ConfigMap
  - ControllerRevision
  - CronJob
  - DaemonSet
  - Deployment
  - Event
  - HorizontalPodAutoscaler
  - Ingress
  - Issuer
  - Job
  - Lease
  - LimitRange
  - NetworkPolicy
  - PersistentVolumeClaim
  - Pod
  - PodDisruptionBudget
  - ReplicaSet
  - ReplicationController
  - ResourceQuota
  - Role
  - RoleBinding
  - Secret
  - Service
  - ServiceAccount
  - StatefulSet
  - VerticalPodAutoscaler
  - VolumeSnapshot

- View, modify, create, and delete resources in DP module namespaces from the `networking` subsystem.

  Resources the user will be able to manage:

  - EgressGateway
  - EgressGatewayPolicy
  - FlowSchema
  - IngressClass
  - IngressIstioController
  - IngressNginxController
  - IPRuleSet
  - IstioFederation
  - IstioMulticluster
  - RoutingTable

- Modify module configurations (view, modify, create, and delete ModuleConfig resources) in the `networking` subsystem.

  Modules the user will be able to manage:

  - `cilium-hubble`
  - `cni-cilium`
  - `cni-flannel`
  - `cni-simple-bridge`
  - `flow-schema`
  - `ingress-nginx`
  - `istio`
  - `kube-dns`
  - `kube-proxy`
  - `metallb`
  - `network-gateway`
  - `network-policy-engine`
  - `node-local-dns`
  - `openvpn`
  - `static-routing-manager`

- Run the following commands on Pods and Services in the module namespaces within the `networking` subsystem:

  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`

{% endofftopic %}

### Granting administrator permissions to a user within a namespace (granular role model)

To assign or restrict user permissions to specific namespaces,
apply a [namespace role](rbac-experimental.html#namespace-roles) with the corresponding access level in a [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resource.

For example, to allow a user to manage application resources in a namespace (without giving them access to DP module configurations), use the `d8:namespace:admin` role in a [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resource for the corresponding namespace. To grant the same access across all namespaces of a project at once, use a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) with the equivalent `d8:project:admin` role instead.

Example of granting application developer `app-developer` permissions within the `myapp` namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myapp-developer
  namespace: myapp
subjects:
- kind: User
  name: app-developer@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:namespace:admin
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Permissions the user will obtain" %}
The user’s permissions will be limited to the following within the `myapp` namespace:

- View, modify, create, and delete Kubernetes resources.
  Example resources:

  - Certificate
  - CertificateRequest
  - ConfigMap
  - ControllerRevision
  - CronJob
  - DaemonSet
  - Deployment
  - Event
  - HorizontalPodAutoscaler
  - Ingress
  - Issuer
  - Job
  - Lease
  - LimitRange
  - NetworkPolicy
  - PersistentVolumeClaim
  - Pod
  - PodDisruptionBudget
  - ReplicaSet
  - ReplicationController
  - ResourceQuota
  - Role
  - RoleBinding
  - Secret
  - Service
  - ServiceAccount
  - StatefulSet
  - VerticalPodAutoscaler
  - VolumeSnapshot

- View, modify, create, and delete the following DP module resources:

  - DexAuthenticator
  - DexClient
  - PodLoggingConfig

- Run the following commands on Pods and Services:

  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`

{% endofftopic %}
