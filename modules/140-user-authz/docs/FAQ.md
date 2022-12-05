---
title: "The user-authz module: FAQ"
---

## How do I create a user?

[Creating a user](usage.html#creating-a-user).

## How do I limit user rights to specific namespaces?

Use the `limitNamespaces` parameter in the [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule) CR.

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
  limitNamespaces:
  - review-.*
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
  limitNamespaces:
  - prod
  - stage
```

1. `jane.doe@example.com` has the right to get and list any objects access all review namespaces.
2. `Administrators` can get, edit, list, and delete objects on the cluster level and in the namespaces `prod` and `stage`.

Because `Jane Doe` matches two rules, some calculations will be made:
* She will have the most powerful accessLevel across all matching rules — `ClusterAdmin`.
* The `limitNamespaces` options will be combined, so that Jane will have access to the following namespaces.

The resulting rights will be:

```yaml
accessLevel: ClusterAdmin
limitNamespaces:
- prod
- stage
- review-.*
```

> **Note!** If there is a rule without the limitNamespaces option, it means that all namespaces are allowed excluding system namespaces, which will affect the resulting limit namespaces calculation.
