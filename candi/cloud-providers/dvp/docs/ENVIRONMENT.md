---
title: "Cloud provider — DVP: preparing environment"
description: "Configuring Deckhouse for DVP cloud provider operation"
---

Deckhouse components interact with DVP resources through the DVP API.
To configure this connection, create a new user (ServiceAccount), assign the necessary permissions, and generate a kubeconfig.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

## Creating a user

Create a new user in the DVP cluster using the following command:

```bash
d8 k create -f -<<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa-demo
  namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: sa-demo-token
  namespace: default
  annotations:
    kubernetes.io/service-account.name: sa-demo
type: kubernetes.io/service-account-token
EOF
```

## Adding a role

Add a role to the created user in the DVP cluster using the following command:

```bash
d8 k create -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: sa-demo-rb
  namespace: default
subjects:
  - kind: ServiceAccount
    name: sa-demo
    namespace: default
roleRef:
  kind: ClusterRole
  name: d8:use:role:manager
  apiGroup: rbac.authorization.k8s.io
EOF
```

## Generating a kubeconfig

To generate a kubeconfig, follow the [user creation guide](/products/kubernetes-platform/documentation/v1/modules/user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access) starting from **step 3**.
