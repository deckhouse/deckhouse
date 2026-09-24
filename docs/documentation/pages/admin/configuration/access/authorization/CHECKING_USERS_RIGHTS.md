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

For a full picture of what a user, group, or ServiceAccount is allowed to do — not just one verb on one resource — use the [SubjectAccessReport](#getting-a-full-permission-report) resource described below.

## Checking whether a user has access

To check whether a user has the necessary permissions with a raw SubjectAccessReview request, run the following command, which includes:

- `resourceAttributes` (as in RBAC): Permission checking target.
- `user`: User name.
- `groups`: User groups.

{% alert level="info" %}
If you’re using the [`user-authn`](/modules/user-authn/) module, you can see the user’s name and groups in the Dex logs
(only logged during authorization) by running:

```shell
d8 k -n d8-user-authn logs -l app=dex
```

{% endalert %}

```shell
cat  <<EOF | 2>&1 d8 k create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
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
  "reason": "RBAC: allowed by ClusterRoleBinding \"system:kube-controller-manager\" of ClusterRole \"system:kube-controller-manager\" to User \"system:kube-controller-manager\""
}
```

Example response if the user does not have access permissions:

```json
{
  "allowed": false
}
```

The checks above only cover the granular role model, enforced by RBAC directly. If **multitenancy** mode is enabled in the cluster and the [basic role model](/modules/user-authz/#basic-role-based-model) (ClusterAuthorizationRule/AuthorizationRule) is also in use, its namespace restrictions are enforced by a separate authorization webhook — run an additional check against it to verify that the user has access to the namespace:

```shell
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
    ]
  }
}
EOF
```

Example response if the user has access permissions:

```json
{
  "allowed": false
}
```

A response with `"allowed": false` means the webhook is not blocking the request.
If the webhook does block the request, you will see an error message like this:

```json
{
  "allowed": false,
  "denied": true,
  "reason": "making cluster scoped requests for namespaced resources are not allowed"
}
```

## Getting a full permission report

With multitenancy mode enabled ([`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)), the `SubjectAccessReport` resource of the `user-authz` module gives a ready-made report of everything a subject is allowed to do, instead of checking one verb on one resource at a time: which roles are granted through which bindings, which actions on which resources are allowed cluster-wide and in every namespace, and where each permission comes from.

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

The `spec.subject.kind` field accepts `User`, `Group`, and `ServiceAccount` (for the latter, `spec.subject.namespace` is required). If `spec.subject` is omitted, the report is built for the caller.

A report about oneself is available to every authenticated user and requires no extra permissions. Building a report about **another** subject requires the `d8:user-authz:subject-access-checker` cluster role, which is not bound to anyone by default.

There is also a reverse query — the `WhoCan` resource — which answers "who can perform action X on resource Y?" and returns the list of users, groups, and ServiceAccounts. Creating `WhoCan` queries requires the `d8:user-authz:who-can-checker` cluster role. For details on both resources, see the [`user-authz` module FAQ](/modules/user-authz/faq.html#how-do-i-find-out-what-a-specific-user-group-or-serviceaccount-is-allowed-to-do).
