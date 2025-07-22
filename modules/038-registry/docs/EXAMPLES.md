---
title: "Module registry: usage example"
description: ""
---

## Switching to Direct Mode

To switch an already running cluster to `Direct` mode, follow these steps:

{% alert level="danger" %}
- During the first switch, the `Containerd V1` service will be restarted, as the switch to the [new authorization configuration](./faq.html#how-to-prepare-containerd-v1) will take place.
- When changing the registry mode or registry parameters, Deckhouse will be restarted.
{% endalert %}

1. If the cluster is running with `Containerd V1`, [you need to prepare custom containerd configuration](./faq.html#how-to-prepare-containerd-v1).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

    ```bash
    kubectl get module registry -o wide
    # Example output:
    # NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
    # registry   38     ...  Ready   True                         True
    ```

1. Set the `Direct` mode configuration in the `ModuleConfig` for the `deckhouse` module. If you're using a registry other than `registry.deckhouse.ru`, refer to the [`deckhouse`](/products/kubernetes-platform/documentation/v1/modules/deckhouse/) module documentation for correct configuration.

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

1. Check the registry switch status in the `registry-state` secret using [this guide](./faq.html#how-to-check-the-registry-mode-switch-status). Example output:

    ```yaml
    ...
      - lastTransitionTime: "..."
        message: ""
        reason: ""
        status: "True"
        type: Ready
    hash: ..
    mode: Direct
    target_mode: Direct
    ```

## Switching to Unmanaged Mode

{% alert level="danger" %}
When changing the registry mode or registry parameters, Deckhouse will be restarted.
{% endalert %}

{% alert level="warning" %}
Switching to the `Unmanaged` mode is only available from `Direct` mode. Registry configuration parameters will be taken from the previously active mode.
{% endalert %}

To switch the cluster to `Unmanaged` mode, follow these steps:

1. Ensure that the `registry` module is running in `Direct` mode and the switch status to `Direct` is `Ready`. You can verify the state via the `registry-state` secret using [this guide](./faq.html#how-to-check-the-registry-mode-switch-status). Example output:

    ```yaml
    ...
      - lastTransitionTime: "..."
        message: ""
        reason: ""
        status: "True"
        type: Ready
    hash: ..
    mode: Direct
    target_mode: Direct
    ```

1. Set the `Unmanaged` mode in the `ModuleConfig` for the `deckhouse` module:

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
    ```

1. Check the registry switch status in the `registry-state` secret using [this guide](./faq.html#how-to-check-the-registry-mode-switch-status). Example output:

    ```yaml
    ...
      - lastTransitionTime: "..."
        message: ""
        reason: ""
        status: "True"
        type: Ready
    hash: ..
    mode: Unmanaged
    target_mode: Unmanaged
    ```

1. If you need to switch back to the previous `Containerd V1` auth configuration, refer to the [instruction](./faq.html#how-to-switch-back-to-the-previous-containerd-v1-auth-configuration).
