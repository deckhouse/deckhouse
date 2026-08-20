---
title: "Cloud provider — Yandex Cloud: configuration"
---

> **Note!** If the parameters provided below are changed, the **existing Machines are NOT redeployed** (new machines will be created with the updated parameters). Redeployment is only performed when `NodeGroup` and `YandexInstanceClass` are changed. Details in the [node-manager module's documentation](/modules/node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

## Storage

The module automatically creates StorageClasses covering all available disks in Yandex:

| Type | StorageClass Name | Comment |
|---|---|---|
| network-hdd | network-hdd | |
| network-ssd | network-ssd | |
| network-ssd-nonreplicated | network-ssd-nonreplicated | |
| network-ssd-io-m3         | network-ssd-io-m3 | Disk size must be a multiple of 93 GB. |

You can filter out the unnecessary StorageClasses via the [exclude](#parameters-storageclass-exclude) parameter.

Use the [provision](#parameters-storageclass-provision) parameter to create additional StorageClasses or to override the parameters of the ones created by default. It also allows setting the [block size](https://yandex.cloud/en/docs/compute/operations/disk-create/empty-disk-blocksize) of the disks, which defines their maximum size: `8Ti` for the default `4Ki` block, and twice as much for every next block size, up to `256Ti` for the `128Ki` block.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  settings:
    storageClass:
      provision:
      - name: network-ssd-64k
        type: network-ssd
        blockSize: 64Ki
```

> The block size of an existing disk cannot be changed. Changing the `blockSize` parameter recreates the StorageClass, but the volumes provisioned before the change keep their block size.

## LoadBalancer

The module subscribes to Service objects of the `LoadBalancer` type and creates the corresponding `NetworkLoadBalancer` and `TargetGroup` in Yandex Cloud.

For more information, see the [Kubernetes Cloud Controller Manager for Yandex Cloud documentation](https://github.com/flant/yandex-cloud-controller-manager).

{% include module-settings.liquid %}
