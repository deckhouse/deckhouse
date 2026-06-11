---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

<div style="height: 0;" id="how-do-i-limit-user-rights-to-specific-namespaces-obsolete-role-based-model"></div>

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces in the experimental role-based model, use `RoleBinding` with the [namespace role](./#namespace-roles) that has the appropriate level of access. [Example...](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

In the current role-based model, use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [`ClusterAuthorizationRule`](../../modules/user-authz/cr.html#clusterauthorizationrule) CR.

## What if there are two ClusterAuthorizationRules matching to a single user?

In the example, the user `jane.doe@example.com` is in the `administrators` group. There are two cluster authorization rules:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: jane
spec:
  subjects:
    - kind: User
      name: jane.doe@example.com
  accessLevel: User
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: Group
    name: administrators
  accessLevel: ClusterAdmin
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: env
        operator: In
        values:
        - prod
        - stage
```

1. `jane.doe@example.com` has the right to get and list any objects in the namespaces labeled `env=review`
2. `Administrators` can get, edit, list, and delete objects on the cluster level and in the namespaces labeled `env=prod` and `env=stage`.

Because `Jane Doe` matches two rules, some calculations will be made:

* `Jane Doe` will have the most powerful accessLevel across all matching rules — `ClusterAdmin`.
* The `namespaceSelector` options will be combined, so that Jane will have access to all the namespaces labeled with `env` label of the following values: `review`, `stage`, or `prod`.

{% alert level="warning" %}
If there is a rule without the `namespaceSelector` option and `limitNamespaces` deprecated option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.
{% endalert %}

## How do I extend a role or create a new one?

[The experimental role model](./#experimental-role-based-model) is based on the aggregation principle; it compiles smaller roles into larger ones,
thus providing easy ways to enhance the model with custom roles.

### Creating a new role subsystem

Suppose that the current subsystems do not fit the role distribution in the company. You need to create a new [subsystem](./#subsystems-of-the-role-based-model)
that includes roles from the `deckhouse` subsystem, the `kubernetes` subsystem and the user-authn module.

To meet this need, create the following role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:custom:subsystem:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: custom-role
    rbac.deckhouse.io/scope: subsystem
    rbac.deckhouse.io/subsystem: mycustom
    rbac.deckhouse.io/aggregate-to-system-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/scope: system
        module: user-authn
rules: []
```

The labels for the new role listed at the top suggest that:

- The hook will use this namespace role when creating `RoleBinding` in the module namespaces:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- The role is a custom role (custom roles never define their own rules, they only aggregate capabilities):

  ```yaml
  rbac.deckhouse.io/kind: custom-role
  ```

  > Note that this label is mandatory.

- The role is a subsystem one, and it shall be handled accordingly:

  ```yaml
  rbac.deckhouse.io/scope: subsystem
  ```

- There is a subsystem for which the role is responsible:

  ```yaml
  rbac.deckhouse.io/subsystem: mycustom
  ```

- The `d8:system:manager` role can aggregate this role:

  ```yaml
  rbac.deckhouse.io/aggregate-to-system-as: manager
  ```

Then there are selectors that implement aggregation:

- This one aggregates the manager role from the `deckhouse` subsystem:

  ```yaml
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- This one aggregates all the system-scope capabilities defined for the user-authn module:

  ```yaml
   rbac.deckhouse.io/scope: system
   module: user-authn
  ```

This way, your role will combine permissions of the `deckhouse` subsystem, `kubernetes` subsystem, and the user-authn module.

Notes:

* Custom roles and capabilities must be named with the `d8:custom:` prefix (the rest of the `d8:` prefix space is reserved for Deckhouse built-in objects).
* Namespace roles (`RoleBinding` with `d8:namespace:<level>`) will be created in the namespaces of the aggregated subsystems' modules, the level is specified by the `rbac.deckhouse.io/use-role` label.

### Extending the custom role

Suppose a new cluster CRD object, MySuperResource, has been created in the cluster (a manage role example), and you need to extend the custom role from the example above to include the permissions to interact with this resource.

First, you have to add a new selector to the role:

```yaml
rbac.deckhouse.io/aggregate-to-mycustom-as: manager
```

This selector would enable capabilities to be aggregated to a new subsystem by specifying this label. After adding the new selector, the role will look as follows:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: d8:custom:subsystem:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: admin
     rbac.deckhouse.io/kind: custom-role
     rbac.deckhouse.io/scope: subsystem
     rbac.deckhouse.io/subsystem: mycustom
     rbac.deckhouse.io/aggregate-to-system-as: manager
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
     - matchLabels:
         rbac.deckhouse.io/scope: system
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/aggregate-to-mycustom-as: manager
 rules: []
 ```

 Next, you need to create a new capability and define permissions for the new resource, e. g., the read-only permission:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-mycustom-as: manager
     rbac.deckhouse.io/kind: custom-capability
   name: d8:custom:capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

The capability will update the subsystem role to include its rights, so that the role bearer will be able to view the new object.

Notes:

* Custom capabilities must be named with the `d8:custom:` prefix; the rest of the name is not restricted, but we recommend following the same pattern for the sake of readability.

### Extending the existing subsystem roles

To extend an existing role, follow the procedure outlined in the section above. Be sure to change the labels and the role name!

For example, here's how you can extend the manager role from the `deckhouse` (`d8:subsystem:deckhouse:manager`) subsystem:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: custom-capability
  name: d8:custom:capability:mycustom:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

This way, the new capability will extend the `d8:subsystem:deckhouse:manager` role.

### Extending subsystem roles and adding a new namespace

If you need to create a new namespace (to create a namespace role binding in it by the hook), you only need to add one label:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

This label instructs the hook to create a `RoleBinding` with the namespace role in this namespace:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: custom-capability
     rbac.deckhouse.io/namespace: namespace
   name: d8:custom:capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

The hook monitors `ClusterRoleBinding`, and when creating a bindings, it loops through all the system and subsystem roles to find all the aggregated capabilities by checking the aggregation rule. It then fetches the namespace from the `rbac.deckhouse.io/namespace` label and creates a `RoleBinding` with the namespace role in that namespace.

### Extending the existing namespace roles

If the resource belongs to a namespace, you need to extend the namespace role instead of the system/subsystem role. The only difference is the labels and the name:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-namespace-as: user
     rbac.deckhouse.io/kind: custom-capability
   name: d8:custom:namespace-capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

This capability will be added to the `d8:namespace:user` role.
