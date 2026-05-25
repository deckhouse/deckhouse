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

## Protecting resources with sensitive fields

The following example demonstrates how to protect sensitive fields in resources using the `CRDSensitiveData` feature gate and the `x-kubernetes-sensitive-data` schema marker.

For instructions on enabling this feature, see [FAQ](faq.html#how-do-i-protect-sensitive-fields-in-custom-resources).

1. Enabling etcd encryption with the `encryptionEnabled` parameter. The `CRDSensitiveData` feature gate for `kube-apiserver` is enabled by default.

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

1. Defining sensitive fields in the resource schema.

   Fields marked with `x-kubernetes-sensitive-data: true` are encrypted in etcd and removed from API responses for callers that do not have access to the `<resource>/sensitive` subresource.

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

1. Creating a custom resource with values filled in sensitive fields.

   ```yaml
   apiVersion: example.com/v1
   kind: DbConfig
   metadata:
     name: primary
     namespace: default
   spec:
     host: db.example.com
     username: admin
     password: s3cr3t
   ```

   Once saved, the entire object is encrypted in etcd and the `password` value is hidden in the audit log and removed from the API responses of the caller has no permissions to the `dbconfigs/sensitive` subresource.

1. Configuring access to sensitive fields using RBAC and the `<resource>/sensitive` subresource.

   ```yaml
   # Regular role: can read the resource, but sensitive fields are removed from responses.
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: dbconfig-reader
   rules:
   - apiGroups: ["example.com"]
     resources: ["dbconfigs"]
     verbs: ["get", "list", "watch"]
   ---
   # Privileged role: can read full data, including sensitive fields.
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

1. A result of sensitive field protection.

   - A user with the `dbconfig-reader` role who runs `d8 k get dbconfig primary -o json` can see the resource with sensitive fields removed:

     ```json
     {
       "spec": {
         "host": "db.example.com",
         "username": "admin"
       }
     }
     ```

   - A user with the `dbconfig-sensitive-reader` role can see the full data:

     ```json
     {
       "spec": {
         "host": "db.example.com",
         "username": "admin",
         "password": "s3cr3t"
       }
     }
     ```

   - In audit logs, values of sensitive fields are always masked, regardless of caller permissions:

     ```json
     {
       "spec": {
         "host": "db.example.com",
         "username": "admin",
         "password": "******"
       }
     }
     ```
