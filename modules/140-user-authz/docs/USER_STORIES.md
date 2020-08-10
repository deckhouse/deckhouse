---
title: "User stories"
---

1. There is a CRD `ClusterAuthorizationRule`. Its resources are used to generate `ClusterRoleBindings` for users who mentioned in the field `subjects`. The set of `ClusterRoles` to bind is declared by fields:
    1. `accessLevel` — pre-defined `ClusterRole` set.
    2. `portForwarding` — pre-defined `ClusterRole` set.
    3. `additionalRoles` — user-defined `ClusterRole` set.
2. The configuration of fields `allowAccessToSystemNamespaces` and `limitNamespaces` affects the `user-authz-webhook` DaemonSet, which is authorization agent of apiserver,
3. When creating `ClusterRole` objects with annotation `user-authz.deckhouse.io/access-level`, the set of `ClusterRoles` for binding to the corresponding subject is extended.
