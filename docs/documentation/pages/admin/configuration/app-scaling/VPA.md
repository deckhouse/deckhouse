---
title: "Vertical pod autoscaling"
permalink: en/admin/configuration/app-scaling/vpa.html
description: "Configure Vertical Pod Autoscaler (VPA) in Deckhouse Kubernetes Platform. Automatic container resource management and optimization based on usage metrics for improved efficiency."
---

## How Vertical Scaling (VPA) works

Vertical Pod Autoscaler (VPA) automates container resource management and significantly improves application efficiency. VPA is useful in scenarios where the exact resource requirements of an application are unknown in advance. When VPA is enabled and the appropriate operating mode is set, the requested resources are determined based on actual usage metrics gathered from the [monitoring system](../monitoring/). It is also possible to configure the system to only recommend resource settings without applying them automatically.

If the application load changes depending on time of day, user requests, or other factors, VPA automatically adjusts the allocated resources. This helps prevent outages due to lack of resources or excessive CPU and memory consumption.

## VPA Operating Modes

VPA can operate in two modes:

- Automatic resource adjustment:
  - **Auto** (default): changes resource requests without recreating pods. In current versions of Kubernetes, this mode behaves the same as **Recreate**: when resource changes are needed, VPA restarts the pod. However, in the future, with the introduction of [in-place resource updates](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates), the **Auto** mode will use this feature to modify resources without restarting the pod.
  - **Recreate**: VPA adjusts the resources of running pods by restarting them. For a single pod (replicas: 1), this will result in service unavailability during the restart. VPA does not restart pods that were created without a controller.

- Recommendations only, without modifying resources:
  - **Initial**: resource requests are set only at pod creation time, not during runtime.
  - **Off**: VPA does not change resources automatically. However, it still provides recommendations, which can be viewed using the `d8 k describe vpa` command.

When VPA is enabled and configured appropriately, resource requests are set automatically based on Prometheus data. It is also possible to configure the system to only provide recommendations without applying any changes.

## How to enable or disable VPA

You can enable VPA in the following ways:

1. Via a ModuleConfig resource (e.g., ModuleConfig/vertical-pod-autoscaler). Set the `spec.enabled` parameter to `true` or `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     enabled: true
   ```

1. Via the `d8` command (run inside the `d8-system/deckhouse pod`):

   ```console
   d8 system module enable vertical-pod-autoscaler
   ```

1. Through the [Deckhouse web interface](/modules/console/):

   - Go to the “Deckhouse → Modules” section;
   - Find the vertical-pod-autoscaler module and click on it;
   - Toggle the “Module enabled” switch.

The [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) module has no required configuration — it can be enabled and used with default settings.

After creating a [VerticalPodAutoscaler](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler) resource, you can check VPA recommendations using the following command:

```console
d8 k describe vpa my-app-vpa
```

In the `status` section, you’ll see the following parameters:

- `Target`: The recommended resource amount for the pod.
- `Lower Bound`: The minimum recommended resource amount for the application.
- `Upper Bound`: The maximum recommended resource amount for the application.
- `Uncapped Target`: The resource value based on extreme metrics without considering historical data.

## VPA configuration

1. Create the VPA module configuration.

   To configure VPA, you need to create a configuration file for the module. Example configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: vertical-pod-autoscaler
   spec:
     version: 1
     enabled: true
     settings:
       nodeSelector:
         node-role/system: ""
       tolerations:
       - key: dedicated.deckhouse.io
         operator: Equal
         value: system
    ```

1. Apply the VPA configuration file using `d8 k apply -f <your-config-file-name>`.

For more details on VPA resource limits configuration, see [the documentation](../../../user/configuration/app-scaling/vpa.html#how-vpa-interacts-with-limits).

### Module configuration example

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: vertical-pod-autoscaler
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/system: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: system
```
