---
title: "Managing control plane: examples"
---

## The connection of an external scheduler plugin

Example of connecting an external scheduler plugin via webhook.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: sds-replicated-volume
webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler
      namespace: d8-sds-replicated-volume
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
```

## CRD with sensitive fields

This example demonstrates how to protect sensitive fields in a Custom Resource using the `CRDSensitiveData`
feature gate together with the `x-kubernetes-sensitive-data` schema marker.

### 1. Enable encryption

Turning on `apiserver.encryptionEnabled` automatically enables the `CRDSensitiveData` feature gate for `kube-apiserver` — there is no separate switch:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 2
  enabled: true
  settings:
    apiserver:
      encryptionEnabled: true
```

### 2. Define a CRD with sensitive fields

Fields marked with `x-kubernetes-sensitive-data: true` will be encrypted in etcd and filtered
from API responses for callers without access to the `<resource>/sensitive` subresource.

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: dbconfigs.example.com
spec:
  group: example.com
  scope: Namespaced
  names:
    plural: dbconfigs
    singular: dbconfig
    kind: DbConfig
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                host:
                  type: string
                username:
                  type: string
                password:
                  type: string
                  x-kubernetes-sensitive-data: true
```

### 3. Configure RBAC

Grant access to sensitive fields via the `<resource>/sensitive` subresource:

```yaml
# Regular role: can read the resource, but sensitive fields are stripped.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dbconfig-reader
rules:
- apiGroups: ["example.com"]
  resources: ["dbconfigs"]
  verbs: ["get", "list", "watch"]
---
# Privileged role: can read full data including sensitive fields.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dbconfig-sensitive-reader
rules:
- apiGroups: ["example.com"]
  resources: ["dbconfigs"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["example.com"]
  resources: ["dbconfigs/sensitive"]
  verbs: ["get", "list", "watch"]
```

### 4. Observe the result

A caller bound to `dbconfig-reader` will see the resource with sensitive fields stripped:

```json
{
  "spec": {
    "host": "db.example.com",
    "username": "admin"
  }
}
```

A caller bound to `dbconfig-sensitive-reader` will see the full data:

```json
{
  "spec": {
    "host": "db.example.com",
    "username": "admin",
    "password": "s3cr3t"
  }
}
```

In audit logs, sensitive values are always masked, regardless of the caller's permissions:

```json
{
  "spec": {
    "host": "db.example.com",
    "username": "admin",
    "password": "******"
  }
}
```
