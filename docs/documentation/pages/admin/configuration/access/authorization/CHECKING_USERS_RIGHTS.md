---
title: "Checking user permissions"
permalink: en/admin/configuration/access/authorization/check.html
---

To check whether a user has the necessary permissions, run the following command, which includes:

- `resourceAttributes` (as in RBAC): Permission checking target.
- `user`: User name.
- `groups`: User groups.

{% alert level="info" %}
If you’re using the `user-authn` module, you can view the user’s groups and name in the Dex logs
(displayed only during authorization) using the following command:

```shell
kubectl -n d8-user-authn logs -l app=dex
```

{% endalert %}

```shell
cat  <<EOF | 2>&1 kubectl  create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
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

If **multitenancy** mode is enabled in the cluster,
run an additional check to verify that the user has access to the namespace:

```shell
cat  <<EOF | 2>&1 kubectl --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
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
