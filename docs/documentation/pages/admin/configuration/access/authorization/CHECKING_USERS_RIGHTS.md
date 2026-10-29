---
title: "Checking user permissions"
permalink: en/admin/configuration/access/authorization/check.html
description: "Check user permissions and access rights in Deckhouse Platform. RBAC permission verification, user access testing, and authorization debugging tools."
---

## Quick check of a single permission

The quickest way to check a single permission is the `d8 k auth can-i` command:

```shell
d8 k auth can-i --as=user@example.com get pods -n other-namespace
```

To see the full set of actions that a user, group, or ServiceAccount is allowed to perform — not just a single action on a single resource — use the [SubjectAccessReport](#getting-a-full-permission-report) resource described below.

## Checking whether a user has access

To check whether a user has access permissions, run the following command, which specifies:

* `resourceAttributes` (as in RBAC) — what access is checked for;
* `user` — the user name;
* `groups` — the user groups.

{% alert level="info" %}
When used together with the [`user-authn` module](/modules/user-authn/), the user name and groups can be found in the Dex logs with the `d8 k -n d8-user-authn logs -l app=dex` command. They appear in the logs only after the user signs in.
{% endalert %}

```shell
cat  <<EOF | 2>&1 d8 k create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
    ]
  }
}
EOF
```

The response will show whether access is allowed and which role grants it.

Example response if the user has access permissions:

```json
{
  "allowed": true,
  "reason": "RBAC: allowed by RoleBinding \"<BINDING_NAME>/dev\" of ClusterRole \"<ROLE_NAME>\" to User \"user@example.com\""
}
```

Example response if the user does not have access permissions:

```json
{
  "allowed": false
}
```

In **multitenancy** mode, the namespace restrictions of the [basic role model](rbac-current.html) (ClusterAuthorizationRule/AuthorizationRule) are enforced by the authorization webhook. The answer of the API server already includes the decision of the webhook. To see the answer of the webhook alone, run the following command as root on a [master node](../../../../glossary.html#master-node):

```shell
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
    ]
  }
}
EOF
```

Example response if the user does not have access permissions:

```json
{
  "allowed": false
}
```

A response with `"allowed": false` means the webhook is not blocking the request. If the webhook blocks the request, you get, for example, the following message:

```json
{
  "allowed": false,
  "denied": true,
  "reason": "either you have no access to the namespace or the namespace does not exist"
}
```

## Getting a full permission report

With multitenancy mode enabled ([`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)), the `SubjectAccessReport` resource of the `user-authz` module gives a ready-made report of everything a subject is allowed to do, instead of checking one verb on one resource at a time: which roles are granted through which bindings, which actions on which resources are allowed cluster-wide and in every namespace, and where each permission comes from.

Example of getting a report:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: SubjectAccessReport
metadata:
  name: what-can-jane-do
spec:
  subject:
    kind: User
    name: jane@example.com
EOF
```

The `spec.subject.kind` field accepts `User`, `Group`, and `ServiceAccount` (for the latter, `spec.subject.namespace` is required). If `spec.subject` is omitted, the report is generated for the caller.

A report about oneself is available to every authenticated user and requires no extra permissions. Building a report about **another** subject requires the `d8:user-authz:subject-access-checker` cluster role, which is not bound to anyone by default.

There is also a reverse query — the `WhoCan` resource — which answers "who can perform action X on resource Y?" and returns the list of users, groups, and ServiceAccounts. Example:

```shell
d8 k create -o yaml -f - <<EOF
apiVersion: authorization.deckhouse.io/v1alpha1
kind: WhoCan
metadata:
  name: who-can-create-networkpolicies
spec:
  resourceAttributes:
    namespace: my-namespace
    verb: create
    group: networking.k8s.io
    resource: networkpolicies
EOF
```

The answer is returned in the `status` field (it contains the `users`, `groups`, and `serviceAccounts` fields) right in the command output. The object is not stored anywhere.

The right to create `WhoCan` queries is granted by the `d8:user-authz:who-can-checker` cluster role. It is not bound to anyone by default: the result of a query reveals the access subjects in all namespaces, so grant it only to trusted administrators through a ClusterRoleBinding.

The managers of the `iam` and `security` [subsystems](../../../../glossary.html#subsystem) can build reports about other subjects and create `WhoCan` queries as well. This right comes with the `d8:subsystem-capability:iam:inspect_access` capability of the `d8:subsystem:iam:manager` and `d8:subsystem:security:manager` roles, so the roles above them have it too (`d8:subsystem:iam:superadmin`, `d8:subsystem:security:superadmin`, `d8:system:manager`, and `d8:system:superadmin`). For details on both resources, see the [`user-authz` module FAQ](/modules/user-authz/faq.html#how-do-i-find-out-what-a-specific-user-group-or-serviceaccount-is-allowed-to-do).
