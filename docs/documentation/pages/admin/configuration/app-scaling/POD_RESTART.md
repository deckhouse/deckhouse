---
title: "Pod restart on configuration change"
permalink: en/admin/configuration/app-scaling/pod-restart.html
---

Deckhouse Kubernetes Platform supports automatic pod rollout (restart with new replicas) when ConfigMap or Secret resources change. This mechanism runs on system nodes and is based on the [Reloader](https://github.com/stakater/Reloader) project. It is controlled via annotations applied to workload controllers (Deployment, DaemonSet, StatefulSet).

> Reloader is not designed to be highly available.

The following annotations allow you to control pod restarts.

## Supported annotations

| Annotation | Applied to | Description | Example values |
|------------|------------|-------------|----------------|
| `pod-reloader.deckhouse.io/auto` | Deployment, DaemonSet, StatefulSet | Automatically restarts pods when any related ConfigMap or Secret (used as a volume or environment variable) is changed | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/search` | Deployment, DaemonSet, StatefulSet | Triggers restart only when a related resource is annotated with `pod-reloader.deckhouse.io/match: "true"` | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Specifies the list of `ConfigMap` objects that trigger a restart when changed | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload` | Deployment, DaemonSet, StatefulSet | Specifies the list of `Secret` objects that trigger a restart when changed | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match` | ConfigMap, Secret | Marks a resource as relevant for `pod-reloader.deckhouse.io/search: "true"` mode so its changes are tracked | `"true"`, `"false"` |

> The `pod-reloader.deckhouse.io/search` annotation must not be used together with `pod-reloader.deckhouse.io/auto: "true"`. In this case, both `pod-reloader.deckhouse.io/search` and `pod-reloader.deckhouse.io/match` annotations will be ignored. To ensure proper operation, either set `pod-reloader.deckhouse.io/auto` to `"false"` or remove it.
>
> Likewise, `configmap-reload` and `secret-reload` will be ignored if `pod-reloader.deckhouse.io/auto: "true"` is set. Disable `auto` to make them effective.

## Usage examples

### Tracking all changes in all attached resources: mounted as volumes or used in environment values


```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
  annotations:
    pod-reloader.deckhouse.io/auto: "true"
spec:
  template:
    spec:
      containers:
        - name: nginx
          env:
            - name: SECRET_WORD
              valueFrom:
                secretKeyRef:
                  name: nginx-secret-value
                  key: extra
          volumeMounts:
            - name: pages
              mountPath: "/usr/share/nginx/pages"
      volumes:
        - name: pages
          configMap:
            name: nginx-pages
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: nginx-secret-value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-pages
```

### Tracking changes in specific resources

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    pod-reloader.deckhouse.io/search: "true"
spec:
  template:
    spec:
      containers:
        - name: nginx
          env:
            - name: SECRET_WORD
              valueFrom:
                secretKeyRef:
                  name: nginx-secret-value
                  key: extra
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: nginx-secret-value
  annotations:
    pod-reloader.deckhouse.io/match: "true"
```

### Tracking changes in resources from the list

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    pod-reloader.deckhouse.io/configmap-reload: "nginx-config,nginx-pages"
spec:
  template:
    spec:
      containers:
        - name: nginx
          volumeMounts:
            - name: pages
              mountPath: "/usr/share/nginx/pages"
            - name: config
              mountPath: "/etc/nginx/templates"
      volumes:
        - name: pages
          configMap:
            name: nginx-pages
        - name: config
          configMap:
            name: nginx-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-pages
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-config
```

## Enabling or disabling pod restart

You can enable or disable the pod restart functionality in the following ways:

1. Using a `ModuleConfig` resource (e.g., `ModuleConfig/pod-reloader`). Set the `spec.enabled` field to `true` or `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: pod-reloader
   spec:
     enabled: true
   ```

1. Using the `d8` command (in the `d8-system/deckhouse` pod):

   ```console
   d8 platform module enable pod-reloader
   ```

## Configuration

The pod restart mechanism works out of the box and does not require any mandatory configuration. By default, it is enabled in the Default and Managed module bundles and disabled in the Minimal bundle.

If needed, its behavior can be adjusted using the ModuleConfig resource.

Available parameters:

| Parameter         | Type     | Description                                                                 | Default      |
|------------------|----------|-----------------------------------------------------------------------------|--------------|
| `reloadOnCreate` | boolean  | Enables restart when a `ConfigMap` or `Secret` is created, not only modified | `true`       |
| `nodeSelector`   | object   | Restricts component placement to specific nodes (same as `spec.nodeSelector`) | Not set      |
| `tolerations`    | array    | Allows scheduling on tainted nodes (same as `spec.tolerations`)             | Not set      |
