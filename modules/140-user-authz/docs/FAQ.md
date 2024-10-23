---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

## How do I limit user rights to specific namespaces?

To limit a user's rights to specific namespaces, use `RoleBinding` with the [use role](./#use-roles) that has the appropriate level of access. [Example...](usage.html#example-of-assigning-administrative-rights-to-a-user-within-a-namespace).

### How do I limit user rights to specific namespaces (obsolete role-based model)?

{% alert level="warning" %}
The example uses the [obsolete role-based model](./#the-obsolete-role-based-model).
{% endalert %}

Use the `namespaceSelector` or `limitNamespaces` (deprecated) parameters in the [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule) CR.

## What if there are two ClusterAuthorizationRules matching to a single user?

Imagine that the user `jane.doe@example.com` is in the `administrators` group. There are two cluster authorization rules:

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
* She will have the most powerful accessLevel across all matching rules — `ClusterAdmin`.
* The `namespaceSelector` options will be combined, so that Jane will have access to all the namespaces labeled with `env` label of the following values: `review`, `stage`, or `prod`.

> **Note!** If there is a rule without the `namespaceSelector` option and `limitNamespaces` deprecated option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.

## How to extend roles or create a new one ?

The new model facilities the principles of aggregation, it combines smaller roles into larger ones, so it can be easily extended.

1. Creating a new scope role.

   Suppose that the current scopes roles do not match the company's role model and a new role, 
   that contains the user-authn module and the deckhouse and kubernetes scopes, needs to be created.

   To solve it, the following role can be created:

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: custom:manage:custom:admin
      labels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/level: scope
        rbac.deckhouse.io/scope: custom
        rbac.deckhouse.io/aggregate-to-all-as: admin
    aggregationRule:
      clusterRoleSelectors:
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-kubernetes-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            module: user-authn
    rules: []
    ```

   Let`s look at the role. firstly, we need to specify labels:
    - This label needs to handle this role as a manage role.
    ```yaml
    rbac.deckhouse.io/kind: manage
    ```

    - This label shows that the role is a scope level role.
    ```yaml
    rbac.deckhouse.io/level: scope
    ```

    - This label specifies the scope of the role.
    ```yaml
    rbac.deckhouse.io/scope: custom
    ```

    - This label lets the manage:all role to aggregate this role.
    ```yaml
    rbac.deckhouse.io/aggregate-to-all-as: admin
    ```
   The next part is selectors and they implement aggregation:
    - This selector aggregates role admin from the deckhouse scope.
    ```yaml
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
    ```
    - This selector aggregates all roles from the user-authn module.
   ```yaml
    rbac.deckhouse.io/kind: manage
    module: user-authn
   ```
   
   So, our role gets permissions in the deckhouse and the kubernetes scopes and in the user-authn module.

   * Names of the scope roles must be this way, because the last word after ':' defines the use role witch will be created in namespaces
   * No restrictions on names of capabilities, but for readability keep them this way.
   * Use roles will be created in the scopes namespaces and in the module`s namespace.

2. Extend a custom role.

   Suppose that a new CRD cluster object has been created (this is an example for a manage role) - MySuperResource,
   and we need to extend our role (from the example above) to get new permissions to interact with the object.

   First, we need to add a new selector to our scope role:
    ```yaml
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/aggregate-to-custom-as: admin
    ```
   This selector allows this role to aggregate roles(capability) to our new scope using this label.
   After adding this selector, the role looks is:
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: custom:manage:custom:admin
      labels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/level: scope
        rbac.deckhouse.io/scope: custom
        rbac.deckhouse.io/aggregate-to-all-as: admin
    aggregationRule:
      clusterRoleSelectors:
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-kubernetes-as: admin
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            module: user-authn
        - matchLabels:
            rbac.deckhouse.io/kind: manage
            rbac.deckhouse.io/aggregate-to-custom-as: admin
    rules: []
    ```

   Then we need to create a new role(capability) and specify permissions, for example let`s add read, list and watch verbs:
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-custom-as: admin
        rbac.deckhouse.io/kind: manage
      name: custom:manage:capability:custom:superresource:view
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

   Aggregation is defined by this label.
    ```yaml
    rbac.deckhouse.io/aggregate-to-custom-as: admin
    ```

   And this role complements our scope role.

    * Names of the scope roles must be this way, because the last word after ':' defines the use role witch will be created in namespaces
    * No restrictions on names of capabilities, but for readability keep them this way.

3. Extend exising manage scope roles.

   If we want to extend an existing scope role, we can use the way above, but change the aggregation label and name.
   Example to extend the deckhouse scope role(```d8:manage:deckhouse:admin```):
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        rbac.deckhouse.io/kind: manage
      name: custom:manage:capability:custom:superresource:view
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
   
    And this role complements the ```d8:manage:deckhouse``` role.

4. Extend existing manage scope roles with a new namespace.

   If we want to add a namespace to a scope role(to create the use role in this namespace by the hook), we can add just one label:
   ```yaml
   "rbac.deckhouse.io/namespace": namespace
   ```
   This label informs the hook to create the use role in this namespace.
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-deckhouse-as: admin
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/namespace: namespace
      name: custom:manage:capability:custom:superresource:view
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
   
   How it works ? The hook watches for ClusterRoleBinding, when a binding is created, 
   it iterates over the all manage roles(with the label ```rbac.deckhouse.io/kind: manage```) to find all aggregated roles by checking the aggregation rule,
   then it takes namespace from the label ```rbac.deckhouse.io/namespace```, and creates a use role in this namespace, 
   the use role is specified by last world after ':' in the scope role(in our case - admin).

5. Extend existing use roles.

   If our resource is namespaced, we need to extend the use role instead of the manage role. The difference is labels and name:
    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      labels:
        rbac.deckhouse.io/aggregate-to-role: user
        rbac.deckhouse.io/kind: use
      name: custom:use:capability:custom:superresource:view
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

   And this role complements the ```d8:use:role:user``` role.
