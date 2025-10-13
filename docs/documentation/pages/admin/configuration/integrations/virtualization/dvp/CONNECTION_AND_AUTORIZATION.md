---
title: Connection and authorization
permalink: en/admin/integrations/virtualization/dvp/authorization.html
---

To interact with DVP resources, Deckhouse Kubernetes Platform components use the DVP API. To configure access, create a user (ServiceAccount), assign the necessary permissions, and generate a kubeconfig.

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

## Assigning a role

Assign a role to the created user in the DVP cluster using the following command:

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

To generate a kubeconfig, follow the [instruction](/modules/user-authn/faq.html#how-to-generate-a-kubeconfig-and-access-kubernetes-api).
