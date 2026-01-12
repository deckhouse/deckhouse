---
title: "Pod restart on configuration change"
permalink: en/admin/configuration/app-scaling/pod-restart.html
description: "Configure automatic pod restart on configuration changes in Deckhouse Kubernetes Platform. Pod reloader integration for ConfigMap and Secret updates with pod restart automation."
---

Deckhouse Kubernetes Platform can automatically restart Pods when certain ConfigMap or Secret resources are modified. This functionality is based on the [Reloader](https://github.com/stakater/Reloader) project and is controlled via annotations added to Pod controllers (Deployment, DaemonSet, StatefulSet).

{% alert %}
Reloader is not designed to be highly available.
{% endalert %}

The following annotations allow you to control pod restarts.

## Supported annotations

| Annotation | Applies to objects | Purpose | Example values |
|-----------|--------------------|---------|----------------|
| `pod-reloader.deckhouse.io/auto` | Deployment, DaemonSet, StatefulSet | Automatically restarts Pods when any related ConfigMap or Secret changes (used as a volume or environment variable) | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/search` | Deployment, DaemonSet, StatefulSet | Restarts only when resources annotated with `pod-reloader.deckhouse.io/match: "true"` are changed | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Specifies particular `ConfigMap` resources that should trigger a restart when changed | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload` | Deployment, DaemonSet, StatefulSet | Specifies particular `Secret` resources that should trigger a restart when changed | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match` | ConfigMap, Secret | Marks resources to be tracked when `pod-reloader.deckhouse.io/search: "true"` is used | `"true"`, `"false"` |

{% alert level="warning"%}
The `pod-reloader.deckhouse.io/search` annotation must not be used together with `pod-reloader.deckhouse.io/auto: "true"`. In this case, both `pod-reloader.deckhouse.io/search` and `pod-reloader.deckhouse.io/match` annotations will be ignored. To ensure correct behavior, set `pod-reloader.deckhouse.io/auto: "false"` or remove it.

The `pod-reloader.deckhouse.io/configmap-reload` and `pod-reloader.deckhouse.io/secret-reload` annotations do not work when `pod-reloader.deckhouse.io/auto: "true"` is present. To ensure correct behavior, disable `auto`.
{% endalert %}

## Usage examples

### Tracking all changes in all attached resources

Connected resources can be used either as environment variables or mounted as volumes.

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

Indicates that only those Secrets or ConfigMaps with the annotation `pod-reloader.deckhouse.io/match: "true"` should be monitored. This is useful when a pod uses many resources, but restarts are required only upon changes to specific ones.

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

Explicitly specifies a list of ConfigMaps that should trigger a pod restart when changed.

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
   d8 system module enable pod-reloader
   ```

1. Through the [Deckhouse web interface](/modules/console/):

   - Go to the “Deckhouse → Modules” section;
   - Find the `pod-reloader` module and click on it;
   - Toggle the “Module enabled” switch.

## Configuration

The pod restart mechanism works out of the box and does not require any mandatory configuration. By default, it is enabled in the Default and Managed module bundles and disabled in the Minimal bundle.

If needed, its behavior can be adjusted in the settings of the [pod-reloader](/modules/pod-reloader/) module (ModuleConfig `pod-reloader`).

Available parameters:

| Parameter         | Type    | Description                                                                 | Default&nbsp;Value          |
|------------------|---------|-----------------------------------------------------------------------------|-----------------------------|
| `reloadOnCreate` | boolean | Restart on ConfigMap or Secret creation, not only on modification           | `true`                      |
| `nodeSelector`   | object  | Limits the nodes where the component can run (equivalent to `spec.nodeSelector`) | Not&nbsp;set               |
| `tolerations`    | array   | Allows scheduling on tainted nodes (equivalent to `spec.tolerations`)      | Not&nbsp;set               |
