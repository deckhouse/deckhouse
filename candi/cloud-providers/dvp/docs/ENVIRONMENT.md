---
title: "Cloud provider — DVP: preparing environment"
description: "Configuring Deckhouse for DVP cloud provider operation."
---

Deckhouse components use the DVP API to interact with resources in DVP. To configure this connection, you need to create a user and assign appropriate access rights to it, and generate kubeconfig

## User Creation

The user must be created in the DVP cluster

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

## Adding a Role

Add a role to a created user in a DVP cluster

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

## Generating kubeconfig

You can read how to generate kubeconfig in the link starting from the 3rd paragraph.

<https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authz/usage.html#%D1%81%D0%BE%D0%B7%D0%B4%D0%B0%D0%BD%D0%B8%D0%B5-serviceaccount-%D0%B4%D0%BB%D1%8F-%D1%81%D0%B5%D1%80%D0%B2%D0%B5%D1%80%D0%B0-%D0%B8-%D0%BF%D1%80%D0%B5%D0%B4%D0%BE%D1%81%D1%82%D0%B0%D0%B2%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D0%B5%D0%BC%D1%83-%D0%B4%D0%BE%D1%81%D1%82%D1%83%D0%BF%D0%B0>
