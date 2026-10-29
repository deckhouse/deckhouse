---
title: "Granting permissions to users and service accounts"
permalink: en/admin/configuration/access/authorization/granting.html
description: "Grant RBAC permissions to users and service accounts in Deckhouse Platform. Role and ClusterRole binding configuration for secure access control."
---

To grant permissions in Deckhouse Platform (DP), you need to define a `subjects` block. Its format and contents depend on the resource type:

- In [AuthorizationRule and ClusterAuthorizationRule](#granting-permissions-using-authorizationrule-and-clusterauthorizationrule-basic-role-model) resources (basic [role model](../../../../glossary.html#rbac)), as well as [ProjectRoleBinding and ClusterProjectRoleBinding](#granting-permissions-using-clusterrolebinding-and-rolebinding-granular-role-model) of the `multitenancy-manager` module (granular role model, [project](../../../../glossary.html#project) scope) — use the [`subjects`](/modules/user-authz/cr.html#authorizationrule-v1alpha1-spec-subjects) block shown below.
- In standard Kubernetes [RoleBinding and ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resources (granular role model, namespace/[subsystem](../../../../glossary.html#subsystem)/whole-platform scope), the format differs: for `kind: User` and `kind: Group`, a `subjects` item additionally requires the `apiGroup: rbac.authorization.k8s.io` field (not needed for `kind: ServiceAccount`). For details, see the examples in [Granting permissions using ClusterRoleBinding and RoleBinding](#granting-permissions-using-clusterrolebinding-and-rolebinding-granular-role-model) below.

For users, it should be specified in the following format:

```yaml
subjects:
- kind: User
  name: <user email>
```

{% alert level="warning" %}
If you are using the [`user-authn`](/modules/user-authn/) module and [static users](../../../../glossary.html#static-user), make sure to specify the user’s email in `subjects`,
not the name of the [User](/modules/user-authn/cr.html#user) resource.
{% endalert %}

or

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

AuthorizationRule and ClusterAuthorizationRule objects can also be managed without writing YAML manifests, with the [`d8 iam access`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-access) command: `d8 iam access grant` creates them, and `d8 iam access revoke` deletes them. For example, to grant the `Admin` access level to a user in the `dev` namespace, run:

```shell
d8 iam access grant user anton --access-level Admin -n dev
```

For more examples, see the [`user-authz` module documentation](/modules/user-authz/usage.html#managing-permissions-via-cli).

### Granting permissions to a user within a single namespace

To grant a user permissions within a single namespace, use the [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) resource.
It is applied within a single namespace.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
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

To grant a user permissions across all namespaces (for example, to assign administrator permissions),
use the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) resource.
It is applied cluster-wide.

{% alert level="info" %}
In multitenancy mode, a ClusterAuthorizationRule without [`namespaceSelector`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector) does not grant access to [system namespaces](../../../../glossary.html#system-namespace): they require [`namespaceSelector.matchAny: true`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector-matchany) or the deprecated [`allowAccessToSystemNamespaces: true`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-allowaccesstosystemnamespaces).
{% endalert %}

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
  # Available only with the enableMultiTenancy parameter of the user-authz module
  # turned on (BE, SE, SE+, EE, and CSE editions).
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
  accessLevel: Admin
  portForwarding: true
```

{% alert level="warning" %}
The `namespaceSelector` and `limitNamespaces` parameters only restrict access to objects in namespaces. A subject with the `ClusterAdmin` or `SuperAdmin` access level keeps its cluster-wide permissions, including creating ClusterRoleBinding and ClusterAuthorizationRule objects, so these parameters do not limit the permissions it can grant across the whole cluster. Grant these levels only to those who may be given access to the whole cluster.
{% endalert %}

## Granting permissions using ClusterRoleBinding and RoleBinding (granular role model)

When using the granular [role model](../../../../glossary.html#rbac) in DP,
you can grant permissions to users via the [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) and [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) resources — or, to grant access across all namespaces of a [project](../../../../glossary.html#project) at once, the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) resources of the `multitenancy-manager` module.

Roles of the granular model operate in one of four scopes — namespace, project, [subsystem](../../../../glossary.html#subsystem), or the whole platform — each with its own role name format and access levels. For details, see [Role scopes](rbac-experimental.html#role-scopes) in the granular role model documentation.

{% alert level="info" %}
Access can also be granted without writing YAML manifests, with the access grant wizard in the [Deckhouse web UI](../../../../user/web/ui.html). The wizard walks you through the steps (who gets access → where → at which level), picks the binding kind itself (RoleBinding, ClusterRoleBinding, ProjectRoleBinding, or ClusterProjectRoleBinding), and lets you assemble a custom role from ready-made capabilities.
{% endalert %}

### Assigning cluster administrator permissions (granular role model)

To assign cluster administrator permissions, use the [system role](rbac-experimental.html#system-and-subsystem-roles) `d8:system:manager` in a [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resource.

Example of assigning cluster administrator permissions to the user `jane.doe@example.com`:

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
The user gets access to cluster-wide resources, to the configuration of DP modules, and to the namespaces of the enabled DP modules. The user gets no access to the namespaces of user applications.

The user will have the following permissions:

- View, modify, delete, and create cluster-wide Kubernetes resources and the cluster-wide resources of DP modules.
- Modify module configurations (view, edit, delete, and create ModuleConfig resources).
- View objects, logs, metrics, and events in the namespaces of DP modules, without access to Secrets.

The `d8:system:manager` role does not allow writing to the namespaces of DP modules, reading Secrets there, or running `kubectl exec`, `kubectl attach`, and `kubectl port-forward` on their Pods. These operations require the `d8:system:superadmin` role. For details, see [Access of system and subsystem roles to the namespaces of the modules](rbac-experimental.html#access-of-system-and-subsystem-roles-to-the-namespaces-of-the-modules).

{% endofftopic %}

### Assigning networking administrator permissions (granular role model)

To assign networking administrator permissions for managing the cluster’s network subsystem,
use the [subsystem role](rbac-experimental.html#system-and-subsystem-roles) `d8:subsystem:network:manager` in a [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resource.

Example of assigning networking administrator permissions to the user `jane.doe@example.com`:

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
  name: d8:subsystem:network:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="Permissions the user will obtain" %}
The user’s permissions will be limited to the following DP module namespaces from the `network` subsystem
(the actual list depends on the modules enabled in the cluster):

- `d8-cni-cilium`
- `d8-cni-flannel`
- `d8-cni-simple-bridge`
- `d8-ingress-nginx`
- `d8-istio`
- `d8-keepalived`
- `d8-metallb`
- `d8-monitoring`
- `d8-network-gateway`
- `d8-openvpn`
- `d8-service-with-healthchecks`
- `d8-static-routing-manager`
- `d8-system`
- `kube-system`

The user will have the following permissions:

- View objects, logs, metrics, and events in the module namespaces from the `network` subsystem, without access to Secrets.

- View, modify, create, and delete the following resources of the modules from the `network` subsystem:

  - EgressGateway
  - EgressGatewayPolicy
  - FlowSchema
  - IngressClass
  - IngressIstioController
  - IngressNginxController
  - IPRuleSet
  - IstioFederation
  - IstioMulticluster
  - KeepalivedInstance
  - RoutingTable

- Modify module configurations (view, modify, create, and delete ModuleConfig resources) in the `network` subsystem.

  Modules the user will be able to manage:

  - `cilium-hubble`
  - `cni-cilium`
  - `cni-flannel`
  - `cni-simple-bridge`
  - `ingress-nginx`
  - `istio`
  - `keepalived`
  - `kube-dns`
  - `kube-proxy`
  - `metallb`
  - `monitoring-ping`
  - `network-gateway`
  - `network-policy-engine`
  - `node-local-dns`
  - `openvpn`
  - `sdn`
  - `service-with-healthchecks`
  - `static-routing-manager`

The `d8:subsystem:network:manager` role does not allow writing to the module namespaces, reading Secrets there, or running `kubectl exec`, `kubectl attach`, and `kubectl port-forward` on their Pods. These operations require the `d8:subsystem:network:superadmin` role. For details, see [Access of system and subsystem roles to the namespaces of the modules](rbac-experimental.html#access-of-system-and-subsystem-roles-to-the-namespaces-of-the-modules).

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
