---
title: "Cloud provider — Yandex Cloud: configuration"
---

Deckhouse Platform reads the credentials for accessing the Yandex Cloud API from the `d8-credentials` Secret, whose format is described in ["Credentials Secret"](environment.html#credentials-secret).

{% alert level="warning" %}
When the module settings change, the existing Machine objects are not recreated, and only new objects are created with the new parameters. Nodes are recreated when the parameters of [NodeGroup](/modules/node-manager/cr.html#nodegroup) and [YandexInstanceClass](cr.html#yandexinstanceclass) change. For recreating nodes manually, see ["How do I redeploy ephemeral machines in the cloud with a new configuration?"](/modules/node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).
{% endalert %}

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

## Module configuration

Every provider setting except the credentials is defined in the module's ModuleConfig resource. Deckhouse Platform applies the settings both when creating a cluster and in a running cluster.

The settings are grouped under four top-level parameters:

- [`provider`](#parameters-provider) sets the Yandex Cloud cloud and folder IDs, the parameter is required;
- [`nodes`](#parameters-nodes) sets the layout, the network parameters, and the parameters of the nodes being created, the parameter is required;
- [`storage`](#parameters-storage) sets the StorageClass the module creates;
- [`ccm`](#parameters-ccm) sets how the cloud controller manager works.

The nodes, storage, and cloud controller manager subsystems are turned off with the nested `disabled` parameter.

Example module configuration with provider, node, storage, and cloud controller manager settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: <SSH_PUBLIC_KEY>
    storage:
      parameters:
        excludedStorageClasses:
          - network-hdd
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4snovl2ko4p15em
```

Where:

- `CLOUD_ID` and `FOLDER_ID` are the Yandex Cloud cloud and folder IDs;
- `SSH_PUBLIC_KEY` is the public SSH key for accessing the nodes.

The virtual machine parameters are set in the [YandexInstanceClass](cr.html#yandexinstanceclass) resource that a [NodeGroup](/modules/node-manager/cr.html#nodegroup) refers to.

## Storage

The module automatically creates a StorageClass for every Yandex Cloud disk type:

| Type | StorageClass name | Note |
|---|---|---|
| network-hdd | network-hdd | |
| network-ssd | network-ssd | |
| network-ssd-nonreplicated | network-ssd-nonreplicated | Disk size must be a multiple of 93 GB. |
| network-ssd-io-m3         | network-ssd-io-m3 | Disk size must be a multiple of 93 GB. |

You can filter out the unnecessary StorageClasses via the [`excludedStorageClasses`](#parameters-storage-parameters-excludedstorageclasses) parameter.

The [`provisionedStorageClasses`](#parameters-storage-parameters-provisionedstorageclasses) parameter lets you create additional StorageClasses or override the parameters of StorageClasses created by the module by default.

Use the [`blockSize`](#parameters-storage-parameters-provisionedstorageclasses-blocksize) parameter to set the [block size](https://yandex.cloud/en/docs/compute/operations/disk-create/empty-disk-blocksize) for provisioned disks. The block size determines the maximum disk size. It is `8Ti` for `4Ki` and doubles with each next block size, up to `256Ti` for `128Ki`.

Example configuration of the `cloud-provider-yandex` module with an additional StorageClass configured with a block size of `64Ki`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 192.168.12.0/24
        sshPublicKey: <SSH_PUBLIC_KEY>
    storage:
      parameters:
        provisionedStorageClasses:
          - name: network-ssd-64k
            type: network-ssd
            blockSize: 64Ki
```

{% alert level="warning" %}
After a disk is created, its block size cannot be changed. Changing the `blockSize` parameter recreates the StorageClass, but does not change the block size of previously provisioned volumes.
{% endalert %}

## LoadBalancer

The module watches Service objects of the `LoadBalancer` type and creates a [network load balancer](https://yandex.cloud/en/docs/network-load-balancer/concepts/) and a [target group](https://yandex.cloud/en/docs/network-load-balancer/concepts/target-resources) for them in Yandex Cloud. The annotations that configure the balancer are listed in ["Service annotations"](examples.html#service-annotations).

For more information, see the [Kubernetes Cloud Controller Manager for Yandex Cloud documentation](https://github.com/flant/yandex-cloud-controller-manager).

{% include module-settings.liquid %}
