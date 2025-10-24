---
title: "Module registry: usage example"
description: "Examples for switching between Direct and Unmanaged registry modes in Deckhouse Kubernets Platform, including configuration examples and status monitoring."
---

## Switching to the `Direct` Mode

To switch an already running cluster to `Direct` mode, follow these steps:

{% alert level="danger" %}
When changing the registry mode or registry parameters, Deckhouse will be restarted.
{% endalert %}

1. Before switching, perform the [migration to use the `registry` module](faq.html#how-to-migrate-to-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

   ```shell
   d8 platform queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Set the `Direct` mode configuration in the ModuleConfig `deckhouse`. If you're using a registry other than `registry.deckhouse.ru`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Direct
   target_mode: Direct
   ```

## Switching to the `Unmanaged` Mode

To switch an already running cluster to `Unmanaged` mode, follow these steps:

{% alert level="danger" %}
Changing the registry mode or its parameters will cause Deckhouse to restart.
{% endalert %}

1. Before switching, perform the [migration to use the `registry` module](faq.html#how-to-migrate-to-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

   ```shell
   d8 platform queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Set the `Unmanaged` mode configuration in the ModuleConfig `deckhouse`. If you're using a registry other than `registry.deckhouse.ru`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. If you need to switch back to the old registry management method, refer to the [instruction](faq.html#how-to-migrate-back-from-the-registry-module).

{% alert level="warning" %}
This is a deprecated format for registry management.
{% endalert %}
